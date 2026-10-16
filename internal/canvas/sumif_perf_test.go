package canvas

import (
	"context"
	"testing"
	"time"

	"github.com/lalamefine/slitable/internal/grid"
)

// Un agrégat sur une colonne calculée par SOMME.SI garde les totaux groupés
// (précalculés) au lieu d'un parcours de Lignes par commande.
func TestSumIfColumnPerf(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	const nc, nl = 600, 20000
	_, err := d.Import(ctx, "Importer", []ImportSheet{{Name: "Données", Tables: []ImportTable{
		{Name: "Commandes", Headers: []string{"Id", "Client"}, Types: []grid.Type{grid.Integer, grid.Integer}, Count: nc,
			Rows: func(y func([]any) error) error {
				for i := range nc {
					if err := y([]any{int64(i), int64(i % 150)}); err != nil {
						return err
					}
				}
				return nil
			}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.Import(ctx, "Importer", []ImportSheet{{Name: "Lignes", Tables: []ImportTable{
		{Name: "Lignes", Headers: []string{"Commande", "Montant"}, Types: []grid.Type{grid.Integer, grid.Decimal}, Count: nl,
			Rows: func(y func([]any) error) error {
				for i := range nl {
					if err := y([]any{int64(i % nc), float64(i%97) + 0.5}); err != nil {
						return err
					}
				}
				return nil
			}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	var sheet int64
	d.read(ctx, func(x *tx) error { return x.row(`SELECT sheet FROM _slitable_tables WHERE name = 'Commandes'`).Scan(&sheet) })
	step := func(name string, fn func() error) {
		s := time.Now()
		if err := fn(); err != nil {
			t.Fatalf("%s : %v", name, err)
		}
		t.Logf("%-30s %v", name, time.Since(s))
	}
	step("colonne SOMME.SI", func() error {
		if _, err := d.SetCell(ctx, sheet, 0, 2, "Montant HT"); err != nil {
			return err
		}
		_, err := d.SetCell(ctx, sheet, 1, 2, "=SOMME.SI(Lignes[Commande];[@Id];Lignes[Montant])")
		return err
	})
	step("MOYENNE de la colonne", func() error {
		_, err := d.SetCell(ctx, sheet, 0, 5, "=MOYENNE(Commandes[Montant HT])")
		return err
	})
	step("mise en page", func() error { _, err := d.Layout(ctx, sheet); return err })
	l, _ := d.Layout(ctx, sheet)
	for _, c := range l.Cells {
		if c.Col == 5 && c.Value != "1615,48166666667" {
			t.Fatalf("MOYENNE = %s", c.Value)
		}
	}
}
