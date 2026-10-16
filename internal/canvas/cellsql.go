package canvas

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/lalamefine/slitable/internal/formula"
)

// cellRefRe trouve les adresses de cellules libres d'un texte (formule ou
// requête SQL) : Cell[A1], Feuille2!Cell[B3], 'Ma feuille'!Cell[B3], et les
// anciennes formes Cell_A1, Feuille2.Cell_B3, "Ma feuille".Cell_B3. Les
// chaînes entre guillemets ou apostrophes sont reconnues pour être sautées.
//
// Groupes : 1 feuille entre apostrophes, 2 feuille entre guillemets, 3 nom de
// feuille ; 4 adresse après !Cell[ ; 5 adresse après .Cell_ ; 6 adresse de
// Cell[ ] seule ; 7 adresse de Cell_ seule.
var cellRefRe = regexp.MustCompile(`(?i)(?:'((?:[^']|'')+)'|"((?:[^"]|"")+)"|([\p{L}_][\p{L}\p{N}_]*))(?:!cell\[\s*([a-z]{1,3}[1-9][0-9]*)\s*\]|\.cell_([a-z]{1,3}[1-9][0-9]*)\b)|'(?:[^']|'')*'|"(?:[^"]|"")*"|\bcell\[\s*([a-z]{1,3}[1-9][0-9]*)\s*\]|\bcell_([a-z]{1,3}[1-9][0-9]*)\b`)

// cellRefs remplace chaque adresse de cellule d'un texte : fn reçoit la
// feuille citée ("" : celle du contexte), l'adresse (« A1 ») et le texte
// d'origine, et rend le remplacement. Les chaînes restent telles quelles.
func cellRefs(text string, fn func(sheet, addr, orig string) string) string {
	return cellRefRe.ReplaceAllStringFunc(text, func(m string) string {
		g := cellRefRe.FindStringSubmatch(m)
		switch {
		case g[4] != "" || g[5] != "":
			sheet := g[3]
			if g[1] != "" {
				sheet = strings.ReplaceAll(g[1], "''", "'")
			}
			if g[2] != "" {
				sheet = strings.ReplaceAll(g[2], `""`, `"`)
			}
			return fn(sheet, g[4]+g[5], m)
		case g[6] != "":
			return fn("", g[6], m)
		case g[7] != "":
			return fn("", g[7], m)
		}
		return m // chaîne ou identifiant entre guillemets
	})
}

// mentionsCell indique si un texte peut citer une adresse de cellule.
func mentionsCell(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "cell_") || strings.Contains(l, "cell[")
}

// sheetIDs donne l'identifiant de chaque feuille, par nom en minuscules.
func (t *tx) sheetIDs() (map[string]int64, error) {
	out := map[string]int64{}
	rs, err := t.query(`SELECT id, name FROM _slitable_sheets`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		var id int64
		var name string
		if err := rs.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[strings.ToLower(name)] = id
	}
	return out, rs.Err()
}

// CellSQL remplace les adresses de cellules d'une requête de l'onglet SQL
// par leurs valeurs, en littéral SQL, avant l'exécution. Une adresse sans
// feuille se lit sur la feuille sheet (la feuille affichée).
func (m *Model) CellSQL(ctx context.Context, q string, sheet int64) (string, error) {
	if !mentionsCell(q) {
		return q, nil
	}
	var out string
	err := m.read(ctx, func(t *tx) error {
		sheets, err := t.sheetIDs()
		if err != nil {
			return err
		}
		var values map[string]formula.Value
		var failed error
		out = cellRefs(q, func(name, addr, orig string) string {
			on := sheet
			if name != "" {
				id, ok := sheets[strings.ToLower(name)]
				if !ok {
					return orig // Tableau.Colonne nommée Cell_… : laissé tel quel
				}
				on = id
			}
			if failed != nil {
				return orig
			}
			if values == nil {
				schema, err := t.schema()
				if err == nil {
					values, err = t.cellValues(schema)
				}
				if err != nil {
					failed = err
					return orig
				}
			}
			r, c, _ := formula.CellRef("cell_" + addr)
			v := values[formula.CellKey(on, r, c)]
			if v.IsErr() {
				failed = fmt.Errorf("%s vaut %s", formula.CellText(formula.CellName(r, c)), v.String())
				return orig
			}
			return sqlCellValue(v)
		})
		return failed
	})
	return out, err
}

// cellValues : nameValues avec toutes les cellules libres, même si aucune
// formule enregistrée ne cite d'adresse.
func (t *tx) cellValues(schema *formula.Schema) (map[string]formula.Value, error) {
	return cached(t, "names+cells", func() (map[string]formula.Value, error) { return t.nameValuesWith(schema, true) })
}

func sqlCellValue(v formula.Value) string {
	switch v.Kind {
	case formula.Empty:
		return "NULL"
	case formula.Bool:
		if v.Bool {
			return "1"
		}
		return "0"
	case formula.Text:
		return sqlLiteral(v.Str)
	}
	if v.Kind == formula.Number {
		return strconv.FormatFloat(v.Num, 'g', -1, 64)
	}
	return sqlLiteral(v.String())
}
