package canvas

import (
	"context"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

// Tapé sans en-têtes (demande de Louis) : que des textes, la 1re ligne
// ressemble aux suivantes ; toutes les lignes sont des données sous un
// en-tête « Colonne n » posé au-dessus. « Utiliser la première ligne comme
// en-têtes » la remet en en-tête, à sa place ; les formules suivent ; une
// étape.
func TestTypedUnnamed(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	mustSet(t, d, 1, 2, 0, "Nom")
	mustSet(t, d, 1, 2, 1, "Ville")
	ev, err := d.SetCell(ctx, 1, 3, 0, "Léa")
	must(t, err)
	if ev.Created != "Feuille1" || !ev.Unnamed || !ev.Headless {
		t.Fatalf("naissance : %+v", ev)
	}
	typeIn(t, d, 3, 1, [][]string{{"Paris"}, {""}})
	typeIn(t, d, 4, 0, [][]string{{"Max", "Lyon"}})
	mustSet(t, d, 1, 9, 0, "=NBVAL(Feuille1[Colonne 2])")
	if got, want := layoutOf(t, d), "Feuille1@A2:Colonne 1|Colonne 2×3 A10==NBVAL(Feuille1[Colonne 2])"; got != want {
		t.Fatalf("tapé :\n got %s\nwant %s", got, want)
	}
	if v := cellOn(t, d, 1, 9, 0).Value; v != "3" {
		t.Fatalf("formule avant : %q", v)
	}
	before := dump(t, d)
	must(t, d.PromoteHeader(ctx, "Feuille1"))
	check(t, d, "1re ligne en en-têtes")
	if got, want := layoutOf(t, d), "Feuille1@A3:Nom|Ville×2 A10==NBVAL(Feuille1[Ville])"; got != want {
		t.Fatalf("après :\n got %s\nwant %s", got, want)
	}
	if v := cellOn(t, d, 1, 9, 0).Value; v != "2" {
		t.Fatalf("formule après : %q", v)
	}
	undoOnce(t, d, before, "1re ligne en en-têtes")
}

// Un texte au-dessus de nombres : la 1re ligne se distingue, elle est
// l'en-tête comme avant.
func TestTypedHeaderOverNumbers(t *testing.T) {
	d := open(t)
	typeIn(t, d, 2, 0, [][]string{{"Nom", "Âge"}, {"Léa", "31"}})
	if got, want := layoutOf(t, d), "Feuille1@A3:Nom|Âge×1"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	// Années au-dessus de nombres (reconnu comme au collage).
	d = open(t)
	typeIn(t, d, 2, 0, [][]string{{"Pays", "2024"}, {"France", "12"}})
	if got, want := layoutOf(t, d), "Feuille1@A3:Pays|2024×1"; got != want {
		t.Fatalf("années :\n got %s\nwant %s", got, want)
	}
}

// Pas de place au-dessus (ligne 1, titre) : la 1re ligne reste l'en-tête.
func TestTypedUnnamedNoRoom(t *testing.T) {
	d := open(t)
	typeIn(t, d, 0, 0, [][]string{{"Nom", "Ville"}, {"Léa", "Paris"}})
	if got, want := layoutOf(t, d), "Feuille1@A1:Nom|Ville×1"; got != want {
		t.Fatalf("ligne 1 :\n got %s\nwant %s", got, want)
	}
	d = open(t)
	typeIn(t, d, 0, 0, [][]string{{"Clients"}, {"Nom", "Ville"}, {"Léa", "Paris"}})
	if got, want := layoutOf(t, d), "Feuille1@A2:Nom|Ville×1 A1=Clients"; got != want {
		t.Fatalf("titre au-dessus :\n got %s\nwant %s", got, want)
	}
	// Collé : inchangé (« Colonne n » au-dessus d'un bloc tout en texte).
	d = open(t)
	if _, err := d.Paste(context.Background(), 1, 3, 0, [][]string{{"Nom", "Ville"}, {"Léa", "Paris"}}); err != nil {
		t.Fatal(err)
	}
	if got, want := layoutOf(t, d), "Feuille1@A3:Colonne 1|Colonne 2×2"; got != want {
		t.Fatalf("collé :\n got %s\nwant %s", got, want)
	}
}

// Un nombre tapé ensuite sous un texte de la 1re ligne d'un tableau tout
// juste tapé sans en-têtes : elle devient l'en-tête, à sa place.
func TestTypedUnnamedRehead(t *testing.T) {
	d := open(t)
	typeIn(t, d, 2, 0, [][]string{{"Nom", "Prénom", "Âge"}, {"Dupont", "Jean"}})
	if got, want := layoutOf(t, d), "Feuille1@A2:Colonne 1|Colonne 2|Colonne 3×2"; got != want {
		t.Fatalf("tapé :\n got %s\nwant %s", got, want)
	}
	before := dump(t, d)
	d.TakeNotice()
	mustSet(t, d, 1, 3, 2, "42")
	if got, want := layoutOf(t, d), "Feuille1@A3:Nom|Prénom|Âge×1"; got != want {
		t.Fatalf("après 42 :\n got %s\nwant %s", got, want)
	}
	if ts := tablesOf(t, d); ts[0].Columns[2].Type != grid.Integer {
		t.Fatalf("Âge : %s", ts[0].Columns[2].Type)
	}
	if n := d.TakeNotice(); !strings.Contains(n, "prise comme en-têtes") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "en-têtes repris")
	// Un texte de plus ne change rien (« Petit » : pas de même longueur que
	// « Dupont », qui voterait pour un en-tête comme au collage).
	mustSet(t, d, 1, 4, 0, "Petit")
	if got := layoutOf(t, d); !strings.HasPrefix(got, "Feuille1@A2:Colonne 1|") {
		t.Fatalf("texte : %s", got)
	}
}

// « Utiliser la première ligne comme en-têtes » : une colonne passée en
// texte à cause de cette ligne reprend son type ; tout autre nom est
// remplacé.
func TestPromoteHeaderRetypes(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		_, err := x.createTable(1, "T", 1, 0, []string{"A", "B"}, [][]string{{"Nom", "Âge"}, {"Léa", "31"}, {"Max", "40"}})
		return err
	}))
	if ts := tablesOf(t, d); ts[0].Columns[1].Type != grid.Text {
		t.Fatalf("avant : %s", ts[0].Columns[1].Type)
	}
	before := dump(t, d)
	must(t, d.PromoteHeader(ctx, "T"))
	check(t, d, "promu")
	if got, want := layoutOf(t, d), "T@A3:Nom|Âge×2"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	if ts := tablesOf(t, d); ts[0].Columns[1].Type != grid.Integer {
		t.Fatalf("Âge : %s", ts[0].Columns[1].Type)
	}
	undoOnce(t, d, before, "promu")
	// Noms échangés (« B | A ») : chaque colonne garde ses formules.
	mustSet(t, d, 1, 9, 0, "=NBVAL(T[A])")
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		tb, err := x.tableByName("T")
		if err != nil {
			return err
		}
		return x.setCell(tb, 0, 0, "B", &Event{})
	}))
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		tb, err := x.tableByName("T")
		if err != nil {
			return err
		}
		return x.setCell(tb, 0, 1, "A", &Event{})
	}))
	must(t, d.PromoteHeader(ctx, "T"))
	if got, want := layoutOf(t, d), "T@A3:B|A×2 A10==NBVAL(T[B])"; got != want {
		t.Fatalf("échange :\n got %s\nwant %s", got, want)
	}
}

// Noms « Colonne n » d'un tableau tapé sans en-têtes : comptés comme donnés par Lattis (dataHeader) ; des
// noms tapés juste au-dessus sont proposés comme en-têtes, et y entrent sans
// ligne en plus.
func TestUnnamedHeaderOffer(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		_, err := x.createTable(1, "T", 1, 0, autoHeaders(2), [][]string{{"Léa", "Paris"}, {"Max", "Lyon"}})
		return err
	}))
	mustSet(t, d, 1, 0, 0, "Nom")
	ev, err := d.SetCell(ctx, 1, 0, 1, "Ville")
	if err != nil || ev.HeaderOffer != "T" {
		t.Fatalf("proposition : %q, %v", ev.HeaderOffer, err)
	}
	must(t, d.TitleToHeader(ctx, "T"))
	if got, want := layoutOf(t, d), "T@A1:Nom|Ville×2"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	// Une ligne plus large que le tableau y entre d'elle-même (headerOver) :
	// les « Colonne n » ne deviennent pas une ligne.
	d = open(t)
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		_, err := x.createTable(1, "T", 1, 0, autoHeaders(2), [][]string{{"Léa", "Paris"}, {"Max", "Lyon"}})
		return err
	}))
	typeIn(t, d, 0, 0, [][]string{{"Nom", "Ville", "Pays"}})
	ts := tablesOf(t, d)
	if len(ts) != 1 || colNames(ts[0]) != "Nom|Ville|Pays" || ts[0].Rows != 2 {
		t.Fatalf("ligne plus large : %s", layoutOf(t, d))
	}
}
