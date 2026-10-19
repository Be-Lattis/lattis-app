package archive

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// saved crée un classeur enregistré dans un dossier temporaire et le rend
// ouvert.
func saved(t *testing.T, name string) (*Doc, string) {
	t.Helper()
	d, err := New()
	if err != nil {
		t.Fatal(err)
	}
	exec1(t, d, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT) STRICT`)
	exec1(t, d, `INSERT INTO t (v) VALUES ('a')`)
	path := filepath.Join(t.TempDir(), name)
	if err := d.Save(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	return d, path
}

func writePeer(t *testing.T, path string, h Holder, age time.Duration) {
	t.Helper()
	b, _ := json.Marshal(h)
	lp := LockPath(path)
	if err := os.WriteFile(lp, b, 0o644); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(lp, when, when); err != nil {
		t.Fatal(err)
	}
}

// otherFile rend un autre classeur, enregistré par « un autre poste ».
func otherFile(t *testing.T) []byte {
	t.Helper()
	other, err := New()
	if err != nil {
		t.Fatal(err)
	}
	exec1(t, other, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT) STRICT`)
	exec1(t, other, `INSERT INTO t (v) VALUES ('autre poste')`)
	elsewhere := filepath.Join(t.TempDir(), "autre.lattis")
	if err := other.Save(context.Background(), elsewhere); err != nil {
		t.Fatal(err)
	}
	other.Close()
	b, err := os.ReadFile(elsewhere)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Le verrou ~$ est posé au premier enregistrement (ou à l'ouverture),
// caché, et retiré à la fermeture. Celui de ce processus ne compte pas
// comme tenu (le verrou du répertoire de travail s'en charge).
func TestPeerLockLifeCycle(t *testing.T) {
	useRoot(t)
	d, path := saved(t, "cycle.lattis")
	lp := LockPath(path)
	if filepath.Base(lp) != "~$cycle.lattis" {
		t.Fatalf("verrou %s", lp)
	}
	h, ok := readPeer(lp)
	if !ok || h.PID != os.Getpid() || h.Machine == "" || h.Since.IsZero() {
		t.Fatalf("verrou posé : %+v (%v)", h, ok)
	}
	if !hidden(lp) {
		t.Error("verrou non caché")
	}
	if got := LockedBy(path); got != nil {
		t.Fatalf("verrou de ce processus vu comme tenu : %+v", got)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lp); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("verrou laissé après la fermeture : %v", err)
	}
	// Réouverture : Open ne pose pas le verrou, Lock le fait ; Unlock le
	// retire sans fermer (arrêt en gardant la session).
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, ok := readPeer(lp); ok {
		t.Fatal("verrou posé par Open")
	}
	if err := d.Lock(); err != nil {
		t.Fatal(err)
	}
	if _, ok := readPeer(lp); !ok {
		t.Fatal("verrou absent après Lock")
	}
	d.Unlock()
	if _, ok := readPeer(lp); ok {
		t.Fatal("verrou laissé par Unlock")
	}
}

// Un verrou d'un autre poste est respecté tant qu'il est frais ; abandonné
// (pas rafraîchi depuis lockStale), il est remplacé.
func TestPeerLockForeign(t *testing.T) {
	useRoot(t)
	d, path := saved(t, "partage.lattis")
	d.Unlock()
	other := Holder{User: "Jean Dupont", Login: `COMPTA\jdupont`, Machine: "PC-COMPTA", PID: 4242, Since: time.Now().Add(-time.Hour)}
	writePeer(t, path, other, time.Minute)
	h := LockedBy(path)
	if h == nil || h.User != "Jean Dupont" {
		t.Fatalf("verrou d'un autre poste ignoré : %+v", h)
	}
	if s := h.String(); !strings.Contains(s, "Jean Dupont") || !strings.Contains(s, "PC-COMPTA") {
		t.Fatalf("description %q", s)
	}
	var le *LockedError
	if err := d.Lock(); !errors.As(err, &le) || le.Holder.Machine != "PC-COMPTA" {
		t.Fatalf("Lock sur un verrou tenu : %v", err)
	}
	if !strings.Contains(le.Error(), "partage.lattis") {
		t.Fatalf("message %q", le.Error())
	}
	// Enregistrer sous ce nom un autre classeur : refusé.
	d2, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer d2.Close()
	if err := d2.Save(context.Background(), path); !errors.As(err, &le) {
		t.Fatalf("enregistrement sur un fichier ouvert ailleurs : %v", err)
	}

	// Abandonné : ignoré et remplacé.
	writePeer(t, path, other, lockStale+time.Hour)
	if h := LockedBy(path); h != nil {
		t.Fatalf("verrou abandonné vu comme tenu : %+v", h)
	}
	if err := d.Lock(); err != nil {
		t.Fatal(err)
	}
	if h, _ := readPeer(LockPath(path)); h.PID != os.Getpid() {
		t.Fatalf("verrou non remplacé : %+v", h)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

// Sur ce poste, le verrou d'un processus disparu est abandonné, même frais.
func TestPeerLockDeadProcess(t *testing.T) {
	useRoot(t)
	d, path := saved(t, "mort.lattis")
	defer d.Close()
	d.Unlock()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	me := whoAmI()
	me.PID = cmd.Process.Pid
	writePeer(t, path, me, 0)
	if h := LockedBy(path); h != nil {
		t.Fatalf("verrou d'un processus disparu vu comme tenu : %+v", h)
	}
	me.PID = os.Getppid() // go test : vivant
	writePeer(t, path, me, 0)
	if LockedBy(path) == nil {
		t.Fatal("verrou d'un processus vivant ignoré")
	}
}

// Fichier modifié par ailleurs depuis l'ouverture : Save refuse de
// l'écraser, SaveOver l'écrase ; une heure retouchée sans changement de
// contenu (outil de synchronisation) n'est pas une modification.
func TestChangedOnDisk(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	d, path := saved(t, "commun.lattis")
	defer d.Close()

	// Heure seule retouchée : pas de conflit.
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if changed, err := d.Changed(); err != nil || changed {
		t.Fatalf("heure retouchée vue comme une modification : %v %v", changed, err)
	}
	exec1(t, d, `INSERT INTO t (v) VALUES ('b')`)
	if err := d.Save(ctx, ""); err != nil {
		t.Fatal(err)
	}

	// Un autre poste enregistre le même fichier.
	b := otherFile(t)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, _ := d.Changed(); !changed {
		t.Fatal("modification par ailleurs non vue")
	}
	exec1(t, d, `INSERT INTO t (v) VALUES ('c')`)
	var ce *ChangedError
	if err := d.Save(ctx, ""); !errors.As(err, &ce) {
		t.Fatalf("Save sur un fichier modifié par ailleurs : %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != string(b) {
		t.Fatal("fichier modifié par ailleurs écrasé")
	}
	if err := d.SaveOver(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if changed, _ := d.Changed(); changed {
		t.Fatal("encore modifié après SaveOver")
	}
	if err := d.Save(ctx, ""); err != nil {
		t.Fatalf("enregistrement suivant : %v", err)
	}
}

// Lecture seule : le fichier d'origine n'est pas écrasé ; enregistré sous
// un autre nom, le classeur redevient modifiable et verrouille sa copie.
func TestReadOnlySave(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	d, path := saved(t, "lecture.lattis")
	defer d.Close()
	d.Unlock()
	d.ReadOnly = true
	if err := d.Save(ctx, ""); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Save en lecture seule : %v", err)
	}
	copie := filepath.Join(filepath.Dir(path), "lecture (copie).lattis")
	if err := d.Save(ctx, copie); err != nil {
		t.Fatal(err)
	}
	if _, ok := readPeer(LockPath(copie)); d.ReadOnly || d.Path != copie || !ok {
		t.Fatalf("après Enregistrer sous : lecture seule %v, %s, verrou %v", d.ReadOnly, d.Path, ok)
	}
}

// Session interrompue : la récupération compare le fichier à ce qu'il était
// au début de la session.
func TestRecoverSeesChangedFile(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	d, path := saved(t, "repris.lattis")
	d.Close()
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	exec1(t, d, `INSERT INTO t (v) VALUES ('non enregistré')`)
	if err := d.Release(); err != nil { // comme un arrêt brutal
		t.Fatal(err)
	}
	// Un autre poste enregistre le fichier entre-temps.
	other, err := Open(path)
	var rec *RecoverableError
	if !errors.As(err, &rec) {
		t.Fatalf("session interrompue non proposée : %v %v", other, err)
	}
	if err := os.WriteFile(path, otherFile(t), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err = Recover(rec.Orphan)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if changed, _ := d.Changed(); !changed {
		t.Fatal("fichier modifié depuis la session non vu à la récupération")
	}
	var ce *ChangedError
	if err := d.Save(ctx, ""); !errors.As(err, &ce) {
		t.Fatalf("Save après récupération : %v", err)
	}
}

// Dossier où le verrou ne peut pas être posé : une erreur, pas un verrou
// d'un autre.
func TestPeerLockImpossible(t *testing.T) {
	useRoot(t)
	d, path := saved(t, "bloque.lattis")
	defer d.Close()
	d.Unlock()
	lp := LockPath(path)
	// Un dossier non vide à la place du verrou : impossible à créer comme à
	// supprimer.
	if err := os.MkdirAll(filepath.Join(lp, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := d.Lock()
	var le *LockedError
	if err == nil || errors.As(err, &le) {
		t.Fatalf("verrou impossible : %v", err)
	}
	if LockedBy(path) != nil {
		t.Fatal("verrou illisible vu comme tenu")
	}
}
