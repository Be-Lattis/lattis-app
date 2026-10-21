package canvas

import (
	"context"
	"testing"
)

// Une mise en forme posée sur une cellule d'un TCD vaut pour toute sa
// colonne (lignes, ou en-tête), et la suit quand le TCD est actualisé, trié
// ou s'élargit ; bordures et anciens styles de cellule restent à la cellule.
func TestPivotColumnStyles(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	sheet, id, base := handPivot(t, d) // en-tête en B2 : NID (C1), Nombre (C2)
	cellStyle := func(row, col int) Style {
		t.Helper()
		var s string
		_ = d.read(ctx, func(x *tx) error {
			return x.row(`SELECT coalesce((SELECT style FROM _lattis_cell_styles WHERE sheet = ? AND row = ? AND col = ?), '')`, sheet, row, col).Scan(&s)
		})
		return parseStyle(s)
	}
	styles := func() ([]string, []string) {
		t.Helper()
		p := onePivot(t, d, sheet)
		return p.Styles, p.HeadStyles
	}
	// Un ancien style de cellule (posé à la position, avant ce changement).
	must(t, d.Do(ctx, "ancien", func(x *tx) error {
		return x.styleFree(sheet, 3, 2, Style{"i": true, "fg": "#ff0000"}, nil)
	}))
	// Une cellule de la colonne « Nombre » : toute la colonne, pas l'en-tête.
	must(t, d.ApplyStyle(ctx, sheet, 2, 2, 2, 2, Style{"b": true, "fg": "#0000ff", "bb": true}))
	body, head := styles()
	if len(body) != 2 || body[0] != "" || body[1] != `{"b":true,"fg":"#0000ff"}` || head[1] != "" {
		t.Fatalf("colonne : %q / %q", body, head)
	}
	if s := cellStyle(2, 2); s["bb"] != true || s["b"] != nil {
		t.Fatalf("bordure à la cellule : %v", s)
	}
	// L'ancien style perd la couleur reprise par la colonne, garde l'italique.
	if s := cellStyle(3, 2); s["fg"] != nil || s["i"] != true {
		t.Fatalf("ancien style : %v", s)
	}
	// L'en-tête d'une colonne.
	must(t, d.ApplyStyle(ctx, sheet, 1, 1, 1, 1, Style{"bg": "#ffff00"}))
	if _, head = styles(); head[0] != `{"bg":"#ffff00"}` {
		t.Fatalf("en-tête : %q", head)
	}
	// Actualisé (une ligne de plus) puis trié : la mise en forme suit.
	if _, err := d.Paste(ctx, 1, 5, 0, [][]string{{"D", "Metz"}}); err != nil {
		t.Fatal(err)
	}
	cfg := base
	cfg.Sort = map[string]string{"NID": "desc"}
	must(t, d.SetPivot(ctx, id, cfg))
	p := onePivot(t, d, sheet)
	if len(p.Cells) != 6 || p.Cells[1][0] != "D" || p.Styles[1] != `{"b":true,"fg":"#0000ff"}` || p.HeadStyles[0] != `{"bg":"#ffff00"}` {
		t.Fatalf("après actualisation et tri : %v %q %q", p.Cells, p.Styles, p.HeadStyles)
	}
	// Un champ en colonnes : toutes les colonnes de la valeur.
	cfg.Cols = "Ville"
	must(t, d.SetPivot(ctx, id, cfg))
	p = onePivot(t, d, sheet)
	for j := 1; j < len(p.Cells[0]); j++ {
		if p.Styles[j] != `{"b":true,"fg":"#0000ff"}` {
			t.Fatalf("colonne %d (%s) : %q", j, p.Cells[0][j], p.Styles)
		}
	}
	// Le champ remis en place : rien n'est perdu ; Ctrl+Z défait une mise en forme.
	must(t, d.SetPivot(ctx, id, base))
	must(t, d.ApplyStyle(ctx, sheet, 3, 2, 4, 2, Style{"b": false}))
	if body, _ = styles(); body[1] != `{"fg":"#0000ff"}` {
		t.Fatalf("gras retiré : %q", body)
	}
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if body, _ = styles(); body[1] != `{"b":true,"fg":"#0000ff"}` {
		t.Fatalf("annulation : %q", body)
	}
	// Une colonne de la feuille mise en forme l'emporte sur celle du TCD.
	must(t, d.ApplyLineStyle(ctx, sheet, false, 2, 2, Style{"fg": "#00ff00"}))
	if body, _ = styles(); body[1] != `{"b":true}` {
		t.Fatalf("colonne de la feuille : %q", body)
	}
	check(t, d, "mise en forme d'un TCD")
}

// Colonnes en arborescence et colonnes saisies : désignées par leur champ
// ou leur numéro stable.
func TestPivotColKey(t *testing.T) {
	cfg := PivotConfig{RowFields: []string{"Service", "Poste"}, Values: []PivotValue{{Field: "Salaire", Agg: "sum"}, {Agg: "count"}}, Hand: []PivotHandCol{{H: 3, Name: "note"}}}
	want := []string{"r:*", "v:salaire|sum|", "v:|count|", "h:3", ""}
	for j, w := range want {
		if got := pivotColKey(cfg, 3, j); got != w {
			t.Fatalf("colonne %d : %q, voulu %q", j, got, w)
		}
	}
	flat := PivotConfig{RowFields: []string{"Ville"}, Cols: "Année", Values: []PivotValue{{Field: "X", Agg: "sum"}, {Field: "X", Agg: "avg"}}}
	want = []string{"r:ville", "v:x|sum|", "v:x|avg|", "v:x|sum|", "v:x|avg|"}
	for j, w := range want {
		if got := pivotColKey(flat, 5, j); got != w {
			t.Fatalf("à plat, colonne %d : %q, voulu %q", j, got, w)
		}
	}
}
