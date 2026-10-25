package versions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// clock fait avancer l'instant des versions à la main.
func clock(t *testing.T, start time.Time) func(d time.Duration) {
	t.Helper()
	cur := start
	now = func() time.Time { return cur }
	t.Cleanup(func() { now = time.Now })
	return func(d time.Duration) { cur = cur.Add(d) }
}

// save écrit le « classeur » (contenu quelconque), puis en garde une version.
func save(t *testing.T, path, content, name string) (Entry, bool) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// Date de modification avant l'instant de la version (horloge des essais).
	old := now().Add(-time.Second)
	os.Chtimes(path, old, old)
	e, ok, err := Snapshot(path, "Alice", name)
	if err != nil {
		t.Fatal(err)
	}
	return e, ok
}

func TestSnapshotAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Budget.lattis")
	tick := clock(t, time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC))
	tick(time.Second)
	e1, ok := save(t, path, "v1", "")
	if !ok || e1.Author != "Alice" || e1.Size != 2 || !strings.HasPrefix(e1.ID, "Budget ") {
		t.Fatalf("première version : %+v %v", e1, ok)
	}
	// Enregistré sans changement : pas de nouvelle version.
	if _, ok, _ := Snapshot(path, "Alice", ""); ok {
		t.Fatal("version d'un fichier inchangé")
	}
	// Même tranche de 10 minutes : la version automatique est remplacée.
	tick(3 * time.Minute)
	e2, _ := save(t, path, "v2", "")
	if l, _ := List(path); len(l) != 1 || l[0].ID != e2.ID {
		t.Fatalf("même tranche : %+v", l)
	}
	if _, err := os.Stat(filepath.Join(Dir(path), e1.ID)); !os.IsNotExist(err) {
		t.Fatal("copie remplacée gardée")
	}
	// Tranche suivante : une version de plus ; puis une version nommée.
	tick(10 * time.Minute)
	e3, _ := save(t, path, "v3", "")
	tick(time.Minute)
	named, ok := save(t, path, "v3", "Avant envoi")
	if !ok || named.ID != e3.ID || named.Name != "Avant envoi" {
		t.Fatalf("version nommée d'un contenu déjà gardé : %+v", named)
	}
	l, _ := List(path)
	if len(l) != 2 || l[0].ID != e3.ID || l[1].ID != e2.ID {
		t.Fatalf("liste : %+v", l)
	}
	// Restaurer v2 : la version courante est gardée d'abord (déjà là), puis
	// le fichier redevient v2.
	tick(time.Minute)
	os.WriteFile(path, []byte("v4 non gardée"), 0o644)
	if err := Restore(path, e2.ID, "Alice", "Avant restauration"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "v2" {
		t.Fatalf("restauré : %q", b)
	}
	l, _ = List(path)
	if len(l) != 3 || l[0].Name != "Avant restauration" {
		t.Fatalf("version d'avant la restauration : %+v", l)
	}
	f, err := File(path, l[0].ID)
	if b, _ := os.ReadFile(f); err != nil || string(b) != "v4 non gardée" {
		t.Fatalf("copie d'avant la restauration : %q %v", b, err)
	}
	// La copie ouverte en lecture seule retrouve son classeur.
	if orig, ok := OriginalOf(f); !ok || orig != path {
		t.Fatalf("classeur d'une copie : %q %v", orig, ok)
	}
	if _, ok := OriginalOf(path); ok {
		t.Fatal("un classeur pris pour une copie")
	}
	if _, err := File(path, "../Budget.lattis"); err != ErrNotFound {
		t.Fatalf("chemin hors du dossier : %v", err)
	}
	if err := Delete(path, l[0].ID); err != nil {
		t.Fatal(err)
	}
	if l, _ = List(path); len(l) != 2 {
		t.Fatalf("après suppression : %+v", l)
	}
}

// Au plus MaxAuto versions automatiques ; les versions nommées restent.
func TestPruneAuto(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Suivi.lattis")
	tick := clock(t, time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC))
	tick(time.Second)
	first, _ := save(t, path, "nommée", "Départ")
	for i := range MaxAuto + 5 {
		tick(Gap)
		save(t, path, strings.Repeat("x", i+1), "")
	}
	l, _ := List(path)
	autos := 0
	for _, e := range l {
		if e.Auto() {
			autos++
		}
	}
	if autos != MaxAuto || l[len(l)-1].ID != first.ID {
		t.Fatalf("%d versions automatiques, la plus ancienne %+v", autos, l[len(l)-1])
	}
	entries, _ := os.ReadDir(Dir(path))
	if len(entries) != MaxAuto+2 { // copies + index
		t.Fatalf("%d fichiers dans le dossier des versions", len(entries))
	}
}

// La place prise est bornée (10 fois la taille, 50 Mo au moins) : les plus
// anciennes versions automatiques partent.
func TestPruneSize(t *testing.T) {
	es := []Entry{}
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for i := range 5 {
		es = append(es, Entry{ID: string(rune('a' + i)), At: at.Add(time.Duration(i) * time.Hour), Size: 20 << 20})
	}
	out := prune(t.TempDir(), es, "e", 1<<20)
	if len(out) != 2 || out[0].ID != "d" || out[1].ID != "e" {
		t.Fatalf("gardées : %+v", out)
	}
}
