package canvas

import (
	"context"
	"testing"
)

// dvRaw : la propriété dv qui s'applique à la cellule (nil : aucune).
func dvRaw(t *testing.T, d *fixture, sheet int64, row, col int) any {
	t.Helper()
	var v any
	must(t, d.read(context.Background(), func(tx *tx) error {
		tables, err := tx.tables(sheet)
		if err != nil {
			return err
		}
		_, v, err = tx.dvRawAt(sheet, row, col, tables)
		return err
	}))
	if empty(v) {
		return nil
	}
	return v
}

// La recopie porte la validation de la source (cellules libres) : D6
// recopiée de D5 refuse ce que D5 refuse (la source la garde, même si un
// tableau naît) ; une seule annulation retire valeurs et règles.
func TestFillCarriesValidationFree(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	rule := map[string]any{"type": "whole", "op": "<", "min": "3"}
	must(t, d.ApplyStyle(ctx, 1, 4, 3, 4, 3, Style{"dv": rule}))
	_, err := d.SetCell(ctx, 1, 4, 3, "1")
	must(t, err)
	// Vers le bas : D6, D7.
	_, err = d.Fill(ctx, 1, Rect{4, 3, 4, 3}, 5, 3, [][]string{{"1"}, {"1"}})
	must(t, err)
	for _, r := range []int{5, 6} {
		if st, _ := refusal(t, d, typing, 1, r, 3, "99"); st != "stop" {
			t.Fatalf("ligne %d recopiée : 99 accepté", r)
		}
	}
	// Annuler la recopie : valeurs et règles partent ensemble.
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	for _, r := range []int{5, 6} {
		if v := dvRaw(t, d, 1, r, 3); v != nil {
			t.Fatalf("ligne %d : règle restée après Annuler : %v", r, v)
		}
		if got := freeRawAt(t, d, 1, r, 3); got != "" {
			t.Fatalf("ligne %d : valeur restée après Annuler : %q", r, got)
		}
	}
	if dvRaw(t, d, 1, 4, 3) == nil {
		t.Fatal("règle de la source perdue")
	}
}

// Une source sans validation la retire de la destination (comme un collage).
func TestFillClearsValidation(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	must(t, d.ApplyStyle(ctx, 1, 6, 1, 6, 1, Style{"dv": []any{"Oui", "Non"}}))
	_, err := d.SetCell(ctx, 1, 5, 1, "x")
	must(t, err)
	_, err = d.Fill(ctx, 1, Rect{5, 1, 5, 1}, 6, 1, [][]string{{"x"}})
	must(t, err)
	if v := dvRaw(t, d, 1, 6, 1); v != nil {
		t.Fatalf("validation gardée : %v", v)
	}
}

// Sans validation nulle part, la recopie reste un collage de valeurs.
func TestFillWithoutValidation(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	_, err := d.SetCell(ctx, 1, 0, 0, "1")
	must(t, err)
	_, err = d.Fill(ctx, 1, Rect{0, 0, 0, 0}, 1, 0, [][]string{{"2"}, {"3"}})
	must(t, err)
	if got := freeRawAt(t, d, 1, 2, 0); got != "3" {
		t.Fatalf("A3 : %q", got)
	}
	if v := dvRaw(t, d, 1, 2, 0); v != nil {
		t.Fatalf("validation inventée : %v", v)
	}
}

// Dans un tableau : la règle d'une cellule passe aux lignes recopiées, vers
// le bas comme vers le haut.
func TestFillCarriesValidationTable(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Nom", "Qté"}, {"a", "1"}, {"b", "2"}, {"c", "2"}})
	must(t, err)
	must(t, d.ApplyStyle(ctx, 1, 1, 1, 1, 1, Style{"dv": map[string]any{"type": "whole", "op": "<", "min": "3"}}))
	if st, _ := refusal(t, d, typing, 1, 2, 1, "99"); st != "" {
		t.Fatal("ligne 3 refusée avant la recopie")
	}
	_, err = d.Fill(ctx, 1, Rect{1, 1, 1, 1}, 2, 1, [][]string{{"1"}, {"1"}})
	must(t, err)
	for _, r := range []int{2, 3} {
		if st, _ := refusal(t, d, typing, 1, r, 1, "99"); st != "stop" {
			t.Fatalf("ligne %d du tableau recopiée : 99 accepté", r)
		}
	}
	d2 := open(t)
	_, err = d2.Paste(ctx, 1, 0, 0, [][]string{{"Nom", "Qté"}, {"a", "1"}, {"b", "2"}, {"c", "2"}})
	must(t, err)
	must(t, d2.ApplyStyle(ctx, 1, 3, 1, 3, 1, Style{"dv": map[string]any{"type": "whole", "op": "<", "min": "3"}}))
	_, err = d2.Fill(ctx, 1, Rect{3, 1, 3, 1}, 2, 1, [][]string{{"2"}})
	must(t, err)
	if st, _ := refusal(t, d2, typing, 1, 2, 1, "99"); st != "stop" {
		t.Fatal("recopie vers le haut : 99 accepté")
	}
	if st, _ := refusal(t, d2, typing, 1, 1, 1, "99"); st != "" {
		t.Fatal("ligne au-dessus de la recopie : 99 refusé")
	}
}
