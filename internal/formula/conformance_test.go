package formula

import (
	"bufio"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/store"
)

func mustParse(t testing.TB, src string, loc Locale) Node {
	t.Helper()
	n, err := Parse(src, loc)
	if err != nil {
		t.Fatalf("Parse(%q) : %v", src, err)
	}
	return n
}

type fixtureDoc struct {
	db     *store.Store
	schema *Schema
	data   Data
	names  *Names
	ventes *Table
}

// fixture crée deux tableaux, Ventes (avec deux colonnes calculées) et
// Clients (clé Id), et quelques cellules nommées.
func fixture(t testing.TB) *fixtureDoc {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(filepath.Join(t.TempDir(), "f.sqlite"), Register)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ventes := &Table{Name: "Ventes", SQL: "t_v", Cols: []Column{
		{Name: "Produit", SQL: "c_1", Type: grid.Text},
		{Name: "Prix", SQL: "c_2", Type: grid.Decimal},
		{Name: "Qté", SQL: "c_3", Type: grid.Integer},
		{Name: "Payé", SQL: "c_4", Type: grid.Boolean},
		{Name: "Client", SQL: "c_5", Type: grid.Integer},
		{Name: "Note", SQL: "c_6", Type: grid.Text},
		{Name: "Date", SQL: "c_7", Type: grid.Date},
		{Name: "Montant", Formula: mustParse(t, "=[@Prix]*[@Qté]", FR)},
		{Name: "NomClient", Formula: mustParse(t, "=REFERENCE(Clients;Nom;[@Client])", FR)},
	}}
	clients := &Table{Name: "Clients", SQL: "t_c", Cols: []Column{
		{Name: "Id", SQL: "c_1", Type: grid.Integer},
		{Name: "Nom", SQL: "c_2", Type: grid.Text},
		{Name: "Ville", SQL: "c_3", Type: grid.Text},
	}}
	baremes := &Table{Name: "Baremes", SQL: "t_b", Cols: []Column{
		{Name: "Seuil", SQL: "c_1", Type: grid.Integer},
		{Name: "Taux", SQL: "c_2", Type: grid.Decimal},
	}}
	feries := &Table{Name: "Feries", SQL: "t_f", Cols: []Column{{Name: "Jour", SQL: "c_1", Type: grid.Date}}}
	// Etapes : l'ordre d'affichage (_pos) n'est pas celui des identifiants.
	etapes := &Table{Name: "Etapes", SQL: "t_e", Cols: []Column{
		{Name: "Nom", SQL: "c_1", Type: grid.Text},
		{Name: "Valeur", SQL: "c_2", Type: grid.Integer},
	}}
	// Textes et Formats : nombres écrits dans des colonnes de texte
	// (décision 0013).
	textes := &Table{Name: "Textes", SQL: "t_x", Cols: []Column{
		{Name: "Val", SQL: "c_1", Type: grid.Text},
		{Name: "Groupe", SQL: "c_2", Type: grid.Text},
		{Name: "N", SQL: "c_3", Type: grid.Integer},
	}}
	formats := &Table{Name: "Formats", SQL: "t_y", Cols: []Column{
		{Name: "Val", SQL: "c_1", Type: grid.Text},
		{Name: "N", SQL: "c_2", Type: grid.Integer},
	}}
	schema := &Schema{
		Tables: []*Table{ventes, clients, baremes, feries, etapes, textes, formats},
		Now:    time.Date(2026, 10, 15, 14, 30, 0, 0, time.Local),
		Seed:   42,
	}
	err = db.Write(ctx, func(tx *sql.Tx) error {
		for _, q := range []string{
			`CREATE TABLE t_v (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 TEXT, c_2 REAL, c_3 INTEGER, c_4 INTEGER, c_5 INTEGER, c_6 TEXT, c_7 TEXT) STRICT`,
			`INSERT INTO t_v VALUES (1, 'a', 'Clavier', 25.5, 2, 1, 1, '12', '2026-01-15'), (2, 'b', 'Souris', 10, 3, 0, 2, 'abc', '2026-02-28'),
				(3, 'c', 'Écran', 199.99, 1, 1, 9, NULL, '2025-12-31'), (4, 'd', 'Câble', NULL, 5, NULL, NULL, NULL, NULL)`,
			`CREATE TABLE t_b (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 INTEGER, c_2 REAL) STRICT`,
			`INSERT INTO t_b VALUES (1, 'a', 0, 0), (2, 'b', 100, 0.05), (3, 'c', 500, 0.1)`,
			`CREATE TABLE t_f (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 TEXT) STRICT`,
			`INSERT INTO t_f VALUES (1, 'a', '2026-05-01'), (2, 'b', '2026-05-08'), (3, 'c', '2026-05-14')`,
			`CREATE TABLE t_e (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 TEXT, c_2 INTEGER) STRICT`,
			`INSERT INTO t_e VALUES (1, 'c', 'C', 30), (2, 'a', 'A', 10), (3, 'b', 'B', 20)`,
			`CREATE TABLE t_c (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 INTEGER, c_2 TEXT, c_3 TEXT) STRICT`,
			`INSERT INTO t_c VALUES (1, 'a', 1, 'Dupont', 'Paris'), (2, 'b', 2, 'Martin', 'Lyon'), (3, 'c', 3, 'Durand', 'Nice')`,
			`CREATE TABLE t_x (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 TEXT, c_2 TEXT, c_3 INTEGER) STRICT`,
			`INSERT INTO t_x VALUES (1, 'a', '12', 'A', 1), (2, 'b', '1 234,5', 'A', 2), (3, 'c', '12 €', 'B', 3),
				(4, 'd', '15 %', 'B', 4), (5, 'e', 'abc', 'A', 5), (6, 'f', NULL, 'B', 6), (7, 'g', '', 'A', 7)`,
			`CREATE TABLE t_y (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL, c_1 TEXT, c_2 INTEGER) STRICT`,
			`INSERT INTO t_y VALUES (1, 'a', '1,234.56', 1), (2, 'b', '1.234,56', 2), (3, 'c', '1''234.5', 3), (4, 'd', '1,234,567', 4),
				(5, 'e', '3.5', 5), (6, 'f', '1.234', 6), (7, 'g', '1,234', 7), (8, 'h', '(12)', 8), (9, 'i', '$12', 9), (10, 'j', '1,5e3', 10),
				(11, 'k', '-1 234,5 EUR', 11), (12, 'l', '0.250', 12)`,
		} {
			if _, err := tx.Exec(q); err != nil {
				return err
			}
		}
		if err := EnsureKey(ctx, tx, schema, KeyNeed{"Baremes", "Seuil"}); err != nil {
			return err
		}
		return EnsureKey(ctx, tx, schema, KeyNeed{"Clients", "Id"})
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := LoadData(ctx, db.Read(), schema)
	if err != nil {
		t.Fatal(err)
	}
	names := &Names{Formulas: map[string]Node{}}
	for name, f := range map[string]string{
		"tva":     "=0,2",
		"ca":      "=SOMME(Ventes[Montant])",
		"ca_ttc":  "=CA*(1+TVA)",
		"boucle1": "=Boucle2+1",
		"boucle2": "=Boucle1+1",
		// Cellules libres de la plage Cell[A1:B3] (A4 vide).
		"cell_a1": "=1",
		"cell_a2": "=2",
		"cell_a3": `="x"`,
		"cell_b1": "=10",
		"cell_b2": "=20",
		"cell_b3": "=30",
	} {
		names.Formulas[name] = mustParse(t, f, FR)
	}
	names.Recalc(&Env{Schema: schema, Data: data})
	return &fixtureDoc{db, schema, data, names, ventes}
}

// evalSQL évalue une formule compilée, sur la ligne n (1..) de Ventes si row.
func (f *fixtureDoc) evalSQL(t testing.TB, n Node, row int) Value {
	t.Helper()
	var c Compiled
	q := "SELECT "
	if row > 0 {
		c = Compile(n, f.schema, f.names.Values, f.ventes, "o")
		q += c.SQL + " FROM t_v AS o WHERE o.id = " + strconv.Itoa(row)
	} else {
		c = Compile(n, f.schema, f.names.Values, nil, "")
		q += c.SQL
	}
	var v any
	if err := f.db.Read().QueryRow(q, c.Args...).Scan(&v); err != nil {
		t.Errorf("%s : %v", q, err)
		return Str("(erreur SQL)")
	}
	return FromResult(v)
}

func (f *fixtureDoc) evalGo(n Node, row int) Value {
	env := &Env{Schema: f.schema, Data: f.data, Names: f.names.Values}
	if row > 0 {
		env.Table, env.Row = f.ventes, row-1
	}
	return Eval(n, env)
}

// TestConformite exécute testdata/conformite.txt sur les deux backends.
func TestConformite(t *testing.T) {
	f := fixture(t)
	file, err := os.Open("testdata/conformite.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	sc := bufio.NewScanner(file)
	row, loc, line, cases := 0, FR, 0, 0
	for sc.Scan() {
		line++
		text := strings.TrimSuffix(sc.Text(), "\r") // fichier extrait en CRLF sous Windows
		switch {
		case strings.TrimSpace(text) == "" || strings.HasPrefix(text, "#"):
			continue
		case strings.HasPrefix(text, "@"):
			row = 0
			if fields := strings.Fields(text); len(fields) == 3 {
				row, _ = strconv.Atoi(fields[2])
			}
			continue
		case strings.HasPrefix(text, "locale "):
			loc = map[string]Locale{"FR": FR, "EN": EN}[strings.TrimPrefix(text, "locale ")]
			continue
		case strings.HasPrefix(text, "nombres "):
			// Langue des nombres écrits en texte (Schema.Numbers).
			f.schema.Numbers = map[string]Locale{"FR": FR, "EN": EN}[strings.TrimPrefix(text, "nombres ")]
			continue
		}
		src, want, ok := strings.Cut(text, "\t")
		if !ok {
			t.Fatalf("ligne %d : tabulation attendue", line)
		}
		n, err := Parse(src, loc)
		if err != nil {
			t.Errorf("ligne %d %s : %v", line, src, err)
			continue
		}
		cases++
		if got := f.evalGo(n, row).String(); got != want {
			t.Errorf("ligne %d %s : Go donne %q, attendu %q", line, src, got, want)
		}
		if got := f.evalSQL(t, n, row).String(); got != want {
			t.Errorf("ligne %d %s : SQL donne %q, attendu %q", line, src, got, want)
		}
		// L'affichage réécrit doit redonner le même AST, dans les deux langues.
		for _, l := range []Locale{FR, EN} {
			back, err := Parse(Format(n, l), l)
			if err != nil || !reflect.DeepEqual(back, n) {
				t.Errorf("ligne %d : %s réécrit %s ne redonne pas le même AST (%v)", line, src, Format(n, l), err)
			}
		}
	}
	t.Logf("%d formules vérifiées sur les deux backends", cases)
}
