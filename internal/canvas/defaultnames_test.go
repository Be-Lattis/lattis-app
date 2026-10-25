package canvas

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// sheetNames : les noms des onglets, dans l'ordre.
func sheetNames(t *testing.T, d *fixture) []string {
	t.Helper()
	l, err := d.Layout(context.Background(), 0)
	must(t, err)
	var out []string
	for _, s := range l.Sheets {
		out = append(out, s.Name)
	}
	return out
}

// Noms par défaut et libellés des TCD dans la langue de la session
// (docs/i18n.md, § 7) : un TCD créé en anglais s'appelle PivotTable1 et
// écrit « Sum of Ventes », pour toujours (Lang) ; un TCD d'avant, ou créé en
// français, garde « Somme de Ventes » et TCDn, quelle que soit la langue ;
// les formules qui citent ses colonnes se résolvent toujours.
func TestDefaultNamesInLanguage(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Pays", "Ventes"}, {"FR", "10"}, {"IT", "5"}, {"FR", "7"}}); err != nil {
		t.Fatal(err)
	}
	cfg := PivotConfig{Source: "Feuille1", RowFields: []string{"Pays"}, Values: []PivotValue{{Field: "Ventes", Agg: "sum"}}}
	// Français : comme avant.
	frSheet, err := d.CreatePivot(ctx, cfg)
	must(t, err)
	fr := onePivot(t, d, frSheet)
	if fr.Config.Name != "TCD1" || fr.Config.Lang != "" || fr.Cells[0][1] != "Somme de Ventes" {
		t.Fatalf("TCD français : %q %q %v", fr.Config.Name, fr.Config.Lang, fr.Cells[0])
	}
	speak(d, "en")
	enSheet, err := d.CreatePivot(ctx, cfg)
	must(t, err)
	en := onePivot(t, d, enSheet)
	if en.Config.Name != "PivotTable1" || en.Config.Lang != "en" || en.Cells[0][1] != "Sum of Ventes" {
		t.Fatalf("TCD anglais : %q %q %v", en.Config.Name, en.Config.Lang, en.Cells[0])
	}
	if names := sheetNames(t, d); names[len(names)-1] != "PivotTable 1" {
		t.Fatalf("feuille du TCD : %v", names)
	}
	// L'ancien TCD ne change pas en anglais ; une formule le cite toujours.
	if got := onePivot(t, d, frSheet).Cells[0][1]; got != "Somme de Ventes" {
		t.Fatalf("TCD français vu en anglais : %q", got)
	}
	mustSet(t, d, frSheet, 1, 3, "half")
	mustSet(t, d, frSheet, 2, 3, "=ROUND([Somme de Ventes]/2, 0)")
	if got := onePivot(t, d, frSheet).Cells[1][2]; got != "9" {
		t.Fatalf("formule sur « Somme de Ventes » : %q", got)
	}
	mustSet(t, d, enSheet, 1, 3, "half")
	mustSet(t, d, enSheet, 2, 3, "=ROUND([Sum of Ventes]/2, 0)")
	if got := onePivot(t, d, enSheet).Cells[1][2]; got != "9" {
		t.Fatalf("formule sur « Sum of Ventes » : %q", got)
	}
	// Repassé en français : le TCD anglais garde ses libellés et ses formules.
	speak(d, "fr")
	if p := onePivot(t, d, enSheet); p.Cells[0][1] != "Sum of Ventes" || p.Cells[1][2] != "9" {
		t.Fatalf("TCD anglais vu en français : %v", p.Cells)
	}
	// Le volet renvoie la configuration sans sa langue : elle reste.
	avg := cfg
	avg.Values = []PivotValue{{Field: "Ventes", Agg: "avg", Show: "pctTotal"}}
	must(t, d.SetPivot(ctx, en.ID, avg))
	if p := onePivot(t, d, enSheet); p.Config.Lang != "en" || p.Cells[0][1] != "Average of Ventes (% of total)" {
		t.Fatalf("TCD anglais modifié : %q %v", p.Config.Lang, p.Cells[0])
	}
	// Scripts et assistant (forme gardée) : en français, quelle que soit la langue.
	speak(d, "en")
	sSheet, err := d.CreatePivot(StorageForm(ctx), cfg)
	must(t, err)
	if p := onePivot(t, d, sSheet); p.Config.Name != "TCD2" || p.Config.Lang != "" || p.Cells[0][1] != "Somme de Ventes" {
		t.Fatalf("TCD d'un script : %q %q %v", p.Config.Name, p.Config.Lang, p.Cells[0])
	}
	// Nouvelle feuille, nouveau tableau : dans la langue de la session.
	sheet, err := d.AddSheet(ctx)
	must(t, err)
	if names := sheetNames(t, d); !strings.HasPrefix(names[len(names)-1], "Sheet") {
		t.Fatalf("nouvelle feuille : %v", names)
	}
	if _, err := d.Paste(ctx, sheet, 0, 0, [][]string{{"A", "B"}, {"1", "2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Paste(ctx, sheet, 0, 5, [][]string{{"C", "D"}, {"1", "2"}}); err != nil {
		t.Fatal(err)
	}
	names, err := d.TableNames(ctx)
	must(t, err)
	if _, ok := names["Table1"]; !ok {
		t.Fatalf("tableaux : %v", names)
	}
	speak(d, "de")
	if _, err := d.AddSheet(ctx); err != nil {
		t.Fatal(err)
	}
	if names := sheetNames(t, d); !strings.HasPrefix(names[len(names)-1], "Tabelle") {
		t.Fatalf("neue Tabelle : %v", names)
	}
}

// Regroupement d'onglets en anglais : onglets January…March (reconnus dans
// toutes les langues), colonne de catégorie « Month » et valeurs en anglais.
func TestMergeSheetsInLanguage(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	speak(d, "en")
	var tables []string
	for _, s := range []string{"January", "April", "May"} {
		tables = append(tables, tableOn(t, d, sheetWith(t, d, s, [][]string{{"Client", "Amount"}, {"A " + s, "1"}})))
	}
	gs := similar(t, d)
	if len(gs) != 1 || gs[0].Category.Column != "Month" || !slices.Equal(gs[0].Category.Labels, []string{"January", "April", "May"}) {
		t.Fatalf("groupe : %+v", gs)
	}
	p, err := d.MergePreview(ctx, MergePlan{Tables: tables})
	must(t, err)
	if p.Plan.Category == nil || p.Plan.Category.Name != "Month" || p.Rows[1][0] != "April" || p.Header[0] != "Month" {
		t.Fatalf("aperçu : %+v %v %v", p.Plan.Category, p.Header, p.Rows)
	}
	if p.Plan.Name == "" || strings.Contains(p.Plan.Name, "Regroupement") {
		t.Fatalf("nom proposé : %q", p.Plan.Name)
	}
}
