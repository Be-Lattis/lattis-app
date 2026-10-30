package collab_test

import (
	"context"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/remote/collab"
)

// Partage automatique contre un vrai PostgreSQL, suivi par LISTEN (repli
// d'un serveur sans relais) : le tableau collé par Alice (propriétaire)
// part de lui-même, Bob le reçoit sur « *meta » avec ses lignes, puis les
// saisies d'Alice.
func TestPostgresAutoShareFollowed(t *testing.T) {
	p := pgSetupClients(t)
	pgHas0025(t, p)
	old := collab.Debounce
	collab.Debounce = 20 * time.Millisecond
	t.Cleanup(func() { collab.Debounce = old })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	alice, bob := p.alice, p.bob
	bob.follow(ctx, p.bobServer, time.Hour, nil)
	alice.follow(ctx, p.env.As(p.aliceRole), time.Hour, nil)
	waitOnline(t, bob, alice)

	newLocalTable(t, alice, "Stock", 10)
	alice.e.Kick()
	waitFor(t, "Stock chez Bob", func() bool { return hasTable(bob, "Stock") })
	waitFor(t, "lignes chez Bob", func() bool { return len(values(t, bob, "Stock", "Stock")) == 2 })
	eq(t, "serveur", pgColumns(t, p, "Stock"), []string{"Article:text", "Stock:integer"})
	_, err := alice.d.SetCell(ctx, 1, 11, 1, "130")
	must(t, err)
	alice.e.Kick()
	waitFor(t, "valeur d'Alice chez Bob", func() bool { return values(t, bob, "Stock", "Stock")[0] == "130" })
}
