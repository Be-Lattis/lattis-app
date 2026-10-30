package canvas

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/grid"
)

// TestMesureTCD chronomètre la mise en page d'une feuille qui montre un TCD
// et un graphique sur un grand tableau, après une saisie dans une colonne
// qu'ils lisent et dans une colonne qu'ils ne lisent pas :
// LATTIS_MESURE_TCD=1000000 go test ./internal/canvas -run TestMesureTCD -v
func TestMesureTCD(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("LATTIS_MESURE_TCD"))
	if n == 0 {
		t.Skip("LATTIS_MESURE_TCD non défini")
	}
	ctx := context.Background()
	d := open(t)
	villes := []string{"Lyon", "Paris", "Nice", "Brest", "Lille", "Rome", "Milan", "Berlin", "Madrid", "Porto"}
	pays := []string{"FR", "FR", "FR", "FR", "FR", "IT", "IT", "DE", "ES", "PT"}
	mesure := func(nom string, fn func() error) {
		t.Helper()
		start := time.Now()
		if err := fn(); err != nil {
			t.Fatalf("%s : %v", nom, err)
		}
		t.Logf("%-46s %8.0f ms", nom, float64(time.Since(start).Microseconds())/1000)
	}
	var err error
	mesure(fmt.Sprintf("import de %d lignes × 6 colonnes", n), func() error {
		_, err = d.Import(ctx, "Importer", []ImportSheet{{Name: "Données", Tables: []ImportTable{{
			Name: "Commandes", Headers: []string{"Client", "Ville", "Pays", "Montant", "Qté", "Date"},
			Types: []grid.Type{grid.Text, grid.Text, grid.Text, grid.Decimal, grid.Integer, grid.Date},
			Count: n,
			Rows: func(yield func([]any) error) error {
				for i := range n {
					v := i % len(villes)
					if err := yield([]any{"Client " + strconv.Itoa(i%5000), villes[v], pays[v], float64(i%997) + 0.5, int64(i % 13), fmt.Sprintf("2026-%02d-%02d", i%12+1, i%28+1)}); err != nil {
						return err
					}
				}
				return nil
			},
		}}}})
		return err
	})
	var sheet int64
	d.read(ctx, func(t *tx) error {
		return t.row(`SELECT sheet FROM _lattis_tables WHERE name = 'Commandes'`).Scan(&sheet)
	})
	var pivotSheet int64
	mesure("créer un TCD Pays × Ville (somme, nombre)", func() error {
		pivotSheet, err = d.CreatePivot(ctx, PivotConfig{Source: "Commandes", RowFields: []string{"Pays"}, Cols: "Ville", Values: []PivotValue{{Field: "Montant", Agg: "sum"}, {Agg: "count"}}})
		return err
	})
	if _, err := d.CreateChart(ctx, pivotSheet, 0, 14, ChartConfig{Source: "Commandes", Kind: "bar", X: "Pays", Series: []string{"Montant"}}); err != nil {
		t.Fatal(err)
	}
	layout := func() error { _, err := d.Layout(ctx, pivotSheet); return err }
	mesure("mise en page (TCD + graphique, calcul)", layout)
	mesure("… sans changement (cache)", layout)
	mesure("saisie dans Qté (colonne non lue)", func() error { _, err := d.SetCell(ctx, sheet, 5000, 4, "99"); return err })
	mesure("mise en page après cette saisie", layout)
	mesure("saisie dans Montant (colonne lue)", func() error { _, err := d.SetCell(ctx, sheet, 5000, 3, "13,5"); return err })
	mesure("mise en page après cette saisie", layout)
	mesure("trier par Qté", func() error { return d.SortTable(ctx, "Commandes", 4, true) })
	mesure("mise en page après le tri", layout)
}
