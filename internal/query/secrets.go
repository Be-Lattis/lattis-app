package query

import (
	"strings"
	"sync"

	"github.com/zalando/go-keyring"
)

// Les mots de passe des connexions ne sont jamais écrits dans le classeur :
// ils sont gardés en mémoire pour la session et, si l'utilisateur coche
// « Mémoriser », dans le trousseau du système (Gestionnaire d'identification
// Windows, Trousseau macOS, Secret Service Linux). Un trousseau absent ou en
// panne n'empêche rien : on retombe sur la mémoire.
//
// Un mot de passe est lié au classeur qui l'a reçu, pas seulement au
// serveur : sinon un classeur reçu par courriel, qui vise le même serveur
// et le même utilisateur, s'en servirait sans rien demander (pour lire ou
// modifier la base au nom de l'utilisateur). Le classeur est désigné par son
// fichier (chemin absolu, voir doc.Document.SecretScope), une donnée de ce
// poste qu'un classeur ne peut pas imiter, à la différence d'un identifiant
// rangé dans le classeur lui-même. Un classeur jamais enregistré a une
// portée de session : ses mots de passe restent en mémoire et passent à son
// fichier au premier enregistrement (Rebind). Le prix : un même serveur
// utilisé par deux classeurs demande son mot de passe une fois par classeur,
// et un classeur déplacé ou renommé le redemande.

// keyringService est le service sous lequel les mots de passe sont rangés.
const keyringService = "Lattis"

// sessionScope préfixe la portée d'un classeur jamais enregistré : elle ne
// survit pas à la session, rien n'en va donc au trousseau.
const sessionScope = "session:"

// Secrets garde les mots de passe des connexions, par compte
// (postgres://utilisateur@hôte:port/base) et par classeur (For).
type Secrets struct {
	v     *vault
	scope string // classeur ; vide : aucun (tests), rien n'est lu au trousseau
}

type vault struct {
	mu  sync.Mutex
	mem map[string]secret // par clé (key)
	// keyring : utiliser le trousseau du système (faux dans les tests).
	keyring bool
}

type secret struct {
	pw       string
	remember bool // à garder au trousseau (ou lu dans le trousseau)
}

// NewSecrets crée un coffre ; useKeyring : lire et écrire le trousseau.
func NewSecrets(useKeyring bool) *Secrets {
	return &Secrets{v: &vault{mem: map[string]secret{}, keyring: useKeyring}}
}

// DefaultSecrets est le coffre de l'application (trousseau du système).
var DefaultSecrets = NewSecrets(true)

// For rend la vue du coffre propre au classeur scope
// (doc.Document.SecretScope).
func (s *Secrets) For(scope string) *Secrets {
	return &Secrets{v: s.v, scope: scope}
}

// key désigne un compte pour ce classeur, en mémoire et au trousseau.
func (s *Secrets) key(account string) string {
	if s.scope == "" {
		return account
	}
	return account + " | classeur " + s.scope
}

// persistent dit si le trousseau sert pour ce classeur.
func (s *Secrets) persistent() bool {
	return s.v.keyring && s.scope != "" && !strings.HasPrefix(s.scope, sessionScope)
}

// Get donne le mot de passe connu d'un compte pour ce classeur : mémoire,
// puis trousseau.
func (s *Secrets) Get(account string) (string, bool) {
	k := s.key(account)
	s.v.mu.Lock()
	e, ok := s.v.mem[k]
	s.v.mu.Unlock()
	if ok {
		return e.pw, true
	}
	if !s.persistent() {
		return "", false
	}
	pw, err := keyring.Get(keyringService, k)
	if err != nil {
		return "", false
	}
	s.v.mu.Lock()
	s.v.mem[k] = secret{pw, true}
	s.v.mu.Unlock()
	return pw, true
}

// Set garde un mot de passe pour la session, et dans le trousseau si
// remember (au premier enregistrement pour un classeur qui n'en a pas
// encore). Renvoie l'erreur du trousseau (le mot de passe reste en mémoire).
func (s *Secrets) Set(account, pw string, remember bool) error {
	k := s.key(account)
	s.v.mu.Lock()
	s.v.mem[k] = secret{pw, remember}
	s.v.mu.Unlock()
	if remember && s.persistent() {
		return keyring.Set(keyringService, k, pw)
	}
	return nil
}

// Forget oublie un mot de passe pour ce classeur (mémoire et trousseau).
func (s *Secrets) Forget(account string) error {
	k := s.key(account)
	s.v.mu.Lock()
	delete(s.v.mem, k)
	s.v.mu.Unlock()
	if s.persistent() {
		if err := keyring.Delete(keyringService, k); err != nil && err != keyring.ErrNotFound {
			return err
		}
	}
	return nil
}

// Remembered indique si le trousseau du système connaît le compte pour ce
// classeur (ou, pour un classeur pas encore enregistré, s'il le connaîtra).
func (s *Secrets) Remembered(account string) bool {
	k := s.key(account)
	if !s.persistent() {
		s.v.mu.Lock()
		e, ok := s.v.mem[k]
		s.v.mu.Unlock()
		return ok && e.remember
	}
	_, err := keyring.Get(keyringService, k)
	return err == nil
}

// Rebind donne au classeur enregistré sous une nouvelle portée (premier
// enregistrement, Enregistrer sous) les mots de passe connus de l'ancienne
// pour cette session ; ceux à mémoriser vont au trousseau sous la nouvelle.
// Ceux de l'ancienne restent (Enregistrer sous laisse l'ancien fichier).
func (s *Secrets) Rebind(from, to string) error {
	if from == to || from == "" || to == "" {
		return nil
	}
	src, dst := s.For(from), s.For(to)
	suffix := src.key("")
	type moved struct {
		account string
		e       secret
	}
	var ms []moved
	s.v.mu.Lock()
	for k, e := range s.v.mem {
		if strings.HasSuffix(k, suffix) {
			ms = append(ms, moved{strings.TrimSuffix(k, suffix), e})
		}
	}
	s.v.mu.Unlock()
	var first error
	for _, m := range ms {
		if err := dst.Set(m.account, m.e.pw, m.e.remember); err != nil && first == nil {
			first = err
		}
	}
	return first
}
