package bridge

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/doc"
)

// xlsxFile écrit un petit fichier .xlsx (Enregistrer sous en .xlsx).
func xlsxFile(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	d, err := doc.New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range []string{"Produit", "Prix"} {
		if _, err := d.SetCell(ctx, 1, 0, i, v); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(t.TempDir(), "Ventes.xlsx")
	if err := d.Save(ctx, path); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// Fichier .xlsx ouvert : titre « Ventes.xlsx » ; le premier Enregistrer
// demande (garder ou convertir), garder enregistre dans le .xlsx et ne
// redemande plus ; réglage keep : la question ne porte que sur les pertes.
func TestXLSXSaveQuestion(t *testing.T) {
	workRoot(t)
	isolateRecent(t)
	defer config.SetCurrent(config.Current())
	ctx := context.Background()
	path := xlsxFile(t)
	s := &DocumentService{}
	t.Cleanup(func() { s.CloseDocument(ctx) })
	r, err := s.Open(ctx, path)
	if err != nil || r.Locked != nil || r.Shared != nil {
		t.Fatalf("ouverture : %+v %v", r, err)
	}
	if r.State.Title != "Ventes.xlsx" || r.State.Format != "xlsx" || r.State.Dirty {
		t.Fatalf("état : %+v", r.State)
	}
	l, err := s.Layout(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	sheet := l.Sheets[0].ID
	if _, err := s.SetCell(ctx, sheet, 3, 0, "ajout"); err != nil {
		t.Fatal(err)
	}
	st, err := s.Save(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if st.XLSX == nil || !st.XLSX.Ask || st.XLSX.File != "Ventes.xlsx" || !st.Dirty {
		t.Fatalf("question attendue : %+v", st)
	}
	if st, err = s.SaveXLSX(ctx, true); err != nil || st.Dirty || st.XLSX != nil || st.Path != path {
		t.Fatalf("garder le .xlsx : %+v %v", st, err)
	}
	if _, err := s.SetCell(ctx, sheet, 4, 0, "encore"); err != nil {
		t.Fatal(err)
	}
	if st, err = s.Save(ctx, false); err != nil || st.XLSX != nil || st.Dirty {
		t.Fatalf("deuxième enregistrement : sans question : %+v %v", st, err)
	}

	// Réglage keep : rien à demander tant que le .xlsx garde tout ; une
	// feuille protégée (pas gardée en .xlsx) : la question des pertes.
	config.SetCurrent(config.Resolve(ctx, config.Options{}, &config.TreeSource{N: "essai", R: config.RankUser,
		T: config.Tree{"files": config.Tree{"xlsxSave": "keep"}}}))
	s.CloseDocument(ctx)
	if _, err := s.Open(ctx, path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetCell(ctx, sheet, 5, 0, "x"); err != nil {
		t.Fatal(err)
	}
	if st, err = s.Save(ctx, false); err != nil || st.XLSX != nil || st.Dirty {
		t.Fatalf("keep sans perte : %+v %v", st, err)
	}
	s.CloseDocument(ctx)
	if _, err := s.Open(ctx, path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ProtectSheet(ctx, sheet, ""); err != nil {
		t.Fatal(err)
	}
	if st, err = s.Save(ctx, false); err != nil || st.XLSX == nil || st.XLSX.Ask || !st.XLSX.Report.Lossy() {
		t.Fatalf("keep avec perte : question attendue : %+v %v", st, err)
	}
}
