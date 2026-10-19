package bridge

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Regroupement puis lecture comme la grille : changer la valeur de l'onglet
// à choix (« (Toutes) ») gardait en cache le tableau de la grille avec le
// nombre de lignes de l'ancienne valeur, d'où « grid: hors du tableau » pour
// les blocs suivants (store.tableMeta ne voyait pas _lattis_sheets.choice).

// readAll lit, comme la grille, chaque tableau de chaque feuille par blocs
// de 200 lignes (Layout puis Window).
func readAll(t *testing.T, s *DocumentService, ctx context.Context, when string) {
	t.Helper()
	l, err := s.Layout(ctx, 1)
	if err != nil {
		t.Fatalf("%s : %v", when, err)
	}
	for _, sh := range l.Sheets {
		ls, err := s.Layout(ctx, sh.ID)
		if err != nil {
			t.Errorf("%s, feuille %s : %v", when, sh.Name, err)
			continue
		}
		for _, tb := range ls.Tables {
			for first := 0; first < tb.Rows; first += 200 {
				if _, err := s.Window(ctx, sh.ID, tb.ID, first, 200); err != nil {
					t.Errorf("%s, %s.%s lignes %d+ (%d affichées sur %d) : %v", when, sh.Name, tb.Name, first, tb.Rows, tb.Total, err)
				}
			}
		}
	}
}

func TestMergeSheetsThenRead(t *testing.T) {
	ctx := context.Background()
	s := &DocumentService{}
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.CloseDocument(ctx) })
	var tables []string
	var sheets []int64
	for i, m := range []string{"Janvier", "Février", "Mars"} {
		id := int64(1)
		if i > 0 {
			var err error
			if id, err = s.AddSheet(ctx); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.RenameSheet(ctx, id, m); err != nil {
			t.Fatal(err)
		}
		block := [][]string{{"Client", "Montant"}}
		for r := range 250 + 100*i {
			block = append(block, []string{fmt.Sprintf("C%d", r), fmt.Sprint(r)})
		}
		if _, err := s.Paste(ctx, id, 0, 0, block); err != nil {
			t.Fatal(err)
		}
		if i == 0 { // un second tableau sur la feuille : elle ne part pas
			if _, err := s.Paste(ctx, id, 0, 5, [][]string{{"Remise", "Taux"}, {"Or", "5"}}); err != nil {
				t.Fatal(err)
			}
		}
		l, _ := s.Layout(ctx, id)
		tables = append(tables, l.Tables[0].Name)
		sheets = append(sheets, id)
	}
	readAll(t, s, ctx, "avant")
	res, err := s.MergeSheets(ctx, canvas.MergePlan{Tables: tables, Name: "Ventes", Old: canvas.MergeOldDelete})
	if err != nil {
		t.Fatal(err)
	}
	readAll(t, s, ctx, "après regroupement")
	if _, err := s.ChooseSheetValue(ctx, res.Sheet, "", true); err != nil {
		t.Fatal(err)
	}
	readAll(t, s, ctx, "toutes les valeurs")
	if _, err := s.ChooseSheetValue(ctx, res.Sheet, "Janvier", false); err != nil {
		t.Fatal(err)
	}
	readAll(t, s, ctx, "janvier")
	// Janvier porte un autre tableau : gardé (et dit), puis supprimé à la main.
	l, _ := s.Layout(ctx, res.Sheet)
	if len(l.Sheets) != 2 || l.Sheets[1].ID != sheets[0] || !strings.Contains(res.Notice, "« Janvier » gardée") {
		t.Fatalf("onglets après regroupement : %+v ; %s", l.Sheets, res.Notice)
	}
	if _, err := s.DeleteSheet(ctx, sheets[0]); err != nil {
		t.Fatalf("suppression à la main : %v", err)
	}
	readAll(t, s, ctx, "après suppression")
}
