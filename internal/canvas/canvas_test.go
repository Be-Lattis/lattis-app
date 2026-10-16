package canvas

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/slitable/internal/command"
	"github.com/lalamefine/slitable/internal/formula"
	"github.com/lalamefine/slitable/internal/grid"
	"github.com/lalamefine/slitable/internal/meta"
	"github.com/lalamefine/slitable/internal/store"
)

type fixture struct {
	*Model
	bus *command.Bus
}

func open(t *testing.T) *fixture {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "doc.sqlite"), Register)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := meta.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	bus := command.New(db)
	return &fixture{New(db, bus), bus}
}

func view(t *testing.T, d *fixture) string {
	t.Helper()
	v, err := d.View(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func dump(t *testing.T, d *fixture) string {
	t.Helper()
	s, err := d.Dump(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func check(t *testing.T, d *fixture, when string) {
	t.Helper()
	problems, err := d.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Errorf("%s : %s", when, p)
	}
}

func lines(s ...string) string { return strings.Join(s, "\n") + "\n" }

// scenarioStep est une action du scénario, avec la feuille attendue après coup
// (vide = pas de vérification de l'affichage).
type scenarioStep struct {
	name string
	do   func(context.Context, *fixture) error
	want string
}

func set(row, col int, raw string) func(context.Context, *fixture) error {
	return func(ctx context.Context, d *fixture) error {
		_, err := d.SetCell(ctx, 1, row, col, raw)
		return err
	}
}

// TestScenario est le critère de sortie du spike : une suite d'actions,
// chacune suivie d'une vérification des invariants ; puis tout est annulé
// pas à pas (l'état doit retrouver exactement chaque état intermédiaire) et
// rétabli (l'état final doit être identique).
func TestScenario(t *testing.T) {
	ctx := context.Background()
	d := open(t)

	steps := []scenarioStep{
		{"en-tête Produit", set(0, 0, "Produit"), lines("'Produit")},
		{"en-tête Prix", set(0, 1, "Prix"), lines("'Produit|'Prix")},
		{"naissance du tableau", set(1, 0, "Clavier"), lines(
			"[Produit]|[Prix]",
			"Clavier")},
		{"colonne vide typée par sa 1re valeur", set(1, 1, "12,5"), lines(
			"[Produit]|[Prix]",
			"Clavier|12.5")},
		{"ligne grisée : ajout de ligne", set(2, 0, "Souris"), lines(
			"[Produit]|[Prix]",
			"Clavier|12.5",
			"Souris")},
		{"entier dans une colonne décimale", set(2, 1, "8"), ""},
		{"deuxième tableau sous la ligne grisée", set(4, 0, "Ville"), ""},
		{"naissance du deuxième tableau", set(5, 0, "Paris"), lines(
			"[Produit]|[Prix]",
			"Clavier|12.5",
			"Souris|8",
			"",
			"[Ville]",
			"Paris")},
		{"insertion au milieu : le tableau du dessous descend", func(ctx context.Context, d *fixture) error {
			return d.InsertRow(ctx, "Feuille1", 1)
		}, lines(
			"[Produit]|[Prix]",
			"Clavier|12.5",
			"",
			"Souris|8",
			"",
			"[Ville]",
			"Paris")},
		{"remplir la ligne insérée", set(2, 0, "Écran"), ""},
		{"colonne grisée : ajout de colonne par l'en-tête", set(0, 2, "Stock"), ""},
		{"valeur dans la nouvelle colonne", set(1, 2, "3"), ""},
		{"deuxième valeur", set(2, 2, "10"), ""},
		{"élargissement entier → texte", set(3, 2, "beaucoup"), lines(
			"[Produit]|[Prix]|[Stock]",
			"Clavier|12.5|3",
			"Écran||10",
			"Souris|8|beaucoup",
			"",
			"[Ville]",
			"Paris")},
		{"renommer une colonne", set(0, 0, "Article"), ""},
		{"supprimer la colonne Prix", func(ctx context.Context, d *fixture) error {
			return d.DeleteColumn(ctx, "Feuille1", 1)
		}, lines(
			"[Article]|[Stock]",
			"Clavier|3",
			"Écran|10",
			"Souris|beaucoup",
			"",
			"[Ville]",
			"Paris")},
		{"insérer une colonne au milieu", func(ctx context.Context, d *fixture) error {
			return d.InsertColumn(ctx, "Feuille1", 1, "Date")
		}, ""},
		{"date française", set(1, 1, "15/10/2026"), ""},
		{"supprimer une ligne", func(ctx context.Context, d *fixture) error {
			return d.DeleteRow(ctx, "Feuille1", 1)
		}, lines(
			"[Article]|[Date]|[Stock]",
			"Clavier|2026-10-15|3",
			"Souris||beaucoup",
			"",
			"",
			"[Ville]",
			"Paris")},
		{"fixer le type de Ville", func(ctx context.Context, d *fixture) error { return d.FixType(ctx, "Tableau1", 0) }, ""},
		{"colonne grisée du deuxième tableau", set(6, 1, "75000"), ""},
		{"ligne grisée du deuxième tableau", set(7, 0, "Lyon"), ""},
		{"note libre isolée", set(10, 4, "note"), ""},
		{"tableau à droite du premier", set(0, 5, "Code"), ""},
		{"naissance du tableau de droite", set(1, 5, "A1"), ""},
		{"colonne grisée : le tableau de droite se décale", set(0, 3, "Payé"), ""},
		{"booléen dans la colonne Payé", set(1, 3, "VRAI"), lines(
			"[Article]|[Date]|[Stock]|[Payé]||[Code]",
			"Clavier|2026-10-15|3|1||A1",
			"Souris||beaucoup",
			"",
			"",
			"[Ville]|[Colonne 2]",
			"Paris|75000",
			"Lyon",
			"",
			"",
			"||||'note")},
		{"effacer une cellule de tableau", set(1, 0, ""), ""},
		{"effacer une cellule libre", set(10, 4, ""), ""},
	}

	states := []string{dump(t, d)}
	for _, s := range steps {
		if err := s.do(ctx, d); err != nil {
			t.Fatalf("%s : %v", s.name, err)
		}
		check(t, d, s.name)
		if s.want != "" {
			if got := view(t, d); got != s.want {
				t.Fatalf("%s : feuille\n%s\nattendu\n%s", s.name, got, s.want)
			}
		}
		states = append(states, dump(t, d))
	}

	for i := len(steps) - 1; i >= 0; i-- {
		if _, err := d.bus.Undo(ctx); err != nil {
			t.Fatalf("annuler %q : %v", steps[i].name, err)
		}
		check(t, d, "après annulation de "+steps[i].name)
		if got := dump(t, d); got != states[i] {
			t.Fatalf("annuler %q ne restaure pas l'état précédent :\n%s", steps[i].name, diff(states[i], got))
		}
	}
	if st, _ := d.bus.State(ctx); st.Undo != "" {
		t.Fatal("la pile d'annulation devrait être vide")
	}
	for i := range steps {
		if _, err := d.bus.Redo(ctx); err != nil {
			t.Fatalf("rétablir %q : %v", steps[i].name, err)
		}
		if got := dump(t, d); got != states[i+1] {
			t.Fatalf("rétablir %q ne redonne pas le même état :\n%s", steps[i].name, diff(states[i+1], got))
		}
	}
	check(t, d, "après rétablissement")
}

func TestFixedTypeMarksInvalid(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	for _, c := range []struct {
		r, c int
		s    string
	}{{0, 0, "Qté"}, {1, 0, "1"}, {2, 0, "2"}} {
		if _, err := d.SetCell(ctx, 1, c.r, c.c, c.s); err != nil {
			t.Fatal(err)
		}
	}
	before := dump(t, d)
	if err := d.FixType(ctx, "Feuille1", 0); err != nil {
		t.Fatal(err)
	}
	ev, err := d.SetCell(ctx, 1, 2, 0, "deux")
	if err != nil || ev.Widened != "" {
		t.Fatalf("SetCell = %+v, %v", ev, err)
	}
	check(t, d, "saisie invalide")
	if !strings.Contains(dump(t, d), "1 | 2 | 1 | deux") {
		t.Fatalf("saisie invalide non enregistrée :\n%s", dump(t, d))
	}
	d.bus.Undo(ctx)
	d.bus.Undo(ctx)
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestWideningEvent(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	d.SetCell(ctx, 1, 0, 0, "Prix")
	d.SetCell(ctx, 1, 1, 0, "3")
	ev, err := d.SetCell(ctx, 1, 2, 0, "3,5")
	if err != nil || ev.Widened != "Feuille1.Prix" || ev.From != grid.Integer || ev.To != grid.Decimal {
		t.Fatalf("SetCell = %+v, %v", ev, err)
	}
	check(t, d, "élargissement")
}

func TestInfer(t *testing.T) {
	for raw, want := range map[string]grid.Type{
		"12": grid.Integer, "1 234": grid.Integer, "12,5": grid.Decimal, "VRAI": grid.Boolean,
		"15/10/2026": grid.Date, "abc": grid.Text, "": "",
	} {
		if got := Infer(raw); got != want {
			t.Errorf("Infer(%q) = %q, attendu %q", raw, got, want)
		}
	}
	if Join(grid.Boolean, grid.Decimal) != grid.Decimal || Join(grid.Date, grid.Integer) != grid.Text {
		t.Error("Join")
	}
}

// diff montre les lignes qui diffèrent entre deux dumps.
func diff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	var b strings.Builder
	for i := range max(len(w), len(g)) {
		var a, c string
		if i < len(w) {
			a = w[i]
		}
		if i < len(g) {
			c = g[i]
		}
		if a != c {
			b.WriteString("- " + a + "\n+ " + c + "\n")
		}
	}
	return b.String()
}

func TestPaste(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	before := dump(t, d)
	// Bloc collé depuis Excel dans une zone vide : un tableau, une étape ; seul
	// tableau de la feuille, il prend le nom de l'onglet.
	ev, err := d.Paste(ctx, 1, 2, 1, [][]string{
		{"Ville", "Habitants", "Capitale"},
		{"Paris", "2 100 000", "VRAI"},
		{"Lyon", "520 000", "FAUX"},
		{"Nice", "", "FAUX"},
	})
	if err != nil || ev.Created != "Feuille1" {
		t.Fatalf("Paste = %+v, %v", ev, err)
	}
	check(t, d, "collage d'un bloc")
	want := lines(
		"", "",
		"|[Ville]|[Habitants]|[Capitale]",
		"|Paris|2100000|1",
		"|Lyon|520000|0",
		"|Nice||0")
	if got := view(t, d); got != want {
		t.Fatalf("feuille\n%s\nattendu\n%s", got, want)
	}
	// Collage sur le tableau : cellule par cellule (extension, élargissement).
	if _, err := d.Paste(ctx, 1, 6, 1, [][]string{{"Lille", "inconnu"}, {"Brest", "140 000"}}); err != nil {
		t.Fatal(err)
	}
	check(t, d, "collage sur un tableau")
	if got := view(t, d); !strings.Contains(got, "|Lille|inconnu\n") || !strings.Contains(got, "|Nice||0") || !strings.Contains(got, "|Brest|140 000") {
		t.Fatalf("feuille\n%s", got)
	}
	// Deux annulations défont les deux collages.
	d.bus.Undo(ctx)
	d.bus.Undo(ctx)
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestSheets(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	before := dump(t, d)
	id, err := d.AddSheet(ctx)
	if err != nil || id != 2 {
		t.Fatalf("AddSheet = %d, %v", id, err)
	}
	if err := d.RenameSheet(ctx, id, "Budget"); err != nil {
		t.Fatal(err)
	}
	if err := d.RenameSheet(ctx, 1, "Budget"); !errors.Is(err, ErrSheet) {
		t.Fatalf("doublon de nom accepté : %v", err)
	}
	if _, err := d.SetCell(ctx, id, 3, 3, "note"); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteSheet(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteSheet(ctx, 1); !errors.Is(err, ErrSheet) {
		t.Fatalf("dernière feuille supprimée : %v", err)
	}
	check(t, d, "feuilles")
	for range 4 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestSetType(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Article", "Prix"}, {"a", "12,5"}, {"b", "8"}, {"c", ""}}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	n, err := d.ConvertPreview(ctx, "Feuille1", 1, grid.Integer)
	if err != nil || n != 1 {
		t.Fatalf("aperçu : %d, %v", n, err)
	}
	if err := d.SetType(ctx, "Feuille1", 1, grid.Integer); err != nil {
		t.Fatal(err)
	}
	check(t, d, "type fixé en entier")
	if got := view(t, d); got != lines("[Article]|[Prix]", "a", "b|8", "c") {
		t.Fatalf("feuille\n%s", got)
	}
	if !strings.Contains(dump(t, d), "1 | 1 | 2 | 12,5") {
		t.Fatalf("12,5 non marqué en erreur :\n%s", dump(t, d))
	}
	// Vers le texte : les nombres s'écrivent à la française.
	if err := d.SetType(ctx, "Feuille1", 1, grid.Text); err != nil {
		t.Fatal(err)
	}
	check(t, d, "type fixé en texte")
	d.bus.Undo(ctx)
	d.bus.Undo(ctx)
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
	if err := d.SetType(ctx, "Feuille1", 1, grid.Text); err != nil {
		t.Fatal(err)
	}
	if got := view(t, d); got != lines("[Article]|[Prix]", "a|12,5", "b|8", "c") {
		t.Fatalf("feuille\n%s", got)
	}
}

func TestFormulas(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix", "Qté"}, {"Clavier", "25,5", "2"}, {"Souris", "10", "3"}}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	// En-tête dans la colonne grisée, puis formule dans une cellule : toute la
	// colonne est calculée.
	if _, err := d.SetCell(ctx, 1, 0, 3, "Montant"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 2, 3, "=[@Prix]*[@Qté]"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 3, "123"); !errors.As(err, &ErrComputed{}) {
		t.Fatalf("valeur acceptée dans une colonne calculée : %v", err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 3, "=[@Prix]*"); !errors.As(err, &ErrFormula{}) {
		t.Fatalf("formule invalide acceptée : %v", err)
	}
	// Valeurs calculées par les expressions de la lecture par fenêtres.
	exprs, args, computed, err := d.ColumnExprs(ctx, 1)
	if err != nil || len(exprs) != 4 || !computed[3] {
		t.Fatalf("ColumnExprs = %v, %v, %v", exprs, computed, err)
	}
	rows, err := d.db.Read().Query(`SELECT `+exprs[3]+` FROM t_1 ORDER BY _pos`, args...)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var v any
		rows.Scan(&v)
		got = append(got, formula.FromResult(v).String())
	}
	rows.Close()
	if strings.Join(got, ";") != "51;30" {
		t.Fatalf("Montant = %v", got)
	}
	// Cellule libre avec formule : total et recherche (la colonne Produit
	// devient la clé du tableau).
	if _, err := d.SetCell(ctx, 1, 6, 0, "=SOMME(Feuille1[Montant])"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 6, 2, "=RECHERCHEV(\"souris\";Feuille1;2;FAUX)"); err != nil {
		t.Fatal(err)
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	vals := map[string]string{}
	for _, c := range l.Cells {
		vals[c.Raw] = c.Value
	}
	if vals["=SOMME(Feuille1[Montant])"] != "81" || vals["=REFERENCE(Feuille1; Prix; \"souris\")"] != "10" {
		t.Fatalf("cellules libres : %v", vals)
	}
	if l.Tables[0].Columns[3].Formula != "=[@Prix]*[@Qté]" {
		t.Fatalf("formule affichée : %q", l.Tables[0].Columns[3].Formula)
	}
	check(t, d, "formules")
	for range 4 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestNames(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Clavier", "25,5"}, {"Souris", "10"}}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	set := func(r, c int, raw string) {
		t.Helper()
		if _, err := d.SetCell(ctx, 1, r, c, raw); err != nil {
			t.Fatal(err)
		}
	}
	set(5, 0, "0,2")
	if err := d.SetName(ctx, 1, 5, 0, "Taux"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"Feuille1", "SOMME", "VRAI", "2x", "avec espace"} {
		if err := d.SetName(ctx, 1, 6, 0, bad); !errors.As(err, &ErrName{}) {
			t.Errorf("nom %q accepté : %v", bad, err)
		}
	}
	set(5, 2, "=Taux*100")
	set(5, 4, "=Inconnu+1")
	set(8, 0, "=B+1")
	set(8, 2, "=A+1")
	if err := d.SetName(ctx, 1, 8, 0, "A"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetName(ctx, 1, 8, 2, "B"); err != nil {
		t.Fatal(err)
	}
	// Colonne calculée qui utilise un nom.
	set(0, 2, "TTC")
	set(1, 2, "=[@Prix]*(1+Taux)")
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	vals := map[string]string{}
	for _, c := range l.Cells {
		vals[c.Raw] = c.Value
	}
	if vals["=Taux*100"] != "20" || vals["=Inconnu+1"] != "#NOM?" || vals["=B+1"] != "#REF!" {
		t.Fatalf("valeurs : %v", vals)
	}
	exprs, args, _, err := d.ColumnExprs(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := d.db.Read().QueryRow(`SELECT `+exprs[2]+` FROM t_1 ORDER BY _pos LIMIT 1`, args...).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if got := formula.FromResult(v).String(); got != "30,6" {
		t.Fatalf("TTC = %s", got)
	}
	// Deux lignes ajoutées au tableau poussent la cellule nommée : le nom suit.
	for range 2 {
		if err := d.InsertRow(ctx, "Feuille1", 0); err != nil {
			t.Fatal(err)
		}
	}
	l, _ = d.Layout(ctx, 1)
	for _, n := range l.Names {
		if n.Name == "Taux" && n.Row != 6 {
			t.Fatalf("Taux est resté en ligne %d", n.Row)
		}
	}
	check(t, d, "noms")
	for range 12 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestScriptsLibrary(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	before := dump(t, d)
	id, err := d.SaveScript(ctx, 0, "Bonjour", `print("bonjour")`)
	if err != nil || id != 1 {
		t.Fatalf("SaveScript = %d, %v", id, err)
	}
	if _, err := d.SaveScript(ctx, 0, "bonjour", "x"); err == nil {
		t.Fatal("doublon de nom accepté")
	}
	if _, err := d.SaveScript(ctx, id, "Bonjour", `print("salut")`); err != nil {
		t.Fatal(err)
	}
	list, _ := d.Scripts(ctx)
	if len(list) != 1 || list[0].Source != `print("salut")` {
		t.Fatalf("bibliothèque : %+v", list)
	}
	if err := d.DeleteScript(ctx, id); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestSort(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Pop"}, {"lyon", "520"}, {"Paris", "2100"}, {"nice", ""}, {"Brest", "140"}}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	if err := d.SortTable(ctx, "Feuille1", 0, false); err != nil {
		t.Fatal(err)
	}
	if got := view(t, d); got != lines("[Ville]|[Pop]", "Brest|140", "lyon|520", "nice", "Paris|2100") {
		t.Fatalf("tri par ville :\n%s", got)
	}
	if err := d.SortTable(ctx, "Feuille1", 1, true); err != nil {
		t.Fatal(err)
	}
	if got := view(t, d); got != lines("[Ville]|[Pop]", "Paris|2100", "lyon|520", "Brest|140", "nice") {
		t.Fatalf("tri par population décroissante :\n%s", got)
	}
	check(t, d, "tri")
	// On peut encore insérer entre deux lignes triées.
	if err := d.InsertRow(ctx, "Feuille1", 1); err != nil {
		t.Fatal(err)
	}
	check(t, d, "insertion après tri")
	for range 3 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestStyles(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Pop"}, {"Lyon", "520"}, {"Paris", "2100"}, {"Nice", "340"}}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	// Toute la colonne Pop (lignes 1 à 3) : style de colonne.
	if err := d.ApplyStyle(ctx, 1, 1, 1, 3, 1, Style{"nf": "0,00", "al": "center"}); err != nil {
		t.Fatal(err)
	}
	// Une cellule (Paris) en gras, l'en-tête en couleur, une cellule libre.
	if err := d.ApplyStyle(ctx, 1, 2, 0, 2, 0, Style{"b": true}); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyStyle(ctx, 1, 0, 0, 0, 1, Style{"bg": "#ffcc00"}); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyStyle(ctx, 1, 6, 3, 6, 3, Style{"i": true}); err != nil {
		t.Fatal(err)
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	cols := l.Tables[0].Columns
	if cols[1].Style != `{"al":"center","nf":"0,00"}` || cols[0].HeaderStyle != `{"bg":"#ffcc00"}` {
		t.Fatalf("styles de colonne : %+v", cols)
	}
	if len(l.Styles) != 1 || l.Styles[0].Style != `{"i":true}` {
		t.Fatalf("styles libres : %+v", l.Styles)
	}
	// Le gras suit Paris quand on trie.
	if err := d.SortTable(ctx, "Feuille1", 0, false); err != nil {
		t.Fatal(err)
	}
	var rid int64
	d.db.Read().QueryRow(`SELECT rid FROM _slitable_styles WHERE rid > 0`).Scan(&rid)
	var ville string
	d.db.Read().QueryRow(`SELECT c_1 FROM t_1 WHERE id = ?`, rid).Scan(&ville)
	if ville != "Paris" {
		t.Fatalf("le gras est sur %q", ville)
	}
	// Retirer le gras.
	if err := d.ApplyStyle(ctx, 1, 3, 0, 3, 0, Style{"b": false}); err != nil {
		t.Fatal(err)
	}
	check(t, d, "styles")
	for range 6 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestFindReplace(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Note"}, {"Paris", "paris centre"}, {"Lyon", "1"}, {"PARIS", ""}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 6, 0, "voir Paris"); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	ms, err := d.Find(ctx, 1, "paris")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(ms) != "[{1 0} {1 1} {3 0} {6 0}]" {
		t.Fatalf("Find = %v", ms)
	}
	if ms, _ := d.Find(ctx, 1, "100%"); len(ms) != 0 {
		t.Fatalf("%% non protégé : %v", ms)
	}
	n, err := d.Replace(ctx, 1, "paris", "Marseille")
	if err != nil || n != 4 {
		t.Fatalf("Replace = %d, %v", n, err)
	}
	if got := view(t, d); !strings.Contains(got, "Marseille|Marseille centre") || !strings.Contains(got, "'voir Marseille") {
		t.Fatalf("feuille :\n%s", got)
	}
	check(t, d, "remplacer")
	d.bus.Undo(ctx)
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestRules(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Pop"}, {"Lyon", "520"}, {"Paris", "2100"}}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	rules := []Rule{{Op: ">", Value: "1000", Style: Style{"bg": "#ffc7ce", "fg": "#9c0006"}}, {Op: "empty", Style: Style{"bg": "#eeeeee"}}}
	if err := d.SetRules(ctx, "Feuille1", 1, rules); err != nil {
		t.Fatal(err)
	}
	if err := d.SetRules(ctx, "Feuille1", 1, []Rule{{Op: "~"}}); err == nil {
		t.Fatal("condition inconnue acceptée")
	}
	l, _ := d.Layout(ctx, 1)
	if got := l.Tables[0].Columns[1].Rules; !strings.Contains(got, `"op":">"`) || !strings.Contains(got, `"#ffc7ce"`) {
		t.Fatalf("règles : %s", got)
	}
	// Supprimer la colonne puis annuler rend ses règles.
	if err := d.DeleteColumn(ctx, "Feuille1", 1); err != nil {
		t.Fatal(err)
	}
	d.bus.Undo(ctx)
	l, _ = d.Layout(ctx, 1)
	if l.Tables[0].Columns[1].Rules == "" {
		t.Fatal("règles perdues après annulation de la suppression")
	}
	d.bus.Undo(ctx)
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestFilter(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Pays"}, {"Lyon", "FR"}, {"Rome", "IT"}, {"Paris", "FR"}, {"Nice", ""}}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	vals, err := d.FilterValues(ctx, "Feuille1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(vals) != "[{ 1} {FR 2} {IT 1}]" {
		t.Fatalf("valeurs : %v", vals)
	}
	if err := d.SetFilter(ctx, "Feuille1", 1, &ColumnFilter{Values: []string{"FR"}}); err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, 1)
	tb := l.Tables[0]
	if tb.Rows != 2 || tb.Total != 4 || tb.Filter == "" {
		t.Fatalf("filtré : %d/%d %q", tb.Rows, tb.Total, tb.Filter)
	}
	// La 2e ligne affichée est Paris : la saisie y va.
	if _, err := d.SetCell(ctx, 1, 2, 0, "Paris 2"); err != nil {
		t.Fatal(err)
	}
	var got string
	d.read(ctx, func(t *tx) error { return t.row(`SELECT c_1 FROM t_1 WHERE id = 3`).Scan(&got) })
	if got != "Paris 2" {
		t.Fatalf("saisie sur la ligne filtrée : %q", got)
	}
	// Pas d'ajout de ligne dans un tableau filtré, ni dans les lignes masquées.
	if _, err := d.SetCell(ctx, 1, 3, 0, "Brest"); !errors.As(err, new(ErrFiltered)) {
		t.Fatalf("saisie dans les lignes masquées : %v", err)
	}
	if err := d.InsertRow(ctx, "Feuille1", 0); !errors.As(err, new(ErrFiltered)) {
		t.Fatalf("insertion : %v", err)
	}
	// Supprimer la ligne affichée 0 supprime Lyon.
	if err := d.DeleteRow(ctx, "Feuille1", 0); err != nil {
		t.Fatal(err)
	}
	var n int
	d.read(ctx, func(t *tx) error { return t.row(`SELECT count(*) FROM t_1 WHERE c_1 = 'Lyon'`).Scan(&n) })
	if n != 0 {
		t.Fatal("Lyon non supprimée")
	}
	d.bus.Undo(ctx)
	// Les vides se filtrent par "".
	if err := d.SetFilter(ctx, "Feuille1", 1, &ColumnFilter{Values: []string{""}}); err != nil {
		t.Fatal(err)
	}
	l, _ = d.Layout(ctx, 1)
	if l.Tables[0].Rows != 1 {
		t.Fatalf("vides : %d", l.Tables[0].Rows)
	}
	if err := d.ClearFilters(ctx, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	l, _ = d.Layout(ctx, 1)
	if l.Tables[0].Rows != 4 || l.Tables[0].Filter != "" {
		t.Fatalf("sans filtre : %d %q", l.Tables[0].Rows, l.Tables[0].Filter)
	}
	for range 4 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestFilterErrors(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"N"}, {"1"}, {"2"}, {"3"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.FixType(ctx, "Feuille1", 0); err != nil {
		t.Skip("FixType :", err)
	}
	if _, err := d.SetCell(ctx, 1, 2, 0, "deux"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetFilter(ctx, "Feuille1", 0, &ColumnFilter{Errors: true}); err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, 1)
	if l.Tables[0].Rows != 1 {
		t.Fatalf("erreurs seulement : %d", l.Tables[0].Rows)
	}
}

// Les colonnes calculées se lisent aussi à travers la vue filtrée.
func TestFilterComputed(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Qté", "Prix", "Total"}, {"2", "10", "x"}, {"3", "5", "y"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 2, "=[@Qté]*[@Prix]"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetFilter(ctx, "Feuille1", 0, &ColumnFilter{Values: []string{"3"}}); err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, 1)
	ti := l.Tables[0]
	exprs, args, _, err := d.ColumnExprs(ctx, ti.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got any
	err = d.read(ctx, func(t *tx) error {
		return t.row(`SELECT `+exprs[2]+` FROM `+ti.Source, args...).Scan(&got)
	})
	if err != nil || fmt.Sprint(got) != "15" {
		t.Fatalf("total filtré : %v (%v)", got, err)
	}
}

func TestTablesMoveMergeSplit(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Pop"}, {"Lyon", "520"}, {"Paris", "2100"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Paste(ctx, 1, 0, 5, [][]string{{"Pays", "Note"}, {"FR", "x"}, {"IT", "y"}, {"DE", "z"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.SetFilter(ctx, "Tableau1", 0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 2, 6, "zz"); err != nil {
		t.Fatal(err)
	}
	check(t, d, "deux tableaux")
	start := dump(t, d)

	// Renommer : identifiant unique.
	if err := d.RenameTable(ctx, "Tableau1", "Pays Note"); !errors.Is(err, ErrTable) {
		t.Fatalf("nom avec espace : %v", err)
	}
	if err := d.RenameTable(ctx, "Tableau1", "feuille1"); !errors.Is(err, ErrTable) {
		t.Fatalf("nom pris : %v", err)
	}
	if err := d.RenameTable(ctx, "Tableau1", "Pays"); err != nil {
		t.Fatal(err)
	}

	// Déplacer sur l'autre tableau : refusé ; côte à côte : fusion.
	if err := d.MoveTable(ctx, "Pays", 1, 1); !errors.Is(err, ErrTable) {
		t.Fatalf("recouvrement : %v", err)
	}
	if err := d.MoveTable(ctx, "Pays", 0, 2); err != nil {
		t.Fatal(err)
	}
	check(t, d, "fusion")
	want := lines(
		"[Ville]|[Pop]|[Pays]|[Note]",
		"Lyon|520|FR|x",
		"Paris|2100|IT|zz",
		"||DE|z")
	if got := view(t, d); !strings.Contains(got, "[Ville]|[Pop]|[Pays]|[Note]") || !strings.Contains(got, "Paris|2100|IT|zz") || !strings.Contains(got, "\n||DE|z") {
		t.Fatalf("fusion :\n%s\nattendu à peu près\n%s", got, want)
	}

	// Scinder : Pays et Note repartent en dessous.
	if err := d.ExtractColumns(ctx, "Feuille1", 2, 3, 6, 0); err != nil {
		t.Fatal(err)
	}
	check(t, d, "scission")
	l, _ := d.Layout(ctx, 1)
	if len(l.Tables) != 2 || len(l.Tables[0].Columns) != 2 || l.Tables[1].Row != 6 || l.Tables[1].Rows != 3 {
		t.Fatalf("scission : %+v", l.Tables)
	}

	// Supprimer les deux tableaux puis tout annuler.
	if err := d.DeleteTables(ctx, []string{"Feuille1", l.Tables[1].Name}); err != nil {
		t.Fatal(err)
	}
	check(t, d, "suppression")
	if l, _ := d.Layout(ctx, 1); len(l.Tables) != 0 {
		t.Fatalf("tableaux restants : %d", len(l.Tables))
	}
	// Un nouveau tableau peut reprendre un identifiant libéré.
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"A"}, {"1"}}); err != nil {
		t.Fatal(err)
	}
	check(t, d, "nouveau tableau")
	for range 5 {
		if _, err := d.bus.Undo(ctx); err != nil {
			t.Fatal(err)
		}
	}
	check(t, d, "annulation")
	if got := dump(t, d); got != start {
		t.Fatalf("annulation incomplète :\n%s", diff(start, got))
	}
	for range 5 {
		if _, err := d.bus.Redo(ctx); err != nil {
			t.Fatal(err)
		}
	}
	check(t, d, "rétablissement")
}

// Saisir dans la colonne qui sépare deux tableaux côte à côte les fusionne.
func TestTypeBetweenTablesMerges(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"A"}, {"1"}, {"2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Paste(ctx, 1, 0, 2, [][]string{{"C"}, {"x"}, {"y"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 1, "B"); err != nil {
		t.Fatal(err)
	}
	check(t, d, "fusion par saisie")
	l, _ := d.Layout(ctx, 1)
	if len(l.Tables) != 1 || len(l.Tables[0].Columns) != 3 || l.Tables[0].Columns[2].Name != "C" {
		t.Fatalf("fusion par saisie : %+v", l.Tables)
	}
}

// SOUS.TOTAL ne compte que les lignes que le filtre laisse affichées.
func TestSubtotalFilter(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes"}, {"FR", "10"}, {"IT", "5"}, {"FR", "7"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 4, `=SOUS.TOTAL(9;Feuille1[Ventes])`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 4, `=SOMME(Feuille1[Ventes])`); err != nil {
		t.Fatal(err)
	}
	value := func(row int) string {
		l, _ := d.Layout(ctx, 1)
		for _, c := range l.Cells {
			if c.Row == row && c.Col == 4 {
				return c.Value
			}
		}
		return "?"
	}
	if value(0) != "22" {
		t.Fatalf("sans filtre : %s", value(0))
	}
	if err := d.SetFilter(ctx, "Feuille1", 0, &ColumnFilter{Values: []string{"FR"}}); err != nil {
		t.Fatal(err)
	}
	if value(0) != "17" || value(1) != "22" {
		t.Fatalf("filtré : SOUS.TOTAL %s, SOMME %s", value(0), value(1))
	}
}

// ALEA garde sa valeur d'une lecture à l'autre et change après Recalc (F9).
func TestRecalc(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.SetCell(ctx, 1, 0, 0, "=ALEA()"); err != nil {
		t.Fatal(err)
	}
	value := func() string {
		l, _ := d.Layout(ctx, 1)
		return l.Cells[0].Value
	}
	a := value()
	if b := value(); a != b {
		t.Fatalf("ALEA instable : %s puis %s", a, b)
	}
	d.Recalc()
	if c := value(); c == a {
		t.Fatalf("ALEA inchangé après F9 : %s", c)
	}
}

// FILTRE dans une cellule libre se déverse ; une cellule sur le chemin bloque.
func TestSpill(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes"}, {"FR", "10"}, {"IT", "5"}, {"FR", "7"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 4, `=FILTRE(Feuille1;Feuille1[Pays]="FR")`); err != nil {
		t.Fatal(err)
	}
	spill := func() CellInfo {
		l, _ := d.Layout(ctx, 1)
		for _, c := range l.Cells {
			if c.Row == 0 && c.Col == 4 {
				return c
			}
		}
		return CellInfo{}
	}
	c := spill()
	if fmt.Sprint(c.Spill) != "[[FR 10] [FR 7]]" || c.Error {
		t.Fatalf("déversement : %+v", c)
	}
	if _, err := d.SetCell(ctx, 1, 1, 5, "x"); err != nil {
		t.Fatal(err)
	}
	if c := spill(); c.Value != ErrSpillText || c.Spill != nil {
		t.Fatalf("déversement bloqué : %+v", c)
	}
}

func TestRemoveDuplicates(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Pays"}, {"Lyon", "FR"}, {"lyon", "FR"}, {"Rome", "IT"}, {"Lyon", "BE"}, {"Rome", "IT"}}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	n, err := d.RemoveDuplicates(ctx, "Feuille1", nil)
	if err != nil || n != 2 {
		t.Fatalf("doublons : %d, %v", n, err)
	}
	check(t, d, "doublons")
	if got := view(t, d); !strings.Contains(got, "Lyon|FR\nRome|IT\nLyon|BE\n") {
		t.Fatalf("après :\n%s", got)
	}
	d.bus.Undo(ctx)
	if n, _ := d.RemoveDuplicates(ctx, "Feuille1", []int{0}); n != 3 {
		t.Fatalf("doublons sur Ville : %d", n)
	}
	d.bus.Undo(ctx)
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
}

func TestPivot(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Année", "Ventes"}, {"FR", "2024", "10"}, {"IT", "2024", "5"}, {"FR", "2025", "7"}, {"FR", "2024", "1"}}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	if _, err := d.CreatePivot(ctx, PivotConfig{Source: "Feuille1", Rows: "Pays", Agg: "sum"}); err == nil {
		t.Fatal("somme sans valeur acceptée")
	}
	sheet, err := d.CreatePivot(ctx, PivotConfig{Source: "Feuille1", Rows: "Pays", Cols: "Année", Value: "Ventes", Agg: "sum"})
	if err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, sheet)
	if len(l.Pivots) != 1 {
		t.Fatalf("tableaux croisés : %+v", l.Pivots)
	}
	if got := fmt.Sprint(l.Pivots[0].Cells); got != "[[Pays 2024 2025 Total] [FR 11 7 18] [IT 5  5] [Total 16 7 23]]" {
		t.Fatalf("TCD : %s", got)
	}
	// La source change : le TCD suit.
	if _, err := d.SetCell(ctx, 1, 2, 2, "50"); err != nil {
		t.Fatal(err)
	}
	l, _ = d.Layout(ctx, sheet)
	if got := fmt.Sprint(l.Pivots[0].Cells[2]); got != "[IT 50  50]" {
		t.Fatalf("après modification : %s", got)
	}
	if err := d.SetPivot(ctx, l.Pivots[0].ID, PivotConfig{Source: "Feuille1", Rows: "Année", Agg: "count"}); err != nil {
		t.Fatal(err)
	}
	l, _ = d.Layout(ctx, sheet)
	if got := fmt.Sprint(l.Pivots[0].Cells); got != "[[Année Nombre] [2024 3] [2025 1] [Total 4]]" {
		t.Fatalf("nombre : %s", got)
	}
	for range 3 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
}
