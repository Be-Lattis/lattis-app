package bridge

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Nom choisi pour un serveur (Gérer les serveurs › Renommer) : pris par les
// récents, les messages de la barre d'état et des classeurs disparus ;
// sans lui, le nom annoncé par le serveur, sinon son hôte.
func TestServerLabelShown(t *testing.T) {
	dir := t.TempDir()
	oldRecent, oldLabel := recentFile, serverLabelOf
	recentFile = func() (string, error) { return filepath.Join(dir, "recent.json"), nil }
	serverLabelOf = func(server string) string {
		if server == "https://srv.test" {
			return "Labo"
		}
		return ""
	}
	t.Cleanup(func() { recentFile, serverLabelOf = oldRecent, oldLabel })

	if got := serverLabel("https://srv.test", "LPO"); got != "Labo" {
		t.Fatalf("serverLabel = %q", got)
	}
	if got := serverLabel("https://autre.test", "LPO"); got != "LPO" {
		t.Fatalf("serverLabel sans nom choisi = %q", got)
	}
	if got := serverLabel("https://autre.test", ""); got != "autre.test" {
		t.Fatalf("serverLabel sans nom = %q", got)
	}
	if err := goneError(workbook.GoneDeleted, "LPO", "https://srv.test"); !strings.Contains(err.Error(), "Labo") {
		t.Fatalf("classeur supprimé : %v", err)
	}
	n := remoteSavedNotice(&workbook.State{Server: "https://srv.test", ServerName: "LPO", Title: "Nids", Mode: workbook.Collaborative, Online: true})
	if !strings.Contains(n, "Labo") || strings.Contains(n, "LPO") {
		t.Fatalf("message d'enregistrement : %q", n)
	}
	pushRecent(RecentFile{Title: "Nids", Server: "https://srv.test", ServerName: "LPO", WorkbookID: "w1"})
	pushRecent(RecentFile{Title: "Oiseaux", Server: "https://autre.test", ServerName: "Autre", WorkbookID: "w2"})
	got := (&DocumentService{}).Recent(context.Background())
	if len(got) != 2 || got[0].ServerName != "Autre" || got[1].ServerName != "Labo" {
		t.Fatalf("récents : %+v", got)
	}
	// Le fichier des récents garde le nom du serveur.
	if raw := readRecent(); raw[1].ServerName != "LPO" {
		t.Fatalf("récents enregistrés : %+v", raw)
	}
}
