package canvas

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// cellValues : les valeurs des cellules libres d'une feuille, par adresse.
func cellValues(t *testing.T, d *fixture, sheet int64) map[string]string {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, c := range l.Cells {
		out[fmt.Sprintf("%d,%d", c.Row, c.Col)] = c.Value
	}
	return out
}

func cellRaws(t *testing.T, d *fixture, sheet int64) map[string]string {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, c := range l.Cells {
		out[fmt.Sprintf("%d,%d", c.Row, c.Col)] = c.Raw
	}
	return out
}

// pivotCells : les cases du premier TCD de la feuille.
func pivotCells(t *testing.T, d *fixture, sheet int64) string {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	if err != nil || len(l.Pivots) == 0 {
		t.Fatalf("TCD de la feuille %d : %v %+v", sheet, err, l.Pivots)
	}
	p := l.Pivots[0]
	if p.Error != "" {
		t.Fatalf("TCD : %s", p.Error)
	}
	return fmt.Sprint(p.Cells)
}

// chartText : les étiquettes et séries du premier graphique de la feuille.
func chartText(t *testing.T, d *fixture, sheet int64) string {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	if err != nil || len(l.Charts) == 0 {
		t.Fatalf("graphique de la feuille %d : %v", sheet, err)
	}
	ch := l.Charts[0]
	if ch.Error != "" {
		t.Fatalf("graphique : %s", ch.Error)
	}
	var b strings.Builder
	fmt.Fprint(&b, ch.Labels)
	for _, s := range ch.Series {
		b.WriteString(" " + s.Name + ":")
		for _, v := range s.Values {
			if v == nil {
				b.WriteString(" -")
			} else {
				fmt.Fprintf(&b, " %g", *v)
			}
		}
	}
	return b.String()
}

// Lot 2 (§8) : une synthèse, une colonne calculée, une règle, un TCD et un
// graphique qui citaient les onglets mensuels sont repris sur le tableau
// commun filtré par le mois, avec les mêmes valeurs ; une ligne précise
// reste à reprendre (anciens onglets alors masqués) ; une seule annulation.
func TestMergeSheetsRewrite(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	data := map[string][][]string{
		"Janvier": {{"Client", "Montant", "Qté", "Ville"}, {"Durand", "10", "2", "Paris"}, {"Martin", "20", "1", ""}, {"Durand", "5", "3", "Lyon"}},
		"Février": {{"Client", "Montant", "Qté", "Ville"}, {"Dupont", "12", "1", "Paris"}, {"Durand", "-4", "2", "Paris"}},
		"Mars":    {{"Client", "Montant €", "Qté", "Ville"}, {"Petit", "7", "5", "Nice"}},
	}
	months := []string{"Janvier", "Février", "Mars"}
	tabs := map[string]string{}
	var sources []string
	for _, m := range months {
		id := sheetWith(t, d, m, data[m])
		tabs[m] = tableOn(t, d, id)
		sources = append(sources, tabs[m])
	}
	syn := sheetWith(t, d, "Synthèse", nil)
	j, f, m := tabs["Janvier"], tabs["Février"], tabs["Mars"]
	formulas := []string{
		// Agrégats d'une colonne : .SI.ENS.
		"=SOMME(" + j + "[Montant])", "=SOMME(" + f + "[Montant])", "=SOMME(" + m + "[Montant €])",
		"=MOYENNE(" + j + "[Montant])", "=MAX(" + f + "[Montant])", "=MIN(" + j + "[Montant])",
		"=NBVAL(" + j + "[Ville])", "=NBVAL(" + f + "[Client])", "=NB(" + j + "[Montant])",
		// Expression sur les colonnes, critères, nombre de lignes.
		"=SOMME(" + j + "[Montant]*" + j + "[Qté])", "=SOMME.SI(" + j + "[Client]; \"Durand\"; " + j + "[Montant])",
		"=NB.SI(" + f + "[Ville]; \"Paris\")", "=SOMME.SI.ENS(" + j + "[Montant]; " + j + "[Ville]; \"Paris\"; " + j + "[Qté]; \">1\")",
		"=MOYENNE.SI(" + j + "[Client]; \"Durand\"; " + j + "[Qté])", "=LIGNES(" + j + ")", "=LIGNES(" + f + "[Client])",
		// Plusieurs colonnes, autres agrégats, tableaux dynamiques.
		"=SOMME(" + j + "[Montant]; " + f + "[Montant])", "=MAX(" + j + "[Montant]; " + f + "[Montant])",
		"=MEDIANE(" + j + "[Montant])", "=JOINDRE.TEXTE(\", \"; VRAI; " + j + "[Client])",
		"=NBVAL(UNIQUE(" + j + "[Client]))", "=SOMME(FILTRE(" + j + "[Montant]; " + j + "[Qté]>1))",
		"=SOMME(" + j + "[Montant])/SOMME(" + j + "[Montant]; " + f + "[Montant]; " + m + "[Montant €])",
		// Une ligne précise : à reprendre.
		"=SOMME(" + j + "[Montant][1:2])",
	}
	for i, fm := range formulas {
		mustSet(t, d, syn, i, 0, fm)
	}
	// Une colonne calculée et une règle d'un autre tableau.
	cib := sheetWith(t, d, "Cibles", [][]string{{"Nom", "Cible"}, {"A", "30"}, {"B", "50"}})
	cibles := tableOn(t, d, cib)
	mustSet(t, d, cib, 0, 2, "Écart")
	mustSet(t, d, cib, 1, 2, "=SOMME("+j+"[Montant])-[Cible]")
	must(t, d.SetRules(ctx, cibles, 1, []Rule{{Op: "formula", Value: "=[Cible]>SOMME(" + j + "[Montant])", Style: Style{"b": true}}}))
	// Un TCD sur Janvier, un graphique sur Mars (colonne appariée).
	tcd, err := d.CreatePivot(ctx, PivotConfig{Source: j, RowFields: []string{"Client"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}})
	must(t, err)
	_, err = d.CreateChart(ctx, syn, 30, 0, ChartConfig{Source: m, Kind: "bar", X: "Client", Series: []string{"Montant €"}})
	must(t, err)

	vals := cellValues(t, d, syn)
	cibVals := cellValues(t, d, cib)
	cibRows := tableRows(t, d, cibles)
	pivot := pivotCells(t, d, tcd)
	chart := chartText(t, d, syn)
	if vals["0,0"] != "35" || vals["23,0"] != "30" || chart != "[Petit] Montant €: 7" {
		t.Fatalf("avant : %v, graphique %s", vals, chart)
	}
	before := stripStaging(dump(t, d))

	p, err := d.MergePreview(ctx, MergePlan{Tables: sources, Name: "Ventes"})
	must(t, err)
	if len(p.Pending) != 1 || p.Pending[0].Where != "Synthèse A24" || p.Pending[0].Why == "" || p.Old != MergeOldHide {
		t.Fatalf("à reprendre : %+v (%s)", p.Pending, p.Old)
	}
	if n := len(formulas) - 1 + 4; len(p.Rewritten) != n {
		var ws []string
		for _, r := range p.Rewritten {
			ws = append(ws, r.Where)
		}
		t.Fatalf("repris %d, attendu %d : %v", len(p.Rewritten), n, ws)
	}
	if r := p.Rewritten[0]; r.Where != "Synthèse A1" || r.Formula != formulas[0] || r.After != `=SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Janvier")` {
		t.Fatalf("A1 : %+v", r)
	}

	res, err := d.MergeSheets(ctx, MergePlan{Tables: sources, Name: "Ventes"})
	must(t, err)
	check(t, d, "reprise des formules")
	if res.Old != MergeOldHide || len(res.Pending) != 1 || len(res.Rewritten) != len(p.Rewritten) ||
		!strings.Contains(res.Notice, fmt.Sprintf("%d formules reprises, une à reprendre à la main", len(p.Rewritten))) {
		t.Fatalf("résultat : %s %+v", res.Notice, res.Pending)
	}
	if !strings.Contains(strings.Join(res.Notes, " "), "tableau croisé sur « Ventes »") {
		t.Errorf("suggestion d'un TCD : %v", res.Notes)
	}
	// Mêmes valeurs ; les formules reprises ne citent plus les anciens
	// tableaux, celle à reprendre cite l'ancien tableau masqué.
	after := cellValues(t, d, syn)
	raws := cellRaws(t, d, syn)
	for i := range formulas {
		k := fmt.Sprintf("%d,0", i)
		if after[k] != vals[k] {
			t.Errorf("ligne %d : %s → %s donne %q, attendu %q", i+1, formulas[i], raws[k], after[k], vals[k])
		}
		old := strings.Contains(raws[k], "_ancien")
		if old != (i == len(formulas)-1) || !old && !strings.Contains(raws[k], "Ventes[") {
			t.Errorf("ligne %d : %s", i+1, raws[k])
		}
	}
	wantRaw := map[int]string{
		6:  `=NB.SI.ENS(Ventes[Ville]; "<>"; Ventes[Mois]; "=Janvier")`,
		8:  `=NB(FILTRE(Ventes[Montant]; Ventes[Mois]="Janvier"))`,
		2:  `=SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Mars")`,
		10: `=SOMME.SI.ENS(Ventes[Montant]; Ventes[Client]; "Durand"; Ventes[Mois]; "=Janvier")`,
		14: `=NB.SI.ENS(Ventes[Mois]; "=Janvier")`,
		16: `=SOMME(SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Janvier"); SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Février"))`,
	}
	for r, w := range wantRaw {
		if got := raws[fmt.Sprintf("%d,0", r)]; got != w {
			t.Errorf("ligne %d : %s, attendu %s", r+1, got, w)
		}
	}
	if got := tableRows(t, d, cibles); !slices.Equal(got, cibRows) {
		t.Errorf("colonne calculée : %v, attendu %v", got, cibRows)
	}
	if got := cellValues(t, d, cib); fmt.Sprint(got) != fmt.Sprint(cibVals) {
		t.Errorf("Cibles : %v", got)
	}
	if rules := sqlText(t, d, `SELECT rules FROM _lattis_columns c JOIN _lattis_tables t ON t.id = c.tbl WHERE t.name = ? AND c.name = 'Cible'`, cibles); !strings.Contains(rules, `SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; \"=Janvier\")`) || !strings.Contains(rules, `SUMIFS(Ventes[Montant], Ventes[Mois], \"=Janvier\")`) {
		t.Errorf("règle : %s", rules)
	}
	if got := pivotCells(t, d, tcd); got != pivot {
		t.Errorf("TCD : %s, attendu %s", got, pivot)
	}
	// La série prend le nom de la colonne du tableau commun.
	if got := chartText(t, d, syn); got != strings.ReplaceAll(chart, "Montant €", "Montant") {
		t.Errorf("graphique : %s, attendu %s", got, chart)
	}
	l, _ := d.Layout(ctx, syn)
	if c := l.Charts[0].Config; c.Source != "Ventes" || c.Series[0] != "Montant" || len(c.Filters) != 1 || c.Filters[0].Field != "Mois" || c.Filters[0].Values[0] != "Mars" {
		t.Errorf("graphique : %+v", c)
	}

	// Une seule annulation rend tout ; rétablir refait la reprise.
	_, err = d.bus.Undo(ctx)
	must(t, err)
	if got := stripStaging(dump(t, d)); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
	check(t, d, "annulation")
	_, err = d.bus.Redo(ctx)
	must(t, err)
	if got := cellValues(t, d, syn); fmt.Sprint(got) != fmt.Sprint(after) {
		t.Fatalf("rétablissement : %v", got)
	}
}

// Sans rien à reprendre à la main, les anciens onglets sont supprimés (§12.1)
// et la synthèse cite le tableau commun ; catégories entières et dates.
func TestMergeSheetsRewriteDelete(t *testing.T) {
	ctx := context.Background()
	for _, names := range [][]string{{"Ventes 2023", "Ventes 2024"}, {"Janv 2025", "Févr 2025"}, {"Lot 1", "Lot 2"}} {
		d := open(t)
		a := sheetWith(t, d, names[0], [][]string{{"Client", "Montant"}, {"A", "10"}, {"B", "1"}})
		b := sheetWith(t, d, names[1], [][]string{{"Client", "Montant"}, {"B", "20"}})
		syn := sheetWith(t, d, "Synthèse", nil)
		ta, tb := tableOn(t, d, a), tableOn(t, d, b)
		mustSet(t, d, syn, 0, 0, "=SOMME("+ta+"[Montant])+SOMME("+tb+"[Montant])")
		mustSet(t, d, syn, 1, 0, "=NB.SI("+ta+"[Client]; \"B\")")
		mustSet(t, d, syn, 2, 0, "=MOYENNE("+tb+"[Montant])")
		mustSet(t, d, syn, 3, 0, "=NB("+ta+"[Montant])")
		tcd, err := d.CreatePivot(ctx, PivotConfig{Source: tb, RowFields: []string{"Client"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}})
		must(t, err)
		vals, pivot := cellValues(t, d, syn), pivotCells(t, d, tcd)
		res, err := d.MergeSheets(ctx, MergePlan{Tables: []string{ta, tb}, Name: "Ventes"})
		must(t, err)
		check(t, d, "suppression")
		if res.Old != MergeOldDelete || len(res.Pending) != 0 || len(res.Rewritten) != 5 {
			t.Fatalf("%v : %s %+v", names, res.Old, res.Pending)
		}
		if got := cellValues(t, d, syn); fmt.Sprint(got) != fmt.Sprint(vals) {
			t.Errorf("%v : %v, attendu %v (%v)", names, got, vals, cellRaws(t, d, syn))
		}
		if got := pivotCells(t, d, tcd); got != pivot {
			t.Errorf("%v : TCD %s, attendu %s", names, got, pivot)
		}
		l, _ := d.Layout(ctx, syn)
		hidden := 0
		for _, s := range l.Sheets {
			if s.Hidden {
				hidden++
			}
		}
		if hidden != 0 || len(l.Sheets) != 3 {
			t.Errorf("%v : onglets %+v", names, l.Sheets)
		}
	}
}

// Sans colonne de catégorie (cas F), rien ne distingue les onglets : tout
// reste à reprendre.
func TestMergeSheetsRewriteNoCategory(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	a := sheetWith(t, d, "Ventes 2023", [][]string{{"Client", "Montant"}, {"A", "10"}})
	b := sheetWith(t, d, "Ventes 2024", [][]string{{"Client", "Montant"}, {"B", "20"}})
	syn := sheetWith(t, d, "Synthèse", nil)
	ta, tb := tableOn(t, d, a), tableOn(t, d, b)
	mustSet(t, d, syn, 0, 0, "=SOMME("+ta+"[Montant])")
	p, err := d.MergePreview(ctx, MergePlan{Tables: []string{ta, tb}, NoCategory: true})
	must(t, err)
	if len(p.Pending) != 1 || len(p.Rewritten) != 0 || !strings.Contains(p.Pending[0].Why, "catégorie") || p.Old != MergeOldHide {
		t.Fatalf("sans catégorie : %+v %+v", p.Pending, p.Rewritten)
	}
}

// Le tableau commun prend le nom d'un ancien tableau, gardé masqué (une
// formule reste à reprendre) : l'ancien devient Ventes_ancien sans emporter
// les formules reprises ; les graphiques et TCD d'un Document suivent.
func TestMergeSheetsRewriteSameName(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	a := sheetWith(t, d, "Ventes 2023", [][]string{{"Client", "Montant"}, {"A", "10"}, {"C", "4"}})
	b := sheetWith(t, d, "Ventes 2024", [][]string{{"Client", "Montant"}, {"B", "20"}})
	syn := sheetWith(t, d, "Synthèse", nil)
	must(t, d.RenameTable(ctx, tableOn(t, d, a), "Ventes"))
	tb := tableOn(t, d, b)
	mustSet(t, d, syn, 0, 0, "=SOMME(Ventes[Montant])")
	mustSet(t, d, syn, 1, 0, "=SOMME("+tb+"[Montant])")
	mustSet(t, d, syn, 2, 0, "=Ventes[Montant][2]")
	doc, err := d.AddDoc(ctx, 0)
	must(t, err)
	src := `{"ops":[{"insert":"Bilan\n"},{"insert":{"lattisChart":{"source":"Ventes","kind":"bar","x":"Client","series":["Montant"]}}},` +
		`{"insert":{"lattisPivot":{"source":"Ventes","rowFields":["Client"],"values":[{"field":"Montant","agg":"sum"}]}}},{"insert":"\n"}]}`
	must(t, d.SetPage(ctx, doc, src))
	vals := cellValues(t, d, syn)
	res, err := d.MergeSheets(ctx, MergePlan{Tables: []string{"Ventes", tb}, Name: "Ventes"})
	must(t, err)
	check(t, d, "même nom")
	raws := cellRaws(t, d, syn)
	if res.Table != "Ventes" || res.Old != MergeOldHide || len(res.Pending) != 1 || len(res.Rewritten) != 3 {
		t.Fatalf("résultat : %+v", res)
	}
	if raws["0,0"] != `=SOMME.SI.ENS(Ventes[Montant]; Ventes[Année]; 2023)` || raws["2,0"] != "=Ventes_ancien[Montant][2]" {
		t.Fatalf("formules : %v", raws)
	}
	if got := cellValues(t, d, syn); fmt.Sprint(got) != fmt.Sprint(vals) {
		t.Fatalf("valeurs : %v, attendu %v", got, vals)
	}
	page := sqlText(t, d, `SELECT source FROM _lattis_pages WHERE sheet = ?`, doc)
	if strings.Count(page, `"source":"Ventes"`) != 2 || strings.Count(page, `"filters":[{"field":"Année","values":["2023"]}]`) != 2 {
		t.Fatalf("Document : %s", page)
	}
	ch := d.ChartData(ctx, ChartConfig{Source: "Ventes", Kind: "bar", X: "Client", Series: []string{"Montant"}, Filters: []PivotFilter{{Field: "Année", Values: []string{"2023"}}}})
	if ch.Error != "" || fmt.Sprint(ch.Labels) != "[A C]" {
		t.Fatalf("graphique du Document : %+v", ch)
	}
}

// Une colonne calculée qui lit tout le tableau de son onglet (la part du
// total du mois) changerait de sens sur le tableau commun : elle garde ses
// valeurs.
func TestMergeSheetsComputedWholeTable(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	var tables []string
	for i, s := range []string{"Janvier", "Février", "Mars"} {
		id := sheetWith(t, d, s, [][]string{{"Client", "Montant", "Part"}, {"A", fmt.Sprint(i + 1), "0"}, {"B", "1", "0"}})
		tb := tableOn(t, d, id)
		mustSet(t, d, id, 1, 2, "=[Montant]/SOMME("+tb+"[Montant])")
		tables = append(tables, tb)
	}
	res, err := d.MergeSheets(ctx, MergePlan{Tables: tables, Name: "Ventes"})
	must(t, err)
	l, _ := d.Layout(ctx, res.Sheet)
	if f := l.Tables[0].Columns[3].Formula; f != "" || !strings.Contains(strings.Join(res.Notes, " "), "tout le tableau de son onglet") {
		t.Fatalf("formule %q, notes %v", f, res.Notes)
	}
	if rows := tableRows(t, d, "Ventes"); len(rows) != 6 || !strings.HasPrefix(rows[2], "Février|A|2|0.666") {
		t.Fatalf("lignes : %v", rows)
	}
}
