package canvas

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lalamefine/slitable/internal/grid"
)

// TestMesureElargissement mesure l'élargissement d'une colonne (entier →
// texte) sur le tableau de démonstration, en STRICT (reconstruction de la
// table) et en ANY (métadonnées seules), pendant que la grille lit des
// fenêtres en parallèle. Lancé seulement avec SLITABLE_MESURE=<lignes>.
func TestMesureElargissement(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("SLITABLE_MESURE"))
	if n <= 0 {
		t.Skip("SLITABLE_MESURE=<nombre de lignes> pour lancer la mesure")
	}
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "mesure.sqlite")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	start := time.Now()
	if err := grid.CreateDemo(ctx, d.db, n, nil); err != nil {
		t.Fatal(err)
	}
	t.Logf("création de %d lignes × %d colonnes : %v", n, len(grid.DemoColumns), time.Since(start).Round(time.Millisecond))
	err = d.Do(ctx, "enregistrer", func(tx *tx) error {
		do := []stmt{st(`INSERT INTO _slitable_tables (id, sheet, name, row, col) VALUES (1, 1, 'Commandes', 0, 0)`)}
		for i, c := range grid.DemoColumns {
			do = append(do, st(`INSERT INTO _slitable_columns (tbl, id, name, type, pos) VALUES (1, ?, ?, ?, ?)`, i, c.Name, string(c.Type), i))
		}
		return tx.apply(do, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	d.undo = nil

	// Lecteur : fenêtres de 200 lignes en boucle, comme la grille.
	var stop atomic.Bool
	var wg sync.WaitGroup
	var worst time.Duration
	var reads, failures int
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			t0 := time.Now()
			rs, err := d.db.Read().QueryContext(ctx, `SELECT * FROM t_1 WHERE _pos >= '' ORDER BY _pos LIMIT 200 OFFSET ?`, reads%800)
			if err == nil {
				for rs.Next() {
				}
				err = rs.Err()
				rs.Close()
			}
			if err != nil {
				failures++
			}
			worst = max(worst, time.Since(t0))
			reads++
			time.Sleep(5 * time.Millisecond)
		}
	}()

	size := func() string {
		var total int64
		for _, s := range []string{"", "-wal"} {
			if st, err := os.Stat(path + s); err == nil {
				total += st.Size()
			}
		}
		return strconv.FormatInt(total/1e6, 10) + " Mo"
	}
	t.Logf("fichier + WAL avant : %s", size())

	// STRICT : reconstruction de la table.
	start = time.Now()
	err = d.Do(ctx, "élargir", func(tx *tx) error {
		tb, err := tx.table(1)
		if err != nil {
			return err
		}
		_, err = tx.retype(tb, 9, grid.Text) // Quantité
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("STRICT, élargissement entier → texte (reconstruction) : %v", time.Since(start).Round(time.Millisecond))
	t.Logf("fichier + WAL après : %s", size())
	start = time.Now()
	if _, err := d.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	t.Logf("STRICT, annulation (reconstruction inverse) : %v", time.Since(start).Round(time.Millisecond))

	stop.Store(true)
	wg.Wait()
	t.Logf("lecteur pendant les reconstructions : %d fenêtres, pire %v, %d échecs", reads, worst.Round(time.Millisecond), failures)

	// ANY : l'élargissement ne touche que les métadonnées.
	start = time.Now()
	err = d.Do(ctx, "élargir (ANY)", func(tx *tx) error {
		q := `UPDATE _slitable_columns SET type = ? WHERE tbl = 1 AND id = 9`
		return tx.apply([]stmt{st(q, "text")}, []stmt{st(q, "integer")})
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ANY, élargissement (métadonnées) : %v", time.Since(start).Round(time.Microsecond))

}
