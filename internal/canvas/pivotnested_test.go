package canvas

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// salesSheet : la feuille 1 porte Feuille1 (Date, Région, Produit, Montant)
// sur deux années.
func salesSheet(t *testing.T) *fixture {
	t.Helper()
	d := open(t)
	if _, err := d.Paste(context.Background(), 1, 0, 0, [][]string{
		{"Date", "Région", "Produit", "Montant"},
		{"15/01/2024", "Nord", "A", "10"},
		{"10/02/2024", "Sud", "B", "20"},
		{"05/04/2024", "Nord", "B", "30"},
		{"20/07/2024", "Sud", "A", "40"},
		{"03/01/2025", "Nord", "A", "50"},
		{"12/05/2025", "Sud", "B", "60"},
	}); err != nil {
		t.Fatal(err)
	}
	return d
}

// pivotOf crée le TCD cfg et rend sa feuille et une lecture de son résultat.
func pivotOf(t *testing.T, d *fixture, cfg PivotConfig) (int64, int64, func() PivotInfo) {
	t.Helper()
	ctx := context.Background()
	sheet, err := d.CreatePivot(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	read := func() PivotInfo {
		t.Helper()
		l, err := d.Layout(ctx, sheet)
		if err != nil {
			t.Fatal(err)
		}
		p := l.Pivots[0]
		if p.Error != "" {
			t.Fatal(p.Error)
		}
		return p
	}
	return sheet, read().ID, read
}

func rowsText(cells [][]string) string {
	var out []string
	for _, r := range cells {
		out = append(out, strings.Join(r, ","))
	}
	return strings.Join(out, " | ")
}

// Plusieurs champs en colonnes : en-têtes imbriqués, sous-totaux par
// niveau, groupe plié ; la vue à plat (SQL, graphiques) garde une colonne
// par combinaison, nommée par son chemin.
func TestPivotNestedColumns(t *testing.T) {
	ctx := context.Background()
	d := salesSheet(t)
	cfg := PivotConfig{Source: "Feuille1", ColFields: []string{"Région", "Produit"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}}
	_, id, read := pivotOf(t, d, cfg)
	p := read()
	if got := rowsText(p.ColHead); got != ",Nord,,,Sud,,," {
		t.Fatalf("en-tête du haut : %q", got)
	}
	if got := rowsText(p.Cells); got != ",A,B,Total Nord,B,A,Total Sud,Total | Total,60,30,90,80,40,120,210" {
		t.Fatalf("TCD : %q", got)
	}
	if !p.Cols[3].Subtotal || p.Cols[7].Total == false || fmt.Sprint(p.Cols[1].Path) != "[Nord A]" {
		t.Fatalf("description des colonnes : %+v", p.Cols)
	}
	// Zone sur la feuille : la ligne d'en-tête de plus.
	if r := pivotRect(p); r.R1-r.R0 != 2 {
		t.Fatalf("zone : %+v", r)
	}
	// Plier Nord : une colonne, son sous-total.
	cfg.ColCollapsed = [][]string{{"Nord"}}
	must(t, d.SetPivot(ctx, id, cfg))
	p = read()
	if got := rowsText(p.Cells); got != ",Nord,B,A,Total Sud,Total | Total,90,80,40,120,210" || !p.Cols[1].Collapsed {
		t.Fatalf("Nord plié : %q", got)
	}
	// Sans sous-totaux.
	cfg.ColCollapsed, cfg.Subtotals = nil, SubtotalsNone
	must(t, d.SetPivot(ctx, id, cfg))
	if got := rowsText(read().Cells); got != ",A,B,B,A,Total | Total,60,30,80,40,210" {
		t.Fatalf("sans sous-totaux : %q", got)
	}
	// À plat : chemin complet.
	var flat [][]string
	must(t, d.read(ctx, func(t *tx) error {
		var err error
		flat, err = t.pivotFlat(cfg)
		return err
	}))
	if got := rowsText(flat); got != ",Nord · A,Nord · B,Sud · B,Sud · A,Total | Total,60,30,80,40,210" {
		t.Fatalf("à plat : %q", got)
	}
	// Avec un champ en lignes et deux valeurs.
	cfg = PivotConfig{Source: "Feuille1", RowFields: []string{"Produit"}, ColFields: []string{"Région", "Produit"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}}
	cfg.ColFields = []string{"Région", "Date"}
	cfg.Groups = []PivotGroup{{Field: "Date", By: []string{"years"}}}
	must(t, d.SetPivot(ctx, id, cfg))
	p = read()
	if got := rowsText(p.Cells); got != "Produit,2024,2025,Total Nord,2024,2025,Total Sud,Total | A,10,50,60,40,,40,100 | B,30,,30,20,60,80,110 | Total,40,50,90,60,60,120,210" {
		t.Fatalf("lignes et colonnes : %q", got)
	}
	// Un TCD se lit par le chemin de ses colonnes : TCD1[Nord · 2024].
	mustSet(t, d, 1, 20, 8, "=SOMME("+p.Config.Name+"[Nord · 2024])")
	if got := freeValue(t, d, 1, 20, 8); got != "40" {
		t.Fatalf("SOMME(TCD[Nord · 2024]) : %q", got)
	}
}

// treeText : les lignes d'un TCD en arborescence (niveau, groupe −/+).
func treeText(p PivotInfo) string {
	var out []string
	for i, r := range p.Rows {
		k := fmt.Sprint(r.Depth)
		switch {
		case r.Total:
			k = "T"
		case r.Group:
			k += "-"
		}
		out = append(out, k+":"+strings.Join(p.Cells[i+1], "|"))
	}
	return strings.Join(out, " ")
}

// Grouper une date par années, trimestres et mois (champs ajoutés devant
// elle), en lignes comme en colonnes ; un nombre par intervalles.
func TestPivotGroupDates(t *testing.T) {
	ctx := context.Background()
	d := salesSheet(t)
	groups := []PivotGroup{{Field: "Date", By: []string{"years", "quarters", "months"}, Names: []string{"Années", "Trimestres"}}}
	cfg := PivotConfig{Source: "Feuille1", RowFields: []string{"Années", "Trimestres", "Date"}, Groups: groups, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}}
	_, id, read := pivotOf(t, d, cfg)
	want := "0-:2024|100 1-:T1|30 2:janv.|10 2:févr.|20 1-:T2|30 2:avr.|30 1-:T3|40 2:juil.|40 0-:2025|110 1-:T1|50 2:janv.|50 1-:T2|60 2:mai|60 T:Total|210"
	if got := treeText(read()); got != want {
		t.Fatalf("dates groupées :\n%s\n%s", got, want)
	}
	// Tri décroissant d'un niveau : par sa clé (2025 avant 2024).
	cfg.Sort = map[string]string{"Années": "desc"}
	must(t, d.SetPivot(ctx, id, cfg))
	if got := treeText(read()); !strings.HasPrefix(got, "0-:2025|110") {
		t.Fatalf("années décroissantes : %s", got)
	}
	// Trimestres seuls, en colonnes : T1 regroupe les deux années.
	cfg = PivotConfig{Source: "Feuille1", RowFields: []string{"Région"}, ColFields: []string{"Date"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}},
		Groups: []PivotGroup{{Field: "Date", By: []string{"quarters"}}}}
	must(t, d.SetPivot(ctx, id, cfg))
	if got := rowsText(read().Cells); got != "Région,T1,T2,T3,Total | Nord,60,30,,90 | Sud,20,60,40,120 | Total,80,90,40,210" {
		t.Fatalf("trimestres en colonnes : %q", got)
	}
	// Filtre d'un champ groupé (▾ de son étiquette) : par son étiquette.
	cfg.Items = []PivotFilter{{Field: "Date", Values: []string{"T1"}}}
	must(t, d.SetPivot(ctx, id, cfg))
	if got := rowsText(read().Cells); got != "Région,T1,Total | Nord,60,60 | Sud,20,20 | Total,80,80" {
		t.Fatalf("trimestre filtré : %q", got)
	}
	// Valeurs du champ groupé pour le volet et les segments.
	vals, err := d.PivotFieldValues(ctx, cfg, "Date")
	if err != nil || fmt.Sprint(vals) != "[{T1 3} {T2 2} {T3 1}]" {
		t.Fatalf("valeurs : %v %v", vals, err)
	}
	// Nombres par intervalles de 25 de 0 à 49 : au-delà, « >49 ».
	start, end := 0.0, 49.0
	cfg = PivotConfig{Source: "Feuille1", RowFields: []string{"Montant"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}},
		Groups: []PivotGroup{{Field: "Montant", Start: &start, End: &end, Step: 25}}}
	must(t, d.SetPivot(ctx, id, cfg))
	if got := rowsText(read().Cells); got != "Montant,Somme de Montant | 0-24,30 | 25-49,70 | >49,110 | Total,210" {
		t.Fatalf("intervalles : %q", got)
	}
	cfg.Groups[0].Step = 0
	if err := d.SetPivot(ctx, id, cfg); err == nil {
		t.Fatal("pas nul accepté")
	}
}

// « Afficher les valeurs » : chaque calcul, sur les agrégats SQL.
func TestPivotShowValuesAs(t *testing.T) {
	ctx := context.Background()
	d := salesSheet(t)
	base := PivotConfig{Source: "Feuille1", RowFields: []string{"Région"}, ColFields: []string{"Produit"}}
	cfg := base
	cfg.Values = []PivotValue{{Field: "Montant", Agg: "sum"}}
	_, id, read := pivotOf(t, d, cfg)
	if got := rowsText(read().Cells); got != "Région,A,B,Total | Nord,60,30,90 | Sud,40,80,120 | Total,100,110,210" {
		t.Fatalf("valeurs : %q", got)
	}
	for _, c := range []struct {
		v    PivotValue
		want string
	}{
		{PivotValue{Show: "diff", BaseField: "Région", BaseItem: "Nord"}, "Nord,,, | Sud,-20,50,30 | Total,,,"},
		{PivotValue{Show: "pctOf", BaseField: "Région", BaseItem: "Nord"}, "Nord,100 %,100 %,100 % | Sud,66,7 %,266,7 %,133,3 % | Total,,,"},
		{PivotValue{Show: "pctDiff", BaseField: "Produit", BasePos: "prev"}, "Nord,,-50 %, | Sud,,100 %, | Total,,10 %,"},
		{PivotValue{Show: "runTotal", BaseField: "Région"}, "Nord,60,30,90 | Sud,100,110,210 | Total,,,"},
		{PivotValue{Show: "pctRunTotal", BaseField: "Région"}, "Nord,60 %,27,3 %,42,9 % | Sud,100 %,100 %,100 % | Total,,,"},
		{PivotValue{Show: "rankDesc", BaseField: "Produit"}, "Nord,1,2, | Sud,2,1, | Total,2,1,"},
		{PivotValue{Show: "rankAsc", BaseField: "Région"}, "Nord,2,1,1 | Sud,1,2,2 | Total,,,"},
		{PivotValue{Show: "index"}, "Nord,1,4,0,636363636363636,1 | Sud,0,7,1,27272727272727,1 | Total,1,1,1"},
		{PivotValue{Show: "pctTotal"}, "Nord,28,6 %,14,3 %,42,9 % | Sud,19 %,38,1 %,57,1 % | Total,47,6 %,52,4 %,100 %"},
		{PivotValue{Show: "pctCol"}, "Nord,60 %,27,3 %,42,9 % | Sud,40 %,72,7 %,57,1 % | Total,100 %,100 %,100 %"},
		{PivotValue{Show: "pctRow"}, "Nord,66,7 %,33,3 %,100 % | Sud,33,3 %,66,7 %,100 % | Total,47,6 %,52,4 %,100 %"},
		{PivotValue{Show: "pctParentCol"}, "Nord,66,7 %,33,3 %,100 % | Sud,33,3 %,66,7 %,100 % | Total,47,6 %,52,4 %,100 %"},
		{PivotValue{Show: "pctParentRow"}, "Nord,60 %,27,3 %,42,9 % | Sud,40 %,72,7 %,57,1 % | Total,100 %,100 %,100 %"},
	} {
		cfg := base
		v := c.v
		v.Field, v.Agg = "Montant", "sum"
		cfg.Values = []PivotValue{v}
		must(t, d.SetPivot(ctx, id, cfg))
		if got := rowsText(read().Cells[1:]); got != c.want {
			t.Errorf("%s : %q, voulu %q", v.Show, got, c.want)
		}
	}
	// % du total parent, en arborescence : la part de chaque produit dans sa région.
	cfg = PivotConfig{Source: "Feuille1", RowFields: []string{"Région", "Produit"}, Values: []PivotValue{{Field: "Montant", Agg: "sum", Show: "pctParent", BaseField: "Région"}}}
	must(t, d.SetPivot(ctx, id, cfg))
	if got := treeText(read()); got != "0-:Nord|100 % 1:A|66,7 % 1:B|33,3 % 0-:Sud|100 % 1:B|66,7 % 1:A|33,3 % T:Total|" {
		t.Fatalf("pctParent : %s", got)
	}
}

// Segments : leur sélection filtre le TCD, et les TCD reliés ; une
// chronologie filtre par périodes ; déplacer un segment ne recalcule rien.
func TestPivotSlicers(t *testing.T) {
	ctx := context.Background()
	d := salesSheet(t)
	cfg := PivotConfig{Source: "Feuille1", RowFields: []string{"Produit"}, Values: []PivotValue{{Field: "Montant", Agg: "sum"}}}
	sheet, id, read := pivotOf(t, d, cfg)
	id2, err := d.CreatePivotAt(ctx, sheet, 1, 6, cfg)
	if err != nil {
		t.Fatal(err)
	}
	second := func() PivotInfo {
		l, _ := d.Layout(ctx, sheet)
		for _, p := range l.Pivots {
			if p.ID == id2 {
				return p
			}
		}
		t.Fatal("second TCD introuvable")
		return PivotInfo{}
	}
	name2 := second().Config.Name
	cfg.Slicers = []PivotSlicer{{Field: "Région", Values: []string{"Sud"}, Row: 1, Col: 4, Pivots: []string{name2}}}
	must(t, d.SetPivot(ctx, id, cfg))
	p := read()
	if got := rowsText(p.Cells); got != "Produit,Somme de Montant | B,80 | A,40 | Total,120" {
		t.Fatalf("segment : %q", got)
	}
	if len(p.Slicers) != 1 || fmt.Sprint(p.Slicers[0].Items) != "[{Nord 3} {Sud 3}]" {
		t.Fatalf("éléments : %+v", p.Slicers)
	}
	if got := rowsText(second().Cells); got != "Produit,Somme de Montant | B,80 | A,40 | Total,120" {
		t.Fatalf("TCD relié : %q", got)
	}
	// Annuler : plus de segment, plus de filtre.
	d.bus.Undo(ctx)
	if got := rowsText(second().Cells); got != "Produit,Somme de Montant | A,100 | B,110 | Total,210" {
		t.Fatalf("annulé : %q", got)
	}
	// Chronologie par mois : janvier et février 2024.
	cfg.Slicers = []PivotSlicer{{Field: "Date", Kind: "timeline", Values: []string{"2024-01", "2024-02"}, Row: 1, Col: 4}}
	must(t, d.SetPivot(ctx, id, cfg))
	p = read()
	if got := rowsText(p.Cells); got != "Produit,Somme de Montant | A,10 | B,20 | Total,30" {
		t.Fatalf("chronologie : %q", got)
	}
	if got := fmt.Sprint(p.Slicers[0].Items); got != "[{2024-01 1} {2024-02 1} {2024-04 1} {2024-07 1} {2025-01 1} {2025-05 1}]" {
		t.Fatalf("périodes : %s", got)
	}
	// Déplacer le segment : même calcul (en cache).
	cfg.Slicers[0].Row, cfg.Slicers[0].OffX = 8, 12
	must(t, d.SetPivot(ctx, id, cfg))
	if got := read().Config.Slicers[0]; got.Row != 8 || got.OffX != 12 {
		t.Fatalf("segment déplacé : %+v", got)
	}
}

// Une colonne calculée du tableau source sert de champ partout (lignes,
// colonnes, valeurs, filtres), même quand sa formule en lit une autre.
func TestPivotComputedFields(t *testing.T) {
	ctx := context.Background()
	d := salesSheet(t)
	mustSet(t, d, 1, 0, 4, "Double")
	mustSet(t, d, 1, 1, 4, "=[@Montant]*2")
	mustSet(t, d, 1, 0, 5, "Taille")
	mustSet(t, d, 1, 1, 5, `=SI([@Double]>50;"Gros";"Petit")`)
	cfg := PivotConfig{Source: "Feuille1", RowFields: []string{"Taille"}, ColFields: []string{"Région"}, Values: []PivotValue{{Field: "Double", Agg: "sum"}}}
	_, id, read := pivotOf(t, d, cfg)
	if got := rowsText(read().Cells); got != "Taille,Nord,Sud,Total | Petit,20,40,60 | Gros,160,200,360 | Total,180,240,420" {
		t.Fatalf("colonnes calculées : %q", got)
	}
	cfg.Filters = []PivotFilter{{Field: "Taille", Values: []string{"Gros"}}}
	must(t, d.SetPivot(ctx, id, cfg))
	if got := rowsText(read().Cells); got != "Taille,Nord,Sud,Total | Gros,160,200,360 | Total,160,200,360" {
		t.Fatalf("filtre : %q", got)
	}
	// La source change : le TCD suit.
	mustSet(t, d, 1, 1, 3, "100")
	if got := rowsText(read().Cells); got != "Taille,Nord,Sud,Total | Gros,360,200,560 | Total,360,200,560" {
		t.Fatalf("après saisie : %q", got)
	}
}
