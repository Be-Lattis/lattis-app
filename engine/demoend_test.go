package engine

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/bridge"
)

// Fin de la démo publique annoncée par lattisd (LATTIS_DEMO_DEADLINE) :
// Platform la donne à l'interface avec l'heure du moteur ; passé l'heure,
// les modifications sont refusées (demo.ended), la lecture et Enregistrer
// (téléchargement) restent permis.
func TestEnginePublicDemoEnd(t *testing.T) {
	t.Cleanup(func() {
		bridge.SetDemoDeadline(time.Time{}, 0)
		bridge.UsePublicDemo(false)
	})
	e, err := New(context.Background(), Options{Dir: filepath.Join(t.TempDir(), "session"), PublicDemo: true})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(e.Handler())
	t.Cleanup(func() {
		srv.Close()
		e.Close()
	})
	c := dial(t, srv)
	type platform struct {
		Demo           bool  `json:"demo"`
		DemoDeadline   int64 `json:"demoDeadline"`
		DemoWarnBefore int64 `json:"demoWarnBefore"`
		Now            int64 `json:"now"`
	}

	// Sans fin annoncée (lattisd plus ancien) : rien de particulier.
	var p platform
	json.Unmarshal(c.mustCall("DocumentService.Platform"), &p)
	if !p.Demo || p.DemoDeadline != 0 || p.DemoWarnBefore != 0 || p.Now != 0 {
		t.Fatalf("Platform sans fin = %+v", p)
	}
	c.mustCall("DocumentService.New")
	c.mustCall("DocumentService.AddDoc", 0)

	// Fin dans une heure : annoncée, rien n'est refusé.
	end := time.Now().Add(time.Hour).Truncate(time.Second)
	bridge.SetDemoDeadline(end, 2*time.Minute)
	p = platform{}
	json.Unmarshal(c.mustCall("DocumentService.Platform"), &p)
	if p.DemoDeadline != end.UnixMilli() || p.DemoWarnBefore != 120000 || p.Now == 0 || p.Now > end.UnixMilli() {
		t.Fatalf("Platform avec fin = %+v", p)
	}
	c.mustCall("DocumentService.AddDoc", 0)

	// Fin passée : modifications refusées, lecture et téléchargement permis.
	bridge.SetDemoDeadline(time.Now().Add(-time.Second), 2*time.Minute)
	for _, m := range []struct {
		name   string
		params []any
	}{
		{"DocumentService.AddDoc", []any{0}},
		{"DocumentService.SetZoom", []any{1, 120}},
		{"DocumentService.Undo", nil},
		{"DocumentService.New", nil},
		{"GridService.SetCell", []any{1, 1, "x"}},
	} {
		_, err := c.call(m.name, m.params...)
		if err == nil || err.Data == nil {
			t.Fatalf("%s permis après la fin de la démo (%+v)", m.name, err)
		}
		cause, _ := err.Data.Cause.(map[string]any)
		if cause["id"] != "demo.ended" || !mentionsDemo(err.Message) {
			t.Fatalf("%s : %+v %+v", m.name, err, cause)
		}
	}
	c.mustCall("DocumentService.State")
	c.mustCall("DocumentService.Platform")
	var st struct {
		Dirty bool `json:"dirty"`
	}
	json.Unmarshal(c.mustCall("DocumentService.Save", false), &st)
	var dl Download
	json.Unmarshal(c.event(DownloadEvent).Data.(json.RawMessage), &dl)
	if !strings.HasSuffix(dl.Name, ".lattis") || st.Dirty {
		t.Fatalf("Save après la fin : %+v, %+v", dl, st)
	}
}
