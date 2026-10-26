package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// Visite guidée (frontend/src/lib/onboarding) : l'accueil du premier
// lancement la propose tant qu'elle n'a été ni faite ni passée sur ce
// poste. Un fichier témoin, Lattis/tour dans le dossier de configuration
// de l'utilisateur (à côté de first-launch), le retient : « done » (menée
// au bout) ou « skipped » (Passer). Édition web : rien n'est retenu ici (le
// dossier est celui du serveur ; la démo publique retient la visite dans
// la session du navigateur).

const (
	tourDone    = "done"
	tourSkipped = "skipped"
)

// tourFile est le fichier témoin ; surchargeable pour les tests.
var tourFile = func() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Lattis", "tour"), nil
}

// TourState dit ce qu'est devenue la visite guidée sur ce poste : "done",
// "skipped", ou "" (jamais faite ; aussi en édition web, et quand le
// témoin est illisible).
func (s *DocumentService) TourState(ctx context.Context) string {
	if s.sys().Platform().Web {
		return ""
	}
	p, err := tourFile()
	if err != nil {
		return ""
	}
	return readTour(p)
}

// SetTourState retient la fin de la visite guidée : "done" ou "skipped"
// (autre valeur : ignorée). Édition web : rien.
func (s *DocumentService) SetTourState(ctx context.Context, state string) {
	if s.sys().Platform().Web || (state != tourDone && state != tourSkipped) {
		return
	}
	if p, err := tourFile(); err == nil {
		_ = writeFirstLaunch(p, state)
	}
}

func readTour(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	switch v := strings.TrimSpace(string(b)); v {
	case tourDone, tourSkipped:
		return v
	}
	return ""
}
