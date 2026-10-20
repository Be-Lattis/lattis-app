package canvas

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/sqltext"
)

// Colonnes calculées en SQL (onglet SQL, db.query, scripts).
//
// Une colonne calculée n'est pas stockée : sa colonne c_<id> de la table
// t_<id> reste vide, la grille et les graphiques lisent l'expression
// compilée de sa formule (columnSQL). Pour qu'une requête de lecture voie
// ses valeurs, chaque tableau cité qui a une colonne calculée devient une
// expression de table du même nom, en tête de la clause WITH :
//
//	WITH t_3 AS (SELECT id, _pos, c_1, c_2, <formule de c_3> AS c_3 FROM main.t_3 AS t_3) ...
//
// Les colonnes gardent leurs noms et leur ordre : la requête traduite par
// l'onglet SQL (t_3, c_3) n'a pas à changer. Dans les expressions, les
// tableaux sont lus dans main (main.t_3) : une formule qui parcourt son
// propre tableau ou un autre ne lit pas une expression de table (ce qui
// serait circulaire). Les paramètres des formules (?1, ?2…) deviennent des
// littéraux, la requête de l'utilisateur gardant les siens.

// ReadCTE prépare une requête de lecture (onglet SQL, db.query, requête
// liée) : tableaux croisés cités définis (PivotCTE), colonnes calculées des
// tableaux cités avec leurs valeurs (ComputedCTE).
func (m *Model) ReadCTE(ctx context.Context, q string) (string, error) {
	defs, err := m.ComputedDefs(ctx, q)
	if err != nil {
		return q, err
	}
	if q, err = m.PivotCTE(ctx, q); err != nil {
		return q, err
	}
	return sqltext.PrependWith(q, defs), nil
}

// ComputedCTE ajoute à une requête de lecture les expressions de table des
// tableaux cités qui ont des colonnes calculées (voir ComputedDefs).
func (m *Model) ComputedCTE(ctx context.Context, q string) (string, error) {
	defs, err := m.ComputedDefs(ctx, q)
	if err != nil {
		return q, err
	}
	return sqltext.PrependWith(q, defs), nil
}

// ComputedDefs donne les expressions de table (« t_3 AS (SELECT …) ») des
// tableaux cités par q, par leur nom interne (t_3) ou affiché (Commandes),
// qui ont au moins une colonne calculée.
func (m *Model) ComputedDefs(ctx context.Context, q string) ([]string, error) {
	var defs []string
	err := m.read(ctx, func(t *tx) error {
		used := map[string]bool{}
		for _, w := range sqlWord.FindAllString(q, -1) {
			if strings.HasPrefix(w, `"`) {
				w = strings.ReplaceAll(strings.Trim(w, `"`), `""`, `"`)
			}
			used[strings.ToLower(w)] = true
		}
		rs, err := t.query(`SELECT DISTINCT t.id, t.name FROM _lattis_tables t JOIN _lattis_columns c ON c.tbl = t.id WHERE coalesce(c.formula, '') <> '' ORDER BY t.id`)
		if err != nil {
			return err
		}
		var ids []int64
		for rs.Next() {
			var id int64
			var name string
			if err := rs.Scan(&id, &name); err != nil {
				rs.Close()
				return err
			}
			if used[strings.ToLower(name)] || used[fmt.Sprintf("t_%d", id)] {
				ids = append(ids, id)
			}
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return err
		}
		for _, id := range ids {
			tb, err := t.table(id)
			if err != nil {
				return err
			}
			src, ok, err := t.computedSource(tb)
			if err != nil {
				return err
			}
			if ok {
				defs = append(defs, tb.sql()+" AS ("+src+")")
			}
		}
		return nil
	})
	return defs, err
}

// computedSource donne une requête autonome qui rend la table du tableau
// (id, _pos, c_…, dans l'ordre du schéma) avec les valeurs de ses colonnes
// calculées ; faux s'il n'en a pas.
func (t *tx) computedSource(tb Table) (string, bool, error) {
	exprs, args, computed, err := t.columnSQL(tb)
	if err != nil {
		return "", false, err
	}
	has := false
	byName := map[string]int{}
	for i, c := range tb.Cols {
		byName[colSQL(c.ID)] = i
		has = has || computed[i]
	}
	if !has {
		return "", false, nil
	}
	rs, err := t.query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, tb.sql())
	if err != nil {
		return "", false, err
	}
	var phys []string
	for rs.Next() {
		var n string
		if err := rs.Scan(&n); err != nil {
			rs.Close()
			return "", false, err
		}
		phys = append(phys, n)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return "", false, err
	}
	expr := func(i int) (string, error) { return inlineSQL(exprs[i], args) }
	var sel []string
	seen := map[string]bool{}
	for _, n := range phys {
		seen[n] = true
		i, ok := byName[n]
		if !ok || !computed[i] {
			sel = append(sel, n)
			continue
		}
		e, err := expr(i)
		if err != nil {
			return "", false, err
		}
		sel = append(sel, e+" AS "+n)
	}
	// Colonne calculée sans colonne stockée : en fin de liste.
	for i, c := range tb.Cols {
		if computed[i] && !seen[colSQL(c.ID)] {
			e, err := expr(i)
			if err != nil {
				return "", false, err
			}
			sel = append(sel, e+" AS "+colSQL(c.ID))
		}
	}
	return "SELECT " + strings.Join(sel, ", ") + " FROM main." + tb.sql() + " AS " + tb.sql(), true, nil
}

var internalTable = regexp.MustCompile(`^t_[0-9]+$`)

// inlineSQL prépare une expression compilée pour une requête autonome :
// paramètres ?N remplacés par leur valeur en littéral, tables t_<id> lues
// dans main (main.t_3). Les chaînes et identifiants entre guillemets restent
// tels quels.
func inlineSQL(s string, args []any) (string, error) { return inlineExpr(s, args, true) }

// inlineExpr : paramètres ?N remplacés par leur valeur en littéral, et
// tables t_<id> lues dans main si inMain.
func inlineExpr(s string, args []any, inMain bool) (string, error) {
	var b strings.Builder
	prev := "" // mot précédent, en minuscules
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\'' || c == '"':
			j := i + 1
			for j < len(s) {
				if s[j] == c {
					if j+1 < len(s) && s[j+1] == c {
						j += 2
						continue
					}
					break
				}
				j++
			}
			j = min(j+1, len(s))
			b.WriteString(s[i:j])
			i, prev = j, ""
		case c == '?':
			j := i + 1
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			n, err := strconv.Atoi(s[i+1 : j])
			if err != nil || n < 1 || n > len(args) {
				return "", msg.New("computed.paramNotFound", msg.Args{"param": fmt.Sprintf("%q", s[i:j])})
			}
			b.WriteString(sqlArgLiteral(args[n-1]))
			i, prev = j, ""
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			j := i + 1
			for j < len(s) && (s[j] == '_' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= '0' && s[j] <= '9') {
				j++
			}
			w := s[i:j]
			qualified := i > 0 && s[i-1] == '.'
			if inMain && internalTable.MatchString(w) && !qualified && prev != "as" {
				b.WriteString("main.")
			}
			b.WriteString(w)
			i, prev = j, strings.ToLower(w)
		case c >= '0' && c <= '9':
			// Nombre : ses chiffres et lettres (1e5, 0x1F) ne sont pas un mot.
			j := i + 1
			for j < len(s) && (s[j] == '.' || s[j] == '_' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= '0' && s[j] <= '9') {
				j++
			}
			b.WriteString(s[i:j])
			i, prev = j, ""
		default:
			b.WriteByte(c)
			if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
				prev = ""
			}
			i++
		}
	}
	return b.String(), nil
}

// sqlArgLiteral écrit un paramètre de formule compilée en littéral SQL, du
// même type que la valeur liée (un réel reste réel : 3.0, pas 3).
func sqlArgLiteral(v any) string {
	switch v := v.(type) {
	case nil:
		return "NULL"
	case bool:
		if v {
			return "1"
		}
		return "0"
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		switch {
		case math.IsNaN(v):
			return "NULL"
		case math.IsInf(v, 1):
			return "1e999"
		case math.IsInf(v, -1):
			return "-1e999"
		}
		s := strconv.FormatFloat(v, 'g', -1, 64)
		if !strings.ContainsAny(s, ".e") {
			s += ".0"
		}
		if v < 0 {
			return "(" + s + ")"
		}
		return s
	case string:
		return sqlLiteral(v)
	case []byte:
		return "X'" + hex.EncodeToString(v) + "'"
	}
	return sqlLiteral(fmt.Sprint(v))
}
