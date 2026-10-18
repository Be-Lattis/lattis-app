package archive

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Un classeur neuf jamais modifié, laissé par un processus tué (simulé par
// Release : base et verrou relâchés, répertoire gardé), n'est pas proposé en
// récupération : son répertoire est supprimé au démarrage suivant.
func TestUnmodifiedNewNotOffered(t *testing.T) {
	root := useRoot(t)
	ctx := context.Background()
	d, err := New()
	if err != nil {
		t.Fatal(err)
	}
	exec1(t, d, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT) STRICT`) // schéma, avant la marque
	if err := d.MarkPristine(ctx); err != nil {
		t.Fatal(err)
	}
	if !d.Pristine() {
		t.Fatal("marque absente")
	}
	d.Release()
	o, err := Orphans()
	if err != nil {
		t.Fatal(err)
	}
	if len(o) != 0 {
		t.Fatalf("session sans modification proposée : %+v", o)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Fatalf("répertoire de travail laissé : %v", entries)
	}
}

// La première écriture retire la marque, avant d'être validée : une
// modification n'est jamais perdue.
func TestModifiedNewOffered(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	d, err := New()
	if err != nil {
		t.Fatal(err)
	}
	exec1(t, d, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT) STRICT`)
	d.SetWorkbook("abc123")
	if err := d.MarkPristine(ctx); err != nil {
		t.Fatal(err)
	}
	exec1(t, d, `INSERT INTO t (v) VALUES ('saisie')`)
	if d.Pristine() {
		t.Fatal("marque gardée après une écriture")
	}
	d.Release()
	o, err := Orphans()
	if err != nil {
		t.Fatal(err)
	}
	if len(o) != 1 || o[0].Workbook != "abc123" {
		t.Fatalf("sessions = %+v, attendu 1 du classeur abc123", o)
	}
}

// Enregistré puis tué sans autre modification : la copie de travail est
// celle du fichier, rien à proposer ; une écriture après l'enregistrement
// la rend de nouveau récupérable.
func TestSavedNotOffered(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "c.lattis")
	d, err := New()
	if err != nil {
		t.Fatal(err)
	}
	exec1(t, d, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT) STRICT`)
	exec1(t, d, `INSERT INTO t (v) VALUES ('a')`)
	if d.Pristine() {
		t.Fatal("marque sur un classeur modifié")
	}
	if err := d.Save(ctx, path); err != nil {
		t.Fatal(err)
	}
	if !d.Pristine() {
		t.Fatal("enregistré sans modification depuis : marque attendue")
	}
	d.Release()
	if o, _ := Orphans(); len(o) != 0 {
		t.Fatalf("session enregistrée proposée : %+v", o)
	}

	// Ouvert, modifié, tué : proposé (Open le signale aussi).
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	d.MarkPristine(ctx)
	exec1(t, d, `INSERT INTO t (v) VALUES ('b')`)
	d.Release()
	var rec *RecoverableError
	if _, err := Open(path); !errors.As(err, &rec) {
		t.Fatalf("Open = %v, attendu RecoverableError", err)
	}
	if o, _ := Orphans(); len(o) != 1 {
		t.Fatalf("sessions = %+v, attendu 1", o)
	}
}

// Fichier seulement ouvert, processus tué : le rouvrir l'ouvre directement,
// sans proposer de récupération.
func TestOpenedOnlyReopens(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "c.lattis")
	d, err := New()
	if err != nil {
		t.Fatal(err)
	}
	exec1(t, d, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT) STRICT`)
	if err := d.Save(ctx, path); err != nil {
		t.Fatal(err)
	}
	d.Close()
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.MarkPristine(ctx); err != nil {
		t.Fatal(err)
	}
	d.Release()
	d, err = Open(path)
	if err != nil {
		t.Fatalf("réouverture : %v", err)
	}
	d.Close()
}

// Mark conditionnel : une écriture passée depuis la génération attendue
// empêche la marque (enregistrement concurrent d'une saisie).
func TestMarkSince(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	d, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	g := d.DB.Generation()
	exec1(t, d, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT) STRICT`)
	if ok, err := d.markPristine(ctx, g); err != nil || ok {
		t.Fatalf("marque posée malgré une écriture (%v, %v)", ok, err)
	}
	if ok, err := d.markPristine(ctx, d.DB.Generation()); err != nil || !ok {
		t.Fatalf("marque non posée (%v, %v)", ok, err)
	}
}

// Session d'une version précédente (sans workbook.txt) : l'identifiant du
// classeur est lu dans la base, ouverte en lecture seule.
func TestLegacyWorkbookID(t *testing.T) {
	useRoot(t)
	d, err := New()
	if err != nil {
		t.Fatal(err)
	}
	exec1(t, d, `CREATE TABLE _lattis_info (key TEXT PRIMARY KEY, value TEXT)`)
	exec1(t, d, `INSERT INTO _lattis_info VALUES ('id', 'ancien42')`)
	d.Release()
	o, err := Orphans()
	if err != nil {
		t.Fatal(err)
	}
	if len(o) != 1 || o[0].Workbook != "ancien42" {
		t.Fatalf("sessions = %+v, attendu le classeur ancien42", o)
	}
	if err := Discard(o[0]); err != nil {
		t.Fatal(err)
	}
}

// Sous go test, le dossier de travail par défaut n'est pas celui du profil
// de l'utilisateur.
func TestDefaultRootUnderTest(t *testing.T) {
	root, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	if cache, err := os.UserCacheDir(); err == nil && filepath.Dir(filepath.Dir(root)) == cache {
		t.Fatalf("dossier de travail des tests dans le profil : %s", root)
	}
}
