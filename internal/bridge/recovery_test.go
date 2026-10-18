package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lalamefine/lattis/internal/archive"
)

// workRoot isole les répertoires de travail du test.
func workRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "work")
	old := archive.Root
	archive.Root = func() (string, error) { return root, nil }
	t.Cleanup(func() { archive.Root = old })
	return root
}

func workDirs(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// Un classeur neuf, affiché comme le fait l'interface au démarrage (état,
// mise en page, aide, scripts, favoris, partage, écouteurs open), n'écrit
// rien : il reste marqué « rien à récupérer ». Ce n'était pas le cas avant :
// chaque lancement tué (wails3 dev, fermeture brutale) laissait un
// « Classeur sans titre » à récupérer.
func TestNewDocStaysPristineOnStartup(t *testing.T) {
	workRoot(t)
	ctx := context.Background()
	s := &DocumentService{}
	t.Cleanup(func() { s.CloseDocument(ctx) })
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	s.SetActiveSheet(1)
	if _, err := s.State(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Layout(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FormulaHelp(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Scripts(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Favorites(ctx); err != nil {
		t.Fatal(err)
	}
	s.ShareState()
	s.WorkbookReady(ctx)
	if !s.doc.Pristine() {
		t.Fatal("classeur neuf affiché : marque « rien à récupérer » perdue")
	}
	if _, err := s.SetCell(ctx, 1, 0, 0, "42"); err != nil {
		t.Fatal(err)
	}
	if s.doc.Pristine() {
		t.Fatal("classeur modifié encore marqué « rien à récupérer »")
	}
}

// New appelé deux fois (rechargement de l'interface) ne laisse qu'un
// répertoire de travail : celui du classeur remplacé est supprimé.
func TestReplacedDocumentCleaned(t *testing.T) {
	root := workRoot(t)
	ctx := context.Background()
	s := &DocumentService{}
	t.Cleanup(func() { s.CloseDocument(ctx) })
	for range 3 {
		if _, err := s.New(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if dirs := workDirs(t, root); len(dirs) != 1 {
		t.Fatalf("répertoires de travail = %v, attendu 1", dirs)
	}
	if o, err := s.Orphans(); err != nil || len(o) != 0 {
		t.Fatalf("sessions proposées = %v, %v", o, err)
	}
	s.CloseDocument(ctx)
	if dirs := workDirs(t, root); len(dirs) != 0 {
		t.Fatalf("répertoires de travail après fermeture = %v", dirs)
	}
}
