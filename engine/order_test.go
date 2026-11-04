package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// burst envoie les appels d'un bloc, sans attendre les réponses (file d'un
// onglet vidée à la reconnexion, appels retenus pendant l'ouverture, saisie
// rapide derrière une liaison lente), puis attend toutes les réponses.
func (c *client) burst(calls [][]any) {
	c.t.Helper()
	first := c.next + 1
	for _, a := range calls {
		c.next++
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": c.next, "method": a[0], "params": a[1:]})
		if err := c.ws.Write(context.Background(), websocket.MessageText, b); err != nil {
			c.t.Fatal(err)
		}
	}
	for i := first; i <= c.next; i++ {
		select {
		case m := <-c.resp:
			if e, ok := m["error"]; ok {
				c.t.Fatalf("réponse %s : %s", m["id"], e)
			}
		case <-time.After(20 * time.Second):
			c.t.Fatal("réponses manquantes")
		}
	}
}

func sheetCSV(t *testing.T, c *client) string {
	t.Helper()
	var p string
	json.Unmarshal(c.mustCall("DocumentService.ExportSheetCSV", 1, "x"), &p)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	f := strings.Split(strings.TrimSpace(lines[len(lines)-1]), ";")
	return f[len(f)-1]
}

// Les appels d'un onglet s'exécutent dans l'ordre d'arrivée : la dernière
// saisie d'une cellule l'emporte, même envoyée d'un bloc.
func TestCallsRunInOrder(t *testing.T) {
	_, srv := newEngine(t, Options{})
	c := dial(t, srv)
	c.mustCall("DocumentService.New")
	for round := 0; round < 5; round++ {
		var calls [][]any
		for i := 1; i <= 60; i++ {
			calls = append(calls, []any{"DocumentService.SetCell", 1, 1, 1, fmt.Sprint(round*1000 + i)})
		}
		c.burst(calls)
		if got, want := strings.TrimSpace(sheetCSV(t, c)), fmt.Sprint(round*1000+60); got != want {
			t.Fatalf("tour %d : cellule %q, attendu %q", round, got, want)
		}
	}
}

// Appels retenus pendant l'ouverture du classeur de la session : relâchés
// dans l'ordre où ils sont arrivés.
func TestStartingReleasesInOrder(t *testing.T) {
	e, srv := newEngine(t, Options{})
	c0 := dial(t, srv)
	c0.mustCall("DocumentService.New")
	release := make(chan struct{})
	e.Starting(func() { <-release })
	c := dial(t, srv)
	done := make(chan struct{})
	go func() {
		defer close(done)
		var calls [][]any
		for i := 1; i <= 60; i++ {
			calls = append(calls, []any{"DocumentService.SetCell", 1, 1, 1, fmt.Sprint(i)})
		}
		c.burst(calls)
	}()
	time.Sleep(300 * time.Millisecond)
	close(release)
	<-done
	if got := strings.TrimSpace(sheetCSV(t, c)); got != "60" {
		t.Fatalf("cellule %q après l'ouverture, attendu 60", got)
	}
}
