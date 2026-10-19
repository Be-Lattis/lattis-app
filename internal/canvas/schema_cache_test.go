package canvas

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/meta"
	"github.com/lalamefine/lattis/internal/store"
)

// BenchmarkLayoutAfterEdit : mise en page d'une feuille de 100 formules
// libres dans un classeur de 12 tableaux, après une saisie (caches vidés),
// comme au passage à une autre feuille. ~110 ms avant le schéma gardé par
// génération, ~8 ms après (go test -bench LayoutAfterEdit ./internal/canvas).
func BenchmarkLayoutAfterEdit(b *testing.B) {
	ctx := context.Background()
	db, err := store.Open(filepath.Join(b.TempDir(), "doc.sqlite"), Register)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	if err := meta.Migrate(ctx, db); err != nil {
		b.Fatal(err)
	}
	d := New(db, command.New(db))
	other, err := d.AddSheet(ctx)
	if err != nil {
		b.Fatal(err)
	}
	for k := 0; k < 12; k++ {
		block := [][]string{{"Code", "Valeur", "A", "B", "C", "D", "E", "F"}}
		for r := 0; r < 50; r++ {
			block = append(block, []string{fmt.Sprint("c", r), fmt.Sprint(r), "1", "2", "3", "4", "5", "6"})
		}
		if _, err := d.Paste(ctx, other, 0, k*10, block); err != nil {
			b.Fatal(err)
		}
	}
	for r := 0; r < 100; r++ {
		if _, err := d.SetCell(ctx, 1, 2*r, 0, fmt.Sprintf("=SOMME(Tableau%d[Valeur])+%d", r%11+1, r)); err != nil { // Feuille2, Tableau1 à 11
			b.Fatal(err)
		}
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		b.Fatal(err)
	}
	for _, c := range l.Cells {
		if c.Error {
			b.Fatalf("%s : %s", c.Raw, c.Value)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		if _, err := d.SetCell(ctx, 1, 0, 5, fmt.Sprint(i)); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		if _, err := d.Layout(ctx, 1); err != nil {
			b.Fatal(err)
		}
	}
}

// Changer de feuille relit sa mise en page, qui calcule chaque formule
// libre : le schéma des tableaux (tous les tableaux du classeur, relus en
// base) ne se refait pas pour chacune. Il est gardé pour la génération du
// Store, refait après une écriture et après F9 (instant et graine neufs).
func TestSchemaCachedPerGeneration(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes"}, {"FR", "10"}, {"IT", "5"}}); err != nil {
		t.Fatal(err)
	}
	schema := func() *formula.Schema {
		t.Helper()
		var s *formula.Schema
		if err := d.read(ctx, func(x *tx) error {
			var err error
			s, err = x.schema()
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return s
	}
	a := schema()
	if b := schema(); b != a {
		t.Fatal("schéma refait d'une lecture à l'autre sans écriture")
	}
	if _, err := d.Paste(ctx, 1, 0, 4, [][]string{{"Code", "Prix"}, {"A", "1"}, {"B", "2"}}); err != nil {
		t.Fatal(err)
	}
	b := schema()
	if b == a || len(b.Tables) != 2 {
		t.Fatalf("schéma pas relu après une écriture : %d tableau(x)", len(b.Tables))
	}
	d.Recalc()
	if c := schema(); c == b || !c.Now.After(b.Now) && c.Seed == b.Seed {
		t.Fatal("schéma pas refait après F9 (instant et graine)")
	}
	// Les formules libres lisent toujours le classeur à jour : une colonne
	// ajoutée au tableau après un premier calcul.
	sum := func(row int) string {
		t.Helper()
		l, err := d.Layout(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range l.Cells {
			if c.Row == row && c.Col == 0 {
				return c.Value
			}
		}
		return ""
	}
	if _, err := d.SetCell(ctx, 1, 6, 0, "=SOMME(Tableau1[Prix])"); err != nil {
		t.Fatal(err)
	}
	if v := sum(6); v != "3" {
		t.Fatalf("SOMME(Tableau1[Prix]) = %q, attendu 3", v)
	}
	if err := d.InsertColumn(ctx, "Tableau1", 2, "Qté"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 8, 0, "=NBVAL(Tableau1[Qté])+1"); err != nil {
		t.Fatal(err)
	}
	if v := sum(8); v != "1" {
		t.Fatalf("NBVAL(Tableau1[Qté])+1 = %q, attendu 1 (colonne ajoutée ignorée)", v)
	}
}
