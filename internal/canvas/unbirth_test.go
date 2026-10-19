package canvas

import (
	"context"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/command"
)

func withScript(ctx context.Context) context.Context { return command.WithGroup(ctx, "Script") }

// Un tableau créé par une saisie : le premier Ctrl+Z le défait, les cellules
// restent libres (la dernière saisie comprise) et marquées « plage » ; le
// second annule la saisie ; rétablir refait l'un puis l'autre.
func TestUndoBirthTyping(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	typeIn(t, d, 0, 0, [][]string{{"Nom", "Age"}})
	beforeLast := dump(t, d)
	// La 3e cellule du carré 2 × 2 fait naître le tableau.
	mustSet(t, d, 1, 1, 0, "Léa")
	if got := layoutOf(t, d); got != "Feuille1@A1:Nom|Age×1" {
		t.Fatalf("naissance : %s", got)
	}
	born := dump(t, d)
	d.TakeNotice()
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	check(t, d, "création défaite")
	if got := layoutOf(t, d); got != "A1=Nom B1=Age A2=Léa" {
		t.Fatalf("création défaite : %s", got)
	}
	if got := rangesOf(t, d, 1); len(got) != 1 || got[0] != (Rect{0, 0, 1, 1}) {
		t.Fatalf("plage : %+v", got)
	}
	if n := d.TakeNotice(); n != "Tableau « Feuille1 » défait : les cellules restent libres (Ctrl+Z de nouveau annule la saisie)" {
		t.Fatalf("message : %q", n)
	}
	// Second Ctrl+Z : la saisie de Léa.
	undoOnce(t, d, beforeLast, "saisie")
	// Rétablir : la saisie (cellules libres en plage), puis le tableau.
	if _, err := d.bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := layoutOf(t, d); got != "A1=Nom B1=Age A2=Léa" {
		t.Fatalf("saisie rétablie : %s", got)
	}
	if _, err := d.bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := dump(t, d); got != born {
		t.Fatalf("création rétablie :\n%s", diff(born, got))
	}
	// Défaite de nouveau, la zone ne refait pas de tableau en s'agrandissant.
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	mustSet(t, d, 1, 1, 1, "30")
	mustSet(t, d, 1, 2, 0, "Paul")
	mustSet(t, d, 1, 2, 1, "40")
	if got := layoutOf(t, d); got != "A1=Nom B1=Age A2=Léa B2=30 A3=Paul B3=40" {
		t.Fatalf("plage agrandie : %s", got)
	}
}

// Un collage qui crée un tableau se défait en deux temps aussi.
func TestUndoBirthPaste(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	before := dump(t, d)
	if ev, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "CP"}, {"Paris", "75000"}, {"Lyon", "69000"}}); err != nil || !ev.Split {
		t.Fatalf("collage %+v, %v", ev, err)
	}
	if st, _ := d.bus.State(ctx); st.Undo != "Création du tableau « Feuille1 »" {
		t.Fatalf("étape : %q", st.Undo)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := layoutOf(t, d); got != "A1=Ville B1=CP A2=Paris B2=75000 A3=Lyon B3=69000" {
		t.Fatalf("création défaite : %s", got)
	}
	if n := d.TakeNotice(); !strings.HasSuffix(n, "(Ctrl+Z de nouveau annule le collage)") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "collage")
}

// Une saisie qui agrandit un tableau existant (sans en créer) reste une
// seule étape ; dans un script (groupe d'actions), la création ne se coupe
// pas non plus.
func TestUndoBirthOnlyWhenCreated(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	typeIn(t, d, 0, 0, ventesRows)
	before := dump(t, d)
	mustSet(t, d, 1, 4, 0, "Kiwis")
	undoOnce(t, d, before, "nouvelle ligne")

	e := open(t)
	start := dump(t, e)
	gctx := withScript(ctx)
	if ev, err := e.Paste(gctx, 1, 0, 0, [][]string{{"Ville", "CP"}, {"Paris", "75000"}}); err != nil || ev.Split {
		t.Fatalf("collage %+v, %v", ev, err)
	}
	undoOnce(t, e, start, "collage dans un script")
}
