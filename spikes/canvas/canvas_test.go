package canvas

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/slitable/internal/grid"
)

func open(t *testing.T) *Doc {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "doc.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func view(t *testing.T, d *Doc) string {
	t.Helper()
	v, err := d.View(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func dump(t *testing.T, d *Doc) string {
	t.Helper()
	s, err := d.Dump(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func check(t *testing.T, d *Doc, when string) {
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
	do   func(context.Context, *Doc) error
	want string
}

func set(row, col int, raw string) func(context.Context, *Doc) error {
	return func(ctx context.Context, d *Doc) error {
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
		{"insertion au milieu : le tableau du dessous descend", func(ctx context.Context, d *Doc) error {
			return d.InsertRow(ctx, "Feuille 1", 1)
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
		{"supprimer la colonne Prix", func(ctx context.Context, d *Doc) error {
			return d.DeleteColumn(ctx, "Feuille 1", 1)
		}, lines(
			"[Article]|[Stock]",
			"Clavier|3",
			"Écran|10",
			"Souris|beaucoup",
			"",
			"[Ville]",
			"Paris")},
		{"insérer une colonne au milieu", func(ctx context.Context, d *Doc) error {
			return d.InsertColumn(ctx, "Feuille 1", 1, "Date")
		}, ""},
		{"date française", set(1, 1, "15/10/2026"), ""},
		{"supprimer une ligne", func(ctx context.Context, d *Doc) error {
			return d.DeleteRow(ctx, "Feuille 1", 1)
		}, lines(
			"[Article]|[Date]|[Stock]",
			"Clavier|2026-10-15|3",
			"Souris||beaucoup",
			"",
			"",
			"[Ville]",
			"Paris")},
		{"fixer le type de Ville", func(ctx context.Context, d *Doc) error { return d.FixType(ctx, "Tableau 1", 0) }, ""},
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
		if _, err := d.Undo(ctx); err != nil {
			t.Fatalf("annuler %q : %v", steps[i].name, err)
		}
		check(t, d, "après annulation de "+steps[i].name)
		if got := dump(t, d); got != states[i] {
			t.Fatalf("annuler %q ne restaure pas l'état précédent :\n%s", steps[i].name, diff(states[i], got))
		}
	}
	if d.CanUndo() {
		t.Fatal("la pile d'annulation devrait être vide")
	}
	for i := range steps {
		if _, err := d.Redo(ctx); err != nil {
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
	if err := d.FixType(ctx, "Feuille 1", 0); err != nil {
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
	d.Undo(ctx)
	d.Undo(ctx)
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
	if err != nil || ev.Widened != "Feuille 1.Prix" || ev.From != grid.Integer || ev.To != grid.Decimal {
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
