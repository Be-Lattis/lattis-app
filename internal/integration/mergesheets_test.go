package integration

import (
	"context"
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
