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

// Page web sans scripting (version web sans web_scripting, démo publique) :
// lecture seule des données.
func TestPageReadCall(t *testing.T) {
	h := sample(t)
	ctx := context.Background()
	if _, err := h.d.AddFavorite(ctx, "Grosses ventes", `SELECT Client, Montant FROM Commandes WHERE Montant > ? ORDER BY Montant DESC`); err != nil {
		t.Fatal(err)
	}
	if _, err := h.d.AddFavorite(ctx, "Remise à zéro", `UPDATE Commandes SET Montant = 0`); err != nil {
		t.Fatal(err)
	}
	read := func(name string, args map[string]any) (string, error) {
		res, err := PageReadCall(ctx, h, name, args)
		return asJSON(res), err
	}

	// SQL de lecture, avec paramètres.
	if got, err := read("db.query", map[string]any{"sql": `SELECT sum(Montant) FROM Commandes WHERE Montant > ?`, "params": []any{float64(50)}}); err != nil || !strings.Contains(got, `"rows":[[200.5]]`) {
		t.Fatalf("db.query %s %v", got, err)
	}
	if got, err := read("db.query", map[string]any{"sql": "-- total\nWITH t AS (SELECT Montant FROM Commandes) SELECT count(*) FROM t"}); err != nil || !strings.Contains(got, `"rows":[[3]]`) {
		t.Fatalf("WITH %s %v", got, err)
	}
	// Requête favorite par son nom (la casse ne compte pas), et la liste.
	if got, err := read("favorites.run", map[string]any{"name": "grosses ventes", "params": []any{float64(50)}}); err != nil || !strings.Contains(got, `"columns":["Client","Montant"]`) || !strings.Contains(got, `["Durand",120]`) {
		t.Fatalf("favorites.run %s %v", got, err)
	}
	if got, err := read("favorites.list", nil); err != nil || !strings.Contains(got, `"name":"Grosses ventes"`) {
		t.Fatalf("favorites.list %s %v", got, err)
	}
	if _, err := read("favorites.run", map[string]any{"name": "Inconnue"}); !msgIs(err, "api.favoriteNotFound") {
		t.Fatalf("favorite inconnue : %v", err)
	}
	// Métadonnées des feuilles et tableaux.
	for _, name := range []string{"sheets.list", "tables.list"} {
		if _, err := read(name, nil); err != nil {
			t.Fatalf("%s : %v", name, err)
		}
	}
	if got, err := read("tables.rows", map[string]any{"table": "Commandes"}); err != nil || !strings.Contains(got, "Durand") {
		t.Fatalf("tables.rows %s %v", got, err)
	}

	// Rien ne s'écrit : écritures, structure, ATTACH, PRAGMA, et favorite qui modifie.
	for _, q := range []string{
		`UPDATE Commandes SET Montant = 0`,
		`DELETE FROM Commandes`,
		`INSERT INTO Commandes (Client) VALUES ('x')`,
		`DROP TABLE Commandes`,
		`CREATE TABLE x (a)`,
		`ATTACH DATABASE 'x.db' AS x`,
		`PRAGMA query_only = 0`,
		`/* commentaire */ PRAGMA writable_schema = 1`,
		`WITH x AS (SELECT 1) DELETE FROM Commandes`,
		`SELECT 1; DELETE FROM Commandes`,
	} {
		if _, err := read("db.query", map[string]any{"sql": q}); err == nil {
			t.Errorf("accepté : %s", q)
		}
	}
	if _, err := read("favorites.run", map[string]any{"name": "Remise à zéro"}); !msgIs(err, "api.sqlReadOnly") {
		t.Fatalf("favorite d'écriture : %v", err)
	}
	if got := asJSON(call(t, h, "db.query", map[string]any{"sql": `SELECT count(*), sum(Montant) FROM Commandes`})); !strings.Contains(got, `[[3,240.5]]`) {
		t.Fatalf("classeur modifié : %s", got)
	}

	// Commandes des pages hors de la lecture : refusées, avec leur nom.
	for _, name := range []string{"cells.set", "db.execute", "tables.insert_row", "buttons.run", "controls.fire",
		"http.get", "ui.alert", "ui.prompt", "doc.state", "queries.list", "queries.refresh"} {
		_, err := read(name, map[string]any{"row": float64(1), "col": float64(1), "value": "x", "sql": "UPDATE Commandes SET Montant = 0"})
		var me *msg.Error
		if !errors.Is(err, ErrPageNeedsScripting) || !errors.As(err, &me) || me.Args["command"] != name {
			t.Fatalf("%s : %v", name, err)
		}
	}
	// Commandes qu'aucune page n'a : même refus qu'avec le scripting.
	if _, err := read("scripts.run", nil); err == nil || !strings.Contains(err.Error(), "non disponible") {
		t.Fatalf("scripts.run : %v", err)
	}
	if _, err := read("rien.du.tout", nil); !errors.Is(err, ErrUnknown) {
		t.Fatalf("commande inconnue : %v", err)
	}
	// La lecture est une partie des commandes des pages.
	for name := range pageRead {
		if !PageCommand(name) {
			t.Errorf("%s : lecture sans être une commande des pages", name)
		}
	}
	if len(h.dialogs) != 0 {
		t.Fatalf("une page en lecture seule a posé une question : %+v", h.dialogs)
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
