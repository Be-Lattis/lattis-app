package transfer

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Le compte rendu d'export dit ce qui n'est pas exporté tel quel : graphique,
// TCD en valeurs, formules sans équivalent (CUMUL, FILTRE), règle de mise
// en forme conditionnelle, note.
func TestExportReport(t *testing.T) {
	ctx := context.Background()
	d := exportFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := d.SetCell(ctx, 1, 0, 5, "Cumul")
	must(err)
	_, err = d.SetCell(ctx, 1, 1, 5, "=CUMUL([Prix])")
	must(err)
	_, err = d.SetCell(ctx, 1, 14, 0, `=FILTRE(Ventes[Produit]; Ventes[Prix] > 1)`)
	must(err)
	_, err = d.CreateChart(ctx, 1, 20, 0, canvas.ChartConfig{Source: "Ventes", Kind: "bar", X: "Produit", Series: []string{"Prix"}})
	must(err)
	must(d.SetRules(ctx, "Ventes", 1, []canvas.Rule{{ID: "r1", Op: ">", Value: "1", Style: canvas.Style{"bg": "#FF0000"}}}))
	must(d.SetNote(ctx, 1, 6, 0, "À vérifier"))
	_, err = d.CreatePivot(ctx, canvas.PivotConfig{Source: "Ventes", Rows: "Produit", Value: "Prix", Agg: "sum"})
	must(err)

	for _, ext := range []string{".xlsx", ".ods"} {
		p := filepath.Join(t.TempDir(), "export"+ext)
		var rep ExportReport
		if ext == ".xlsx" {
			rep, err = ExportXLSXFile(ctx, d.Model, p)
		} else {
			rep, err = ExportODSFile(ctx, d.Model, p)
		}
		must(err)
		if len(rep.Sheets) != 3 {
			t.Fatalf("%s : %d feuilles", ext, len(rep.Sheets))
		}
		s := rep.Sheets[0]
		lost := 1 // graphique, règle et note : exportés en .xlsx
		if ext == ".xlsx" {
			lost = 0
		}
		if s.Name != "Données" || s.Charts != lost || s.Rules != lost || s.Notes != lost || s.Tables != 1 {
			t.Errorf("%s : feuille Données : %+v", ext, s)
		}
		// FILTRE (cellule), TEXTE en .ods (cellule), CUMUL (colonne calculée).
		cells, fns := 2, []string{"CUMUL", "FILTRE", "TEXTE"}
		if ext == ".xlsx" {
			cells, fns = 1, fns[:2]
		}
		if s.Formulas != cells || s.Columns != 1 {
			t.Errorf("%s : formules en valeurs : %+v", ext, s)
		}
		for _, f := range fns {
			if !slices.Contains(s.Functions, f) {
				t.Errorf("%s : %s attendue parmi %v", ext, f, s.Functions)
			}
		}
		if s.Translated == 0 {
			t.Errorf("%s : aucune formule traduite : %+v", ext, s)
		}
		if p := rep.Sheets[2]; p.Pivots != 1 {
			t.Errorf("%s : TCD : %+v", ext, p)
		}
		if n := rep.Sheets[1]; n.Charts+n.Pivots+n.Formulas+n.Columns+n.Rules+n.Notes != 0 {
			t.Errorf("%s : feuille Notes : %+v", ext, n)
		}
	}
}
