package canvas

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

// Audit de la détection des tableaux (revue/audit-detection-tableaux.md) :
// titres, notes, totaux et remarques restent hors des tableaux.

var ventesRows = [][]string{
	{"Produit", "Région", "Montant"},
	{"Pommes", "Nord", "120"},
	{"Poires", "Sud", "80"},
	{"Prunes", "Est", "45"},
}

func typeIn(t *testing.T, d *fixture, r0, c0 int, rows [][]string) {
	t.Helper()
	for i, r := range rows {
		for j, v := range r {
			if v != "" {
				mustSet(t, d, 1, r0+i, c0+j, v)
			}
		}
	}
}

// layoutOf résume la feuille : tableaux (nom@ancre:colonnes×lignes) et
// cellules libres (adresse=texte, * si gardée hors tableau).
func layoutOf(t *testing.T, d *fixture) string {
	t.Helper()
	l, err := d.Layout(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	outs := map[[2]int]bool{}
	for _, s := range l.Styles {
		if parseStyle(s.Style)[outKey] == true {
			outs[[2]int{s.Row, s.Col}] = true
		}
	}
	var parts []string
	for _, tb := range l.Tables {
		parts = append(parts, tb.Name+"@"+cellAddr(tb.Row, tb.Col)+":"+colNames(tb)+"×"+itoa(tb.Total))
	}
	for _, c := range l.Cells {
		mark := ""
		if outs[[2]int{c.Row, c.Col}] {
			mark = "*"
		}
		parts = append(parts, cellAddr(c.Row, c.Col)+mark+"="+c.Raw)
	}
	return strings.Join(parts, " ")
}

func cellAddr(r, c int) string {
	s := ""
	for c++; c > 0; c = (c - 1) / 26 {
		s = string(rune('A'+(c-1)%26)) + s
	}
	return s + itoa(r+1)
}

func TestLegendTyping(t *testing.T) {
	for _, tc := range []struct {
		name string
		play func(t *testing.T, d *fixture)
		want string
	}{
		{"titre tapé avant", func(t *testing.T, d *fixture) {
			mustSet(t, d, 1, 0, 0, "Ventes 2026")
			typeIn(t, d, 1, 0, ventesRows)
		}, "Feuille1@A2:Produit|Région|Montant×3 A1=Ventes 2026"},
		{"titre et sous-titre", func(t *testing.T, d *fixture) {
			mustSet(t, d, 1, 0, 0, "Ventes 2026")
			mustSet(t, d, 1, 1, 0, "Source : interne")
			typeIn(t, d, 2, 0, ventesRows)
		}, "Feuille1@A3:Produit|Région|Montant×3 A1=Ventes 2026 A2=Source : interne"},
		{"titre au-dessus d'une autre colonne", func(t *testing.T, d *fixture) {
			mustSet(t, d, 1, 0, 1, "Ventes 2026")
			typeIn(t, d, 1, 0, ventesRows)
		}, "Feuille1@A2:Produit|Région|Montant×3 B1=Ventes 2026"},
		{"en-tête sur deux lignes", func(t *testing.T, d *fixture) {
			typeIn(t, d, 0, 0, [][]string{{"", "2025", "", "2026", ""}, {"Région", "T1", "T2", "T1", "T2"}, {"Nord", "10", "12", "14", "15"}, {"Sud", "8", "9", "11", "10"}})
		}, "Feuille1@A2:Région|T1|T2|T1 2|T2 2×2 B1=2025 D1=2026"},
		{"note sous le tableau", func(t *testing.T, d *fixture) {
			typeIn(t, d, 0, 0, ventesRows)
			mustSet(t, d, 1, 4, 0, "Source : INSEE")
			mustSet(t, d, 1, 5, 0, "* hors taxes")
		}, "Feuille1@A1:Produit|Région|Montant×3 A5*=Source : INSEE A6*=* hors taxes"},
		{"remarque à droite", func(t *testing.T, d *fixture) {
			typeIn(t, d, 0, 0, ventesRows)
			mustSet(t, d, 1, 2, 3, "← à vérifier")
		}, "Feuille1@A1:Produit|Région|Montant×3 D3*=← à vérifier"},
		{"nouvelle colonne à droite (inchangé)", func(t *testing.T, d *fixture) {
			typeIn(t, d, 0, 0, ventesRows)
			mustSet(t, d, 1, 2, 3, "12")
		}, "Feuille1@A1:Produit|Région|Montant|Colonne 4×3"},
		{"nouvelle ligne (inchangé)", func(t *testing.T, d *fixture) {
			typeIn(t, d, 0, 0, ventesRows)
			mustSet(t, d, 1, 4, 0, "Kiwis")
		}, "Feuille1@A1:Produit|Région|Montant×4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := open(t)
			tc.play(t, d)
			if got := layoutOf(t, d); got != tc.want {
				t.Fatalf("\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

// Une ligne Total (libellé puis SOMME) reste sous le tableau : la colonne ne
// devient pas calculée, le total suit le tableau quand il grandit.
func TestLegendTotal(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	typeIn(t, d, 0, 0, ventesRows)
	mustSet(t, d, 1, 4, 1, "Total")
	mustSet(t, d, 1, 4, 2, "=SOMME([Montant])")
	ts := tablesOf(t, d)
	if len(ts) != 1 || ts[0].Total != 3 || ts[0].Columns[2].Formula != "" || ts[0].Columns[2].Type != grid.Integer {
		t.Fatalf("tableau = %+v", ts)
	}
	if c := cellOn(t, d, 1, 4, 2); c.Value != "245" {
		t.Fatalf("total = %+v", c)
	}
	// Une ligne de données tapée sur la ligne du total : le total descend,
	// toujours collé au tableau.
	mustSet(t, d, 1, 4, 0, "Kiwis")
	mustSet(t, d, 1, 4, 2, "30")
	if got, want := layoutOf(t, d), "Feuille1@A1:Produit|Région|Montant×4 B6*=Total C6*==SOMME(Feuille1[Montant])"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	if v := cellOn(t, d, 1, 5, 2).Value; v != "275" {
		t.Fatalf("total = %q", v)
	}
	must(t, d.InsertRow(ctx, "Feuille1", 4))
	if got := cellOn(t, d, 1, 6, 1).Raw; got != "Total" {
		t.Fatalf("après insertion, B7 = %q (%s)", got, layoutOf(t, d))
	}
}

// Un texte sous une colonne de nombres reste une note : la colonne garde son
// type.
func TestLegendNoteKeepsType(t *testing.T) {
	d := open(t)
	typeIn(t, d, 0, 0, ventesRows)
	mustSet(t, d, 1, 4, 2, "en milliers d'euros")
	ts := tablesOf(t, d)
	if ts[0].Total != 3 || ts[0].Columns[2].Type != grid.Integer {
		t.Fatalf("tableau = %+v", ts[0])
	}
}

func TestLegendPaste(t *testing.T) {
	ventes := func(extra ...[]string) [][]string {
		return append(append([][]string{}, ventesRows...), extra...)
	}
	for _, tc := range []struct {
		name  string
		block [][]string
		want  string
	}{
		{"titre, ligne vide, tableau, note", append(append([][]string{{"Ventes 2026", "", ""}, {"", "", ""}}, ventesRows...), []string{"", "", ""}, []string{"Source : INSEE", "", ""}),
			"Feuille1@A3:Produit|Région|Montant×3 A1=Ventes 2026 A8=Source : INSEE"},
		{"titre collé", append([][]string{{"Ventes 2026", "", ""}}, ventesRows...),
			"Feuille1@A2:Produit|Région|Montant×3 A1=Ventes 2026"},
		{"note collée", ventes([]string{"Source : INSEE", "", ""}),
			"Feuille1@A1:Produit|Région|Montant×3 A5*=Source : INSEE"},
		{"ligne Total", ventes([]string{"", "Total", "245"}),
			"Feuille1@A1:Produit|Région|Montant×3 B5*=Total C5*=245"},
		{"ligne vide au milieu", [][]string{ventesRows[0], ventesRows[1], {"", "", ""}, ventesRows[2], ventesRows[3]},
			"Feuille1@A1:Produit|Région|Montant×3"},
		{"deux tableaux côte à côte", [][]string{{"Produit", "Prix", "", "Ville", "Pop"}, {"Pommes", "2", "", "Lyon", "500"}, {"Poires", "3", "", "Nice", "340"}},
			"Feuille1@A1:Produit|Prix×2 Tableau1@D1:Ville|Pop×2"},
		{"deux tableaux empilés", [][]string{{"Produit", "Prix"}, {"Pommes", "2"}, {"", ""}, {"Ville", "Pop"}, {"Lyon", "500"}},
			"Feuille1@A1:Produit|Prix×1 Tableau1@A4:Ville|Pop×1"},
		{"en-têtes années", [][]string{{"Pays", "2024", "2025"}, {"France", "10", "11"}, {"Italie", "8", "9"}},
			"Feuille1@A1:Pays|2024|2025×2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := open(t)
			if _, err := d.Paste(context.Background(), 1, 0, 0, tc.block); err != nil {
				t.Fatal(err)
			}
			if got := layoutOf(t, d); got != tc.want {
				t.Fatalf("\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestLegendCommands(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	// Titre pris pour en-tête (titre, puis une seule colonne dessous : un
	// tableau d'avant la règle du carré de 2 × 2, ou relu).
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		_, err := x.createTable(1, "Feuille1", 0, 0, []string{"Ventes 2026"}, [][]string{{"Produit"}, {"Pommes"}})
		return err
	}))
	if got, want := layoutOf(t, d), "Feuille1@A1:Ventes 2026×2"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	must(t, d.HeaderToTitle(ctx, "Feuille1"))
	if got, want := layoutOf(t, d), "Feuille1@A2:Produit×1 A1=Ventes 2026"; got != want {
		t.Fatalf("en-tête → titre :\n got %s\nwant %s", got, want)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got, want := layoutOf(t, d), "Feuille1@A1:Ventes 2026×2"; got != want {
		t.Fatalf("annulé :\n got %s\nwant %s", got, want)
	}

	d = open(t)
	typeIn(t, d, 0, 0, append(append([][]string{}, ventesRows...), []string{"Les prix sont HT", "", ""}))
	if got, want := layoutOf(t, d), "Feuille1@A1:Produit|Région|Montant×4"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	if err := d.RowToNote(ctx, "Feuille1", 0); err == nil {
		t.Fatal("une ligne du milieu ne sort pas")
	}
	must(t, d.RowToNote(ctx, "Feuille1", 3))
	if got, want := layoutOf(t, d), "Feuille1@A1:Produit|Région|Montant×3 A5*=Les prix sont HT"; got != want {
		t.Fatalf("ligne → note :\n got %s\nwant %s", got, want)
	}
	must(t, d.JoinTable(ctx, 1, 4, 0))
	if got, want := layoutOf(t, d), "Feuille1@A1:Produit|Région|Montant×4"; got != want {
		t.Fatalf("ajoutée au tableau :\n got %s\nwant %s", got, want)
	}
}

// Sortir l'en-tête d'un tableau sans en-têtes (« Colonne n ») : la 1re ligne
// devient l'en-tête, sans titre ni ligne vide au-dessus.
func TestHeaderToTitleAutoNames(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		_, err := x.createTable(1, "T", 0, 0, []string{"Colonne 1", "Colonne 2"}, [][]string{{"Nom", "Âge"}, {"Léa", "31"}})
		return err
	}))
	must(t, d.HeaderToTitle(ctx, "T"))
	if got, want := layoutOf(t, d), "T@A1:Nom|Âge×1"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

// queryTable charge en A1 de la feuille 1 le tableau « Q » d'une requête
// (Produit, Montant) ; rows : ses lignes. Rend de quoi l'actualiser.
func queryTable(t *testing.T, d *fixture, rows [][]any) func(rows [][]any) {
	t.Helper()
	ctx := context.Background()
	id, err := d.SaveQuery(ctx, QueryDef{Name: "Q", Source: `{"kind":"table","table":"X"}`})
	must(t, err)
	load := func(rows [][]any) {
		t.Helper()
		it := resultTable("", []string{"Produit", "Montant"}, rows)
		it.At = &QueryAnchor{Sheet: 1}
		_, err := d.LoadQuery(ctx, id, it)
		must(t, err)
	}
	load(rows)
	return load
}

// Audit 2, I10 : sous un tableau chargé par une requête, un total, une note
// ou une remarque à côté d'une ligne restent permis (cellules gardées à
// part) ; une nouvelle ligne ou colonne est refusée, avec le message de
// l'interface. L'actualisation ne recouvre pas ce qui est gardé à part.
func TestQueryTableLegend(t *testing.T) {
	ctx := context.Background()
	rows := [][]any{{"Pommes", int64(120)}, {"Poires", int64(80)}}
	d := open(t)
	refresh := queryTable(t, d, rows)
	if got, want := layoutOf(t, d), "Q@A1:Produit|Montant×2"; got != want {
		t.Fatalf("chargé :\n got %s\nwant %s", got, want)
	}

	// Note et formule de total dessous.
	d3 := open(t)
	refresh3 := queryTable(t, d3, rows)
	mustSet(t, d3, 1, 3, 0, "Source : ERP")
	mustSet(t, d3, 1, 3, 1, "=SOMME([Montant])")
	if got, want := layoutOf(t, d3), "Q@A1:Produit|Montant×2 A4*=Source : ERP B4*==SOMME(Q[Montant])"; got != want {
		t.Fatalf("note et formule :\n got %s\nwant %s", got, want)
	}
	if v := cellOn(t, d3, 1, 3, 1).Value; v != "200" {
		t.Fatalf("total : %q", v)
	}

	// « Total » dessous, puis sa valeur à droite (une donnée, gardée sur la
	// ligne du total) ; remarque à côté d'une ligne.
	mustSet(t, d, 1, 3, 0, "Total")
	mustSet(t, d, 1, 3, 1, "245")
	mustSet(t, d, 1, 1, 2, "← à vérifier")
	if got, want := layoutOf(t, d), "Q@A1:Produit|Montant×2 C2*=← à vérifier A4*=Total B4*=245"; got != want {
		t.Fatalf("légende :\n got %s\nwant %s", got, want)
	}

	// Agrandir le tableau : refusé, rien ne change.
	before := dump(t, d)
	for _, c := range []struct {
		row, col int
		raw      string
	}{
		{2, 2, "12"},   // à droite d'une ligne : nouvelle colonne
	} {
		_, err := d.SetCell(ctx, 1, c.row, c.col, c.raw)
		if !errors.Is(err, ErrQueryTable) || err.Error() != "Tableau chargé par une requête : ses cellules ne se modifient pas ; changez la requête (Données › Requêtes) puis actualisez (F5)" {
			t.Fatalf("%s en %d,%d : %v", c.raw, c.row, c.col, err)
		}
		if got := dump(t, d); got != before {
			t.Fatalf("refus incomplet :\n%s", diff(before, got))
		}
	}
	d2 := open(t)
	queryTable(t, d2, rows)
	before2 := dump(t, d2)
	if _, err := d2.SetCell(ctx, 1, 3, 0, "Prunes"); !errors.Is(err, ErrQueryTable) {
		t.Fatalf("nouvelle ligne : %v", err)
	}
	// Collé contre le tableau (cellule par cellule) : refusé aussi.
	if _, err := d2.Paste(ctx, 1, 3, 0, [][]string{{"Prunes", "45"}}); !errors.Is(err, ErrQueryTable) {
		t.Fatalf("collage dessous : %v", err)
	}
	if got := dump(t, d2); got != before2 {
		t.Fatalf("refus incomplet :\n%s", diff(before2, got))
	}

	// Actualisation avec une ligne de plus : le tableau grandit, la légende
	// descend avec son bas (le total reste collé), la remarque reste.
	refresh(append(rows, []any{"Prunes", int64(45)}))
	if got, want := layoutOf(t, d), "Q@A1:Produit|Montant×3 C2*=← à vérifier A5*=Total B5*=245"; got != want {
		t.Fatalf("actualisé :\n got %s\nwant %s", got, want)
	}
	// Une ligne de moins : le total remonte contre le bas.
	refresh(rows[:1])
	if got, want := layoutOf(t, d), "Q@A1:Produit|Montant×1 C2*=← à vérifier A3*=Total B3*=245"; got != want {
		t.Fatalf("raccourci :\n got %s\nwant %s", got, want)
	}
	// La formule de total suit le bas sans être recouverte, et se recalcule.
	refresh3(append(rows, []any{"Prunes", int64(45)}))
	if got, want := layoutOf(t, d3), "Q@A1:Produit|Montant×3 A5*=Source : ERP B5*==SOMME(Q[Montant])"; got != want {
		t.Fatalf("formule actualisée :\n got %s\nwant %s", got, want)
	}
	if v := cellOn(t, d3, 1, 4, 1).Value; v != "245" {
		t.Fatalf("total actualisé : %q", v)
	}
}

// « Sortir cette ligne du tableau (total ou note) » sur la dernière ligne
// d'un tableau de requête : la ligne reste dessous, à part, en une étape ;
// la barre d'état prévient que l'actualisation la remettra. Actualisé, le
// tableau la reprend et la ligne gardée à part descend (rien n'est écrasé).
func TestQueryRowToNote(t *testing.T) {
	ctx := context.Background()
	rows := [][]any{{"Pommes", int64(120)}, {"Total", int64(120)}}
	d := open(t)
	refresh := queryTable(t, d, rows)
	before := dump(t, d)
	d.TakeNotice()
	if err := d.RowToNote(ctx, "Q", 0); !errors.Is(err, ErrNotLastRow) {
		t.Fatalf("ligne du milieu : %v", err)
	}
	must(t, d.RowToNote(ctx, "Q", 1))
	if got, want := layoutOf(t, d), "Q@A1:Produit|Montant×1 A3*=Total B3*=120"; got != want {
		t.Fatalf("ligne sortie :\n got %s\nwant %s", got, want)
	}
	if n := d.TakeNotice(); !strings.Contains(n, "actualisation de la requête « Q »") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "ligne sortie")
	must(t, d.RowToNote(ctx, "Q", 1))
	refresh(rows)
	if got, want := layoutOf(t, d), "Q@A1:Produit|Montant×2 A4*=Total B4*=120"; got != want {
		t.Fatalf("actualisé :\n got %s\nwant %s", got, want)
	}
}
