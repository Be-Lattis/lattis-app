package canvas

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Un tableau croisé porte un nom (« TCD1 »…) : il s'interroge dans l'onglet
// SQL comme une vue, et sert de source à un graphique.

// pivotRow est un tableau croisé lu en base, toutes feuilles.
type pivotRow struct {
	ID    int64
	Sheet int64
	Cfg   PivotConfig
}

func (t *tx) allPivots() ([]pivotRow, error) {
	rs, err := t.query(`SELECT id, sheet, config FROM _slitable_pivots ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []pivotRow
	for rs.Next() {
		var p pivotRow
		var cfg string
		if err := rs.Scan(&p.ID, &p.Sheet, &cfg); err != nil {
			return nil, err
		}
		if json.Unmarshal([]byte(cfg), &p.Cfg) != nil {
			continue
		}
		p.Cfg = p.Cfg.normalized()
		if p.Cfg.Name == "" {
			p.Cfg.Name = fmt.Sprintf("TCD%d", p.ID)
		}
		out = append(out, p)
	}
	return out, rs.Err()
}

func (t *tx) pivotByName(name string) (PivotConfig, bool, error) {
	ps, err := t.allPivots()
	if err != nil {
		return PivotConfig{}, false, err
	}
	for _, p := range ps {
		if strings.EqualFold(p.Cfg.Name, name) {
			return p.Cfg, true, nil
		}
	}
	return PivotConfig{}, false, nil
}

// pivotName choisit un nom libre « TCDn » (ni tableau, ni autre TCD).
func (t *tx) pivotName() (string, error) {
	ps, err := t.allPivots()
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for _, p := range ps {
		taken[strings.ToLower(p.Cfg.Name)] = true
	}
	for n := 1; ; n++ {
		name := fmt.Sprintf("TCD%d", n)
		var k int
		if err := t.row(`SELECT count(*) FROM _slitable_tables WHERE name = ? COLLATE NOCASE`, name).Scan(&k); err != nil {
			return "", err
		}
		if k == 0 && !taken[strings.ToLower(name)] {
			return name, nil
		}
	}
}

// pivotQuery donne une requête SQL autonome (valeurs en littéraux) qui rend
// les lignes du tableau croisé, sans ligne de total : une colonne par champ
// de lignes, puis une par mesure, nommées comme les en-têtes.
func (t *tx) pivotQuery(raw PivotConfig) (string, []string, error) {
	cfg := raw.normalized()
	cfg.NoGrandTotal = true
	cells, err := t.pivotCells(cfg)
	if err != nil {
		return "", nil, err
	}
	header := cells[0]
	// Les valeurs déjà calculées deviennent des lignes VALUES : la vue reflète
	// exactement le TCD affiché (pourcentages compris).
	quote := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
	cols := make([]string, len(header))
	seen := map[string]int{}
	for i, h := range header {
		if h == "" {
			h = "Groupe"
		}
		if n := seen[strings.ToLower(h)]; n > 0 {
			h = fmt.Sprintf("%s %d", h, n+1)
		}
		seen[strings.ToLower(h)]++
		cols[i] = h
	}
	if len(cells) == 1 {
		sel := make([]string, len(cols))
		for i, c := range cols {
			sel[i] = "NULL AS " + quote(c)
		}
		return "SELECT " + strings.Join(sel, ", ") + " WHERE 0", cols, nil
	}
	var rows []string
	for _, r := range cells[1:] {
		vals := make([]string, len(r))
		for i, v := range r {
			vals[i] = sqlValue(v)
		}
		rows = append(rows, "("+strings.Join(vals, ", ")+")")
	}
	sel := make([]string, len(cols))
	for i, c := range cols {
		sel[i] = "column" + strconv.Itoa(i+1) + " AS " + quote(c)
	}
	return "SELECT " + strings.Join(sel, ", ") + " FROM (VALUES " + strings.Join(rows, ", ") + ")", cols, nil
}

// sqlValue écrit une cellule de TCD en littéral : nombre si c'en est un
// (écriture française, pourcentage compris), texte sinon.
func sqlValue(s string) string {
	if f, ok := parseShown(s); ok {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	if s == "" {
		return "NULL"
	}
	return sqlLiteral(s)
}

// parseShown relit un nombre affiché (« 1 234,5 », « 12,5 % »).
func parseShown(s string) (float64, bool) {
	t := strings.NewReplacer(" ", "", " ", "", " ", "").Replace(s)
	pct := strings.HasSuffix(t, "%")
	t = strings.TrimSuffix(t, "%")
	t = strings.Replace(t, ",", ".", 1)
	f, err := strconv.ParseFloat(t, 64)
	if err != nil || t == "" {
		return 0, false
	}
	if pct {
		f /= 100
	}
	return f, true
}

var sqlWord = regexp.MustCompile(`"(?:[^"]|"")*"|[\p{L}_][\p{L}\p{N}_]*`)

// PivotCTE ajoute aux requêtes de l'onglet SQL qui citent un tableau croisé
// une clause WITH qui le définit : il s'interroge comme une vue.
func (m *Model) PivotCTE(ctx context.Context, q string) (string, error) {
	out := q
	err := m.read(ctx, func(t *tx) error {
		ps, err := t.allPivots()
		if err != nil || len(ps) == 0 {
			return err
		}
		used := map[string]bool{}
		for _, w := range sqlWord.FindAllString(q, -1) {
			if strings.HasPrefix(w, `"`) {
				w = strings.ReplaceAll(strings.Trim(w, `"`), `""`, `"`)
			}
			used[strings.ToLower(w)] = true
		}
		var defs []string
		for _, p := range ps {
			if !used[strings.ToLower(p.Cfg.Name)] {
				continue
			}
			sql, _, err := t.pivotQuery(p.Cfg)
			if err != nil {
				return err
			}
			defs = append(defs, `"`+p.Cfg.Name+`" AS (`+sql+`)`)
		}
		if len(defs) == 0 {
			return nil
		}
		trimmed := strings.TrimLeft(q, " \t\r\n")
		if len(trimmed) >= 4 && strings.EqualFold(trimmed[:4], "WITH") {
			out = "WITH " + strings.Join(defs, ", ") + ", " + strings.TrimLeft(trimmed[4:], " \t\r\n")
		} else {
			out = "WITH " + strings.Join(defs, ", ") + " " + q
		}
		return nil
	})
	return out, err
}
