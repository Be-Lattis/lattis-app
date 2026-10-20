package canvas

import (
	"context"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

// Règle de Louis (20/10/2026, décision 0005) : la 1re ligne tapée est
// l'en-tête, même si elle ressemble aux suivantes (que des textes).
func TestTypedTextFirstRowIsHeader(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	mustSet(t, d, 1, 2, 0, "Nom")
	mustSet(t, d, 1, 2, 1, "Ville")
	ev, err := d.SetCell(ctx, 1, 3, 0, "Léa")
	must(t, err)
	if ev.Created != "Feuille1" || ev.Unnamed || ev.Headless {
		t.Fatalf("naissance : %+v", ev)
	}
	typeIn(t, d, 3, 1, [][]string{{"Paris"}})
	typeIn(t, d, 4, 0, [][]string{{"Max", "Lyon"}})
	if got, want := layoutOf(t, d), "Feuille1@A3:Nom|Ville×2"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	// Un texte au-dessus de nombres : l'en-tête aussi.
	d = open(t)
	typeIn(t, d, 2, 0, [][]string{{"Nom", "Âge"}, {"Léa", "31"}})
	if got, want := layoutOf(t, d), "Feuille1@A3:Nom|Âge×1"; got != want {
		t.Fatalf("texte sur nombres :\n got %s\nwant %s", got, want)
	}
	// En ligne 1 : de même.
	d = open(t)
	typeIn(t, d, 0, 0, [][]string{{"Nom", "Ville"}, {"Léa", "Paris"}})
	if got, want := layoutOf(t, d), "Feuille1@A1:Nom|Ville×1"; got != want {
		t.Fatalf("ligne 1 :\n got %s\nwant %s", got, want)
	}
	// Sous un titre gardé libre : de même.
	d = open(t)
	typeIn(t, d, 0, 0, [][]string{{"Clients"}, {"Nom", "Ville"}, {"Léa", "Paris"}})
	if got, want := layoutOf(t, d), "Feuille1@A2:Nom|Ville×1 A1=Clients"; got != want {
		t.Fatalf("titre au-dessus :\n got %s\nwant %s", got, want)
	}
	// Un nombre forcé en texte par l'apostrophe ('2024) reste un nom.
	d = open(t)
	typeIn(t, d, 2, 0, [][]string{{"Pays", "'2024"}, {"France", "12"}})
	if ts := tablesOf(t, d); len(ts) != 1 || ts[0].Row != 2 || ts[0].Columns[0].Name != "Pays" {
		t.Fatalf("apostrophe : %s", layoutOf(t, d))
	}
}

// Une valeur qui n'est pas un texte dans la 1re ligne (« Léa | 31 ») : toutes
// les lignes tapées sont des données, sous un en-tête « Colonne n » posé
// au-dessus. « Utiliser la première ligne comme en-têtes » la remet en
// en-tête, à sa place ; les formules suivent ; une étape.
func TestTypedMixedFirstRowIsData(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	mustSet(t, d, 1, 2, 0, "Léa")
	mustSet(t, d, 1, 2, 1, "31")
	ev, err := d.SetCell(ctx, 1, 3, 0, "Max")
	must(t, err)
	if ev.Created != "Feuille1" || !ev.Unnamed || !ev.Headless {
		t.Fatalf("naissance : %+v", ev)
	}
	typeIn(t, d, 3, 1, [][]string{{"40"}})
	typeIn(t, d, 4, 0, [][]string{{"Zoé", "25"}})
	mustSet(t, d, 1, 9, 0, "=SOMME(Feuille1[Colonne 2])")
	if got, want := layoutOf(t, d), "Feuille1@A2:Colonne 1|Colonne 2×3 A10==SOMME(Feuille1[Colonne 2])"; got != want {
		t.Fatalf("tapé :\n got %s\nwant %s", got, want)
	}
	if ts := tablesOf(t, d); ts[0].Columns[1].Type != grid.Integer {
		t.Fatalf("type : %s", ts[0].Columns[1].Type)
	}
	if v := cellOn(t, d, 1, 9, 0).Value; v != "96" {
		t.Fatalf("formule avant : %q", v)
	}
	before := dump(t, d)
	must(t, d.PromoteHeader(ctx, "Feuille1"))
	check(t, d, "1re ligne en en-têtes")
	if got, want := layoutOf(t, d), "Feuille1@A3:Léa|31×2 A10==SOMME(Feuille1[31])"; got != want {
		t.Fatalf("après :\n got %s\nwant %s", got, want)
	}
	if v := cellOn(t, d, 1, 9, 0).Value; v != "65" {
		t.Fatalf("formule après : %q", v)
	}
	undoOnce(t, d, before, "1re ligne en en-têtes")
}

// Années, dates, heures, booléens, pourcentages, montants, nombres seuls dans
// la 1re ligne : une donnée. Sans aucun texte, la barre d'état ne propose pas
// d'en faire l'en-tête (Headless, pas Unnamed).
func TestTypedNonTextFirstRow(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		rows    [][]string
		unnamed bool
	}{
		{"années", [][]string{{"Pays", "2024"}, {"France", "12"}}, true},
		{"date", [][]string{{"Réunion", "21/10/2026"}, {"Bilan", "22/10/2026"}}, true},
		{"heure", [][]string{{"Départ", "10:30"}, {"Arrivée", "12:00"}}, true},
		{"booléen", [][]string{{"Payé", "VRAI"}, {"Dû", "FAUX"}}, true},
		{"pourcentage", [][]string{{"TVA", "20 %"}, {"Réduite", "5,5 %"}}, true},
		{"montant", [][]string{{"Loyer", "800 €"}, {"Charges", "120 €"}}, true},
		{"nombres seuls", [][]string{{"10", "20"}, {"30", "40"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := open(t)
			mustSet(t, d, 1, 2, 0, tc.rows[0][0])
			mustSet(t, d, 1, 3, 0, tc.rows[1][0])
			ev, err := d.SetCell(ctx, 1, 2, 1, tc.rows[0][1])
			must(t, err)
			mustSet(t, d, 1, 3, 1, tc.rows[1][1])
			if ev.Created == "" || ev.Unnamed != tc.unnamed || !ev.Headless {
				t.Fatalf("naissance : %+v", ev)
			}
			if ts := tablesOf(t, d); len(ts) != 1 || ts[0].Row != 1 || ts[0].Rows != 2 || colNames(ts[0]) != "Colonne 1|Colonne 2" {
				t.Fatalf("%s", layoutOf(t, d))
			}
		})
	}
}

// Pas de place pour l'en-tête au-dessus d'une 1re ligne de données (ligne 1,
// cellule au-dessus) : les cellules restent libres, comme une liste ;
// « Mettre sous forme de tableau » les reprend. Un mot seul au-dessus de
// « Léa | 31 » n'est pas un titre (rien ne ressemble à un en-tête dessous) :
// c'est la 1re ligne de l'îlot, un texte, donc l'en-tête (comme avant).
func TestTypedMixedFirstRowNoRoom(t *testing.T) {
	d := open(t)
	typeIn(t, d, 0, 0, [][]string{{"Léa", "31"}, {"Max", "40"}})
	if got, want := layoutOf(t, d), "A1=Léa B1=31 A2=Max B2=40"; got != want {
		t.Fatalf("ligne 1 :\n got %s\nwant %s", got, want)
	}
	d = open(t)
	typeIn(t, d, 0, 0, [][]string{{"Clients"}, {"Léa", "31"}, {"Max", "40"}})
	if got, want := layoutOf(t, d), "Feuille1@A1:Clients|Colonne 2×2"; got != want {
		t.Fatalf("mot au-dessus :\n got %s\nwant %s", got, want)
	}
	d = open(t)
	typeIn(t, d, 2, 2, [][]string{{"Note"}})
	typeIn(t, d, 3, 0, [][]string{{"Léa", "31"}, {"Max", "40"}})
	if l := tablesOf(t, d); len(l) != 0 {
		t.Fatalf("cellule au-dessus : %s", layoutOf(t, d))
	}
}

// Une seule colonne tapée, un nombre en tête : une liste, pas de tableau
// (règle du carré de 2 × 2, inchangée).
func TestTypedSingleColumnNumber(t *testing.T) {
	d := open(t)
	typeIn(t, d, 2, 0, [][]string{{"42"}, {"43"}, {"44"}})
	if l := tablesOf(t, d); len(l) != 0 {
		t.Fatalf("%s", layoutOf(t, d))
	}
}

// La règle se décide sur la 1re ligne seule : un nombre tapé ensuite sous un
// nom de l'en-tête ne change rien (plus de relecture), la colonne prend son
// type.
func TestTypedHeaderNoRehead(t *testing.T) {
	d := open(t)
	typeIn(t, d, 2, 0, [][]string{{"Nom", "Prénom", "Âge"}, {"Dupont", "Jean"}})
	if got, want := layoutOf(t, d), "Feuille1@A3:Nom|Prénom|Âge×1"; got != want {
		t.Fatalf("tapé :\n got %s\nwant %s", got, want)
	}
	mustSet(t, d, 1, 3, 2, "42")
	if got, want := layoutOf(t, d), "Feuille1@A3:Nom|Prénom|Âge×1"; got != want {
		t.Fatalf("après 42 :\n got %s\nwant %s", got, want)
	}
	if ts := tablesOf(t, d); ts[0].Columns[2].Type != grid.Integer {
		t.Fatalf("Âge : %s", ts[0].Columns[2].Type)
	}
}

// Dans la langue de la session : « 10/21/2026 » ou « 1,234.5 » tapés en
// anglais sont une date et un nombre (lus avant la règle) ; « Name » et
// « Date » restent des textes.
func TestTypedFirstRowSessionLocale(t *testing.T) {
	d := open(t)
	speak(d, "en")
	typeIn(t, d, 2, 0, [][]string{{"Meeting", "10/21/2026"}, {"Review", "10/22/2026"}})
	if ts := tablesOf(t, d); len(ts) != 1 || ts[0].Row != 1 || ts[0].Columns[1].Type != grid.Date {
		t.Fatalf("date : %s", layoutOf(t, d))
	}
	d = open(t)
	speak(d, "en")
	typeIn(t, d, 2, 0, [][]string{{"Rent", "1,234.5"}, {"Fees", "12.5"}})
	if ts := tablesOf(t, d); len(ts) != 1 || ts[0].Row != 1 || ts[0].Columns[1].Type != grid.Decimal {
		t.Fatalf("nombre : %s", layoutOf(t, d))
	}
	d = open(t)
	speak(d, "en")
	typeIn(t, d, 2, 0, [][]string{{"Name", "Date"}, {"Lea", "10/21/2026"}})
	if ts := tablesOf(t, d); len(ts) != 1 || ts[0].Row != 2 || colNames(ts[0]) != "Name|Date" {
		t.Fatalf("en-tête : %s", layoutOf(t, d))
	}
}

// Collage d'un bloc : sa propre règle (hasHeader), inchangée ; un bloc tout
// en texte dont la 1re ligne ressemble aux suivantes reste sans en-têtes.
func TestPastedBlockKeepsItsRule(t *testing.T) {
	d := open(t)
	if _, err := d.Paste(context.Background(), 1, 3, 0, [][]string{{"Nom", "Ville"}, {"Léa", "Paris"}}); err != nil {
		t.Fatal(err)
	}
	if got, want := layoutOf(t, d), "Feuille1@A3:Colonne 1|Colonne 2×2"; got != want {
		t.Fatalf("collé :\n got %s\nwant %s", got, want)
	}
	d = open(t)
	if _, err := d.Paste(context.Background(), 1, 3, 0, [][]string{{"Pays", "2024"}, {"France", "12"}}); err != nil {
		t.Fatal(err)
	}
	if got, want := layoutOf(t, d), "Feuille1@A4:Pays|2024×1"; got != want {
		t.Fatalf("années collées :\n got %s\nwant %s", got, want)
	}
}

func TestTextRow(t *testing.T) {
	for _, tt := range []struct {
		row  []string
		want bool
	}{
		{[]string{"Nom", "Ville"}, true},
		{[]string{"", "Jan", "Fév"}, true},
		{[]string{"Pays", "'2024"}, true},
		{[]string{"Léa", "31"}, false},
		{[]string{"Pays", "2024"}, false},
		{[]string{"21/10/2026"}, false},
		{[]string{"x", "10:30"}, false},
		{[]string{"x", "15 %"}, false},
		{[]string{"x", "VRAI"}, false},
		{[]string{"x", "12 €"}, false},
		{[]string{"x", "2,5"}, false},
	} {
		if got := textRow(tt.row); got != tt.want {
			t.Errorf("textRow(%v) = %v", tt.row, got)
		}
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

// Noms « Colonne n » d'un tableau tapé sans en-têtes : comptés comme
// donnés par Lattis (dataHeader) ; des noms tapés juste au-dessus sont proposés comme en-têtes, et y entrent sans
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
