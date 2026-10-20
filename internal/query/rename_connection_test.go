package query

import (
	"errors"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/msg"
)

// Renommer une connexion : les requêtes qui la citent (source, fusion)
// suivent, en une étape annulable ; un nom déjà pris est refusé.
func TestRenameConnection(t *testing.T) {
	d := newDoc(t)
	mk := func(name string) int64 {
		def, err := EncodeConnection(Connection{Name: name, Config: DBConfig{Host: "db", Database: "ventes", User: "louis"}})
		if err != nil {
			t.Fatal(err)
		}
		id, err := d.SaveConnection(ctx, def)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	prod, _ := mk("Prod"), mk("Recette")
	save(t, d, Query{Name: "PG", Source: Source{Kind: SourcePostgres, Connection: "prod", SQL: "SELECT 1"}})
	save(t, d, Query{Name: "Autre", Source: Source{Kind: SourcePostgres, Connection: "Recette", SQL: "SELECT 2"},
		Steps: []Step{{Op: OpAppend, Other: &Source{Kind: SourcePostgres, Connection: "Prod", SQL: "SELECT 3"}}}})
	conns := func() (src, other string) {
		t.Helper()
		pg, err := d.QueryByName(ctx, "PG")
		if err != nil {
			t.Fatal(err)
		}
		q, err := Decode(pg)
		if err != nil {
			t.Fatal(err)
		}
		a, err := d.QueryByName(ctx, "Autre")
		if err != nil {
			t.Fatal(err)
		}
		qa, err := Decode(a)
		if err != nil {
			t.Fatal(err)
		}
		if qa.Source.Connection != "Recette" || len(qa.Steps) != 1 || qa.Steps[0].Other == nil || qa.Steps[0].Other.SQL != "SELECT 3" {
			t.Fatalf("requête « Autre » abîmée : %+v", qa)
		}
		return q.Source.Connection, qa.Steps[0].Other.Connection
	}

	if err := d.RenameConnection(ctx, prod, "  recette "); !errors.Is(err, canvas.ErrQuery) || msgID(err) != "query.connectionExists" {
		t.Fatalf("nom pris : %v", err)
	}
	if err := d.RenameConnection(ctx, prod, " "); err == nil {
		t.Fatal("nom vide accepté")
	}
	if err := d.RenameConnection(ctx, prod, " Production "); err != nil {
		t.Fatal(err)
	}
	if src, other := conns(); src != "Production" || other != "Production" {
		t.Fatalf("requêtes après renommage : %q %q", src, other)
	}
	cs, err := d.Connections(ctx)
	if err != nil || len(cs) != 2 || cs[0].Name != "Production" {
		t.Fatalf("connexions : %+v %v", cs, err)
	}
	// Une seule étape annulée : connexion et requêtes.
	if _, err := d.Bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if src, other := conns(); src != "prod" || other != "Prod" {
		t.Fatalf("annulation : %q %q", src, other)
	}
	if cs, _ := d.Connections(ctx); cs[0].Name != "Prod" {
		t.Fatalf("annulation (connexion) : %+v", cs)
	}
	// Renommer par la boîte de la connexion : les requêtes suivent aussi.
	cs, _ = d.Connections(ctx)
	cs[0].Name = "Prod2"
	if _, err := d.SaveConnection(ctx, cs[0]); err != nil {
		t.Fatal(err)
	}
	if src, other := conns(); src != "Prod2" || other != "Prod2" {
		t.Fatalf("modifier la connexion : %q %q", src, other)
	}
}

func msgID(err error) msg.ID {
	var m *msg.Error
	if errors.As(err, &m) {
		return m.ID
	}
	return ""
}
