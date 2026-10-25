package api

import (
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
)

// Liens hypertexte : propriété link de cells.style, et colonne calculée
// =LIEN_HYPERTEXTE(…) dont les fenêtres donnent l'adresse de chaque ligne.
func TestLinks(t *testing.T) {
	h := newHost(t)
	call(t, h, "cells.paste", map[string]any{"row": int64(1), "col": int64(1), "values": []any{
		[]any{"Nom", "Site"}, []any{"Lattis", "lattis.be"}, []any{"Mauvais", "javascript:x"},
	}})
	call(t, h, "cells.set", map[string]any{"row": int64(1), "col": int64(3), "value": "Lien"})
	call(t, h, "cells.set", map[string]any{"row": int64(2), "col": int64(3), "value": `=LIEN_HYPERTEXTE("https://"&[@Site]; [@Nom])`})
	call(t, h, "cells.style", map[string]any{"row": int64(8), "col": int64(1), "style": map[string]any{"link": "jean@exemple.fr"}})
	if _, err := Invoke(t.Context(), h, "cells.style", map[string]any{"row": int64(8), "col": int64(2), "style": map[string]any{"link": "javascript:alert(1)"}}); err == nil {
		t.Fatal("javascript: accepté")
	}
	d, _ := h.Doc()
	l, err := d.Layout(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	var free string
	for _, s := range l.Styles {
		if s.Row == 7 && s.Col == 0 {
			free = s.Style
		}
	}
	if free != `{"ln":"mailto:jean@exemple.fr"}` {
		t.Fatalf("style link : %q", free)
	}
	g, err := LoadGrid(t.Context(), d, l.Tables[0])
	if err != nil {
		t.Fatal(err)
	}
	w, err := g.Window(t.Context(), 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if w.Cells[0][2] != "Lattis" {
		t.Fatalf("valeur : %v", w.Cells[0])
	}
	// « https://javascript:x » reste une adresse web ; une adresse refusée
	// (javascript:) ne serait pas rendue.
	if len(w.Links) != 2 || w.Links[0] != (grid.InvalidCell{Row: 0, Col: 2, Raw: "https://lattis.be"}) {
		t.Fatalf("liens de la fenêtre : %+v", w.Links)
	}
}
