package query

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/lalamefine/lattis/internal/keychain"
)

// Les mots de passe des connexions ne sont jamais écrits dans le classeur :
// ils sont gardés en mémoire pour la session et, si l'utilisateur coche
// « Mémoriser », dans le trousseau du système (Gestionnaire d'identification
// Windows, Trousseau macOS, Secret Service Linux), par compte
// (genre://utilisateur@hôte:port/base) : à l'échelle de l'ordinateur. Un
// trousseau absent, en panne ou qui ne répond pas n'empêche rien : on
// retombe sur la mémoire jusqu'à la fermeture (internal/keychain).
//
// Un classeur ne s'en sert qu'avec l'accord de l'utilisateur : sinon un
// classeur reçu par courriel, qui vise le même serveur et le même
// utilisateur, s'en servirait sans rien demander (pour lire ou modifier la
// base en son nom). Chaque classeur porte un identifiant aléatoire
// (doc.Document.SecretScope) ; la première fois qu'il veut un mot de passe
// enregistré, l'interface demande « Autoriser » (ErrPassword.Stored), et
// l'accord (compte, classeur) est retenu sur ce poste. Saisir soi-même le
// mot de passe dans un classeur vaut accord pour lui.

// keyringService est le service sous lequel les mots de passe sont rangés.
const keyringService = "Lattis"

// Secrets garde les mots de passe des connexions, par compte, et les
// accords des classeurs (For : la vue d'un classeur).
type Secrets struct {
	v     *vault
	scope string // classeur ; vide : aucun contrôle d'accord (tests, outils)
}

type vault struct {
	mu  sync.Mutex
	mem map[string]secret // par compte
	// keyring : utiliser le trousseau du système (faux dans les tests).
	keyring bool
	// grants : classeurs autorisés, par compte ; gardés dans grantsPath
	// (vide : en mémoire seulement).
	grants     map[string][]string
	grantsPath string
	loaded     bool
}

type secret struct {
	pw       string
	remember bool // à garder au trousseau (ou lu dans le trousseau)
}

// NewSecrets crée un coffre ; useKeyring : lire et écrire le trousseau ;
// grantsPath : fichier des accords (vide : en mémoire).
func NewSecrets(useKeyring bool, grantsPath string) *Secrets {
	return &Secrets{v: &vault{mem: map[string]secret{}, keyring: useKeyring, grants: map[string][]string{}, grantsPath: grantsPath}}
}

// DefaultSecrets est le coffre de l'application (trousseau du système,
// accords dans le dossier de configuration de l'utilisateur).
var DefaultSecrets = NewSecrets(true, defaultGrantsPath())

func defaultGrantsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "Lattis", "connexions-autorisees.json")
}

// For rend la vue du coffre propre au classeur scope
// (doc.Document.SecretScope).
func (s *Secrets) For(scope string) *Secrets {
	return &Secrets{v: s.v, scope: scope}
}

// lookup donne le mot de passe connu d'un compte : mémoire, puis trousseau.
func (s *Secrets) lookup(account string) (string, bool) {
	s.v.mu.Lock()
	e, ok := s.v.mem[account]
	s.v.mu.Unlock()
	if ok {
		return e.pw, true
	}
	if !s.v.keyring {
		return "", false
	}
	pw, err := keychain.Get(keyringService, account)
	if err != nil {
		return "", false
	}
	s.v.mu.Lock()
	s.v.mem[account] = secret{pw, true}
	s.v.mu.Unlock()
	return pw, true
}

// Get donne le mot de passe d'un compte, si ce classeur a l'accord de
// l'utilisateur pour s'en servir.
func (s *Secrets) Get(account string) (string, bool) {
	pw, ok := s.lookup(account)
	if !ok || !s.Granted(account) {
		return "", false
	}
	return pw, true
}

// Pending dit qu'un mot de passe est enregistré pour ce compte mais que ce
// classeur n'a pas encore l'accord de s'en servir : l'interface demande
// « Autoriser » plutôt que le mot de passe.
func (s *Secrets) Pending(account string) bool {
	if s.Granted(account) {
		return false
	}
	_, ok := s.lookup(account)
	return ok
}

// Granted dit si ce classeur peut se servir du mot de passe du compte.
func (s *Secrets) Granted(account string) bool {
	if s.scope == "" {
		return true
	}
	s.v.mu.Lock()
	defer s.v.mu.Unlock()
	s.v.load()
	return slices.Contains(s.v.grants[account], s.scope)
}

// Grant donne à ce classeur l'accord de se servir du mot de passe du compte
// (retenu sur ce poste).
func (s *Secrets) Grant(account string) error {
	if s.scope == "" {
		return nil
	}
	s.v.mu.Lock()
	defer s.v.mu.Unlock()
	s.v.load()
	if slices.Contains(s.v.grants[account], s.scope) {
		return nil
	}
	s.v.grants[account] = append(s.v.grants[account], s.scope)
	return s.v.save()
}

// load lit les accords (une fois) ; mu tenu.
func (v *vault) load() {
	if v.loaded {
		return
	}
	v.loaded = true
	if v.grantsPath == "" {
		return
	}
	b, err := os.ReadFile(v.grantsPath)
	if err != nil {
		return
	}
	var g map[string][]string
	if json.Unmarshal(b, &g) == nil && g != nil {
		v.grants = g
	}
}

// save écrit les accords ; mu tenu.
func (v *vault) save() error {
	if v.grantsPath == "" {
		return nil
	}
	b, err := json.MarshalIndent(v.grants, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(v.grantsPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(v.grantsPath, b, 0o600)
}

// Set garde un mot de passe saisi dans ce classeur pour la session, et dans
// le trousseau si remember ; le classeur reçoit l'accord de s'en servir.
// Renvoie l'erreur du trousseau (le mot de passe reste en mémoire).
func (s *Secrets) Set(account, pw string, remember bool) error {
	s.v.mu.Lock()
	s.v.mem[account] = secret{pw, remember}
	s.v.mu.Unlock()
	grantErr := s.Grant(account)
	if remember && s.v.keyring {
		if err := keychain.Set(keyringService, account, pw); err != nil {
			return err
		}
	}
	return grantErr
}

// Forget oublie le mot de passe d'un compte (mémoire et trousseau).
func (s *Secrets) Forget(account string) error {
	s.v.mu.Lock()
	delete(s.v.mem, account)
	s.v.mu.Unlock()
	if s.v.keyring {
		if err := keychain.Delete(keyringService, account); err != nil {
			return err
		}
	}
	return nil
}

// Remembered indique si le trousseau du système connaît le compte (ou, sans
// trousseau, si le mot de passe saisi doit l'être).
func (s *Secrets) Remembered(account string) bool {
	if !s.v.keyring {
		s.v.mu.Lock()
		e, ok := s.v.mem[account]
		s.v.mu.Unlock()
		return ok && e.remember
	}
	_, err := keychain.Get(keyringService, account)
	return err == nil
}
