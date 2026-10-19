package api

import (
	"context"
	"strings"
	"testing"
)

func TestControlsLua(t *testing.T) {
	h := newHost(t)
	run(t, h, `
local l = controls.add("select", 2, 2, {options = {"Tous", "Payés"}, value = "Payés"})
assert(l.name == "Liste1" and l.kind == "select" and l.link == "C2" and l.value == "Payés", l.name)
local c = controls.add("checkbox", 3, 2, {name = "Urgent", label = "Urgent", link = "D3"})
assert(c.value == false)
controls.set("Urgent", {value = true, label = "Très urgent", hidden = true})
c = controls.get("urgent")
assert(c.value == true and c.label == "Très urgent" and c.hidden, tostring(c.value))
controls.set("Urgent", {visible = true, row = 5, col = 1, width = 140})
c = controls.get("Urgent")
assert(not c.hidden and c.row == 5 and c.col == 1 and c.width == 140)
local s = controls.add("input", 1, 1, {label = "Client…"})
assert(s.name == "Saisie1" and s.link == "B1" and s.label == "Client…")
controls.set(s.name, {value = "Durand"})
assert(cells.get(1, 2).value == "Durand")
local z = controls.add("textarea", 8, 1)
local f = controls.add("file", 9, 1)
local b = controls.add("button", 10, 1, {label = "Go"})
assert(z.name == "Zone1" and f.name == "Fichier1" and f.label ~= "" and b.name == "Bouton1" and b.kind == "button")
assert(#controls.list() == 6 and #controls.list("Feuille1") == 6)
controls.delete("Zone1")
assert(#controls.list() == 5)
`)
	if _, err := Invoke(context.Background(), h, "controls.set", map[string]any{"control": "Urgent", "props": map[string]any{"couleur": "#fff"}}); err == nil || !strings.Contains(err.Error(), "inconnue") {
		t.Fatalf("propriété inconnue : %v", err)
	}
	if _, err := Invoke(context.Background(), h, "controls.set", map[string]any{"control": "Urgent", "props": map[string]any{"name": "Liste1"}}); err == nil {
		t.Fatal("nom en double accepté")
	}
	if _, err := Invoke(context.Background(), h, "controls.get", map[string]any{"control": "Nulle"}); err == nil {
		t.Fatal("contrôle inconnu accepté")
	}
	// Les anciennes commandes buttons.* trouvent un contrôle par son nom.
	call(t, h, "buttons.set", map[string]any{"button": "Bouton1", "label": "Partir"})
	if got := asJSON(call(t, h, "buttons.list", nil)); !strings.Contains(got, `"name":"Bouton1","kind":"button","label":"Partir"`) {
		t.Fatalf("boutons %s", got)
	}
}

func TestControlEvents(t *testing.T) {
	h := newHost(t)
	call(t, h, "scripts.save", map[string]any{"name": "Écoute", "source": `
function action(e)
  print("action", e.control, e.event, e.value, e.old)
end
function noter(e)
  print("noter", e.control, e.event, e.value, e.old, event.value)
  cells.set(1, 1, "vu " .. e.value)
end
function lier()
  controls.on("Liste1", "change", noter) -- la fonction elle-même
end
`})
	call(t, h, "controls.add", map[string]any{"kind": "select", "row": int64(2), "col": int64(2), "props": map[string]any{
		"options": []any{"A", "B"}, "value": "A", "script": "Écoute", "function": "action",
	}})
	// Lié depuis le script lui-même : le script en cours est retenu.
	res, err := RunScript(h, "Écoute", `controls.on("Liste1", "change", "noter")`, "")
	if err != nil || res.Error != "" {
		t.Fatalf("%+v %v", res, err)
	}
	ls := call(t, h, "controls.listeners", map[string]any{"control": "Liste1"}).([]ListenerView)
	if len(ls) != 2 || !ls[0].Action || ls[1] != (ListenerView{Control: "Liste1", Event: "change", Script: "Écoute", Function: "noter"}) {
		t.Fatalf("écouteurs %+v", ls)
	}
	// Une seconde fois : rien ne change ; par la fonction elle-même aussi.
	res, _ = RunScript(h, "Écoute", mustSource(t, h, "Écoute"), "lier")
	if res.Error != "" {
		t.Fatal(res.Error)
	}
	if ls := call(t, h, "controls.listeners", nil).([]ListenerView); len(ls) != 2 {
		t.Fatalf("doublon : %+v", ls)
	}

	// Depuis l'interface : la valeur est déjà dans la cellule liée.
	call(t, h, "cells.set", map[string]any{"row": int64(2), "col": int64(3), "value": "B"})
	h.prints = nil
	res, err = FireControl(h, idOf(t, h, "Liste1"), "change", "B", "A")
	if err != nil || res.Error != "" {
		t.Fatalf("%+v %v", res, err)
	}
	want := "action\tListe1\tchange\tB\tA;noter\tListe1\tchange\tB\tA\tB"
	if got := strings.Join(h.prints, ";"); got != want {
		t.Fatalf("sortie %q, attendu %q", got, want)
	}
	if v := call(t, h, "cells.get", map[string]any{"row": int64(1), "col": int64(1)}).(CellValue); v.Value != "vu B" {
		t.Fatalf("écriture de l'écouteur : %v", v.Value)
	}
	// Une seule étape d'annulation, nommée d'après le premier script.
	if st := call(t, h, "doc.state", nil).(DocState); st.Undo != "Script « Écoute »" {
		t.Fatalf("étape %q", st.Undo)
	}

	// Depuis un script : à la fin du script, dans la même étape.
	h.prints = nil
	run(t, h, `
controls.set("Liste1", {value = "A"})
print("fin du script")
controls.set("Liste1", {value = "A"}) -- même valeur : pas d'évènement
`)
	if got := strings.Join(h.prints, ";"); got != "fin du script;action\tListe1\tchange\tA\tB;noter\tListe1\tchange\tA\tB\tA" {
		t.Fatalf("sortie %q", got)
	}
	// Serveur MCP : tout de suite.
	h.prints = nil
	call(t, h, "controls.set", map[string]any{"control": "Liste1", "props": map[string]any{"value": "B"}})
	if len(h.prints) != 2 {
		t.Fatalf("sortie %q", h.prints)
	}
	// Désabonnement.
	if n := call(t, h, "controls.off", map[string]any{"control": "Liste1", "handler": "noter"}); n != 1 {
		t.Fatalf("désabonnés : %v", n)
	}
	h.prints = nil
	res, _ = RunButton(h, idOf(t, h, "Liste1"))
	if res.Error != "" || strings.Join(h.prints, ";") != "action\tListe1\tchange\tB\tB" {
		t.Fatalf("après off : %+v %q", res, h.prints)
	}
	// Un écouteur ne peut pas s'abonner à un évènement que le contrôle ne produit pas.
	if _, err := Invoke(context.Background(), h, "controls.on", map[string]any{"control": "Liste1", "event": "click", "handler": "noter", "script": "Écoute"}); err == nil {
		t.Fatal("click d'une liste accepté")
	}
	// Hors d'un script, le script est obligatoire et doit exister.
	if _, err := Invoke(context.Background(), h, "controls.on", map[string]any{"control": "Liste1", "event": "change", "handler": "noter"}); err == nil {
		t.Fatal("écouteur sans script accepté")
	}
	if _, err := Invoke(context.Background(), h, "controls.on", map[string]any{"control": "Liste1", "event": "change", "handler": "f", "script": "Nul"}); err == nil {
		t.Fatal("script inconnu accepté")
	}
}

func TestControlClickAndChain(t *testing.T) {
	h := newHost(t)
	call(t, h, "scripts.save", map[string]any{"name": "Clics", "source": `
function clic(e)
  print("clic", e.control, e.event, e.value)
  controls.set("Compteur", {value = (tonumber(controls.get("Compteur").value) or 0) + 1})
end
function boucle(e)
  controls.set("Compteur", {value = tonumber(e.value) + 1})
end
`})
	call(t, h, "controls.add", map[string]any{"kind": "button", "row": int64(1), "col": int64(1), "props": map[string]any{"label": "Plus"}})
	call(t, h, "controls.add", map[string]any{"kind": "input", "row": int64(2), "col": int64(1), "props": map[string]any{"name": "Compteur"}})
	call(t, h, "controls.on", map[string]any{"control": "Bouton1", "event": "click", "handler": "clic", "script": "Clics"})
	id := idOf(t, h, "Bouton1")
	for i := 0; i < 2; i++ {
		if res, err := RunButton(h, id); err != nil || res.Error != "" {
			t.Fatalf("%+v %v", res, err)
		}
	}
	if v := call(t, h, "controls.get", map[string]any{"control": "Compteur"}).(ControlView); v.Value != "2" {
		t.Fatalf("compteur %v", v.Value)
	}
	if h.prints[0] != "clic\tBouton1\tclick\tnil" {
		t.Fatalf("sortie %q", h.prints)
	}
	// Un écouteur qui change la valeur de son propre contrôle s'arrête.
	call(t, h, "controls.on", map[string]any{"control": "Compteur", "event": "change", "handler": "boucle", "script": "Clics"})
	res, err := RunButton(h, id)
	if err != nil || !strings.Contains(res.Error, "en chaîne") {
		t.Fatalf("boucle : %+v %v", res, err)
	}
	call(t, h, "doc.undo", nil)
	if v := call(t, h, "controls.get", map[string]any{"control": "Compteur"}).(ControlView); v.Value != "2" {
		t.Fatalf("annulation de la chaîne : %v", v.Value)
	}
}

func idOf(t *testing.T, h *fakeHost, name string) int64 {
	t.Helper()
	return call(t, h, "controls.get", map[string]any{"control": name}).(ControlView).ID
}

func mustSource(t *testing.T, h *fakeHost, name string) string {
	t.Helper()
	list, err := h.d.Scripts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.Name == name {
			return s.Source
		}
	}
	t.Fatalf("script %s introuvable", name)
	return ""
}
