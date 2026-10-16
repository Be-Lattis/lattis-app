package query

import (
	"sync"

	"github.com/zalando/go-keyring"
)

// Les mots de passe des connexions ne sont jamais écrits dans le classeur :
// ils sont gardés en mémoire pour la session et, si l'utilisateur coche
// « Mémoriser », dans le trousseau du système (Gestionnaire d'identification
// Windows, Trousseau macOS, Secret Service Linux). Un trousseau absent ou en
// panne n'empêche rien : on retombe sur la mémoire.

// keyringService est le service sous lequel les mots de passe sont rangés.
const keyringService = "Slitable"

// Secrets garde les mots de passe des connexions, par compte
// (postgres://utilisateur@hôte:port/base).
type Secrets struct {
	mu  sync.Mutex
	mem map[string]string
	// keyring : utiliser le trousseau du système (faux dans les tests).
	keyring bool
}

// NewSecrets crée un coffre ; useKeyring : lire et écrire le trousseau.
func NewSecrets(useKeyring bool) *Secrets {
	return &Secrets{mem: map[string]string{}, keyring: useKeyring}
}

// DefaultSecrets est le coffre de l'application (trousseau du système).
var DefaultSecrets = NewSecrets(true)

// Get donne le mot de passe connu d'un compte : mémoire, puis trousseau.
func (s *Secrets) Get(account string) (string, bool) {
	s.mu.Lock()
	pw, ok := s.mem[account]
	s.mu.Unlock()
	if ok {
		return pw, true
	}
	if !s.keyring {
		return "", false
	}
	pw, err := keyring.Get(keyringService, account)
	if err != nil {
		return "", false
	}
	s.mu.Lock()
	s.mem[account] = pw
	s.mu.Unlock()
	return pw, true
}

// Set garde un mot de passe pour la session, et dans le trousseau si
// remember. Renvoie l'erreur du trousseau (le mot de passe reste en mémoire).
func (s *Secrets) Set(account, pw string, remember bool) error {
	s.mu.Lock()
	s.mem[account] = pw
	s.mu.Unlock()
	if remember && s.keyring {
		return keyring.Set(keyringService, account, pw)
	}
	return nil
}

// Forget oublie un mot de passe (mémoire et trousseau).
func (s *Secrets) Forget(account string) error {
	s.mu.Lock()
	delete(s.mem, account)
	s.mu.Unlock()
	if s.keyring {
		if err := keyring.Delete(keyringService, account); err != nil && err != keyring.ErrNotFound {
			return err
		}
	}
	return nil
}

// Remembered indique si le trousseau du système connaît le compte.
func (s *Secrets) Remembered(account string) bool {
	if !s.keyring {
		return false
	}
	_, err := keyring.Get(keyringService, account)
	return err == nil
}
