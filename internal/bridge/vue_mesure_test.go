package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/grid"
)

// TestMesureVue chronomètre ce que l'interface demande pour afficher la
// grille d'un gros classeur (docs/audits/vue-d-abord-gros-classeurs-2026-10-22.md) :
// à l'ouverture, au retour d'une autre feuille, et l'ordre des appels.
// LATTIS_MESURE=1000000 go test ./internal/bridge -run TestMesureVue -v
func TestMesureVue(t *testing.T) {
	// LATTIS_MESURE_OUVRIR : ouverture seule d'un classeur déjà fait.
	if p := os.Getenv("LATTIS_MESURE_OUVRIR"); p != "" {
		ctx := context.Background()
		s := &DocumentService{}
		start := time.Now()
		if _, err := s.Open(ctx, p); err != nil {
			t.Fatal(err)
		}
		t.Logf("Open : %v", time.Since(start))
		start = time.Now()
		if _, err := s.LayoutFirst(ctx, 1); err != nil {
			t.Fatal(err)
		}
		t.Logf("LayoutFirst : %v", time.Since(start))
		s.CloseDocument(ctx)
		return
	}
	n, _ := strconv.Atoi(os.Getenv("LATTIS_MESURE"))
	if n == 0 {
		t.Skip("LATTIS_MESURE non défini")
	}
	ctx := context.Background()
	// LATTIS_MESURE_FICHIER : le classeur est gardé là (essai dans l'application).
	path := os.Getenv("LATTIS_MESURE_FICHIER")
	if path == "" {
		path = filepath.Join(t.TempDir(), "gros.lattis")
	}
	os.Remove(path)
	s := &DocumentService{}
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	d, err := s.current()
	if err != nil {
		t.Fatal(err)
	}
	headers := make([]string, 10)
	types := make([]grid.Type, 10)
	for i := range headers {
		headers[i] = fmt.Sprintf("Col%d", i+1)
		types[i] = grid.Text
		if i%2 == 1 {
			types[i] = grid.Integer
		}
	}
	types[0] = grid.Integer
	start := time.Now()
	if _, err := d.Import(ctx, "Importer", []canvas.ImportSheet{{Name: "Données", Tables: []canvas.ImportTable{{
		Name: "Gros", Headers: headers, Types: types, Count: n,
		Rows: func(yield func([]any) error) error {
			row := make([]any, 10)
			for i := range n {
				for c := range row {
					if types[c] == grid.Integer {
						row[c] = int64(i*(c+1)) % 100003
					} else {
						row[c] = "Texte " + strconv.Itoa(i%7919+c)
					}
				}
				row[0] = int64(i + 1)
				if err := yield(row); err != nil {
					return err
				}
			}
			return nil
		},
	}}}}); err != nil {
		t.Fatal(err)
	}
	t.Logf("import %d × 10 : %v", n, time.Since(start))
	if err := d.Save(ctx, path); err != nil {
		t.Fatal(err)
	}
	other, err := s.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Save(ctx, path); err != nil {
		t.Fatal(err)
	}
	s.CloseDocument(ctx)

	mesure := func(nom string, fn func() error) time.Duration {
		t.Helper()
		start := time.Now()
		if err := fn(); err != nil {
			t.Fatalf("%s : %v", nom, err)
		}
		el := time.Since(start)
		t.Logf("%-48s %8.1f ms", nom, float64(el.Microseconds())/1000)
		return el
	}
	mesure("Open", func() error { _, err := s.Open(ctx, path); return err })
	var sheet, table int64
	l, _ := s.Layout(ctx, 0)
	for _, sh := range l.Sheets {
		if sh.Name == "Données" {
			sheet = sh.ID
		}
	}
	// Ce que fait reload() à l'ouverture : tout lancé ensemble, la grille
	// attend State et LayoutFirst, puis demande ses blocs ; Layout complète
	// ensuite (suggestion de clé, numérotation).
	var wg sync.WaitGroup
	side := func(nom string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mesure("  (en parallèle) "+nom, fn)
		}()
	}
	t0 := time.Now()
	side("FormulaHelp", func() error { _, err := s.FormulaHelp(ctx); return err })
	side("Scripts", func() error { _, err := s.Scripts(ctx); return err })
	side("Favorites", func() error { _, err := s.Favorites(ctx); return err })
	side("SimilarSheets", func() error { _, err := s.SimilarSheets(ctx); return err })
	mesure("State", func() error { _, err := s.State(ctx); return err })
	mesure("LayoutFirst (ouverture)", func() error {
		l, err := s.LayoutFirst(ctx, sheet)
		if err == nil && len(l.Tables) > 0 {
			table = l.Tables[0].ID
		}
		return err
	})
	mesure("Window 0+200 (premier bloc)", func() error { _, err := s.Window(ctx, sheet, table, 0, 200); return err })
	t.Logf("première vue (ouverture) : %v", time.Since(t0))
	wg.Wait()
	mesure("Layout complet (après la vue)", func() error { _, err := s.Layout(ctx, sheet); return err })
	mesure("Layout (en cache)", func() error { _, err := s.Layout(ctx, sheet); return err })
	mesure("Window 200+200", func() error { _, err := s.Window(ctx, sheet, table, 200, 200); return err })
	mesure("saisie (5e colonne)", func() error { _, err := s.SetCell(ctx, sheet, 10, 4, "7"); return err })
	mesure("Layout après la saisie", func() error { _, err := s.Layout(ctx, sheet); return err })
	mesure("Window 0+200 après la saisie", func() error { _, err := s.Window(ctx, sheet, table, 0, 200); return err })
	mesure("Layout (autre feuille)", func() error { _, err := s.Layout(ctx, other); return err })
	t0 = time.Now()
	mesure("Layout (retour)", func() error { _, err := s.Layout(ctx, sheet); return err })
	mesure("Window 0+200 (retour)", func() error { _, err := s.Window(ctx, sheet, table, 0, 200); return err })
	t.Logf("première vue (retour) : %v", time.Since(t0))
	mesure("Layout (sans changement)", func() error { _, err := s.Layout(ctx, sheet); return err })
	mesure("FormulaHelp (seul)", func() error { _, err := s.FormulaHelp(ctx); return err })
	s.CloseDocument(ctx)
}
