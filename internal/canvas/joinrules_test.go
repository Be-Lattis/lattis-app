package canvas

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

// cellOn donne la cellule libre (row, col) d'une feuille.
func cellOn(t *testing.T, d *fixture, sheet int64, row, col int) CellInfo {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range l.Cells {
		if c.Row == row && c.Col == col {
			return c
		}
	}
	return CellInfo{Row: row, Col: col}
}

// sqlText lit une valeur des métadonnées.
func sqlText(t *testing.T, d *fixture, q string, args ...any) string {
	t.Helper()
	var s sql.NullString
	if err := d.read(context.Background(), func(tx *tx) error { return tx.row(q, args...).Scan(&s) }); err != nil {
		t.Fatalf("%s : %v", q, err)
	}
	return s.String
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustSet(t *testing.T, d *fixture, sheet int64, row, col int, raw string) {
	t.Helper()
	if _, err := d.SetCell(context.Background(), sheet, row, col, raw); err != nil {
		t.Fatalf("saisie %q en (%d, %d) : %v", raw, row, col, err)
	}
}

// H1, H2 : après une fusion, ce qui citait le tableau absorbé (formules,
// colonne calculée, TCD, graphique, requête) cite le tableau fusionné, sous
// les nouveaux noms de ses colonnes ; une seule étape d'annulation.
func TestMergeRewritesReferences(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Nom", "Prix"}, {"a", "1"}, {"b", "2"}})
	must(t, err)
	_, err = d.Paste(ctx, 1, 0, 4, [][]string{{"Prix", "Pop"}, {"10", "100"}, {"20", "200"}})
	must(t, err)
	ts := tablesOf(t, d)
	a, b := ts[0].Name, ts[1].Name
	mustSet(t, d, 1, 0, 6, "Double")
	mustSet(t, d, 1, 1, 6, "=[Prix]*2")
	mustSet(t, d, 1, 6, 0, "=SOMME("+b+"[Pop])")
	mustSet(t, d, 1, 7, 0, "="+b+"[Pop][2]*2")
	mustSet(t, d, 1, 8, 0, "=SOMME("+b+"[Double])")
	must(t, d.SetKey(ctx, b, 0))
	mustSet(t, d, 1, 9, 0, `=REFERENCE(`+b+`[Pop]; 20)`)
	_, err = d.CreatePivot(ctx, PivotConfig{Source: b, RowFields: []string{"Prix"}, Values: []PivotValue{{Field: "Pop", Agg: "sum"}}})
	must(t, err)
	_, err = d.CreateChart(ctx, 1, 12, 0, ChartConfig{Source: b, Kind: "bar", X: "Prix", Series: []string{"Pop"}})
	must(t, err)
	_, err = d.SaveQuery(ctx, QueryDef{Name: "q", Source: `{"kind":"table","table":"` + b + `"}`, Steps: `[{"op":"filter","conditions":[{"column":"Prix","cmp":">","value":"5"}]}]`})
	must(t, err)
	want := map[int]string{6: "300", 7: "400", 8: "60", 9: "200"}
	for r, v := range want {
		if got := cellOn(t, d, 1, r, 0).Value; got != v {
			t.Fatalf("avant : ligne %d = %q, attendu %s", r, got, v)
		}
	}
	before := dump(t, d)

	// La colonne vide entre les deux disparaît : ils se touchent.
	must(t, d.DeleteSheetColumns(ctx, 1, 2, 3))
	check(t, d, "fusion")
	ts = tablesOf(t, d)
	// Le plus gros (b : 2 lignes × 3 colonnes) garde son nom.
	if len(ts) != 1 || ts[0].Name != b || colNames(ts[0]) != "Nom|Prix|Prix 2|Pop|Double" {
		t.Fatalf("fusion : %+v", ts)
	}
	for r, v := range want {
		if c := cellOn(t, d, 1, r, 0); c.Value != v {
			t.Fatalf("après fusion : ligne %d = %q (%s), attendu %s", r, c.Value, c.Raw, v)
		}
	}
	if f := ts[0].Columns[4].Formula; !strings.Contains(f, "[Prix 2]") {
		t.Fatalf("colonne calculée : %q", f)
	}
	l, _ := d.Layout(ctx, 1)
	for _, ch := range l.Charts {
		if ch.Error != "" || ch.Config.X != "Prix 2" {
			t.Fatalf("graphique : %+v", ch)
		}
	}
	for _, s := range l.Sheets {
		lp, _ := d.Layout(ctx, s.ID)
		for _, p := range lp.Pivots {
			if p.Error != "" {
				t.Fatalf("TCD : %s", p.Error)
			}
		}
	}
	q, err := d.QueryByName(ctx, "q")
	must(t, err)
	if !strings.Contains(q.Steps, `"op":"select"`) || !strings.Contains(q.Steps, `"column":"Prix 2","name":"Prix"`) || !strings.Contains(q.Source, `"table":"`+b+`"`) {
		t.Fatalf("requête : %s %s", q.Source, q.Steps)
	}
	if !strings.Contains(sqlText(t, d, `SELECT raw FROM _lattis_cells WHERE row = 6 AND col = 0`), b+"[Pop]") {
		t.Fatal("formule libre non réécrite")
	}
	_ = a
	undoOnce(t, d, before, "fusion")
}

// H3 : deux tableaux côte à côte dont les en-têtes ne sont pas sur la même
// ligne ne fusionnent pas (un en-tête se mêlerait aux données) : celui de
// droite s'écarte d'une colonne, avec un message.
func TestSideBySideHeadersDiffer(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"ID"}, {"1"}, {"2"}, {"3"}})
	must(t, err)
	_, err = d.Paste(ctx, 1, 2, 2, [][]string{{"Nom"}, {"x"}, {"y"}})
	must(t, err)
	before := dump(t, d)
	d.TakeNotice()
	must(t, d.DeleteSheetColumns(ctx, 1, 1, 1))
	check(t, d, "en-têtes décalés")
	ts := tablesOf(t, d)
	if len(ts) != 2 || ts[1].Col != 2 || ts[1].Row != 2 || ts[0].Rows != 3 || ts[1].Rows != 2 {
		t.Fatalf("en-têtes décalés : %+v\n%s", ts, view(t, d))
	}
	if n := d.TakeNotice(); !strings.Contains(n, "en-têtes sur des lignes différentes") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "en-têtes décalés")
}

// M1 : un tableau juste sous un autre (son en-tête collé à la dernière
// ligne) descend d'une ligne, même à colonnes identiques : rien ne change de
// sens (SOMME(B[Age]) garde sa valeur).
func TestStackedTables(t *testing.T) {
	ctx := context.Background()
	for _, second := range [][][]string{
		{{"Nom", "Age"}, {"c", "3"}, {"d", "4"}},
		{{"Ville", "Code"}, {"Paris", "75"}},
	} {
		d := open(t)
		_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Nom", "Age"}, {"a", "1"}, {"b", "2"}})
		must(t, err)
		_, err = d.Paste(ctx, 1, 4, 0, second)
		must(t, err)
		ts := tablesOf(t, d)
		b := ts[1].Name
		mustSet(t, d, 1, 0, 4, "=NBVAL("+b+"["+second[0][0]+"])")
		before := dump(t, d)
		d.TakeNotice()
		must(t, d.DeleteSheetRows(ctx, 1, 3, 3))
		check(t, d, "empilés")
		ts = tablesOf(t, d)
		if len(ts) != 2 || ts[1].Row != 4 || ts[0].Row+ts[0].Rows != 2 {
			t.Fatalf("empilés : %+v\n%s", ts, view(t, d))
		}
		if c := cellOn(t, d, 1, 0, 4); c.Value != fmt.Sprint(len(second)-1) {
			t.Fatalf("formule : %+v", c)
		}
		if n := d.TakeNotice(); !strings.Contains(n, "descendu d'une ligne") {
			t.Fatalf("message : %q", n)
		}
		undoOnce(t, d, before, "empilés")
	}
}

// M3 : un tableau filtré ne fusionne pas : ce qui vient se coller à lui
// s'écarte (ou reste libre), avec un message ; la saisie d'une donnée à côté
// de lui est refusée avec un message juste.
func TestFilteredNeighbour(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Nom"}, {"1"}, {"2"}})
	must(t, err)
	_, err = d.Paste(ctx, 1, 0, 2, [][]string{{"Ville"}, {"7"}, {"8"}})
	must(t, err)
	ts := tablesOf(t, d)
	must(t, d.SetFilter(ctx, ts[0].Name, 0, &ColumnFilter{Values: []string{"1"}}))
	before := dump(t, d)
	d.TakeNotice()
	// La colonne qui les sépare disparaît : le second s'écarte.
	must(t, d.DeleteSheetColumns(ctx, 1, 1, 1))
	check(t, d, "contre un tableau filtré")
	ts = tablesOf(t, d)
	if len(ts) != 2 || ts[1].Col != 2 {
		t.Fatalf("contre un tableau filtré : %+v\n%s", ts, view(t, d))
	}
	if n := d.TakeNotice(); !strings.Contains(n, "est filtré") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "contre un tableau filtré")

	// Saisir entre les deux : une colonne du tableau filtré, et l'autre
	// s'écarte.
	mustSet(t, d, 1, 0, 1, "Note")
	check(t, d, "entre filtré et autre")
	ts = tablesOf(t, d)
	if len(ts) != 2 || ts[0].Col+len(ts[0].Columns)+1 > ts[1].Col {
		t.Fatalf("entre filtré et autre : %+v\n%s", ts, view(t, d))
	}
	undoOnce(t, d, before, "entre filtré et autre")

	_, err = d.SetCell(ctx, 1, 1, 1, "x")
	if err == nil || !strings.HasPrefix(err.Error(), "Le tableau") || !strings.Contains(err.Error(), "une colonne") {
		t.Fatalf("donnée à côté d'un tableau filtré : %v", err)
	}
}

// M4 : le rattachement ne regarde que les feuilles touchées par l'action.
func TestJoinOnlyTouchedSheets(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	sh, err := d.AddSheet(ctx)
	must(t, err)
	_, err = d.Paste(ctx, sh, 0, 0, [][]string{{"Nom"}, {"a"}})
	must(t, err)
	// Ancien classeur : une cellule collée au tableau, laissée telle quelle.
	must(t, d.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO _lattis_cells (sheet, row, col, raw) VALUES (?, 0, 1, 'x')`, sh)
		return err
	}))
	mustSet(t, d, 1, 0, 0, "ailleurs")
	if l, _ := d.Layout(ctx, sh); len(l.Cells) != 1 {
		t.Fatalf("feuille non touchée modifiée : %+v", l.Cells)
	}
	mustSet(t, d, sh, 5, 5, "ici")
	if l, _ := d.Layout(ctx, sh); len(l.Cells) != 1 || len(l.Tables[0].Columns) != 2 {
		t.Fatalf("feuille touchée : %+v %+v", l.Cells, l.Tables)
	}
}

// M5, M6 : une cellule libre rattachée garde sa mise en forme ; une formule
// entre par sa valeur ; les formules qui la citaient (adresse ou nom) lisent
// la cellule du tableau ; la clé du tableau absorbé est gardée.
func TestAbsorbedCellKeepsMeaning(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Nom", "Prix"}, {"a", "1"}, {"b", "2"}})
	must(t, err)
	tb := tablesOf(t, d)[0].Name
	mustSet(t, d, 1, 4, 1, "=2*3")
	mustSet(t, d, 1, 8, 0, "=Cell[B5]*10")
	mustSet(t, d, 1, 1, 3, "7")
	must(t, d.SetName(ctx, 1, 1, 3, "Taux"))
	must(t, d.ApplyStyle(ctx, 1, 1, 3, 1, 3, Style{"b": true}))
	mustSet(t, d, 1, 9, 0, "=Taux*2")
	before := dump(t, d)
	d.TakeNotice()

	// Formule collée dessous : une ligne, par sa valeur.
	must(t, d.DeleteSheetRows(ctx, 1, 3, 3))
	check(t, d, "formule rattachée")
	ts := tablesOf(t, d)
	if ts[0].Rows != 3 || ts[0].Columns[1].Formula != "" {
		t.Fatalf("formule rattachée : %+v", ts)
	}
	if got := view(t, d); !strings.Contains(got, "\n|6") {
		t.Fatalf("formule rattachée :\n%s", got)
	}
	if c := cellOn(t, d, 1, 7, 0); c.Value != "60" || !strings.Contains(c.Raw, tb+"[Prix][3]") {
		t.Fatalf("formule qui la citait : %+v", c)
	}
	if n := d.TakeNotice(); !strings.Contains(n, "par sa valeur") {
		t.Fatalf("message : %q", n)
	}
	undoOnce(t, d, before, "formule rattachée")

	// Cellule nommée et mise en forme, collée à droite.
	must(t, d.DeleteSheetColumns(ctx, 1, 2, 2))
	check(t, d, "cellule nommée rattachée")
	ts = tablesOf(t, d)
	if colNames(ts[0]) != "Nom|Prix|Colonne 3" {
		t.Fatalf("cellule nommée rattachée : %+v", ts)
	}
	if c := cellOn(t, d, 1, 9, 0); c.Value != "14" || !strings.Contains(c.Raw, tb+"[Colonne 3][1]") {
		t.Fatalf("formule du nom : %+v", c)
	}
	if n := sqlText(t, d, `SELECT count(*) FROM _lattis_names`); n != "0" {
		t.Fatalf("nom resté : %s", n)
	}
	if s := sqlText(t, d, `SELECT style FROM _lattis_styles WHERE rid > 0`); !strings.Contains(s, `"b":true`) {
		t.Fatalf("mise en forme : %q", s)
	}
	if n := sqlText(t, d, `SELECT count(*) FROM _lattis_cell_styles`); n != "0" {
		t.Fatalf("mise en forme orpheline : %s", n)
	}
	undoOnce(t, d, before, "cellule nommée rattachée")
}

// M6 : la clé du tableau absorbé devient celle du tableau fusionné.
func TestMergeKeepsKey(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Nom"}, {"a"}, {"b"}})
	must(t, err)
	_, err = d.Paste(ctx, 1, 0, 2, [][]string{{"Code", "Val"}, {"x", "1"}, {"y", "2"}})
	must(t, err)
	b := tablesOf(t, d)[1].Name
	must(t, d.SetKey(ctx, b, 0))
	mustSet(t, d, 1, 6, 0, `=REFERENCE(`+b+`[Val]; "y")`)
	before := dump(t, d)
	must(t, d.DeleteSheetColumns(ctx, 1, 1, 1))
	check(t, d, "clé")
	ts := tablesOf(t, d)
	if len(ts) != 1 || ts[0].Key != 1 {
		t.Fatalf("clé : %+v", ts)
	}
	if c := cellOn(t, d, 1, 6, 0); c.Value != "2" {
		t.Fatalf("REFERENCE : %+v", c)
	}
	undoOnce(t, d, before, "clé")
}

// M7 : une cellule libre écartée par un tableau qui s'élargit emporte son
// nom et sa mise en forme, et les formules qui la citent suivent.
func TestGapMovesNamesAndRefs(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Nom"}, {"a"}})
	must(t, err)
	mustSet(t, d, 1, 0, 2, "G")
	must(t, d.SetName(ctx, 1, 0, 2, "Pin"))
	must(t, d.ApplyStyle(ctx, 1, 0, 2, 0, 2, Style{"b": true}))
	mustSet(t, d, 1, 6, 0, "=Cell[C1]&Pin")
	before := dump(t, d)
	mustSet(t, d, 1, 0, 1, "Ville")
	check(t, d, "écartement")
	if c := cellOn(t, d, 1, 0, 3); c.Raw != "G" {
		t.Fatalf("cellule écartée : %+v\n%s", c, view(t, d))
	}
	if c := cellOn(t, d, 1, 6, 0); c.Value != "GG" || !strings.Contains(c.Raw, "Cell[D1]") {
		t.Fatalf("formule : %+v", c)
	}
	if n := sqlText(t, d, `SELECT col FROM _lattis_names WHERE name = 'Pin'`); n != "3" {
		t.Fatalf("nom : colonne %s", n)
	}
	if s := sqlText(t, d, `SELECT style FROM _lattis_cell_styles WHERE row = 0 AND col = 3`); !strings.Contains(s, `"b":true`) {
		t.Fatalf("mise en forme : %q", s)
	}
	undoOnce(t, d, before, "écartement")
}

// Renommer un tableau ou une colonne : règles par formule, liste d'un
// contrôle et requête suivent.
func TestRenameFollowsRulesControlsQueries(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Nom", "Prix"}, {"a", "1"}, {"b", "2"}})
	must(t, err)
	_, err = d.Paste(ctx, 1, 0, 4, [][]string{{"Ville", "Pop"}, {"p", "10"}})
	must(t, err)
	ts := tablesOf(t, d)
	a, b := ts[0].Name, ts[1].Name
	must(t, d.SetRules(ctx, a, 1, []Rule{{Op: "formula", Value: "=[Prix]>" + b + "[Pop][1]", Style: Style{"b": true}}}))
	_, err = d.CreateButton(ctx, 1, 6, 0, ButtonConfig{Kind: "select", Label: "x", OptionsFrom: b + "[Ville]"})
	must(t, err)
	_, err = d.SaveQuery(ctx, QueryDef{Name: "q", Source: `{"kind":"table","table":"` + b + `"}`, Steps: `[]`})
	must(t, err)
	before := dump(t, d)
	must(t, d.RenameTable(ctx, b, "Villes"))
	mustSet(t, d, 1, 0, 1, "PU")
	mustSet(t, d, 1, 0, 4, "Cité")
	rules := sqlText(t, d, `SELECT rules FROM _lattis_columns WHERE rules IS NOT NULL`)
	if !strings.Contains(rules, "[PU]>Villes[Pop][1]") {
		t.Fatalf("règle : %s", rules)
	}
	if cfg := sqlText(t, d, `SELECT config FROM _lattis_buttons`); !strings.Contains(cfg, `"optionsFrom":"Villes[Cité]"`) {
		t.Fatalf("contrôle : %s", cfg)
	}
	q, err := d.QueryByName(ctx, "q")
	must(t, err)
	if !strings.Contains(q.Source, `"table":"Villes"`) || !strings.Contains(q.Steps, `"column":"Cité","name":"Ville"`) {
		t.Fatalf("requête : %s %s", q.Source, q.Steps)
	}
	for range 3 {
		if _, err := d.bus.Undo(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
}
