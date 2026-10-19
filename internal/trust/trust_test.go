package trust

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Les accords survivent au redémarrage ; un refus ne vaut que pour la
// session.
func TestStorePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Lattis", "accords.json")
	s := New(path, nil)
	h := Hash("print(1)\r\n")
	if h != Hash("print(1)\n") {
		t.Fatal("empreinte sensible aux fins de ligne")
	}
	if s.ScriptAllowed(h) {
		t.Fatal("autorisé d'office")
	}
	s.RefuseScript(h)
	if !s.ScriptRefused(h) {
		t.Fatal("refus oublié")
	}
	if err := s.AllowScript(h, "Calcul"); err != nil {
		t.Fatal(err)
	}
	if s.ScriptRefused(h) {
		t.Fatal("refus gardé après l'accord")
	}
	if err := s.AllowHosts("classeur:a", "API.example.com ", "autre.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := s.ForgetHost("classeur:a", "autre.example.com"); err != nil {
		t.Fatal(err)
	}

	s2 := New(path, nil)
	if !s2.ScriptAllowed(h) || s2.ScriptAllowed(Hash("print(2)")) {
		t.Fatal("accords des scripts non relus")
	}
	if got := s2.Hosts("classeur:a"); len(got) != 1 || got[0] != "api.example.com" {
		t.Fatalf("hôtes %v", got)
	}
	if !s2.HostAllowed("classeur:a", "api.example.com") || s2.HostAllowed("classeur:b", "api.example.com") {
		t.Fatal("hôte accordé à un autre classeur")
	}
	if fi, err := os.Stat(path); err != nil || fi.Size() == 0 {
		t.Fatalf("fichier des accords : %v", err)
	}
}

// À la création du fichier, les classeurs récents sont notés comme anciens,
// une seule fois.
func TestLegacy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accords.json")
	old := filepath.Join(dir, "Ventes.lattis")
	calls := 0
	recent := func() []string { calls++; return []string{old} }
	s := New(path, recent)
	if s.ClaimLegacy(filepath.Join(dir, "Autre.lattis")) {
		t.Fatal("classeur inconnu repris")
	}
	if !s.ClaimLegacy(strings.ToUpper(old[:1]) + old[1:]) {
		t.Fatal("classeur récent non repris")
	}
	if s.ClaimLegacy(old) {
		t.Fatal("reprise faite deux fois")
	}
	s2 := New(path, recent)
	if s2.ClaimLegacy(old) || calls != 1 {
		t.Fatalf("liste relue (%d fois) ou reprise refaite", calls)
	}
}

// Les accords des pages web survivent au redémarrage, séparés de ceux des
// scripts.
func TestPagesPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accords.json")
	s := New(path, nil)
	h := Hash("<h1>a</h1>\r\n")
	if s.PageAllowed(h) {
		t.Fatal("page autorisée d'office")
	}
	if err := s.AllowPage(h, "Page1"); err != nil {
		t.Fatal(err)
	}
	s2 := New(path, nil)
	if !s2.PageAllowed(Hash("<h1>a</h1>\n")) || s2.ScriptAllowed(h) {
		t.Fatal("accord de page non relu, ou confondu avec un script")
	}
}
