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

// TestMesureOperations chronomètre les opérations d'un classeur sur un grand
// tableau (Commandes : client, ville, pays, montant, quantité, date) :
// LATTIS_MESURE=1000000 go test ./internal/canvas -run TestMesureOperations -v
func TestMesureOperations(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("LATTIS_MESURE"))
	if n == 0 {
		t.Skip("LATTIS_MESURE non défini")
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
		t.Logf("%-42s %8.0f ms", nom, float64(time.Since(start).Microseconds())/1000)
	}
	mesure(fmt.Sprintf("import de %d lignes × 6 colonnes", n), func() error {
		_, err := d.Import(ctx, "Importer", []ImportSheet{{Name: "Données", Tables: []ImportTable{{
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
	mesure("mise en page de la feuille", func() error { _, err := d.Layout(ctx, sheet); return err })
	mesure("saisie d'une cellule", func() error { _, err := d.SetCell(ctx, sheet, 5000, 3, "12,5"); return err })
	mesure("annuler la saisie", func() error { _, err := d.bus.Undo(ctx); return err })
	mesure("insérer une ligne au milieu", func() error { return d.InsertRow(ctx, "Commandes", n/2) })
	mesure("supprimer cette ligne", func() error { return d.DeleteRow(ctx, "Commandes", n/2) })
	mesure("colonne calculée [@Montant]*[@Qté]", func() error {
		_, err := d.SetCell(ctx, sheet, 0, 6, "Total")
		if err == nil {
			_, err = d.SetCell(ctx, sheet, 1, 6, "=[@Montant]*[@Qté]")
		}
		return err
	})
	mesure("SOMME.SI sur la colonne (cellule libre)", func() error {
		_, err := d.SetCell(ctx, sheet, 0, 10, `=SOMME.SI(Commandes[Pays];"FR";Commandes[Montant])`)
		return err
	})
	mesure("mise en page avec formule libre", func() error { _, err := d.Layout(ctx, sheet); return err })
	mesure("… la même, sans changement (cache)", func() error { _, err := d.Layout(ctx, sheet); return err })
	mesure("valeurs du filtre (Ville)", func() error { _, err := d.FilterValues(ctx, "Commandes", 1); return err })
	mesure("filtrer Ville = Lyon", func() error {
		return d.SetFilter(ctx, "Commandes", 1, &ColumnFilter{Values: []string{"Lyon"}})
	})
	mesure("mise en page filtrée", func() error { _, err := d.Layout(ctx, sheet); return err })
	mesure("effacer le filtre", func() error { return d.ClearFilters(ctx, "Commandes") })
	mesure("trier par Montant", func() error { return d.SortTable(ctx, "Commandes", 3, true) })
	mesure("annuler le tri", func() error { _, err := d.bus.Undo(ctx); return err })
	mesure("rechercher « Client 4999 »", func() error { _, err := d.Find(ctx, sheet, "Client 4999"); return err })
	mesure("aperçu des doublons (Client, Ville)", func() error { _, err := d.DuplicatesPreview(ctx, "Commandes", []int{0, 1}); return err })
	var pivotSheet int64
	mesure("créer un TCD Pays × Ville, somme", func() error {
		var err error
		pivotSheet, err = d.CreatePivot(ctx, PivotConfig{Source: "Commandes", RowFields: []string{"Pays"}, Cols: "Ville", Values: []PivotValue{{Field: "Montant", Agg: "sum"}, {Agg: "count"}}})
		return err
	})
	mesure("mise en page du TCD (calcul)", func() error { _, err := d.Layout(ctx, pivotSheet); return err })
	mesure("… la même, sans changement (cache)", func() error { _, err := d.Layout(ctx, pivotSheet); return err })
	mesure("liste des objets (aide, onglet SQL)", func() error { _, err := d.AllTables(ctx); return err })
	mesure("graphique Pays (somme du Montant)", func() error {
		_, err := d.CreateChart(ctx, sheet, 0, 12, ChartConfig{Source: "Commandes", Kind: "bar", X: "Pays", Series: []string{"Montant"}})
		return err
	})
	mesure("mise en page avec graphique", func() error { _, err := d.Layout(ctx, sheet); return err })
	mesure("requête SQL : UPDATE de 10 % des lignes", func() error {
		_, err := d.ExecSQL(ctx, `UPDATE "Commandes" SET "Qté" = "Qté" + 1 WHERE "Ville" = 'Lyon'`)
		return err
	})
	mesure("annuler la requête", func() error { _, err := d.bus.Undo(ctx); return err })
	mesure("vue SQL du TCD", func() error {
		q, err := d.PivotCTE(ctx, `SELECT * FROM "TCD1"`)
		if err != nil {
			return err
		}
		return d.read(ctx, func(t *tx) error {
			rs, err := t.query(q)
			if err != nil {
				return err
			}
			defer rs.Close()
			for rs.Next() {
			}
			return rs.Err()
		})
	})
}
