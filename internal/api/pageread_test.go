package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/query"
)

// Requêtes favorites lues par une page web (lattis.query) : par leur nom,
// en lecture seule.
func TestFavoritesRun(t *testing.T) {
	h := sample(t)
	ctx := context.Background()
	if _, err := h.d.AddFavorite(ctx, "Grosses ventes", `SELECT Client, Montant FROM Commandes WHERE Montant > ? ORDER BY Montant DESC`); err != nil {
		t.Fatal(err)
	}
	if _, err := h.d.AddFavorite(ctx, "Remise à zéro", `UPDATE Commandes SET Montant = 0`); err != nil {
		t.Fatal(err)
	}
	read := func(name string, args map[string]any) (string, error) {
		res, err := Invoke(ctx, h, name, args)
		return asJSON(res), err
	}
	// Par son nom (la casse ne compte pas), et la liste.
	if got, err := read("favorites.run", map[string]any{"name": "grosses ventes", "params": []any{float64(50)}}); err != nil || !strings.Contains(got, `"columns":["Client","Montant"]`) || !strings.Contains(got, `["Durand",120]`) {
		t.Fatalf("favorites.run %s %v", got, err)
	}
	if got, err := read("favorites.list", nil); err != nil || !strings.Contains(got, `"name":"Grosses ventes"`) {
		t.Fatalf("favorites.list %s %v", got, err)
	}
	if _, err := read("favorites.run", map[string]any{"name": "Inconnue"}); !msgIs(err, "api.favoriteNotFound") {
		t.Fatalf("favorite inconnue : %v", err)
	}
	// Une favorite qui modifie est refusée ; rien ne change.
	if _, err := read("favorites.run", map[string]any{"name": "Remise à zéro"}); !msgIs(err, "api.sqlReadOnly") {
		t.Fatalf("favorite d'écriture : %v", err)
	}
	if got := asJSON(call(t, h, "db.query", map[string]any{"sql": `SELECT count(*), sum(Montant) FROM Commandes`})); !strings.Contains(got, `[[3,240.5]]`) {
		t.Fatalf("classeur modifié : %s", got)
	}
}

func TestReadOnlySQL(t *testing.T) {
	for _, q := range []string{"SELECT 1", "  select 1", "-- c\nWITH a AS (SELECT 1) SELECT * FROM a", "/* x */ VALUES (1)"} {
		if err := ReadOnlySQL(q); err != nil {
			t.Errorf("%q : %v", q, err)
		}
	}
	for _, q := range []string{"", "  ", "UPDATE t SET a = 1", "PRAGMA query_only = 0", "ATTACH 'x' AS y", "REPLACE INTO t VALUES (1)"} {
		if err := ReadOnlySQL(q); err == nil {
			t.Errorf("%q accepté", q)
		}
	}
}

// guardedHost est un hôte dont le Guard refuse tout réseau (démo publique).
type guardedHost struct {
	*fakeHost
	g *query.Guard
}

func (h guardedHost) QueryGuard() *query.Guard { return h.g }

// Démo publique : la tuile est refusée sans question ni requête, avec un
// message que la carte affiche.
func TestMapTileOffline(t *testing.T) {
	old := httpTransport
	t.Cleanup(func() { httpTransport = old })
	httpTransport = roundTrip(func(r *http.Request) (*http.Response, error) {
		t.Errorf("requête %s", r.URL)
		return nil, errors.New("pas de réseau")
	})
	h := guardedHost{newHost(t), &query.Guard{Offline: true}}
	h.answer = map[string]any{}
	_, err := Invoke(context.Background(), h, "charts.tile", map[string]any{"z": 1, "x": 0, "y": 0})
	if !errors.Is(err, ErrTileOffline) || len(h.dialogs) != 0 {
		t.Fatalf("démo : %v, questions %+v", err, h.dialogs)
	}
	// Avec réseau : la question est celle des cartes, dans la langue de la session.
	h2 := guardedHost{newHost(t), &query.Guard{}}
	Invoke(context.Background(), h2, "charts.tile", map[string]any{"z": 1, "x": 0, "y": 0})
	if len(h2.dialogs) != 1 || h2.dialogs[0].Title != msg.Format(msg.Current(), "api.tileAskTitle", nil) || !strings.Contains(h2.dialogs[0].Message, "OpenStreetMap") {
		t.Fatalf("question %+v", h2.dialogs)
	}
}

// msgIs dit si err vient du message id.
func msgIs(err error, id msg.ID) bool {
	var me *msg.Error
	return errors.As(err, &me) && me.ID == id
}
