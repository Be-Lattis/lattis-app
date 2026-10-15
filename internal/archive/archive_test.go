package archive

import (
	"archive/zip"
	"context"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func useRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "work")
	old := Root
	Root = func() (string, error) { return root, nil }
	t.Cleanup(func() { Root = old })
	return root
}

func exec1(t *testing.T, d *Doc, q string) {
	t.Helper()
	err := d.DB.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(q)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func count(t *testing.T, d *Doc) int {
	t.Helper()
	var n int
	if err := d.DB.Read().QueryRow(`SELECT count(*) FROM t`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSaveOpenRoundTrip(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	d, err := New()
	if err != nil {
		t.Fatal(err)
	}
	exec1(t, d, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT) STRICT`)
	exec1(t, d, `INSERT INTO t (v) VALUES ('a'), ('b')`)
	path := filepath.Join(t.TempDir(), "classeur.slitable")
	if err := d.Save(ctx, path); err != nil {
		t.Fatal(err)
	}
	exec1(t, d, `INSERT INTO t (v) VALUES ('non enregistré')`)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	// L'archive : un manifeste et la base, stockée sans compression.
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]uint16{}
	for _, f := range zr.File {
		names[f.Name] = f.Method
	}
	zr.Close()
	if _, ok := names[manifestName]; !ok || names[dbName] != zip.Store {
		t.Fatalf("contenu de l'archive : %v", names)
	}

	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if n := count(t, d); n != 2 {
		t.Fatalf("%d lignes rouvertes, attendu 2 (la modification non enregistrée est perdue)", n)
	}
	// Ouvrir deux fois la même archive est refusé.
	if _, err := Open(path); err == nil {
		t.Fatal("deuxième ouverture acceptée")
	}
}

func TestRejectsNewerFormat(t *testing.T) {
	useRoot(t)
	path := filepath.Join(t.TempDir(), "futur.slitable")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	w, _ := zw.Create(manifestName)
	w.Write([]byte("format_version: 99\napplication: Slitable\n"))
	w, _ = zw.Create(dbName)
	w.Write([]byte("x"))
	zw.Close()
	f.Close()
	if _, err := Open(path); !errors.Is(err, ErrFormat) {
		t.Fatalf("archive de format 99 : %v", err)
	}
}

// Critère de sortie : le processus est tué sans enregistrer ; à la relance,
// le répertoire de travail orphelin est proposé en récupération, avec les
// écritures validées avant l'arrêt.
func TestCrashRecovery(t *testing.T) {
	if os.Getenv("SLITABLE_CRASH_CHILD") != "" {
		crashChild()
		return
	}
	root := useRoot(t)
	ctx := context.Background()
	d, err := New()
	if err != nil {
		t.Fatal(err)
	}
	exec1(t, d, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT) STRICT`)
	exec1(t, d, `INSERT INTO t (v) VALUES ('enregistré')`)
	path := filepath.Join(t.TempDir(), "crash.slitable")
	if err := d.Save(ctx, path); err != nil {
		t.Fatal(err)
	}
	d.Close()

	// Enfant : ouvre l'archive, écrit, puis meurt brutalement (os.Exit sans
	// fermer la base ni relâcher le verrou).
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashRecovery$")
	cmd.Env = append(os.Environ(), "SLITABLE_CRASH_CHILD="+path, "SLITABLE_CRASH_ROOT="+root)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("l'enfant devait mourir avec le code 3 : %v\n%s", err, out)
	}

	orphans, err := Orphans()
	if err != nil || len(orphans) != 1 || orphans[0].Source != path {
		t.Fatalf("orphelins : %+v, %v", orphans, err)
	}
	// Rouvrir l'archive signale la session interrompue au lieu d'écraser.
	var rec *RecoverableError
	if _, err := Open(path); !errors.As(err, &rec) {
		t.Fatalf("Open après crash : %v", err)
	}
	d, err = Recover(rec.Orphan)
	if err != nil {
		t.Fatal(err)
	}
	if n := count(t, d); n != 1001 {
		t.Fatalf("%d lignes récupérées, attendu 1001", n)
	}
	if err := d.Save(ctx, ""); err != nil {
		t.Fatal(err)
	}
	d.Close()
	if o, _ := Orphans(); len(o) != 0 {
		t.Fatalf("orphelins restants : %v", o)
	}
}

func crashChild() {
	Root = func() (string, error) { return os.Getenv("SLITABLE_CRASH_ROOT"), nil }
	d, err := Open(os.Getenv("SLITABLE_CRASH_CHILD"))
	if err != nil {
		panic(err)
	}
	for i := 0; i < 1000; i++ {
		err := d.DB.Write(context.Background(), func(tx *sql.Tx) error {
			_, err := tx.Exec(`INSERT INTO t (v) VALUES ('non enregistré')`)
			return err
		})
		if err != nil {
			panic(err)
		}
	}
	os.Exit(3)
}

func TestDiscardOrphan(t *testing.T) {
	useRoot(t)
	d, err := New()
	if err != nil {
		t.Fatal(err)
	}
	// Tant que le document est ouvert, ce n'est pas un orphelin.
	if o, _ := Orphans(); len(o) != 0 {
		t.Fatalf("document ouvert vu comme orphelin : %v", o)
	}
	d.DB.Close()
	d.lock.Close() // simule un processus disparu
	o, _ := Orphans()
	if len(o) != 1 {
		t.Fatalf("orphelins : %v", o)
	}
	if err := Discard(o[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(o[0].Dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("répertoire non supprimé")
	}
}
