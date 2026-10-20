package canvas

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Seul tableau d'une feuille : son nom et celui de l'onglet se suivent.

var soleBlock = [][]string{{"Ville", "Habitants"}, {"Paris", "2100000"}, {"Lyon", "520000"}}

// soleNames rend le nom de l'onglet et ceux de ses tableaux.
func soleNames(t *testing.T, d *fixture, sheet int64) (string, string) {
	t.Helper()
	var name string
	var tables []string
	err := d.read(context.Background(), func(tx *tx) error {
		if err := tx.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&name); err != nil {
			return err
		}
		rs, err := tx.query(`SELECT name FROM _lattis_tables WHERE sheet = ? ORDER BY id`, sheet)
		if err != nil {
			return err
		}
		return eachRow(rs, func() error {
			var n string
			if err := rs.Scan(&n); err != nil {
				return err
			}
			tables = append(tables, n)
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	return name, strings.Join(tables, ",")
}

func soleWant(t *testing.T, d *fixture, sheet int64, sheetName, tables string) {
	t.Helper()
	if s, tb := soleNames(t, d, sheet); s != sheetName || tb != tables {
		t.Fatalf("onglet %q, tableaux %q ; attendu %q, %q", s, tb, sheetName, tables)
	}
}

func TestSoleTableFollowsSheet(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if ev, err := d.Paste(ctx, 1, 2, 1, soleBlock); err != nil || ev.Created != "Feuille1" {
		t.Fatalf("Paste = %+v, %v", ev, err)
	}
	// Renommer l'onglet renomme le tableau (sans espace), en une étape.
	if err := d.RenameSheetLinked(ctx, 1, "Ventes 2026"); err != nil {
		t.Fatal(err)
	}
	soleWant(t, d, 1, "Ventes 2026", "Ventes2026")
	check(t, d, "onglet renommé")
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	soleWant(t, d, 1, "Feuille1", "Feuille1")
	// Et l'inverse : renommer le tableau renomme l'onglet.
	if err := d.RenameTableLinked(ctx, "Feuille1", "Clients"); err != nil {
		t.Fatal(err)
	}
	soleWant(t, d, 1, "Clients", "Clients")
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	soleWant(t, d, 1, "Feuille1", "Feuille1")
	// Scripts, assistant (RenameSheet, RenameTable) : l'un sans l'autre.
	if err := d.RenameSheet(ctx, 1, "Seul"); err != nil {
		t.Fatal(err)
	}
	soleWant(t, d, 1, "Seul", "Feuille1")
	// Un nom qui ne peut pas nommer un tableau (adresse) : l'onglet seul.
	d.TakeNotice()
	if err := d.RenameSheetLinked(ctx, 1, "Q1"); err != nil {
		t.Fatal(err)
	}
	soleWant(t, d, 1, "Q1", "Feuille1")
	if n := d.TakeNotice(); !strings.Contains(n, "garde son nom « Feuille1 »") {
		t.Fatalf("barre d'état : %q", n)
	}
}

func TestSoleTableConflicts(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 2, 1, soleBlock); err != nil {
		t.Fatal(err)
	}
	s2, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ev, err := d.Paste(ctx, s2, 2, 1, soleBlock); err != nil || ev.Created != "Feuille2" {
		t.Fatalf("Paste = %+v, %v", ev, err)
	}
	s3, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Nom de tableau pris : l'onglet seul est renommé, la barre d'état le dit.
	d.TakeNotice()
	if err := d.RenameSheetLinked(ctx, 1, "feuille 2"); err != nil {
		t.Fatal(err)
	}
	soleWant(t, d, 1, "feuille 2", "Feuille1")
	if n := d.TakeNotice(); !strings.Contains(n, "déjà le nom d'un autre tableau") {
		t.Fatalf("barre d'état : %q", n)
	}
	// Nom de feuille pris (l'onglet vide Feuille3) : le tableau seul.
	if err := d.RenameTableLinked(ctx, "Feuille1", "Feuille3"); err != nil {
		t.Fatal(err)
	}
	soleWant(t, d, 1, "feuille 2", "Feuille3")
	soleWant(t, d, s3, "Feuille3", "")
	if n := d.TakeNotice(); !strings.Contains(n, "déjà le nom d'une autre feuille") {
		t.Fatalf("barre d'état : %q", n)
	}
	// Deux tableaux sur la feuille : plus de lien.
	if ev, err := d.Paste(ctx, s2, 2, 10, soleBlock); err != nil || ev.Created != "Tableau1" {
		t.Fatalf("Paste = %+v, %v", ev, err)
	}
	if err := d.RenameSheetLinked(ctx, s2, "Deux"); err != nil {
		t.Fatal(err)
	}
	soleWant(t, d, s2, "Deux", "Feuille2,Tableau1")
	if err := d.RenameTableLinked(ctx, "Feuille2", "Premier"); err != nil {
		t.Fatal(err)
	}
	soleWant(t, d, s2, "Deux", "Premier,Tableau1")
}

// Le lien naît sans rien renommer : une feuille passée de deux tableaux à un
// garde ses noms, le renommage suivant les accorde.
func TestSoleTableLinkStarts(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 2, 1, soleBlock); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Paste(ctx, 1, 2, 10, soleBlock); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteTables(ctx, []string{"Feuille1"}); err != nil {
		t.Fatal(err)
	}
	soleWant(t, d, 1, "Feuille1", "Tableau1")
	if err := d.RenameTableLinked(ctx, "Tableau1", "Stock"); err != nil {
		t.Fatal(err)
	}
	soleWant(t, d, 1, "Stock", "Stock")
}

// Naissance : le premier tableau d'une feuille prend le nom de l'onglet, s'il
// peut nommer un tableau et n'est pas pris ; sinon « TableauN ».
func TestSoleTableBirthName(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if err := d.RenameSheet(ctx, 1, "Budget prévu"); err != nil {
		t.Fatal(err)
	}
	if ev, err := d.Paste(ctx, 1, 4, 3, soleBlock); err != nil || ev.Created != "Budgetprévu" {
		t.Fatalf("Paste = %+v, %v", ev, err)
	}
	s2, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.RenameSheet(ctx, s2, "T1"); err != nil {
		t.Fatal(err)
	}
	if ev, err := d.Paste(ctx, s2, 2, 1, soleBlock); err != nil || ev.Created != "Tableau1" {
		t.Fatalf("Paste = %+v, %v", ev, err)
	}
}

type layoutLocked struct{}

func (layoutLocked) TableLock(int64) error   { return nil }
func (layoutLocked) TableShare(int64) string { return "private" }
func (layoutLocked) LayoutLock() error       { return errors.New("concepteurs seulement") }

// Classeur collaboratif, non-concepteur : l'onglet ne se renomme pas, donc
// son seul tableau non plus (refus du tout, message ordinaire).
func TestSoleTableLayoutLocked(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 2, 1, soleBlock); err != nil {
		t.Fatal(err)
	}
	d.SetSharing(layoutLocked{})
	if err := d.RenameTableLinked(ctx, "Feuille1", "Clients"); err == nil || !strings.Contains(err.Error(), "concepteurs seulement") {
		t.Fatalf("renommage permis : %v", err)
	}
	if err := d.RenameSheetLinked(ctx, 1, "Clients"); err == nil {
		t.Fatal("onglet renommé")
	}
	soleWant(t, d, 1, "Feuille1", "Feuille1")
}

// Classeur protégé (onglets figés) : le tableau seul est renommé.
func TestSoleTableProtectedTab(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 2, 1, soleBlock); err != nil {
		t.Fatal(err)
	}
	if err := d.ProtectWorkbook(ctx, "", false, false); err != nil {
		t.Fatal(err)
	}
	d.TakeNotice()
	if err := d.RenameTableLinked(ctx, "Feuille1", "Clients"); err != nil {
		t.Fatal(err)
	}
	soleWant(t, d, 1, "Feuille1", "Clients")
	if n := d.TakeNotice(); !strings.Contains(n, "protégé") {
		t.Fatalf("barre d'état : %q", n)
	}
}
