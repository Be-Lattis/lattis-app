package canvas

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func protected(t *testing.T, err error, sheet string) {
	t.Helper()
	var pe ErrProtected
	if !errors.As(err, &pe) || pe.Sheet != sheet {
		t.Fatalf("attendu : protection de « %s », obtenu : %v", sheet, err)
	}
}

// Une feuille protégée refuse toute modification, par la grille comme par le
// SQL ou un script ; les lectures et l'affichage restent permis.
func TestProtectSheet(t *testing.T) {
	ctx := context.Background()
	d := sqlFixture(t) // Feuille1 : tableaux Feuille1 (Client, Montant, Payé) et Tableau1
	other, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectSheet(ctx, 1, "secret"); err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectSheet(ctx, 1, ""); err == nil {
		t.Fatal("protection posée deux fois")
	}
	before := dumpData(t, d)
	if strings.Contains(before, "secret") {
		t.Fatal("mot de passe stocké en clair")
	}

	_, err = d.SetCell(ctx, 1, 1, 1, "999")
	protected(t, err, "Feuille1")
	_, err = d.SetCell(ctx, 1, 10, 10, "libre")
	protected(t, err, "Feuille1")
	_, err = d.Paste(ctx, 1, 10, 10, [][]string{{"a", "b"}})
	protected(t, err, "Feuille1")
	protected(t, d.ApplyStyle(ctx, 1, 1, 1, 1, 1, Style{"b": true}), "Feuille1")
	protected(t, d.ApplyLineStyle(ctx, 1, true, 1, 1, Style{"b": true}), "Feuille1")
	protected(t, d.InsertSheetRows(ctx, 1, 0, 1), "Feuille1")
	protected(t, d.DeleteSheetColumns(ctx, 1, 0, 0), "Feuille1")
	protected(t, d.InsertRow(ctx, "Feuille1", 0), "Feuille1")
	protected(t, d.DeleteColumn(ctx, "Feuille1", 2), "Feuille1")
	protected(t, d.SortTable(ctx, "Feuille1", 1, true), "Feuille1")
	protected(t, d.RenameTable(ctx, "Feuille1", "Ventes"), "Feuille1")
	protected(t, d.SetColumnWidth(ctx, 1, 2, 200), "Feuille1")
	protected(t, d.MoveCells(ctx, 1, 0, 0, 0, 0, 20, 20), "Feuille1")
	_, err = d.ExecSQL(ctx, `UPDATE Feuille1 SET Montant = 0`)
	protected(t, err, "Feuille1")
	_, err = d.ExecSQL(ctx, `DELETE FROM Tableau1`)
	protected(t, err, "Feuille1")
	_, err = d.Editor("script").Insert(ctx, "Feuille1", map[string]string{"Client": "X"})
	protected(t, err, "Feuille1")
	if _, err := d.Editor("script").Exec(ctx, `UPDATE Feuille1 SET Client = 'Z'`); err == nil {
		t.Fatal("script : UPDATE accepté sur une feuille protégée")
	}
	if got := dumpData(t, d); got != before {
		t.Fatalf("feuille protégée modifiée :\n%s", diff(before, got))
	}

	// Permis : filtre, quadrillage, lecture, requête sans effet sur la
	// feuille, et tout ce qui touche une autre feuille.
	if err := d.SetFilter(ctx, "Feuille1", 0, &ColumnFilter{Values: []string{"Durand"}}); err != nil {
		t.Fatalf("filtre refusé : %v", err)
	}
	if err := d.SetGridlines(ctx, 1, true); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ExecSQL(ctx, `UPDATE Feuille1 SET Montant = 0 WHERE 0`); err != nil {
		t.Fatalf("requête sans effet refusée : %v", err)
	}
	if _, err := d.SetCell(ctx, other, 0, 0, "ailleurs"); err != nil {
		t.Fatal(err)
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !l.Sheets[0].Protected || !l.Sheets[0].Password || l.Sheets[1].Protected {
		t.Fatalf("protection mal décrite : %+v", l.Sheets)
	}

	// Mot de passe : faux refusé, juste accepté.
	if err := d.UnprotectSheet(ctx, 1, "faux"); !errors.Is(err, ErrPassword) {
		t.Fatalf("mauvais mot de passe accepté : %v", err)
	}
	if err := d.UnprotectSheet(ctx, 1, "secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 1, "999"); err != nil {
		t.Fatalf("feuille déprotégée encore refusée : %v", err)
	}
	if d.ProtectionChanges() != 2 {
		t.Fatalf("changements de protection : %d", d.ProtectionChanges())
	}
	check(t, d, "protection")
}

// Sans mot de passe, la protection s'ôte directement.
func TestProtectSheetNoPassword(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if err := d.ProtectSheet(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	_, err := d.SetCell(ctx, 1, 0, 0, "x")
	protected(t, err, "Feuille1")
	if err := d.UnprotectSheet(ctx, 1, "n'importe"); err != nil {
		t.Fatal(err)
	}
	if err := d.UnprotectSheet(ctx, 1, ""); err == nil {
		t.Fatal("déprotection d'une feuille non protégée acceptée")
	}
}

// Le classeur protégé refuse d'ajouter, renommer, déplacer ou supprimer des
// feuilles, et verrouille aussi le contenu de toutes les feuilles (choix de
// Louis, 17/10/2026).
func TestProtectWorkbook(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	id, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectWorkbook(ctx, "clé"); err != nil {
		t.Fatal(err)
	}
	_, err = d.AddSheet(ctx)
	protected(t, err, "")
	_, err = d.InsertSheet(ctx, 1)
	protected(t, err, "")
	protected(t, d.RenameSheet(ctx, id, "Budget"), "")
	protected(t, d.DeleteSheet(ctx, id), "")
	protected(t, d.MoveSheet(ctx, id, 0), "")
	_, err = d.SetCell(ctx, id, 0, 0, "contenu")
	protected(t, err, "")
	_, err = d.SetCell(ctx, 1, 0, 0, "contenu")
	protected(t, err, "")
	if l, _ := d.Layout(ctx, 1); !l.WorkbookProtected || !l.WorkbookPassword {
		t.Fatalf("protection du classeur mal décrite : %+v", l)
	}
	if err := d.UnprotectWorkbook(ctx, ""); !errors.Is(err, ErrPassword) {
		t.Fatalf("mot de passe vide accepté : %v", err)
	}
	if err := d.UnprotectWorkbook(ctx, "clé"); err != nil {
		t.Fatal(err)
	}
	if err := d.RenameSheet(ctx, id, "Budget"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, id, 0, 0, "contenu"); err != nil {
		t.Fatalf("contenu refusé après déprotection : %v", err)
	}
}

// Annuler ou rétablir une action antérieure ne contourne pas la protection ;
// les actions sur les autres feuilles s'annulent normalement.
func TestProtectUndo(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	other, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 0, "avant"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, other, 0, 0, "ailleurs"); err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectSheet(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	// La saisie sur l'autre feuille s'annule et se rétablit.
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	// La saisie sur la feuille protégée ne s'annule pas.
	_, err = d.bus.Undo(ctx)
	protected(t, err, "Feuille1")
	if got := dump(t, d); got != before {
		t.Fatalf("annulation refusée mais appliquée :\n%s", diff(before, got))
	}
	// Une fois la protection ôtée, elle s'annule.
	if err := d.UnprotectSheet(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	// Rétablir sur une feuille protégée est refusé aussi.
	if err := d.ProtectSheet(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	_, err = d.bus.Redo(ctx)
	protected(t, err, "Feuille1")

	// Classeur : l'ajout d'une feuille ne s'annule pas une fois la structure
	// protégée.
	if err := d.UnprotectSheet(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := d.AddSheet(ctx); err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectWorkbook(ctx, ""); err != nil {
		t.Fatal(err)
	}
	_, err = d.bus.Undo(ctx)
	protected(t, err, "")
}

// Supprimer une feuille protégée (structure libre) emporte sa protection ;
// l'annulation la rend.
func TestProtectDeleteSheet(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	id, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectSheet(ctx, id, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteSheet(ctx, id); err != nil {
		t.Fatal(err)
	}
	if p, _ := d.Protection(ctx); len(p.Sheets) != 0 {
		t.Fatalf("protection orpheline : %+v", p)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if p, _ := d.Protection(ctx); len(p.Sheets) != 1 {
		t.Fatalf("protection non rendue : %+v", p)
	}
}

func TestPasswordHash(t *testing.T) {
	h, err := hashPassword("mot de passe")
	if err != nil || h == "" || strings.Contains(h, "mot de passe") {
		t.Fatalf("empreinte %q, %v", h, err)
	}
	h2, _ := hashPassword("mot de passe")
	if h == h2 {
		t.Fatal("sel absent : deux empreintes identiques")
	}
	if !checkPassword(h, "mot de passe") || checkPassword(h, "Mot de passe") || checkPassword(h, "") {
		t.Fatal("vérification du mot de passe")
	}
}
