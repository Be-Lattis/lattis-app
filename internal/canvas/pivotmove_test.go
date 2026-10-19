package canvas

import (
	"context"
	"testing"
)

// Déplacer un tableau croisé (clic droit › Déplacer, glisser, couper-coller)
// ou le créer à une cellule : il se pose exactement où l'on a cliqué ; les
// tableaux et tableaux croisés qu'il recouvre ou touche partent vers la
// droite, une colonne vide entre eux ; d'une feuille à l'autre aussi ; une
// seule étape d'annulation à chaque fois.
func TestPivotMovePushes(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	// A : lignes 0-2, colonnes 0-1 ; B : lignes 0-1, colonnes 6-7.
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes"}, {"FR", "10"}, {"IT", "5"}})
	must(t, err)
	_, err = d.Paste(ctx, 1, 0, 6, [][]string{{"X", "Y"}, {"1", "2"}})
	must(t, err)
	sheet, err := d.CreatePivot(ctx, PivotConfig{Source: "Feuille1", RowFields: []string{"Pays"}, Values: []PivotValue{{Field: "Ventes", Agg: "sum"}}})
	must(t, err)
	layout := func(s int64) Layout {
		t.Helper()
		l, err := d.Layout(ctx, s)
		must(t, err)
		return l
	}
	l := layout(sheet)
	if len(l.Pivots) != 1 {
		t.Fatalf("tcd : %+v", l.Pivots)
	}
	id := l.Pivots[0].ID
	// En-tête, FR, IT, total : 4 lignes × 2 colonnes.
	if r := pivotRect(l.Pivots[0]); r.R1-r.R0 != 3 || r.C1-r.C0 != 1 {
		t.Fatalf("taille du TCD : %+v", r)
	}
	tableAt := func(l Layout, name string) (int, int) {
		t.Helper()
		for _, tb := range l.Tables {
			if tb.Name == name {
				return tb.Row, tb.Col
			}
		}
		t.Fatalf("tableau %q absent : %+v", name, l.Tables)
		return 0, 0
	}
	l1 := layout(1)
	if len(l1.Tables) != 2 {
		t.Fatalf("deux tableaux attendus : %+v", l1.Tables)
	}
	b := ""
	for _, tb := range l1.Tables {
		if tb.Name != "Feuille1" {
			b = tb.Name
		}
	}

	// Vers une autre feuille, à une place libre : rien ne bouge.
	before := dump(t, d)
	must(t, d.MovePivot(ctx, id, 1, 5, 0))
	if len(layout(sheet).Pivots) != 0 {
		t.Fatal("le TCD est resté sur sa feuille")
	}
	l1 = layout(1)
	if len(l1.Pivots) != 1 || l1.Pivots[0].Row != 5 || l1.Pivots[0].Col != 0 {
		t.Fatalf("déplacé sur la feuille 1 : %+v", l1.Pivots)
	}
	if r, c := tableAt(l1, "Feuille1"); r != 0 || c != 0 {
		t.Fatalf("A a bougé : %d,%d", r, c)
	}
	undoOnce(t, d, before, "déplacement vers une autre feuille")
	must(t, d.MovePivot(ctx, id, 1, 5, 0))

	// Sur B (lignes 1-4, colonnes 5-6) : B part à droite, une colonne vide
	// entre eux (colonne 8) ; A, loin, ne bouge pas.
	before = dump(t, d)
	must(t, d.MovePivot(ctx, id, 1, 1, 5))
	l1 = layout(1)
	if p := l1.Pivots[0]; p.Row != 1 || p.Col != 5 {
		t.Fatalf("TCD : %+v", p)
	}
	if r, c := tableAt(l1, b); r != 0 || c != 8 {
		t.Fatalf("B poussé en 0,8 attendu : %d,%d", r, c)
	}
	if r, c := tableAt(l1, "Feuille1"); r != 0 || c != 0 {
		t.Fatalf("A a bougé : %d,%d", r, c)
	}
	check(t, d, "TCD posé sur B")
	undoOnce(t, d, before, "TCD posé sur B")

	// Contre A, sans colonne vide (colonnes 2-3) : A part à droite du TCD
	// (colonne 5), et pousse B à son tour (colonne 8).
	before = dump(t, d)
	must(t, d.MovePivot(ctx, id, 1, 0, 2))
	l1 = layout(1)
	if p := l1.Pivots[0]; p.Row != 0 || p.Col != 2 {
		t.Fatalf("TCD : %+v", p)
	}
	if r, c := tableAt(l1, "Feuille1"); r != 0 || c != 5 {
		t.Fatalf("A poussé en 0,5 attendu : %d,%d", r, c)
	}
	if r, c := tableAt(l1, b); r != 0 || c != 8 {
		t.Fatalf("B poussé en 0,8 attendu : %d,%d", r, c)
	}
	check(t, d, "TCD contre A")
	undoOnce(t, d, before, "TCD contre A")

	// Un autre tableau croisé recouvert part lui aussi, avec sa marge.
	must(t, d.MovePivot(ctx, id, 1, 10, 0))
	n2, err := d.CreatePivotAt(ctx, 1, 10, 3, PivotConfig{Source: "Feuille1", RowFields: []string{"Pays"}, Values: []PivotValue{{Field: "Ventes", Agg: "sum"}}})
	must(t, err)
	before = dump(t, d)
	must(t, d.MovePivot(ctx, id, 1, 11, 2))
	for _, p := range layout(1).Pivots {
		if p.ID == n2 && (p.Row != 10 || p.Col != 5) {
			t.Fatalf("TCD voisin poussé en 10,5 attendu : %+v", p)
		}
	}
	undoOnce(t, d, before, "TCD sur un autre TCD")

	// Même place : rien à annuler de plus.
	must(t, d.MovePivot(ctx, id, 1, 10, 0))
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := dump(t, d); got == before {
		t.Fatal("un déplacement sur place a laissé une étape d'annulation")
	}

	// Feuille inconnue : refusé.
	if err := d.MovePivot(ctx, id, 999, 0, 0); err == nil {
		t.Fatal("déplacé vers une feuille inconnue")
	}
}

// Insertion › Tableau croisé puis un clic : CreatePivotAt pose le TCD à la
// cellule, écarte le tableau source qu'il recouvre, et s'annule en une fois.
func TestCreatePivotAt(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes"}, {"FR", "10"}, {"IT", "5"}})
	must(t, err)
	before := dump(t, d)
	cfg := PivotConfig{Source: "Feuille1", RowFields: []string{"Pays"}, Values: []PivotValue{{Field: "Ventes", Agg: "sum"}}}
	id, err := d.CreatePivotAt(ctx, 1, 1, 1, cfg)
	must(t, err)
	l, err := d.Layout(ctx, 1)
	must(t, err)
	if len(l.Sheets) != 1 {
		t.Fatalf("une nouvelle feuille a été créée : %+v", l.Sheets)
	}
	if len(l.Pivots) != 1 || l.Pivots[0].ID != id || l.Pivots[0].Row != 1 || l.Pivots[0].Col != 1 || l.Pivots[0].Config.Name == "" {
		t.Fatalf("TCD : %+v", l.Pivots)
	}
	// Le TCD occupe les colonnes 1-2 : le tableau part en colonne 4.
	if tb := l.Tables[0]; tb.Row != 0 || tb.Col != 4 {
		t.Fatalf("tableau poussé en 0,4 attendu : %+v", tb)
	}
	check(t, d, "TCD créé sur le tableau")
	undoOnce(t, d, before, "création du TCD à une cellule")
	// Configuration impossible : refusé, rien ne change.
	if _, err := d.CreatePivotAt(ctx, 1, 10, 0, PivotConfig{Source: "Inconnu", RowFields: []string{"Pays"}}); err == nil {
		t.Fatal("TCD d'une source inconnue créé")
	}
	if got := dump(t, d); got != before {
		t.Fatalf("refus incomplet :\n%s", diff(before, got))
	}
}
