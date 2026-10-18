package demo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	for _, c := range []struct {
		args   []string
		preset string
		seed   uint64
		err    bool
	}{
		{nil, "", 0, false},
		{[]string{"--demo"}, "med", 0, false},
		{[]string{"--demo", "1kx100"}, "1Kx100", 0, false},
		{[]string{"--demo=med", "--seed", "42"}, "med", 42, false},
		{[]string{"-demo", "--seed=7"}, "med", 7, false},
		{[]string{"--demo", "géant"}, "", 0, true},
		{[]string{"--seed", "x"}, "", 0, true},
		{[]string{"--help"}, "", 0, true},
		{[]string{"-h"}, "", 0, true},
		{[]string{"--inconnue"}, "", 0, true},
		{[]string{"fichier.lattis"}, "", 0, true},
		{[]string{"-psn_0_12345"}, "", 0, false},
	} {
		p, s, err := ParseArgs(c.args)
		if p != c.preset || s != c.seed || (err != nil) != c.err {
			t.Errorf("%v : %q %d %v", c.args, p, s, err)
		}
	}
}

// --demo plan.yaml : chemin absolu du fichier, lu tout de suite (ses erreurs
// s'affichent avant l'ouverture de la fenêtre).
func TestParseArgsFichier(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "plan.yaml")
	os.WriteFile(ok, []byte("sheets:\n  - name: A\n    cells: [{cell: A1, value: x}]\n"), 0o644)
	bad := filepath.Join(dir, "faux.yml")
	os.WriteFile(bad, []byte("sheets:\n  - nom: A\n"), 0o644)
	if p, s, err := ParseArgs([]string{"--demo", ok, "--seed", "3"}); err != nil || p != ok || s != 3 {
		t.Errorf("%q %d %v", p, s, err)
	}
	if _, _, err := ParseArgs([]string{"--demo", bad}); err == nil || !strings.Contains(err.Error(), "faux.yml:2 : clé « nom » inconnue") {
		t.Errorf("fichier faux : %v", err)
	}
	if _, _, err := ParseArgs([]string{"--demo", filepath.Join(dir, "absent.yaml")}); err == nil {
		t.Error("fichier absent accepté")
	}
}
