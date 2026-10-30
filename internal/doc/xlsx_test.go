package doc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/msg"
)

// firstSheet donne la première feuille du classeur et son contenu.
func firstSheet(t *testing.T, d *Document) (int64, string) {
	t.Helper()
	ctx := context.Background()
	l, err := d.Layout(ctx, 0)
	must(t, err)
	id := l.Sheets[0].ID
	v, err := d.View(ctx, id)
	must(t, err)
	return id, v
}

// freeValue donne la valeur affichée d'une cellule libre.
func freeValue(t *testing.T, d *Document, sheet int64, row, col int) string {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	must(t, err)
	for _, c := range l.Cells {
		if c.Row == row && c.Col == col {
			return c.Value
		}
	}
	return ""
}

// Fichier .xlsx ouvert dans Lattis : il garde son format (enregistrer
// réécrit le .xlsx), l'ouverture n'est pas une étape d'annulation, et une
// session interrompue est proposée en récupération comme un .lattis.
func TestXLSXBackedWorkbook(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "Ventes.xlsx")

	// Enregistrer sous en .xlsx : le classeur devient adossé au fichier.
	d, err := New(ctx)
	must(t, err)
	set(t, d, 0, 0, "Produit")
	set(t, d, 0, 1, "Prix")
	set(t, d, 1, 0, "Pomme")
	set(t, d, 1, 1, "12")
	set(t, d, 4, 1, "=B2*2")
	must(t, d.Save(ctx, path))
	if !d.XLSX() || d.Path() != path {
		t.Fatalf("après Enregistrer sous : XLSX %v, chemin %s", d.XLSX(), d.Path())
	}
	if _, err := os.Stat(filepath.Join(dir, "~$Ventes.xlsx")); err == nil {
		t.Fatal("pas de verrou ~$ à côté d'un .xlsx (nom du fichier de propriétaire d'un tableur)")
	}
	must(t, d.Close())

	// Ouverture : contenu importé, rien à annuler, rien à enregistrer.
	d, err = OpenXLSX(ctx, path, OpenNormal, msg.FR)
	must(t, err)
	sheet, v := firstSheet(t, d)
	if !strings.Contains(v, "Pomme") || freeValue(t, d, sheet, 4, 1) != "24" {
		t.Fatalf("contenu ouvert :\n%s", v)
	}
	if dirty, _ := d.Dirty(ctx); dirty {
		t.Fatal("tout juste ouvert : rien à enregistrer")
	}
	if h, _ := d.Bus.State(ctx); h.Undo != "" {
		t.Fatalf("l'ouverture est annulable : %q", h.Undo)
	}
	if !d.Pristine() {
		t.Fatal("tout juste ouvert : rien à récupérer")
	}
	_, err = d.SetCell(ctx, sheet, 1, 1, "20")
	must(t, err)
	must(t, d.Save(ctx, ""))
	if dirty, _ := d.Dirty(ctx); dirty || !d.Pristine() {
		t.Fatal("enregistré dans le .xlsx : propre et rien à récupérer")
	}
	must(t, d.Close())
	d, err = OpenXLSX(ctx, path, OpenNormal, msg.FR)
	must(t, err)
	sheet, v = firstSheet(t, d)
	if freeValue(t, d, sheet, 4, 1) != "40" {
		t.Fatalf("modification non relue :\n%s", v)
	}

	// Arrêt brutal : la session est proposée, puis récupérée.
	_, err = d.SetCell(ctx, sheet, 2, 0, "Poire")
	must(t, err)
	kill(d)
	_, err = OpenXLSX(ctx, path, OpenNormal, msg.FR)
	var rec *archive.RecoverableError
	if !errors.As(err, &rec) || rec.Orphan.Source != path {
		t.Fatalf("session interrompue non proposée : %v", err)
	}
	d, err = Recover(ctx, rec.Orphan)
	must(t, err)
	if !d.XLSX() {
		t.Fatal("récupéré : toujours adossé au .xlsx")
	}
	if _, v = firstSheet(t, d); !strings.Contains(v, "Poire") {
		t.Fatalf("récupération :\n%s", v)
	}
	must(t, d.Save(ctx, ""))

	// Modifié par ailleurs : pas écrasé.
	_, err = d.SetCell(ctx, sheet, 3, 0, "Prune")
	must(t, err)
	other, err := New(ctx)
	must(t, err)
	set(t, other, 0, 0, "autre")
	must(t, other.Save(ctx, filepath.Join(dir, "autre.xlsx")))
	must(t, other.Close())
	b, err := os.ReadFile(filepath.Join(dir, "autre.xlsx"))
	must(t, err)
	must(t, os.WriteFile(path, b, 0o644))
	var changed *archive.ChangedError
	if err := d.Save(ctx, ""); !errors.As(err, &changed) {
		t.Fatalf("fichier modifié par ailleurs écrasé : %v", err)
	}

	// Convertir : Enregistrer sous en .lattis, un classeur ordinaire.
	lattis := filepath.Join(dir, "Ventes.lattis")
	must(t, d.Save(ctx, lattis))
	if d.XLSX() || d.Path() != lattis {
		t.Fatalf("converti : XLSX %v, chemin %s", d.XLSX(), d.Path())
	}
	must(t, d.Close())
	if _, err := archive.ReadManifest(lattis); err != nil {
		t.Fatalf("archive .lattis attendue : %v", err)
	}
}

// Un fichier qui n'est pas un classeur .xlsx est refusé sans laisser de
// répertoire de travail.
func TestXLSXOpenInvalid(t *testing.T) {
	root := useRoot(t)
	p := filepath.Join(t.TempDir(), "faux.xlsx")
	must(t, os.WriteFile(p, []byte("pas un zip"), 0o644))
	if _, err := OpenXLSX(context.Background(), p, OpenNormal, msg.FR); err == nil {
		t.Fatal("fichier invalide ouvert")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatalf("répertoire de travail laissé : %v", entries)
	}
}
