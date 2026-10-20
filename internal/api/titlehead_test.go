package api

import (
	"strings"
	"testing"
)

// tables.title_to_header : la ligne tapée au-dessus du tableau donne les noms
// des colonnes, l'ancien en-tête devient la 1re ligne de données.
func TestTitleToHeader(t *testing.T) {
	h := newHost(t)
	// Données sans en-têtes, puis la 1re ligne prise pour en-tête : « Léa | 31 ».
	call(t, h, "cells.paste", map[string]any{"row": int64(3), "col": int64(1), "values": []any{
		[]any{"Léa", int64(31)}, []any{"Max", int64(40)}, []any{"Zoé", int64(25)},
	}})
	name := call(t, h, "sheets.get", nil).(SheetView).Tables[0].Name
	// La 1re ligne devient l'en-tête à sa place (ligne 3) : l'en-tête
	// « Colonne n » de la ligne 2 disparaît.
	call(t, h, "tables.promote_header", map[string]any{"table": name})
	if tb := call(t, h, "sheets.get", nil).(SheetView).Tables[0]; tb.Row != 3 || tb.Rows != 2 {
		t.Fatalf("1re ligne en en-têtes : %s", asJSON(tb))
	}
	call(t, h, "cells.set", map[string]any{"row": int64(2), "col": int64(1), "value": "Nom"})
	call(t, h, "cells.set", map[string]any{"row": int64(2), "col": int64(2), "value": "Age"})
	call(t, h, "tables.title_to_header", map[string]any{"table": name})
	tb := call(t, h, "sheets.get", nil).(SheetView).Tables[0]
	if v := asJSON(tb); tb.Row != 2 || !strings.Contains(v, `{"name":"Nom","type":"text"`) || !strings.Contains(v, `{"name":"Age","type":"integer"`) {
		t.Fatalf("après : %s", v)
	}
	got := asJSON(call(t, h, "cells.read", map[string]any{"row": int64(2), "col": int64(1), "row2": int64(4), "col2": int64(2)}))
	if want := `[["Nom","Age"],["Léa","31"],["Max","40"]]`; got != want {
		t.Fatalf("lecture :\n got %s\nwant %s", got, want)
	}
	if _, err := Invoke(t.Context(), h, "tables.title_to_header", map[string]any{"table": name}); err == nil {
		t.Fatal("plus rien au-dessus : refus attendu")
	}
}
