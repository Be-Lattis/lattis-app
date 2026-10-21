package bridge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/msg"
)

// useSettings pose une configuration (défauts et couches données) le temps
// d'un essai.
func useSettings(t *testing.T, layers ...config.Source) {
	t.Helper()
	old := config.Current()
	src := append([]config.Source{&config.TreeSource{N: "défaut", R: config.RankDefault, T: config.Defaults()}}, layers...)
	config.SetCurrent(config.Resolve(context.Background(), config.Options{}, src...))
	t.Cleanup(func() { config.SetCurrent(old) })
}

func htmlTabs(r config.Rank, trusted, on bool) config.Source {
	return &config.TreeSource{N: r.String(), R: r, Trust: trusted, T: config.Tree{"features": config.Tree{"htmlTabs": on}}}
}

// Version web sans scripting (et démo publique) : une page web lit le
// classeur (SQL en lecture seule, requêtes favorites), rien d'autre.
func TestPageReadWithoutScripting(t *testing.T) {
	ctx := context.Background()
	useSettings(t, htmlTabs(config.RankUser, false, true))
	s := &DocumentService{}
	UseHost(webLike{}, nil, s)
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	// features.htmlTabs suffit pour ajouter une page, sans scripting.
	id, err := s.AddPage(ctx, 0)
	if err != nil || id == 0 {
		t.Fatalf("AddPage sans scripting : %d, %v", id, err)
	}
	d, _ := s.current()
	if _, err := d.AddFavorite(ctx, "Feuilles", `SELECT count(*) AS n FROM sqlite_schema WHERE type = 'table'`); err != nil {
		t.Fatal(err)
	}
	res, err := s.PageCall(ctx, "db.query", map[string]any{"sql": "SELECT 6 * 7 AS r"})
	if err != nil || !strings.Contains(asText(res), "42") {
		t.Fatalf("db.query : %v, %v", res, err)
	}
	if _, err := s.PageCall(ctx, "favorites.run", map[string]any{"name": "Feuilles"}); err != nil {
		t.Fatalf("favorites.run : %v", err)
	}
	if _, err := s.PageCall(ctx, "sheets.list", nil); err != nil {
		t.Fatalf("sheets.list : %v", err)
	}
	for _, c := range []struct {
		name string
		args map[string]any
	}{
		{"cells.set", map[string]any{"row": 1.0, "col": 1.0, "value": 1.0}},
		{"db.execute", map[string]any{"sql": "DELETE FROM x"}},
		{"http.get", map[string]any{"url": "https://exemple.fr/"}},
		{"buttons.run", nil},
	} {
		if _, err := s.PageCall(ctx, c.name, c.args); !errors.Is(err, api.ErrPageNeedsScripting) {
			t.Fatalf("%s sans scripting : %v", c.name, err)
		}
	}
	if _, err := s.PageCall(ctx, "db.query", map[string]any{"sql": "PRAGMA query_only = 0"}); err == nil {
		t.Fatal("PRAGMA accepté")
	}
	// Une page reçue (version jamais vue sur ce poste) attend toujours
	// l'accord de l'utilisateur, scripting ou non ; une page écrite ici non.
	if s.PageAllowed(ctx, "<h1>reçue</h1><script>lattis.sql('SELECT 1')</script>") {
		t.Fatal("page reçue autorisée d'office")
	}
	if _, err := s.SavePage(ctx, id, "<h1>écrite ici</h1>"); err != nil {
		t.Fatal(err)
	}
	if !s.PageAllowed(ctx, "<h1>écrite ici</h1>") || !doc.PageAllowed("<h1>écrite ici</h1>") {
		t.Fatal("page écrite ici non autorisée")
	}
}

// Pages web coupées et verrouillées par l'organisation : la page ne lit plus
// rien, et l'interface le dit (page.disabledByOrg).
func TestPageCallDisabledByOrg(t *testing.T) {
	ctx := context.Background()
	useSettings(t, htmlTabs(config.RankPolicy, true, false))
	if !config.Current().Locked("features.htmlTabs") {
		t.Skip("couche de stratégie non verrouillée")
	}
	s := &DocumentService{}
	UseHost(NoHost{}, nil, s)
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	_, err := s.PageCall(ctx, "db.query", map[string]any{"sql": "SELECT 1"})
	var me *msg.Error
	if !errors.As(err, &me) || me.ID != "page.disabledByOrg" {
		t.Fatalf("pages coupées par l'organisation : %v", err)
	}
	// Coupées par l'utilisateur seulement : les pages déjà posées lisent toujours.
	useSettings(t, htmlTabs(config.RankUser, false, false))
	if _, err := s.PageCall(ctx, "db.query", map[string]any{"sql": "SELECT 1"}); err != nil {
		t.Fatalf("pages coupées par l'utilisateur : %v", err)
	}
}

func asText(v any) string { return fmt.Sprint(v) }
