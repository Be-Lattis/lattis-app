package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// Tout premier lancement sur ce poste (profil de l'utilisateur) : l'accueil
// (premiers pas, langue, Nouveau, Ouvrir, récents) s'affiche au lieu d'un
// classeur vide. Un fichier témoin, Lattis/first-launch dans le dossier de
// configuration de l'utilisateur (à côté de config.json), le retient :
//
//	absent    premier appel : un profil déjà utilisé (config.json,
//	          device-id ou recent.json présents) n'est pas un premier
//	          lancement (« done ») ; sinon « pending »
//	pending   l'accueil n'a pas encore mené à un classeur : il revient au
//	          lancement suivant (langue choisie puis fenêtre fermée)
//	done      lancements suivants : comportement habituel
//
// Témoin impossible à écrire : pas de premier lancement (le comportement
// habituel plutôt qu'un accueil à chaque lancement). Édition web : jamais
// (le dossier est celui du serveur, partagé par tous ses utilisateurs ; la
// page s'ouvre comme avant sur un classeur vide).

const (
	firstLaunchPending = "pending"
	firstLaunchDone    = "done"
)

// firstLaunchFile est le fichier témoin ; surchargeable pour les tests.
var firstLaunchFile = func() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Lattis", "first-launch"), nil
}

// profileFiles sont les fichiers d'un profil déjà utilisé ; surchargeable
// pour les tests.
var profileFiles = func() []string {
	var out []string
	if dir, err := os.UserConfigDir(); err == nil {
		out = append(out, filepath.Join(dir, "Lattis", "config.json"), filepath.Join(dir, "Lattis", "device-id"))
	}
	if p, err := recentFile(); err == nil {
		out = append(out, p)
	}
	return out
}

// FirstLaunch dit si c'est le tout premier lancement de Lattis sur ce poste
// (accueil au lieu d'un classeur vide), jusqu'à FirstLaunchDone.
func (s *DocumentService) FirstLaunch(ctx context.Context) bool {
	if s.sys().Platform().Web {
		return false
	}
	p, err := firstLaunchFile()
	if err != nil {
		return false
	}
	return firstLaunchAt(p, profileFiles())
}

// FirstLaunchDone retient que l'accueil du premier lancement a mené à un
// classeur : les lancements suivants s'ouvrent comme d'habitude.
func (s *DocumentService) FirstLaunchDone(ctx context.Context) {
	if s.sys().Platform().Web {
		return
	}
	if p, err := firstLaunchFile(); err == nil {
		_ = writeFirstLaunch(p, firstLaunchDone)
	}
}

// firstLaunchAt lit (ou crée) le témoin marker ; evidence : les fichiers
// d'un profil déjà utilisé.
func firstLaunchAt(marker string, evidence []string) bool {
	if b, err := os.ReadFile(marker); err == nil {
		return strings.TrimSpace(string(b)) == firstLaunchPending
	} else if !os.IsNotExist(err) {
		return false
	}
	state := firstLaunchPending
	for _, f := range evidence {
		if _, err := os.Stat(f); err == nil {
			state = firstLaunchDone
			break
		}
	}
	if writeFirstLaunch(marker, state) != nil {
		return false
	}
	return state == firstLaunchPending
}

func writeFirstLaunch(path, state string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(state+"\n"), 0o600)
}
