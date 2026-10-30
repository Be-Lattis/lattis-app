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

// Mesures de la suite de l'audit des performances du 30/10 (TCD seul, tri) :
// elles ne tournent qu'avec leur variable d'environnement.

// TestMesureTCDSeul chronomètre le calcul de TCD de formes diverses sur un
// grand tableau, après une saisie dans une colonne qu'ils lisent (sans
// graphique : le TCD seul) :
// LATTIS_MESURE_TCD_SEUL=1000000 go test ./internal/canvas -run TestMesureTCDSeul -v
func TestMesureTCDSeul(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("LATTIS_MESURE_TCD_SEUL"))
	if n == 0 {
		t.Skip("LATTIS_MESURE_TCD_SEUL non défini")
	}
	ctx := context.Background()
	d := open(t)
	villes := []string{"Lyon", "Paris", "Nice", "Brest", "Lille", "Rome", "Milan", "Berlin", "Madrid", "Porto"}
	pays := []string{"FR", "FR", "FR", "FR", "FR", "IT", "IT", "DE", "ES", "PT"}
	if _, err := d.Import(ctx, "Importer", []ImportSheet{{Name: "Données", Tables: []ImportTable{{
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
	}}}}); err != nil {
		t.Fatal(err)
	}
	var sheet int64
	d.read(ctx, func(t *tx) error {
		return t.row(`SELECT sheet FROM _lattis_tables WHERE name = 'Commandes'`).Scan(&sheet)
	})
	cases := []struct {
		name string
		cfg  PivotConfig
	}{
		{"Pays × Ville (somme, nombre)", PivotConfig{RowFields: []string{"Pays"}, ColFields: []string{"Ville"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}, {Agg: "count"}}}},
		{"Pays, Ville en lignes (somme)", PivotConfig{RowFields: []string{"Pays", "Ville"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}}},
		{"Client × Pays (moyenne, max)", PivotConfig{RowFields: []string{"Client"}, ColFields: []string{"Pays"}, Values: []PivotValue{{Field: "Montant", Agg: "avg"}, {Field: "Qté", Agg: "max"}}}},
		{"Pays × Ville trié (somme)", PivotConfig{RowFields: []string{"Pays"}, ColFields: []string{"Ville"}, Sort: map[string]string{"Pays": "desc"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}}},
		{"Pays × Ville (nombre distinct)", PivotConfig{RowFields: []string{"Pays"}, ColFields: []string{"Ville"}, Values: []PivotValue{{Field: "Client", Agg: "countDistinct"}}}},
	}
	for i, c := range cases {
		c.cfg.Source = "Commandes"
		ps, err := d.CreatePivot(ctx, c.cfg)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.SetCell(ctx, sheet, 5000+i, 3, "13,5"); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		l, err := d.Layout(ctx, ps)
		if err != nil || len(l.Pivots) == 0 || l.Pivots[0].Error != "" {
			t.Fatalf("%s : %v %+v", c.name, err, l.Pivots)
		}
		t.Logf("%-34s %7d ms", c.name, time.Since(start).Milliseconds())
	}
}

// TestMesureTri chronomètre le tri d'un grand tableau de 10 colonnes, étape
// par étape (nombre, texte, annulation, rétablissement) :
// LATTIS_MESURE_TRI=10000000 go test ./internal/canvas -run TestMesureTri -v -timeout 90m
func TestMesureTri(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("LATTIS_MESURE_TRI"))
	if n == 0 {
		t.Skip("LATTIS_MESURE_TRI non défini")
	}
	instant(t)
	ctx := context.Background()
	d := open(t)
	start := time.Now()
	sheet := importRows(t, d, n)
	t.Logf("import de %d lignes × 10 colonnes : %d ms", n, time.Since(start).Milliseconds())
	mesure := func(nom string, fn func(ctx context.Context) error) {
		t.Helper()
		var p phases
		wctx := p.start(ctx, true)
		start := time.Now()
		err := fn(wctx)
		total := time.Since(start)
		p.tr.End()
		if err != nil {
			t.Fatalf("%s : %v", nom, err)
		}
		t.Logf("%-28s %7d ms %s", nom, total.Milliseconds(), p.summary(total))
	}
	mesure("trier par Prix (nombre)", func(ctx context.Context) error { return d.SortTable(ctx, "Commandes", 3, true) })
	mesure("annuler", func(ctx context.Context) error { _, err := d.bus.Undo(ctx); return err })
	mesure("rétablir", func(ctx context.Context) error { _, err := d.bus.Redo(ctx); return err })
	mesure("trier par Client (texte)", func(ctx context.Context) error { return d.SortTable(ctx, "Commandes", 0, false) })
	mesure("mise en page", func(ctx context.Context) error { _, err := d.Layout(ctx, sheet); return err })
}
