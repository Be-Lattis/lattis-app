package engine

import (
	"testing"
	"time"
)

// Classeur de la session en cours d'ouverture (--workbook) : la page se
// connecte déjà ; ses appels attendent la fin, sauf « Annuler » et ce qui
// affiche la page.
func TestStartingHoldsCalls(t *testing.T) {
	e, srv := newEngine(t, Options{})
	release := make(chan struct{})
	e.Starting(func() { <-release })
	c := dial(t, srv)

	c.mustCall("DocumentService.Platform")
	if _, err := c.call("DocumentService.CancelWork", 123456); err != nil {
		t.Fatalf("Annuler retenu : %+v", err)
	}
	got := make(chan struct{})
	c2 := dial(t, srv)
	go func() {
		c2.call("DocumentService.State")
		close(got)
	}()
	select {
	case <-got:
		t.Fatal("appel servi pendant l'ouverture")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	select {
	case <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("appel toujours retenu après l'ouverture")
	}
}
