package collab_test

// Essais tirés de l'audit du mode en ligne (docs/audits/audit-en-ligne-2026-10-20.md),
// contre PostgreSQL (LATTIS_PG_TEST=1, voir publication.md §6).

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
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

// Décimal saisi ailleurs (formulaire, SQL, pipeline) avec plus de 17
// chiffres : une modification de Bob ne doit pas tourner en conflit.
func TestPostgresDecimalPrecision(t *testing.T) {
	p := pgSetupClients(t)
	ctx := context.Background()
	alice, bob := p.alice, p.bob
	// D'abord un décimal ordinaire saisi dans l'application.
	alice.mustSet(0, 2, "0,1")
	alice.send()
	bob.receive()
	bob.mustSet(0, 2, "0,2")
	if r := bob.send(); r.Sent != 1 || r.Conflicts != 0 {
		t.Fatalf("0,1 puis 0,2 : %+v", r)
	}
	info := bob.d.Share.Info()
	id := rowID(t, bob.d, 1)
	p.env.Exec(t, `UPDATE `+pgx.Identifier{info.Schema, "Ventes"}.Sanitize()+` SET "Prix" = 0.12345678901234567891 WHERE lattis_id = $1::uuid`, id)
	bob.receive()
	bob.mustSet(1, 2, "7")
	r := bob.send()
	if r.Conflicts == 0 {
		return
	}
	cs, err := bob.e.Conflicts(ctx)
	must(t, err)
	must(t, bob.e.Resolve(ctx, cs[0].ID, true))
	r2 := bob.send()
	t.Fatalf("modification d'un décimal long : %+v (conflit %+v), puis « Garder la mienne » : %+v", r, cs[0], r2)
}

// Conflit, puis la collègue change encore la cellule ; « Garder la sienne »
// doit donner la valeur actuelle du serveur.
func TestPostgresStaleTheirs(t *testing.T) {
	p := pgSetupClients(t)
	ctx := context.Background()
	alice, bob := p.alice, p.bob
	alice.mustSet(1, 2, "11")
	bob.mustSet(1, 2, "12")
	alice.send()
	if r := bob.send(); r.Conflicts != 1 {
		t.Fatalf("conflit attendu : %+v", r)
	}
	alice.mustSet(1, 2, "15")
	alice.send()
	bob.receive()
	cs, err := bob.e.Conflicts(ctx)
	must(t, err)
	must(t, bob.e.Resolve(ctx, cs[0].ID, false))
	bob.receive()
	eq(t, "Bob après « Garder la sienne »", bob.col("Prix"), []string{"4.5", "15.0", "3.25"})
}
