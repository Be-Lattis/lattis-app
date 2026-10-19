package canvas

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func formatProtected(t *testing.T, err error, sheet string) {
	t.Helper()
	var pe ErrFormatProtected
	if !errors.As(err, &pe) || pe.Sheet != sheet {
		t.Fatalf("attendu : mise en forme protégée de « %s », obtenu : %v", sheet, err)
	}
}

// freeStyle rend le style d'une cellule libre de la feuille ("" sans style).
func freeStyle(t *testing.T, d *fixture, sheet int64, row, col int) string {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range l.Styles {
		if s.Row == row && s.Col == col {
			return s.Style
		}
	}
	return ""
}

// La mise en forme protégée refuse styles, largeurs, fusions et règles ;
// la saisie, les lignes, le tri, les notes et l'effacement du contenu
// restent permis.
func TestProtectFormat(t *testing.T) {
	ctx := context.Background()
	d := sqlFixture(t) // Feuille1 : tableaux Feuille1 (Client, Montant, Payé) et Tableau1
	other, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 10, 10, "libre"); err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectFormat(ctx, 1, "secret"); err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectFormat(ctx, 1, ""); err == nil {
		t.Fatal("protection de la mise en forme posée deux fois")
	}
	before := dumpData(t, d)
	if strings.Contains(before, "secret") {
		t.Fatal("mot de passe stocké en clair")
	}

	formatProtected(t, d.ApplyStyle(ctx, 1, 1, 1, 1, 1, Style{"b": true}), "Feuille1")
	formatProtected(t, d.ApplyStyle(ctx, 1, 10, 10, 10, 10, Style{"nf": "%"}), "Feuille1")
	formatProtected(t, d.ApplyLineStyle(ctx, 1, true, 1, 1, Style{"bg": "#ff0000"}), "Feuille1")
	formatProtected(t, d.SetColumnWidth(ctx, 1, 2, 200), "Feuille1")
	formatProtected(t, d.SetRowHeight(ctx, 1, 2, 40), "Feuille1")
	_, err = d.MergeCells(ctx, 1, 10, 10, 11, 11)
	formatProtected(t, err, "Feuille1")
	rule := []Rule{{Op: ">", Value: "100", Style: Style{"bg": "#ffc7ce"}}}
	formatProtected(t, d.SetRules(ctx, "Feuille1", 1, rule), "Feuille1")
	formatProtected(t, d.SetLineRules(ctx, 1, true, 10, 10, rule), "Feuille1")
	formatProtected(t, d.SetSheetRules(ctx, 1, RuleChanges{Cells: []RangeRules{{R0: 10, C0: 10, R1: 10, C1: 10, Rules: rule}}}), "Feuille1")
	formatProtected(t, d.ClearCells(ctx, 1, 10, 10, 10, 10), "Feuille1")
	formatProtected(t, d.ClearRange(ctx, 1, 10, 10, 10, 10, true), "Feuille1")
	if got := dumpData(t, d); got != before {
		t.Fatalf("mise en forme protégée modifiée :\n%s", diff(before, got))
	}

	// Permis : saisie, collage, lignes, tri, notes, masquage, effacement du
	// contenu.
	if _, err := d.SetCell(ctx, 1, 1, 1, "999"); err != nil {
		t.Fatalf("saisie refusée : %v", err)
	}
	if _, err := d.Paste(ctx, 1, 20, 20, [][]string{{"a", "b"}}); err != nil {
		t.Fatalf("collage refusé : %v", err)
	}
	if err := d.InsertRow(ctx, "Feuille1", 0); err != nil {
		t.Fatalf("insertion refusée : %v", err)
	}
	if err := d.SortTable(ctx, "Feuille1", 1, true); err != nil {
		t.Fatalf("tri refusé : %v", err)
	}
	if err := d.SetNote(ctx, 1, 10, 10, "note"); err != nil {
		t.Fatalf("note refusée : %v", err)
	}
	if err := d.SetLinesHidden(ctx, 1, true, 30, 30, true); err != nil {
		t.Fatalf("masquage refusé : %v", err)
	}
	if err := d.ClearRange(ctx, 1, 20, 20, 20, 21, false); err != nil {
		t.Fatalf("effacement du contenu refusé : %v", err)
	}

	// Copier dans Lattis vers la feuille : valeurs seules ; déplacer depuis
	// une autre feuille : refusé.
	if _, err := d.SetCell(ctx, other, 0, 0, "gras"); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyStyle(ctx, other, 0, 0, 0, 0, Style{"b": true}); err != nil {
		t.Fatal(err)
	}
	src := Rect{0, 0, 0, 0}
	if _, err := d.Transfer(ctx, other, src, 1, Rect{40, 0, 40, 0}, false, [][]string{{"gras"}}); err != nil {
		t.Fatalf("copie refusée : %v", err)
	}
	if v := d.cellRaw(t, 1, 40, 0); v != "gras" {
		t.Fatalf("copie : %q", v)
	}
	if s := freeStyle(t, d, 1, 40, 0); s != "" {
		t.Fatalf("copie : mise en forme collée %q", s)
	}
	_, err = d.Transfer(ctx, other, src, 1, Rect{41, 0, 41, 0}, true, [][]string{{"gras"}})
	formatProtected(t, err, "Feuille1")

	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !l.Sheets[0].FormatProtected || !l.Sheets[0].FormatPassword || l.Sheets[0].Protected || l.Sheets[1].FormatProtected {
		t.Fatalf("protection mal décrite : %+v", l.Sheets)
	}

	// La protection de la feuille l'emporte.
	if err := d.ProtectSheet(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	protected(t, d.ApplyStyle(ctx, 1, 1, 1, 1, 1, Style{"b": true}), "Feuille1")
	if err := d.UnprotectSheet(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}

	// Mot de passe : faux refusé, juste accepté.
	if err := d.UnprotectFormat(ctx, 1, "faux"); !errors.Is(err, ErrPassword) {
		t.Fatalf("mauvais mot de passe accepté : %v", err)
	}
	if err := d.UnprotectFormat(ctx, 1, "secret"); err != nil {
		t.Fatal(err)
	}
	if err := d.ApplyStyle(ctx, 1, 1, 1, 1, 1, Style{"b": true}); err != nil {
		t.Fatalf("mise en forme déprotégée encore refusée : %v", err)
	}
	if err := d.UnprotectFormat(ctx, 1, ""); err == nil {
		t.Fatal("déprotection d'une mise en forme non protégée acceptée")
	}
	check(t, d, "protection de la mise en forme")
}

// cellRaw rend la saisie d'une cellule libre.
func (d *fixture) cellRaw(t *testing.T, sheet int64, row, col int) string {
	t.Helper()
	var raw string
	err := d.read(context.Background(), func(tx *tx) error {
		return tx.row(`SELECT coalesce(max(raw), '') FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?`, sheet, row, col).Scan(&raw)
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// L'annulation ne contourne pas la mise en forme protégée : une commande
// antérieure qui l'a changée ne s'annule ni ne se rétablit ; les commandes
// faites depuis (saisie, suppression d'une ligne mise en forme) s'annulent.
func TestProtectFormatUndo(t *testing.T) {
	ctx := context.Background()
	d := sqlFixture(t)
	// Une cellule du tableau en gras, une saisie libre, puis la protection.
	if err := d.ApplyStyle(ctx, 1, 1, 1, 1, 1, Style{"b": true}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 10, 10, "libre"); err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectFormat(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	// Supprimer la ligne en gras (sa mise en forme part avec elle), puis
	// l'annuler : permis, la commande suit la protection.
	if err := d.DeleteRow(ctx, "Feuille1", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatalf("annulation d'une suppression de ligne refusée : %v", err)
	}
	// La saisie libre, antérieure mais sans mise en forme, s'annule.
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatalf("annulation d'une saisie refusée : %v", err)
	}
	before := dump(t, d)
	// La mise en forme antérieure ne s'annule pas.
	_, err := d.bus.Undo(ctx)
	formatProtected(t, err, "Feuille1")
	if got := dump(t, d); got != before {
		t.Fatalf("annulation refusée mais appliquée :\n%s", diff(before, got))
	}
	// Ôtée, elle s'annule ; reposée, le rétablissement est refusé.
	if err := d.UnprotectFormat(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectFormat(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	_, err = d.bus.Redo(ctx)
	formatProtected(t, err, "Feuille1")
}

// Supprimer une feuille emporte la protection de sa mise en forme ;
// l'annulation la rend.
func TestProtectFormatDeleteSheet(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	id, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectFormat(ctx, id, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteSheet(ctx, id); err != nil {
		t.Fatal(err)
	}
	if p, _ := d.Protection(ctx); len(p.Formats) != 0 {
		t.Fatalf("protection orpheline : %+v", p)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if p, _ := d.Protection(ctx); len(p.Formats) != 1 {
		t.Fatalf("protection non rendue : %+v", p)
	}
}
