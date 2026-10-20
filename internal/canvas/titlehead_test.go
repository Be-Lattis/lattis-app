package canvas

import (
	"context"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

// En-têtes tapés après les données, au-dessus d'un tableau dont l'en-tête
// ressemble à des données : la saisie le propose (à partir de deux noms),
// « Utiliser comme en-têtes » les fait entrer ; l'ancien en-tête devient la
// 1re ligne, typée (31 reste un nombre) ; les formules suivent ; une étape.
func TestTitleToHeader(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		_, err := x.createTable(1, "T", 1, 0, []string{"Léa", "31"}, [][]string{{"Max", "40"}, {"Zoé", "25"}})
		return err
	}))
	mustSet(t, d, 1, 8, 0, "=SOMME(T[31])")
	ev, err := d.SetCell(ctx, 1, 0, 0, "Nom")
	must(t, err)
	if ev.HeaderOffer != "" {
		t.Fatalf("un seul nom au-dessus : un titre, pas de proposition (%q)", ev.HeaderOffer)
	}
	if ev, err = d.SetCell(ctx, 1, 0, 1, "Age"); err != nil || ev.HeaderOffer != "T" {
		t.Fatalf("deux noms au-dessus d'un en-tête « Léa | 31 » : proposition attendue (%q, %v)", ev.HeaderOffer, err)
	}
	if got, want := layoutOf(t, d), "T@A2:Léa|31×2 A1=Nom B1=Age A9==SOMME(T[31])"; got != want {
		t.Fatalf("avant :\n got %s\nwant %s", got, want)
	}
	before := dump(t, d)
	must(t, d.TitleToHeader(ctx, "T"))
	check(t, d, "en-têtes")
	if got, want := layoutOf(t, d), "T@A1:Nom|Age×3 A9==SOMME(T[Age])"; got != want {
		t.Fatalf("après :\n got %s\nwant %s", got, want)
	}
	if ts := tablesOf(t, d); ts[0].Columns[1].Type != grid.Integer {
		t.Fatalf("31 reste un nombre : %s", ts[0].Columns[1].Type)
	}
	if v := cellOn(t, d, 1, 8, 0).Value; v != "96" {
		t.Fatalf("formule après : %q", v)
	}
	if got := view(t, d); !strings.Contains(got, "Léa|31\nMax|40") {
		t.Fatalf("l'ancien en-tête en 1re ligne :\n%s", got)
	}
	if n := d.TakeNotice(); !strings.Contains(n, "l'ancien en-tête est devenu la 1re ligne") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "en-têtes")
}

// Noms « Colonne n » : pas de ligne en plus, les données remontent sous les
// noms tapés. Une cellule vide au-dessus garde le nom de sa colonne.
func TestTitleToHeaderAutoNames(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		_, err := x.createTable(1, "T", 1, 0, []string{"Colonne 1", "Colonne 2", "Colonne 3"}, [][]string{{"Léa", "31", "Paris"}})
		return err
	}))
	mustSet(t, d, 1, 0, 0, "Nom")
	ev, err := d.SetCell(ctx, 1, 0, 2, "Ville")
	if err != nil || ev.HeaderOffer != "T" {
		t.Fatalf("proposition : %q, %v", ev.HeaderOffer, err)
	}
	must(t, d.TitleToHeader(ctx, "T"))
	check(t, d, "noms automatiques")
	if got, want := layoutOf(t, d), "T@A1:Nom|Colonne 2|Ville×1"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
}

// Sur un tableau aux vrais en-têtes, la commande reste possible (la saisie
// ne la propose pas) ; ce qui dépasse à côté des colonnes reste libre.
func TestTitleToHeaderRealNames(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		_, err := x.createTable(1, "T", 1, 1, []string{"Prénom", "Ville"}, [][]string{{"Léa", "Paris"}})
		return err
	}))
	mustSet(t, d, 1, 0, 0, "Liste")
	mustSet(t, d, 1, 0, 1, "Nom")
	ev, err := d.SetCell(ctx, 1, 0, 2, "Lieu")
	if err != nil || ev.HeaderOffer != "" {
		t.Fatalf("vrais en-têtes : pas de proposition (%q, %v)", ev.HeaderOffer, err)
	}
	if got, want := layoutOf(t, d), "T@B2:Prénom|Ville×1 A1=Liste B1=Nom C1=Lieu"; got != want {
		t.Fatalf("avant :\n got %s\nwant %s", got, want)
	}
	before := dump(t, d)
	must(t, d.TitleToHeader(ctx, "T"))
	check(t, d, "vrais en-têtes")
	if got, want := layoutOf(t, d), "T@B1:Nom|Lieu×2 A1*=Liste"; got != want {
		t.Fatalf("après :\n got %s\nwant %s", got, want)
	}
	if got := view(t, d); !strings.Contains(got, "|Prénom|Ville\n|Léa|Paris") {
		t.Fatalf("\n%s", got)
	}
	undoOnce(t, d, before, "vrais en-têtes")
}

// Refus : rien au-dessus, une formule, la ligne d'un autre tableau, des
// en-têtes sur deux lignes.
func TestTitleToHeaderRefused(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		if _, err := x.createTable(1, "Haut", 0, 0, []string{"A", "B"}, [][]string{{"1", "2"}}); err != nil {
			return err
		}
		_, err := x.createTable(1, "Bas", 2, 0, []string{"C", "D"}, [][]string{{"3", "4"}})
		return err
	}))
	if err := d.TitleToHeader(ctx, "Haut"); err == nil || !strings.Contains(err.Error(), "rien à prendre") {
		t.Fatalf("rien au-dessus : %v", err)
	}
	// Deux tableaux empilés restent séparés d'une ligne vide : rien non plus.
	if err := d.TitleToHeader(ctx, "Bas"); err == nil || !strings.Contains(err.Error(), "rien à prendre") {
		t.Fatalf("sous un autre tableau : %v", err)
	}

	d = open(t)
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		_, err := x.createTable(1, "T", 2, 0, []string{"Nom", "Statut"}, [][]string{{"Chêne", "vivant"}})
		return err
	}))
	mustSet(t, d, 1, 1, 0, "=1+1")
	if err := d.TitleToHeader(ctx, "T"); err == nil || !strings.Contains(err.Error(), "formule") {
		t.Fatalf("formule : %v", err)
	}
	mustSet(t, d, 1, 1, 0, "Arbre")
	mustSet(t, d, 1, 1, 1, "État")
	if _, err := d.MergeCells(ctx, 1, 1, 0, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := d.TitleToHeader(ctx, "T"); err == nil || !strings.Contains(err.Error(), "fusionnées") {
		t.Fatalf("fusion : %v", err)
	}
	mustSet(t, d, 1, 1, 0, "ARBRE")
	must(t, d.SetTwoLevel(ctx, "T", true))
	if err := d.TitleToHeader(ctx, "T"); err == nil || !strings.Contains(err.Error(), "deux lignes") {
		t.Fatalf("deux niveaux : %v", err)
	}
}

// Un nom tapé qui est celui d'une autre colonne : les formules suivent
// chaque colonne, pas le nom.
func TestTitleToHeaderSwap(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	must(t, d.Do(ctx, "Saisie", func(x *tx) error {
		_, err := x.createTable(1, "T", 1, 0, []string{"Nom", "2024"}, [][]string{{"Léa", "5"}})
		return err
	}))
	mustSet(t, d, 1, 8, 0, "=SOMME(T[2024])")
	mustSet(t, d, 1, 0, 0, "Prénom")
	mustSet(t, d, 1, 0, 1, "Nom")
	must(t, d.TitleToHeader(ctx, "T"))
	check(t, d, "noms échangés")
	if got, want := layoutOf(t, d), "T@A1:Prénom|Nom×2 A9==SOMME(T[Nom])"; got != want {
		t.Fatalf("\n got %s\nwant %s", got, want)
	}
	if v := cellOn(t, d, 1, 8, 0).Value; v != "2029" {
		t.Fatalf("formule : %q", v)
	}
}
