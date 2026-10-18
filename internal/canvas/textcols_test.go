package canvas

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestSplitText(t *testing.T) {
	cases := []struct {
		in   string
		o    TextColumnsOptions
		want []string
	}{
		{"a;b;c", TextColumnsOptions{Delims: ";"}, []string{"a", "b", "c"}},
		{"a;;b", TextColumnsOptions{Delims: ";"}, []string{"a", "", "b"}},
		{"a;;b", TextColumnsOptions{Delims: ";", Merge: true}, []string{"a", "b"}},
		{"a  b\tc", TextColumnsOptions{Delims: " \t", Merge: true}, []string{"a", "b", "c"}},
		{`"Dupont; Jean";12`, TextColumnsOptions{Delims: ";", Quote: `"`}, []string{"Dupont; Jean", "12"}},
		{`"dit ""oui""";x`, TextColumnsOptions{Delims: ";", Quote: `"`}, []string{`dit "oui"`, "x"}},
		{`"a;b";c`, TextColumnsOptions{Delims: ";"}, []string{`"a`, `b"`, "c"}},
		{"'a,b',c", TextColumnsOptions{Delims: ",", Quote: "'"}, []string{"a,b", "c"}},
		{"abc", TextColumnsOptions{}, []string{"abc"}},
		{"ABCDEFGH", TextColumnsOptions{Fixed: true, Breaks: []int{5, 2}}, []string{"AB", "CDE", "FGH"}},
		{"AB  CD  ", TextColumnsOptions{Fixed: true, Breaks: []int{4}}, []string{"AB", "CD"}},
		{"AB", TextColumnsOptions{Fixed: true, Breaks: []int{4, 6}}, []string{"AB", "", ""}},
		{"éàç", TextColumnsOptions{Fixed: true, Breaks: []int{1}}, []string{"é", "àç"}},
	}
	for _, c := range cases {
		if got := splitText(c.in, c.o); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitText(%q, %+v) = %q, attendu %q", c.in, c.o, got, c.want)
		}
	}
}

func TestLocalNumber(t *testing.T) {
	cases := []struct {
		in, dec, thou, want string
		ok                  bool
	}{
		{"1 234,5", ",", " ", "1234,5", true},
		{"1.234,5", ",", ".", "1234,5", true},
		{"1,234.5", ".", ",", "1234,5", true},
		{"12.5", ".", "", "12,5", true},
		{"12.5", ",", " ", "", false}, // séparateur étranger : texte
		{"42-", ",", " ", "-42", true},
		{"007", ",", " ", "007", true},
		{"abc", ",", " ", "", false},
	}
	for _, c := range cases {
		got, ok := localNumber(c.in, c.dec, c.thou)
		if got != c.want || ok != c.ok {
			t.Errorf("localNumber(%q, %q, %q) = %q, %v ; attendu %q, %v", c.in, c.dec, c.thou, got, ok, c.want, c.ok)
		}
	}
}

func TestOrderedDate(t *testing.T) {
	cases := []struct{ in, order, want string }{
		{"03/04/2024", "DMY", "2024-04-03"},
		{"03/04/2024", "MDY", "2024-03-04"},
		{"2024-04-03", "YMD", "2024-04-03"},
		{"3.4.24", "DMY", "2024-04-03"},
		{"3.4.95", "DMY", "1995-04-03"},
		{"31/02/2024", "DMY", ""},
		{"13/13/2024", "DMY", ""},
	}
	for _, c := range cases {
		d, ok := orderedDate(c.in, c.order)
		got := ""
		if ok {
			got = d.Format("2006-01-02")
		}
		if got != c.want {
			t.Errorf("orderedDate(%q, %q) = %q, attendu %q", c.in, c.order, got, c.want)
		}
	}
}

func TestTextToColumnsTable(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Personne", "Ville"}, {"Dupont;12;03/04/2024", "Lyon"}, {"Martin;7,5;05/06/2023", "Rome"}, {"Durand", "Paris"}}); err != nil {
		t.Fatal(err)
	}
	if err := promoteIfData(ctx, d, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	// Un objet à droite du tableau : il doit être poussé.
	if _, err := d.SetCell(ctx, 1, 0, 3, "note"); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	o := TextColumnsOptions{Delims: ";", Columns: []TextColumnFormat{{}, {}, {Format: "date", Order: "DMY"}}}
	p, err := d.TextToColumnsPreview(ctx, 1, 1, 3, 0, o)
	if err != nil {
		t.Fatal(err)
	}
	if p.Table != "Feuille1" || p.Column != "Personne" || p.Count != 3 || p.Total != 3 || p.Header {
		t.Fatalf("aperçu : %+v", p)
	}
	n, err := d.TextToColumns(ctx, 1, 1, 3, 0, o)
	if err != nil || n != 3 {
		t.Fatalf("conversion : %d, %v", n, err)
	}
	check(t, d, "conversion")
	want := lines(
		"[Personne]|[Personne 2]|[Personne 3]|[Ville]||'note",
		"Dupont|12|2024-04-03|Lyon",
		"Martin|7.5|2023-06-05|Rome",
		"Durand|||Paris")
	if got := view(t, d); got != want {
		t.Fatalf("après :\n%s", diff(want, got))
	}
	types := tableTypes(t, d, "Feuille1")
	if !reflect.DeepEqual(types, []string{"text", "decimal", "date", "text"}) {
		t.Fatalf("types : %v", types)
	}
	after := dump(t, d)
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation (une seule étape) :\n%s", diff(before, got))
	}
	if _, err := d.bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := dump(t, d); got != after {
		t.Fatalf("rétablissement :\n%s", diff(after, got))
	}
}

func TestTextToColumnsTableHeaderAndFormats(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Code,Montant,Ignoré"}, {"007,\"1,234.5\",x"}, {"012,10,y"}}); err != nil {
		t.Fatal(err)
	}
	if err := promoteIfData(ctx, d, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	// En-tête compris, la première colonne en texte (zéros gardés), point
	// décimal et virgule des milliers, troisième colonne non importée.
	o := TextColumnsOptions{Delims: ",", Quote: `"`, Decimal: ".", Thousands: ",",
		Columns: []TextColumnFormat{{Format: "text"}, {}, {Format: "skip"}}}
	p, err := d.TextToColumnsPreview(ctx, 1, 0, 2, 0, o)
	if err != nil || !p.Header || p.Count != 3 {
		t.Fatalf("aperçu : %+v, %v", p, err)
	}
	if _, err := d.TextToColumns(ctx, 1, 0, 2, 0, o); err != nil {
		t.Fatal(err)
	}
	check(t, d, "conversion")
	want := lines(
		"[Code]|[Montant]",
		"007|1234.5",
		"012|10")
	if got := view(t, d); got != want {
		t.Fatalf("après :\n%s", diff(want, got))
	}
	if types := tableTypes(t, d, "Feuille1"); !reflect.DeepEqual(types, []string{"text", "decimal"}) {
		t.Fatalf("types : %v", types)
	}
	d.bus.Undo(ctx)
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
}

func TestTextToColumnsPartialRows(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Valeur"}, {"1-2"}, {"3-4"}, {"5"}}); err != nil {
		t.Fatal(err)
	}
	if err := promoteIfData(ctx, d, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	// Seule la deuxième ligne de données : les autres gardent leur valeur, et
	// la colonne prend un type qui les accepte toutes.
	if _, err := d.TextToColumns(ctx, 1, 2, 2, 0, TextColumnsOptions{Delims: "-"}); err != nil {
		t.Fatal(err)
	}
	check(t, d, "conversion partielle")
	want := lines(
		"[Valeur]|[Valeur 2]",
		"1-2",
		"3|4",
		"5")
	if got := view(t, d); got != want {
		t.Fatalf("après :\n%s", diff(want, got))
	}
}

func TestTextToColumnsFreeCells(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	// Deux cellules séparées d'une ligne vide : elles restent libres.
	for _, c := range []struct {
		row, col int
		raw      string
	}{{0, 0, "a;1,5;03/04/2024;VRAI"}, {2, 0, "b;007;x;"}, {2, 2, "occupé"}, {5, 0, "=1+1"}} {
		if _, err := d.SetCell(ctx, 1, c.row, c.col, c.raw); err != nil {
			t.Fatal(err)
		}
	}
	before := dump(t, d)
	o := TextColumnsOptions{Delims: ";", Columns: []TextColumnFormat{{}, {Format: "text"}}}
	p, err := d.TextToColumnsPreview(ctx, 1, 0, 5, 0, o)
	if err != nil {
		t.Fatal(err)
	}
	if p.Table != "" || p.Total != 2 || p.Count != 4 || !p.Overwrite || p.Dest != "A1" {
		t.Fatalf("aperçu : %+v", p)
	}
	if _, err := d.TextToColumns(ctx, 1, 0, 5, 0, o); err != nil {
		t.Fatal(err)
	}
	check(t, d, "cellules libres")
	want := lines(
		"'a|''1,5|'03/04/2024|'VRAI",
		"",
		"'b|''007|'x",
		"",
		"",
		"'=1+1")
	if got := view(t, d); got != want {
		t.Fatalf("après :\n%s", diff(want, got))
	}
	d.bus.Undo(ctx)
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}

	// Une autre destination : la source reste, rien n'est écrasé.
	o.Dest = "$E$1"
	if p, err := d.TextToColumnsPreview(ctx, 1, 0, 2, 0, o); err != nil || p.Overwrite {
		t.Fatalf("aperçu vers E1 : %+v, %v", p, err)
	}
	if _, err := d.TextToColumns(ctx, 1, 0, 2, 0, o); err != nil {
		t.Fatal(err)
	}
	if got := view(t, d); !strings.HasPrefix(got, "'a;1,5;03/04/2024;VRAI||||'a|''1,5|'03/04/2024|'VRAI\n") {
		t.Fatalf("vers E1 :\n%s", got)
	}
	check(t, d, "destination")

	// Une destination sur un tableau est refusée.
	if _, err := d.Paste(ctx, 1, 10, 4, [][]string{{"X", "Y"}, {"1", "2"}}); err != nil {
		t.Fatal(err)
	}
	o.Dest = "D11"
	if _, err := d.TextToColumns(ctx, 1, 0, 2, 0, o); err == nil || !strings.Contains(err.Error(), "recouvre") {
		t.Fatalf("destination sur un tableau : %v", err)
	}
}

func TestTextToColumnsRefusals(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.TextToColumns(ctx, 1, 0, 3, 0, TextColumnsOptions{Delims: ";"}); err != ErrNoText {
		t.Fatalf("sans texte : %v", err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 0, "a;b"); err != nil {
		t.Fatal(err)
	}
	o := TextColumnsOptions{Delims: ";", Columns: []TextColumnFormat{{Format: "skip"}, {Format: "skip"}}}
	if _, err := d.TextToColumns(ctx, 1, 0, 0, 0, o); err == nil {
		t.Fatal("aucune colonne importée : accepté")
	}
}

// tableTypes donne les types des colonnes d'un tableau.
func tableTypes(t *testing.T, d *fixture, name string) []string {
	t.Helper()
	all, err := d.AllTables(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, tb := range all {
		if tb.Name == name {
			return tb.Types
		}
	}
	t.Fatalf("tableau %s introuvable", name)
	return nil
}
