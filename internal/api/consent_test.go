package api

import (
	"context"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/trust"
)

// receivedButton prépare un classeur avec un bouton lié au script
// « Action », puis passe sur « un autre poste » (accords vides) : le
// classeur est reçu.
func receivedButton(t *testing.T) (*fakeHost, int64, string) {
	t.Helper()
	t.Cleanup(trust.Use(trust.New("", nil)))
	h := newHost(t)
	src := `function go() print("action") end`
	call(t, h, "scripts.save", map[string]any{"name": "Action", "source": src})
	call(t, h, "scripts.save", map[string]any{"name": "Autre", "source": `print("autre")`})
	call(t, h, "controls.add", map[string]any{"kind": "button", "row": int64(1), "col": int64(1), "props": map[string]any{"label": "Go"}})
	call(t, h, "controls.on", map[string]any{"control": "Bouton1", "event": "click", "handler": "go", "script": "Action"})
	id := idOf(t, h, "Bouton1")
	// Créé ici : aucune question.
	if res, err := RunButton(h, id); err != nil || res.Error != "" || len(h.dialogs) != 0 {
		t.Fatalf("classeur de ce poste : %+v %v, %d questions", res, err, len(h.dialogs))
	}
	t.Cleanup(trust.Use(trust.New("", nil)))
	h.prints = nil
	return h, id, src
}

// Un script d'un classeur reçu demande l'accord à sa première exécution,
// avec son code ; un refus vaut pour la session.
func TestScriptConsentRefused(t *testing.T) {
	h, id, src := receivedButton(t)
	res, err := RunButton(h, id)
	if err != nil || !strings.Contains(res.Error, "refusé") || len(h.prints) != 0 {
		t.Fatalf("refus : %+v %v %v", res, err, h.prints)
	}
	if len(h.dialogs) != 1 || h.dialogs[0].Code != src || h.dialogs[0].OK != "Autoriser" || h.dialogs[0].Cancel != "Refuser" || !strings.Contains(h.dialogs[0].Message, "Action") || !strings.Contains(h.dialogs[0].Warning, "faites confiance à la personne") {
		t.Fatalf("question : %+v", h.dialogs)
	}
	if res, _ := RunButton(h, id); !strings.Contains(res.Error, "pour cette session") || len(h.dialogs) != 1 {
		t.Fatalf("question reposée : %+v, %d questions", res, len(h.dialogs))
	}
	// Lancé depuis l'onglet SCRIPT (code sous les yeux) : autorisé.
	if _, err := RunScript(h, "Action", src, "go"); err != nil {
		t.Fatal(err)
	}
	if res, err := RunButton(h, id); err != nil || res.Error != "" {
		t.Fatalf("après exécution depuis l'onglet SCRIPT : %+v %v", res, err)
	}
}

// Autoriser exécute le script et n'est plus redemandé pour cette version ;
// une autre version redemande.
func TestScriptConsentAllowed(t *testing.T) {
	h, id, _ := receivedButton(t)
	h.answer = map[string]any{"all": false}
	for range 2 {
		if res, err := RunButton(h, id); err != nil || res.Error != "" {
			t.Fatalf("%+v %v", res, err)
		}
	}
	if len(h.dialogs) != 1 || strings.Join(h.prints, ",") != "action,action" {
		t.Fatalf("%d questions, sortie %v", len(h.dialogs), h.prints)
	}
	// L'autre script du classeur n'est pas autorisé pour autant…
	res, err := Invoke(WithoutPrompt(context.Background()), h, "scripts.run", map[string]any{"name": "Autre"})
	if r, ok := res.(ScriptResult); err != nil || !ok || !strings.Contains(r.Error, "seul l'utilisateur") {
		t.Fatalf("script non autorisé exécuté : %+v %v", res, err)
	}
	// … et une nouvelle version, modifiée ailleurs (sans passer par
	// l'enregistrement de ce poste), redemande.
	var sid int64
	list, _ := h.d.Scripts(context.Background())
	for _, s := range list {
		if s.Name == "Action" {
			sid = s.ID
		}
	}
	if _, err := h.d.Model.SaveScript(context.Background(), sid, "Action", `function go() print("v2") end`); err != nil {
		t.Fatal(err)
	}
	h.answer = nil
	if res, _ := RunButton(h, id); !strings.Contains(res.Error, "refusé") || len(h.dialogs) != 2 || !strings.Contains(h.dialogs[1].Code, "v2") {
		t.Fatalf("nouvelle version : %+v, %d questions", res, len(h.dialogs))
	}
}

// « Autoriser aussi les autres scripts » autorise toute la bibliothèque.
func TestScriptConsentAll(t *testing.T) {
	h, id, _ := receivedButton(t)
	h.answer = map[string]any{"all": true}
	if res, err := RunButton(h, id); err != nil || res.Error != "" {
		t.Fatalf("%+v %v", res, err)
	}
	res, err := Invoke(WithoutPrompt(context.Background()), h, "scripts.run", map[string]any{"name": "Autre"})
	if err != nil || res.(ScriptResult).Error != "" {
		t.Fatalf("autre script : %+v %v", res, err)
	}
}

// Le serveur MCP ne peut pas consulter l'utilisateur : un script du
// classeur non autorisé est refusé, le code qu'il passe lui-même tourne.
func TestScriptConsentMCP(t *testing.T) {
	h, _, _ := receivedButton(t)
	ctx := WithoutPrompt(context.Background())
	res, err := Invoke(ctx, h, "scripts.run", map[string]any{"name": "Action"})
	if err != nil {
		t.Fatal(err)
	}
	if r := res.(ScriptResult); !strings.Contains(r.Error, "seul l'utilisateur") || len(h.dialogs) != 0 {
		t.Fatalf("MCP : %+v, %d questions", r, len(h.dialogs))
	}
	res, err = Invoke(ctx, h, "scripts.run", map[string]any{"source": `print("ad hoc")`})
	if err != nil || res.(ScriptResult).Error != "" {
		t.Fatalf("code passé par MCP : %+v %v", res, err)
	}
}
