package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// Colonnes de référence (clés étrangères), décision 0016.
//
// Une validation par liste dont la source est exactement la colonne clé
// d'un tableau (Espèces[Nom], toute la colonne) fait de la colonne validée
// une référence vers ce tableau : rien de plus n'est rangé, la référence se
// déduit de la validation. Elle suit donc les renommages (rewriteValidations)
// et disparaît avec la validation (Ctrl+Z compris) ou la clé du tableau
// visé. Aucune contrainte FOREIGN KEY : la validation signale déjà les
// valeurs absentes, et les tableaux restent des grilles libres.

// Reference : la colonne Column du tableau Table cite la clé Key du tableau
// Target.
type Reference struct {
	Table  string `json:"table"`
	Column string `json:"column"`
	Target string `json:"target"`
	Key    string `json:"key"`
}

// Citation : lignes d'un tableau qui citent une valeur de clé (Lignes qui
// citent…) ; First est la première d'entre elles (identifiant, LocateRow).
type Citation struct {
	Table  string `json:"table"`
	Column string `json:"column"`
	Count  int    `json:"count"`
	First  int64  `json:"first"`
}

// KeyRow : la ligne du tableau dont la clé vaut value.
type KeyRow struct {
	Table string `json:"table"` // nom exact du tableau
	Key   string `json:"key"`   // sa colonne clé ("" : pas de clé)
	Found bool   `json:"found"`
	ID    int64  `json:"id,omitempty"`
}

// refTarget : le tableau dont src (source d'une validation) est la colonne
// clé entière, et le rang de cette colonne ; ok faux sinon.
func (t *tx) refTarget(src string) (Table, int, bool, error) {
	s, err := NormalizeDVSource(src)
	if err != nil {
		return Table{}, 0, false, nil
	}
	n, err := formula.Parse(s, formula.FR)
	if err != nil {
		return Table{}, 0, false, nil
	}
	r, ok := n.(formula.Ref)
	if !ok || r.Table == "" || r.Row || strings.TrimSpace(r.Col) == "" {
		return Table{}, 0, false, nil
	}
	tb, ok, err := t.tableFold(r.Table)
	if err != nil || !ok || tb.Key == 0 {
		return Table{}, 0, false, err
	}
	for i, c := range tb.Cols {
		if c.ID == tb.Key && strings.EqualFold(c.Name, strings.TrimSpace(r.Col)) {
			return tb, i, true, nil
		}
	}
	return Table{}, 0, false, nil
}

// tableFold : le tableau nommé name (sans casse), ok faux s'il n'existe pas.
func (t *tx) tableFold(name string) (Table, bool, error) {
	var id int64
	if err := t.row(`SELECT id FROM _lattis_tables WHERE name = ? COLLATE NOCASE`, strings.TrimSpace(name)).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Table{}, false, nil
		}
		return Table{}, false, err
	}
	tb, err := t.table(id)
	return tb, err == nil, err
}

// References liste les colonnes de référence du classeur (validation de
// toute une colonne de tableau tirée de la clé d'un tableau).
func (m *Model) References(ctx context.Context) ([]Reference, error) {
	out := []Reference{}
	err := m.read(ctx, func(t *tx) error {
		var err error
		out, err = t.references()
		return err
	})
	return out, err
}

func (t *tx) references() ([]Reference, error) {
	rs, err := t.query(`SELECT s.tbl, s.col, s.style FROM _lattis_styles s JOIN _lattis_columns c ON c.tbl = s.tbl AND c.id = s.col
		WHERE s.rid = 0 AND s.style LIKE '%"dv":{%' ORDER BY s.tbl, c.pos`)
	if err != nil {
		return nil, err
	}
	type dv struct {
		tbl, col int64
		src      string
	}
	var all []dv
	for rs.Next() {
		var d dv
		var s string
		if err := rs.Scan(&d.tbl, &d.col, &s); err != nil {
			rs.Close()
			return nil, err
		}
		var st map[string]any
		if json.Unmarshal([]byte(s), &st) != nil {
			continue
		}
		if d.src, _ = DVSource(st["dv"]); d.src != "" {
			all = append(all, d)
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	out := []Reference{}
	for _, d := range all {
		target, k, ok, err := t.refTarget(d.src)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		tb, err := t.table(d.tbl)
		if err != nil {
			continue
		}
		for _, c := range tb.Cols {
			if c.ID == d.col {
				out = append(out, Reference{Table: tb.Name, Column: c.Name, Target: target.Name, Key: target.Cols[k].Name})
			}
		}
	}
	return out, nil
}

// ReferenceRow : la ligne visée par une cellule dont la validation a pour
// source src et qui vaut value (« Basculer sur »). Table vide : src n'est
// pas la colonne clé d'un tableau.
func (m *Model) ReferenceRow(ctx context.Context, src, value string) (KeyRow, error) {
	var out KeyRow
	err := m.read(ctx, func(t *tx) error {
		tb, k, ok, err := t.refTarget(src)
		if err != nil || !ok {
			return err
		}
		out = KeyRow{Table: tb.Name, Key: tb.Cols[k].Name}
		out.ID, out.Found, err = t.rowByKey(tb, tb.Cols[k], t.ui().valueIn(value))
		return err
	})
	return out, err
}

// rowByKey : identifiant de la ligne dont la colonne c vaut value (sans
// casse pour un texte ou une date, comme la clé).
func (t *tx) rowByKey(tb Table, c Column, value string) (int64, bool, error) {
	if strings.TrimSpace(value) == "" {
		return 0, false, nil
	}
	v, err := grid.Parse(c.Type, value)
	if err != nil || v == nil {
		return 0, false, nil
	}
	var id int64
	err = t.row(`SELECT id FROM `+tb.sql()+` WHERE `+keyExpr(c.ID, c.Type)+` = ? ORDER BY _pos LIMIT 1`, v).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return id, err == nil, err
}

// Citations : pour la valeur value de la clé du tableau table, les colonnes
// de référence qui la citent, avec le nombre de lignes et la première.
func (m *Model) Citations(ctx context.Context, table, value string) ([]Citation, error) {
	out := []Citation{}
	err := m.read(ctx, func(t *tx) error {
		refs, err := t.references()
		if err != nil {
			return err
		}
		for _, r := range refs {
			if !strings.EqualFold(r.Target, table) {
				continue
			}
			tb, err := t.tableByName(r.Table)
			if err != nil {
				return err
			}
			for _, c := range tb.Cols {
				if !strings.EqualFold(c.Name, r.Column) {
					continue
				}
				v, err := grid.Parse(c.Type, value)
				if err != nil || v == nil {
					break
				}
				where := keyExpr(c.ID, c.Type) + ` = ?`
				var n int
				var first sql.NullInt64
				if err := t.row(`SELECT count(*), (SELECT id FROM `+tb.sql()+` WHERE `+where+` ORDER BY _pos LIMIT 1) FROM `+tb.sql()+` WHERE `+where, v, v).Scan(&n, &first); err != nil {
					return err
				}
				out = append(out, Citation{Table: tb.Name, Column: c.Name, Count: n, First: first.Int64})
				break
			}
		}
		return nil
	})
	return out, err
}
