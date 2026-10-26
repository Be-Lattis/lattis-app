package canvas

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
)

// Un tableau croisé se lit dans les formules comme un tableau nommé, à la
// manière d'une source externe : TCD1[Nombre] est la colonne entière (lignes
// de détail seulement, un total n'est pas compté deux fois), TCD1[Nombre][2]
// la deuxième ligne de détail. Ses colonnes saisies (pivothand.go) en sont
// des colonnes comme les autres ; en arborescence, chaque champ des lignes
// aussi ([Ville]).
//
// Le TCD n'est pas dans le schéma en cache (il faudrait calculer tous les TCD
// du classeur à chaque écriture) : formula.Schema.Extra le calcule quand une
// formule cite son nom, une fois par génération du Store en lecture.
//
// Limite (comme la lecture par adresse) : les colonnes saisies à formule y
// sont calculées sans les cellules libres qu'elles lisent (calcul des
// cellules libres en cours : pas de récursion).

// pivotFormulaTable trouve le TCD de ce nom, toutes feuilles, comme tableau
// de formules ; ok faux si aucun TCD ne s'appelle ainsi.
func (t *tx) pivotFormulaTable(name string) (*formula.Table, bool) {
	low := strings.ToLower(strings.TrimSpace(name))
	if low == "" || t.pivotTables[low] {
		return nil, false // un TCD dont les colonnes saisies se citent l'une l'autre
	}
	ft, err := cached(t, "pivottable:"+low, func() (*formula.Table, error) {
		if t.pivotTables == nil {
			t.pivotTables = map[string]bool{}
		}
		t.pivotTables[low] = true
		defer delete(t.pivotTables, low)
		return t.pivotFormulaTableRaw(low)
	})
	if err != nil || ft == nil {
		return nil, false
	}
	return ft, true
}

func (t *tx) pivotFormulaTableRaw(low string) (*formula.Table, error) {
	ps, err := cached(t, "allpivots", t.allPivots)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		if strings.ToLower(p.Cfg.Name) == low {
			return t.pivotTableOf(p)
		}
	}
	return nil, nil
}

// pivotTableOf décrit le TCD p, tel qu'affiché (filtre de son onglet à choix
// compris), en tableau de formules : une ligne par ligne de détail.
func (t *tx) pivotTableOf(p pivotRow) (*formula.Table, error) {
	cfg := p.Cfg
	ft := &formula.Table{Name: cfg.Name, Sheet: p.Sheet}
	view, err := t.pivotViewOf(cfg)
	cells, rows := view.cells, view.rows
	if err != nil || len(cells) == 0 {
		// TCD en erreur : un tableau sans colonne (ses références donnent #REF!).
		ft.SQL = "(SELECT NULL AS id, NULL AS _pos WHERE 0)"
		return ft, nil
	}
	keys := pivotRowKeys(cfg, cells, rows)
	var extra [][]string
	var vals handVals
	if len(cfg.Hand) > 0 {
		if vals, err = t.pivotHandVals(pivotOwner(p.ID)); err != nil {
			return nil, err
		}
		if extra, err = t.pivotHandCells(p.ID, p.Sheet, cfg, cells, rows, nil); err != nil {
			return nil, err
		}
	}
	// Avec plusieurs champs en colonnes, chaque colonne porte son chemin
	// (TCD1[2024 · T1]).
	header := pivotFullHeader(view)
	compact := rows != nil
	taken := map[string]bool{}
	var sel []string
	var value []func(i int) string
	add := func(name string, v func(i int) string) {
		low := strings.ToLower(strings.TrimSpace(name))
		if low == "" || taken[low] {
			return
		}
		taken[low] = true
		alias := fmt.Sprintf("c%d", len(ft.Cols))
		sel = append(sel, fmt.Sprintf("column%d AS %s", len(ft.Cols)+3, alias))
		ft.Cols = append(ft.Cols, formula.Column{Name: strings.TrimSpace(name), SQL: alias})
		value = append(value, v)
	}
	for j, h := range header {
		add(h, func(i int) string {
			if j < len(cells[i]) {
				return sqlValue(cells[i][j])
			}
			return "NULL"
		})
	}
	if compact {
		for k, f := range cfg.RowFields {
			add(f, func(i int) string {
				if i-1 < len(rows) && k < len(rows[i-1].Path) && rows[i-1].Path[k] != "" {
					return sqlValue(rows[i-1].Path[k])
				}
				return "NULL"
			})
		}
	}
	for j, h := range cfg.Hand {
		add(h.Name, func(i int) string {
			if h.Formula == "" {
				if keys[i] == "" {
					return "NULL"
				}
				return handSQL(vals[keys[i]][h.H])
			}
			if i < len(extra) && j < len(extra[i]) {
				return sqlValue(extra[i][j])
			}
			return "NULL"
		})
	}
	cols := append([]string{"column1 AS id", "column2 AS _pos"}, sel...)
	var lines []string
	for i := 1; i < len(cells); i++ {
		if !pivotDetail(cfg, keys, i) {
			continue
		}
		parts := []string{strconv.Itoa(i), strconv.Itoa(len(lines) + 1)}
		for _, v := range value {
			parts = append(parts, v(i))
		}
		lines = append(lines, "("+strings.Join(parts, ", ")+")")
	}
	if len(lines) == 0 {
		empty := []string{"NULL AS id", "NULL AS _pos"}
		for _, c := range ft.Cols {
			empty = append(empty, "NULL AS "+c.SQL)
		}
		ft.SQL = "(SELECT " + strings.Join(empty, ", ") + " WHERE 0)"
		return ft, nil
	}
	ft.SQL = "(SELECT " + strings.Join(cols, ", ") + " FROM (VALUES " + strings.Join(lines, ", ") + "))"
	return ft, nil
}

// pivotDetail : la ligne i (indice dans les cellules du TCD) est-elle une
// ligne de détail ? Ni total général, ni groupe, ni sous-total ; sans champ
// en lignes, la seule ligne (le total) en est une.
func pivotDetail(cfg PivotConfig, keys []string, i int) bool {
	return keys[i] != "" || len(cfg.RowFields) == 0
}

// pivotSchemaTable : le tableau name du schéma est-il un TCD (trouvé par
// Schema.Extra, pas un tableau du classeur) ?
func pivotSchemaTable(s *formula.Schema, name string) bool {
	for _, ft := range s.Tables {
		if strings.EqualFold(ft.Name, name) {
			return false
		}
	}
	_, ok := s.Table(name)
	return ok
}
