package bridge

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestLaunchLinkArgs(t *testing.T) {
	if !IsDesktopLink("LATTIS://open?server=x") || IsDesktopLink("C:\\lattis\\a.lattis") || IsDesktopLink("--demo") {
		t.Fatal("IsDesktopLink")
	}
	want := filepath.FromSlash("/home/léa/Ventes 2026.lattis")
	if runtime.GOOS == "windows" {
		want = filepath.FromSlash("C:/Users/lea/Ventes 2026.lattis")
		if got := FileURI("file:///C:/Users/lea/Ventes%202026.lattis"); got != want {
			t.Fatalf("FileURI = %q", got)
		}
	} else if got := FileURI("file:///home/l%C3%A9a/Ventes%202026.lattis"); got != want {
		t.Fatalf("FileURI = %q", got)
	}
	if got := FileURI("Ventes.lattis"); got != "Ventes.lattis" {
		t.Fatalf("chemin simple : %q", got)
	}
	if got := FileURI("file://autre-poste/partage/a.lattis"); got != "file://autre-poste/partage/a.lattis" {
		t.Fatalf("autre hôte : %q", got)
	}
}

// Un lien reçu avant que l'interface l'ait demandé attend LaunchLink ; il
// n'est rendu qu'une fois.
func TestRequestLink(t *testing.T) {
	s := &DocumentService{}
	s.RequestLink("lattis://open?x")
	if got := s.LaunchLink(); got != "lattis://open?x" {
		t.Fatalf("LaunchLink = %q", got)
	}
	if got := s.LaunchLink(); got != "" {
		t.Fatalf("deuxième LaunchLink = %q", got)
	}
}
