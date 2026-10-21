package bridge

import (
	"errors"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/ai"
	"github.com/lalamefine/lattis/internal/keychain"
)

// brokenKeychain : trousseau du système absent (Linux sans Secret Service).
type brokenKeychain struct{}

var errNoSecretService = errors.New("The name org.freedesktop.secrets was not provided by any .service files")

func (brokenKeychain) Get(string, string) (string, error) { return "", errNoSecretService }
func (brokenKeychain) Set(string, string, string) error   { return errNoSecretService }
func (brokenKeychain) Delete(string, string) error        { return errNoSecretService }

// Sans trousseau : la clé d'API reste pour la session sans erreur, et la
// barre d'état le dit une seule fois.
func TestKeychainUnavailableNotice(t *testing.T) {
	keychain.Use(brokenKeychain{})
	defer keychain.Use(keychain.System{})
	keychainPending.Store(false)
	s := &DocumentService{}
	e := &emitter{}
	UseHost(e, nil, s)
	WatchPanics(s)
	defer panicDocs.Store(nil)

	k := ai.NewKeys(true)
	if err := k.Set("anthropic", "sk-essai"); err != nil {
		t.Fatalf("Set : %v (attendu : gardée pour la session, sans erreur)", err)
	}
	if v, ok := k.Get("anthropic"); !ok || v != "sk-essai" {
		t.Fatalf("Get = %q, %v", v, ok)
	}
	keychain.Get("Lattis", "lattis://acme/alice")
	keychain.Set("Lattis", "pg://louis@db/ventes", "secret")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		e.mu.Lock()
		n := len(e.events)
		e.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.events) != 1 {
		t.Fatalf("évènements : %+v (attendu : un seul avis)", e.events)
	}
	c, ok := e.events[0].data.(UICommand)
	const want = "Pas de trousseau système : connexions et mots de passe gardés jusqu'à la fermeture de Lattis"
	if !ok || c.Kind != "notify" || c.Message != want {
		t.Fatalf("avis : %+v", e.events[0])
	}
	// L'ouverture du classeur partagé qui suit le redit, une fois.
	if got := s.takeKeychainNotice(); got != want {
		t.Fatalf("takeKeychainNotice = %q", got)
	}
	if got := s.takeKeychainNotice(); got != "" {
		t.Fatalf("takeKeychainNotice, deuxième fois = %q", got)
	}
}
