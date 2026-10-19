package query

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/grid"
)

var ctx = context.Background()

func newDoc(t *testing.T) *doc.Document {
	t.Helper()
	d, err := doc.New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func newSession(t *testing.T) *Session {
	t.Helper()
	s, err := NewSession()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func env(d *doc.Document) Env { return Env{Model: d.Model, Secrets: NewSecrets(false, "")} }

// addTable crée un tableau sur une nouvelle feuille.
func addTable(t *testing.T, d *doc.Document, name string, headers []string, types []grid.Type, rows ...[]any) {
	t.Helper()
	it := canvas.ImportTable{Name: name, Headers: headers, Types: types, Count: len(rows), Rows: func(y func([]any) error) error {
		for _, r := range rows {
			if err := y(r); err != nil {
				return err
			}
		}
		return nil
	}}
	if _, err := d.Import(ctx, "test", []canvas.ImportSheet{{Name: name, Tables: []canvas.ImportTable{it}}}); err != nil {
		t.Fatal(err)
	}
}

// dump lit un tableau du classeur, en-têtes (avec le type) puis lignes.
func dump(t *testing.T, d *doc.Document, table string) [][]string {
	t.Helper()
	var out [][]string
	var cols []canvas.ExportColumn
	err := d.ExportTable(ctx, table, func(c []canvas.ExportColumn) error {
		cols = c
		var h []string
		for _, c := range c {
			h = append(h, c.Name+":"+string(c.Type))
		}
		out = append(out, h)
		return nil
	}, func(vals []any) error {
		var r []string
		for i, v := range vals {
			r = append(r, display(plain(v), cols[i].Type))
		}
		out = append(out, r)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func save(t *testing.T, d *doc.Document, q Query) int64 {
	t.Helper()
	def, err := Encode(q)
	if err != nil {
		t.Fatal(err)
	}
	id, err := d.SaveQuery(ctx, def)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func writeCSV(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ventes.csv")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const ventesCSV = "Ville;Produit;Montant;Date;Code\n" +
	"Paris;Clavier;10,5;01/02/2026;01000\n" +
	"Lyon;Souris;20;02/02/2026;02000\n" +
	"Paris;Souris;5;03/02/2026;03000\n" +
	"Nice;Écran;100;04/02/2026;04000\n" +
	"Lyon;Clavier;7,25;05/02/2026;05000\n"

func TestCSVPreviewAndSteps(t *testing.T) {
	d := newDoc(t)
	s := newSession(t)
	q := Query{Name: "Ventes", Source: Source{Kind: SourceCSV, Path: writeCSV(t, ventesCSV)}, Steps: []Step{
		{Op: OpFilter, Conditions: []Cond{{Column: "Montant", Cmp: ">", Value: "6"}}},
		{Op: OpRename, Column: "Montant", Name: "Total"},
		{Op: OpGroup, Columns: []string{"Ville"}, Aggs: []Agg{{Name: "Somme", Op: "sum", Column: "Total"}, {Op: "count"}}},
		{Op: OpSort, Sort: []SortKey{{Column: "Somme", Desc: true}}},
	}}
	// Source seule : types inférés comme l'import (code postal en texte).
	pv := s.Preview(ctx, env(d), q, 0, false)
	if pv.Error != "" {
		t.Fatal(pv.Error)
	}
	wantCols := []Column{{"Ville", grid.Text}, {"Produit", grid.Text}, {"Montant", grid.Decimal}, {"Date", grid.Date}, {"Code", grid.Text}}
	if !reflect.DeepEqual(pv.Columns, wantCols) {
		t.Fatalf("colonnes %v", pv.Columns)
	}
	if pv.Count != 5 || pv.Rows[0][3] != "01/02/2026" || pv.Rows[0][4] != "01000" || pv.Rows[0][2] != "10,5" {
		t.Fatalf("aperçu source %v", pv.Rows)
	}
	// Toutes les étapes.
	pv = s.Preview(ctx, env(d), q, -1, false)
	if pv.Error != "" {
		t.Fatal(pv.Error)
	}
	got := fmt.Sprint(pv.Rows)
	if got != "[[Nice 100 1] [Lyon 27,25 2] [Paris 10,5 1]]" {
		t.Fatalf("aperçu final %s\n%s", got, pv.SQL)
	}
	if len(pv.Labels) != 5 || pv.Labels[2] != "Colonne renommée (Montant → Total)" {
		t.Fatalf("libellés %v", pv.Labels)
	}
	// Étape en erreur : l'aperçu s'arrête à la précédente.
	bad := q
	bad.Steps = append(append([]Step{}, q.Steps[:2]...), Step{Op: OpRemove, Columns: []string{"Inconnue"}})
	pv = s.Preview(ctx, env(d), bad, -1, false)
	if pv.ErrorStep != 2 || !strings.Contains(pv.Error, "Inconnue") || pv.Step != 2 || len(pv.Rows) != 4 {
		t.Fatalf("erreur %d %q étape %d lignes %d", pv.ErrorStep, pv.Error, pv.Step, len(pv.Rows))
	}
}

func TestRefreshLoadsAndReplaces(t *testing.T) {
	d := newDoc(t)
	s := newSession(t)
	path := writeCSV(t, ventesCSV)
	id := save(t, d, Query{Name: "Ventes par ville", Source: Source{Kind: SourceCSV, Path: path}, Steps: []Step{
		{Op: OpGroup, Columns: []string{"Ville"}, Aggs: []Agg{{Name: "Total", Op: "sum", Column: "Montant"}}},
	}})
	l, err := s.Refresh(ctx, env(d), id)
	if err != nil {
		t.Fatal(err)
	}
	if l.Table != "Ventesparville" || l.Rows != 3 {
		t.Fatalf("chargement %+v", l)
	}
	want := "[[Ville:text Total:decimal] [Lyon 27,25] [Nice 100] [Paris 15,5]]"
	if got := fmt.Sprint(dump(t, d, l.Table)); got != want {
		t.Fatalf("tableau %s", got)
	}
	// Colonne calculée ajoutée à la main : gardée à l'actualisation.
	tbl := l.Table
	lay, err := d.Layout(ctx, l.Sheet)
	if err != nil {
		t.Fatal(err)
	}
	ti := lay.Tables[0]
	// Le tableau se sait chargé par la requête (fond bleu, « Actualiser »).
	if ti.QueryID != id || ti.QueryName != "Ventes par ville" {
		t.Fatalf("requête du tableau : %d %q", ti.QueryID, ti.QueryName)
	}
	if _, err := d.SetCell(ctx, l.Sheet, ti.Row, ti.Col+2, "Double"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, l.Sheet, ti.Row+1, ti.Col+2, "=[Total]*2"); err != nil {
		t.Fatal(err)
	}
	// Le fichier change : l'actualisation suit, même nom, même place.
	if err := os.WriteFile(path, []byte(ventesCSV+"Nice;Souris;1;06/02/2026;06000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l2, err := s.Refresh(ctx, env(d), id)
	if err != nil {
		t.Fatal(err)
	}
	if l2.Table != tbl || l2.Sheet != l.Sheet {
		t.Fatalf("rechargé ailleurs : %+v", l2)
	}
	want = "[[Ville:text Total:decimal Double:text] [Lyon 27,25 54,5] [Nice 101 202] [Paris 15,5 31]]"
	if got := fmt.Sprint(dump(t, d, tbl)); got != want {
		t.Fatalf("après actualisation %s", got)
	}
	// Une seule étape d'annulation.
	if _, err := d.Bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(dump(t, d, tbl)); !strings.Contains(got, "[Nice 100 200]") {
		t.Fatalf("après annulation %s", got)
	}
	if _, err := d.Bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(dump(t, d, tbl)); got != want {
		t.Fatalf("après rétablissement %s", got)
	}
	qs, err := d.Queries(ctx)
	if err != nil || len(qs) != 1 || qs[0].Table != tbl || qs[0].Refreshed == "" {
		t.Fatalf("requêtes %+v %v", qs, err)
	}
	if msg, err := d.Check(ctx); err != nil || len(msg) > 0 {
		t.Fatalf("cohérence : %q %v", msg, err)
	}
}

func TestTableStepsMergeAppendPivot(t *testing.T) {
	d := newDoc(t)
	s := newSession(t)
	addTable(t, d, "Ventes", []string{"Client", "Mois", "Montant", "Tags"}, []grid.Type{grid.Text, grid.Text, grid.Integer, grid.Text},
		[]any{"A", "jan", int64(10), "x-1"},
		[]any{"B", "jan", int64(5), "y-2"},
		[]any{"A", "fev", int64(7), "z-3"},
		[]any{"C", "fev", int64(1), nil},
	)
	addTable(t, d, "Clients", []string{"Code", "Ville"}, []grid.Type{grid.Text, grid.Text},
		[]any{"A", "Paris"}, []any{"B", "Lyon"},
	)
	addTable(t, d, "Autres", []string{"Client", "Montant", "Note"}, []grid.Type{grid.Text, grid.Decimal, grid.Text},
		[]any{"D", 2.5, "nouveau"},
	)
	e := env(d)
	check := func(name string, steps []Step, want string) {
		t.Helper()
		pv := s.Preview(ctx, e, Query{Name: name, Source: Source{Kind: SourceTable, Table: "Ventes"}, Steps: steps}, -1, false)
		if pv.Error != "" {
			t.Fatalf("%s : %s (étape %d)\n%s", name, pv.Error, pv.ErrorStep, pv.SQL)
		}
		var h []string
		for _, c := range pv.Columns {
			h = append(h, c.Name+":"+string(c.Type))
		}
		if got := fmt.Sprint(h, pv.Rows); got != want {
			t.Fatalf("%s :\n%s\nattendu\n%s\n%s", name, got, want, pv.SQL)
		}
	}
	check("fusion", []Step{
		{Op: OpMerge, Other: &Source{Kind: SourceTable, Table: "Clients"}, Keys: []string{"Client"}, OtherKeys: []string{"Code"}, Join: "left"},
		{Op: OpSelect, Columns: []string{"Client", "Ville", "Montant"}},
	}, "[Client:text Ville:text Montant:integer] [[A Paris 10] [B Lyon 5] [A Paris 7] [C  1]]")
	check("anti", []Step{
		{Op: OpMerge, Other: &Source{Kind: SourceTable, Table: "Clients"}, Keys: []string{"Client"}, OtherKeys: []string{"Code"}, Join: "anti"},
		{Op: OpRemove, Columns: []string{"Tags"}},
	}, "[Client:text Mois:text Montant:integer] [[C fev 1]]")
	check("ajout", []Step{
		{Op: OpRemove, Columns: []string{"Mois", "Tags"}},
		{Op: OpAppend, Other: &Source{Kind: SourceTable, Table: "Autres"}},
		{Op: OpTop, Count: 10},
	}, "[Client:text Montant:decimal Note:text] [[A 10 ] [B 5 ] [A 7 ] [C 1 ] [D 2,5 nouveau]]")
	check("pivot", []Step{
		{Op: OpRemove, Columns: []string{"Tags"}},
		{Op: OpPivot, Column: "Mois", ValueColumn: "Montant", Agg: "sum"},
	}, "[Client:text fev:integer jan:integer] [[A 7 10] [B  5] [C 1 ]]")
	check("dépivot", []Step{
		{Op: OpSelect, Columns: []string{"Client", "Mois", "Tags"}},
		{Op: OpUnpivot, Columns: []string{"Mois", "Tags"}, Name: "Champ", ValueName: "Val"},
		{Op: OpSort, Sort: []SortKey{{Column: "Client"}, {Column: "Champ"}, {Column: "Val"}}},
	}, "[Client:text Champ:text Val:text] [[A Mois fev] [A Mois jan] [A Tags x-1] [A Tags z-3] [B Mois jan] [B Tags y-2] [C Mois fev]]")
	check("fractionner, personnalisée, index", []Step{
		{Op: OpSplit, Column: "Tags", Delimiter: "-", Parts: 2},
		{Op: OpAddColumn, Name: "Double", Expr: "[Montant] * 2 + [Tags.2]"},
		{Op: OpIndex, Name: "N", Start: 1},
		{Op: OpFilter, Any: true, Conditions: []Cond{{Column: "Tags.1", Cmp: "=", Value: "x"}, {Column: "Tags.1", Cmp: "empty"}}},
		{Op: OpRemove, Columns: []string{"Mois"}},
	}, "[Client:text Montant:integer Tags.1:text Tags.2:integer Double:integer N:integer] [[A 10 x 1 21 1] [C 1    4]]")
	check("type, remplacer, texte, doublons", []Step{
		{Op: OpType, Column: "Montant", Type: grid.Text},
		{Op: OpReplace, Column: "Client", Value: "A", With: "Alpha"},
		{Op: OpTransform, Column: "Mois", Fn: "upper"},
		{Op: OpDistinct, Columns: []string{"Client"}},
		{Op: OpRemove, Columns: []string{"Tags"}},
	}, "[Client:text Mois:text Montant:text] [[Alpha JAN 10] [B JAN 5] [C FEV 1]]")

	// Une requête peut en citer une autre ; une boucle est refusée.
	save(t, d, Query{Name: "Base", Source: Source{Kind: SourceTable, Table: "Ventes"}, Steps: []Step{{Op: OpFilter, Conditions: []Cond{{Column: "Montant", Cmp: ">=", Value: "5"}}}}})
	pv := s.Preview(ctx, e, Query{Name: "Dérivée", Source: Source{Kind: SourceQuery, Query: "Base"}, Steps: []Step{{Op: OpGroup, Aggs: []Agg{{Name: "N", Op: "count"}}}}}, -1, false)
	if pv.Error != "" || fmt.Sprint(pv.Rows) != "[[3]]" {
		t.Fatalf("requête citée : %v %s", pv.Rows, pv.Error)
	}
	save(t, d, Query{Name: "Boucle", Source: Source{Kind: SourceQuery, Query: "Boucle"}})
	pv = s.Preview(ctx, e, Query{Name: "Boucle", Source: Source{Kind: SourceQuery, Query: "Boucle"}}, -1, false)
	if !strings.Contains(pv.Error, "elle-même") {
		t.Fatalf("boucle : %q", pv.Error)
	}
}

func TestQueryDefinitionsUndo(t *testing.T) {
	d := newDoc(t)
	id := save(t, d, Query{Name: "R1", Source: Source{Kind: SourceTable, Table: "X"}})
	if _, err := d.SaveQuery(ctx, canvas.QueryDef{Name: "r1", Source: "{}"}); err == nil {
		t.Fatal("nom en double accepté")
	}
	def, _ := Encode(Query{ID: id, Name: "R2", Source: Source{Kind: SourceTable, Table: "Y"}})
	if _, err := d.SaveQuery(ctx, def); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	q, err := d.QueryByName(ctx, "R1")
	if err != nil || !strings.Contains(q.Source, `"X"`) {
		t.Fatalf("annulation : %+v %v", q, err)
	}
	// Connexions : jamais de mot de passe, suppression refusée si utilisée.
	cdef, err := EncodeConnection(Connection{Name: "Prod", Config: DBConfig{Host: "db", Database: "ventes", User: "louis"}})
	if err != nil {
		t.Fatal(err)
	}
	cid, err := d.SaveConnection(ctx, cdef)
	if err != nil {
		t.Fatal(err)
	}
	save(t, d, Query{Name: "PG", Source: Source{Kind: SourcePostgres, Connection: "Prod", SQL: "SELECT 1"}})
	if err := d.DeleteConnection(ctx, cid); err == nil {
		t.Fatal("connexion utilisée supprimée")
	}
	if err := d.DeleteQuery(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.QueryByName(ctx, "R1"); err != nil {
		t.Fatal(err)
	}
}

// Un mot de passe est gardé par compte, sur ce poste ; un classeur ne s'en
// sert qu'avec l'accord de l'utilisateur (Grant), retenu par classeur.
// Saisir le mot de passe dans un classeur vaut accord pour lui.
func TestSecretsPerWorkbook(t *testing.T) {
	grants := filepath.Join(t.TempDir(), "accords.json")
	root := NewSecrets(false, grants)
	a, b := root.For("classeur:a"), root.For("classeur:b")
	if err := a.Set("pg://louis@db/ventes", "secret", true); err != nil {
		t.Fatal(err)
	}
	if pw, ok := a.Get("pg://louis@db/ventes"); !ok || pw != "secret" {
		t.Fatal("mot de passe perdu")
	}
	// Un autre classeur : pas sans accord, mais l'interface peut le proposer.
	if _, ok := b.Get("pg://louis@db/ventes"); ok {
		t.Fatal("un autre classeur réutilise le mot de passe sans accord")
	}
	if !b.Pending("pg://louis@db/ventes") || a.Pending("pg://louis@db/ventes") {
		t.Fatal("accord à demander mal signalé")
	}
	if err := b.Grant("pg://louis@db/ventes"); err != nil {
		t.Fatal(err)
	}
	if pw, ok := b.Get("pg://louis@db/ventes"); !ok || pw != "secret" {
		t.Fatal("accord donné, mot de passe refusé")
	}
	// Les accords sont retenus sur le poste (nouveau coffre, même fichier).
	again := NewSecrets(false, grants)
	again.For("classeur:a").Set("pg://louis@db/ventes", "secret", false)
	if !again.For("classeur:b").Granted("pg://louis@db/ventes") || again.For("classeur:c").Granted("pg://louis@db/ventes") {
		t.Fatal("accords non relus")
	}
	// Env.Workbook choisit la vue du classeur.
	if pw, ok := (Env{Secrets: root, Workbook: "classeur:a"}).secrets().Get("pg://louis@db/ventes"); !ok || pw != "secret" {
		t.Fatal("Env.Workbook ignoré")
	}
	if _, ok := (Env{Secrets: root, Workbook: "classeur:z"}).secrets().Get("pg://louis@db/ventes"); ok {
		t.Fatal("Env d'un classeur non autorisé : mot de passe réutilisé")
	}
}

func TestSecrets(t *testing.T) {
	s := NewSecrets(false, "")
	if _, ok := s.Get("a"); ok {
		t.Fatal("mot de passe inventé")
	}
	s.Set("a", "secret", true)
	if pw, ok := s.Get("a"); !ok || pw != "secret" {
		t.Fatal("mot de passe perdu")
	}
	s.Forget("a")
	if _, ok := s.Get("a"); ok {
		t.Fatal("mot de passe gardé")
	}
}

// Une requête SQL sur le classeur (Référencer depuis l'onglet SQL) se relit
// à chaque actualisation.
func TestSQLSource(t *testing.T) {
	d := newDoc(t)
	addTable(t, d, "Ventes", []string{"Client", "Montant"}, []grid.Type{grid.Text, grid.Integer}, []any{"A", int64(10)}, []any{"B", int64(5)}, []any{"A", int64(1)})
	id := save(t, d, Query{Name: "Totaux", Source: Source{Kind: SourceSQL, SQL: `SELECT c_1, SUM(c_2) AS s FROM t_1 GROUP BY c_1 ORDER BY c_1`}, Steps: []Step{{Op: OpRename, Column: "c_1", Name: "Client"}, {Op: OpRename, Column: "s", Name: "Total"}}})
	l, err := Refresh(ctx, env(d), id)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(dump(t, d, l.Table)); got != "[[Client:text Total:integer] [A 11] [B 5]]" {
		t.Fatalf("chargé : %s", got)
	}
	bad := save(t, d, Query{Name: "Vide", Source: Source{Kind: SourceSQL, SQL: "SELECT * FROM Absent"}})
	if _, err := Refresh(ctx, env(d), bad); err == nil {
		t.Fatal("tableau absent : erreur attendue")
	}
}

// Une requête SQL enregistrée en noms internes avec SELECT * (sans la
// traduction de l'onglet SQL) se relit avec les en-têtes affichés, sans
// _pos, à chaque actualisation ; un nom interne cité par une étape reste
// tel quel pour elle.
func TestSQLSourceHeaders(t *testing.T) {
	d := newDoc(t)
	addTable(t, d, "Nids2021", []string{"Espèce", "Nids"}, []grid.Type{grid.Text, grid.Integer}, []any{"Mésange", int64(4)})
	addTable(t, d, "Nids2022", []string{"Espèce", "Nids"}, []grid.Type{grid.Text, grid.Integer}, []any{"Rouge-gorge", int64(2)})
	id := save(t, d, Query{Name: "Suivi", Source: Source{Kind: SourceSQL, SQL: `SELECT *, 2021 AS Année FROM t_1 UNION SELECT *, 2022 FROM t_2 ORDER BY 1, 2`}})
	want := "[[id:integer Espèce:text Nids:integer Année:integer] [1 Mésange 4 2021] [1 Rouge-gorge 2 2022]]"
	for i := range 2 {
		l, err := Refresh(ctx, env(d), id)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprint(dump(t, d, l.Table)); got != want {
			t.Fatalf("actualisation %d : %s", i+1, got)
		}
	}
	// Requête traduite (Référencer d'aujourd'hui) : inchangée.
	tr := save(t, d, Query{Name: "Traduite", Source: Source{Kind: SourceSQL, SQL: `SELECT id, c_1 AS "Espèce", c_2 AS "Nids" FROM t_1`}})
	l, err := Refresh(ctx, env(d), tr)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(dump(t, d, l.Table)); got != "[[id:integer Espèce:text Nids:integer] [1 Mésange 4]]" {
		t.Fatalf("traduite : %s", got)
	}
	// Étape qui cite c_2 : le nom interne lui reste.
	st := save(t, d, Query{Name: "Étapes", Source: Source{Kind: SourceSQL, SQL: `SELECT * FROM t_1`}, Steps: []Step{{Op: OpRename, Column: "c_2", Name: "Nombre"}}})
	if l, err = Refresh(ctx, env(d), st); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(dump(t, d, l.Table)); got != "[[id:integer Espèce:text Nombre:integer] [1 Mésange 4]]" {
		t.Fatalf("étape : %s", got)
	}
}

// Colonne saisie à côté d'un résultat référencé depuis l'onglet SQL (UNION
// de deux tableaux annuels, mêmes id) : jointe par « id + Année », elle
// suit ses lignes quand la source change (décision 0008).
func TestSQLSourceHandColumn(t *testing.T) {
	d := newDoc(t)
	addTable(t, d, "Nids2021", []string{"Espèce", "Nids"}, []grid.Type{grid.Text, grid.Integer}, []any{"Mésange", int64(4)}, []any{"Merle", int64(1)})
	addTable(t, d, "Nids2022", []string{"Espèce", "Nids"}, []grid.Type{grid.Text, grid.Integer}, []any{"Mésange", int64(5)})
	id := save(t, d, Query{Name: "Suivi", Source: Source{Kind: SourceSQL, SQL: `SELECT id, c_1 AS "Espèce", c_2 AS "Nids", 2021 AS Année FROM t_1 UNION SELECT id, c_1 AS "Espèce", c_2 AS "Nids", 2022 FROM t_2 ORDER BY 4, 1`}})
	l, err := Refresh(ctx, env(d), id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, l.Sheet, 0, 4, "Remarque"); err != nil {
		t.Fatal(err)
	}
	// Ligne 3 : Mésange 2022 (id 1, comme Mésange 2021).
	if _, err := d.SetCell(canvas.WithKeyChoice(ctx, "hand:id + Année"), l.Sheet, 3, 4, "à surveiller"); err != nil {
		t.Fatal(err)
	}
	// Une ligne de 2021 de plus : les lignes de 2022 descendent, la remarque suit.
	lay, err := d.Layout(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	var src int64
	for _, s := range lay.Sheets {
		if s.Name == "Nids2021" {
			src = s.ID
		}
	}
	if _, err := d.SetCell(ctx, src, 3, 0, "Pic"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, src, 3, 1, "2"); err != nil {
		t.Fatal(err)
	}
	if l, err = Refresh(ctx, env(d), id); err != nil {
		t.Fatal(err)
	}
	want := "[[id:integer Espèce:text Nids:integer Année:integer Remarque:text] [1 Mésange 4 2021 ] [2 Merle 1 2021 ] [3 Pic 2 2021 ] [1 Mésange 5 2022 à surveiller]]"
	if got := fmt.Sprint(dump(t, d, l.Table)); got != want {
		t.Fatalf("après actualisation :\n got %s\nwant %s", got, want)
	}
}

// Un tableau supprimé avec sa feuille laisse sa requête sans tableau : un
// résultat copié ensuite ne prend pas son identifiant, et donc pas sa
// requête (F5 l'aurait remplacé par elle).
func TestDeletedTargetNotAdopted(t *testing.T) {
	d := newDoc(t)
	addTable(t, d, "Nids", []string{"Espèce", "Nids"}, []grid.Type{grid.Text, grid.Integer}, []any{"Mésange", int64(4)})
	id := save(t, d, Query{Name: "Requête1", Source: Source{Kind: SourceSQL, SQL: `SELECT * FROM t_1`}})
	l, err := Refresh(ctx, env(d), id)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteSheet(ctx, l.Sheet); err != nil {
		t.Fatal(err)
	}
	r, err := d.QueryToSheet(ctx, `SELECT c_1 AS "Espèce" FROM t_1`, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	lay, err := d.Layout(ctx, r.Sheets[0])
	if err != nil {
		t.Fatal(err)
	}
	if q := lay.Tables[0].QueryID; q != 0 {
		t.Fatalf("résultat copié lié à la requête %d", q)
	}
	// Annuler la suppression rend le tableau à sa requête.
	if _, err := d.Bus.Undo(ctx); err != nil { // copie
		t.Fatal(err)
	}
	if _, err := d.Bus.Undo(ctx); err != nil { // suppression de la feuille
		t.Fatal(err)
	}
	qs, err := d.Queries(ctx)
	if err != nil || len(qs) != 1 || qs[0].Table == "" {
		t.Fatalf("requête après annulation : %+v %v", qs, err)
	}
}
