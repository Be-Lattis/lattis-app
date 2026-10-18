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
		t.Fatal("valeurs uniques : pas de suggestion")
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
		{"vide", func() error { _, err := d.SetCell(ctx, 1, 2, 0, ""); return err }},
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

// Quel que soit le type : texte, décimal, date, dans n'importe quel ordre.
func TestKeyHintAnyType(t *testing.T) {
	ctx := context.Background()
	for _, rows := range [][][]string{
		{{"Code", "Nom"}, {"B", "Ana"}, {"A", "Bob"}},
		{{"Taux", "Nom"}, {"2,5", "Ana"}, {"1,25", "Bob"}},
		{{"Jour", "Nom"}, {"18/10/2026", "Ana"}, {"01/01/2026", "Bob"}},
	} {
		d := open(t)
		if _, err := d.Paste(ctx, 1, 0, 0, rows); err != nil {
			t.Fatal(err)
		}
		if !keyHintOf(t, d) {
			t.Fatalf("%s : valeurs uniques non proposées", rows[0][0])
		}
	}
}
