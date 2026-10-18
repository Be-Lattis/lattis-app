package trace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseVerbose(t *testing.T) {
	for _, c := range []struct {
		arg  string
		n    int
		ok   bool
		fail bool
	}{
		{"-v", 1, true, false},
		{"-vv", 2, true, false},
		{"-vvv", 3, true, false},
		{"-vvvvv", 3, true, false},
		{"--verbose", 1, true, false},
		{"--verbose=2", 2, true, false},
		{"--verbose=9", 0, true, true},
		{"--demo", 0, false, false},
		{"-", 0, false, false},
		{"classeur.lattis", 0, false, false},
	} {
		n, ok, err := ParseVerbose(c.arg)
		if n != c.n || ok != c.ok || (err != nil) != c.fail {
			t.Errorf("%s : (%d, %v, %v), attendu (%d, %v, erreur %v)", c.arg, n, ok, err, c.n, c.ok, c.fail)
		}
	}
}

// Le niveau et les catégories décident des lignes écrites ; une ligne
// porte sa catégorie, sa durée et son erreur.
func TestLogf(t *testing.T) {
	file := filepath.Join(t.TempDir(), "lattis.log")
	c, err := ToFile(file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		SetLevel(0)
		Only(nil)
		c.Close()
		mu.Lock()
		out = os.Stderr
		mu.Unlock()
	})
	SetLevel(1)
	Logf(1, Doc, "ouverture de %s", "a.lattis")
	Logf(2, Doc, "trop détaillé")
	err = errors.New("disque plein")
	Since(1, Doc, time.Now(), &err, "enregistrement")
	if Only([]string{"edit"}) != nil {
		t.Fatal("catégorie edit refusée")
	}
	Logf(1, Doc, "hors catégorie")
	Logf(1, Edit, "Saisie")
	if Only([]string{"inconnue"}) == nil {
		t.Fatal("catégorie inconnue acceptée")
	}
	b, _ := os.ReadFile(file)
	got := string(b)
	for _, want := range []string{"[DOC] ouverture de a.lattis", "[DOC] enregistrement (", ") : ERREUR disque plein", "[EDIT] Saisie"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q absent du journal :\n%s", want, got)
		}
	}
	for _, no := range []string{"trop détaillé", "hors catégorie"} {
		if strings.Contains(got, no) {
			t.Errorf("%q écrit à tort :\n%s", no, got)
		}
	}
}
