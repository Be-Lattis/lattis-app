package canvas

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func structureProtected(t *testing.T, err error, sheet string) {
	t.Helper()
	var pe ErrStructureProtected
	if !errors.As(err, &pe) || pe.Sheet != sheet {
		t.Fatalf("attendu : structure protégée de « %s », obtenu : %v", sheet, err)
	}
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

// tableRows rend le nombre de lignes de données d'un tableau.
func (d *fixture) tableRows(t *testing.T, name string) int {
	t.Helper()
	var n int
	err := d.read(context.Background(), func(tx *tx) error {
		tb, err := tx.tableByName(name)
		if err != nil {
			return err
		}
		return tx.row(`SELECT count(*) FROM ` + tableSQL(tb.ID)).Scan(&n)
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// La structure protégée refuse tout sauf la saisie dans les tableaux :
// valeurs, lignes ajoutées ou supprimées, tri et filtre.
func TestProtectStructure(t *testing.T) {
	ctx := context.Background()
	d := sqlFixture(t) // Feuille1 : tableaux Feuille1 (Client, Montant, Payé ; lignes 1 à 3) et Tableau1 (ligne 5)
	other, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 10, 10, "libre"); err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectStructure(ctx, 1, "secret"); err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectStructure(ctx, 1, ""); err == nil {
		t.Fatal("protection de la structure posée deux fois")
	}
	before := dumpData(t, d)
	if strings.Contains(before, "secret") {
		t.Fatal("mot de passe stocké en clair")
	}

	// Refusé : cellules libres, en-têtes, colonnes, mise en forme, lignes de
	// la feuille, objets, onglet, mise en page, SQL d'écriture.
	_, err = d.SetCell(ctx, 1, 10, 10, "autre")
	structureProtected(t, err, "Feuille1")
	_, err = d.SetCell(ctx, 1, 0, 1, "Prix")
	structureProtected(t, err, "Feuille1")
	_, err = d.SetCell(ctx, 1, 1, 3, "nouvelle colonne")
	structureProtected(t, err, "Feuille1")
	_, err = d.Paste(ctx, 1, 20, 20, [][]string{{"a", "b"}})
	structureProtected(t, err, "Feuille1")
	structureProtected(t, d.ApplyStyle(ctx, 1, 1, 1, 1, 1, Style{"b": true}), "Feuille1")
	structureProtected(t, d.ApplyLineStyle(ctx, 1, true, 1, 1, Style{"bg": "#ff0000"}), "Feuille1")
	structureProtected(t, d.SetColumnWidth(ctx, 1, 2, 200), "Feuille1")
	structureProtected(t, d.SetRowHeight(ctx, 1, 2, 40), "Feuille1")
	_, err = d.MergeCells(ctx, 1, 10, 10, 11, 11)
	structureProtected(t, err, "Feuille1")
	rule := []Rule{{Op: ">", Value: "100", Style: Style{"bg": "#ffc7ce"}}}
	structureProtected(t, d.SetRules(ctx, "Feuille1", 1, rule), "Feuille1")
	structureProtected(t, d.SetLineRules(ctx, 1, true, 10, 10, rule), "Feuille1")
	structureProtected(t, d.ClearCells(ctx, 1, 10, 10, 10, 10), "Feuille1")
	structureProtected(t, d.ClearRange(ctx, 1, 1, 1, 1, 1, true), "Feuille1")
	structureProtected(t, d.ClearRange(ctx, 1, 0, 0, 3, 2, false), "Feuille1") // en-têtes compris
	structureProtected(t, d.SetNote(ctx, 1, 1, 1, "note"), "Feuille1")
	structureProtected(t, d.InsertColumn(ctx, "Feuille1", 1, "Nouvelle"), "Feuille1")
	structureProtected(t, d.DeleteColumn(ctx, "Feuille1", 2), "Feuille1")
	structureProtected(t, d.RenameTable(ctx, "Feuille1", "Ventes"), "Feuille1")
	structureProtected(t, d.InsertSheetRows(ctx, 1, 0, 1), "Feuille1")
	structureProtected(t, d.SetLinesHidden(ctx, 1, true, 30, 30, true), "Feuille1")
	structureProtected(t, d.MoveCells(ctx, 1, 10, 10, 10, 10, 1, 1), "Feuille1")
	structureProtected(t, d.RenameSheet(ctx, 1, "Ventes"), "Feuille1")
	structureProtected(t, d.SetSheetColor(ctx, 1, "#ff0000"), "Feuille1")
	structureProtected(t, d.DeleteSheet(ctx, 1), "Feuille1")
	structureProtected(t, d.SetPageSetup(ctx, 1, PageSetup{Landscape: true}), "Feuille1")
	_, err = d.ExecSQL(ctx, `UPDATE Feuille1 SET Montant = 0`)
	structureProtected(t, err, "Feuille1")
	if got := dumpData(t, d); got != before {
		t.Fatalf("structure protégée modifiée :\n%s", diff(before, got))
	}

	// Permis : saisie et collage dans les données, ligne ajoutée dessous,
	// lignes insérées et supprimées, Suppr, tri, filtre, scripts.
	if _, err := d.SetCell(ctx, 1, 1, 1, "999"); err != nil {
		t.Fatalf("saisie refusée : %v", err)
	}
	if _, err := d.Paste(ctx, 1, 2, 1, [][]string{{"5"}, {"6"}}); err != nil {
		t.Fatalf("collage refusé : %v", err)
	}
	if _, err := d.SetCell(ctx, 1, 4, 0, "Petit"); err != nil {
		t.Fatalf("ligne ajoutée sous le tableau refusée : %v", err)
	}
	if n := d.tableRows(t, "Feuille1"); n != 4 {
		t.Fatalf("lignes après ajout : %d", n)
	}
	if err := d.InsertRow(ctx, "Feuille1", 0); err != nil {
		t.Fatalf("insertion refusée : %v", err)
	}
	if err := d.DeleteRow(ctx, "Feuille1", 0); err != nil {
		t.Fatalf("suppression refusée : %v", err)
	}
	if err := d.ClearRange(ctx, 1, 1, 1, 2, 2, false); err != nil {
		t.Fatalf("Suppr dans le tableau refusé : %v", err)
	}
	if err := d.SortTable(ctx, "Feuille1", 0, true); err != nil {
		t.Fatalf("tri refusé : %v", err)
	}
	if err := d.SetFilter(ctx, "Feuille1", 0, &ColumnFilter{Values: []string{"Durand"}}); err != nil {
		t.Fatalf("filtre refusé : %v", err)
	}
	if err := d.SetFilter(ctx, "Feuille1", 0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Editor("script").Insert(ctx, "Feuille1", map[string]string{"Client": "X"}); err != nil {
		t.Fatalf("db.insert refusé : %v", err)
	}
	if got := d.cellRaw(t, 1, 10, 10); got != "libre" {
		t.Fatalf("cellule libre : %q", got)
	}

	// Copier dans Lattis vers les données d'un tableau : valeurs seules ;
	// déplacer depuis une autre feuille : refusé.
	if _, err := d.SetCell(ctx, other, 0, 0, "Copié"); err != nil {
		t.Fatal(err)
	}
	src := Rect{0, 0, 0, 0}
	if _, err := d.Transfer(ctx, other, src, 1, Rect{1, 0, 1, 0}, false, [][]string{{"Copié"}}); err != nil {
		t.Fatalf("copie refusée : %v", err)
	}
	_, err = d.Transfer(ctx, other, src, 1, Rect{2, 0, 2, 0}, true, [][]string{{"Copié"}})
	structureProtected(t, err, "Feuille1")
	_, err = d.Transfer(ctx, other, src, 1, Rect{20, 0, 20, 0}, false, [][]string{{"Copié"}})
	structureProtected(t, err, "Feuille1")

	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !l.Sheets[0].StructureProtected || !l.Sheets[0].StructurePassword || l.Sheets[0].Protected || l.Sheets[1].StructureProtected {
		t.Fatalf("protection mal décrite : %+v", l.Sheets)
	}

	// La protection de la feuille l'emporte.
	if err := d.ProtectSheet(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	_, err = d.SetCell(ctx, 1, 1, 1, "1")
	protected(t, err, "Feuille1")
	if err := d.UnprotectSheet(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}

	// Mot de passe : faux refusé, juste accepté.
	if err := d.UnprotectStructure(ctx, 1, "faux"); !errors.Is(err, ErrPassword) {
		t.Fatalf("mauvais mot de passe accepté : %v", err)
	}
	if err := d.UnprotectStructure(ctx, 1, "secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 10, 10, "autre"); err != nil {
		t.Fatalf("structure déprotégée encore refusée : %v", err)
	}
	if err := d.UnprotectStructure(ctx, 1, ""); err == nil {
		t.Fatal("déprotection d'une structure non protégée acceptée")
	}
	check(t, d, "protection de la structure")
}

// L'annulation ne contourne pas la structure protégée : une commande
// antérieure qui l'a changée ne s'annule ni ne se rétablit ; une saisie
// antérieure dans un tableau, et les commandes faites depuis, s'annulent.
func TestProtectStructureUndo(t *testing.T) {
	ctx := context.Background()
	d := sqlFixture(t)
	// Une cellule libre, une saisie dans le tableau, puis la protection.
	if _, err := d.SetCell(ctx, 1, 10, 10, "libre"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 1, "77"); err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectStructure(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteRow(ctx, "Feuille1", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatalf("annulation d'une suppression de ligne refusée : %v", err)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatalf("annulation d'une saisie dans le tableau refusée : %v", err)
	}
	before := dump(t, d)
	// La cellule libre, antérieure, ne s'annule pas.
	_, err := d.bus.Undo(ctx)
	structureProtected(t, err, "Feuille1")
	if got := dump(t, d); got != before {
		t.Fatalf("annulation refusée mais appliquée :\n%s", diff(before, got))
	}
	// Ôtée, elle s'annule ; reposée, le rétablissement est refusé.
	if err := d.UnprotectStructure(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectStructure(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	_, err = d.bus.Redo(ctx)
	structureProtected(t, err, "Feuille1")
}

// Protéger le classeur avec la structure de toutes les feuilles : onglets
// verrouillés, chaque feuille comme Protéger la structure ; le mot de passe
// du classeur ôte tout ; l'annulation ne contourne pas le verrou.
func TestProtectWorkbookStructure(t *testing.T) {
	ctx := context.Background()
	d := sqlFixture(t)
	other, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, other, 0, 0, "libre"); err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectWorkbook(ctx, "clé", false, true); err != nil {
		t.Fatal(err)
	}
	isBook := func(err error) {
		t.Helper()
		var pe ErrStructureProtected
		if !errors.As(err, &pe) || !pe.Book || !strings.Contains(err.Error(), "Ôter la protection du classeur") {
			t.Fatalf("attendu : structure du classeur protégée, obtenu : %v", err)
		}
	}
	// Annuler la cellule libre, antérieure : refusé.
	_, err = d.bus.Undo(ctx)
	isBook(err)
	_, err = d.SetCell(ctx, other, 1, 1, "x")
	isBook(err)
	isBook(d.ApplyStyle(ctx, 1, 1, 1, 1, 1, Style{"b": true}))
	isBook(d.InsertColumn(ctx, "Feuille1", 1, "Nouvelle"))
	protected(t, d.RenameSheet(ctx, 1, "Ventes"), "")
	_, err = d.AddSheet(ctx)
	protected(t, err, "")
	if _, err := d.SetCell(ctx, 1, 1, 1, "999"); err != nil {
		t.Fatalf("saisie dans le tableau refusée : %v", err)
	}
	if err := d.InsertRow(ctx, "Feuille1", 0); err != nil {
		t.Fatalf("insertion refusée : %v", err)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatalf("annulation de l'insertion refusée : %v", err)
	}
	p, err := d.Protection(ctx)
	if err != nil || !p.Workbook || !p.WorkbookStructure || p.WorkbookSheets {
		t.Fatalf("protection mal décrite : %+v %v", p, err)
	}
	if err := d.UnprotectWorkbook(ctx, "clé"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, other, 1, 1, "x"); err != nil {
		t.Fatalf("structure encore verrouillée : %v", err)
	}
	if p, _ := d.Protection(ctx); p.WorkbookStructure {
		t.Fatal("verrou de la structure resté")
	}
}
