package api

import (
	"context"
	"strings"
	"testing"
)

// cells.style accepte une validation tirée d'une plage (« =Tableau[Col] ») ;
// une source qui n'est pas une plage est refusée.
func TestStyleValidationRange(t *testing.T) {
	h := newHost(t)
	run(t, h, `
cells.paste{row = 1, col = 1, values = {{"Pays", "N"}, {"FR", 1}, {"IT", 2}, {"FR", 3}}}
tables.rename(sheets.get().tables[1].name, "Pays")
cells.style{row = 10, col = 1, style = {validation = "=Pays[Pays]"}}
cells.style{row = 11, col = 1, style = {validation = {"Oui", "Non"}}}
`)
	l, err := h.d.ValidationList(context.Background(), 1, "Pays[Pays]")
	if err != nil || l.Error != "" || strings.Join(l.Values, ",") != "FR,IT" {
		t.Fatalf("liste : %+v %v", l, err)
	}
	res, err := RunScript(h, "Test", `cells.style{row = 12, col = 1, style = {validation = "=1+2"}}`, "")
	if err != nil || !strings.Contains(res.Error, "n'est pas une plage") {
		t.Fatalf("source refusée : %+v %v", res, err)
	}
}

// tables.get : une colonne dont la validation est tirée de la clé d'un autre
// tableau porte ref = "Tableau[Clé]" (décision 0016).
func TestTableColumnRef(t *testing.T) {
	h := newHost(t)
	res := run(t, h, `
cells.paste{row = 1, col = 1, values = {{"Pays", "Nom"}, {"FR", "France"}, {"IT", "Italie"}}}
tables.rename(sheets.get().tables[1].name, "Pays")
tables.set_key("Pays", "Pays")
cells.paste{row = 1, col = 5, values = {{"Ville", "Pays"}, {"Paris", "FR"}, {"Rome", "IT"}}}
local v
for _, t in ipairs(sheets.get().tables) do if t.name ~= "Pays" then v = t.name end end
tables.rename(v, "Villes")
cells.style{row = 2, col = 6, row2 = 3, col2 = 6, style = {validation = "=Pays[Pays]"}}
local c = tables.get("Villes").columns
print(tostring(c[1].ref) .. "|" .. tostring(c[2].ref))
`)
	if len(res.Output) != 1 || res.Output[0] != "nil|Pays[Pays]" {
		t.Fatalf("ref : %v", res.Output)
	}
}
