package bridge

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestRecent(t *testing.T) {
	dir := t.TempDir()
	recentFile = func() (string, error) { return filepath.Join(dir, "cache", "recent.json"), nil }
	var files []string
	for _, n := range []string{"a", "b", "c"} {
		p := filepath.Join(dir, n+".lattis")
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
		addRecent(p)
	}
	addRecent(files[0]) // rouvert : repasse en tête, sans doublon
	os.Remove(files[1]) // disparu : n'est plus proposé
	got := (&DocumentService{}).Recent(context.Background())
	if want := []string{files[0], files[2]}; !slices.Equal(got, want) {
		t.Fatalf("Recent = %v, attendu %v", got, want)
	}
}
