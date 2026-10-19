package bridge

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/api"
)

// Évènements du classeur déclenchés par les actions de l'interface : chaque
// écouteur ajoute une ligne à la cellule A1 de la feuille Journal.
const journal = `
local function noter(t)
  local avant = cells.get{sheet = "Journal", row = 1, col = 1}.value
  cells.set{sheet = "Journal", row = 1, col = 1, value = avant and (avant .. "\n" .. t) or t}
end
function tout(e)
  local t = e.event
  for _, k in ipairs({"sheet", "address", "old", "value", "source", "previous", "table", "line", "path"}) do
    if e[k] ~= nil then t = t .. " " .. k .. "=" .. tostring(e[k]) end
  end
  noter(t)
end
function non(e)
  noter(e.event .. " refusé")
  return false
end
`

func eventsService(t *testing.T) (*DocumentService, context.Context) {
	t.Helper()
	ctx := context.Background()
	s := &DocumentService{}
	if _, err := s.New(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.CloseDocument(ctx) })
	for name, args := range map[string]map[string]any{
		"scripts.save": {"name": "Journal", "source": journal},
		"sheets.add":   {"name": "Journal"},
	} {
		if _, err := s.Call(ctx, name, args); err != nil {
			t.Fatal(err)
		}
	}
	return s, ctx
}

func on(t *testing.T, s *DocumentService, event, fn string) {
	t.Helper()
	if _, err := s.Call(context.Background(), "events.on", map[string]any{"event": event, "handler": fn, "script": "Journal"}); err != nil {
		t.Fatal(err)
	}
}

// lines lit le journal ; wait : attendre qu'il ait n lignes.
func lines(t *testing.T, s *DocumentService, n int) []string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var out []string
		v, err := s.Call(context.Background(), "cells.get", map[string]any{"sheet": "Journal", "row": int64(1), "col": int64(1)})
		if err != nil {
			t.Fatal(err)
		}
		if x := v.(api.CellValue).Value; x != nil {
			out = strings.Split(fmt.Sprint(x), "\n")
		}
		if len(out) >= n || time.Now().After(deadline) {
			return out
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func want(t *testing.T, s *DocumentService, exp ...string) {
	t.Helper()
	got := lines(t, s, len(exp))
	// Laisser passer un évènement de trop éventuel avant de comparer.
	time.Sleep(50 * time.Millisecond)
	got = lines(t, s, len(exp))
	if strings.Join(got, "\n") != strings.Join(exp, "\n") {
		t.Fatalf("journal :\n%s\nattendu :\n%s", strings.Join(got, "\n"), strings.Join(exp, "\n"))
	}
}

func TestEventChangeFromUI(t *testing.T) {
	s, ctx := eventsService(t)
	on(t, s, "change", "tout")
	if _, err := s.SetCell(ctx, 1, 1, 1, "un"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetCell(ctx, 1, 1, 1, "deux"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Paste(ctx, 1, 4, 1, [][]string{{"Nom", "Âge"}, {"Léa", "31"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClearCells(ctx, 1, 1, 1, 1, 1); err != nil {
		t.Fatal(err)
	}
	want(t, s,
		"change sheet=Feuille1 address=B2 value=un source=edit",
		"change sheet=Feuille1 address=B2 old=un value=deux source=edit",
		"change sheet=Feuille1 address=B5:C6 source=paste table=Feuille1",
		"change sheet=Feuille1 address=B2 old=deux source=clear",
	)
	// Les écritures de l'écouteur ne relancent pas change, et forment leur
	// propre étape d'annulation.
	st, _ := s.State(ctx)
	if st.Undo != "Évènement « change »" {
		t.Fatalf("étape %q", st.Undo)
	}
}

func TestEventTableRows(t *testing.T) {
	s, ctx := eventsService(t)
	if _, err := s.Paste(ctx, 1, 0, 0, [][]string{{"Nom", "Âge"}, {"Léa", "31"}, {"Tom", "40"}}); err != nil {
		t.Fatal(err)
	}
	sh, err := s.Call(ctx, "sheets.get", map[string]any{"sheet": "Feuille1"})
	if err != nil {
		t.Fatal(err)
	}
	tb := sh.(api.SheetView).Tables[0].Name
	on(t, s, "change", "tout")
	if _, err := s.DeleteRow(ctx, tb, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InsertRow(ctx, tb, 1); err != nil {
		t.Fatal(err)
	}
	want(t, s,
		"change sheet=Feuille1 address=A2:B2 source=delete_row table="+tb+" line=1",
		"change sheet=Feuille1 address=A3:B3 source=insert_row table="+tb+" line=2",
	)
}

func TestEventSheetsAndRecalc(t *testing.T) {
	s, ctx := eventsService(t)
	on(t, s, "sheet_add", "tout")
	on(t, s, "sheet_delete", "tout")
	on(t, s, "recalc", "tout")
	id, err := s.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteSheet(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Recalc(ctx); err != nil {
		t.Fatal(err)
	}
	want(t, s, "sheet_add sheet=Feuille3", "sheet_delete sheet=Feuille3", "recalc")
}

func TestEventSelectionAndSheet(t *testing.T) {
	s, ctx := eventsService(t)
	on(t, s, "selection", "tout")
	on(t, s, "sheet", "tout")
	sel := func(sheet int64, r0, c0, r1, c1 int) api.Selection {
		return api.Selection{Sheet: sheet, R0: r0, C0: c0, R1: r1, C1: c1, View: "grid"}
	}
	s.SetSelection(sel(1, 0, 0, 0, 0)) // première sélection rapportée : rien
	// Glisser : seule la dernière sélection compte, après une pause.
	s.SetSelection(sel(1, 0, 0, 1, 1))
	s.SetSelection(sel(1, 0, 0, 2, 1))
	s.SetSelection(sel(1, 0, 0, 3, 2))
	want(t, s, "selection sheet=Feuille1 address=A1:C4")
	// Changer de vue n'est pas changer de sélection.
	s.SetSelection(api.Selection{Sheet: 1, R1: 3, C1: 2, View: "sql"})
	// Changer d'onglet : sheet, pas selection.
	s.SetSelection(sel(2, 0, 0, 0, 0))
	s.SetSelection(sel(2, 4, 4, 4, 4))
	time.Sleep(selectionPause + 100*time.Millisecond)
	want(t, s, "selection sheet=Feuille1 address=A1:C4", "sheet sheet=Journal previous=Feuille1")
	// Une sélection posée par un script, rapportée ensuite par l'interface :
	// rien (un écouteur qui sélectionne ne se relance pas).
	time.Sleep(sheetSettle)
	if err := (host{s}).Select(ctx, sel(2, 7, 1, 7, 1)); err != nil {
		t.Fatal(err)
	}
	s.SetSelection(sel(2, 7, 1, 7, 1))
	time.Sleep(selectionPause + 100*time.Millisecond)
	want(t, s, "selection sheet=Feuille1 address=A1:C4", "sheet sheet=Journal previous=Feuille1")
	// Coupés, les évènements ne tournent plus.
	api.SetEventsEnabled(false)
	s.SetSelection(sel(1, 0, 0, 0, 0))
	time.Sleep(50 * time.Millisecond)
	want(t, s, "selection sheet=Feuille1 address=A1:C4", "sheet sheet=Journal previous=Feuille1")
	api.SetEventsEnabled(true)
}

func TestEventOpenCloseSave(t *testing.T) {
	s, ctx := eventsService(t)
	on(t, s, "open", "tout")
	s.WorkbookReady(ctx)
	s.WorkbookReady(ctx) // une fois par classeur
	want(t, s, "open path=")
	if !s.BeforeClose(ctx) {
		t.Fatal("aucun écouteur before_close : fermer")
	}
	on(t, s, "before_close", "non")
	if s.BeforeClose(ctx) {
		t.Fatal("before_close a renvoyé false : ne pas fermer")
	}
	on(t, s, "before_save", "non")
	st, err := s.Save(ctx, false)
	if err != nil || !strings.Contains(st.Notice, "annulé") {
		t.Fatalf("%+v %v", st, err)
	}
	want(t, s, "open path=", "before_close refusé", "before_save refusé")
}
