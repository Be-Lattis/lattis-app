package canvas

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/lalamefine/lattis/internal/grid"
)

// MaxMatches borne une recherche.
const MaxMatches = 5000

// Match est une cellule trouvée (coordonnées de la feuille).
type Match struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// likeEscape protège % et _ pour LIKE … ESCAPE '\'.
func likeEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(s) + "%"
}

// Find cherche un texte dans une feuille, sans tenir compte de la casse,
// comme Excel : en-têtes, cellules des tableaux (telles que saisies) et
// cellules libres, ligne par ligne. Les colonnes calculées ne sont pas
// parcourues.
func (m *Model) Find(ctx context.Context, sheet int64, text string) ([]Match, error) {
	out := []Match{}
	if text == "" {
		return out, nil
	}
	pattern := likeEscape(text)
	err := m.read(ctx, func(t *tx) error {
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		for _, tb := range tables {
			// Un seul parcours du tableau : une condition par colonne saisie
			// (le texte directement, les autres types par leur texte affiché).
			var conds, sel []string
			var args []any
			var idx []int
			for i, c := range tb.Cols {
				if strings.Contains(strings.ToLower(c.Name), strings.ToLower(text)) {
					out = append(out, Match{tb.Row, tb.Col + i})
				}
				if c.Formula != "" || !canHold(c.Type, text) {
					continue
				}
				col := colSQL(c.ID)
				sel = append(sel, col)
				switch c.Type {
				case grid.Text:
					conds = append(conds, col+` LIKE ? ESCAPE '\'`)
					args = append(args, pattern)
				case grid.Integer:
					// Entier : son texte est celui de SQLite (sans passer par Go).
					conds = append(conds, `CAST(`+col+` AS TEXT) LIKE ? ESCAPE '\'`)
					args = append(args, pattern)
				default:
					conds = append(conds, `slt_text(`+col+`, ?) LIKE ? ESCAPE '\'`)
					args = append(args, string(c.Type), pattern)
				}
				idx = append(idx, i)
			}
			if len(conds) == 0 {
				continue
			}
			// Rien de trouvé (cas le plus long : tout est parcouru) : pas de
			// numérotation des lignes, qui coûte un tri de tout le tableau.
			var found int
			if err := t.row(`SELECT EXISTS (SELECT 1 FROM `+tb.view()+` WHERE `+strings.Join(conds, " OR ")+`)`, args...).Scan(&found); err != nil {
				return err
			}
			if found == 0 {
				continue
			}
			q := `SELECT n, ` + strings.Join(conds, ", ") + ` FROM (SELECT row_number() OVER (ORDER BY _pos) AS n, ` +
				strings.Join(sel, ", ") + ` FROM ` + tb.view() + `) WHERE ` + strings.Join(conds, " OR ") + ` LIMIT ?`
			all := append(append(append([]any{}, args...), args...), MaxMatches)
			rs, err := t.query(q, all...)
			if err != nil {
				return err
			}
			for rs.Next() {
				vals := make([]any, len(conds)+1)
				ptrs := make([]any, len(vals))
				for k := range vals {
					ptrs[k] = &vals[k]
				}
				if err := rs.Scan(ptrs...); err != nil {
					rs.Close()
					return err
				}
				n := int(vals[0].(int64))
				for k, v := range vals[1:] {
					if b, ok := v.(int64); ok && b != 0 {
						out = append(out, Match{tb.Row + n, tb.Col + idx[k]})
					}
				}
			}
			rs.Close()
		}
		rs, err := t.query(`SELECT row, col FROM _lattis_cells WHERE sheet = ? AND raw LIKE ? ESCAPE '\' LIMIT ?`, sheet, pattern, MaxMatches)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var mt Match
			if err := rs.Scan(&mt.Row, &mt.Col); err != nil {
				return err
			}
			out = append(out, mt)
		}
		return rs.Err()
	})
	// Ordre de lecture : ligne puis colonne.
	sortMatches(out)
	if len(out) > MaxMatches {
		out = out[:MaxMatches]
	}
	return out, err
}

// canHold : le texte affiché d'une valeur de ce type peut-il contenir s ?
// Un entier ne s'écrit qu'avec des chiffres et « - », un décimal aussi
// avec « , », une date avec « / », un booléen VRAI ou FAUX : chercher des
// lettres n'y lit pas une seule valeur.
func canHold(t grid.Type, s string) bool {
	only := func(set string) bool {
		for _, r := range s {
			if !strings.ContainsRune(set, r) {
				return false
			}
		}
		return true
	}
	switch t {
	case grid.Integer:
		return only("-0123456789")
	case grid.Decimal:
		return only("-0123456789,")
	case grid.Date:
		return only("0123456789/")
	case grid.Boolean:
		u := strings.ToUpper(s)
		return strings.Contains("VRAI", u) || strings.Contains("FAUX", u)
	}
	return true
}

func sortMatches(ms []Match) {
	sort.Slice(ms, func(i, j int) bool {
		if ms[i].Row != ms[j].Row {
			return ms[i].Row < ms[j].Row
		}
		return ms[i].Col < ms[j].Col
	})
}

// Replace remplace un texte dans toutes les cellules trouvées (en-têtes
// compris, sans tenir compte de la casse) : chaque cellule est ressaisie
// comme au clavier (types, élargissement). Une seule étape d'annulation ;
// renvoie le nombre de cellules modifiées.
func (m *Model) Replace(ctx context.Context, sheet int64, text, by string) (int, error) {
	matches, err := m.Find(ctx, sheet, text)
	if err != nil || len(matches) == 0 {
		return 0, err
	}
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(text))
	n := 0
	err = m.Do(ctx, "Remplacer", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		for _, mt := range matches {
			raw, ok, err := t.rawAt(sheet, mt.Row, mt.Col)
			if err != nil {
				return err
			}
			if !ok || isFormula(raw) {
				continue
			}
			neu := re.ReplaceAllLiteralString(raw, by)
			if neu == raw {
				continue
			}
			var ev Event
			if err := t.input(sheet, mt.Row, mt.Col, neu, &ev); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

// rawAt donne la saisie d'une cellule de la feuille (texte tel que tapé).
func (t *tx) rawAt(sheet int64, row, col int) (string, bool, error) {
	tables, err := t.tables(sheet)
	if err != nil {
		return "", false, err
	}
	for _, tb := range tables {
		if col < tb.Col || col > tb.Right() || row < tb.Row || row > tb.Bottom() {
			continue
		}
		c := tb.Cols[col-tb.Col]
		if row == tb.Row {
			return c.Name, true, nil
		}
		var s *string
		err := t.row(`SELECT slt_text(`+colSQL(c.ID)+`, ?) FROM `+tb.view()+` ORDER BY _pos LIMIT 1 OFFSET ?`, string(c.Type), row-tb.Row-1).Scan(&s)
		if err != nil || s == nil {
			return "", false, err
		}
		return *s, true, nil
	}
	var raw string
	err = t.row(`SELECT raw FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, row, col).Scan(&raw)
	if err != nil {
		return "", false, nil
	}
	return raw, true, nil
}
