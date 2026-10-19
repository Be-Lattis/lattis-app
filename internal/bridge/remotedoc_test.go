package bridge

import (
	"testing"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Ctrl+S sur un classeur du serveur : le message dit ce qui est arrivé au
// serveur, jamais le chemin du cache.
func TestRemoteSavedNotice(t *testing.T) {
	collab := workbook.State{Server: "https://srv.test", ServerName: "LPO", Title: "Fichier nids", Mode: workbook.Collaborative, Online: true}
	for _, c := range []struct {
		st   func(s *workbook.State)
		want string
	}{
		{func(s *workbook.State) {}, "Enregistré sur LPO : Fichier nids"},
		{func(s *workbook.State) { s.ServerName = "" }, "Enregistré sur srv.test : Fichier nids"},
		{func(s *workbook.State) { s.Pending = 2 }, "Enregistré sur LPO : Fichier nids (2 modifications en cours d'envoi)"},
		{func(s *workbook.State) { s.Online = false }, "Hors ligne : « Fichier nids » enregistré sur ce poste ; vos modifications partiront vers LPO à la reconnexion"},
		{func(s *workbook.State) { s.Mode, s.CanDesign = workbook.Frozen, true }, "Copie locale de « Fichier nids » enregistrée ; Fichier › Republier pour publier vos changements sur LPO"},
	} {
		s := collab
		c.st(&s)
		if got := remoteSavedNotice(&s); got != c.want {
			t.Errorf("remoteSavedNotice = %q, attendu %q", got, c.want)
		}
	}
}
