package canvas

import (
	"context"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

// tableValues : valeurs gardées d'une colonne (nil : NULL), dans l'ordre
// des lignes, et son type.
func tableValues(t *testing.T, d *fixture, table string, col int) ([]any, grid.Type) {
	t.Helper()
	var out []any
	var typ grid.Type
	if err := d.read(context.Background(), func(tx *tx) error {
		tb, err := tx.tableByName(table)
		if err != nil {
			return err
		}
		typ = tb.Cols[col].Type
		rs, err := tx.query(`SELECT ` + colSQL(tb.Cols[col].ID) + ` FROM ` + tb.sql() + ` ORDER BY _pos`)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var v any
			if err := rs.Scan(&v); err != nil {
				return err
			}
			out = append(out, v)
		}
		return rs.Err()
	}); err != nil {
		t.Fatal(err)
	}
	return out, typ
}

// Dates avec heure saisies dans un tableau, dans chaque langue de
// l'interface et dans les deux ordres (demande de Louis, 23/10/2026).
func TestDateTimeEntryLanguages(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct{ lang, first, second, third string }{
		{"fr", "23/10/2026 14:30", "14:30 24/10/2026", "25/10/2026 08:15:30"},
		{"es", "23/10/2026 14:30", "14:30 24/10/2026", "25/10/2026 08:15:30"},
		{"de", "23.10.2026 14:30", "14:30 24.10.2026", "25.10.2026 08:15:30"},
		{"en", "10/23/2026 2:30 PM", "14:30 10/24/2026", "10/25/2026 8:15:30 AM"},
		{"zh", "2026/10/23 14:30", "14:30 2026/10/24", "2026-10-25 08:15:30"},
	} {
		d := open(t)
		speak(d, c.lang)
		// Collé (les deux premières lignes), puis tapé sous le tableau.
		if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Quand", "Qui"}, {c.first, "a"}, {c.second, "b"}}); err != nil {
			t.Fatalf("%s : %v", c.lang, err)
		}
		if _, err := d.SetCell(ctx, 1, 3, 0, c.third); err != nil {
			t.Fatalf("%s %q : %v", c.lang, c.third, err)
		}
		if err := promoteIfData(ctx, d, "Feuille1"); err != nil {
			t.Fatal(err)
		}
		vals, typ := tableValues(t, d, "Feuille1", 0)
		want := []any{"2026-10-23 14:30", "2026-10-24 14:30", "2026-10-25 08:15:30"}
		if typ != grid.Date || len(vals) != 3 || vals[0] != want[0] || vals[1] != want[1] || vals[2] != want[2] {
			t.Errorf("%s : %s %v", c.lang, typ, vals)
		}
	}
}

// Collage d'un bloc : colonne de dates avec heure en Date, « null » en
// case vide (NULL), une date seule reste une date.
func TestDateTimeAndNullPaste(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{
		{"Quand", "Nom", "N"},
		{"23/10/2026 14:30", "Léa", "1"},
		{"14:30:15 24/10/2026", "null", "NULL"},
		{"25/10/2026", "Paul", "3"},
		{"null", " Null ", "4"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := promoteIfData(ctx, d, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	when, typ := tableValues(t, d, "Feuille1", 0)
	if typ != grid.Date || when[0] != "2026-10-23 14:30" || when[1] != "2026-10-24 14:30:15" || when[2] != "2026-10-25" || when[3] != nil {
		t.Errorf("Quand : %s %v", typ, when)
	}
	names, typ := tableValues(t, d, "Feuille1", 1)
	if typ != grid.Text || names[0] != "Léa" || names[1] != nil || names[3] != nil {
		t.Errorf("Nom : %s %v", typ, names)
	}
	n, typ := tableValues(t, d, "Feuille1", 2)
	if typ != grid.Integer || n[1] != nil || n[0] != int64(1) {
		t.Errorf("N : %s %v", typ, n)
	}
	// « null » tapé dans une case remplie la vide ; « 'null » reste du texte.
	if _, err := d.SetCell(ctx, 1, 1, 1, "NULL"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 3, 1, "'null"); err != nil {
		t.Fatal(err)
	}
	names, _ = tableValues(t, d, "Feuille1", 1)
	if names[0] != nil || names[2] != "null" {
		t.Errorf("Nom après saisie : %v", names)
	}
	// Une colonne Texte vide qui reçoit « null » garde son type.
	if _, err := d.SetCell(ctx, 1, 2, 2, "null"); err != nil {
		t.Fatal(err)
	}
	if n, typ = tableValues(t, d, "Feuille1", 2); typ != grid.Integer || n[1] != nil {
		t.Errorf("N après saisie : %s %v", typ, n)
	}
}

// Une cellule libre : date avec heure gardée jj/mm/aaaa hh:mm (lue par les
// formules), heure d'abord comprise ; « null » y reste un texte.
func TestDateTimeFreeCell(t *testing.T) {
	ctx := context.Background()
	stored := StorageForm(ctx)
	d := open(t)
	speak(d, "en")
	for r, c := range []struct{ typed, kept string }{
		{"2:30 PM 10/23/2026", "23/10/2026 14:30"},
		{"10/23/2026 14:30:15", "23/10/2026 14:30:15"},
		{"null", "null"},
	} {
		if _, err := d.SetCell(ctx, 1, r*3, 4, c.typed); err != nil {
			t.Fatal(err)
		}
		if raw, _ := freeCell(t, stored, d, r*3, 4); raw != c.kept {
			t.Errorf("%q gardé %q, attendu %q", c.typed, raw, c.kept)
		}
	}
	if _, err := d.SetCell(ctx, 1, 9, 4, "=HEURE(E1)"); err != nil {
		t.Fatal(err)
	}
	if _, v := freeCell(t, stored, d, 9, 4); v != "14" {
		t.Errorf("HEURE : %q", v)
	}
	speak(d, "de")
	if raw, _ := freeCell(t, ctx, d, 0, 4); raw != "23.10.2026 14:30" {
		t.Errorf("allemand : %q", raw)
	}
}
