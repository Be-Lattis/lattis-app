package canvas

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/remote/oplog"
	"github.com/lalamefine/lattis/internal/store"
)

// TestMesureOplog chronomètre de grandes écritures sans journal logique,
// avec le Recorder sur un tableau non publié, et sur un tableau publié :
// LATTIS_MESURE=1000000 go test ./internal/canvas -run TestMesureOplog -v
func TestMesureOplog(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("LATTIS_MESURE"))
	if n == 0 {
		t.Skip("LATTIS_MESURE non défini")
	}
	store.SetSQLLog(false)
	ctx := context.Background()
	block := make([][]string, 500)
	for i := range block {
		block[i] = []string{"Collé " + strconv.Itoa(i), "Brest", strconv.Itoa(i%997) + ",5"}
	}
	for _, mode := range []string{"sans Recorder", "non publié", "publié"} {
		d := open(t)
		if mode != "sans Recorder" {
			d.bus.Recorder = oplog.New(d.db)
		}
		mesure := func(nom string, fn func() error) {
			t.Helper()
			start := time.Now()
			if err := fn(); err != nil {
				t.Fatalf("%s : %v", nom, err)
			}
			t.Logf("%-14s %-36s %8.0f ms", mode, nom, float64(time.Since(start).Microseconds())/1000)
		}
		mesure(fmt.Sprintf("import de %d lignes", n), func() error {
			_, err := d.Import(ctx, "Importer", []ImportSheet{{Name: "Données", Tables: []ImportTable{{
				Name: "Commandes", Headers: []string{"Client", "Ville", "Montant"},
				Types: []grid.Type{grid.Text, grid.Text, grid.Decimal}, Count: n,
				Rows: func(yield func([]any) error) error {
					for i := range n {
						if err := yield([]any{"Client " + strconv.Itoa(i%5000), "Ville " + strconv.Itoa(i%97), float64(i%997) + 0.5}); err != nil {
							return err
						}
					}
					return nil
				},
			}}}})
			return err
		})
		name := "Commandes"
		var sheet int64
		d.db.Read().QueryRow(`SELECT sheet FROM _lattis_tables WHERE id = 1`).Scan(&sheet)
		if mode == "publié" {
			mesure("publication (identifiants)", func() error { return oplog.Publish(ctx, d.db, 1) })
		}
		mesure("saisie d'une cellule", func() error { _, err := d.SetCell(ctx, sheet, 5000, 2, "12,5"); return err })
		mesure(fmt.Sprintf("collage de %d lignes sous le tableau", len(block)), func() error { _, err := d.Paste(ctx, sheet, n+1, 0, block); return err })
		mesure("annuler le collage", func() error { _, err := d.bus.Undo(ctx); return err })
		mesure("trier par Montant", func() error { return d.SortTable(ctx, name, 2, true) })
		mesure("annuler le tri", func() error { _, err := d.bus.Undo(ctx); return err })
		mesure("SQL : UPDATE de toutes les lignes", func() error {
			_, err := d.ExecSQL(ctx, `UPDATE "`+name+`" SET Montant = Montant + 1`)
			return err
		})
		mesure("annuler l'UPDATE", func() error { _, err := d.bus.Undo(ctx); return err })
		mesure(fmt.Sprintf("SQL : INSERT de %d lignes", n/10), func() error {
			_, err := d.ExecSQL(ctx, `WITH RECURSIVE s(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM s WHERE i < ?) INSERT INTO "`+name+`" (Client, Ville, Montant) SELECT 'Nouveau ' || i, 'Lyon', i FROM s`, n/10)
			return err
		})
		mesure("annuler l'INSERT", func() error { _, err := d.bus.Undo(ctx); return err })
		var entries, size int
		d.db.Read().QueryRow(`SELECT count(*), coalesce(sum(length(ops)), 0) FROM _lattis_outbox`).Scan(&entries, &size)
		t.Logf("%-14s file d'envoi : %d entrées, %d Mo", mode, entries, size>>20)
	}
}
