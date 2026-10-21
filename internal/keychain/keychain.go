// Package keychain : trousseau du système (Gestionnaire d'identification
// Windows, Trousseau macOS, Secret Service Linux) pour les secrets de
// l'utilisateur : jetons des Serveurs Lattis, mots de passe des connexions,
// clés d'API. Jamais de fichier en clair.
//
// Un trousseau absent ou bloqué ne doit rien bloquer : sous Linux sans
// Secret Service (conteneur, session kiosque, pas de gnome-keyring ni de
// KWallet), un appel D-Bus peut échouer, ou attendre sans fin une invite de
// déverrouillage que personne ne peut afficher (go-keyring attend le signal
// Completed sans délai). Chaque appel est donc borné (Timeout) ; au premier
// échec ou dépassement, le trousseau est tenu pour indisponible jusqu'à la
// fermeture : les secrets restent en mémoire (perdus à la fermeture, il
// faudra se reconnecter au lancement suivant), et l'utilisateur est prévenu
// une fois (OnUnavailable).
package keychain

import (
	"errors"
	"runtime"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
)

// ErrNotFound : aucun secret pour ce compte.
var ErrNotFound = errors.New("keychain: secret introuvable")

// ErrTooBig : secret trop long pour le trousseau (refus, pas une panne).
var ErrTooBig = keyring.ErrSetDataTooBig

// errTimeout : le trousseau n'a pas répondu à temps.
var errTimeout = errors.New("keychain: le trousseau du système ne répond pas")

// Backend est un trousseau : celui du système (bureau), ou le coffre de la
// session fourni par le moteur web (bridge.SecretStore). Get rend
// ErrNotFound pour un compte inconnu.
type Backend interface {
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
	Delete(service, account string) error
}

// System est le trousseau du système (go-keyring).
type System struct{}

func (System) Get(service, account string) (string, error) {
	v, err := keyring.Get(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return v, err
}

func (System) Set(service, account, secret string) error {
	return keyring.Set(service, account, secret)
}

func (System) Delete(service, account string) error {
	err := keyring.Delete(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// Timeout borne chaque appel au trousseau. Sous macOS, le système peut
// demander à l'utilisateur d'autoriser l'accès (après chaque mise à jour,
// l'application n'étant pas signée) : on lui laisse le temps de répondre.
var Timeout = defaultTimeout()

func defaultTimeout() time.Duration {
	if runtime.GOOS == "darwin" {
		return time.Minute
	}
	return 5 * time.Second
}

var (
	mu      sync.Mutex
	backend Backend           = System{}
	down    bool              // trousseau indisponible jusqu'à la fermeture
	mem     map[string]string // secrets gardés en mémoire quand down
	notify  func()
	// gen : génération du trousseau (Use) ; un appel commencé avec un autre
	// trousseau ne le déclare pas indisponible.
	gen int
)

// Use remplace le trousseau (moteur web : coffre de la session ; tests) et
// oublie l'état précédent (indisponibilité, secrets en mémoire).
func Use(b Backend) {
	mu.Lock()
	defer mu.Unlock()
	backend, down, mem = b, false, nil
	gen++
}

// OnUnavailable désigne ce qui prévient l'utilisateur, une fois, que le
// trousseau est indisponible (appelé dans sa propre goroutine). Déjà
// indisponible : appelé tout de suite.
func OnUnavailable(f func()) {
	mu.Lock()
	notify = f
	already := down
	mu.Unlock()
	if already && f != nil {
		go f()
	}
}

// Unavailable dit si le trousseau est tenu pour indisponible (secrets en
// mémoire jusqu'à la fermeture).
func Unavailable() bool {
	mu.Lock()
	defer mu.Unlock()
	return down
}

func key(service, account string) string { return service + "\x00" + account }

// call fait un appel au trousseau, borné par Timeout. ok faux : le
// trousseau est indisponible (déjà, ou à cause de cet appel), à remplacer
// par la mémoire.
func call(f func(Backend) error) (ok bool, err error) {
	mu.Lock()
	if down {
		mu.Unlock()
		return false, nil
	}
	b, g := backend, gen
	mu.Unlock()
	done := make(chan error, 1)
	go func() {
		defer func() {
			if v := recover(); v != nil {
				done <- errors.New("keychain: panique du trousseau")
			}
		}()
		done <- f(b)
	}()
	t := time.NewTimer(Timeout)
	defer t.Stop()
	select {
	case err = <-done:
	case <-t.C:
		err = errTimeout
	}
	if err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrTooBig) {
		return true, err
	}
	markDown(g)
	return false, nil
}

// markDown tient le trousseau pour indisponible et prévient (une fois).
func markDown(g int) {
	mu.Lock()
	if down || g != gen {
		mu.Unlock()
		return
	}
	down = true
	f := notify
	mu.Unlock()
	if f != nil {
		go f()
	}
}

// Get donne le secret d'un compte ; ErrNotFound s'il n'y en a pas.
func Get(service, account string) (string, error) {
	var v string
	ok, err := call(func(b Backend) error {
		var err error
		v, err = b.Get(service, account)
		return err
	})
	if ok {
		return v, err
	}
	mu.Lock()
	defer mu.Unlock()
	if s, found := mem[key(service, account)]; found {
		return s, nil
	}
	return "", ErrNotFound
}

// Set range le secret d'un compte ; trousseau indisponible : en mémoire
// jusqu'à la fermeture (pas d'erreur, l'utilisateur est prévenu une fois).
func Set(service, account, secret string) error {
	ok, err := call(func(b Backend) error { return b.Set(service, account, secret) })
	if ok {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if mem == nil {
		mem = map[string]string{}
	}
	mem[key(service, account)] = secret
	return nil
}

// Delete oublie le secret d'un compte (sans erreur s'il n'y en a pas).
func Delete(service, account string) error {
	mu.Lock()
	delete(mem, key(service, account))
	mu.Unlock()
	ok, err := call(func(b Backend) error { return b.Delete(service, account) })
	if !ok || errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}
