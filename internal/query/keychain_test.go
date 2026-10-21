package query

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/keychain"
	"github.com/lalamefine/lattis/internal/remote/auth"
)

// blocking est un trousseau qui ne répond jamais (Linux sans Secret Service
// joignable : invite de déverrouillage que rien n'affiche).
type blocking struct{ c chan struct{} }

func (b blocking) Get(string, string) (string, error) { <-b.c; return "", nil }
func (b blocking) Set(string, string, string) error   { <-b.c; return nil }
func (b blocking) Delete(string, string) error        { <-b.c; return nil }

// Rapport de la v0.3.3-beta : sans trousseau du système, l'ouverture d'un
// classeur partagé restait sur « Ouverture… ». Le gestionnaire des Serveurs
// Lattis lisait le jeton au trousseau en tenant son verrou ; l'appel ne
// revenait jamais, ni lui ni les suivants (LattisState, Session). Borné, il
// rend la main : session à rouvrir, jetons et mots de passe en mémoire.
func TestBlockingKeychainDoesNotHang(t *testing.T) {
	b := blocking{make(chan struct{})}
	defer close(b.c)
	old := keychain.Timeout
	keychain.Timeout = 50 * time.Millisecond
	keychain.Use(b)
	defer func() {
		keychain.Timeout = old
		keychain.Use(keychain.System{})
	}()

	// Index d'un poste où l'utilisateur s'était connecté (jeton au trousseau).
	index := filepath.Join(t.TempDir(), "serveurs-lattis.json")
	if err := os.WriteFile(index, []byte(`{"servers":[{"url":"https://lattis.acme.fr","name":"Acme","user":{"email":"alice@acme.fr"}}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewSecrets(true, "")
	m := auth.NewManager(auth.Options{Store: LattisTokens(s), IndexPath: index})

	done := make(chan []auth.ServerState, 1)
	go func() { done <- m.Servers() }()
	var servers []auth.ServerState
	select {
	case servers = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Servers bloqué par le trousseau")
	}
	if len(servers) != 1 || servers[0].User != nil {
		t.Fatalf("Servers = %+v (attendu : Acme, sans session)", servers)
	}
	if _, _, err := m.Session(t.Context(), "https://lattis.acme.fr"); err == nil {
		t.Fatal("Session sans jeton : connexion attendue")
	}
	if !keychain.Unavailable() {
		t.Fatal("trousseau pas tenu pour indisponible")
	}

	// Jeton reçu à la connexion, mot de passe « Mémoriser » : gardés en
	// mémoire, sans erreur, sans attendre.
	start := time.Now()
	ts := LattisTokens(s)
	if err := ts.Set("lattis://lattis.acme.fr/alice@acme.fr", "rt"); err != nil {
		t.Fatal(err)
	}
	if v, ok := ts.Get("lattis://lattis.acme.fr/alice@acme.fr"); !ok || v != "rt" {
		t.Fatalf("jeton = %q, %v", v, ok)
	}
	if err := s.Set("pg://louis@db/ventes", "secret", true); err != nil {
		t.Fatal(err)
	}
	if !s.Remembered("pg://louis@db/ventes") {
		t.Fatal("mot de passe pas gardé pour la session")
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("trousseau encore attendu : %v", d)
	}
}
