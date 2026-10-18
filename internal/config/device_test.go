package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeviceIDStable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Lattis", "device-id")
	a := deviceIDAt(path)
	if b := deviceIDAt(path); a == "" || a != b {
		t.Fatalf("identifiant du poste instable : %q puis %q", a, b)
	}
	// Fichier illisible : remplacé.
	if err := os.WriteFile(path, []byte("n'importe quoi"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := deviceIDAt(path)
	if c == a || c != deviceIDAt(path) {
		t.Fatalf("identifiant recréé : %q, puis %q", c, deviceIDAt(path))
	}
}
