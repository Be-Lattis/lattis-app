package engine

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/bridge"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// Session web : le classeur revient d'un brouillon dans un moteur neuf, par
// l'écran de récupération ; la démo publique n'en écrit jamais.
func TestEngineDrafts(t *testing.T) {
	store := workbooktest.NewDrafts()
	e, err := New(context.Background(), Options{Dir: filepath.Join(t.TempDir(), "a"), Drafts: store})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(e.Handler())
	c := dial(t, srv)
	c.mustCall("DocumentService.New")
	c.mustCall("DocumentService.SetCell", 1, 0, 0, "gardé")
	var st bridge.DraftStatus
	for deadline := time.Now().Add(20 * time.Second); st.State != "saved"; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("état = %+v", st)
		}
		json.Unmarshal(c.mustCall("DocumentService.DraftStatus"), &st)
	}
	srv.Close()
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}

	e2, err := New(context.Background(), Options{Dir: filepath.Join(t.TempDir(), "b"), Drafts: store})
	if err != nil {
		t.Fatal(err)
	}
	srv2 := httptest.NewServer(e2.Handler())
	t.Cleanup(func() {
		srv2.Close()
		e2.Close()
	})
	c2 := dial(t, srv2)
	var list []archive.Orphan
	json.Unmarshal(c2.mustCall("DocumentService.Orphans"), &list)
	dir := ""
	for _, o := range list {
		if len(o.Dir) > 6 && o.Dir[:6] == "draft:" {
			dir = o.Dir
		}
	}
	if dir == "" {
		t.Fatalf("proposés = %+v", list)
	}
	var state bridge.DocState
	json.Unmarshal(c2.mustCall("DocumentService.Recover", dir), &state)
	if !state.Open || !state.Dirty {
		t.Fatalf("restauré : %+v", state)
	}
}

func TestEngineNoDraftInPublicDemo(t *testing.T) {
	t.Cleanup(func() { bridge.UsePublicDemo(false) })
	store := workbooktest.NewDrafts()
	e, err := New(context.Background(), Options{Dir: filepath.Join(t.TempDir(), "s"), PublicDemo: true, Drafts: store})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(e.Handler())
	t.Cleanup(func() {
		srv.Close()
		e.Close()
	})
	c := dial(t, srv)
	c.mustCall("DocumentService.New")
	c.mustCall("DocumentService.SetCell", 1, 0, 0, "a")
	time.Sleep(500 * time.Millisecond)
	if tries, _ := store.Counts(); tries != 0 {
		t.Fatalf("démo publique : %d envois", tries)
	}
	var st bridge.DraftStatus
	json.Unmarshal(c.mustCall("DocumentService.DraftStatus"), &st)
	if st.Active || st.State != "" {
		t.Fatalf("état = %+v", st)
	}
}
