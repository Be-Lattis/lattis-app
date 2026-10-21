package coedit_test

// Essais tirés de l'audit du mode en ligne (docs/audits/audit-en-ligne-2026-10-20.md).

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/coedit"
)

// Constat 5 : Bob perd le dossier partagé plus de 30 s (Stale) ; pendant ce
// temps Alice saisit et enregistre. Le segment d'Alice n'est pas compacté
// tant que Bob est attendu (AwayKeep) : revenu, il reçoit sa saisie.
func TestCompactedWhileAway(t *testing.T) {
	path := workbook(t)
	ctx := context.Background()
	alice := open(t, "alice", path)
	bob := join(t, "bob", path, alice)
	settle(t, alice, bob)

	// Bob coupé : plus aucun tour (ni lecture, ni battement).
	alice.set(0, 1, "42")
	alice.sync()
	time.Sleep(fast.Stale + 50*time.Millisecond)
	alice.sync()
	if st := alice.e.State(); len(st.Peers) != 0 {
		t.Fatalf("Bob devrait être parti pour Alice : %+v", st)
	}
	must(t, alice.d.Save(ctx, ""))
	for range 5 {
		alice.sync()
		time.Sleep(2 * time.Millisecond)
	}
	entries, _ := os.ReadDir(coedit.DirOf(path))
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	t.Logf("dossier après compactage : %s", strings.Join(names, ", "))

	// Bob revient ; Alice continue dans son nouveau segment.
	alice.set(1, 1, "43")
	alice.sync()
	for end := time.Now().Add(3 * time.Second); time.Now().Before(end) && strings.Join(alice.rows(), "|") != strings.Join(bob.rows(), "|"); {
		bob.sync()
		alice.sync()
		time.Sleep(2 * time.Millisecond)
	}
	t.Logf("Alice : %v", alice.rows())
	t.Logf("Bob   : %v", bob.rows())
	if strings.Join(alice.rows(), "\n") != strings.Join(bob.rows(), "\n") {
		t.Errorf("Bob n'a pas reçu ce qu'Alice a fait pendant sa coupure")
	}
}

// Au retour, Bob saisit : Alice le reçoit.
func TestReturnAfterCompaction(t *testing.T) {
	path := workbook(t)
	ctx := context.Background()
	alice := open(t, "alice", path)
	bob := join(t, "bob", path, alice)
	bob.set(2, 1, "5")
	settle(t, alice, bob)
	time.Sleep(fast.Stale + 50*time.Millisecond)
	alice.sync()
	must(t, alice.d.Save(ctx, ""))
	for range 5 {
		alice.sync()
		time.Sleep(2 * time.Millisecond)
	}
	bob.set(2, 1, "6")
	// Machine d'intégration lente : des tours jusqu'à 3 s, pas un nombre fixe.
	for end := time.Now().Add(3 * time.Second); time.Now().Before(end) && alice.rows()[2] != bob.rows()[2]; {
		bob.sync()
		alice.sync()
		time.Sleep(2 * time.Millisecond)
	}
	t.Logf("Alice : %v ; pairs=%d", alice.rows(), len(alice.e.State().Peers))
	t.Logf("Bob   : %v", bob.rows())
	if alice.rows()[2] != bob.rows()[2] {
		t.Errorf("la saisie de Bob après son retour n'arrive pas chez Alice")
	}
}
