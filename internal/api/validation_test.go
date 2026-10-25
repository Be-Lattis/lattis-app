package api

import (
	"context"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
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

// cells.style pose aussi une règle (nombre entier, alerte, message) : les
// bornes d'un script peuvent être des nombres ; un script n'est pas
// vérifié, la grille l'est.
func TestStyleValidationRule(t *testing.T) {
	h := newHost(t)
	run(t, h, `
cells.style{row = 2, col = 2, style = {validation = {type = "whole", op = "between", min = 1, max = 10, blank = false,
	input = {title = "Quantité", msg = "De 1 à 10"}, error = {style = "warning", msg = "Hors bornes"}}}}
cells.set{row = 2, col = 2, value = 50}
`)
	r, err := h.d.ValidationAt(context.Background(), 1, 1, 1)
	if err != nil || r.Type != "whole" || r.Min != "1" || r.Max != "10" || r.Blank == nil || *r.Blank || r.Input.Msg != "De 1 à 10" || r.Error.Style != "warning" {
		t.Fatalf("règle : %+v %v", r, err)
	}
	cells, err := h.d.InvalidCells(context.Background(), 1)
	if err != nil || len(cells) != 1 || cells[0] != (canvas.CellPos{Row: 1, Col: 1}) {
		t.Fatalf("non valides : %v %v", cells, err)
	}
	res, err := RunScript(h, "Test", `cells.style{row = 3, col = 2, style = {validation = {type = "whole", op = "between", min = 1}}}`, "")
	if err != nil || res.Error == "" {
		t.Fatalf("borne manquante acceptée : %+v %v", res, err)
	}
}
