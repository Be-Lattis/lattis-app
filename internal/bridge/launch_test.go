package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsWorkbook(t *testing.T) {
	for p, want := range map[string]bool{
		`C:\Users\a\Budget.lattis`: true,
		"/home/a/Budget.LATTIS":    true,
		"Budget.lattis.bak":        false,
		"Budget.xlsx":              false,
		"lattis":                   false,
	} {
		if got := IsWorkbook(p); got != want {
			t.Errorf("IsWorkbook(%q) = %v, attendu %v", p, got, want)
		}
	}
}

func TestLaunchFileOnce(t *testing.T) {
	var s DocumentService
	s.RequestOpen("")
	s.RequestOpen("a.lattis")
	if got := s.LaunchFile(); got != "a.lattis" {
		t.Fatalf("LaunchFile() = %q, attendu a.lattis", got)
	}
	if got := s.LaunchFile(); got != "" {
		t.Fatalf("second LaunchFile() = %q, attendu vide", got)
	}
}

func TestLaunchArg(t *testing.T) {
	dir := t.TempDir()
	book := filepath.Join(dir, "Budget.lattis")
	if err := os.WriteFile(book, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if p, ok := LaunchArg(book); !ok || p != book {
		t.Fatalf("classeur existant : %q %v", p, ok)
	}
	if p, ok := LaunchArg(filepath.Join(dir, "Absent.lattis")); !ok || p != "" {
		t.Fatalf("classeur absent : %q %v", p, ok)
	}
	for _, a := range []string{"--demo", "med", dir, ""} {
		if _, ok := LaunchArg(a); ok {
			t.Fatalf("%q pris pour un classeur", a)
		}
	}
}
