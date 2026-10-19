package canvas

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/meta"
	"github.com/lalamefine/lattis/internal/store"
)

// freeDV lit la propriété dv d'une cellule libre (JSON de son style).
func freeDV(t *testing.T, d *fixture, sheet int64, row, col int) any {
	t.Helper()
	var s string
	err := d.read(context.Background(), func(tx *tx) error {
		return tx.row(`SELECT coalesce((SELECT style FROM _lattis_cell_styles WHERE sheet = ? AND row = ? AND col = ?), '')`, sheet, row, col).Scan(&s)
	})
	must(t, err)
	return parseStyle(s)["dv"]
}

func dvList(t *testing.T, d *fixture, sheet int64, src string) string {
	t.Helper()
	l, err := d.ValidationList(context.Background(), sheet, src)
	must(t, err)
	if l.Error != "" {
		return "erreur : " + l.Error
	}
	return strings.Join(l.Values, ",")
}

// Liste tirée d'une colonne de tableau : valeurs distinctes dans l'ordre
// des lignes, vides sautés, relue quand la source change ; la source suit le
// renommage du tableau et de la colonne (annulable) ; un tableau supprimé
// donne une erreur.
func TestValidationFromColumn(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "N"}, {"FR", "1"}, {"IT", "2"}, {"fr", "3"}, {"", "4"}, {"BE", "5"}})
	must(t, err)
	must(t, d.RenameTable(ctx, "Feuille1", "Ref"))
	// Saisie « =Ref[Pays] » (comme dans l'API) : rangée en {"src": …}.
	must(t, d.ApplyStyle(ctx, 1, 10, 0, 11, 0, Style{"dv": "=Ref[Pays]"}))
	if src, ok := DVSource(freeDV(t, d, 1, 10, 0)); !ok || src != "Ref[Pays]" {
		t.Fatalf("source rangée : %v", freeDV(t, d, 1, 10, 0))
	}
	if got := dvList(t, d, 1, "Ref[Pays]"); got != "FR,IT,BE" {
		t.Fatalf("liste : %s", got)
	}
	if got := dvList(t, d, 1, "Ref[Pays][1:2]"); got != "FR,IT" {
		t.Fatalf("lignes d'une colonne : %s", got)
	}
	// La source change : la liste suit.
	_, err = d.SetCell(ctx, 1, 6, 0, "DE")
	must(t, err)
	if got := dvList(t, d, 1, "Ref[Pays]"); got != "FR,IT,BE,DE" {
		t.Fatalf("liste après ajout : %s", got)
	}
	// Renommages : tableau, puis colonne (saisie dans l'en-tête).
	must(t, d.RenameTable(ctx, "Ref", "Liste"))
	_, err = d.SetCell(ctx, 1, 0, 0, "Code")
	must(t, err)
	if src, _ := DVSource(freeDV(t, d, 1, 10, 0)); src != "Liste[Code]" {
		t.Fatalf("source renommée : %q", src)
	}
	if src, _ := DVSource(freeDV(t, d, 1, 11, 0)); src != "Liste[Code]" {
		t.Fatalf("source renommée (2e cellule) : %q", src)
	}
	if got := dvList(t, d, 1, "Liste[Code]"); got != "FR,IT,BE,DE" {
		t.Fatalf("liste après renommage : %s", got)
	}
	_, err = d.bus.Undo(ctx)
	must(t, err)
	if src, _ := DVSource(freeDV(t, d, 1, 10, 0)); src != "Liste[Pays]" {
		t.Fatalf("annulation du renommage de la colonne : %q", src)
	}
	_, err = d.bus.Redo(ctx)
	must(t, err)
	// Colonne disparue, tableau supprimé : une erreur.
	if got := dvList(t, d, 1, "Liste[Nope]"); !strings.Contains(got, "colonne « Nope » n'existe plus") {
		t.Fatalf("colonne inconnue : %s", got)
	}
	must(t, d.DeleteTables(ctx, []string{"Liste"}))
	if got := dvList(t, d, 1, "Liste[Code]"); !strings.Contains(got, "tableau « Liste » n'existe plus") {
		t.Fatalf("tableau supprimé : %s", got)
	}
	check(t, d, "validation")
}

// Liste tirée d'une plage de cellules libres : ligne par ligne, formules
// calculées, doublons et vides sautés ; la plage suit l'insertion de lignes,
// devient #REF! quand ses lignes sont supprimées, et suit le renommage de
// sa feuille.
func TestValidationFromRange(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	for _, c := range []struct {
		row  int
		raw  string
		col  int
		side bool
	}{{20, "x", 3, false}, {22, `="y"`, 3, false}, {23, "X", 3, false}, {21, "z", 4, false}} {
		_, err := d.SetCell(ctx, 1, c.row, c.col, c.raw)
		must(t, err)
	}
	must(t, d.ApplyStyle(ctx, 1, 2, 1, 2, 1, Style{"dv": map[string]any{"src": "D21:E24"}}))
	src, _ := DVSource(freeDV(t, d, 1, 2, 1))
	if src != "Cell[D21:E24]" {
		t.Fatalf("plage à la Excel : %q", src)
	}
	if got := dvList(t, d, 1, src); got != "x,z,y" {
		t.Fatalf("plage : %s", got)
	}
	// Des lignes insérées au-dessus : la plage et la cellule validée descendent.
	must(t, d.InsertSheetRows(ctx, 1, 0, 2))
	src, _ = DVSource(freeDV(t, d, 1, 4, 1))
	if src != "Cell[D23:E26]" {
		t.Fatalf("plage déplacée : %q", src)
	}
	if got := dvList(t, d, 1, src); got != "x,z,y" {
		t.Fatalf("plage déplacée : %s", got)
	}
	// Une autre feuille, renommée.
	s2, err := d.AddSheet(ctx)
	must(t, err)
	must(t, d.RenameSheet(ctx, s2, "Listes"))
	_, err = d.SetCell(ctx, s2, 0, 0, "Petit")
	must(t, err)
	_, err = d.SetCell(ctx, s2, 1, 0, "Grand")
	must(t, err)
	must(t, d.ApplyStyle(ctx, 1, 0, 6, 0, 6, Style{"dv": "=Listes!Cell[A1:A2]"}))
	must(t, d.RenameSheet(ctx, s2, "Tailles"))
	src, _ = DVSource(freeDV(t, d, 1, 0, 6))
	if src != "Tailles!Cell[A1:A2]" || dvList(t, d, 1, src) != "Petit,Grand" {
		t.Fatalf("feuille renommée : %q %s", src, dvList(t, d, 1, src))
	}
	// Cellule nommée.
	_, err = d.SetCell(ctx, 1, 40, 0, "Seul")
	must(t, err)
	must(t, d.SetName(ctx, 1, 40, 0, "Objectif"))
	if got := dvList(t, d, 1, "Objectif"); got != "Seul" {
		t.Fatalf("cellule nommée : %s", got)
	}
	// Lignes de la plage supprimées : #REF!, une erreur.
	must(t, d.DeleteSheetRows(ctx, 1, 22, 27))
	src, _ = DVSource(freeDV(t, d, 1, 4, 1))
	if src != "#REF!" || !strings.Contains(dvList(t, d, 1, src), "supprimée") {
		t.Fatalf("plage supprimée : %q %s", src, dvList(t, d, 1, src))
	}
	check(t, d, "validation")
}

// Une source qui n'est pas une plage est refusée ; une liste saisie reste
// une liste ; un texte « a;b » devient une liste.
func TestValidationShapes(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if err := d.ApplyStyle(ctx, 1, 0, 0, 0, 0, Style{"dv": "=1+2"}); err == nil || !strings.Contains(err.Error(), "n'est pas une plage") {
		t.Fatalf("source refusée : %v", err)
	}
	if err := d.ApplyStyle(ctx, 1, 0, 0, 0, 0, Style{"dv": map[string]any{"src": " "}}); err == nil {
		t.Fatal("source vide acceptée")
	}
	must(t, d.ApplyStyle(ctx, 1, 0, 0, 0, 0, Style{"dv": []any{"Oui", "Non"}}))
	if got := fmt.Sprint(freeDV(t, d, 1, 0, 0)); got != "[Oui Non]" {
		t.Fatalf("liste : %s", got)
	}
	must(t, d.ApplyStyle(ctx, 1, 1, 0, 1, 0, Style{"dv": "a; b ;"}))
	if got := fmt.Sprint(freeDV(t, d, 1, 1, 0)); got != "[a b]" {
		t.Fatalf("liste tapée : %s", got)
	}
	// Retirer la validation.
	must(t, d.ApplyStyle(ctx, 1, 0, 0, 0, 0, Style{"dv": nil}))
	if v := freeDV(t, d, 1, 0, 0); v != nil {
		t.Fatalf("validation retirée : %v", v)
	}
}

// Une validation de colonne de tableau tirée d'une plage : enregistrée,
// relue après réouverture du classeur, renommée avec le tableau source.
func TestValidationColumnSaveReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "doc.sqlite")
	openAt := func() (*fixture, func()) {
		db, err := store.Open(path, Register)
		must(t, err)
		must(t, meta.Migrate(ctx, db))
		return &fixture{New(db, command.New(db)), command.New(db)}, func() { db.Close() }
	}
	d, closeDB := openAt()
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "CP"}, {"Lyon", "69"}, {"Nantes", "44"}, {"Lyon", "69"}})
	must(t, err)
	must(t, d.RenameTable(ctx, "Feuille1", "Villes"))
	_, err = d.Paste(ctx, 1, 0, 3, [][]string{{"Client", "Ville"}, {"A", "Lyon"}, {"B", "Nantes"}})
	must(t, err)
	l, err := d.Layout(ctx, 1)
	must(t, err)
	var orders string
	for _, tb := range l.Tables {
		if tb.Name != "Villes" {
			orders = tb.Name
		}
	}
	// Toute la colonne Ville du 2e tableau (lignes 1 à 2, colonne 4).
	must(t, d.ApplyStyle(ctx, 1, 1, 4, 2, 4, Style{"dv": "=Villes[Ville]"}))
	closeDB()
	d, closeDB = openAt()
	defer closeDB()
	must(t, d.RenameTable(ctx, "Villes", "Communes"))
	var s string
	must(t, d.read(ctx, func(tx *tx) error {
		return tx.row(`SELECT s.style FROM _lattis_styles s JOIN _lattis_tables t ON t.id = s.tbl WHERE t.name = ? AND s.rid = 0`, orders).Scan(&s)
	}))
	if src, ok := DVSource(parseStyle(s)["dv"]); !ok || src != "Communes[Ville]" {
		t.Fatalf("style de colonne : %s", s)
	}
	if got := dvList(t, d, 1, "Communes[Ville]"); got != "Lyon,Nantes" {
		t.Fatalf("liste après réouverture : %s", got)
	}
}
