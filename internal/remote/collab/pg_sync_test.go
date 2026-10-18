package collab_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Retours du serveur (migration 0008, contrat client §6) contre un vrai
// PostgreSQL : échos écartés (changes_since … skip), point de sauvegarde
// par écriture (filtre d'écriture), since_covered, clés d'ordre distinctes.
// Ignoré sans la migration 0008.

func pgHas0008(t *testing.T, p *pgClients) {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, p.env.Super)
	must(t, err)
	defer c.Close(ctx)
	var ok bool
	must(t, c.QueryRow(ctx, `SELECT to_regprocedure('lattis.since_covered(xid8)') IS NOT NULL`).Scan(&ok))
	if !ok {
		t.Skip("serveur sans la migration 0008")
	}
}

// pgAs exécute q sous le rôle d'un utilisateur.
func pgAs(t *testing.T, p *pgClients, role, q string, args ...any) {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, p.env.Super)
	must(t, err)
	defer c.Close(ctx)
	_, err = c.Exec(ctx, `SET SESSION AUTHORIZATION `+pgx.Identifier{role}.Sanitize())
	must(t, err)
	_, err = c.Exec(ctx, q, args...)
	must(t, err)
}

// remoteTable lit le tableau Ventes tel que le poste le connaît.
func remoteTable(t *testing.T, c *client) (workbook.RemoteTable, string) {
	t.Helper()
	var rt workbook.RemoteTable
	var cols, since string
	must(t, c.d.DB().Read().QueryRow(`SELECT remote_id, remote_name, columns, coalesce(since, '') FROM _lattis_remote_tables WHERE published = 1`).Scan(&rt.ID, &rt.Name, &cols, &since))
	must(t, json.Unmarshal([]byte(cols), &rt.Columns))
	return rt, since
}

func TestPostgresSyncGaps(t *testing.T) {
	p := pgSetupClients(t)
	pgHas0008(t, p)
	ctx := context.Background()
	alice, bob := p.alice, p.bob

	// Échos : les écritures de Bob ne lui reviennent pas.
	bob.receive()
	bob.mustSet(0, 1, "7")
	if r := bob.send(); r.Sent != 1 {
		t.Fatalf("Bob : %+v", r)
	}
	rt, since := remoteTable(t, bob)
	var skip []string
	var raw string
	must(t, bob.d.DB().Read().QueryRow(`SELECT value FROM _lattis_info WHERE key = 'sync_skip'`).Scan(&raw))
	must(t, json.Unmarshal([]byte(raw), &skip))
	count := func(skip []string) int {
		n := 0
		must(t, p.env.As(p.bobRole).Delta(ctx, func(r workbook.Reader) error {
			return r.Changes(ctx, p.workbookID, rt, since, skip, func(workbook.Change) error { n++; return nil })
		}))
		return n
	}
	if len(skip) != 1 || count(nil) != 1 || count(skip) != 0 {
		t.Fatalf("échos : skip %v, sans skip %d, avec skip %d", skip, count(nil), count(skip))
	}
	if r := bob.receive(); r.Changed() {
		t.Fatalf("réception de son écho : %+v", r)
	}

	// Ligne écrite par Alice puis par Bob avant qu'il ne l'ait reçue : relue
	// à l'envoi (elle ne reviendrait plus par changes_since).
	alice.receive()
	alice.mustSet(1, 1, "11")
	alice.send()
	bob.mustSet(1, 2, "8")
	if r := bob.send(); r.Sent != 1 {
		t.Fatalf("Bob : %+v", r)
	}
	eq(t, "Bob après envoi", bob.col("Quantité"), []string{"7", "11", "1"})

	// Auteur de sa propre ligne, sans écho.
	bob.mustSet(3, 0, "Tisane")
	bob.send()
	var author string
	must(t, bob.d.DB().Read().QueryRow(`SELECT coalesce(lattis_author, '') FROM _lattis_row_ids WHERE lattis_id = ?`, rowID(t, bob.d, 3)).Scan(&author))
	if author != p.bobRole {
		t.Fatalf("auteur : %q", author)
	}

	// Filtre d'écriture (restriction) : seule l'écriture refusée est
	// défaite, le reste de l'étape passe (point de sauvegarde).
	pgAs(t, p, p.aliceRole, `SELECT lattis.add_restriction($1::uuid, $2::uuid, 'all', '{}', $3, 'essai')`, p.workbookID, rt.ID, `"Produit" <> 'Interdit'`)
	_, err := bob.d.Paste(ctx, 1, 1, 0, [][]string{{"Interdit"}, {"Thé vert"}})
	must(t, err)
	r := bob.send()
	if r.Rejected != 1 || len(r.Notices) == 0 {
		t.Fatalf("filtre d'écriture : %+v", r)
	}
	eq(t, "Bob", bob.col("Produit"), []string{"Café", "Thé vert", "Chocolat", "Tisane"})
	_, err = bob.d.Paste(ctx, 1, 5, 0, [][]string{{"Interdit"}, {"Lait"}})
	must(t, err)
	if r := bob.send(); r.Rejected < 1 {
		t.Fatalf("lignes ajoutées : %+v", r)
	}
	eq(t, "Bob lignes", bob.col("Produit"), []string{"Café", "Thé vert", "Chocolat", "Tisane", "Lait"})
	alice.receive()
	eq(t, "Alice", alice.col("Produit"), []string{"Café", "Thé vert", "Chocolat", "Tisane", "Lait"})

	// Point de reprise plus couvert (pierres tombales purgées) : Bob relit
	// le tableau en entier.
	must(t, alice.d.DeleteRow(ctx, "Ventes", 2)) // Chocolat
	alice.send()
	p.env.Exec(t, `UPDATE lattis.sync_horizon SET purged_xid = (SELECT max(xid) FROM lattis.tombstones)`)
	p.env.Exec(t, `DELETE FROM lattis.tombstones`)
	r = bob.receive()
	if r.Reloaded != 1 {
		t.Fatalf("réception : %+v", r)
	}
	eq(t, "Bob rechargé", bob.col("Produit"), []string{"Café", "Thé vert", "Tisane", "Lait"})
	if r := bob.receive(); r.Reloaded != 0 || r.Changed() {
		t.Fatalf("seconde réception : %+v", r)
	}

	// Ajouts simultanés en fin de tableau : clés distinctes, même ordre.
	alice.mustSet(4, 0, "Sucre")
	bob.mustSet(4, 0, "Miel")
	alice.send()
	bob.send()
	alice.receive()
	bob.receive()
	eq(t, "même ordre", bob.col("Produit"), alice.col("Produit"))
	tbl := strconv.FormatInt(tableID(t, alice.d), 10)
	var n, distinct int
	must(t, alice.d.DB().Read().QueryRow(`SELECT count(*), count(DISTINCT _pos) FROM t_`+tbl).Scan(&n, &distinct))
	if n != 6 || n != distinct {
		t.Fatalf("%d lignes, %d clés", n, distinct)
	}
}
