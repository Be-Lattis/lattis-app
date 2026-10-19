package canvas

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/series"
)

// sheetWith ajoute un onglet nommé portant un tableau (en A1) ; le premier
// onglet du classeur (Feuille1) sert pour le premier appel.
func sheetWith(t *testing.T, d *fixture, name string, rows [][]string) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64 = 1
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Sheets) > 1 || len(l.Tables) > 0 || l.Sheets[0].Name != "Feuille1" {
		if id, err = d.AddSheet(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.RenameSheet(ctx, id, name); err != nil {
		t.Fatal(err)
	}
	if rows != nil {
		if _, err := d.Paste(ctx, id, 0, 0, rows); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

// tableOn donne le nom du tableau de la feuille.
func tableOn(t *testing.T, d *fixture, sheet int64) string {
	t.Helper()
	l, err := d.Layout(context.Background(), sheet)
	if err != nil || len(l.Tables) != 1 {
		t.Fatalf("feuille %d : %v %+v", sheet, err, l.Tables)
	}
	return l.Tables[0].Name
}

func similar(t *testing.T, d *fixture) []SheetGroup {
	t.Helper()
	gs, err := d.SimilarSheets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return gs
}

func groupSheets(g SheetGroup) []string {
	var out []string
	for _, tb := range g.Tables {
		out = append(out, tb.SheetName)
	}
	return out
}

func TestSimilarSheetsSeries(t *testing.T) {
	data := func(n string) [][]string {
		return [][]string{{"Client", "Montant", "Payé"}, {"Durand " + n, "10", "vrai"}, {"Martin " + n, "20", "faux"}}
	}
	cases := []struct {
		sheets []string
		kind   series.Kind
		column string
		typ    grid.Type
	}{
		{[]string{"Janvier", "Février", "Mars"}, series.Month, "Mois", grid.Text},
		{[]string{"2023", "2024"}, series.Year, "Année", grid.Integer},
		{[]string{"Lot 1", "Lot 2", "Lot 4"}, series.Number, "Lot", grid.Integer},
		{[]string{"Paris", "Lyon", "Marseille"}, series.City, "Ville", grid.Text},
		{[]string{"Janv 2025", "Févr 2025"}, series.MonthYear, "Mois", grid.Date},
	}
	for _, c := range cases {
		d := open(t)
		for _, s := range c.sheets {
			sheetWith(t, d, s, data(s))
		}
		// Une synthèse aux colonnes différentes n'en fait pas partie.
		sheetWith(t, d, "Synthèse", [][]string{{"Indicateur", "Valeur"}, {"Total", "30"}})
		gs := similar(t, d)
		if len(gs) != 1 {
			t.Fatalf("%v : %d groupes %+v", c.sheets, len(gs), gs)
		}
		g := gs[0]
		if !slices.Equal(groupSheets(g), c.sheets) || g.Category.Kind != c.kind || g.Category.Column != c.column || g.Category.Type != c.typ {
			t.Errorf("%v : %v %s %q %s", c.sheets, groupSheets(g), g.Category.Kind, g.Category.Column, g.Category.Type)
		}
		if !g.Identical || !g.Propose || g.ID == "" {
			t.Errorf("%v : identique %v, proposé %v (score %d)", c.sheets, g.Identical, g.Propose, g.Score)
		}
	}
	// Deux onglets sans noms en série : pas de groupe.
	d := open(t)
	sheetWith(t, d, "Clients", data("a"))
	sheetWith(t, d, "Fournisseurs", data("b"))
	if gs := similar(t, d); len(gs) != 0 {
		t.Errorf("deux onglets sans série : %+v", gs)
	}
	// Trois onglets sans série : trouvés, mais pas proposés d'eux-mêmes.
	sheetWith(t, d, "Partenaires", data("c"))
	if gs := similar(t, d); len(gs) != 1 || gs[0].Propose || gs[0].Category.Kind != series.Other || gs[0].Category.Column != "Catégorie" {
		t.Errorf("trois onglets sans série : %+v", gs)
	}
}

// Tolérance des deux tiers (§2.2) : une colonne ajoutée en cours d'année,
// une colonne renommée (appariée, montrée), un onglet trop différent écarté.
func TestSimilarSheetsTolerance(t *testing.T) {
	d := open(t)
	sheetWith(t, d, "Janvier", [][]string{{"Client", "Montant", "Ville"}, {"A", "1", "Paris"}})
	sheetWith(t, d, "Février", [][]string{{"Client", "Montant", "Ville", "Remise"}, {"B", "2", "Lyon", "5"}})
	sheetWith(t, d, "Mars", [][]string{{"Client", "Montant €", "Ville"}, {"C", "3", "Nice"}})
	sheetWith(t, d, "Avril", [][]string{{"Produit", "Stock", "Ville"}, {"Vis", "3", "Nice"}})
	gs := similar(t, d)
	if len(gs) != 1 || !slices.Equal(groupSheets(gs[0]), []string{"Janvier", "Février", "Mars"}) {
		t.Fatalf("groupes : %+v", gs)
	}
	g := gs[0]
	var names []string
	for _, c := range g.Columns {
		names = append(names, c.Name)
	}
	if !slices.Equal(names, []string{"Client", "Montant", "Ville", "Remise"}) {
		t.Fatalf("colonnes : %v", names)
	}
	if m := g.Columns[1]; m.From[2] != "Montant €" || !m.Paired[2] {
		t.Errorf("Montant € non apparié : %+v", m)
	}
	if r := g.Columns[3]; !slices.Equal(r.From, []string{"", "Remise", ""}) {
		t.Errorf("Remise : %+v", r.From)
	}
	if g.Identical || len(g.Notes) == 0 || !strings.Contains(strings.Join(g.Notes, " "), "Montant €") {
		t.Errorf("notes : %v (identique %v)", g.Notes, g.Identical)
	}

	// De proche en proche, A·B·C ~ B·C·D ~ C·D·E ~ D·E·F : le groupe est
	// vérifié en entier, le plus éloigné sort.
	d = open(t)
	for i, cols := range [][]string{{"A", "B", "C"}, {"B", "C", "D"}, {"C", "D", "E"}, {"D", "E", "F"}} {
		sheetWith(t, d, fmt.Sprint(2021+i), [][]string{cols, {"1", "2", "3"}})
	}
	gs = similar(t, d)
	if len(gs) != 1 || !slices.Equal(groupSheets(gs[0]), []string{"2022", "2023", "2024"}) {
		t.Fatalf("chaîne : %+v", gs)
	}

	// Clés de noms différents : pas ensemble.
	d = open(t)
	a := sheetWith(t, d, "2023", [][]string{{"N° client", "Nom", "Ville"}, {"1", "A", "Paris"}})
	b := sheetWith(t, d, "2024", [][]string{{"Référence", "Nom", "Ville"}, {"1", "A", "Paris"}})
	ctx := context.Background()
	if err := d.SetKey(ctx, tableOn(t, d, a), 0); err != nil {
		t.Fatal(err)
	}
	if err := d.SetKey(ctx, tableOn(t, d, b), 0); err != nil {
		t.Fatal(err)
	}
	if gs := similar(t, d); len(gs) != 0 {
		t.Errorf("clés différentes : %+v", gs)
	}
}

// stripStaging retire du vidage les tables de chargement (gardées pour
// rétablir, comme celles d'Import).
func stripStaging(s string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if !strings.Contains(l, importPrefix) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

func TestMergeSheets(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	jan := sheetWith(t, d, "Janvier", [][]string{{"Client", "Montant", "Payé"}, {"Durand", "10", "vrai"}, {"Martin", "20", "faux"}})
	fev := sheetWith(t, d, "Février", [][]string{{"Client", "Montant", "Payé", "Remise"}, {"Dupont", "12,5", "vrai", "3"}, {"Leroy", "abc", "faux", "1"}})
	mar := sheetWith(t, d, "Mars", [][]string{{"Client", "Montant", "Payé"}, {"Petit", "7", "vrai"}})
	sheetWith(t, d, "Notes", nil)
	if _, err := d.SetCell(ctx, 4, 0, 0, "Bilan"); err != nil {
		t.Fatal(err)
	}
	tJan, tFev, tMar := tableOn(t, d, jan), tableOn(t, d, fev), tableOn(t, d, mar)
	// Février : Montant en décimal avec une saisie invalide (abc).
	if err := d.SetType(ctx, tFev, 1, grid.Decimal); err != nil {
		t.Fatal(err)
	}
	// Une cellule en gras (Leroy, 2e ligne de Février) et un titre libre.
	if err := d.ApplyStyle(ctx, fev, 2, 0, 2, 0, Style{"b": true}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, jan, 6, 0, "Total janvier"); err != nil {
		t.Fatal(err)
	}
	before := stripStaging(dump(t, d))

	p, err := d.MergePreview(ctx, MergePlan{Tables: []string{tJan, tFev, tMar}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Refused != "" || p.Total != 5 || len(p.Rows) != 5 || !slices.Equal(p.Header, []string{"Mois", "Client", "Montant", "Payé", "Remise"}) || p.Old != MergeOldDelete {
		t.Fatalf("aperçu : %+v", p)
	}
	if p.Plan.Category == nil || p.Plan.Category.Name != "Mois" || p.Rows[2][0] != "Février" || p.Rows[2][2] != 12.5 || p.Rows[0][4] != nil {
		t.Fatalf("aperçu, lignes : %+v %v", p.Plan.Category, p.Rows)
	}

	res, err := d.MergeSheets(ctx, MergePlan{Tables: []string{tJan, tFev, tMar}, Name: "Ventes"})
	if err != nil {
		t.Fatal(err)
	}
	check(t, d, "regroupement")
	if res.Table != "Ventes" || res.Rows != 5 || res.Old != MergeOldDelete || res.SheetName != "Ventes" {
		t.Fatalf("résultat : %+v", res)
	}
	// Onglet à choix (§7, étape 8) : par Mois, réglé sur le plus récent.
	if ch, err := d.SheetChoice(ctx, res.Sheet); err != nil || ch == nil || ch.Name != "Mois" || ch.Value != "Mars" {
		t.Fatalf("onglet à choix : %+v, %v", ch, err)
	}
	l, err := d.Layout(ctx, res.Sheet)
	if err != nil {
		t.Fatal(err)
	}
	// Les anciens onglets sont partis (le titre libre avec Janvier) ; la
	// nouvelle feuille prend la place du premier, avant Notes.
	var names []string
	for _, s := range l.Sheets {
		names = append(names, s.Name)
	}
	if !slices.Equal(names, []string{"Ventes", "Notes"}) {
		t.Fatalf("onglets : %v", names)
	}
	if len(l.Tables) != 1 || l.Tables[0].Total != 5 {
		t.Fatalf("tableau : %+v", l.Tables)
	}
	tb := l.Tables[0]
	var cols []string
	for _, c := range tb.Columns {
		cols = append(cols, c.Name+":"+string(c.Type))
	}
	// Montant élargi en décimal, Remise ajoutée en fin, la catégorie en tête.
	if got := strings.Join(cols, " "); got != "Mois:text Client:text Montant:decimal Payé:boolean Remise:integer" {
		t.Fatalf("colonnes : %s", got)
	}
	rows := tableRows(t, d, "Ventes")
	want := []string{
		"Janvier|Durand|10|1|", "Janvier|Martin|20|0|",
		"Février|Dupont|12.5|1|3", "Février|Leroy||0|1",
		"Mars|Petit|7|1|",
	}
	if !slices.Equal(rows, want) {
		t.Fatalf("lignes :\n%v\nattendu\n%v", rows, want)
	}
	// Saisie invalide et mise en forme suivent leur ligne ; liste de
	// validation sur la catégorie.
	var inv string
	var bold, dv int
	err = d.read(ctx, func(x *tx) error {
		if err := x.row(`SELECT i.raw FROM _lattis_invalid i JOIN t_`+fmt.Sprint(tb.ID)+` t ON t.id = i.rid WHERE i.tbl = ? AND t.c_2 = 'Leroy'`, tb.ID).Scan(&inv); err != nil {
			return err
		}
		if err := x.row(`SELECT count(*) FROM _lattis_styles s JOIN t_`+fmt.Sprint(tb.ID)+` t ON t.id = s.rid WHERE s.tbl = ? AND t.c_2 = 'Leroy' AND s.col = 2 AND s.style LIKE '%"b":true%'`, tb.ID).Scan(&bold); err != nil {
			return err
		}
		return x.row(`SELECT count(*) FROM _lattis_styles WHERE tbl = ? AND rid = 0 AND col = 1 AND style LIKE '%"dv":["Janvier","Février","Mars"]%'`, tb.ID).Scan(&dv)
	})
	if err != nil || inv != "abc" || bold != 1 || dv != 1 {
		t.Fatalf("invalide %q, gras %d, liste %d : %v", inv, bold, dv, err)
	}
	if after := similar(t, d); len(after) != 0 {
		t.Errorf("plus rien à regrouper : %+v", after)
	}

	// Une seule étape d'annulation rend tout ; rétablir refait le regroupement.
	if _, err := d.bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if got := stripStaging(dump(t, d)); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
	check(t, d, "annulation")
	if _, err := d.bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	check(t, d, "rétablissement")
	if rows := tableRows(t, d, "Ventes"); !slices.Equal(rows, want) {
		t.Fatalf("après rétablissement : %v", rows)
	}
}

// tableRows lit un tableau, cases séparées par |.
func tableRows(t *testing.T, d *fixture, name string) []string {
	t.Helper()
	var out []string
	err := d.ExportTable(context.Background(), name, func([]ExportColumn) error { return nil }, func(row []any) error {
		var cells []string
		for _, v := range row {
			if v == nil {
				cells = append(cells, "")
				continue
			}
			if _, bad := v.(Invalid); bad {
				cells = append(cells, "")
				continue
			}
			cells = append(cells, fmt.Sprint(v))
		}
		out = append(out, strings.Join(cells, "|"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Clé (§7, étape 5) : gardée sans collision, retirée sinon, avec un exemple.
func TestMergeSheetsKey(t *testing.T) {
	ctx := context.Background()
	for _, collide := range []bool{false, true} {
		d := open(t)
		second := "F-003"
		if collide {
			second = "f-001" // la clé ne tient pas compte de la casse
		}
		a := sheetWith(t, d, "Janvier", [][]string{{"Facture", "Montant"}, {"F-001", "10"}, {"F-002", "20"}})
		b := sheetWith(t, d, "Février", [][]string{{"Facture", "Montant"}, {second, "30"}, {"F-004", "40"}})
		ta, tb := tableOn(t, d, a), tableOn(t, d, b)
		for _, n := range []string{ta, tb} {
			if err := d.SetKey(ctx, n, 0); err != nil {
				t.Fatal(err)
			}
		}
		gs := similar(t, d)
		if len(gs) != 1 || gs[0].Key != "Facture" {
			t.Fatalf("groupe à clé : %+v", gs)
		}
		p, err := d.MergePreview(ctx, MergePlan{Tables: []string{ta, tb}})
		if err != nil {
			t.Fatal(err)
		}
		res, err := d.MergeSheets(ctx, MergePlan{Tables: []string{ta, tb}, Name: "Factures"})
		if err != nil {
			t.Fatal(err)
		}
		check(t, d, "regroupement à clé")
		l, _ := d.Layout(ctx, res.Sheet)
		hasKey := l.Tables[0].Key >= 0
		if collide {
			const msg = "Pas de clé : F-001 est dans Janvier et dans Février"
			if hasKey || res.KeyKept || res.KeyNote != msg || p.KeyNote != msg || p.KeyKept || !strings.Contains(res.Notice, msg) {
				t.Errorf("collision : clé %v, %+v / aperçu %q", hasKey, res, p.KeyNote)
			}
			if rows := tableRows(t, d, "Factures"); len(rows) != 4 {
				t.Errorf("toutes les lignes restent : %v", rows)
			}
		} else if !hasKey || !res.KeyKept || res.KeyNote != "" || !p.KeyKept || l.Tables[0].Columns[l.Tables[0].Key].Name != "Facture" {
			t.Errorf("sans collision : clé %v %+v", hasKey, res)
		}
	}
}

// Une synthèse qui cite les tableaux : à reprendre (lot 2), les anciens
// onglets sont alors gardés masqués, leurs tableaux renommés (les formules
// suivent et continuent de marcher).
func TestMergeSheetsPending(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	a := sheetWith(t, d, "Ventes 2023", [][]string{{"Client", "Montant"}, {"A", "10"}})
	b := sheetWith(t, d, "Ventes 2024", [][]string{{"Client", "Montant"}, {"B", "20"}})
	syn := sheetWith(t, d, "Synthèse", nil)
	ta, tb := tableOn(t, d, a), tableOn(t, d, b)
	if _, err := d.SetCell(ctx, syn, 0, 0, "=SOMME("+ta+"[Montant])+SOMME("+tb+"[Montant])"); err != nil {
		t.Fatal(err)
	}
	gs := similar(t, d)
	if len(gs) != 1 || !slices.Equal(gs[0].Summary, []string{"Synthèse"}) {
		t.Fatalf("synthèse : %+v", gs)
	}
	p, err := d.MergePreview(ctx, MergePlan{Tables: []string{ta, tb}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Pending) != 1 || p.Pending[0].Kind != "cell" || p.Pending[0].Where != "Synthèse A1" || p.Old != MergeOldHide {
		t.Fatalf("à reprendre : %+v (%s)", p.Pending, p.Old)
	}
	if p.Plan.Category.Name != "Année" || p.Rows[1][0] != int64(2024) {
		t.Fatalf("catégorie : %+v %v", p.Plan.Category, p.Rows)
	}
	res, err := d.MergeSheets(ctx, MergePlan{Tables: []string{ta, tb}})
	if err != nil {
		t.Fatal(err)
	}
	check(t, d, "regroupement masqué")
	if res.Old != MergeOldHide || len(res.Pending) != 1 {
		t.Fatalf("résultat : %+v", res)
	}
	l, _ := d.Layout(ctx, syn)
	hidden := 0
	for _, s := range l.Sheets {
		if s.Hidden {
			hidden++
		}
	}
	if hidden != 2 || !strings.Contains(l.Cells[0].Raw, ta+"_ancien[Montant]") || l.Cells[0].Value != "30" {
		t.Fatalf("anciens onglets masqués %d, synthèse %+v", hidden, l.Cells)
	}
	// Supprimer quand même : refusé à rien, mais signalé.
	d.bus.Undo(ctx)
	res, err = d.MergeSheets(ctx, MergePlan{Tables: []string{ta, tb}, Old: MergeOldDelete, NoCategory: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Old != MergeOldDelete || !strings.Contains(strings.Join(res.Notes, " "), "à reprendre") {
		t.Errorf("suppression malgré les formules : %+v", res)
	}
	check(t, d, "suppression")
}

// Garde-fous (§7, étape 1) : tableau filtré, feuille protégée.
func TestMergeSheetsGuards(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	a := sheetWith(t, d, "Ventes 2023", [][]string{{"Client", "Montant"}, {"A", "10"}, {"B", "5"}})
	b := sheetWith(t, d, "Ventes 2024", [][]string{{"Client", "Montant"}, {"B", "20"}})
	ta, tb := tableOn(t, d, a), tableOn(t, d, b)
	if err := d.SetFilter(ctx, ta, 0, &ColumnFilter{Values: []string{"A"}}); err != nil {
		t.Fatal(err)
	}
	p, err := d.MergePreview(ctx, MergePlan{Tables: []string{ta, tb}})
	if err != nil || !strings.Contains(p.Refused, "filtré") {
		t.Fatalf("filtre : %v %q", err, p.Refused)
	}
	if _, err := d.MergeSheets(ctx, MergePlan{Tables: []string{ta, tb}}); err == nil || !strings.Contains(err.Error(), "filtré") {
		t.Fatalf("regroupement d'un tableau filtré : %v", err)
	}
	if _, err := d.MergeSheets(ctx, MergePlan{Tables: []string{ta}}); err == nil {
		t.Fatal("un seul tableau accepté")
	}
}

// « Ne plus proposer » : gardé dans le fichier, le groupe n'est plus proposé.
func TestDismissSimilar(t *testing.T) {
	d := open(t)
	for _, s := range []string{"Janvier", "Février", "Mars"} {
		sheetWith(t, d, s, [][]string{{"Client", "Montant"}, {"A", "1"}})
	}
	g := similar(t, d)[0]
	if !g.Propose {
		t.Fatalf("pas proposé : %+v", g)
	}
	if err := d.DismissSimilar(context.Background(), g.ID); err != nil {
		t.Fatal(err)
	}
	g2 := similar(t, d)[0]
	if g2.Propose || !g2.Dismissed || g2.ID != g.ID {
		t.Fatalf("après « Ne plus proposer » : %+v", g2)
	}
}

// Colonnes calculées (§7, étape 4) : la même formule partout reste une
// formule ; des formules différentes laissent leurs valeurs.
func TestMergeSheetsComputed(t *testing.T) {
	ctx := context.Background()
	for _, same := range []bool{true, false} {
		d := open(t)
		var tables []string
		for i, s := range []string{"Janvier", "Février", "Mars"} {
			id := sheetWith(t, d, s, [][]string{{"Client", "Montant", "Double"}, {"A", fmt.Sprint(i + 1), "0"}})
			f := "=[Montant]*2"
			if !same && i == 2 {
				f = "=[Montant]*3"
			}
			if _, err := d.SetCell(ctx, id, 1, 2, f); err != nil {
				t.Fatal(err)
			}
			tables = append(tables, tableOn(t, d, id))
		}
		res, err := d.MergeSheets(ctx, MergePlan{Tables: tables, Name: "Ventes"})
		if err != nil {
			t.Fatal(err)
		}
		check(t, d, "colonnes calculées")
		l, _ := d.Layout(ctx, res.Sheet)
		f := l.Tables[0].Columns[3].Formula
		rows := tableRows(t, d, "Ventes")
		last := "9"
		if same {
			last = "6"
		}
		if (f != "") != same || len(rows) != 3 || rows[2] != "Mars|A|3|"+last {
			t.Errorf("même formule %v : formule %q, lignes %v, notes %v", same, f, rows, res.Notes)
		}
	}
}

// La feuille commune garde l'écriture des onglets (espaces compris), le
// tableau un nom d'identifiant.
func TestMergeSheetsSheetName(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	var tables []string
	for i, y := range []string{"2024", "2025", "2026"} {
		sheet := int64(1)
		if i > 0 {
			id, err := d.AddSheet(ctx)
			if err != nil {
				t.Fatal(err)
			}
			sheet = id
			if err := d.RenameSheet(ctx, id, "Suivi des nids_"+y); err != nil {
				t.Fatal(err)
			}
		} else if err := d.RenameSheet(ctx, 1, "Suivi des nids_"+y); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Paste(ctx, sheet, 0, 0, [][]string{{"Nid", "Espèce"}, {"N" + y, "Milan"}}); err != nil {
			t.Fatal(err)
		}
		l, err := d.Layout(ctx, sheet)
		if err != nil || len(l.Tables) != 1 {
			t.Fatalf("feuille %s : %v %v", y, l.Tables, err)
		}
		tables = append(tables, l.Tables[0].Name)
	}
	res, err := d.MergeSheets(ctx, MergePlan{Tables: tables, Name: "Suividesnids"})
	if err != nil {
		t.Fatal(err)
	}
	if res.SheetName != "Suivi des nids" || res.Table != "Suividesnids" {
		t.Fatalf("feuille %q, tableau %q", res.SheetName, res.Table)
	}
}
