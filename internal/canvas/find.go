package canvas

import (
	"context"
	"regexp"
	"sort"
	"strings"
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
			for i, c := range tb.Cols {
				if strings.Contains(strings.ToLower(c.Name), strings.ToLower(text)) {
					out = append(out, Match{tb.Row, tb.Col + i})
				}
				if c.Formula != "" {
					continue
				}
				col := colSQL(c.ID)
				rs, err := t.query(`SELECT n FROM (SELECT `+col+` AS v, row_number() OVER (ORDER BY _pos) AS n FROM `+tb.sql()+
					`) WHERE slt_text(v, ?) LIKE ? ESCAPE '\' LIMIT ?`, string(c.Type), pattern, MaxMatches)
				if err != nil {
					return err
				}
				for rs.Next() {
					var n int
					if err := rs.Scan(&n); err != nil {
						rs.Close()
						return err
					}
					out = append(out, Match{tb.Row + n, tb.Col + i})
				}
				rs.Close()
			}
		}
		rs, err := t.query(`SELECT row, col FROM _slitable_cells WHERE sheet = ? AND raw LIKE ? ESCAPE '\' LIMIT ?`, sheet, pattern, MaxMatches)
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
		err := t.row(`SELECT slt_text(`+colSQL(c.ID)+`, ?) FROM `+tb.sql()+` ORDER BY _pos LIMIT 1 OFFSET ?`, string(c.Type), row-tb.Row-1).Scan(&s)
		if err != nil || s == nil {
			return "", false, err
		}
		return *s, true, nil
	}
	var raw string
	err = t.row(`SELECT raw FROM _slitable_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, row, col).Scan(&raw)
	if err != nil {
		return "", false, nil
	}
	return raw, true, nil
}
