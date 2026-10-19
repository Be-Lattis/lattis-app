package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "erreurs.log")
	old := errorLogFile
	errorLogFile = func() (string, error) { return p, nil }
	defer func() { errorLogFile = old }()
	var s DocumentService
	if got := s.ReportError(context.Background(), "onglets", "TypeError: x", "at f (a.js:1)", "clic « Renommer »"); got != p {
		t.Fatalf("chemin : %q", got)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"onglets", "TypeError: x", "a.js:1", "Renommer"} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("%q absent de :\n%s", want, b)
		}
	}
}
