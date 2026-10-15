package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// Style est la mise en forme d'une cellule (JSON compact) : b, i, u
// (gras, italique, souligné), fg et bg (couleurs #rrggbb), al (left,
// center, right), nf (format de nombre : "0", "0,00", "%", "€", "date").
type Style map[string]any

// MaxStyleCells borne une mise en forme cellule par cellule.
const MaxStyleCells = 20_000

// merge applique un correctif : une valeur vide (false, "", nil) retire la
// propriété. Renvoie le style encodé ("" s'il ne reste rien).
func merge(old string, patch Style) (string, error) {
	s := Style{}
	if old != "" {
		if err := json.Unmarshal([]byte(old), &s); err != nil {
			s = Style{}
		}
	}
	for k, v := range patch {
		switch v := v.(type) {
		case nil:
			delete(s, k)
		case bool:
			if v {
				s[k] = true
			} else {
				delete(s, k)
			}
		case string:
			if v == "" {
				delete(s, k)
			} else {
				s[k] = v
			}
		default:
			s[k] = v
		}
	}
	if len(s) == 0 {
		return "", nil
	}
	b, err := json.Marshal(s) // encoding/json trie les clés : sortie stable
	return string(b), err
}

// ApplyStyle met en forme le rectangle [r0, r1] × [c0, c1] d'une feuille.
// Dans un tableau, une sélection qui couvre toutes les lignes d'une colonne
// met en forme la colonne entière (y compris les lignes ajoutées ensuite) ;
// sinon chaque cellule suit sa ligne (tri, insertion). Une seule étape
// d'annulation.
func (m *Model) ApplyStyle(ctx context.Context, sheet int64, r0, c0, r1, c1 int, patch Style) error {
	if (r1-r0+1)*(c1-c0+1) > MaxStyleCells*4 {
		return fmt.Errorf("mise en forme limitée à %d cellules", MaxStyleCells)
	}
	return m.Do(ctx, "Mise en forme", func(t *tx) error {
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		cells := 0
		for c := c0; c <= c1; c++ {
			var tb *Table
			for i := range tables {
				if c >= tables[i].Col && c <= tables[i].Right() && r1 >= tables[i].Row && r0 <= tables[i].Bottom() {
					tb = &tables[i]
				}
			}
			rows := []int{}
			for r := r0; r <= r1; r++ {
				if tb != nil && r >= tb.Row && r <= tb.Bottom() {
					continue // traité avec le tableau
				}
				rows = append(rows, r)
			}
			// Cellules libres (et cellules vides hors tableau).
			for _, r := range rows {
				if cells++; cells > MaxStyleCells {
					return fmt.Errorf("mise en forme limitée à %d cellules", MaxStyleCells)
				}
				if err := t.styleFree(sheet, r, c, patch); err != nil {
					return err
				}
			}
			if tb == nil {
				continue
			}
			colID := tb.Cols[c-tb.Col].ID
			if r0 <= tb.Row && tb.Row <= r1 {
				if err := t.styleTable(tb.ID, -1, colID, patch); err != nil { // en-tête
					return err
				}
			}
			first, last := max(r0, tb.Row+1), min(r1, tb.Bottom())
			if first > last {
				continue
			}
			if first == tb.Row+1 && last == tb.Bottom() {
				// Toute la colonne : style de colonne, et les styles de cellule
				// pour les mêmes propriétés s'effacent.
				if err := t.styleTable(tb.ID, 0, colID, patch); err != nil {
					return err
				}
				if err := t.clearCellStyles(tb.ID, colID, patch); err != nil {
					return err
				}
				continue
			}
			rs, err := t.query(`SELECT id FROM `+tb.sql()+` ORDER BY _pos LIMIT ? OFFSET ?`, last-first+1, first-tb.Row-1)
			if err != nil {
				return err
			}
			var ids []int64
			for rs.Next() {
				var id int64
				if err := rs.Scan(&id); err != nil {
					rs.Close()
					return err
				}
				ids = append(ids, id)
			}
			rs.Close()
			for _, id := range ids {
				if cells++; cells > MaxStyleCells {
					return fmt.Errorf("mise en forme limitée à %d cellules", MaxStyleCells)
				}
				if err := t.styleTable(tb.ID, id, colID, patch); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (t *tx) styleFree(sheet int64, row, col int, patch Style) error {
	var old string
	err := t.row(`SELECT style FROM _slitable_cell_styles WHERE sheet = ? AND row = ? AND col = ?`, sheet, row, col).Scan(&old)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	neu, err := merge(old, patch)
	if err != nil || neu == old {
		return err
	}
	set := func(s string) []stmt {
		out := []stmt{st(`DELETE FROM _slitable_cell_styles WHERE sheet = ? AND row = ? AND col = ?`, sheet, row, col)}
		if s != "" {
			out = append(out, st(`INSERT INTO _slitable_cell_styles (sheet, row, col, style) VALUES (?, ?, ?, ?)`, sheet, row, col, s))
		}
		return out
	}
	return t.apply(set(neu), set(old))
}

func (t *tx) styleTable(tbl, rid, col int64, patch Style) error {
	var old string
	err := t.row(`SELECT style FROM _slitable_styles WHERE tbl = ? AND rid = ? AND col = ?`, tbl, rid, col).Scan(&old)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	neu, err := merge(old, patch)
	if err != nil || neu == old {
		return err
	}
	set := func(s string) []stmt {
		out := []stmt{st(`DELETE FROM _slitable_styles WHERE tbl = ? AND rid = ? AND col = ?`, tbl, rid, col)}
		if s != "" {
			out = append(out, st(`INSERT INTO _slitable_styles (tbl, rid, col, style) VALUES (?, ?, ?, ?)`, tbl, rid, col, s))
		}
		return out
	}
	return t.apply(set(neu), set(old))
}

// clearCellStyles retire des styles de cellule d'une colonne les propriétés
// que l'on vient de poser sur toute la colonne.
func (t *tx) clearCellStyles(tbl, col int64, patch Style) error {
	rs, err := t.query(`SELECT rid FROM _slitable_styles WHERE tbl = ? AND col = ? AND rid > 0`, tbl, col)
	if err != nil {
		return err
	}
	var rids []int64
	for rs.Next() {
		var r int64
		if err := rs.Scan(&r); err != nil {
			rs.Close()
			return err
		}
		rids = append(rids, r)
	}
	rs.Close()
	clear := Style{}
	for k := range patch {
		clear[k] = nil
	}
	for _, r := range rids {
		if err := t.styleTable(tbl, r, col, clear); err != nil {
			return err
		}
	}
	return nil
}
