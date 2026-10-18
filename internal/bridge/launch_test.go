package bridge

import "testing"

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
