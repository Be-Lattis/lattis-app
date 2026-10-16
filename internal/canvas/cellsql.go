package canvas

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/lalamefine/slitable/internal/formula"
)

// Dans l'onglet SQL, Cell_A1 (cellule libre de la feuille active) et
// Feuille2.Cell_B3 ou "Ma feuille".Cell_B3 sont remplacés par la valeur de
// la cellule, en littéral SQL, avant l'exécution. Les chaînes ('…') et les
// identifiants entre guillemets ne sont pas touchés.
var cellSQLRe = regexp.MustCompile(`(?i)(?:"((?:[^"]|"")*)"|([\p{L}_][\p{L}\p{N}_]*))\.(cell_[a-z]{1,3}[1-9][0-9]*)\b|'(?:[^']|'')*'|"(?:[^"]|"")*"|\b(cell_[a-z]{1,3}[1-9][0-9]*)\b`)

// CellSQL remplace les adresses de cellules d'une requête par leurs valeurs.
func (m *Model) CellSQL(ctx context.Context, q string, sheet int64) (string, error) {
	if !strings.Contains(strings.ToLower(q), "cell_") {
		return q, nil
	}
	var out string
	err := m.read(ctx, func(t *tx) error {
		sheets := map[string]int64{}
		rs, err := t.query(`SELECT id, name FROM _slitable_sheets`)
		if err != nil {
			return err
		}
		for rs.Next() {
			var id int64
			var name string
			if err := rs.Scan(&id, &name); err != nil {
				rs.Close()
				return err
			}
			sheets[strings.ToLower(name)] = id
		}
		rs.Close()
		var values map[string]formula.Value
		var failed error
		out = cellSQLRe.ReplaceAllStringFunc(q, func(m string) string {
			sub := cellSQLRe.FindStringSubmatch(m)
			addr, on := sub[4], sheet
			switch {
			case sub[3] != "":
				addr = sub[3]
				name := sub[2]
				if sub[1] != "" {
					name = strings.ReplaceAll(sub[1], `""`, `"`)
				}
				id, ok := sheets[strings.ToLower(name)]
				if !ok {
					return m // Tableau.Colonne nommée Cell_… : laissé tel quel
				}
				on = id
			case addr == "":
				return m // chaîne ou identifiant entre guillemets
			}
			if failed != nil {
				return m
			}
			if values == nil {
				schema, err := t.schema()
				if err == nil {
					values, err = t.cellValues(schema)
				}
				if err != nil {
					failed = err
					return m
				}
			}
			r, c, _ := formula.CellRef(addr)
			v := values[formula.CellKey(on, r, c)]
			if v.IsErr() {
				failed = fmt.Errorf("%s vaut %s", formula.CellName(r, c), v.String())
				return m
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
