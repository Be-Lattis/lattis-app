package api

import (
	"fmt"
	"testing"
)

// cells.paste range = true : des cellules libres, pas de tableau ; cells.read
// lit la plage, tableau et cellules libres confondus.
func TestPasteRangeAndRead(t *testing.T) {
	h := newHost(t)
	block := []any{[]any{"Nom", "Âge"}, []any{"Léa", int64(31)}, []any{"Tom", int64(8)}}
	call(t, h, "cells.paste", map[string]any{"row": int64(1), "col": int64(1), "values": block, "range": true})
	v := call(t, h, "sheets.get", nil).(SheetView)
	if len(v.Tables) != 0 || len(v.Cells) != 6 {
		t.Fatalf("plage : %d tableaux, %d cellules", len(v.Tables), len(v.Cells))
	}
	call(t, h, "cells.paste", map[string]any{"row": int64(1), "col": int64(5), "values": block})
	got := fmt.Sprint(call(t, h, "cells.read", map[string]any{"row": int64(1), "col": int64(1), "row2": int64(3), "col2": int64(6)}))
	if want := "[[Nom Âge   Nom Âge] [Léa 31   Léa 31] [Tom 8   Tom 8]]"; got != want {
		t.Fatalf("lecture :\n got %s\nwant %s", got, want)
	}
}
