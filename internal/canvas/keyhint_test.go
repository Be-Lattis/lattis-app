package canvas

import (
	"context"
	"testing"
)

func keyHintOf(t *testing.T, d *fixture) bool {
	t.Helper()
	l, err := d.Layout(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Tables) != 1 {
		t.Fatalf("%d tableau(x)", len(l.Tables))
	}
	return l.Tables[0].KeyHint
}

func TestKeyHint(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"N°", "Nom"}, {"3", "Ana"}, {"7", "Bob"}, {"12", "Cy"}}); err != nil {
		t.Fatal(err)
	}
	if l, _ := d.Layout(ctx, 1); l.Tables[0].Total != 3 {
		t.Fatalf("%d lignes de données", l.Tables[0].Total)
	}
	if !keyHintOf(t, d) {
		t.Fatal("entier unique et croissant : pas de suggestion")
	}
	// Lu deux fois (cache) : même réponse.
	if !keyHintOf(t, d) {
		t.Fatal("seconde lecture")
	}
	steps := []struct {
		what string
		do   func() error
	}{
		{"valeur en double", func() error { _, err := d.SetCell(ctx, 1, 2, 0, "3"); return err }},
		{"décroissante", func() error { _, err := d.SetCell(ctx, 1, 1, 0, "9"); return err }},
		{"vide", func() error { _, err := d.SetCell(ctx, 1, 2, 0, ""); return err }},
		{"hors type", func() error { _, err := d.SetCell(ctx, 1, 2, 0, "abc"); return err }},
		{"tri décroissant", func() error { return d.SortTable(ctx, "Feuille1", 0, true) }},
		{"clé posée", func() error { return d.SetKey(ctx, "Feuille1", 0) }},
		{"ne plus proposer", func() error { return d.DismissKeyHint(ctx, "Feuille1") }},
	}
	for _, s := range steps {
		if err := s.do(); err != nil {
			t.Fatalf("%s : %v", s.what, err)
		}
		if keyHintOf(t, d) {
			t.Fatalf("%s : suggestion gardée", s.what)
		}
		d.bus.Undo(ctx)
		if !keyHintOf(t, d) {
			t.Fatalf("%s annulé : plus de suggestion", s.what)
		}
	}
	// Une seule ligne : rien à proposer.
	for range 2 {
		if err := d.DeleteRow(ctx, "Feuille1", 0); err != nil {
			t.Fatal(err)
		}
	}
	if keyHintOf(t, d) {
		t.Fatal("une seule ligne")
	}
}

func TestKeyHintText(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Code", "Nom"}, {"A", "Ana"}, {"B", "Bob"}}); err != nil {
		t.Fatal(err)
	}
	if keyHintOf(t, d) {
		t.Fatal("colonne texte proposée")
	}
}
