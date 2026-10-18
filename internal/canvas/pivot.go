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
	// Rows décrit chaque ligne sous l'en-tête quand le TCD s'affiche en
	// arborescence (plusieurs champs en lignes) : niveau, groupe −/+,
	// sous-total. Absent avec un seul champ en lignes.
	Rows []PivotRowInfo `json:"rows"`
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
			taken, err := t.sheetNameTaken(name, 0)
			if err != nil {
				return err
			}
			if !taken {
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
		cells, rows, err := t.pivotCells(out[i].Config)
		if err != nil {
			out[i].Error = strings.TrimPrefix(err.Error(), ErrPivot.Error()+" : ")
			continue
		}
		out[i].Cells, out[i].Rows = cells, rows
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
		cells, rows, err := t.pivotCells(out.Config)
		if err != nil {
			return err
		}
		out.Cells, out.Rows = cells, rows
		out.Formats, err = t.pivotFormats(out.Config, cells)
		return err
	})
	if err != nil {
		out.Error = strings.TrimPrefix(err.Error(), ErrPivot.Error()+" : ")
	}
	return out
}

// MovePivot déplace un tableau croisé (couper-coller du tableau entier) :
// son coin haut gauche va en (row, col) de la feuille sheet.
func (m *Model) MovePivot(ctx context.Context, id, sheet int64, row, col int) error {
	if row < 0 || col < 0 {
		return fmt.Errorf("%w : hors de la feuille", ErrPivot)
	}
	return m.Do(ctx, "Déplacer le tableau croisé", func(t *tx) error {
		if err := t.guardObject(`_lattis_pivots`, id); err != nil {
			return err
		}
		if err := t.guard(sheet); err != nil {
			return err
		}
		var s0 int64
		var r0, c0 int
		var raw string
		if err := t.row(`SELECT sheet, row, col, config FROM _lattis_pivots WHERE id = ?`, id).Scan(&s0, &r0, &c0, &raw); err != nil {
			return fmt.Errorf("%w : tableau croisé introuvable", ErrPivot)
		}
		var cfg PivotConfig
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return err
		}
		if err := t.pivotFree(sheet, row, col, cfg.normalized(), id); err != nil {
			return err
		}
		q := `UPDATE _lattis_pivots SET sheet = ?, row = ?, col = ? WHERE id = ?`
		return t.apply([]stmt{st(q, sheet, row, col, id)}, []stmt{st(q, s0, r0, c0, id)})
	})
}

// CopyPivot pose une copie d'un tableau croisé (copier-coller du tableau
// entier), sous un nouveau nom, en (row, col) de la feuille sheet ; renvoie
// son identifiant.
func (m *Model) CopyPivot(ctx context.Context, id, sheet int64, row, col int) (int64, error) {
	if row < 0 || col < 0 {
		return 0, fmt.Errorf("%w : hors de la feuille", ErrPivot)
	}
	var nid int64
	err := m.Do(ctx, "Coller le tableau croisé", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		var raw string
		if err := t.row(`SELECT config FROM _lattis_pivots WHERE id = ?`, id).Scan(&raw); err != nil {
			return fmt.Errorf("%w : tableau croisé introuvable", ErrPivot)
		}
		var cfg PivotConfig
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return err
		}
		var err error
		nid, err = t.placePivot(sheet, row, col, cfg)
		return err
	})
	return nid, err
}

// PlacePivot pose en (row, col) de la feuille un tableau croisé de
// configuration cfg (collé depuis un document), sous un nouveau nom ;
// renvoie son identifiant.
func (m *Model) PlacePivot(ctx context.Context, sheet int64, row, col int, cfg PivotConfig) (int64, error) {
	if row < 0 || col < 0 {
		return 0, fmt.Errorf("%w : hors de la feuille", ErrPivot)
	}
	var nid int64
	err := m.Do(ctx, "Coller le tableau croisé", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		if _, err := t.pivotPlan(cfg.normalized()); err != nil {
			return err
		}
		var err error
		nid, err = t.placePivot(sheet, row, col, cfg)
		return err
	})
	return nid, err
}

// placePivot insère un tableau croisé à une place libre, sous un nouveau nom.
func (t *tx) placePivot(sheet int64, row, col int, cfg PivotConfig) (int64, error) {
	if err := t.pivotFree(sheet, row, col, cfg.normalized(), 0); err != nil {
		return 0, err
	}
	name, err := t.pivotName()
	if err != nil {
		return 0, err
	}
	cfg.Name = name
	var nid int64
	if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_pivots`).Scan(&nid); err != nil {
		return 0, err
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return 0, err
	}
	return nid, t.apply(
		[]stmt{st(`INSERT INTO _lattis_pivots (id, sheet, row, col, config) VALUES (?, ?, ?, ?, ?)`, nid, sheet, row, col, string(b))},
		[]stmt{st(`DELETE FROM _lattis_pivots WHERE id = ?`, nid)},
	)
}

// pivotFree vérifie qu'un tableau croisé posé en (row, col) de la feuille
// (filtres de rapport compris) ne recouvre ni un tableau ni un autre
// tableau croisé (skip : celui qu'on déplace).
func (t *tx) pivotFree(sheet int64, row, col int, cfg PivotConfig, skip int64) error {
	cells, _, err := t.pivotCells(cfg)
	if err != nil {
		return err
	}
	h, w := len(cells), 1
	if h > 0 {
		w = len(cells[0])
	}
	if len(cfg.Filters) > 0 {
		h += len(cfg.Filters) + 1
	}
	over := func(r0, c0, r1, c1 int) bool {
		return r0 <= row+h-1 && r1 >= row && c0 <= col+w-1 && c1 >= col
	}
	tables, err := t.tables(sheet)
	if err != nil {
		return err
	}
	for _, tb := range tables {
		if over(tb.Row, tb.Col, tb.Row+tb.Rows, tb.Col+len(tb.Cols)-1) {
			return fmt.Errorf("%w : la place est prise par le tableau « %s » ; choisissez une zone libre", ErrPivot, tb.Name)
		}
	}
	others, err := t.pivots(sheet)
	if err != nil {
		return err
	}
	for _, o := range others {
		if o.ID == skip || len(o.Cells) == 0 {
			continue
		}
		oh := len(o.Cells)
		if len(o.Config.Filters) > 0 {
			oh += len(o.Config.Filters) + 1
		}
		if over(o.Row, o.Col, o.Row+oh-1, o.Col+len(o.Cells[0])-1) {
			return fmt.Errorf("%w : la place est prise par le tableau croisé « %s » ; choisissez une zone libre", ErrPivot, o.Config.Name)
		}
	}
	return nil
}
