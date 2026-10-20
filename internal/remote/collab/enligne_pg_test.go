package collab_test

// Essais tirés de l'audit du mode en ligne (docs/audits/audit-en-ligne-2026-10-20.md),
// contre PostgreSQL (LATTIS_PG_TEST=1, voir publication.md §6).

import (
	"context"
	"testing"
)

// Structure changée par Alice pendant que Bob saisit (sa saisie attend
// dans la file) : Bob doit rester en ligne et sa file se vider.
func TestPostgresStructureDuringInput(t *testing.T) {
	for _, c := range []struct {
		name   string
		change func(ctx context.Context, a *client) error
	}{
		{"tableau renommé", func(ctx context.Context, a *client) error { return a.d.Model.RenameTable(ctx, "Ventes", "Achats") }},
		{"colonne supprimée", func(ctx context.Context, a *client) error { return a.d.Model.DeleteColumn(ctx, "Ventes", 2) }},
		{"colonne renommée", func(ctx context.Context, a *client) error { _, err := a.d.SetCell(ctx, 1, 0, 2, "Tarif"); return err }},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := pgSetupClients(t)
			ctx := context.Background()
			alice, bob := p.alice, p.bob
			bob.mustSet(1, 2, "8")
			must(t, c.change(ctx, alice))
			alice.send()
			// Comme le suivi : envoi puis réception, deux tours.
			_, err1 := bob.e.Sync(ctx)
			_, err2 := bob.e.Sync(ctx)
			if err1 != nil || err2 != nil || bob.states()["pending"] != 0 {
				t.Fatalf("Bob : %v / %v, file %v", err1, err2, bob.states())
			}
		})
	}
}
