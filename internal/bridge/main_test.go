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
	// Ni l'index des serveurs ni le trousseau de l'utilisateur.
	serverLabelOf = func(string) string { return "" }
	// Essais rédigés en français, quelle que soit la langue de la machine (CI en anglais).
	os.Setenv("LATTIS_SYSTEM_LOCALE", "fr-FR")
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
