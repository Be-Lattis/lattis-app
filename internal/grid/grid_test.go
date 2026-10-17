package grid

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/lalamefine/slitable/internal/store"
)

func openDemo(t *testing.T, rows int) *Table {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "demo.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	if err := CreateDemo(ctx, db, rows, nil); err != nil {
		t.Fatal(err)
	}
	tbl := Demo(db)
	if err := tbl.Load(ctx); err != nil {
		t.Fatal(err)
	}
	return tbl
}

func TestWindowAcrossAnchors(t *testing.T) {
	const rows = 2*AnchorEvery + 500
	tbl := openDemo(t, rows)
	ctx := context.Background()

	if got := tbl.Info().Rows; got != rows {
		t.Fatalf("Rows = %d, attendu %d", got, rows)
	}
	// La colonne 0 (N° commande) vaut 100000 + rang : elle vérifie l'ordre.
	for _, first := range []int{0, 1, AnchorEvery - 3, AnchorEvery, 2*AnchorEvery + 1, rows - 10} {
		w, err := tbl.Window(ctx, first, 20)
		if err != nil {
			t.Fatalf("Window(%d) : %v", first, err)
		}
		want := min(20, rows-first)
		if len(w.Cells) != want || len(w.IDs) != want {
			t.Fatalf("Window(%d) : %d lignes, attendu %d", first, len(w.Cells), want)
		}
		for i, row := range w.Cells {
			if row[0] != int64(100_000+first+i) {
				t.Fatalf("Window(%d) ligne %d : N° %v, attendu %d", first, i, row[0], 100_000+first+i)
			}
		}
	}
	if _, err := tbl.Window(ctx, rows, 10); !errors.Is(err, ErrRange) {
		t.Fatalf("Window hors tableau : %v", err)
	}
}

func TestSetCell(t *testing.T) {
	tbl := openDemo(t, 100)
	ctx := context.Background()
	w, err := tbl.Window(ctx, 42, 1)
	if err != nil {
		t.Fatal(err)
	}
	id := w.IDs[0]

	v, err := tbl.SetCell(ctx, id, 10, "1 234,5") // Prix unitaire (décimal)
	if err != nil || v != 1234.5 {
		t.Fatalf("SetCell = %v, %v", v, err)
	}
	if _, err := tbl.SetCell(ctx, id, 9, "beaucoup"); !errors.Is(err, ErrValue) {
		t.Fatalf("saisie invalide acceptée : %v", err)
	}
	w, _ = tbl.Window(ctx, 42, 1)
	if w.Cells[0][10] != 1234.5 {
		t.Fatalf("valeur relue %v", w.Cells[0][10])
	}
}

func TestQueryReadOnly(t *testing.T) {
	tbl := openDemo(t, 50)
	ctx := context.Background()
	res, err := Query(ctx, tbl.db.Read(), `SELECT c_0, c_1 FROM t_1 ORDER BY _pos`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 10 || !res.Truncated || res.Columns[1] != "c_1" || res.Rows[0][0] != int64(100_000) {
		t.Fatalf("résultat inattendu : %+v", res)
	}
	if _, err := Query(ctx, tbl.db.Read(), `DELETE FROM t_1`, 10); err == nil {
		t.Fatal("une écriture a été acceptée dans l'onglet SQL")
	}
}

// Stats résume en SQL n'importe quelle plage de lignes, sans limite de taille.
func TestStats(t *testing.T) {
	const rows = 2*AnchorEvery + 500
	tbl := openDemo(t, rows)
	ctx := context.Background()
	// N° commande = 100000 + rang : somme connue.
	first, count := AnchorEvery-5, AnchorEvery+10
	st, err := tbl.Stats(ctx, first, count, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	want := 0.0
	for i := first; i < first+count; i++ {
		want += float64(100_000 + i)
	}
	if st.Count != count || st.Nums != count || st.Sum != want {
		t.Fatalf("Stats = %+v, attendu %d nombres de somme %v", st, count, want)
	}
	// Toute la colonne, et au-delà de la fin : bornée au tableau.
	if st, err = tbl.Stats(ctx, 0, rows+100, []int{0}); err != nil || st.Count != rows {
		t.Fatalf("colonne entière : %+v, %v", st, err)
	}
	if st, err = tbl.Stats(ctx, rows, 10, []int{0}); err != nil || st.Count != 0 {
		t.Fatalf("hors tableau : %+v, %v", st, err)
	}
}
