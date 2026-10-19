package api

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/doc"
)

const eventsScript = `
function saisie(e)
  print("change", e.event, e.sheet, e.address, e.row, e.col, tostring(e.old), tostring(e.value), e.source)
  cells.set(1, 5, "vu " .. e.address) -- ne relance pas change
end
function garde(e)
  print("garde", e.path, tostring(e.as))
  return false
end
function laisse(e)
  return nil
end
function casse(e)
  error("cassé")
end
function boucle(e)
  while true do end
end
function onglet(e)
  print("onglet", e.sheet, e.previous)
end
`

func eventsHost(t *testing.T) *fakeHost {
	t.Helper()
	ResetEvents()
	t.Cleanup(ResetEvents)
	h := newHost(t)
	call(t, h, "scripts.save", map[string]any{"name": "Évènements", "source": eventsScript})
	return h
}

func TestEventsOnOffList(t *testing.T) {
	h := eventsHost(t)
	// Depuis le script lui-même : le script en cours est retenu ; la
	// fonction peut être passée elle-même.
	res, err := RunScript(h, "Évènements", eventsScript+`
assert(events.on("change", saisie) == true)
assert(events.on("change", "saisie") == false) -- déjà abonnée
events.on("before_save", "garde")
events.on("sheet", onglet)
`, "")
	if err != nil || res.Error != "" {
		t.Fatalf("%+v %v", res, err)
	}
	ls := call(t, h, "events.list", nil).([]WorkbookListenerView)
	want := []WorkbookListenerView{{"change", "Évènements", "saisie"}, {"before_save", "Évènements", "garde"}, {"sheet", "Évènements", "onglet"}}
	if asJSON(ls) != asJSON(want) {
		t.Fatalf("écouteurs %s", asJSON(ls))
	}
	if got := call(t, h, "events.list", map[string]any{"event": "sheet"}).([]WorkbookListenerView); len(got) != 1 {
		t.Fatalf("filtre : %+v", got)
	}
	// S'abonner s'annule comme une saisie.
	st := call(t, h, "doc.state", nil).(DocState)
	if st.Undo != "Script « Évènements »" {
		t.Fatalf("étape %q", st.Undo)
	}
	if _, err := Invoke(context.Background(), h, "events.on", map[string]any{"event": "clic", "handler": "saisie", "script": "Évènements"}); err == nil || !strings.Contains(err.Error(), "before_close") {
		t.Fatalf("évènement inconnu : %v", err)
	}
	if _, err := Invoke(context.Background(), h, "events.on", map[string]any{"event": "open", "handler": "f", "script": "Nul"}); err == nil || !strings.Contains(err.Error(), "introuvable") {
		t.Fatalf("script inconnu : %v", err)
	}
	if _, err := Invoke(context.Background(), h, "events.on", map[string]any{"event": "open", "handler": "f"}); err == nil {
		t.Fatal("script absent hors script accepté")
	}
	if n := call(t, h, "events.off", map[string]any{"event": "sheet"}); n != 1 {
		t.Fatalf("off : %v", n)
	}
	if n := call(t, h, "events.off", map[string]any{"handler": "rien"}); n != 0 {
		t.Fatalf("off sans effet : %v", n)
	}
	call(t, h, "doc.undo", nil)
	if got := call(t, h, "events.list", nil).([]WorkbookListenerView); len(got) != 3 {
		t.Fatalf("après annulation : %+v", got)
	}
	call(t, h, "events.off", nil)
	if got := call(t, h, "events.list", nil).([]WorkbookListenerView); len(got) != 0 {
		t.Fatalf("tout retiré : %+v", got)
	}
	for _, e := range WorkbookEvents {
		call(t, h, "events.on", map[string]any{"event": strings.ToUpper(e.Name), "handler": "laisse", "script": "évènements"})
	}
	if got := call(t, h, "events.list", nil).([]WorkbookListenerView); len(got) != len(WorkbookEvents) || got[0].Script != "Évènements" || got[0].Event != "open" {
		t.Fatalf("tous les évènements : %+v", got)
	}
}

// Les écouteurs voyagent avec le fichier.
func TestEventsSaved(t *testing.T) {
	h := eventsHost(t)
	call(t, h, "events.on", map[string]any{"event": "open", "handler": "laisse", "script": "Évènements"})
	path := filepath.Join(t.TempDir(), "évènements.lattis")
	if err := h.d.Save(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	d, err := doc.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ls, err := d.WorkbookListeners(context.Background())
	if err != nil || len(ls) != 1 || ls[0].Event != "open" || ls[0].Function != "laisse" {
		t.Fatalf("%+v %v", ls, err)
	}
}

func TestFireChange(t *testing.T) {
	h := eventsHost(t)
	call(t, h, "events.on", map[string]any{"event": "change", "handler": "saisie", "script": "Évènements"})
	call(t, h, "cells.set", map[string]any{"row": int64(2), "col": int64(3), "value": "avant"})
	old, err := CellAt(h, 1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	call(t, h, "cells.set", map[string]any{"row": int64(2), "col": int64(3), "value": int64(42)})
	data := RangeEventData(h, 1, 1, 2, 1, 2)
	now, _ := CellAt(h, 1, 1, 2)
	data["value"], data["old"], data["source"] = now.Value, old.Value, "edit"
	h.prints = nil
	res, err := FireEvent(h, EventChange, data)
	if err != nil || res.Error != "" || res.Ran != 1 || res.Cancelled || res.Skipped {
		t.Fatalf("%+v %v", res, err)
	}
	if got := strings.Join(h.prints, ";"); got != "change\tchange\tFeuille1\tC2\t2\t3\tavant\t42\tedit" {
		t.Fatalf("sortie %q", got)
	}
	if got := asJSON(call(t, h, "cells.get", map[string]any{"row": int64(1), "col": int64(5)})); !strings.Contains(got, `"value":"vu C2"`) {
		t.Fatalf("écriture de l'écouteur : %s", got)
	}
	// Ce que l'écouteur a modifié forme une étape d'annulation.
	if st := call(t, h, "doc.state", nil).(DocState); st.Undo != "Évènement « change »" {
		t.Fatalf("étape %q", st.Undo)
	}
	call(t, h, "doc.undo", nil)
	if got := asJSON(call(t, h, "cells.get", map[string]any{"row": int64(1), "col": int64(5)})); strings.Contains(got, "vu") {
		t.Fatalf("après annulation : %s", got)
	}
	if got := asJSON(call(t, h, "cells.get", map[string]any{"row": int64(2), "col": int64(3)})); !strings.Contains(got, `"raw":"42"`) {
		t.Fatalf("la saisie de l'utilisateur reste : %s", got)
	}
	// Aucun écouteur pour un autre évènement : rien ne tourne.
	if res, _ := FireEvent(h, EventRecalc, nil); res.Ran != 0 {
		t.Fatalf("recalc : %+v", res)
	}
	if !Listening(h, EventChange) || Listening(h, EventOpen) {
		t.Fatal("Listening")
	}
}

func TestEventData(t *testing.T) {
	h := eventsHost(t)
	call(t, h, "cells.paste", map[string]any{"row": int64(3), "col": int64(2), "values": []any{[]any{"Nom", "Âge"}, []any{"Léa", int64(31)}, []any{"Tom", int64(40)}}})
	tb := call(t, h, "sheets.get", nil).(SheetView).Tables[0].Name
	got := RangeEventData(h, 1, 3, 2, 3, 1) // B4:C4, à l'envers
	if got["address"] != "B4:C4" || got["row"] != 4 || got["col"] != 2 || got["row2"] != 4 || got["col2"] != 3 || got["sheet"] != "Feuille1" || got["table"] != tb || got["column"] != "Nom" {
		t.Fatalf("plage %v", got)
	}
	row := TableRowEventData(h, tb, 1)
	if row["line"] != 2 || row["address"] != "B5:C5" || row["table"] != tb {
		t.Fatalf("ligne %v", row)
	}
	if TableRowEventData(h, "Nul", 0) != nil {
		t.Fatal("tableau inconnu")
	}
	if s := SheetEventData(h, 1); s["sheet"] != "Feuille1" || s["sheet_id"] != int64(1) {
		t.Fatalf("feuille %v", s)
	}
}

func TestFireBeforeCancel(t *testing.T) {
	h := eventsHost(t)
	call(t, h, "events.on", map[string]any{"event": "before_save", "handler": "laisse", "script": "Évènements"})
	if res, err := FireEvent(h, EventBeforeSave, map[string]any{"path": "x.lattis", "as": false}); err != nil || res.Cancelled || res.Ran != 1 {
		t.Fatalf("nil laisse faire : %+v %v", res, err)
	}
	call(t, h, "events.on", map[string]any{"event": "before_save", "handler": "garde", "script": "Évènements"})
	call(t, h, "events.on", map[string]any{"event": "before_save", "handler": "saisie", "script": "Évènements"})
	h.prints = nil
	res, err := FireEvent(h, EventBeforeSave, map[string]any{"path": "x.lattis", "as": false})
	if err != nil || !res.Cancelled || res.Ran != 2 {
		t.Fatalf("false annule : %+v %v", res, err)
	}
	if got := strings.Join(h.prints, ";"); got != "garde\tx.lattis\tfalse" {
		t.Fatalf("après false, les écouteurs suivants ne tournent pas : %q", got)
	}
}

func TestFireErrorReportedOnce(t *testing.T) {
	h := eventsHost(t)
	call(t, h, "events.on", map[string]any{"event": "before_close", "handler": "casse", "script": "Évènements"})
	call(t, h, "events.on", map[string]any{"event": "before_close", "handler": "garde", "script": "Évènements"})
	for range 3 {
		res, err := FireEvent(h, EventBeforeClose, map[string]any{"path": ""})
		// Un écouteur en erreur n'empêche pas de fermer, et arrête les suivants.
		if err != nil || res.Cancelled || !strings.Contains(res.Error, "cassé") {
			t.Fatalf("%+v %v", res, err)
		}
	}
	if len(h.notices) != 1 || !strings.Contains(h.notices[0], "before_close") || !strings.Contains(h.notices[0], "Évènements.casse") {
		t.Fatalf("erreur signalée une fois : %q", h.notices)
	}
	if n := strings.Count(strings.Join(h.prints, "\n"), "cassé"); n != 1 {
		t.Fatalf("console : %q", h.prints)
	}
	// Un nouveau classeur : l'erreur est signalée de nouveau.
	ResetEvents()
	FireEvent(h, EventBeforeClose, nil)
	if len(h.notices) != 2 {
		t.Fatalf("après ResetEvents : %q", h.notices)
	}
	// Script disparu de la bibliothèque.
	call(t, h, "events.off", nil)
	call(t, h, "events.on", map[string]any{"event": "open", "handler": "laisse", "script": "Évènements"})
	list := call(t, h, "scripts.list", nil)
	_ = list
	if _, err := Invoke(context.Background(), h, "scripts.delete", map[string]any{"name": "Évènements"}); err != nil {
		t.Fatal(err)
	}
	if res, _ := FireEvent(h, EventOpen, nil); !strings.Contains(res.Error, "introuvable") {
		t.Fatalf("script disparu : %+v", res)
	}
}

func TestFireBeforeTimeout(t *testing.T) {
	h := eventsHost(t)
	was := BeforeTimeout
	BeforeTimeout = 200 * time.Millisecond
	defer func() { BeforeTimeout = was }()
	call(t, h, "events.on", map[string]any{"event": "before_close", "handler": "boucle", "script": "Évènements"})
	start := time.Now()
	res, err := FireEvent(h, EventBeforeClose, nil)
	if err != nil || res.Cancelled || !strings.Contains(res.Error, "durée maximale") {
		t.Fatalf("%+v %v", res, err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("trop long : %v", d)
	}
}

func TestEventsDisabledAndBusy(t *testing.T) {
	h := eventsHost(t)
	call(t, h, "events.on", map[string]any{"event": "recalc", "handler": "saisie", "script": "Évènements"})
	run(t, h, `assert(events.enabled() == true)
assert(events.enable(false) == true)
assert(events.enabled() == false)`)
	if res, _ := FireEvent(h, EventRecalc, nil); res.Ran != 0 || Listening(h, EventRecalc) {
		t.Fatalf("coupés : %+v", res)
	}
	run(t, h, `assert(events.enable(true) == false)`)
	// Pendant un script, un évènement n'est pas traité.
	running.mu.Lock()
	running.cancel = func() {}
	running.mu.Unlock()
	res, err := FireEvent(h, EventRecalc, map[string]any{})
	running.mu.Lock()
	running.cancel = nil
	running.mu.Unlock()
	if err != nil || !res.Skipped || res.Ran != 0 {
		t.Fatalf("occupé : %+v %v", res, err)
	}
	if _, err := FireEvent(h, "inconnu", nil); err == nil {
		t.Fatal("évènement inconnu accepté")
	}
}

// Un écouteur qui provoque un évènement de contrôle : traité dans la même
// étape d'annulation.
func TestFireDrainsControlEvents(t *testing.T) {
	h := eventsHost(t)
	call(t, h, "scripts.save", map[string]any{"name": "Liste", "source": `
function choisir(e) controls.set("Liste1", {value = "B"}) end
function vu(e) cells.set(1, 1, "liste " .. e.value) end
`})
	call(t, h, "controls.add", map[string]any{"kind": "select", "row": int64(2), "col": int64(2), "props": map[string]any{"options": []any{"A", "B"}, "value": "A"}})
	call(t, h, "controls.on", map[string]any{"control": "Liste1", "event": "change", "handler": "vu", "script": "Liste"})
	call(t, h, "events.on", map[string]any{"event": "open", "handler": "choisir", "script": "Liste"})
	res, err := FireEvent(h, EventOpen, map[string]any{"path": "", "title": "Classeur"})
	if err != nil || res.Error != "" || res.Ran != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if got := asJSON(call(t, h, "cells.get", map[string]any{"row": int64(1), "col": int64(1)})); !strings.Contains(got, `"value":"liste B"`) {
		t.Fatalf("écouteur du contrôle : %s", got)
	}
	if st := call(t, h, "doc.state", nil).(DocState); st.Undo != "Évènement « open »" {
		t.Fatalf("étape %q", st.Undo)
	}
}
