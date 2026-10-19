package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Une panique dans une goroutine du paquet est journalisée et signalée au
// lieu d'arrêter l'application (audit 6, M7).
func TestRecoverPanic(t *testing.T) {
	p := filepath.Join(t.TempDir(), "erreurs.log")
	old := errorLogFile
	errorLogFile = func() (string, error) { return p, nil }
	defer func() { errorLogFile = old }()
	s := &DocumentService{}
	e := &emitter{}
	UseHost(e, nil, s)
	WatchPanics(s)
	defer panicDocs.Store(nil)

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer recoverPanic("essai")
		var m map[string]int
		m["x"] = 1 // panique : affectation dans une table nil
	}()
	<-done

	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"panique · essai", "nil map", "panic_test.go"} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("%q absent de :\n%s", want, b)
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.events) != 1 {
		t.Fatalf("évènements : %+v", e.events)
	}
	c, ok := e.events[0].data.(UICommand)
	if !ok || c.Kind != "notify" || !strings.Contains(c.Message, "Erreur interne de Lattis (essai)") || !strings.Contains(c.Message, p) {
		t.Fatalf("message : %+v", e.events[0])
	}
}
