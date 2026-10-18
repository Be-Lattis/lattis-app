package canvas

import (
	"context"
	"strings"
	"testing"
)

// undoOnce annule la dernière action et vérifie que la feuille revient
// exactement à before (une seule étape d'annulation).
func undoOnce(t *testing.T, d *fixture, before, when string) {
	t.Helper()
	if _, err := d.bus.Undo(context.Background()); err != nil {
		t.Fatal(err)
	}
	check(t, d, when+" annulé")
	if got := dump(t, d); got != before {
		t.Fatalf("%s : annulation incomplète :\n%s", when, diff(before, got))
	}
}

// typeBlock saisit un bloc cellule par cellule, comme au clavier (la
// première ligne de texte devient l'en-tête).
func typeBlock(d *fixture, row, col int, block [][]string) (struct{}, error) {
	for r, line := range block {
		for c, raw := range line {
			if _, err := d.SetCell(context.Background(), 1, row+r, col+c, raw); err != nil {
				return struct{}{}, err
			}
		}
	}
	return struct{}{}, nil
}

func tablesOf(t *testing.T, d *fixture) []TableInfo {
	t.Helper()
	l, err := d.Layout(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return l.Tables
}

func colNames(ti TableInfo) string {
	var s []string
	for _, c := range ti.Columns {
		s = append(s, c.Name)
	}
	return strings.Join(s, "|")
}

// Écrire juste à gauche d'un tableau en fait la nouvelle première colonne :
// sur l'en-tête, son nom ; sur une ligne de données, « Colonne n » avec la
// valeur. Le tableau s'ancre une colonne plus à gauche.
func TestWriteLeftGrows(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := typeBlock(d, 0, 3, [][]string{{"Nom", "Prenom"}, {"Dupont", "Jean"}, {"Durand", "Marie"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 6, `=NBVAL(Feuille1[Nom])`); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	if _, err := d.SetCell(ctx, 1, 0, 2, "ID"); err != nil {
		t.Fatal(err)
	}
	check(t, d, "en-tête à gauche")
	ts := tablesOf(t, d)
	if len(ts) != 1 || ts[0].Col != 2 || colNames(ts[0]) != "ID|Nom|Prenom" || ts[0].Name != "Feuille1" {
		t.Fatalf("en-tête à gauche : %+v", ts)
	}
	if got := view(t, d); !strings.Contains(got, "[ID]|[Nom]|[Prenom]") || !strings.Contains(got, "|Dupont|Jean") {
		t.Fatalf("en-tête à gauche :\n%s", got)
	}
	undoOnce(t, d, before, "en-tête à gauche")

	if _, err := d.SetCell(ctx, 1, 2, 2, "7"); err != nil {
		t.Fatal(err)
	}
	check(t, d, "donnée à gauche")
	ts = tablesOf(t, d)
	if len(ts) != 1 || ts[0].Col != 2 || colNames(ts[0]) != "Colonne 3|Nom|Prenom" {
		t.Fatalf("donnée à gauche : %+v", ts)
	}
	if got := view(t, d); !strings.Contains(got, "7|Durand|Marie") {
		t.Fatalf("donnée à gauche :\n%s", got)
	}
	// La formule qui lit le tableau par nom de colonne reste juste.
	l, _ := d.Layout(ctx, 1)
	for _, c := range l.Cells {
		if c.Row == 0 && c.Col == 6 && c.Value != "2" {
			t.Fatalf("formule : %q", c.Value)
		}
	}
	undoOnce(t, d, before, "donnée à gauche")
}

// Écrire à gauche d'un tableau qui le colle à un autre : les deux fusionnent
// dans la même étape, avec un message.
func TestWriteLeftTouchesOther(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"ID"}, {"1"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := typeBlock(d, 0, 2, [][]string{{"Nom", "Sexe"}, {"Dupont", "M"}, {"Durand", "F"}, {"Petit", "F"}}); err != nil {
		t.Fatal(err)
	}
	ts := tablesOf(t, d)
	if len(ts) != 2 {
		t.Fatalf("départ : %+v", ts)
	}
	before := dump(t, d)
	d.TakeNotice()
	// Ligne 3 : hors des lignes du tableau de gauche, c'est celui de droite
	// qui s'élargit… et touche alors le premier.
	if _, err := d.SetCell(ctx, 1, 3, 1, "x"); err != nil {
		t.Fatal(err)
	}
	check(t, d, "fusion par la gauche")
	ts = tablesOf(t, d)
	if len(ts) != 1 || ts[0].Col != 0 || colNames(ts[0]) != "ID|Colonne 3|Nom|Sexe" {
		t.Fatalf("fusion par la gauche : %+v", ts)
	}
	// Le plus gros garde son nom.
	if ts[0].Name != ts[0].Name || !strings.Contains(view(t, d), "1||Dupont|M") {
		t.Fatalf("fusion par la gauche :\n%s", view(t, d))
	}
	if n := d.TakeNotice(); !strings.Contains(n, "fusionnés (Ctrl+Z pour annuler)") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "fusion par la gauche")
}

// Un collage qui rend deux tableaux jointifs les fusionne.
func TestPasteMakesAdjacentMerges(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := typeBlock(d, 0, 3, [][]string{{"Nom", "Prenom"}, {"Dupont", "Jean"}, {"Durand", "Marie"}}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	if _, err := d.Paste(ctx, 1, 0, 1, [][]string{{"ID", "Code"}, {"1", "a"}, {"2", "b"}}); err != nil {
		t.Fatal(err)
	}
	check(t, d, "collage jointif")
	ts := tablesOf(t, d)
	if len(ts) != 1 || ts[0].Col != 1 || colNames(ts[0]) != "ID|Code|Nom|Prenom" {
		t.Fatalf("collage jointif : %+v\n%s", ts, view(t, d))
	}
	if got := view(t, d); !strings.Contains(got, "1|a|Dupont|Jean") || !strings.Contains(got, "2|b|Durand|Marie") {
		t.Fatalf("collage jointif :\n%s", got)
	}
	undoOnce(t, d, before, "collage jointif")
}

// Un tableau déplacé contre un autre, même décalé d'une ligne, fusionne
// avec lui (lignes alignées par rang).
func TestMoveMakesAdjacentMerges(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"ID"}, {"1"}, {"2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := typeBlock(d, 5, 4, [][]string{{"Nom", "Prenom", "Sexe"}, {"Dupont", "Jean", "M"}, {"Durand", "Marie", "F"}, {"Petit", "Paul", "M"}}); err != nil {
		t.Fatal(err)
	}
	ts := tablesOf(t, d)
	big := ts[1].Name
	before := dump(t, d)
	d.TakeNotice()
	if err := d.MoveTable(ctx, big, 1, 1); err != nil {
		t.Fatal(err)
	}
	check(t, d, "déplacement jointif")
	ts = tablesOf(t, d)
	if len(ts) != 1 || ts[0].Col != 0 || colNames(ts[0]) != "ID|Nom|Prenom|Sexe" || ts[0].Name != big {
		t.Fatalf("déplacement jointif : %+v\n%s", ts, view(t, d))
	}
	if got := view(t, d); !strings.Contains(got, "1|Dupont|Jean|M") || !strings.Contains(got, "|Petit|Paul|M") {
		t.Fatalf("déplacement jointif :\n%s", got)
	}
	if n := d.TakeNotice(); !strings.Contains(n, "fusionnés") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "déplacement jointif")

	// Des cellules libres déplacées contre un tableau le rejoignent.
	if _, err := d.SetCell(ctx, 1, 0, 3, "Note"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 5, "seul"); err != nil {
		t.Fatal(err)
	}
	before = dump(t, d)
	if err := d.MoveCells(ctx, 1, 0, 3, 0, 3, 0, -2); err != nil {
		t.Fatal(err)
	}
	check(t, d, "cellule déplacée")
	if ts := tablesOf(t, d); len(ts) != 2 || colNames(ts[0]) != "ID|Note" {
		t.Fatalf("cellule déplacée : %+v", ts)
	}
	undoOnce(t, d, before, "cellule déplacée")
}

// Supprimer une colonne de la feuille qui rapproche une cellule libre de
// l'en-tête d'un tableau : elle devient une nouvelle colonne (cas de Louis :
// « aa » collé à droite de ID|Nom|Prenom|Sexe), ses valeurs dessous aussi.
func TestDeleteColumnJoinsFreeCell(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"ID", "Nom", "Prenom", "Sexe"}, {"1", "Dupont", "Jean", "M"}, {"2", "Durand", "Marie", "F"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 5, "aa"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 2, 5, "42"); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	d.TakeNotice()
	if err := d.DeleteSheetColumns(ctx, 1, 4, 4); err != nil {
		t.Fatal(err)
	}
	check(t, d, "colonne supprimée")
	ts := tablesOf(t, d)
	if len(ts) != 1 || colNames(ts[0]) != "ID|Nom|Prenom|Sexe|aa" {
		t.Fatalf("colonne supprimée : %+v\n%s", ts, view(t, d))
	}
	if got := view(t, d); !strings.Contains(got, "2|Durand|Marie|F|42") {
		t.Fatalf("colonne supprimée :\n%s", got)
	}
	if l, _ := d.Layout(ctx, 1); len(l.Cells) != 0 {
		t.Fatalf("cellules libres restantes : %+v", l.Cells)
	}
	if n := d.TakeNotice(); !strings.Contains(n, "2 cellules (« aa »…) rattachées au tableau « Feuille1 »") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "colonne supprimée")

	// Même chose sous le tableau : une ligne supprimée colle « zz » dessous.
	if _, err := d.SetCell(ctx, 1, 4, 1, "zz"); err != nil {
		t.Fatal(err)
	}
	before = dump(t, d)
	if err := d.DeleteSheetRows(ctx, 1, 3, 3); err != nil {
		t.Fatal(err)
	}
	check(t, d, "ligne supprimée")
	if ts := tablesOf(t, d); len(ts) != 1 || ts[0].Rows != 3 {
		t.Fatalf("ligne supprimée : %+v\n%s", ts, view(t, d))
	}
	if got := view(t, d); !strings.Contains(got, "\n|zz") {
		t.Fatalf("ligne supprimée :\n%s", got)
	}
	undoOnce(t, d, before, "ligne supprimée")
}

// Un titre au-dessus d'un tableau reste une cellule libre.
func TestTitleAboveStaysFree(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 2, 0, [][]string{{"Nom", "Ventes"}, {"A", "1"}, {"B", "2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 0, "Titre"); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteSheetRows(ctx, 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	check(t, d, "titre")
	l, _ := d.Layout(ctx, 1)
	if len(l.Cells) != 1 || len(l.Tables) != 1 || colNames(l.Tables[0]) != "Nom|Ventes" {
		t.Fatalf("titre : %+v %+v", l.Cells, l.Tables)
	}
}
