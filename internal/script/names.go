package script

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
)

// Les tableaux sont stockés dans des tables t_<id> aux colonnes c_<id>
// (internal/meta). Pour qu'un script écrive ses requêtes avec les noms
// affichés, chaque requête de lecture est préfixée d'une clause WITH qui
// déclare une expression de table par tableau :
//
//	WITH "Commandes"("_id", "Client", "Montant") AS
//	     (SELECT id, c_1, c_2 FROM t_3 ORDER BY _pos) ...
//
// Rien n'est créé dans la base (pas de vue temporaire), ce qui marche aussi
// sur les connexions en lecture seule ; une expression de table que la
// requête n'utilise pas ne coûte rien.

// Column est une colonne d'un tableau, telle que l'utilisateur la voit.
type Column struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Fixed bool   `json:"fixed"`
	sql   string
}

// Table est un tableau du classeur.
type Table struct {
	Name    string   `json:"name"`
	Sheet   string   `json:"sheet"`
	Columns []Column `json:"columns"`
	sql     string
}

// catalog liste les tableaux du classeur et prépare la clause WITH.
type catalog struct {
	tables []Table
	with   string          // "name"(cols) AS (...), ... ; vide sans tableau
	bools  map[string]bool // noms de colonnes booléennes
}

type querier interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}

func loadCatalog(ctx context.Context, db querier) (*catalog, error) {
	rs, err := db.QueryContext(ctx, `
		SELECT t.id, t.name, s.name, c.id, c.name, c.type, c.fixed
		FROM _slitable_tables t
		JOIN _slitable_sheets s ON s.id = t.sheet
		JOIN _slitable_columns c ON c.tbl = t.id
		ORDER BY s.pos, t.id, c.pos`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	cat := &catalog{bools: map[string]bool{}}
	last := int64(-1)
	for rs.Next() {
		var tid, cid int64
		var tname, sheet string
		var c Column
		if err := rs.Scan(&tid, &tname, &sheet, &cid, &c.Name, &c.Type, &c.Fixed); err != nil {
			return nil, err
		}
		if tid != last {
			cat.tables = append(cat.tables, Table{Name: tname, Sheet: sheet, sql: "t_" + itoa(tid)})
			last = tid
		}
		c.sql = "c_" + itoa(cid)
		tb := &cat.tables[len(cat.tables)-1]
		tb.Columns = append(tb.Columns, c)
		if c.Type == "boolean" {
			cat.bools[c.Name] = true
		}
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	cat.with = withClause(cat.tables)
	return cat, nil
}

func withClause(tables []Table) string {
	var b strings.Builder
	seen := map[string]bool{}
	for _, t := range tables {
		// SQLite compare les noms sans tenir compte de la casse.
		key := strings.ToLower(t.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		if b.Len() > 0 {
			b.WriteString(", ")
		}
		names := []string{quote("_id")}
		exprs := []string{"id"}
		used := map[string]bool{"_id": true}
		for _, c := range t.Columns {
			n := c.Name
			for k := 2; used[strings.ToLower(n)]; k++ {
				n = c.Name + " (" + itoa(int64(k)) + ")"
			}
			used[strings.ToLower(n)] = true
			names = append(names, quote(n))
			exprs = append(exprs, c.sql)
		}
		b.WriteString(quote(t.Name) + "(" + strings.Join(names, ", ") + ") AS (SELECT " +
			strings.Join(exprs, ", ") + " FROM " + t.sql + " ORDER BY _pos)")
	}
	return b.String()
}

// rewrite préfixe une requête de lecture de la clause WITH du catalogue.
// Une requête qui n'est ni SELECT, ni VALUES, ni WITH est laissée telle quelle.
func (c *catalog) rewrite(q string) string {
	if c.with == "" {
		return q
	}
	start := skipBlank(q, 0)
	kw, end := word(q, start)
	switch kw {
	case "select", "values":
		return "WITH " + c.with + " " + q[start:]
	case "with":
		rest := skipBlank(q, end)
		head := "WITH "
		if kw2, end2 := word(q, rest); kw2 == "recursive" {
			head, rest = "WITH RECURSIVE ", skipBlank(q, end2)
		}
		return head + c.with + ", " + q[rest:]
	}
	return q
}

// skipBlank saute les espaces et les commentaires à partir de i.
func skipBlank(q string, i int) int {
	for i < len(q) {
		switch {
		case q[i] == ' ' || q[i] == '\t' || q[i] == '\n' || q[i] == '\r':
			i++
		case strings.HasPrefix(q[i:], "--"):
			if j := strings.IndexByte(q[i:], '\n'); j >= 0 {
				i += j + 1
			} else {
				return len(q)
			}
		case strings.HasPrefix(q[i:], "/*"):
			if j := strings.Index(q[i+2:], "*/"); j >= 0 {
				i += j + 4
			} else {
				return len(q)
			}
		default:
			return i
		}
	}
	return i
}

// word lit un mot-clé (lettres ASCII) à partir de i, en minuscules.
func word(q string, i int) (string, int) {
	j := i
	for j < len(q) && (q[j] >= 'a' && q[j] <= 'z' || q[j] >= 'A' && q[j] <= 'Z') {
		j++
	}
	return strings.ToLower(q[i:j]), j
}

func quote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
