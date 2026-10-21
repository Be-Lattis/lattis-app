package ai

import (
	"errors"
	"strings"
	"sync"

	"github.com/lalamefine/lattis/internal/keychain"
)

// Les clés d'API sont personnelles et à l'échelle de l'ordinateur : gardées
// dans le trousseau du système (Gestionnaire d'identification Windows,
// Trousseau macOS, Secret Service Linux), sous le service Lattis et le compte
// ai:<fournisseur>, jamais dans un classeur ni dans config.json. Un trousseau
// absent ou en panne n'empêche rien : la clé reste en mémoire pour la
// session.

const keyringService = "Lattis"

// Keys garde les clés d'API, par fournisseur.
type Keys struct {
	mu  sync.Mutex
	mem map[string]string
	// keyring : utiliser le trousseau du système (faux dans les tests).
	keyring bool
}

// NewKeys crée un coffre ; useKeyring : lire et écrire le trousseau.
func NewKeys(useKeyring bool) *Keys {
	return &Keys{mem: map[string]string{}, keyring: useKeyring}
}

// DefaultKeys est le coffre de l'application.
var DefaultKeys = NewKeys(true)

func account(provider string) string { return "ai:" + provider }

// Get donne la clé d'un fournisseur : mémoire, puis trousseau.
func (k *Keys) Get(provider string) (string, bool) {
	k.mu.Lock()
	v, ok := k.mem[provider]
	k.mu.Unlock()
	if ok {
		return v, v != ""
	}
	if !k.keyring {
		return "", false
	}
	v, err := keychain.Get(keyringService, account(provider))
	if err != nil {
		return "", false
	}
	k.mu.Lock()
	k.mem[provider] = v
	k.mu.Unlock()
	return v, v != ""
}

// Set enregistre la clé d'un fournisseur. Elle reste utilisable pour la
// session même si le trousseau la refuse ; l'erreur du trousseau est rendue
// pour prévenir l'utilisateur qu'elle ne sera pas retenue.
func (k *Keys) Set(provider, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("clé vide")
	}
	k.mu.Lock()
	k.mem[provider] = key
	k.mu.Unlock()
	if !k.keyring {
		return nil
	}
	return keychain.Set(keyringService, account(provider), key)
}

// Delete oublie la clé d'un fournisseur (mémoire et trousseau).
func (k *Keys) Delete(provider string) error {
	k.mu.Lock()
	k.mem[provider] = ""
	k.mu.Unlock()
	if !k.keyring {
		return nil
	}
	if err := keychain.Delete(keyringService, account(provider)); err != nil {
		return err
	}
	return nil
}
