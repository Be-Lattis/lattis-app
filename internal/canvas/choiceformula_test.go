package canvas

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// formulaSheet : la feuille 1 porte le tableau Feuille1 (Date, Produit,
// Montant) sur deux années ; des formules libres à droite (colonne 6).
func formulaSheet(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Date", "Produit", "Montant"}, {"15/03/2024", "A", "10"}, {"02/07/2024", "B", "5"}, {"10/01/2025", "A", "7"}, {"20/05/2025", "C", "3"}}); err != nil {
		t.Fatal(err)
	}
	if err := promoteIfData(ctx, d, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	for i, f := range []string{`=SOMME(Feuille1[Montant])`, `=ONGLET.CHOIX()`, `=ONGLET.CHOIX()+1`} {
		if _, err := d.SetCell(ctx, 1, i, 6, f); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func choiceTexts(vals []FilterValue) string {
	var out []string
	for _, v := range vals {
		out = append(out, fmt.Sprintf("%s:%d", v.Text, v.Count))
	}
	return strings.Join(out, " ")
}

// TestChoiceFormulaPreview : l'aperçu de « Choisir par › Une formule… »
// donne les valeurs et les tableaux qui suivraient, ou l'erreur.
func TestChoiceFormulaPreview(t *testing.T) {
	ctx := context.Background()
	d := formulaSheet(t)
	p, err := d.PreviewChoiceFormula(ctx, 1, "=ANNEE([Date])")
	if err != nil {
		t.Fatal(err)
	}
	if p.Error != "" || choiceTexts(p.Values) != "2024:2 2025:2" || p.Count != 2 || fmt.Sprint(p.Tables) != "[Feuille1]" {
		t.Fatalf("aperçu : %+v", p)
	}
	p, _ = d.PreviewChoiceFormula(ctx, 1, `SI([Montant]>6;"Gros";"Petit")`)
	if p.Error != "" || choiceTexts(p.Values) != "Gros:2 Petit:2" {
		t.Fatalf("aperçu SI : %+v", p)
	}
	for _, src := range []string{"=ANNEE([Jour])", "=1+1", "=ANNEE(", ""} {
		if p, _ := d.PreviewChoiceFormula(ctx, 1, src); p.Error == "" {
			t.Fatalf("%q : pas d'erreur (%+v)", src, p)
		}
	}
}

// TestChoiceFormula : l'onglet à choix par une formule filtre le tableau qui
// a ses colonnes, les formules de la feuille (ONGLET.CHOIX() rend un nombre
// pour =ANNEE) et les TCD de la feuille ; une ligne ajoutée reste affichée
// jusqu'au changement de valeur ; tout s'annule.
func TestChoiceFormula(t *testing.T) {
	ctx := context.Background()
	d := formulaSheet(t)
	if _, err := d.Paste(ctx, 1, 0, 9, [][]string{{"Code", "Prix"}, {"X", "1"}, {"Y", "2"}}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	if err := d.SetSheetChoiceFormula(ctx, 1, "Année", "=ANNEE([Date])", "2025"); err != nil {
		t.Fatal(err)
	}
	l, _ := d.Layout(ctx, 1)
	ch := l.Sheets[0].Choice
	if ch == nil || ch.Name != "Année" || ch.Value != "2025" || ch.Formula != "=ANNEE([Date])" || ch.Table != "" {
		t.Fatalf("choix : %+v", ch)
	}
	var ventes, autre TableInfo
	for _, tb := range l.Tables {
		if tb.Name == "Feuille1" {
			ventes = tb
		} else {
			autre = tb
		}
	}
	if ventes.Rows != 2 || ventes.Total != 4 || !ventes.Follows || !ventes.ByFormula {
		t.Fatalf("tableau : %+v", ventes)
	}
	if autre.Follows || autre.Rows != autre.Total {
		t.Fatalf("tableau sans Date : %+v", autre)
	}
	if got := freeValues(t, d, 1, 6); got != "10|2025|2026" {
		t.Fatalf("formules de la feuille : %s", got)
	}
	vals, err := d.SheetChoiceValues(ctx, 1)
	if err != nil || choiceTexts(vals) != "2024:2 2025:2" {
		t.Fatalf("valeurs : %s %v", choiceTexts(vals), err)
	}
	// TCD de la feuille : le filtre de l'onglet passe par la formule.
	if _, err := d.PlacePivot(ctx, 1, 10, 0, PivotConfig{Source: "Feuille1", RowFields: []string{"Produit"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}}); err != nil {
		t.Fatal(err)
	}
	l, _ = d.Layout(ctx, 1)
	if got := fmt.Sprint(l.Pivots[0].Cells); got != "[[Produit Somme de Montant] [A 7] [C 3] [Total 10]]" {
		t.Fatalf("TCD : %s", got)
	}
	// Une ligne saisie sous le tableau (sans date) reste affichée.
	if _, err := d.SetCell(ctx, 1, 5, 1, "D"); err != nil {
		t.Fatal(err)
	}
	if shown, total := rowsOf(t, d); shown != 3 || total != 5 {
		t.Fatalf("ligne ajoutée : %d/%d", shown, total)
	}
	if _, err := d.SetCell(ctx, 1, 3, 2, "4"); err != nil { // la même ligne, rang 3
		t.Fatal(err)
	}
	if n := countWhere(t, d, `c_2 = 'D' AND c_3 = 4`); n != 1 {
		t.Fatalf("saisie dans la ligne gardée : %d", n)
	}
	// « Nouvelle valeur » : pas pour une catégorie calculée.
	if err := d.NewSheetChoiceValue(ctx, 1, "2026"); err == nil {
		t.Fatal("nouvelle valeur acceptée")
	}
	// Changer de valeur : la ligne sans date n'est plus gardée.
	if err := d.ChooseSheetValue(ctx, 1, "2024", false); err != nil {
		t.Fatal(err)
	}
	if shown, total := rowsOf(t, d); shown != 2 || total != 5 {
		t.Fatalf("2024 : %d/%d", shown, total)
	}
	if err := d.ChooseSheetValue(ctx, 1, "2025", false); err != nil {
		t.Fatal(err)
	}
	if shown, _ := rowsOf(t, d); shown != 2 {
		t.Fatalf("2025 sans la ligne gardée : %d", shown)
	}
	// Renommer la colonne Date : la formule suit.
	if _, err := d.SetCell(ctx, 1, 0, 0, "Jour"); err != nil {
		t.Fatal(err)
	}
	if ch, _ := d.SheetChoice(ctx, 1); ch == nil || ch.Formula != "=ANNEE([Jour])" {
		t.Fatalf("après renommage : %+v", ch)
	}
	if shown, _ := rowsOf(t, d); shown != 2 {
		t.Fatalf("après renommage : %d", shown)
	}
	// « Ne plus suivre » puis « Suivre » ; un tableau sans les colonnes refuse.
	if _, err := d.SetTableFollow(ctx, "Feuille1", false); err != nil {
		t.Fatal(err)
	}
	if shown, total := rowsOf(t, d); shown != total {
		t.Fatalf("ne suit plus : %d/%d", shown, total)
	}
	if _, err := d.SetTableFollow(ctx, "Feuille1", true); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetTableFollow(ctx, autre.Name, true); err == nil {
		t.Fatal("tableau sans les colonnes : suivi accepté")
	}
	check(t, d, "choix par formule")
	for range 7 { // suivre, ne plus suivre, renommer, saisie, saisie, TCD, choisir par
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
}

// TestChoiceFormulaRefused : nom vide, formule sans colonne, aucune table
// qui ait ses colonnes.
func TestChoiceFormulaRefused(t *testing.T) {
	ctx := context.Background()
	d := formulaSheet(t)
	for _, c := range [][2]string{{"", "=ANNEE([Date])"}, {"X", "=1"}, {"X", "=ANNEE([Jour])"}} {
		if err := d.SetSheetChoiceFormula(ctx, 1, c[0], c[1], ""); err == nil {
			t.Fatalf("%v accepté", c)
		}
	}
}

// TestSortChoiceValues : nombres croissants, mois dans l'ordre du
// calendrier, textes dans l'ordre naturel.
func TestSortChoiceValues(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"2025 2024 10 9", "9 10 2024 2025"},
		{"mars janvier décembre", "janvier mars décembre"},
		{"Lot 10|Lot 2|lot 1|Autre", "Autre lot 1 Lot 2 Lot 10"},
		{"Petit Gros", "Gros Petit"},
	} {
		sep := " "
		if strings.Contains(c.in, "|") {
			sep = "|"
		}
		var vals []FilterValue
		for _, s := range strings.Split(c.in, sep) {
			vals = append(vals, FilterValue{Text: s})
		}
		sortChoiceValues(vals)
		var got []string
		for _, v := range vals {
			got = append(got, v.Text)
		}
		if strings.Join(got, " ") != c.want {
			t.Fatalf("%q : %q", c.in, strings.Join(got, " "))
		}
	}
}

// TestPivotCalcField : un champ calculé regroupe les lignes d'un TCD par le
// résultat de sa formule, en lignes, en colonnes et en filtre ; il se
// renomme, se retire et suit le renommage d'une colonne de la source.
func TestPivotCalcField(t *testing.T) {
	ctx := context.Background()
	d := formulaSheet(t)
	cfg := PivotConfig{Source: "Feuille1", Calc: []PivotCalcField{{Name: "An", Text: "=ANNEE([Date])"}}, RowFields: []string{"An"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}}
	id, err := d.CreatePivotAt(ctx, 1, 10, 0, cfg)
	if err != nil {
		t.Fatal(err)
	}
	pivot := func() PivotInfo {
		l, _ := d.Layout(ctx, 1)
		for _, p := range l.Pivots {
			if p.ID == id {
				return p
			}
		}
		t.Fatal("TCD introuvable")
		return PivotInfo{}
	}
	p := pivot()
	if got := fmt.Sprint(p.Cells); got != "[[An Somme de Montant] [2024 15] [2025 10] [Total 25]]" || p.Error != "" {
		t.Fatalf("par année : %s %s", got, p.Error)
	}
	if len(p.Config.Calc) != 1 || p.Config.Calc[0].Text != "=ANNEE([Date])" || p.Config.Calc[0].Formula != "=YEAR([Date])" {
		t.Fatalf("champ calculé : %+v", p.Config.Calc)
	}
	// En colonnes, avec un filtre sur un autre champ calculé.
	cfg = p.Config
	cfg.Calc = append(cfg.Calc, PivotCalcField{Name: "Taille", Text: `=SI([Montant]>6;"Gros";"Petit")`})
	cfg.RowFields, cfg.Cols = []string{"Produit"}, "An"
	cfg.Filters = []PivotFilter{{Field: "Taille", Values: []string{"Gros"}}}
	cfg.Sort = map[string]string{"An": "desc"}
	if err := d.SetPivot(ctx, id, cfg); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(pivot().Cells); got != "[[Produit 2025 2024 Total] [A 7 10 17] [Total 7 10 17]]" {
		t.Fatalf("en colonnes : %s", got)
	}
	// Renommer le champ : il garde sa place ; le retirer : il sort des zones.
	if err := d.SetPivotCalcField(ctx, id, "An", "Année", "=ANNEE([Date])"); err != nil {
		t.Fatal(err)
	}
	if p := pivot(); p.Config.Cols != "Année" || p.Config.Sort["Année"] != "desc" {
		t.Fatalf("renommé : %+v", p.Config)
	}
	if err := d.SetPivotCalcField(ctx, id, "Taille", "", ""); err != nil {
		t.Fatal(err)
	}
	if p := pivot(); len(p.Config.Filters) != 0 || len(p.Config.Calc) != 1 {
		t.Fatalf("retiré : %+v", p.Config)
	}
	// Nom pris par une colonne, formule sans les colonnes : refusés.
	if err := d.SetPivotCalcField(ctx, id, "", "Produit", "=GAUCHE([Produit];1)"); err == nil {
		t.Fatal("nom d'une colonne accepté")
	}
	if err := d.SetPivotCalcField(ctx, id, "", "X", "=GAUCHE([Code];1)"); err == nil {
		t.Fatal("colonne absente acceptée")
	}
	// La source renomme sa colonne : la formule suit.
	if _, err := d.SetCell(ctx, 1, 0, 0, "Jour"); err != nil {
		t.Fatal(err)
	}
	if p := pivot(); p.Config.Calc[0].Text != "=ANNEE([Jour])" || p.Error != "" {
		t.Fatalf("source renommée : %+v %s", p.Config.Calc, p.Error)
	}
	// Aperçu d'un champ calculé.
	pv, err := d.PreviewPivotFormula(ctx, "Feuille1", "=GAUCHE([Produit];1)&\"-\"")
	if err != nil || choiceTexts(pv.Values) != "A-:2 B-:1 C-:1" {
		t.Fatalf("aperçu : %+v %v", pv, err)
	}
	check(t, d, "champ calculé")
}
