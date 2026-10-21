package canvas

import (
	"context"
	"strings"
	"testing"
)

// Un TCD se lit comme un tableau nommé : TCD1[Col] (lignes de détail) et
// TCD1[Col][n] dans une formule libre ; [Col] (ou [@Col]) dans une colonne
// saisie à côté de lui vaut la valeur de la ligne, et suit le TCD quand il
// est trié ou que sa source change.
func TestPivotAsNamedTable(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	sheet, id, base := handPivot(t, d)
	count := onePivot(t, d, sheet).Cells[0][1] // « Nombre »
	// Formules libres, sur la feuille de la source.
	mustSet(t, d, 1, 20, 6, "=SOMME(TCD1["+count+"])")
	mustSet(t, d, 1, 21, 6, "=TCD1["+count+"][2]")
	mustSet(t, d, 1, 22, 6, "=TCD1[NID][3]")
	mustSet(t, d, 1, 23, 6, "=NB(TCD1["+count+"])")
	for _, c := range []struct {
		row  int
		want string
	}{{20, "4"}, {21, "1"}, {22, "C"}, {23, "3"}} {
		if got := freeValue(t, d, 1, c.row, 6); got != c.want {
			t.Fatalf("ligne %d : %q, voulu %q", c.row, got, c.want)
		}
	}
	// Colonnes calculées à côté du TCD : [Col] et [@Col] lisent la ligne.
	mustSet(t, d, sheet, 1, 3, "double")
	mustSet(t, d, sheet, 2, 3, "=["+count+"]*2")
	mustSet(t, d, sheet, 1, 4, "note")
	mustSet(t, d, sheet, 2, 4, "vu")
	mustSet(t, d, sheet, 1, 5, "triple")
	mustSet(t, d, sheet, 3, 5, "=[@"+count+"]*3")
	if got := cellsText(onePivot(t, d, sheet)); got != "NID "+count+" double note triple | A 2 4 vu 6 | B 1 2  3 | C 1 2  3 | Total 4 8  12" {
		t.Fatalf("colonnes calculées : %q", got)
	}
	// Une formule libre lit les colonnes saisies du TCD par leur nom.
	mustSet(t, d, 1, 24, 6, "=SOMME(TCD1[double])")
	if got := freeValue(t, d, 1, 24, 6); got != "8" {
		t.Fatalf("SOMME(TCD1[double]) : %q", got)
	}
	// Source changée (actualisation) puis tri décroissant : les lignes suivent.
	if _, err := d.Paste(ctx, 1, 5, 0, [][]string{{"B", "Metz"}}); err != nil {
		t.Fatal(err)
	}
	cfg := base
	cfg.Sort = map[string]string{"NID": "desc"}
	must(t, d.SetPivot(ctx, id, cfg))
	if got := cellsText(onePivot(t, d, sheet)); got != "NID "+count+" double note triple | C 1 2  3 | B 2 4  6 | A 2 4 vu 6 | Total 5 10  15" {
		t.Fatalf("après tri : %q", got)
	}
	if got := freeValue(t, d, 1, 20, 6); got != "5" {
		t.Fatalf("SOMME(TCD1) après actualisation : %q", got)
	}
	if got := freeValue(t, d, 1, 22, 6); got != "A" {
		t.Fatalf("TCD1[NID][3] après tri : %q", got)
	}
	if got := freeValue(t, d, 1, 24, 6); got != "10" {
		t.Fatalf("SOMME(TCD1[double]) après tri : %q", got)
	}
	check(t, d, "TCD nommé")
}

// En arborescence, [Col] d'une colonne calculée vaut sur chaque ligne la
// valeur affichée de cette ligne : groupes et sous-totaux sur leurs propres
// valeurs (le sous-total du groupe), le total général sur le total ; les
// champs des lignes plus profonds que la ligne y sont vides. TCD1[Col] ne
// lit que les lignes de détail.
func TestPivotRefTree(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Service", "Poste", "Salaire"}, {"RH", "Chargé", "30"}, {"IT", "Dev", "50"}, {"RH", "Chef", "60"}, {"IT", "Dev", "40"}, {"IT", "Admin", "35"}}); err != nil {
		t.Fatal(err)
	}
	sheet, err := d.CreatePivot(ctx, PivotConfig{Source: "Feuille1", RowFields: []string{"Service", "Poste"}, Values: []PivotValue{{Field: "Salaire", Agg: "sum"}}})
	must(t, err)
	p := onePivot(t, d, sheet)
	sum := p.Cells[0][1]
	col := p.Col + len(p.Cells[0])
	mustSet(t, d, sheet, p.Row, col, "part")
	mustSet(t, d, sheet, p.Row+2, col, "=["+sum+"]/SOMME(TCD1["+sum+"])")
	mustSet(t, d, sheet, p.Row, col+1, "qui")
	mustSet(t, d, sheet, p.Row+2, col+1, `=SI(ESTVIDE([Poste]);"-";[Poste])`)
	p = onePivot(t, d, sheet)
	var got []string
	for i, r := range p.Cells[1:] {
		k := "d"
		switch info := p.Rows[i]; {
		case info.Total:
			k = "T"
		case info.Group || info.Subtotal:
			k = "g"
		}
		got = append(got, k+":"+strings.Join(r, "|"))
	}
	want := "g:RH|90|0,418604651162791|- d:Chargé|30|0,13953488372093|Chargé d:Chef|60|0,27906976744186|Chef " +
		"g:IT|125|0,581395348837209|- d:Dev|90|0,418604651162791|Dev d:Admin|35|0,162790697674419|Admin T:Total|215|1|-"
	if strings.Join(got, " ") != want {
		t.Fatalf("arborescence :\n%s\nvoulu\n%s", strings.Join(got, " "), want)
	}
	// Formule libre : TCD1[Somme] ne compte pas les sous-totaux ni le total.
	mustSet(t, d, 1, 20, 6, "=SOMME(TCD1["+sum+"])")
	mustSet(t, d, 1, 21, 6, "=TCD1[Poste][4]")
	if got := freeValue(t, d, 1, 20, 6); got != "215" {
		t.Fatalf("SOMME : %q", got)
	}
	if got := freeValue(t, d, 1, 21, 6); got != "Admin" {
		t.Fatalf("TCD1[Poste][4] : %q", got)
	}
	check(t, d, "arborescence")
}
