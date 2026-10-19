package integration

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Lot 4 : un onglet par mois à plusieurs tableaux (ventes, liste de tarifs
// recopiée, notes du mois), un titre « Janvier 2025 » en A1, un objectif qui
// change chaque mois et un total sous les ventes ; une synthèse cite les
// ventes de janvier. tables.merge_sheets sans tableaux reprend le groupe
// trouvé : un tableau commun par famille (tarifs gardés une fois), le mois
// tiré de A1, l'objectif et le titre par mois. L'onglet à choix filtre
// chaque tableau commun ; annuler, rétablir, enregistrer et rouvrir gardent
// tout, cellules par mois comprises.
func TestMergeSheetFamiliesAPI(t *testing.T) {
	h := newHost(t)
	months := []string{"Janvier 2025", "Février 2025", "Mars 2025"}
	sheets := []string{"Feuille1", "Feuille2", "Feuille3"}
	for i, m := range months {
		if i > 0 {
			h.do("sheets.add", "name", sheets[i])
		}
		s := sheets[i]
		h.do("cells.set", "sheet", s, "row", 1, "col", 1, "value", m)
		h.do("cells.set", "sheet", s, "row", 1, "col", 7, "value", "Objectif")
		h.do("cells.set", "sheet", s, "row", 1, "col", 8, "value", 5000+1000*i)
		h.do("cells.paste", "sheet", s, "row", 3, "col", 1, "values", norm([][]any{{"Client", "Montant"}, {"Durand", 10 * (i + 1)}, {"Martin", i + 1}}))
		h.do("cells.paste", "sheet", s, "row", 3, "col", 5, "values", norm([][]any{{"Produit", "Prix"}, {"Pomme", 2}, {"Poire", 3}}))
		h.do("cells.paste", "sheet", s, "row", 10, "col", 1, "values", norm([][]any{{"Jour", "Note"}, {i + 1, "relance " + m}, {i + 10, "inventaire"}}))
		ventes := ""
		for _, tb := range h.sheet(s).Tables {
			if tb.Row == 3 && tb.Col == 1 {
				ventes = tb.Name
			}
		}
		h.do("cells.set", "sheet", s, "row", 7, "col", 1, "value", "Total")
		h.do("cells.set", "sheet", s, "row", 7, "col", 2, "value", "=SOMME("+ventes+"[Montant])")
		if i == 0 {
			h.do("sheets.add", "name", "Synthèse")
			h.do("cells.set", "sheet", "Synthèse", "row", 1, "col", 1, "value", "=SOMME("+ventes+"[Montant])")
		}
	}
	h.reopen(path(t, "mois.lattis"))
	var groups []canvas.SheetGroup
	decode(t, h.do("tables.similar"), &groups)
	if len(groups) != 1 || len(groups[0].Families) != 2 || groups[0].CategoryFrom != "cell:A1" {
		t.Fatalf("onglets semblables : %+v", groups)
	}
	before := h.snapshot()

	var res canvas.MergeResult
	decode(t, h.do("tables.merge_sheets", "options", map[string]any{"name": "Ventes"}), &res)
	if res.Table != "Ventes" || res.Rows != 6 || len(res.Families) != 3 || res.PerValue != 2 || res.Old != canvas.MergeOldDelete {
		t.Fatalf("regroupement : %+v", res)
	}
	var tarifs, notes string
	for _, f := range res.Families[1:] {
		switch f.Mode {
		case canvas.FamilyOnce:
			tarifs = f.Name
		case canvas.FamilyMerge:
			notes = f.Name
		}
	}
	if tarifs == "" || notes == "" {
		t.Fatalf("familles : %+v", res.Families)
	}
	if got := strings.Join(h.sheetNames(), ","); got != "Ventes,Synthèse" {
		t.Fatalf("onglets : %s", got)
	}
	cell := func(row, col int) string { return fmt.Sprint(h.getOn("Ventes", row, col).Value) }
	rowsOf := func(table string) int { return len(h.rows(table).Rows) }
	want := func(when, choice string, objectif, total string, title string) {
		t.Helper()
		if got := cell(1, 9); got != objectif {
			t.Fatalf("%s : objectif %s, attendu %s", when, got, objectif)
		}
		if got := cell(11, 3); got != total {
			t.Fatalf("%s : total %s, attendu %s (%s)", when, got, total, h.getOn("Ventes", 11, 3).Raw)
		}
		if got := cell(1, 2); got != title {
			t.Fatalf("%s : titre %q, attendu %q", when, got, title)
		}
		if n := rowsOf("Ventes"); n != 2 {
			t.Fatalf("%s : ventes %d lignes", when, n)
		}
		if n := rowsOf(notes); n != 2 {
			t.Fatalf("%s : notes %d lignes", when, n)
		}
		if n := rowsOf(tarifs); n != 2 {
			t.Fatalf("%s : tarifs %d lignes", when, n)
		}
		var ch struct{ Value string }
		decode(t, h.do("sheets.choice", "sheet", "Ventes"), &ch)
		if ch.Value != choice {
			t.Fatalf("%s : choix %s, attendu %s", when, ch.Value, choice)
		}
	}
	want("après le regroupement", "01/03/2025", "7000", "33", "Mars 2025")
	if got := fmt.Sprint(h.getOn("Synthèse", 1, 1).Value); got != "11" {
		t.Fatalf("synthèse : %s (%s)", got, h.getOn("Synthèse", 1, 1).Raw)
	}
	h.do("sheets.set_choice", "sheet", "Ventes", "value", "01/02/2025")
	want("février", "01/02/2025", "6000", "22", "Février 2025")
	h.do("cells.set", "sheet", "Ventes", "row", 1, "col", 9, "value", 6500)
	h.do("sheets.set_choice", "sheet", "Ventes", "value", "01/03/2025")
	want("mars après la saisie de février", "01/03/2025", "7000", "33", "Mars 2025")
	h.do("sheets.set_choice", "sheet", "Ventes", "value", "01/02/2025")
	want("février saisi", "01/02/2025", "6500", "22", "Février 2025")

	h.undo() // la saisie de l'objectif
	want("annulation de la saisie", "01/02/2025", "6000", "22", "Février 2025")
	h.undo() // le regroupement
	if got := h.snapshot(); got != before {
		t.Fatalf("annulation :\n%s\nattendu\n%s", got, before)
	}
	h.redo()
	h.redo()
	h.do("sheets.set_choice", "sheet", "Ventes", "value", "01/02/2025")
	want("rétablissement", "01/02/2025", "6500", "22", "Février 2025")

	h.reopen(path(t, "regroupe.lattis"))
	want("réouverture", "01/02/2025", "6500", "22", "Février 2025")
	h.do("sheets.set_choice", "sheet", "Ventes", "value", "01/01/2025")
	want("janvier après réouverture", "01/01/2025", "5000", "11", "Janvier 2025")

	// « Suivre l'année » sur les tarifs : ils prennent le mois affiché.
	if msg := fmt.Sprint(h.do("tables.follow_choice", "table", tarifs)); !strings.Contains(msg, "suit le mois") {
		t.Fatalf("suivre : %s", msg)
	}
	h.do("sheets.set_choice", "sheet", "Ventes", "value", "01/03/2025")
	if n := rowsOf(tarifs); n != 0 {
		t.Fatalf("tarifs de janvier vus en mars : %d", n)
	}
	h.undo()
	if n := rowsOf(tarifs); n != 2 {
		t.Fatalf("tarifs après annulation : %d", n)
	}
}
