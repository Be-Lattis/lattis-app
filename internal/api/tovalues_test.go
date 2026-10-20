package api

import (
	"strings"
	"testing"
)

// columns.to_values et cells.to_values : la colonne calculée devient une
// colonne saisie qui garde ses valeurs ; la cellule libre garde sa valeur.
func TestToValues(t *testing.T) {
	h := newHost(t)
	call(t, h, "cells.paste", map[string]any{"row": int64(1), "col": int64(1), "values": []any{
		[]any{"Produit", "Prix", "Qté"}, []any{"Clavier", "25,5", int64(2)}, []any{"Souris", int64(10), int64(3)},
	}})
	call(t, h, "cells.set", map[string]any{"row": int64(1), "col": int64(4), "value": "Montant"})
	call(t, h, "cells.set", map[string]any{"row": int64(2), "col": int64(4), "value": "=[@Prix]*[@Qté]"})
	call(t, h, "cells.set", map[string]any{"row": int64(6), "col": int64(1), "value": "=SOMME(Feuille1[Montant])"})
	name := call(t, h, "sheets.get", nil).(SheetView).Tables[0].Name
	if _, err := Invoke(t.Context(), h, "columns.to_values", map[string]any{"table": name, "column": "Prix"}); err == nil {
		t.Fatal("colonne saisie convertie")
	}
	call(t, h, "columns.to_values", map[string]any{"table": name, "column": "Montant"})
	c := call(t, h, "sheets.get", nil).(SheetView).Tables[0].Columns[3]
	if c.Name != "Montant" || c.Formula != "" || c.Type != "decimal" {
		t.Fatalf("colonne convertie : %s", asJSON(c))
	}
	if got, want := asJSON(call(t, h, "cells.read", map[string]any{"row": int64(2), "col": int64(4), "row2": int64(3), "col2": int64(4)})), `[["51"],["30"]]`; got != want {
		t.Fatalf("valeurs :\n got %s\nwant %s", got, want)
	}
	call(t, h, "cells.to_values", map[string]any{"row": int64(6), "col": int64(1), "row2": int64(6), "col2": int64(1)})
	if got, want := asJSON(call(t, h, "cells.get", map[string]any{"row": int64(6), "col": int64(1)})), `"raw":"81"`; !strings.Contains(got, want) {
		t.Fatalf("cellule libre : %s", got)
	}
}
