package bridge

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/lalamefine/lattis/internal/archive"
)

func recentPaths(list []RecentFile) []string {
	var out []string
	for _, r := range list {
		out = append(out, r.Path)
	}
	return out
}

func TestRecent(t *testing.T) {
	dir := t.TempDir()
	old := recentFile
	recentFile = func() (string, error) { return filepath.Join(dir, "cache", "recent.json"), nil }
	t.Cleanup(func() { recentFile = old })
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
	if want := []string{files[0], files[2]}; !slices.Equal(recentPaths(got), want) {
		t.Fatalf("Recent = %v, attendu %v", got, want)
	}
}

// Les classeurs du serveur sont notés par leur serveur et leur titre, à côté
// des chemins des fichiers locaux (liste des versions précédentes lue telle
// quelle) ; ils restent proposés sans leur cache.
func TestRecentRemote(t *testing.T) {
	dir := t.TempDir()
	old, oldCache := recentFile, shareCacheDir
	list := filepath.Join(dir, "recent.json")
	recentFile = func() (string, error) { return list, nil }
	shareCacheDir = func() (string, error) { return filepath.Join(dir, "partages"), nil }
	t.Cleanup(func() { recentFile, shareCacheDir = old, oldCache })
	local := filepath.Join(dir, "Ventes.lattis")
	if err := os.WriteFile(local, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// Liste d'une version précédente : des chemins seulement.
	if err := os.WriteFile(list, []byte(`["`+filepath.ToSlash(local)+`"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(dir, "partages", "srv.test", "480f6888-5ec4-45ee-801b-b0891115cf3f.lattis")
	r := RecentFile{Path: cache, Title: "Fichier nids", Server: "https://srv.test", ServerName: "LPO", WorkbookID: "480f6888-5ec4-45ee-801b-b0891115cf3f"}
	pushRecent(r)
	pushRecent(RecentFile{Path: cache, Title: "Fichier nids (renommé)", Server: r.Server, WorkbookID: r.WorkbookID}) // même classeur : une entrée
	got := (&DocumentService{}).Recent(context.Background())
	if len(got) != 2 {
		t.Fatalf("Recent = %+v", got)
	}
	if g := got[0]; g.Title != "Fichier nids (renommé)" || g.Link != "https://srv.test/w/480f6888-5ec4-45ee-801b-b0891115cf3f" || g.WorkbookID != r.WorkbookID {
		t.Fatalf("classeur du serveur : %+v", g)
	}
	if g := got[1]; g.WorkbookID != "" || filepath.Clean(g.Path) != filepath.Clean(local) {
		t.Fatalf("fichier local : %+v", g)
	}
	if !inShareCache(cache) || inShareCache(local) || inShareCache(filepath.Join(dir, "partages")) {
		t.Fatal("inShareCache")
	}
}

func TestFileTitle(t *testing.T) {
	for in, want := range map[string]string{
		"LPO BFC_Grpe rapaces 71_Fichier nids": "LPO BFC_Grpe rapaces 71_Fichier nids",
		"Ventes 2026/2027 : bilan?":            "Ventes 2026 2027 bilan",
		"  ..  ":                               "Classeur",
	} {
		if got := fileTitle(in); got != want {
			t.Errorf("fileTitle(%q) = %q, attendu %q", in, got, want)
		}
	}
}

// Fichier de l'auteur d'un classeur partagé en direct : une seule entrée,
// le classeur du serveur (demande de Louis, 20/10) ; un classeur figé garde
// son fichier (l'auteur y republie).
func TestRecentAuthorFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	old, oldCache := recentFile, shareCacheDir
	recentFile = func() (string, error) { return filepath.Join(dir, "recent.json"), nil }
	shareCacheDir = func() (string, error) { return filepath.Join(dir, "partages"), nil }
	t.Cleanup(func() { recentFile, shareCacheDir = old, oldCache })
	save := func(name, mode string) string {
		a, err := archive.New()
		if err != nil {
			t.Fatal(err)
		}
		defer a.Close()
		a.Remote = func(context.Context) (*archive.Remote, error) {
			return &archive.Remote{Server: "https://srv.test", WorkbookID: "id-" + mode, Mode: mode}, nil
		}
		p := filepath.Join(dir, name+".lattis")
		if err := a.Save(ctx, p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	collab := save("Fichier nids", "collaborative")
	frozen := save("Bilan", "frozen")
	pushRecent(RecentFile{Path: filepath.Join(dir, "partages", "x.lattis"), Title: "Fichier nids", Server: "https://srv.test", WorkbookID: "id-collaborative"})
	addRecent(frozen)
	addRecent(collab)
	got := (&DocumentService{}).Recent(ctx)
	if len(got) != 2 {
		t.Fatalf("Recent = %+v", got)
	}
	if g := got[0]; g.WorkbookID != "id-collaborative" || g.Path != "" || g.Title != "Fichier nids" {
		t.Fatalf("classeur partagé : %+v", g)
	}
	if g := got[1]; g.WorkbookID != "" || filepath.Clean(g.Path) != filepath.Clean(frozen) {
		t.Fatalf("classeur figé : %+v", g)
	}
}

// Ouvrir le fichier de l'auteur d'un classeur partagé en direct : le
// classeur du serveur (sharedFile) ; un classeur figé ou un fichier ordinaire
// s'ouvre tel quel.
func TestSharedFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	old := shareCacheDir
	shareCacheDir = func() (string, error) { return filepath.Join(dir, "partages"), nil }
	t.Cleanup(func() { shareCacheDir = old })
	save := func(name, mode string) string {
		a, err := archive.New()
		if err != nil {
			t.Fatal(err)
		}
		defer a.Close()
		if mode != "" {
			a.Remote = func(context.Context) (*archive.Remote, error) {
				return &archive.Remote{Server: "https://srv.test", WorkbookID: "480f6888-5ec4-45ee-801b-b0891115cf3f", Mode: mode}, nil
			}
		}
		p := filepath.Join(dir, name+".lattis")
		if err := a.Save(ctx, p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	r := sharedFile(save("Fichier nids", "collaborative"))
	if r == nil || r.WorkbookID != "480f6888-5ec4-45ee-801b-b0891115cf3f" || r.Title != "Fichier nids" || r.Link != "https://srv.test/w/480f6888-5ec4-45ee-801b-b0891115cf3f" || r.Path == "" {
		t.Fatalf("classeur partagé : %+v", r)
	}
	if r := sharedFile(save("Bilan", "frozen")); r != nil {
		t.Fatalf("classeur figé : %+v", r)
	}
	if r := sharedFile(save("Notes", "")); r != nil {
		t.Fatalf("fichier ordinaire : %+v", r)
	}
}
