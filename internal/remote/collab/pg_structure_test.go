package collab_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Structure d'un tableau partagé contre un vrai PostgreSQL et les
// fonctions du serveur (schema/0025_structure.sql) : ajout, renommage,
// conversion, clé, suppression, refus d'une colonne citée par une
// restriction, annulation, relecture par un autre poste. Ignoré sans la
// migration 0025.

func pgHas0025(t *testing.T, p *pgClients) {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, p.env.Super)
	must(t, err)
	defer c.Close(ctx)
	var ok bool
	must(t, c.QueryRow(ctx, `SELECT to_regprocedure('lattis.rename_column(uuid,uuid,uuid,text)') IS NOT NULL`).Scan(&ok))
	if !ok {
		t.Skip("serveur sans la migration 0025 (opérations de structure)")
	}
}

// pgColumns lit les colonnes (nom:type) de Ventes sur le serveur.
func pgColumns(t *testing.T, p *pgClients, table string) []string {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, p.env.Super)
	must(t, err)
	defer c.Close(ctx)
	var schema string
	must(t, c.QueryRow(ctx, `SELECT schema_name FROM lattis.workbooks WHERE id = $1::uuid`, p.workbookID).Scan(&schema))
	rs, err := c.Query(ctx, `SELECT c.name || ':' || c.type FROM `+pgx.Identifier{schema, "lattis_meta_columns"}.Sanitize()+` c
		JOIN `+pgx.Identifier{schema, "lattis_meta_tables"}.Sanitize()+` t ON t.table_id = c.table_id WHERE t.name = $1 ORDER BY c.position`, table)
	must(t, err)
	defer rs.Close()
	var out []string
	for rs.Next() {
		var s string
		must(t, rs.Scan(&s))
		out = append(out, s)
	}
	must(t, rs.Err())
	return out
}

func TestPostgresStructure(t *testing.T) {
	p := pgSetupClients(t)
	pgHas0025(t, p)
	ctx := context.Background()
	alice, bob := p.alice, p.bob
	tbl := func(c *client) map[int64]bool { return map[int64]bool{tableID(t, c.d): true} }

	must(t, alice.d.InsertColumn(ctx, "Ventes", 1, "Remise"))
	must(t, alice.header(3, "Tarif"))
	must(t, alice.d.SetType(ctx, "Ventes", 2, grid.Decimal))
	must(t, alice.d.SetKey(ctx, "Ventes", 0))
	eq(t, "serveur", pgColumns(t, p, "Ventes"), []string{"Produit:text", "Remise:text", "Quantité:decimal", "Tarif:decimal"})
	_, err := bob.e.ReceiveStructure(ctx, nil, tbl(bob))
	must(t, err)
	eq(t, "Bob", colsOf(bob, "Ventes"), []string{"Produit:text", "Remise:text", "Quantité:decimal", "Tarif:decimal"})
	eq(t, "Bob Tarif", bob.col("Tarif"), []string{"4.5", "2.0", "3.25"})
	if keyOf(bob, "Ventes") != "Produit" {
		t.Fatalf("clé chez Bob : %q", keyOf(bob, "Ventes"))
	}

	// Une valeur saisie dans la nouvelle colonne passe.
	alice.mustSet(0, 1, "5 %")
	if r := alice.send(); r.Sent != 1 || r.Rejected != 0 {
		t.Fatalf("envoi : %+v", r)
	}
	bob.receive()
	eq(t, "Bob Remise", bob.col("Remise"), []string{"5 %", "", ""})

	// Colonne citée par une restriction : refus du serveur, en français.
	tid, _ := remoteID(t, alice, "Produit")
	_, err = p.env.As(p.aliceRole).AddRestriction(ctx, p.workbookID, workbook.RestrictionRule{TableID: tid, Command: "select", Predicate: `"Remise" IS NOT NULL OR true`, Label: "Remises"})
	must(t, err)
	err = alice.d.DeleteColumn(ctx, "Ventes", 1)
	var re *collab.RefusedError
	if !errors.As(err, &re) || !strings.Contains(err.Error(), "la colonne « Remise » est citée par la restriction « Remises »") {
		t.Fatalf("suppression refusée : %v", err)
	}
	eq(t, "serveur inchangé", pgColumns(t, p, "Ventes"), []string{"Produit:text", "Remise:text", "Quantité:decimal", "Tarif:decimal"})

	// Annulation : la clé, la conversion et le renommage sont défaits sur
	// le serveur.
	_, err = alice.d.Bus.Undo(ctx) // saisie
	must(t, err)
	_, err = alice.d.Bus.Undo(ctx) // clé
	must(t, err)
	_, err = alice.d.Bus.Undo(ctx) // conversion
	must(t, err)
	_, err = alice.d.Bus.Undo(ctx) // renommage
	must(t, err)
	eq(t, "serveur après annulation", pgColumns(t, p, "Ventes"), []string{"Produit:text", "Remise:text", "Quantité:integer", "Prix:decimal"})
	_, err = bob.e.ReceiveStructure(ctx, nil, tbl(bob))
	must(t, err)
	eq(t, "Bob après annulation", colsOf(bob, "Ventes"), []string{"Produit:text", "Remise:text", "Quantité:integer", "Prix:decimal"})

	// Éditeur : refusé en local.
	if err := bob.d.InsertColumn(ctx, "Ventes", 1, "X"); !errors.Is(err, workbook.ErrDesign) {
		t.Fatalf("éditeur : %v", err)
	}
}
