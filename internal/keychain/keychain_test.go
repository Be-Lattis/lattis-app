package keychain

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fake est un trousseau d'essai : en mémoire, qui peut échouer ou bloquer
// (comme go-keyring sous Linux quand l'invite de déverrouillage du Secret
// Service ne peut pas s'afficher).
type fake struct {
	mu    sync.Mutex
	m     map[string]string
	fail  error
	block chan struct{} // non nil : chaque appel attend sa fermeture
	calls atomic.Int32
}

func (f *fake) wait() error {
	f.calls.Add(1)
	if f.block != nil {
		<-f.block
	}
	return f.fail
}

func (f *fake) Get(s, a string) (string, error) {
	if err := f.wait(); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.m[s+"/"+a]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (f *fake) Set(s, a, v string) error {
	if err := f.wait(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m == nil {
		f.m = map[string]string{}
	}
	f.m[s+"/"+a] = v
	return nil
}

func (f *fake) Delete(s, a string) error {
	if err := f.wait(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.m[s+"/"+a]; !ok {
		return ErrNotFound
	}
	delete(f.m, s+"/"+a)
	return nil
}

// setup installe f et un compteur d'avis ; tout est remis en place à la fin.
func setup(t *testing.T, f *fake, timeout time.Duration) *atomic.Int32 {
	t.Helper()
	old := Timeout
	Timeout = timeout
	Use(f)
	n := &atomic.Int32{}
	OnUnavailable(func() { n.Add(1) })
	t.Cleanup(func() {
		Timeout = old
		OnUnavailable(nil)
		Use(System{})
	})
	return n
}

// waitNotice attend que l'avis (envoyé dans sa goroutine) soit arrivé.
func waitNotice(t *testing.T, n *atomic.Int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for n.Load() < want && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond) // un second avis aurait le temps d'arriver
	if got := n.Load(); got != want {
		t.Fatalf("avis : %d, attendu %d", got, want)
	}
}

func TestAvailable(t *testing.T) {
	f := &fake{}
	n := setup(t, f, time.Second)
	if _, err := Get("Lattis", "a"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get inconnu : %v", err)
	}
	if err := Set("Lattis", "a", "s1"); err != nil {
		t.Fatal(err)
	}
	if v, err := Get("Lattis", "a"); err != nil || v != "s1" {
		t.Fatalf("Get = %q, %v", v, err)
	}
	if err := Delete("Lattis", "a"); err != nil {
		t.Fatal(err)
	}
	if err := Delete("Lattis", "a"); err != nil {
		t.Fatalf("Delete d'un compte inconnu : %v", err)
	}
	if Unavailable() {
		t.Fatal("trousseau tenu pour indisponible")
	}
	waitNotice(t, n, 0)
}

// Trousseau qui ne répond pas (D-Bus bloqué) : l'appel rend la main après
// Timeout, la suite passe par la mémoire sans plus attendre, un seul avis.
func TestBlockingFallsBackToMemory(t *testing.T) {
	f := &fake{block: make(chan struct{})}
	defer close(f.block)
	n := setup(t, f, 50*time.Millisecond)

	start := time.Now()
	if err := Set("Lattis", "lattis://acme/alice", "rt"); err != nil {
		t.Fatalf("Set : %v (attendu : gardé en mémoire, sans erreur)", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("Set a attendu %v", d)
	}
	if !Unavailable() {
		t.Fatal("trousseau pas tenu pour indisponible")
	}
	start = time.Now()
	if v, err := Get("Lattis", "lattis://acme/alice"); err != nil || v != "rt" {
		t.Fatalf("Get = %q, %v (attendu : la mémoire)", v, err)
	}
	if _, err := Get("Lattis", "autre"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get inconnu : %v", err)
	}
	if d := time.Since(start); d > 40*time.Millisecond {
		t.Fatalf("appels suivants : %v (le trousseau ne doit plus être appelé)", d)
	}
	if c := f.calls.Load(); c != 1 {
		t.Fatalf("%d appels au trousseau, attendu 1", c)
	}
	if err := Delete("Lattis", "lattis://acme/alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := Get("Lattis", "lattis://acme/alice"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("après Delete : %v", err)
	}
	waitNotice(t, n, 1)
}

// Trousseau en panne (pas de Secret Service, pas de bus D-Bus) : même
// repli, un seul avis.
func TestFailingFallsBackToMemory(t *testing.T) {
	f := &fake{fail: errors.New("The name org.freedesktop.secrets was not provided by any .service files")}
	n := setup(t, f, time.Second)
	if _, err := Get("Lattis", "pg://louis@db/ventes"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get : %v", err)
	}
	if err := Set("Lattis", "pg://louis@db/ventes", "secret"); err != nil {
		t.Fatalf("Set : %v", err)
	}
	if v, err := Get("Lattis", "pg://louis@db/ventes"); err != nil || v != "secret" {
		t.Fatalf("Get = %q, %v", v, err)
	}
	if c := f.calls.Load(); c != 1 {
		t.Fatalf("%d appels au trousseau, attendu 1", c)
	}
	waitNotice(t, n, 1)
}

// Un secret trop long est un refus, pas une panne : l'erreur est rendue.
func TestTooBigIsNotDown(t *testing.T) {
	f := &fake{fail: ErrTooBig}
	n := setup(t, f, time.Second)
	if err := Set("Lattis", "a", "x"); !errors.Is(err, ErrTooBig) {
		t.Fatalf("Set : %v", err)
	}
	if Unavailable() {
		t.Fatal("trousseau tenu pour indisponible")
	}
	waitNotice(t, n, 0)
}

// Avis désigné après la panne : donné tout de suite, une fois.
func TestNoticeAfterDown(t *testing.T) {
	f := &fake{fail: errors.New("dbus: DBUS_SESSION_BUS_ADDRESS not set")}
	setup(t, f, time.Second)
	OnUnavailable(nil)
	Get("Lattis", "a")
	var n atomic.Int32
	OnUnavailable(func() { n.Add(1) })
	Get("Lattis", "b")
	Set("Lattis", "c", "x")
	waitNotice(t, &n, 1)
}
