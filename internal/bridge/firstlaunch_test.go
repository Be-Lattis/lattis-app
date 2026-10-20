package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// useFirstLaunchDir place le témoin et les fichiers du profil dans dir.
func useFirstLaunchDir(t *testing.T, dir string) (marker string, profile []string) {
	t.Helper()
	marker = filepath.Join(dir, "config", "Lattis", "first-launch")
	profile = []string{filepath.Join(dir, "config", "Lattis", "config.json"), filepath.Join(dir, "cache", "Lattis", "recent.json")}
	oldFile, oldProfile := firstLaunchFile, profileFiles
	firstLaunchFile = func() (string, error) { return marker, nil }
	profileFiles = func() []string { return profile }
	t.Cleanup(func() { firstLaunchFile, profileFiles = oldFile, oldProfile })
	return marker, profile
}

func TestFirstLaunch(t *testing.T) {
	useFirstLaunchDir(t, t.TempDir())
	s := &DocumentService{}
	UseHost(NoHost{}, nil, s)
	ctx := context.Background()
	if !s.FirstLaunch(ctx) {
		t.Fatal("profil neuf : premier lancement attendu")
	}
	// Fermé sans avoir ouvert de classeur : l'accueil revient.
	if !s.FirstLaunch(ctx) {
		t.Fatal("accueil non terminé : premier lancement attendu encore")
	}
	s.FirstLaunchDone(ctx)
	if s.FirstLaunch(ctx) {
		t.Fatal("après FirstLaunchDone : plus de premier lancement")
	}
}

func TestFirstLaunchExistingProfile(t *testing.T) {
	for i := range 2 {
		marker, profile := useFirstLaunchDir(t, t.TempDir())
		if err := os.MkdirAll(filepath.Dir(profile[i]), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(profile[i], []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		s := &DocumentService{}
		UseHost(NoHost{}, nil, s)
		if s.FirstLaunch(context.Background()) {
			t.Fatalf("profil existant (%s) : pas de premier lancement", filepath.Base(profile[i]))
		}
		// Retenu : le profil vidé ensuite (récents effacés) ne le redevient pas.
		os.Remove(profile[i])
		if s.FirstLaunch(context.Background()) {
			t.Fatal("témoin « done » ignoré")
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("témoin non écrit : %v", err)
		}
	}
}

func TestFirstLaunchWeb(t *testing.T) {
	marker, _ := useFirstLaunchDir(t, t.TempDir())
	s := &DocumentService{}
	UseHost(webLike{}, nil, s)
	if s.FirstLaunch(context.Background()) {
		t.Fatal("édition web : jamais de premier lancement")
	}
	s.FirstLaunchDone(context.Background())
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("édition web : témoin écrit (%v)", err)
	}
}

func TestFirstLaunchUnwritable(t *testing.T) {
	dir := t.TempDir()
	// Le dossier du témoin est un fichier : rien ne peut s'y écrire.
	block := filepath.Join(dir, "config")
	if err := os.WriteFile(block, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	useFirstLaunchDir(t, dir)
	s := &DocumentService{}
	UseHost(NoHost{}, nil, s)
	if s.FirstLaunch(context.Background()) {
		t.Fatal("témoin impossible à écrire : comportement habituel attendu")
	}
}
