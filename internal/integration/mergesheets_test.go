package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/transfer"
)

// Un classeur Excel à un onglet par mois, importé : la détection trouve les
// onglets semblables, tables.merge_sheets les réunit en un seul tableau avec
// une colonne Mois (SQL par mois), et une seule annulation rend les onglets.
func TestMergeSheetsAfterXLSXImport(t *testing.T) {
	h := newHost(t)
	for i, m := range []string{"Janvier", "Février", "Mars"} {
		if i == 0 {
			h.do("sheets.rename", "sheet", "Feuille1", "name", m)
		} else {
			h.do("sheets.add", "name", m)
		}
		h.do("cells.paste", "sheet", m, "row", 1, "col", 1, "values", norm([][]any{{"Client", "Montant"}, {"Durand", 10 * (i + 1)}, {"Martin", 5}}))
	}
	h.do("sheets.add", "name", "Synthèse")
	h.do("cells.set", "sheet", "Synthèse", "row", 1, "col", 1, "value", "Bilan de l'année")
	out := path(t, "mois.xlsx")
	if _, err := transfer.ExportXLSXFile(context.Background(), h.d.Model, out); err != nil {
		t.Fatal(err)
	}

	h2 := newHost(t)
	h2.do("doc.import", "path", out)
	before := strings.Join(h2.sheetNames(), ",")
	var groups []canvas.SheetGroup
	decode(t, h2.do("tables.similar"), &groups)
	if len(groups) != 1 || len(groups[0].Tables) != 3 || groups[0].Category.Column != "Mois" || !groups[0].Propose {
		t.Fatalf("onglets semblables après import : %+v", groups)
	}
	var res canvas.MergeResult
	decode(t, h2.do("tables.merge_sheets", "options", map[string]any{"name": "Ventes"}), &res)
	if res.Table != "Ventes" || res.Rows != 6 || res.Old != canvas.MergeOldDelete {
		t.Fatalf("regroupement : %+v", res)
	}
	if got := strings.Join(h2.sheetNames(), ","); !strings.Contains(got, "Ventes") || strings.Contains(got, "Février") || !strings.Contains(got, "Synthèse") {
		t.Fatalf("onglets après regroupement : %s", got)
	}
	if got := h2.value(`SELECT group_concat(m || '=' || s, ' ') FROM (SELECT Mois AS m, sum(Montant) AS s FROM "Ventes" GROUP BY Mois ORDER BY min(_id))`); got != "Janvier=15 Février=25 Mars=35" {
		t.Fatalf("SQL par mois : %s", got)
	}
	h2.undo()
	if got := strings.Join(h2.sheetNames(), ","); got != before {
		t.Fatalf("après annulation : %s, attendu %s", got, before)
	}
	h2.redo()
	// Onglet à choix réglé sur le mois le plus récent : la vue ne montre que Mars
	// (le SQL ci-dessus voit tout).
	h2.wantColumn("Ventes", "Mois", "Mars", "Mars")
}

// Lot 2 : un classeur Excel à un onglet par mois, avec une synthèse à
// formules, un TCD et un graphique sur les onglets, importé puis regroupé :
// chaque formule reprise donne la même valeur, le TCD et le graphique les
// mêmes nombres ; tout survit à l'enregistrement, une annulation rend tout.
func TestMergeSheetsRewritesSummary(t *testing.T) {
	h := newHost(t)
	months := []string{"Janvier", "Février", "Mars"}
	for i, m := range months {
		if i == 0 {
			h.do("sheets.rename", "sheet", "Feuille1", "name", m)
		} else {
			h.do("sheets.add", "name", m)
		}
		h.do("cells.paste", "sheet", m, "row", 1, "col", 1, "values", norm([][]any{{"Client", "Montant", "Ville"}, {"Durand", 10 * (i + 1), "Paris"}, {"Martin", 5, ""}, {"Petit", i - 1, "Lyon"}}))
	}
	out := path(t, "mois.xlsx")
	if _, err := transfer.ExportXLSXFile(context.Background(), h.d.Model, out); err != nil {
		t.Fatal(err)
	}
	h = newHost(t)
	h.do("doc.import", "path", out)
	h.do("sheets.add", "name", "Synthèse")
	var formulas []string
	for _, m := range months {
		tb := h.sheet(m).Tables[0].Name
		formulas = append(formulas,
			"=SOMME("+tb+"[Montant])", "=MOYENNE("+tb+"[Montant])", "=MAX("+tb+"[Montant])", "=MIN("+tb+"[Montant])",
			"=NBVAL("+tb+"[Ville])", "=NB("+tb+"[Montant])", `=NB.SI(`+tb+`[Ville]; "Paris")`, "=LIGNES("+tb+")")
	}
	jan, fev := h.sheet("Janvier").Tables[0].Name, h.sheet("Février").Tables[0].Name
	formulas = append(formulas, "=SOMME("+jan+"[Montant]; "+fev+"[Montant])", "=MEDIANE("+fev+"[Montant])")
	for i, f := range formulas {
		h.do("cells.set", "sheet", "Synthèse", "row", i+1, "col", 1, "value", f)
	}
	var tcd struct{ Name string }
	decode(t, h.do("pivots.create", "config", map[string]any{"source": fev, "rowFields": []any{"Client"},
		"values": []any{map[string]any{"field": "Montant", "agg": "sum"}}}), &tcd)
	h.do("charts.create", "sheet", "Synthèse", "row", 2, "col", 5, "config", map[string]any{"source": jan, "kind": "bar", "x": "Client", "series": []any{"Montant"}})
	h.reopen(path(t, "avant.lattis"))
	values := func() []string {
		var out []string
		for i := range formulas {
			out = append(out, fmt.Sprint(h.getOn("Synthèse", i+1, 1).Value))
		}
		return out
	}
	vals, pivot, chart := values(), h.pivotCells(tcd.Name), h.chartData("Synthèse")
	if vals[0] != "14" || !strings.Contains(chart, `"values":[10,5,-1]`) {
		t.Fatalf("avant : %v %s", vals, chart)
	}
	before := h.snapshot()

	var res canvas.MergeResult
	decode(t, h.do("tables.merge_sheets", "options", map[string]any{"name": "Ventes"}), &res)
	if res.Old != canvas.MergeOldDelete || len(res.Pending) != 0 || len(res.Rewritten) != len(formulas)+2 {
		t.Fatalf("regroupement : %s %+v", res.Notice, res.Pending)
	}
	if !strings.Contains(res.Notice, fmt.Sprintf("%d formules reprises", len(formulas)+2)) {
		t.Errorf("barre d'état : %s", res.Notice)
	}
	check := func(when string) {
		t.Helper()
		if got := values(); fmt.Sprint(got) != fmt.Sprint(vals) {
			t.Fatalf("%s : %v, attendu %v", when, got, vals)
		}
		for i := range formulas {
			if raw := h.getOn("Synthèse", i+1, 1).Raw; !strings.Contains(raw, "Ventes[") {
				t.Fatalf("%s : ligne %d %s", when, i+1, raw)
			}
		}
		if got := h.pivotCells(tcd.Name); got != pivot {
			t.Fatalf("%s : TCD %s, attendu %s", when, got, pivot)
		}
		if got := h.chartData("Synthèse"); got != chart {
			t.Fatalf("%s : graphique %s, attendu %s", when, got, chart)
		}
	}
	check("après le regroupement")
	if names := strings.Join(h.sheetNames(), ","); strings.Contains(names, "Janvier") {
		t.Fatalf("anciens onglets : %s", names)
	}
	h.undo()
	if got := h.snapshot(); got != before {
		t.Fatalf("annulation :\n%s\nattendu\n%s", got, before)
	}
	h.redo()
	check("après rétablissement")
	h.reopen(path(t, "apres.lattis"))
	check("après réouverture")
}
