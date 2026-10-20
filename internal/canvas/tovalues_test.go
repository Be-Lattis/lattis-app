package canvas

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

// valuesFixture : Feuille1 (Produit, Prix, Qté) en A1:C3, colonnes
// calculées Montant (=[@Prix]*[@Qté], décimal), Double (=[@Qté]*2,
// entier) et Libellé (texte), un total libre qui cite Montant.
func valuesFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix", "Qté"}, {"Clavier", "25,5", "2"}, {"Souris", "10", "0"}}); err != nil {
		t.Fatal(err)
	}
	for _, s := range []struct {
		col  int
		head string
		f    string
	}{{3, "Montant", "=[@Prix]*[@Qté]"}, {4, "Double", "=[@Qté]*2"}, {5, "Libellé", `=[@Produit]&" !"`}, {6, "Ratio", "=[@Prix]/[@Qté]"}} {
		if _, err := d.SetCell(ctx, 1, 0, s.col, s.head); err != nil {
			t.Fatal(err)
		}
		if _, err := d.SetCell(ctx, 1, 1, s.col, s.f); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.SetCell(ctx, 1, 6, 0, "=SOMME(Feuille1[Montant])"); err != nil {
		t.Fatal(err)
	}
	return d
}

func columnOf(t *testing.T, d *fixture, name string) ColumnInfo {
	t.Helper()
	for _, c := range tablesOf(t, d)[0].Columns {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("colonne %s absente", name)
	return ColumnInfo{}
}

// Convertir en valeurs : la colonne calculée devient saisie, garde ses
// valeurs et prend leur type ; la formule qui la cite continue de la lire ;
// elle ne suit plus ses sources ; une seule étape d'annulation.
func TestColumnsToValues(t *testing.T) {
	ctx := context.Background()
	d := valuesFixture(t)
	before := dump(t, d)
	if err := d.ColumnsToValues(ctx, "Feuille1", []int{3}); err != nil {
		t.Fatal(err)
	}
	check(t, d, "conversion")
	c := columnOf(t, d, "Montant")
	if c.Formula != "" || c.Type != grid.Decimal {
		t.Fatalf("colonne convertie : %+v", c)
	}
	if n := d.TakeNotice(); n != "Colonne « Montant » convertie en valeurs (la formule est retirée ; Ctrl+Z pour annuler)" {
		t.Errorf("message : %q", n)
	}
	if got := cellsOf(t, d, 1)["6,0"]; got != "=SOMME(Feuille1[Montant])→51" {
		t.Errorf("formule qui cite la colonne : %q", got)
	}
	v := view(t, d)
	if !strings.Contains(v, "51") {
		t.Errorf("valeurs perdues :\n%s", v)
	}
	// La source change : la colonne convertie ne suit plus.
	if _, err := d.SetCell(ctx, 1, 1, 1, "100"); err != nil {
		t.Fatal(err)
	}
	if got := cellsOf(t, d, 1)["6,0"]; got != "=SOMME(Feuille1[Montant])→51" {
		t.Errorf("après changement de la source : %q", got)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	undoOnce(t, d, before, "conversion")
	if c := columnOf(t, d, "Montant"); c.Formula == "" {
		t.Fatal("annulation : la colonne n'est plus calculée")
	}
}

// Type deviné sur les valeurs : entier, texte ; une erreur (#DIV/0!) reste
// visible en saisie en erreur. Plusieurs colonnes d'un coup.
func TestColumnsToValuesTypes(t *testing.T) {
	ctx := context.Background()
	d := valuesFixture(t)
	before := dump(t, d)
	if err := d.ColumnsToValues(ctx, "Feuille1", []int{4, 5, 6}); err != nil {
		t.Fatal(err)
	}
	check(t, d, "conversion")
	if c := columnOf(t, d, "Double"); c.Formula != "" || c.Type != grid.Integer {
		t.Errorf("Double : %+v", c)
	}
	if c := columnOf(t, d, "Libellé"); c.Formula != "" || c.Type != grid.Text {
		t.Errorf("Libellé : %+v", c)
	}
	if c := columnOf(t, d, "Ratio"); c.Formula != "" || c.Type != grid.Decimal || c.Errors != 1 {
		t.Errorf("Ratio : %+v", c)
	}
	if n := d.TakeNotice(); !strings.HasPrefix(n, "3 colonnes converties en valeurs") {
		t.Errorf("message : %q", n)
	}
	v := view(t, d)
	for _, w := range []string{"Clavier !", "Souris !", "12.75"} {
		if !strings.Contains(v, w) {
			t.Errorf("%q absent :\n%s", w, v)
		}
	}
	undoOnce(t, d, before, "conversion")
}

// Refus : colonne saisie, structure protégée.
func TestColumnsToValuesRefused(t *testing.T) {
	ctx := context.Background()
	d := valuesFixture(t)
	if err := d.ColumnsToValues(ctx, "Feuille1", []int{1}); err == nil {
		t.Fatal("colonne saisie convertie")
	}
	if err := d.ProtectStructure(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	structureProtected(t, d.ColumnsToValues(ctx, "Feuille1", []int{3}), "Feuille1")
	structureProtected(t, d.RangeToValues(ctx, 1, 0, 0, 6, 6), "Feuille1")
	if c := columnOf(t, d, "Montant"); c.Formula == "" {
		t.Fatal("colonne convertie sous protection")
	}
}

// Sélection : les colonnes calculées touchées et les cellules libres avec
// formule.
func TestRangeToValues(t *testing.T) {
	ctx := context.Background()
	d := valuesFixture(t)
	if _, err := d.SetCell(ctx, 1, 7, 0, "=A7*2"); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	if err := d.RangeToValues(ctx, 1, 2, 3, 7, 3); err != nil {
		t.Fatal(err)
	}
	check(t, d, "conversion")
	if c := columnOf(t, d, "Montant"); c.Formula != "" {
		t.Errorf("Montant toujours calculée : %+v", c)
	}
	if c := columnOf(t, d, "Double"); c.Formula == "" {
		t.Errorf("Double hors sélection convertie")
	}
	if n := d.TakeNotice(); !strings.HasPrefix(n, "Colonne « Montant » convertie en valeurs") {
		t.Errorf("message : %q", n)
	}
	undoOnce(t, d, before, "conversion")

	if err := d.RangeToValues(ctx, 1, 6, 0, 7, 0); err != nil {
		t.Fatal(err)
	}
	got := cellsOf(t, d, 1)
	if got["6,0"] != "51" || got["7,0"] != "102" {
		t.Errorf("cellules libres : %v", got)
	}
	if n := d.TakeNotice(); !strings.HasPrefix(n, "2 cellules converties en valeurs") {
		t.Errorf("message : %q", n)
	}
	undoOnce(t, d, before, "conversion des cellules")
	if err := d.RangeToValues(ctx, 1, 10, 10, 12, 12); err == nil {
		t.Error("sélection sans formule acceptée")
	}
}

type publishedOpen struct{}

func (publishedOpen) TableLock(int64) error   { return nil }
func (publishedOpen) TableShare(int64) string { return "published" }

// Tableau publié d'un classeur collaboratif : refusé, avec un message clair.
func TestColumnsToValuesShared(t *testing.T) {
	ctx := context.Background()
	d := valuesFixture(t)
	d.SetSharing(publishedOpen{})
	err := d.ColumnsToValues(ctx, "Feuille1", []int{3})
	var e ErrSharedValues
	if !errors.As(err, &e) || e.Column != "Montant" {
		t.Fatalf("attendu : refus du tableau partagé, obtenu %v", err)
	}
	if c := columnOf(t, d, "Montant"); c.Formula == "" {
		t.Fatal("colonne convertie")
	}
}
