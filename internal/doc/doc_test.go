package doc

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lalamefine/slitable/internal/archive"
)

func useRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "work")
	old := archive.Root
	archive.Root = func() (string, error) { return root, nil }
	t.Cleanup(func() { archive.Root = old })
	return root
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func set(t *testing.T, d *Document, row, col int, raw string) {
	t.Helper()
	_, err := d.SetCell(context.Background(), 1, row, col, raw)
	must(t, err)
}

func view(t *testing.T, d *Document) string {
	t.Helper()
	v, err := d.View(context.Background(), 1)
	must(t, err)
	return v
}

func checkOK(t *testing.T, d *Document) {
	t.Helper()
	problems, err := d.Check(context.Background())
	must(t, err)
	for _, p := range problems {
		t.Error(p)
	}
}

// TestM2 est le jalon M2 : ouvrir, modifier, annuler, enregistrer, rouvrir
// (avec l'historique d'annulation), simuler un crash et récupérer.
func TestM2(t *testing.T) {
	if os.Getenv("SLITABLE_M2_CHILD") != "" {
		m2Child()
		return
	}
	root := useRoot(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "ventes.slitable")

	// 1. Nouveau classeur, saisie d'un tableau, annulation, enregistrement.
	d, err := New(ctx)
	must(t, err)
	set(t, d, 0, 0, "Produit")
	set(t, d, 0, 1, "Prix")
	set(t, d, 1, 0, "Clavier")
	set(t, d, 1, 1, "12,5")
	set(t, d, 2, 0, "Souris")
	set(t, d, 2, 1, "abc") // élargit Prix en texte
	if _, err := d.Bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if dirty, _ := d.Dirty(ctx); !dirty {
		t.Fatal("classeur saisi vu comme enregistré")
	}
	must(t, d.Save(ctx, path))
	if dirty, _ := d.Dirty(ctx); dirty {
		t.Fatal("modifié juste après l'enregistrement")
	}
	saved := view(t, d)
	checkOK(t, d)
	must(t, d.Close())

	// 2. Réouverture : même contenu, et l'historique d'annulation est là.
	d, err = Open(ctx, path)
	must(t, err)
	if got := view(t, d); got != saved {
		t.Fatalf("contenu rouvert :\n%s\nattendu\n%s", got, saved)
	}
	if st, _ := d.Bus.State(ctx); st.Undo != "Saisie" || st.Redo != "Saisie" {
		t.Fatalf("historique après réouverture : %+v", st)
	}
	if _, err := d.Bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if dirty, _ := d.Dirty(ctx); !dirty {
		t.Fatal("annuler après réouverture ne marque pas le classeur modifié")
	}
	if _, err := d.Bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	if dirty, _ := d.Dirty(ctx); dirty {
		t.Fatal("revenu à l'état enregistré mais toujours modifié")
	}
	must(t, d.Close())

	// 3. Crash : un autre processus ouvre le fichier, saisit, puis meurt.
	cmd := exec.Command(os.Args[0], "-test.run=^TestM2$")
	cmd.Env = append(os.Environ(), "SLITABLE_M2_CHILD="+path, "SLITABLE_M2_ROOT="+root)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("l'enfant devait mourir avec le code 3 : %v\n%s", err, out)
	}

	// 4. Relance : la session interrompue est proposée et récupérée.
	_, err = Open(ctx, path)
	var rec *archive.RecoverableError
	if !errors.As(err, &rec) {
		t.Fatalf("Open après crash : %v", err)
	}
	d, err = Recover(ctx, rec.Orphan)
	must(t, err)
	want := "[Produit]|[Prix]\nClavier|12.5\nSouris|8\nÉcran|199\n"
	if got := view(t, d); got != want {
		t.Fatalf("contenu récupéré :\n%s\nattendu\n%s", got, want)
	}
	checkOK(t, d)
	// L'historique de la session interrompue est récupéré aussi.
	if _, err := d.Bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := view(t, d); got != "[Produit]|[Prix]\nClavier|12.5\nSouris|8\nÉcran\n" {
		t.Fatalf("annuler après récupération :\n%s", got)
	}
	must(t, d.Save(ctx, ""))
	must(t, d.Close())
	if o, _ := archive.Orphans(); len(o) != 0 {
		t.Fatalf("orphelins restants : %v", o)
	}
}

func m2Child() {
	archive.Root = func() (string, error) { return os.Getenv("SLITABLE_M2_ROOT"), nil }
	ctx := context.Background()
	d, err := Open(ctx, os.Getenv("SLITABLE_M2_CHILD"))
	if err != nil {
		panic(err)
	}
	for _, c := range []struct {
		r, c int
		s    string
	}{{2, 1, "8"}, {3, 0, "Écran"}, {3, 1, "199"}} {
		if _, err := d.SetCell(ctx, 1, c.r, c.c, c.s); err != nil {
			panic(err)
		}
	}
	os.Exit(3) // ni enregistrement ni fermeture
}
