package canvas

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// monthlyBook : trois onglets par mois (noms par défaut Feuille1…3), chacun
// avec un titre « Janvier 2025 » en A1, un tableau des ventes en A3, une
// liste de tarifs recopiée telle quelle en E3, un total sous les ventes, un
// objectif qui change chaque mois (H1) et un tableau de notes du mois en
// A10 ; une feuille Synthèse cite les ventes de Janvier.
func monthlyBook(t *testing.T) (d *fixture, sheets []int64, ventes, tarifs, notes []string, syn int64) {
	t.Helper()
	ctx := context.Background()
	d = open(t)
	months := []string{"Janvier 2025", "Février 2025", "Mars 2025"}
	for i, m := range months {
		var id int64 = 1
		if i > 0 {
			var err error
			id, err = d.AddSheet(ctx)
			must(t, err)
		}
		sheets = append(sheets, id)
		mustSet(t, d, id, 0, 0, m)
		mustSet(t, d, id, 0, 6, "Objectif")
		mustSet(t, d, id, 0, 7, fmt.Sprint(5000+1000*i))
		_, err := d.Paste(ctx, id, 2, 0, [][]string{{"Client", "Montant"}, {"Durand", fmt.Sprint(10 * (i + 1))}, {"Martin", fmt.Sprint(i + 1)}})
		must(t, err)
		_, err = d.Paste(ctx, id, 2, 4, [][]string{{"Produit", "Prix"}, {"Pomme", "2"}, {"Poire", "3"}})
		must(t, err)
		_, err = d.Paste(ctx, id, 9, 0, [][]string{{"Jour", "Note"}, {fmt.Sprint(i + 1), "relance " + m}, {fmt.Sprint(i + 10), "inventaire"}})
		must(t, err)
		l, err := d.Layout(ctx, id)
		must(t, err)
		if len(l.Tables) != 3 {
			t.Fatalf("%s : tableaux %+v", m, l.Tables)
		}
		for _, tb := range l.Tables {
			switch {
			case tb.Row == 2 && tb.Col == 0:
				ventes = append(ventes, tb.Name)
			case tb.Row == 2:
				tarifs = append(tarifs, tb.Name)
			default:
				notes = append(notes, tb.Name)
			}
		}
		mustSet(t, d, id, 6, 0, "Total")
		mustSet(t, d, id, 6, 1, "=SOMME("+ventes[i]+"[Montant])")
	}
	var err error
	syn, err = d.AddSheet(ctx)
	must(t, err)
	must(t, d.RenameSheet(ctx, syn, "Synthèse"))
	mustSet(t, d, syn, 0, 0, "=SOMME("+ventes[0]+"[Montant])")
	return d, sheets, ventes, tarifs, notes, syn
}

func tableAt(t *testing.T, d *fixture, sheet int64, name string) TableInfo {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	must(t, err)
	for _, tb := range l.Tables {
		if tb.Name == name {
			return tb
		}
	}
	t.Fatalf("tableau %s introuvable : %+v", name, l.Tables)
	return TableInfo{}
}

// Lot 4 : une feuille par mois à plusieurs tableaux. La détection apparie
// les tableaux famille par famille (ventes et notes changent, les tarifs
// sont recopiés), propose la cellule A1 comme source du mois (les noms
// d'onglets sont ceux par défaut) ; le regroupement fait un tableau commun
// par famille (tarifs gardés une fois), à la place qu'ils avaient dans le
// dernier mois, reprend les cellules libres (le total une fois, l'objectif
// et le titre par mois), et l'onglet à choix filtre chaque tableau commun.
func TestMergeSheetFamilies(t *testing.T) {
	ctx := context.Background()
	d, sheets, ventes, tarifs, notes, syn := monthlyBook(t)
	gs := similar(t, d)
	if len(gs) != 1 {
		t.Fatalf("groupes : %+v", gs)
	}
	g := gs[0]
	if g.CategoryFrom != "cell:A1" || g.Category.Column != "Mois" || len(g.Sheets) != 3 || len(g.Sources) < 2 {
		t.Fatalf("catégorie : %s %+v %+v", g.CategoryFrom, g.Category, g.Sources)
	}
	if len(g.Families) != 2 {
		t.Fatalf("familles : %+v", g.Families)
	}
	modes := map[string]TableFamily{}
	for _, f := range g.Families {
		modes[f.Tables[0].Table] = f
	}
	if f := modes[tarifs[0]]; !f.Same || f.Mode != FamilyOnce || len(f.Tables) != 3 {
		t.Errorf("tarifs : %+v", f)
	}
	if f := modes[notes[0]]; f.Same || f.Mode != FamilyMerge || len(f.Tables) != 3 {
		t.Errorf("notes : %+v", f)
	}
	if g.Tables[0].Table != ventes[0] {
		t.Errorf("famille principale : %+v", g.Tables)
	}
	before := dump(t, d)
	synBefore := cellValues(t, d, syn)

	plan := MergePlan{Tables: ventes, Name: "Ventes", Sheet: "Ventes", CategoryFrom: g.CategoryFrom,
		Families: []MergeFamily{{Tables: tarifs, Mode: FamilyOnce, Name: "Tarifs"}, {Tables: notes, Mode: FamilyMerge, Name: "Notes"}}}
	p, err := d.MergePreview(ctx, plan)
	must(t, err)
	if p.Refused != "" || len(p.Families) != 3 || p.PerValue != 2 || p.Cells != 5 {
		t.Fatalf("aperçu : refus %q, familles %+v, cellules %d (%d par mois) %+v", p.Refused, p.Families, p.Cells, p.PerValue, p.Layout)
	}
	res, err := d.MergeSheets(ctx, plan)
	must(t, err)
	check(t, d, "regroupement")
	if res.Old != MergeOldDelete || res.Table != "Ventes" || res.PerValue != 2 || len(res.Pending) != 0 {
		t.Fatalf("résultat : %+v", res)
	}
	l, err := d.Layout(ctx, res.Sheet)
	must(t, err)
	var names []string
	for _, s := range l.Sheets {
		names = append(names, s.Name)
	}
	if !slices.Equal(names, []string{"Ventes", "Synthèse"}) {
		t.Fatalf("onglets : %v", names)
	}
	ch := l.Sheets[0].Choice
	if ch == nil || ch.Name != "Mois" || ch.Value != "01/03/2025" {
		t.Fatalf("onglet à choix : %+v", ch)
	}
	// Disposition du dernier mois : les ventes en A3 (Mois puis les colonnes
	// d'avant, décalées d'une colonne), les tarifs en F3, les notes sous les
	// ventes, descendues de ce que les ventes ont grandi (4 lignes).
	v, ta, no := tableAt(t, d, res.Sheet, "Ventes"), tableAt(t, d, res.Sheet, "Tarifs"), tableAt(t, d, res.Sheet, "Notes")
	if v.Row != 2 || v.Col != 0 || v.Total != 6 || v.Rows != 2 || !v.Follows {
		t.Errorf("ventes : %+v", v)
	}
	if ta.Row != 2 || ta.Col != 5 || ta.Total != 2 || ta.Rows != 2 || ta.Follows {
		t.Errorf("tarifs : %+v", ta)
	}
	if no.Row != 13 || no.Col != 0 || no.Total != 6 || no.Rows != 2 || !no.Follows {
		t.Errorf("notes : %+v", no)
	}
	// Cellules : titre et objectif par mois, le reste une fois ; le total
	// sous les ventes suit le mois affiché.
	vals := cellValues(t, d, res.Sheet)
	raws := cellRaws(t, d, res.Sheet)
	want := map[string]string{"0,1": "Mars 2025", "0,7": "Objectif", "0,8": "7000", "10,1": "Total", "10,2": "33"}
	for k, w := range want {
		got := vals[k]
		if got == "" {
			got = raws[k]
		}
		if got != w {
			t.Errorf("cellule %s : %q (%q), attendu %q", k, got, raws[k], w)
		}
	}
	if raws["10,2"] != "=SOMME(Ventes[Montant])" {
		t.Errorf("total : %q", raws["10,2"])
	}
	// La synthèse sur une autre feuille : reprise sur le tableau commun filtré.
	if got := cellValues(t, d, syn); fmt.Sprint(got) != fmt.Sprint(synBefore) {
		t.Errorf("synthèse : %v, attendu %v (%v)", got, synBefore, cellRaws(t, d, syn))
	}
	if r := cellRaws(t, d, syn)["0,0"]; !strings.Contains(r, "SOMME.SI.ENS(Ventes[Montant]") {
		t.Errorf("synthèse : %s", r)
	}

	// Février : chaque tableau commun, les cellules par mois et le total
	// suivent.
	must(t, d.ChooseSheetValue(ctx, res.Sheet, "01/02/2025", false))
	vals, raws = cellValues(t, d, res.Sheet), cellRaws(t, d, res.Sheet)
	if raws["0,1"] != "Février 2025" || raws["0,8"] != "6000" || vals["10,2"] != "22" {
		t.Errorf("février : %v %v", raws, vals)
	}
	if n := tableAt(t, d, res.Sheet, "Notes"); n.Rows != 2 {
		t.Errorf("notes de février : %+v", n)
	}
	// Une saisie dans une cellule par mois ne vaut que pour ce mois.
	mustSet(t, d, res.Sheet, 0, 8, "6500")
	must(t, d.ChooseSheetValue(ctx, res.Sheet, "01/03/2025", false))
	if r := cellRaws(t, d, res.Sheet)["0,8"]; r != "7000" {
		t.Errorf("mars après saisie en février : %s", r)
	}
	must(t, d.ChooseSheetValue(ctx, res.Sheet, "01/02/2025", false))
	if r := cellRaws(t, d, res.Sheet)["0,8"]; r != "6500" {
		t.Errorf("février après saisie : %s", r)
	}
	// Annuler la saisie, vu depuis Mars : février reprend 6000, Mars garde 7000.
	must(t, d.ChooseSheetValue(ctx, res.Sheet, "01/03/2025", false))
	_, err = d.bus.Undo(ctx)
	must(t, err)
	if r := cellRaws(t, d, res.Sheet)["0,8"]; r != "7000" {
		t.Errorf("mars après annulation : %s", r)
	}
	must(t, d.ChooseSheetValue(ctx, res.Sheet, "01/02/2025", false))
	if r := cellRaws(t, d, res.Sheet)["0,8"]; r != "6000" {
		t.Errorf("février après annulation : %s", r)
	}
	// « (Toutes) » : les cellules par mois sont vides, tous les tableaux
	// montrent toutes leurs lignes.
	must(t, d.ChooseSheetValue(ctx, res.Sheet, "", true))
	if r := cellRaws(t, d, res.Sheet)["0,1"]; r != "" {
		t.Errorf("toutes : titre %q", r)
	}
	if v := tableAt(t, d, res.Sheet, "Ventes"); v.Rows != 6 {
		t.Errorf("toutes : %+v", v)
	}
	must(t, d.ChooseSheetValue(ctx, res.Sheet, "01/03/2025", false))

	// Une seule annulation rend les trois onglets.
	_, err = d.bus.Undo(ctx)
	must(t, err)
	check(t, d, "annulation")
	if got := stripStaging(dump(t, d)); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
	_, err = d.bus.Redo(ctx)
	must(t, err)
	check(t, d, "rétablissement")
	if r := cellRaws(t, d, res.Sheet)["0,8"]; r != "7000" {
		t.Errorf("rétablissement : objectif %q", r)
	}
	_ = sheets
}

// « Suivre l'année » : cocher sur un tableau qui ne suit pas ajoute la
// colonne de catégorie (remplie avec la valeur affichée) ; décocher la
// retire quand elle n'a qu'une valeur, sinon la garde et le tableau ne suit
// plus (réglage gardé avec le tableau).
func TestTableFollow(t *testing.T) {
	ctx := context.Background()
	d, _, ventes, tarifs, notes, _ := monthlyBook(t)
	res, err := d.MergeSheets(ctx, MergePlan{Tables: ventes, Name: "Ventes", CategoryFrom: "cell:A1",
		Families: []MergeFamily{{Tables: tarifs, Mode: FamilyOnce, Name: "Tarifs"}, {Tables: notes, Mode: FamilyMerge, Name: "Notes"}}})
	must(t, err)
	before := dump(t, d)
	msg, err := d.SetTableFollow(ctx, "Tarifs", true)
	must(t, err)
	check(t, d, "suivre")
	ta := tableAt(t, d, res.Sheet, "Tarifs")
	if !ta.Follows || ta.Rows != 2 || len(ta.Columns) != 3 || ta.Columns[2].Name != "Mois" || !strings.Contains(msg, "suit le mois") {
		t.Fatalf("tarifs qui suivent : %q %+v", msg, ta)
	}
	must(t, d.ChooseSheetValue(ctx, res.Sheet, "01/02/2025", false))
	if ta := tableAt(t, d, res.Sheet, "Tarifs"); ta.Rows != 0 {
		t.Errorf("tarifs en février : %+v", ta)
	}
	must(t, d.ChooseSheetValue(ctx, res.Sheet, "01/03/2025", false))
	msg, err = d.SetTableFollow(ctx, "Tarifs", false)
	must(t, err)
	if ta := tableAt(t, d, res.Sheet, "Tarifs"); ta.Follows || len(ta.Columns) != 2 || !strings.Contains(msg, "retirée") {
		t.Fatalf("tarifs qui ne suivent plus : %q %+v", msg, ta)
	}
	if got := sqlText(t, d, `SELECT coalesce(follow, 'auto') FROM _lattis_tables WHERE name = 'Tarifs'`); got != "auto" {
		t.Errorf("réglage après aller-retour : %s", got)
	}
	_ = before
	// Les notes ont trois mois : la colonne reste, le tableau ne suit plus.
	msg, err = d.SetTableFollow(ctx, "Notes", false)
	must(t, err)
	no := tableAt(t, d, res.Sheet, "Notes")
	if no.Follows || no.Rows != 6 || !strings.Contains(msg, "gardée") {
		t.Fatalf("notes : %q %+v", msg, no)
	}
	_, err = d.bus.Undo(ctx)
	must(t, err)
	if no := tableAt(t, d, res.Sheet, "Notes"); !no.Follows || no.Rows != 2 {
		t.Errorf("notes après annulation : %+v", no)
	}
	// Un tableau créé (Mettre sous forme de tableau) sur la feuille réglée
	// sur Mars suit le mois : colonne Mois en dernier, remplie avec Mars.
	mustSet(t, d, res.Sheet, 40, 0, "Produit")
	mustSet(t, d, res.Sheet, 41, 0, "Pomme")
	name, err := d.FormatAsTable(ctx, res.Sheet, 40, 0)
	must(t, err)
	nt := tableAt(t, d, res.Sheet, name)
	if !nt.Follows || len(nt.Columns) != 2 || nt.Columns[1].Name != "Mois" || nt.Rows != 1 {
		t.Errorf("nouveau tableau : %+v", nt)
	}
	// La colonne de l'onglet à choix ne se décoche pas.
	if _, err := d.SetTableFollow(ctx, "Ventes", false); err == nil {
		t.Error("ventes : décocher la colonne source du choix est refusé")
	}
}

// Cas E : des tableaux semblables empilés sur une feuille sous des titres
// (« Paris », « Lyon », « Marseille ») : la catégorie vient des titres.
func TestMergeStackedTables(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	cities := []string{"Paris", "Lyon", "Marseille"}
	for i, c := range cities {
		r := i * 6
		mustSet(t, d, 1, r, 0, c)
		_, err := d.Paste(ctx, 1, r+1, 0, [][]string{{"Client", "Montant"}, {"A" + c, fmt.Sprint(i + 1)}, {"B" + c, "10"}})
		must(t, err)
	}
	gs := similar(t, d)
	if len(gs) != 1 || !gs[0].Stacked || gs[0].CategoryFrom != FromTitle || gs[0].Category.Column != "Ville" {
		t.Fatalf("tableaux empilés : %+v", gs)
	}
	var tables []string
	for _, tb := range gs[0].Tables {
		tables = append(tables, tb.Table)
	}
	res, err := d.MergeSheets(ctx, MergePlan{Tables: tables, CategoryFrom: FromTitle, Name: "Ventes"})
	must(t, err)
	check(t, d, "empilés")
	if res.Rows != 6 {
		t.Fatalf("résultat : %+v", res)
	}
	l, _ := d.Layout(ctx, res.Sheet)
	if ch := l.Sheets[len(l.Sheets)-1].Choice; ch == nil && l.Sheets[0].Choice == nil {
		t.Fatalf("onglet à choix : %+v", l.Sheets)
	}
	if got := sqlText(t, d, `SELECT group_concat(v, ',') FROM (SELECT DISTINCT c_1 AS v FROM `+tableSQL(tableAt(t, d, res.Sheet, "Ventes").ID)+` ORDER BY _pos)`); got != "Paris,Lyon,Marseille" {
		t.Errorf("villes : %s", got)
	}
}

// Suggestion à la copie d'onglet : une copie est notée, et les onglets
// copiés les uns des autres comptent dans la détection.
func TestSimilarSheetsCopies(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	jan := sheetWith(t, d, "Janvier", [][]string{{"Client", "Montant"}, {"A", "1"}, {"B", "2"}})
	feb, err := d.CopySheet(ctx, jan, 1)
	must(t, err)
	mar, err := d.CopySheet(ctx, feb, 2)
	must(t, err)
	gs := similar(t, d)
	if len(gs) != 1 || gs[0].Copies != 3 || len(gs[0].Sheets) != 3 {
		t.Fatalf("copies : %+v", gs)
	}
	for _, s := range gs[0].Sheets {
		if !s.Copied {
			t.Errorf("onglet %s non marqué copié", s.Name)
		}
	}
	_ = mar
}

// Cas F : chaque onglet a une colonne Date dont les valeurs tombent dans
// son mois : la catégorie est déjà dans les données.
func TestSimilarSheetsDataCategory(t *testing.T) {
	d := open(t)
	for i, m := range []string{"Janv 2025", "Févr 2025", "Mars 2025"} {
		sheetWith(t, d, m, [][]string{{"Date", "Montant"}, {fmt.Sprintf("03/%02d/2025", i+1), "1"}, {fmt.Sprintf("28/%02d/2025", i+1), "2"}})
	}
	gs := similar(t, d)
	if len(gs) != 1 || gs[0].DataColumn != "Date" {
		t.Fatalf("cas F : %+v", gs)
	}
}
