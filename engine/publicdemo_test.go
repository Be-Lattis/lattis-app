package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/bridge"
	"github.com/lalamefine/lattis/internal/query"
)

// mentionsDemo : message de la démo, dans la langue de la session (fr, en).
func mentionsDemo(s string) bool {
	s = strings.ToLower(s)
	return strings.Contains(s, "démo") || strings.Contains(s, "demo")
}

// Démo publique (lattis-engine --public-demo) : ni scripting, ni session,
// ni connexion sortante ; Enregistrer télécharge ; les récents sont les
// démonstrations ; le répertoire de la session est effacé à la fermeture.
func TestEnginePublicDemo(t *testing.T) {
	t.Cleanup(func() { bridge.UsePublicDemo(false) })
	dir := filepath.Join(t.TempDir(), "session")
	// Scripting et hôtes internes demandés : ignorés en démo publique.
	e, err := New(context.Background(), Options{Dir: dir, PublicDemo: true, Scripting: true, InternalHosts: []string{"example.org"}})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(e.Handler())
	closed := false
	t.Cleanup(func() {
		srv.Close()
		if !closed {
			e.Close()
		}
	})
	c := dial(t, srv)

	var p struct{ Web, Scripting, Demo bool }
	json.Unmarshal(c.mustCall("DocumentService.Platform"), &p)
	if !p.Web || p.Scripting || !p.Demo {
		t.Fatalf("Platform = %+v", p)
	}
	if r := string(c.mustCall("DocumentService.FirstLaunch")); r != "false" {
		t.Fatalf("FirstLaunch = %s", r)
	}
	if r := string(c.mustCall("DocumentService.Orphans")); r != "[]" {
		t.Fatalf("Orphans = %s", r)
	}

	// Récents : les deux démonstrations, rien d'autre.
	var recent []bridge.RecentFile
	json.Unmarshal(c.mustCall("DocumentService.Recent"), &recent)
	if len(recent) != 2 || recent[0].Demo != "sml" || recent[1].Demo != "med" || recent[0].Title != "small" || recent[1].Title != "medium" {
		t.Fatalf("Recent = %+v", recent)
	}

	// Pas de Serveur Lattis (session, connexion).
	var ls struct{ Enabled bool }
	json.Unmarshal(c.mustCall("DocumentService.LattisState"), &ls)
	if ls.Enabled {
		t.Fatal("LattisState : Serveurs Lattis proposés en démo")
	}
	if _, err := c.call("DocumentService.LattisDiscover", "https://lattis.example.org"); err == nil || !mentionsDemo(err.Message) {
		t.Fatalf("LattisDiscover : %+v", err)
	}
	if _, err := c.call("DocumentService.ShareOpen", "https://lattis.example.org", "w1", ""); err == nil {
		t.Fatal("ShareOpen permis en démo")
	}

	// Pas de connexion sortante ; les fichiers envoyés restent lisibles.
	g := e.host.QueryGuard()
	ctx := context.Background()
	for _, h := range []string{"example.org:443", "db.example.org:5432", ""} {
		if err := g.Check(ctx, query.Access{Host: h}); err == nil {
			t.Errorf("Check(%q) permis", h)
		}
	}
	if _, err := g.DialContext(ctx, "tcp", "example.org:443"); err == nil {
		t.Error("DialContext permis")
	}
	if err := g.Check(ctx, query.Access{Path: filepath.Join(dir, "uploads", "x", "a.csv")}); err != nil {
		t.Errorf("fichier envoyé refusé : %v", err)
	}
	if _, err := c.call("DocumentService.RunScript", "x", "print(1)"); err == nil {
		t.Fatal("script permis en démo")
	}

	// Enregistrer : le classeur est téléchargé, à chaque fois.
	c.mustCall("DocumentService.New")
	for i := 0; i < 2; i++ {
		c.mustCall("DocumentService.AddDoc", 0)
		var st struct {
			Dirty  bool   `json:"dirty"`
			Notice string `json:"notice"`
		}
		json.Unmarshal(c.mustCall("DocumentService.Save", false), &st)
		var dl Download
		json.Unmarshal(c.event(DownloadEvent).Data.(json.RawMessage), &dl)
		if !strings.HasSuffix(dl.Name, ".lattis") || st.Dirty || !mentionsDemo(st.Notice) {
			t.Fatalf("Save n° %d : %+v, %+v", i+1, dl, st)
		}
		resp, err := http.Get(srv.URL + dl.URL)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("téléchargement : %v %v", resp, err)
		}
		resp.Body.Close()
	}

	// Démonstrations des récents : celles-là seulement.
	var res struct {
		Sheet int64 `json:"sheet"`
	}
	json.Unmarshal(c.mustCall("DocumentService.OpenPreset", "sml"), &res)
	if res.Sheet == 0 {
		t.Fatalf("OpenPreset = %+v", res)
	}
	if _, err := c.call("DocumentService.OpenPreset", "10Mx100"); err == nil {
		t.Fatal("OpenPreset : preset hors des récents permis")
	}
	// Modifié, non enregistré : rien n'est gardé après la fermeture.
	c.mustCall("DocumentService.AddDoc", 0)

	srv.Close()
	closed = true
	if err := e.Close(); err != nil {
		t.Fatalf("Close : %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		entries, _ := os.ReadDir(dir)
		t.Fatalf("répertoire de la session gardé : %v %v", err, entries)
	}
}
