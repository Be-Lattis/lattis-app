package bridge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/config"
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

// htmlPages : une couche qui règle features.htmlPages.
func htmlPages(r config.Rank, trusted, on bool) config.Source {
	return &config.TreeSource{N: r.String(), R: r, Trust: trusted, T: config.Tree{"features": config.Tree{"htmlPages": on}}}
}

// platformHost : hôte de plateforme p (bureau, web, démo).
type platformHost struct {
	NoHost
	p Platform
}

func (h platformHost) Platform() Platform { return h.p }

var (
	desktop    = Platform{Scripting: true}
	webScripts = Platform{Web: true, Scripting: true}
	demoHost   = Platform{Web: true, Demo: true}
)

func pagesService(t *testing.T, p Platform) *DocumentService {
	t.Helper()
	s := &DocumentService{}
	UseHost(platformHost{p: p}, nil, s)
	if _, err := s.New(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func asText(v any) string { return fmt.Sprint(v) }

// Pages web coupées par défaut, partout : ni ajout, ni appel d'une page,
// ni modification d'une page existante (gardée dans le classeur).
func TestPagesOffByDefault(t *testing.T) {
	ctx := context.Background()
	useSettings(t)
	for _, p := range []Platform{desktop, webScripts, demoHost} {
		s := pagesService(t, p)
		if s.Platform().HTMLPages {
			t.Fatalf("%+v : pages annoncées", p)
		}
		if _, err := s.AddPage(ctx, 0); !errors.Is(err, api.ErrPagesDisabled) {
			t.Fatalf("%+v AddPage : %v", p, err)
		}
		if _, err := s.PageCall(ctx, "db.query", map[string]any{"sql": "SELECT 1"}); !errors.Is(err, api.ErrPagesDisabled) {
			t.Fatalf("%+v PageCall : %v", p, err)
		}
		// Onglet page d'un classeur existant : gardé, pas modifiable ;
		// un Document, si.
		d, _ := s.current()
		id, err := d.AddPage(ctx, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.SavePage(ctx, id, "<h1>x</h1>"); !errors.Is(err, api.ErrPagesDisabled) {
			t.Fatalf("%+v SavePage : %v", p, err)
		}
		doc, err := s.AddDoc(ctx, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.SavePage(ctx, doc, `{"ops":[{"insert":"x\n"}]}`); err != nil {
			t.Fatalf("%+v SavePage d'un document : %v", p, err)
		}
	}
}

// Activées à la main (features.htmlPages) : sur le bureau seulement ; la
// version web et la démo publique les refusent quel que soit le réglage.
func TestPagesOnByConfig(t *testing.T) {
	ctx := context.Background()
	useSettings(t, htmlPages(config.RankUser, false, true))
	s := pagesService(t, desktop)
	if !s.Platform().HTMLPages {
		t.Fatal("pages non annoncées")
	}
	id, err := s.AddPage(ctx, 0)
	if err != nil || id == 0 {
		t.Fatalf("AddPage : %d, %v", id, err)
	}
	if _, err := s.SavePage(ctx, id, "<h1>x</h1>"); err != nil {
		t.Fatalf("SavePage : %v", err)
	}
	res, err := s.PageCall(ctx, "db.query", map[string]any{"sql": "SELECT 6 * 7 AS r"})
	if err != nil || !strings.Contains(asText(res), "42") {
		t.Fatalf("db.query : %v, %v", res, err)
	}
	for _, p := range []Platform{webScripts, demoHost} {
		s := pagesService(t, p)
		if s.Platform().HTMLPages {
			t.Fatalf("%+v : pages annoncées", p)
		}
		if _, err := s.AddPage(ctx, 0); !errors.Is(err, api.ErrPagesDisabled) {
			t.Fatalf("%+v AddPage : %v", p, err)
		}
		if _, err := s.PageCall(ctx, "sheets.list", nil); !errors.Is(err, api.ErrPagesDisabled) {
			t.Fatalf("%+v PageCall : %v", p, err)
		}
	}
}

// La stratégie de l'organisation l'emporte sur le fichier de l'utilisateur.
func TestPagesLockedByPolicy(t *testing.T) {
	ctx := context.Background()
	useSettings(t, htmlPages(config.RankUser, false, true), htmlPages(config.RankPolicy, true, false))
	if !config.Current().Locked(api.HTMLPagesKey) {
		t.Fatal("réglage non verrouillé")
	}
	s := pagesService(t, desktop)
	if _, err := s.AddPage(ctx, 0); !errors.Is(err, api.ErrPagesDisabled) {
		t.Fatalf("AddPage : %v", err)
	}
	// Une stratégie peut aussi les activer pour toute l'organisation.
	useSettings(t, htmlPages(config.RankPolicy, true, true))
	s = pagesService(t, desktop)
	if _, err := s.AddPage(ctx, 0); err != nil {
		t.Fatalf("AddPage par stratégie : %v", err)
	}
}
