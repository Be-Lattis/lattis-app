package doc

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/archive"
)

// kill simule l'arrêt brutal du processus : base et verrou relâchés,
// répertoire de travail laissé tel quel.
func kill(d *Document) { d.arc.Release() }

// Classeur neuf : marqué « rien à récupérer » jusqu'à la première saisie ;
// tué avant, il n'est pas proposé.
func TestNewUnmodifiedNotOffered(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	d, err := New(ctx)
	must(t, err)
	if !d.Pristine() {
		t.Fatal("classeur neuf non marqué")
	}
	kill(d)
	o, err := archive.Orphans()
	must(t, err)
	if len(o) != 0 {
		t.Fatalf("classeur neuf jamais modifié proposé : %+v", o)
	}

	d, err = New(ctx)
	must(t, err)
	set(t, d, 0, 0, "1")
	if d.Pristine() {
		t.Fatal("classeur modifié encore marqué")
	}
	kill(d)
	o, err = archive.Orphans()
	must(t, err)
	if len(o) != 1 || "classeur:"+o[0].Workbook != d.SecretScope() {
		t.Fatalf("sessions = %+v, attendu 1 du classeur %s", o, d.SecretScope())
	}
}

// Démonstration (MarkSaved) et classeur enregistré : rien à proposer tant
// qu'ils ne sont pas modifiés.
func TestMarkSavedAndSavePristine(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	d, err := New(ctx)
	must(t, err)
	set(t, d, 0, 0, "données de démonstration")
	must(t, d.MarkSaved(ctx))
	if !d.Pristine() {
		t.Fatal("MarkSaved : marque attendue")
	}
	set(t, d, 0, 1, "x")
	path := filepath.Join(t.TempDir(), "c.lattis")
	must(t, d.Save(ctx, path))
	if !d.Pristine() {
		t.Fatal("enregistré : marque attendue")
	}
	kill(d)
	if o, _ := archive.Orphans(); len(o) != 0 {
		t.Fatalf("sessions = %+v", o)
	}
}

// Deux sessions du même classeur (le même fichier copié sous deux noms,
// ou sous le même nom dans deux dossiers) portent le même identifiant :
// la récupération les regroupe ; deux classeurs neufs, du même nom
// « Classeur sans titre », restent distincts.
func TestSessionsGroupedByWorkbook(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	dir := t.TempDir()
	a := filepath.Join(dir, "a", "Ventes.lattis")
	b := filepath.Join(dir, "b", "Ventes.lattis")
	must(t, os.MkdirAll(filepath.Dir(a), 0o755))
	must(t, os.MkdirAll(filepath.Dir(b), 0o755))
	d, err := New(ctx)
	must(t, err)
	set(t, d, 0, 0, "1")
	must(t, d.Save(ctx, a))
	id := d.SecretScope()
	d.Close()
	copyFile(t, a, b)
	for _, p := range []string{a, b} {
		d, err := Open(ctx, p)
		must(t, err)
		set(t, d, 1, 0, "modifié")
		kill(d)
	}
	for range 2 {
		d, err := New(ctx)
		must(t, err)
		set(t, d, 0, 0, "x")
		kill(d)
	}
	o, err := archive.Orphans()
	must(t, err)
	if len(o) != 4 {
		t.Fatalf("sessions = %+v, attendu 4", o)
	}
	by := map[string]int{}
	for _, x := range o {
		by[x.Workbook]++
	}
	if by[strings.TrimPrefix(id, "classeur:")] != 2 || len(by) != 3 {
		t.Fatalf("classeurs = %v, attendu 2 sessions de %s et 2 classeurs neufs distincts", by, id)
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	must(t, err)
	defer in.Close()
	out, err := os.Create(dst)
	must(t, err)
	_, err = io.Copy(out, in)
	must(t, err)
	must(t, out.Close())
}
