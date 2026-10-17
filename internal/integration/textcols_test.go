package integration

import (
	"strings"
	"testing"
)

// Convertir (texte en colonnes) par les scripts Lua : colonne de tableau et
// cellules libres.
func TestTextToColumnsLua(t *testing.T) {
	h := newHost(t)
	h.pasteTable("T", 1, 1, []any{"Nom", "Ville"}, []any{"Dupont;007;03/04/2024", "Lyon"}, []any{"Martin;12;05/06/2023", "Rome"})
	h.lua(`assert(tables.text_to_columns("T", "Nom", {delims = ";", columns = {{}, {format = "text"}, {format = "date", order = "DMY"}}}) == 3)`)
	tb := h.table("T")
	var names []string
	for _, c := range tb.Columns {
		names = append(names, c.Name+":"+c.Type)
	}
	if got := strings.Join(names, ","); got != "Nom:text,Nom 2:text,Nom 3:date,Ville:text" {
		t.Fatalf("colonnes : %s", got)
	}
	h.wantColumn("T", "Nom 2", "007", "12")
	h.wantColumn("T", "Nom 3", "2024-04-03", "2023-06-05")

	h.set(10, 1, "a b  c")
	h.lua(`cells.text_to_columns(10, 1, 10, {delims = " ", merge = true, dest = "B12"})`)
	h.wantVal(12, 2, "a")
	h.wantVal(12, 4, "c")
	h.wantVal(10, 1, "a b  c")
}
