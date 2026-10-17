package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// PivotInfo est un tableau croisé posé sur une feuille, avec son résultat.
type PivotInfo struct {
	ID     int64       `json:"id"`
	Row    int         `json:"row"`
	Col    int         `json:"col"`
	Config PivotConfig `json:"config"`
	// Cells : en-tête (1 ligne), groupes, puis la ligne de total.
	Cells [][]string `json:"cells"`
	// Formats : format de nombre par défaut de chaque colonne (style JSON,
	// vide : aucun) : celui de la colonne source, ou deux décimales pour une
	// moyenne ou des décimaux. La mise en forme des cellules l'emporte.
	Formats []string `json:"formats"`
	Error   string   `json:"error"`
}

// ErrPivot signale un tableau croisé impossible.
var ErrPivot = errors.New("tableau croisé")

// Bornes d'un tableau croisé : au-delà, le résultat n'est plus lisible.
const (
	MaxPivotRows = 10000
	MaxPivotCols = 50
)

// CreatePivot crée un tableau croisé sur une nouvelle feuille, comme Excel,
// et renvoie l'identifiant de la feuille.
func (m *Model) CreatePivot(ctx context.Context, cfg PivotConfig) (int64, error) {
	cfg = cfg.normalized()
	var sheet int64
	err := m.Do(ctx, "Tableau croisé dynamique", func(t *tx) error {
		if err := t.guardWorkbook(); err != nil {
			return err
		}
		if _, err := t.pivotPlan(cfg); err != nil {
			return err
		}
		var pos int
		if err := t.row(`SELECT coalesce(max(id), 0) + 1, coalesce(max(pos), -1) + 1 FROM _lattis_sheets`).Scan(&sheet, &pos); err != nil {
			return err
		}
		name := ""
		for n := 1; ; n++ {
			name = fmt.Sprintf("TCD %d", n)
			var k int
			if err := t.row(`SELECT count(*) FROM _lattis_sheets WHERE name = ?`, name).Scan(&k); err != nil {
				return err
			}
			if k == 0 {
				break
			}
		}
		var id int64
		if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_pivots`).Scan(&id); err != nil {
			return err
		}
		if cfg.Name == "" {
			name, err := t.pivotName()
			if err != nil {
				return err
			}
			cfg.Name = name
		}
		b, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return t.apply([]stmt{
			st(`INSERT INTO _lattis_sheets (id, name, pos, gridlines) VALUES (?, ?, ?, 0)`, sheet, name, pos),
			st(`INSERT INTO _lattis_pivots (id, sheet, row, col, config) VALUES (?, ?, 1, 1, ?)`, id, sheet, string(b)),
		}, []stmt{
			st(`DELETE FROM _lattis_pivots WHERE id = ?`, id),
			st(`DELETE FROM _lattis_sheets WHERE id = ?`, sheet),
		})
	})
	return sheet, err
}

// SetPivot change la configuration d'un tableau croisé.
func (m *Model) SetPivot(ctx context.Context, id int64, cfg PivotConfig) error {
	cfg = cfg.normalized()
	return m.Do(ctx, "Modifier le tableau croisé", func(t *tx) error {
		if err := t.guardObject(`_lattis_pivots`, id); err != nil {
			return err
		}
		if _, err := t.pivotPlan(cfg); err != nil {
			return err
		}
		var old string
		if err := t.row(`SELECT config FROM _lattis_pivots WHERE id = ?`, id).Scan(&old); err != nil {
			return err
		}
		if cfg.Name == "" {
			var prev PivotConfig
			if json.Unmarshal([]byte(old), &prev) == nil {
				cfg.Name = prev.Name
			}
		}
		b, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		q := `UPDATE _lattis_pivots SET config = ? WHERE id = ?`
		return t.apply([]stmt{st(q, string(b), id)}, []stmt{st(q, old, id)})
	})
}

// DeletePivot supprime un tableau croisé (sa feuille reste).
func (m *Model) DeletePivot(ctx context.Context, id int64) error {
	return m.Do(ctx, "Supprimer le tableau croisé", func(t *tx) error {
		if err := t.guardObject(`_lattis_pivots`, id); err != nil {
			return err
		}
		var sheet int64
		var row, col int
		var cfg string
		if err := t.row(`SELECT sheet, row, col, config FROM _lattis_pivots WHERE id = ?`, id).Scan(&sheet, &row, &col, &cfg); err != nil {
			return err
		}
		return t.apply(
			[]stmt{st(`DELETE FROM _lattis_pivots WHERE id = ?`, id)},
			[]stmt{st(`INSERT INTO _lattis_pivots (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, sheet, row, col, cfg)},
		)
	})
}

// pivots donne les tableaux croisés d'une feuille, calculés.
func (t *tx) pivots(sheet int64) ([]PivotInfo, error) {
	rs, err := t.query(`SELECT id, row, col, config FROM _lattis_pivots WHERE sheet = ? ORDER BY id`, sheet)
	if err != nil {
		return nil, err
	}
	var out []PivotInfo
	for rs.Next() {
		var p PivotInfo
		var cfg string
		if err := rs.Scan(&p.ID, &p.Row, &p.Col, &cfg); err != nil {
			rs.Close()
			return nil, err
		}
		if err := json.Unmarshal([]byte(cfg), &p.Config); err != nil {
			p.Error = "configuration illisible"
		}
		p.Config = p.Config.normalized()
		if p.Config.Name == "" {
			p.Config.Name = fmt.Sprintf("TCD%d", p.ID)
		}
		out = append(out, p)
	}
	rs.Close()
	for i := range out {
		if out[i].Error != "" {
			continue
		}
		cells, err := t.pivotCells(out[i].Config)
		if err != nil {
			out[i].Error = strings.TrimPrefix(err.Error(), ErrPivot.Error()+" : ")
			continue
		}
		out[i].Cells = cells
		if out[i].Formats, err = t.pivotFormats(out[i].Config, cells); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// pivotIDs liste les tableaux croisés d'une feuille (suppression de feuille).
func (t *tx) pivotRows(sheet int64) ([]stmt, error) {
	rs, err := t.query(`SELECT id, row, col, config FROM _lattis_pivots WHERE sheet = ?`, sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var undo []stmt
	for rs.Next() {
		var id int64
		var row, col int
		var cfg sql.NullString
		if err := rs.Scan(&id, &row, &col, &cfg); err != nil {
			return nil, err
		}
		undo = append(undo, st(`INSERT INTO _lattis_pivots (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, id, sheet, row, col, cfg.String))
	}
	return undo, rs.Err()
}

// PivotData calcule un tableau croisé qui n'est posé sur aucune feuille
// (onglet Document) ; une erreur est rendue dans Error.
func (m *Model) PivotData(ctx context.Context, cfg PivotConfig) PivotInfo {
	out := PivotInfo{Config: cfg.normalized(), Cells: [][]string{}, Formats: []string{}}
	err := m.read(ctx, func(t *tx) error {
		cells, err := t.pivotCells(out.Config)
		if err != nil {
			return err
		}
		out.Cells = cells
		out.Formats, err = t.pivotFormats(out.Config, cells)
		return err
	})
	if err != nil {
		out.Error = strings.TrimPrefix(err.Error(), ErrPivot.Error()+" : ")
	}
	return out
}
