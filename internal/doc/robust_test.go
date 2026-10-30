package doc

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/meta"
)

func digest(t *testing.T, path string) [32]byte {
	t.Helper()
	b, err := os.ReadFile(path)
	must(t, err)
	return sha256.Sum256(b)
}

func clean(t *testing.T, root string) {
	t.Helper()
	if left, _ := os.ReadDir(root); len(left) != 0 {
		t.Fatalf("dossier de travail non nettoyé : %v", left)
	}
}

func write(t *testing.T, a *archive.Doc, qs ...string) {
	t.Helper()
	must(t, a.DB.Write(context.Background(), func(tx *sql.Tx) error {
		for _, q := range qs {
			if _, err := tx.Exec(q); err != nil {
				return err
			}
		}
		return nil
	}))
}

// fixture enregistre un classeur au format v (construit par les
// migrations de ce format), avec une cellule libre.
func fixture(t *testing.T, v int, path string) {
	t.Helper()
	ctx := context.Background()
	a, err := archive.New(canvas.Register)
	must(t, err)
	must(t, meta.MigrateTo(ctx, a.DB, v))
	write(t, a, `INSERT INTO _lattis_cells (sheet, row, col, raw) VALUES (1, 0, 0, 'format ancien')`)
	must(t, a.Save(ctx, path))
	must(t, a.Close())
}

func userVersion(t *testing.T, path string) int {
	t.Helper()
	a, err := archive.Open(path)
	must(t, err)
	defer a.Close()
	var v int
	must(t, a.DB.Read().QueryRow(`PRAGMA user_version`).Scan(&v))
	return v
}

// TestOpenOldFormats : un fichier de chaque format ancien s'ouvre, migré ;
// le premier enregistrement par-dessus garde une copie du fichier d'origine,
// que la version qui l'a écrit sait encore lire.
func TestOpenOldFormats(t *testing.T) {
	root := useRoot(t)
	ctx := context.Background()
	dir := t.TempDir()
	for v := 1; v < meta.Version; v++ {
		// -short (CI sous Windows ; Linux passe tous les formats) : un format
		// sur huit et le dernier.
		if testing.Short() && v%8 != 1 && v != meta.Version-1 {
			continue
		}
		path := filepath.Join(dir, fmt.Sprintf("ancien%d.lattis", v))
		fixture(t, v, path)
		before := digest(t, path)
		d, err := Open(ctx, path)
		if err != nil {
			t.Fatalf("format %d : %v", v, err)
		}
		if got := view(t, d); !strings.Contains(got, "format ancien") {
			t.Fatalf("format %d : contenu %q", v, got)
		}
		backup := filepath.Join(dir, fmt.Sprintf("ancien%d (format %d).lattis", v, v))
		if d.Backup() != backup {
			t.Fatalf("format %d : sauvegarde prévue %q", v, d.Backup())
		}
		// Rien de modifié : enregistrer n'écrit rien, le fichier reste lisible
		// par l'ancienne version.
		must(t, d.Save(ctx, ""))
		if digest(t, path) != before {
			t.Fatalf("format %d : fichier réécrit sans modification", v)
		}
		set(t, d, 1, 0, "nouveau")
		must(t, d.Save(ctx, ""))
		if digest(t, backup) != before {
			t.Fatalf("format %d : la sauvegarde n'est pas le fichier d'origine", v)
		}
		set(t, d, 2, 0, "encore")
		must(t, d.Save(ctx, ""))
		if digest(t, backup) != before {
			t.Fatalf("format %d : sauvegarde écrasée", v)
		}
		must(t, d.Close())
		if got := userVersion(t, path); got != meta.Version {
			t.Fatalf("format %d : enregistré au format %d", v, got)
		}
		if got := userVersion(t, backup); got != v {
			t.Fatalf("format %d : sauvegarde au format %d", v, got)
		}
		clean(t, root)
	}
}

// TestSaveAsOldFormat : Enregistrer sous ailleurs ne fait pas de copie, le
// fichier d'origine reste tel quel.
func TestSaveAsOldFormat(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "ancien.lattis")
	fixture(t, 10, path)
	before := digest(t, path)
	d, err := Open(ctx, path)
	must(t, err)
	defer d.Close()
	set(t, d, 1, 0, "x")
	must(t, d.Save(ctx, filepath.Join(dir, "nouveau.lattis")))
	must(t, d.Save(ctx, ""))
	if digest(t, path) != before {
		t.Fatal("fichier d'origine modifié")
	}
	if m, _ := filepath.Glob(filepath.Join(dir, "*(format*")); len(m) != 0 {
		t.Fatalf("sauvegarde inutile : %v", m)
	}
}

// TestOpenRefused : format plus récent, base étrangère, base endommagée :
// une erreur claire, le fichier intact, rien de laissé.
func TestOpenRefused(t *testing.T) {
	root := useRoot(t)
	ctx := context.Background()
	dir := t.TempDir()

	newer := filepath.Join(dir, "futur.lattis")
	a, err := archive.New(canvas.Register)
	must(t, err)
	must(t, meta.MigrateTo(ctx, a.DB, meta.Version))
	write(t, a, fmt.Sprintf(`PRAGMA user_version = %d`, meta.Version+1))
	must(t, a.Save(ctx, newer))
	must(t, a.Close())

	foreign := filepath.Join(dir, "étranger.lattis")
	a, err = archive.New()
	must(t, err)
	write(t, a, `CREATE TABLE clients (nom TEXT)`, `INSERT INTO clients VALUES ('Dupont')`)
	must(t, a.Save(ctx, foreign))
	must(t, a.Close())

	// Page 1 (schéma) écrasée après l'en-tête : la base ne se lit plus.
	broken := filepath.Join(dir, "cassé.lattis")
	fixture(t, meta.Version, broken)
	rewriteDB(t, broken, func(db []byte) {
		for i := 100; i < 4096 && i < len(db); i++ {
			db[i] = 0x5A
		}
	})

	for _, c := range []struct {
		path, reason string
		kind         error
	}{
		{newer, "plus récente", archive.ErrFormat},
		{foreign, "pas un classeur Lattis", archive.ErrFormat},
		{broken, "endommagée", archive.ErrCorrupt},
	} {
		before := digest(t, c.path)
		_, err := Open(ctx, c.path)
		var oe *archive.OpenError
		if !errors.As(err, &oe) || !errors.Is(err, c.kind) || !strings.Contains(err.Error(), c.reason) {
			t.Fatalf("%s : %v", filepath.Base(c.path), err)
		}
		if digest(t, c.path) != before {
			t.Fatalf("%s modifié", filepath.Base(c.path))
		}
		clean(t, root)
	}
	// Rien à tirer d'une base au schéma illisible.
	if _, _, err := Salvage(ctx, broken); err == nil || !strings.Contains(err.Error(), "aucune donnée lisible") {
		t.Fatalf("récupération d'une base sans schéma : %v", err)
	}
	clean(t, root)
}

// rewriteDB modifie la base d'une archive.
func rewriteDB(t *testing.T, path string, fn func(db []byte)) {
	t.Helper()
	zr, err := archive.OpenZip(path)
	must(t, err)
	files := map[string][]byte{}
	var names []string
	for _, f := range zr.File {
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		r.Close()
		files[f.Name] = b
		names = append(names, f.Name)
	}
	zr.Close()
	fn(files["database.sqlite"])
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range names {
		w, _ := zw.CreateHeader(&zip.FileHeader{Name: n, Method: zip.Store})
		w.Write(files[n])
	}
	zw.Close()
	must(t, os.WriteFile(path, buf.Bytes(), 0o644))
}

// TestSalvageDocument : un classeur tronqué se récupère en partie, dans un
// nouveau classeur jamais enregistré, qu'on peut enregistrer ailleurs.
func TestSalvageDocument(t *testing.T) {
	root := useRoot(t)
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "ventes.lattis")
	d, err := New(ctx)
	must(t, err)
	set(t, d, 0, 0, "Produit")
	set(t, d, 1, 0, "Clavier")
	must(t, d.DB().Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE gros (id INTEGER PRIMARY KEY, v TEXT);
			WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 3000)
			INSERT INTO gros (v) SELECT hex(randomblob(60)) FROM n`)
		return err
	}))
	must(t, d.Save(ctx, path))
	id := d.SecretScope()
	must(t, d.Close())

	b, err := os.ReadFile(path)
	must(t, err)
	must(t, os.WriteFile(path, b[:len(b)*3/4], 0o644))
	before := digest(t, path)
	_, err = Open(ctx, path)
	var oe *archive.OpenError
	if !errors.As(err, &oe) || !oe.Salvage {
		t.Fatalf("ouverture d'un classeur tronqué : %v", err)
	}
	clean(t, root)

	d, rep, err := Salvage(ctx, path)
	must(t, err)
	defer d.Close()
	var n int
	must(t, d.DB().Read().QueryRow(`SELECT count(*) FROM gros`).Scan(&n))
	if n == 0 || n >= 3000 || rep.Rows == 0 || len(rep.Damaged) == 0 {
		t.Fatalf("%d lignes récupérées, rapport %+v", n, rep)
	}
	if !strings.Contains(view(t, d), "Clavier") {
		t.Fatalf("tableau saisi perdu :\n%s", view(t, d))
	}
	if d.Path() != "" || d.SecretScope() != id {
		t.Fatalf("chemin %q, identifiant %q (attendu %q)", d.Path(), d.SecretScope(), id)
	}
	if dirty, _ := d.Dirty(ctx); !dirty {
		t.Fatal("classeur récupéré vu comme enregistré")
	}
	if digest(t, path) != before {
		t.Fatal("fichier d'origine modifié")
	}
	// La table nue du test n'est pas un tableau : Check la signalerait.
	must(t, d.DB().Write(ctx, func(tx *sql.Tx) error { _, err := tx.Exec(`DROP TABLE gros`); return err }))
	checkOK(t, d)
	out := filepath.Join(dir, "récupéré.lattis")
	must(t, d.Save(ctx, out))
	d2, err := Open(ctx, out)
	must(t, err)
	checkOK(t, d2)
	must(t, d2.Close())
}

// TestRecoverFailureKeepsSession : une session interrompue qui ne se
// rouvre pas reste proposée (rien n'est supprimé), et peut être abandonnée.
func TestRecoverFailureKeepsSession(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	d, err := New(ctx)
	must(t, err)
	set(t, d, 0, 0, "à garder")
	dir := d.arc.Dir
	must(t, d.arc.Release()) // processus disparu
	db := filepath.Join(dir, "database.sqlite")
	must(t, os.WriteFile(db, []byte("SQLite format 3\x00 abîmée"), 0o644))

	orphans, err := archive.Orphans()
	if err != nil || len(orphans) != 1 {
		t.Fatalf("orphelins : %+v %v", orphans, err)
	}
	if _, err := Recover(ctx, orphans[0]); err == nil || !strings.Contains(err.Error(), "récupération impossible") {
		t.Fatalf("récupération d'une base abîmée : %v", err)
	}
	if _, err := os.Stat(db); err != nil {
		t.Fatal("session supprimée après un échec de récupération")
	}
	orphans, _ = archive.Orphans()
	if len(orphans) != 1 {
		t.Fatalf("session plus proposée : %+v", orphans)
	}
	must(t, archive.Discard(orphans[0]))
	if o, _ := archive.Orphans(); len(o) != 0 {
		t.Fatalf("orphelins : %+v", o)
	}
}
