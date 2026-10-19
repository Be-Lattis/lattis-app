package archive

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// La casse ne compte que là où le système de fichiers la distingue
// (audit 6, M4).
func TestPathKeyCase(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "Ventes.lattis")
	b := filepath.Join(dir, "ventes.lattis")
	if got := SamePath(a, b); got != foldCase {
		t.Fatalf("SamePath(%q, %q) = %v, attendu %v", a, b, got, foldCase)
	}
	if !SamePath(a, filepath.Join(dir, ".", "Ventes.lattis")) {
		t.Fatal("chemin non nettoyé")
	}
}

// linkDir crée link, un lien vers le dossier target : lien symbolique, ou
// jonction sous Windows (sans droit particulier).
func linkDir(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err == nil {
		return
	}
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err == nil {
			return
		} else {
			t.Skipf("jonction impossible : %v %s", err, out)
		}
	}
	t.Skip("lien impossible ici")
}

// Un fichier atteint par un dossier relié (lien symbolique, jonction) est
// le même fichier : il ne s'ouvre pas deux fois.
func TestPathKeyLinked(t *testing.T) {
	useRoot(t)
	path := savedRows(t, 10)
	link := filepath.Join(t.TempDir(), "relie")
	linkDir(t, filepath.Dir(path), link)
	other := filepath.Join(link, filepath.Base(path))
	if !SamePath(path, other) {
		t.Fatalf("%q et %q : clés différentes (%q, %q)", path, other, PathKey(path), PathKey(other))
	}
	// Un fichier à créer sous le dossier relié aussi.
	if !SamePath(filepath.Join(filepath.Dir(path), "neuf.lattis"), filepath.Join(link, "neuf.lattis")) {
		t.Fatal("fichier à créer sous un dossier relié : autre clé")
	}
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := Open(other); !errors.Is(err, ErrAlreadyOpen) {
		t.Fatalf("deuxième ouverture par le dossier relié : %v", err)
	}
}

// Une session interrompue sous la clé des versions précédentes (chemin
// absolu en minuscules, sans résoudre les liens) est encore proposée à la
// récupération.
func TestLegacyWorkDir(t *testing.T) {
	root := useRoot(t)
	src := savedRows(t, 10)
	link := filepath.Join(t.TempDir(), "Relie")
	linkDir(t, filepath.Dir(src), link)
	path := filepath.Join(link, filepath.Base(src))
	old := filepath.Join(root, legacyKey(path))
	if cur, _ := workDir(path); old == cur {
		t.Fatal("même clé qu'avant pour un chemin relié")
	}
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, dbName), []byte("base"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Open(path)
	var re *RecoverableError
	if !errors.As(err, &re) || re.Orphan.Dir != old {
		t.Fatalf("session de l'ancienne clé : %v", err)
	}
	// Session sans modification : supprimée, le fichier se rouvre.
	if err := os.WriteFile(filepath.Join(old, pristineName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ancienne session intacte non supprimée : %v", err)
	}
}
