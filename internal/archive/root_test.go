package archive

import (
	"os"
	"path/filepath"
	"testing"
)

// Deux nouveaux classeurs créés dans le même tick de l'horloge ont chacun
// leur dossier de travail (avant : le second échouait, « déjà ouvert »).
func TestNewWorkDirsDistinct(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		dir, err := workDir("")
		if err != nil {
			t.Fatal(err)
		}
		if seen[dir] {
			t.Fatalf("dossier de travail en double : %s", dir)
		}
		seen[dir] = true
	}
}

// L'exécutable d'un dépôt de sources (bin/lattis.exe, go.mod au-dessus) est
// reconnu ; une version installée, non.
func TestCheckout(t *testing.T) {
	repo := t.TempDir()
	os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module x\n"), 0o644)
	os.MkdirAll(filepath.Join(repo, "bin"), 0o755)
	if got := checkout(filepath.Join(repo, "bin", "lattis.exe")); got != repo {
		t.Fatalf("checkout = %q, attendu %q", got, repo)
	}
	if got := checkout(filepath.Join(t.TempDir(), "Lattis", "lattis.exe")); got != "" {
		t.Fatalf("version installée prise pour un dépôt : %q", got)
	}
}

// Un dossier de travail par dépôt ; celui d'un dépôt disparu (worktree
// supprimé) est effacé, celui d'un dépôt présent reste.
func TestDevRoot(t *testing.T) {
	base := t.TempDir()
	a, b, gone := t.TempDir(), t.TempDir(), filepath.Join(t.TempDir(), "worktree")
	os.MkdirAll(gone, 0o755)
	ra, err := devRoot(base, a)
	if err != nil {
		t.Fatal(err)
	}
	rg, _ := devRoot(base, gone)
	os.MkdirAll(filepath.Join(rg, "new-1"), 0o755)
	if again, _ := devRoot(base, a); again != ra {
		t.Fatalf("dossier différent pour le même dépôt : %q, %q", ra, again)
	}
	os.RemoveAll(gone)
	rb, _ := devRoot(base, b)
	if rb == ra {
		t.Fatal("deux dépôts partagent un dossier de travail")
	}
	if _, err := os.Stat(rg); !os.IsNotExist(err) {
		t.Fatalf("dossier d'un dépôt disparu gardé : %v", err)
	}
	if _, err := os.Stat(ra); err != nil {
		t.Fatalf("dossier d'un dépôt présent effacé : %v", err)
	}
}
