package canvas

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/lalamefine/slitable/internal/grid"
)

// Dump donne un texte déterministe de tout l'état du document : schéma SQL,
// métadonnées, contenu des tableaux (dans l'ordre d'affichage, avec le type
// SQLite de chaque valeur), cellules libres et saisies invalides. Deux dumps
// égaux = deux documents identiques.
func (m *Model) Dump(ctx context.Context) (string, error) {
	db := m.db.Read()
	var b strings.Builder
	dumpQuery := func(title, q string) error {
		fmt.Fprintf(&b, "## %s\n", title)
		rs, err := db.QueryContext(ctx, q)
		if err != nil {
			return fmt.Errorf("%s : %w", q, err)
		}
		defer rs.Close()
		cols, _ := rs.Columns()
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		for rs.Next() {
			if err := rs.Scan(ptrs...); err != nil {
				return err
			}
			for i, v := range normalize(vals) {
				if i > 0 {
					b.WriteString(" | ")
				}
				fmt.Fprintf(&b, "%v", v)
			}
			b.WriteByte('\n')
		}
		return rs.Err()
	}
	steps := []struct{ title, q string }{
		{"schéma", `SELECT type, name, CASE WHEN type = 'table' AND name GLOB 't_[0-9]*' THEN NULL ELSE replace(sql, '"', '') END FROM sqlite_schema WHERE name NOT LIKE 'sqlite_%' ORDER BY name`},
		{"feuilles", `SELECT * FROM _slitable_sheets ORDER BY id`},
		{"tableaux", `SELECT * FROM _slitable_tables ORDER BY id`},
		{"colonnes", `SELECT * FROM _slitable_columns ORDER BY tbl, pos`},
		{"cellules", `SELECT * FROM _slitable_cells ORDER BY sheet, row, col`},
		{"invalides", `SELECT * FROM _slitable_invalid ORDER BY tbl, rid, col`},
		{"styles", `SELECT * FROM _slitable_styles ORDER BY tbl, rid, col`},
		{"styles libres", `SELECT * FROM _slitable_cell_styles ORDER BY sheet, row, col`},
		{"styles de lignes", `SELECT * FROM _slitable_row_styles ORDER BY sheet, row`},
		{"styles de colonnes", `SELECT * FROM _slitable_col_styles ORDER BY sheet, col`},
		{"noms", `SELECT * FROM _slitable_names ORDER BY name`},
		{"largeurs", `SELECT * FROM _slitable_col_widths ORDER BY sheet, col`},
		{"hauteurs", `SELECT * FROM _slitable_row_heights ORDER BY sheet, row`},
		{"scripts", `SELECT * FROM _slitable_scripts ORDER BY id`},
		{"tableaux croisés", `SELECT * FROM _slitable_pivots ORDER BY id`},
		{"graphiques", `SELECT * FROM _slitable_charts ORDER BY id`},
		{"boutons", `SELECT * FROM _slitable_buttons ORDER BY id`},
		{"favoris", `SELECT * FROM _slitable_favorites ORDER BY id`},
	}
	for _, s := range steps {
		if err := dumpQuery(s.title, s.q); err != nil {
			return "", err
		}
	}
	tables, err := m.tableColumns(ctx)
	if err != nil {
		return "", err
	}
	for _, tc := range tables {
		sel := []string{"id", "_pos"}
		for _, c := range tc.cols {
			sel = append(sel, fmt.Sprintf("typeof(%s) || ':' || quote(%s)", colSQL(c), colSQL(c)))
		}
		// Colonnes SQL par nom : leur ordre physique ne fait pas partie du modèle
		// (une colonne supprimée puis restaurée revient à la fin).
		if err := dumpQuery(tableSQL(tc.id)+" colonnes", "SELECT name, type, \"notnull\", pk FROM pragma_table_info('"+tableSQL(tc.id)+"') ORDER BY name"); err != nil {
			return "", err
		}
		if err := dumpQuery(tableSQL(tc.id), `SELECT `+strings.Join(sel, ", ")+` FROM `+tableSQL(tc.id)+` ORDER BY _pos`); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

type tableCols struct {
	id   int64
	cols []int64
}

func (m *Model) tableColumns(ctx context.Context) ([]tableCols, error) {
	rs, err := m.db.Read().QueryContext(ctx, `SELECT tbl, id FROM _slitable_columns ORDER BY tbl, pos`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []tableCols
	for rs.Next() {
		var tbl, id int64
		if err := rs.Scan(&tbl, &id); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].id != tbl {
			out = append(out, tableCols{id: tbl})
		}
		out[len(out)-1].cols = append(out[len(out)-1].cols, id)
	}
	return out, rs.Err()
}

var isoDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// Check vérifie les invariants du modèle et renvoie la liste des écarts.
func (m *Model) Check(ctx context.Context) ([]string, error) {
	var problems []string
	bad := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	err := m.read(ctx, func(t *tx) error {

		// Objets SQL : exactement les tables déclarées, plus les métadonnées.
		known := map[string]bool{}
		rs, err := t.query(`SELECT id FROM _slitable_sheets`)
		if err != nil {
			return err
		}
		var sheets []int64
		for rs.Next() {
			var id int64
			rs.Scan(&id)
			sheets = append(sheets, id)
		}
		rs.Close()

		var all []Table
		for _, sh := range sheets {
			tables, err := t.tables(sh)
			if err != nil {
				return err
			}
			cells, err := t.cells(sh)
			if err != nil {
				return err
			}
			all = append(all, tables...)
			for i, a := range tables {
				// Pas de chevauchement : un tableau et sa ligne/colonne grisée
				// ne touchent aucun autre objet.
				for _, b := range tables[i+1:] {
					if a.Row <= b.Bottom()+1 && b.Row <= a.Bottom()+1 && a.Col <= b.Right()+1 && b.Col <= a.Right()+1 {
						bad("%s et %s se touchent", a.Name, b.Name)
					}
				}
				for _, c := range cells {
					if c.Row >= a.Row && c.Row <= a.Bottom()+1 && c.Col >= a.Col && c.Col <= a.Right()+1 {
						bad("cellule libre (%d,%d) dans %s ou sa zone grisée", c.Row, c.Col, a.Name)
					}
				}
			}
		}
		for _, tb := range all {
			known[tb.sql()] = true
			known[tb.sql()+"_pos"] = true
			if tb.Key != 0 {
				known[tb.sql()+"_key"] = true
				var k int
				t.row(`SELECT count(*) FROM sqlite_schema WHERE type = 'index' AND name = ?`, tb.sql()+"_key").Scan(&k)
				if k != 1 {
					bad("%s : index de la clé absent", tb.Name)
				}
			}
			// Colonnes SQL = colonnes déclarées, avec le bon type de stockage.
			rs, err := t.query(`SELECT name, type FROM pragma_table_info(?)`, tb.sql())
			if err != nil {
				return err
			}
			got := map[string]string{}
			for rs.Next() {
				var n, typ string
				rs.Scan(&n, &typ)
				got[n] = typ
			}
			rs.Close()
			want := map[string]string{"id": "INTEGER", "_pos": "TEXT"}
			for _, c := range tb.Cols {
				want[colSQL(c.ID)] = sqlType(c.Type)
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				bad("%s : colonnes SQL %v, attendu %v", tb.Name, got, want)
			}
			var idx int
			t.row(`SELECT count(*) FROM sqlite_schema WHERE type = 'index' AND name = ?`, tb.sql()+"_pos").Scan(&idx)
			if idx != 1 {
				bad("%s : index _pos absent", tb.Name)
			}
			// Clés d'ordre valides et uniques.
			var dup, invalidPos int
			t.row(`SELECT count(*) - count(DISTINCT _pos), count(*) FILTER (WHERE _pos = '' OR _pos LIKE '%0') FROM `+tb.sql()).Scan(&dup, &invalidPos)
			if dup > 0 || invalidPos > 0 {
				bad("%s : %d clés d'ordre en double, %d invalides", tb.Name, dup, invalidPos)
			}
			// Valeurs conformes au type logique.
			for _, c := range tb.Cols {
				col := colSQL(c.ID)
				var n int
				var q string
				switch c.Type {
				case grid.Boolean:
					q = col + " NOT IN (0, 1)"
				case grid.Date:
					q = col + " NOT GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]'"
				default:
					continue
				}
				t.row(`SELECT count(*) FROM ` + tb.sql() + ` WHERE ` + col + ` IS NOT NULL AND ` + q).Scan(&n)
				if n > 0 {
					bad("%s.%s : %d valeurs hors type", tb.Name, c.Name, n)
				}
			}
			// Saisies invalides : ligne et colonne existantes, cellule à NULL.
			rs, err = t.query(`SELECT rid, col FROM _slitable_invalid WHERE tbl = ?`, tb.ID)
			if err != nil {
				return err
			}
			var inv [][2]int64
			for rs.Next() {
				var r, c int64
				rs.Scan(&r, &c)
				inv = append(inv, [2]int64{r, c})
			}
			rs.Close()
			for _, p := range inv {
				if !slices.ContainsFunc(tb.Cols, func(c Column) bool { return c.ID == p[1] }) {
					bad("%s : saisie invalide sur une colonne absente", tb.Name)
					continue
				}
				var v any
				if err := t.row(`SELECT `+colSQL(p[1])+` FROM `+tb.sql()+` WHERE id = ?`, p[0]).Scan(&v); err != nil {
					bad("%s : saisie invalide sur une ligne absente", tb.Name)
				} else if v != nil {
					bad("%s : saisie invalide sur une cellule non vide", tb.Name)
				}
			}
		}
		rs, err = t.query(`SELECT name FROM sqlite_schema WHERE name NOT LIKE 'sqlite_%' AND name NOT LIKE '_slitable_%' OR name LIKE '_slitable_trash%'`)
		if err != nil {
			return err
		}
		for rs.Next() {
			var n string
			rs.Scan(&n)
			if !known[n] && !strings.HasPrefix(n, "_slitable_trash") {
				bad("objet SQL orphelin : %s", n)
			}
		}
		rs.Close()
		var orphans int
		t.row(`SELECT count(*) FROM _slitable_columns WHERE tbl NOT IN (SELECT id FROM _slitable_tables)`).Scan(&orphans)
		if orphans > 0 {
			bad("%d colonnes sans tableau", orphans)
		}
		return nil
	})
	return problems, err
}

// View rend une feuille en texte (une ligne par ligne de la feuille, cellules
// séparées par « | »), pour les tests et le débogage.
func (m *Model) View(ctx context.Context, sheet int64) (string, error) {
	var out string
	err := m.read(ctx, func(t *tx) error {
		at := map[[2]int]string{}
		rows, cols := 0, 0
		put := func(r, c int, s string) {
			at[[2]int{r, c}] = s
			rows, cols = max(rows, r+1), max(cols, c+1)
		}
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		for _, tb := range tables {
			names := []string{}
			for i, c := range tb.Cols {
				put(tb.Row, tb.Col+i, "["+c.Name+"]")
				names = append(names, colSQL(c.ID))
			}
			rs, err := t.query(`SELECT ` + strings.Join(names, ", ") + ` FROM ` + tb.sql() + ` ORDER BY _pos`)
			if err != nil {
				return err
			}
			vals := make([]any, len(names))
			ptrs := make([]any, len(names))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			r := tb.Row + 1
			for rs.Next() {
				rs.Scan(ptrs...)
				for i, v := range normalize(vals) {
					s := ""
					if v != nil {
						s = fmt.Sprint(v)
					}
					put(r, tb.Col+i, s)
				}
				r++
			}
			rs.Close()
		}
		cells, err := t.cells(sheet)
		if err != nil {
			return err
		}
		for _, c := range cells {
			put(c.Row, c.Col, "'"+c.Raw)
		}
		var b strings.Builder
		for r := range rows {
			line := make([]string, cols)
			for c := range cols {
				line[c] = at[[2]int{r, c}]
			}
			b.WriteString(strings.TrimRight(strings.Join(line, "|"), "|"))
			b.WriteByte('\n')
		}
		out = b.String()
		return nil
	})
	return out, err
}
