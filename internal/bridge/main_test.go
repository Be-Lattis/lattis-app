package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain isole les essais du paquet : la liste des classeurs récents
// (recent.json) de l'utilisateur n'est jamais touchée, même par un essai
// qui ouvre un classeur sans surcharger recentFile.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "lattis-bridge-test")
	if err != nil {
		panic(err)
	}
	recentFile = func() (string, error) { return filepath.Join(dir, "recent.json"), nil }
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
