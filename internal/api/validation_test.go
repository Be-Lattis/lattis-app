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
