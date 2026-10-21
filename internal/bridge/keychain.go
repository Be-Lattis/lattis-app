package bridge

import (
	"fmt"
	"os"
	"sync/atomic"

	"github.com/lalamefine/lattis/internal/keychain"
)

// Trousseau du système absent ou qui ne répond pas (Linux sans Secret
// Service, session kiosque, conteneur) : jetons, mots de passe et clés
// restent en mémoire jusqu'à la fermeture (internal/keychain). La barre
// d'état le dit tout de suite, au service désigné par WatchPanics ; comme
// cela arrive surtout pendant l'ouverture d'un classeur partagé (dont la fin
// efface les messages anciens), l'ouverture qui suit le redit
// (takeKeychainNotice).

func init() { keychain.OnUnavailable(keychainUnavailable) }

// keychainPending : avis pas encore repris par une ouverture.
var keychainPending atomic.Bool

func keychainUnavailable() {
	fmt.Fprintln(os.Stderr, "[trousseau] trousseau du système indisponible : secrets gardés en mémoire jusqu'à la fermeture")
	keychainPending.Store(true)
	if s := panicDocs.Load(); s != nil {
		host{s}.Notify(s.say("keychain.unavailable", nil))
	}
}

// takeKeychainNotice rend l'avis « pas de trousseau » s'il n'a pas encore
// été repris par une ouverture, "" sinon.
func (s *DocumentService) takeKeychainNotice() string {
	if !keychainPending.Swap(false) {
		return ""
	}
	return s.say("keychain.unavailable", nil)
}
