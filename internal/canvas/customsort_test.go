package canvas

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/meta"
	"github.com/lalamefine/lattis/internal/store"
)

// columnValues lit une colonne d'un tableau dans l'ordre des lignes.
func columnValues(t *testing.T, d *fixture, table string, i int) string {
	t.Helper()
	var out []string
	var cols []ExportColumn
	err := d.ExportTable(context.Background(), table, func(c []ExportColumn) error {
		cols = c
		return nil
	}, func(vals []any) error {
		out = append(out, fmt.Sprint(formulaText(vals[i], cols[i].Type)))
		return nil
	})
	must(t, err)
	return strings.Join(out, ",")
}

// Tri personnalisé d'un tableau : liste prédéfinie « Mois », sans tenir
// compte de la casse ; valeurs hors liste ensuite (ordre croissant), vides
// à la fin ; à rebours ; une seule étape d'annulation ; ordre enregistré
// pour la colonne ; un tri ordinaire le remplace.
func TestSortTableCustom(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{
		{"Mois", "Ventes"},
		{"mars", "1"}, {"Janvier", "2"}, {"zzz", "3"}, {"", "4"}, {"février", "5"}, {"aaa", "6"}, {"AVRIL", "7"}, {"mars", "8"},
	}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	must(t, d.SortTableCustom(ctx, "Feuille1", 0, CustomOrder{List: "mois"}))
	if got := columnValues(t, d, "Feuille1", 0); got != "Janvier,février,mars,mars,AVRIL,aaa,zzz," {
		t.Fatalf("croissant : %s", got)
	}
	// Les égalités gardent l'ordre précédent.
	if got := columnValues(t, d, "Feuille1", 1); got != "2,5,1,8,7,6,3,4" {
		t.Fatalf("lignes suivies : %s", got)
	}
	check(t, d, "tri personnalisé")
	o, ok, applied, err := d.ColumnOrder(ctx, "Feuille1", 0)
	must(t, err)
	if !ok || !applied || o.List != "Mois" || len(o.Values) != 12 || o.Desc {
		t.Fatalf("ordre enregistré : %+v %v %v", o, ok, applied)
	}
	// Une seule étape d'annulation : ordre des lignes et ordre enregistré.
	d.bus.Undo(ctx)
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
	if _, ok, _, _ := d.ColumnOrder(ctx, "Feuille1", 0); ok {
		t.Fatal("ordre resté après l'annulation")
	}
	d.bus.Redo(ctx)
	if got := columnValues(t, d, "Feuille1", 0); got != "Janvier,février,mars,mars,AVRIL,aaa,zzz," {
		t.Fatalf("rétablir : %s", got)
	}
	// À rebours, valeurs données directement.
	must(t, d.SortTableCustom(ctx, "Feuille1", 0, CustomOrder{Values: []string{"zzz", "mars", "Février"}, Desc: true}))
	if got := columnValues(t, d, "Feuille1", 0); got != "février,mars,mars,zzz,aaa,AVRIL,Janvier," {
		t.Fatalf("décroissant : %s", got)
	}
	// Un tri ordinaire : l'ordre reste proposé, mais n'est plus le tri du tableau.
	must(t, d.SortTable(ctx, "Feuille1", 1, false))
	if o, ok, applied, _ := d.ColumnOrder(ctx, "Feuille1", 0); !ok || applied || len(o.Values) != 3 {
		t.Fatalf("après un tri ordinaire : %+v %v %v", o, ok, applied)
	}
	if err := d.SortTableCustom(ctx, "Feuille1", 0, CustomOrder{List: "Inconnue"}); err == nil {
		t.Fatal("liste inconnue acceptée")
	}
	if err := d.SortTableCustom(ctx, "Feuille1", 0, CustomOrder{}); err == nil {
		t.Fatal("ordre vide accepté")
	}
	// Renommer le tableau et la colonne : l'ordre suit.
	must(t, d.RenameTable(ctx, "Feuille1", "Ventes"))
	_, err = d.SetCell(ctx, 1, 0, 0, "Période")
	must(t, err)
	if o, ok, _, _ := d.ColumnOrder(ctx, "Ventes", 0); !ok || len(o.Values) != 3 {
		t.Fatalf("après renommage : %+v %v", o, ok)
	}
}

// Colonne calculée et colonne de dates : triées sur leur texte affiché.
func TestSortTableCustomComputed(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Code", "N"}, {"b", "1"}, {"a", "2"}, {"c", "3"}}); err != nil {
		t.Fatal(err)
	}
	must(t, d.InsertColumn(ctx, "Feuille1", 2, "Maj"))
	_, err := d.SetCell(ctx, 1, 1, 2, "=MAJUSCULE([@Code])")
	must(t, err)
	must(t, d.SortTableCustom(ctx, "Feuille1", 2, CustomOrder{Values: []string{"C", "A"}}))
	if got := columnValues(t, d, "Feuille1", 0); got != "c,a,b" {
		t.Fatalf("colonne calculée : %s", got)
	}
}

// Listes personnalisées du classeur : enregistrées, annulables, réutilisées
// par nom ; les listes prédéfinies ne s'enregistrent pas.
func TestCustomLists(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "doc.sqlite")
	openAt := func() (*fixture, func()) {
		db, err := store.Open(path, Register)
		must(t, err)
		must(t, meta.Migrate(ctx, db))
		bus := command.New(db)
		closed := false
		closeDB := func() {
			if !closed {
				closed = true
				db.Close()
			}
		}
		t.Cleanup(closeDB)
		return &fixture{New(db, bus), bus}, closeDB
	}
	d, closeDB := openAt()
	lists, err := d.CustomLists(ctx)
	must(t, err)
	if len(lists) != len(BuiltinLists) || lists[0].Name != "Jours" || lists[0].Values[6] != "dimanche" || !lists[0].Builtin {
		t.Fatalf("listes prédéfinies : %+v", lists)
	}
	must(t, d.SetCustomLists(ctx, append(lists, CustomList{Name: "Tailles", Values: []string{"S", "M", "", "L", "s", "XL"}})))
	lists, _ = d.CustomLists(ctx)
	if n := len(lists); n != len(BuiltinLists)+1 || lists[n-1].Name != "Tailles" || fmt.Sprint(lists[n-1].Values) != "[S M L XL]" {
		t.Fatalf("liste ajoutée : %+v", lists)
	}
	if err := d.SetCustomLists(ctx, []CustomList{{Name: "mois", Values: []string{"x"}}}); err == nil {
		t.Fatal("nom d'une liste prédéfinie accepté")
	}
	if err := d.SetCustomLists(ctx, []CustomList{{Name: "Vide"}}); err == nil {
		t.Fatal("liste vide acceptée")
	}
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Taille", "N"}, {"XL", "1"}, {"m", "2"}, {"S", "3"}, {"L", "4"}}); err != nil {
		t.Fatal(err)
	}
	must(t, d.SortTableCustom(ctx, "Feuille1", 0, CustomOrder{List: "Tailles"}))
	if got := columnValues(t, d, "Feuille1", 0); got != "S,m,L,XL" {
		t.Fatalf("liste du classeur : %s", got)
	}
	// Enregistrer, rouvrir : listes et ordre de la colonne relus.
	closeDB()
	d, _ = openAt()
	lists, _ = d.CustomLists(ctx)
	if lists[len(lists)-1].Name != "Tailles" {
		t.Fatalf("liste après réouverture : %+v", lists)
	}
	if o, ok, applied, _ := d.ColumnOrder(ctx, "Feuille1", 0); !ok || !applied || o.List != "Tailles" {
		t.Fatalf("ordre après réouverture : %+v %v", o, ok)
	}
	// Retirer la liste s'annule.
	must(t, d.SetCustomLists(ctx, nil))
	if lists, _ := d.CustomLists(ctx); len(lists) != len(BuiltinLists) {
		t.Fatalf("liste retirée : %+v", lists)
	}
	d.bus.Undo(ctx)
	if lists, _ := d.CustomLists(ctx); len(lists) != len(BuiltinLists)+1 {
		t.Fatalf("retrait annulé : %+v", lists)
	}
}

// Tableau chargé par une requête : son dernier tri personnalisé est
// ré-appliqué à l'actualisation (les lignes arrivent dans l'ordre de la
// requête), nouvelles valeurs comprises.
func TestCustomOrderKeptOnRefresh(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	load := handQuery(t, d, []string{"Jour", "N"}, [][]any{{"mardi", int64(1)}, {"lundi", int64(2)}, {"dimanche", int64(3)}})
	must(t, d.SortTableCustom(ctx, "Q", 0, CustomOrder{List: "Jours"}))
	if got := columnValues(t, d, "Q", 0); got != "lundi,mardi,dimanche" {
		t.Fatalf("tri : %s", got)
	}
	load([][]any{{"vendredi", int64(1)}, {"autre", int64(9)}, {"lundi", int64(2)}, {nil, int64(0)}, {"mercredi", int64(3)}})
	if got := columnValues(t, d, "Q", 0); got != "lundi,mercredi,vendredi,autre," {
		t.Fatalf("après actualisation : %s", got)
	}
	// Un tri ordinaire : l'actualisation garde l'ordre de la requête.
	must(t, d.SortTable(ctx, "Q", 1, false))
	load([][]any{{"vendredi", int64(1)}, {"lundi", int64(2)}})
	if got := columnValues(t, d, "Q", 0); got != "vendredi,lundi" {
		t.Fatalf("sans tri personnalisé : %s", got)
	}
}

// TCD : ordre personnalisé d'un champ (lignes et colonnes), gardé quand les
// données changent ; valeurs hors liste ensuite, vides à la fin ; nom de
// liste ; champ renommé.
func TestPivotCustomOrder(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{
		{"Taille", "Ventes"},
		{"L", "1"}, {"S", "2"}, {"XL", "3"}, {"", "4"}, {"M", "5"}, {"autre", "6"},
	}); err != nil {
		t.Fatal(err)
	}
	cfg := PivotConfig{Source: "Feuille1", RowFields: []string{"Taille"}, Values: []PivotValue{{Field: "Ventes", Agg: "sum"}},
		Orders: map[string]CustomOrder{"Taille": {Values: []string{"s", "M", "L", "XL"}}}}
	sheet, err := d.CreatePivot(ctx, cfg)
	must(t, err)
	labels := func() string {
		t.Helper()
		l, _ := d.Layout(ctx, sheet)
		p := l.Pivots[0]
		if p.Error != "" {
			t.Fatal(p.Error)
		}
		var out []string
		for _, r := range p.Cells[1:] {
			out = append(out, r[0])
		}
		return strings.Join(out, ",")
	}
	empty := strings.Split(labels(), ",")[5]
	if got, want := labels(), "S,M,L,XL,autre,"+empty+",Total"; got != want {
		t.Fatalf("ordre personnalisé : %s, voulu %s", got, want)
	}
	// Les données changent : nouvelle valeur de la liste, à sa place.
	_, err = d.SetCell(ctx, 1, 2, 0, "XS")
	must(t, err)
	l, _ := d.Layout(ctx, sheet)
	id := l.Pivots[0].ID
	cfg.Orders = map[string]CustomOrder{"Taille": {Values: []string{"XS", "S", "M", "L", "XL"}, Desc: true}}
	must(t, d.SetPivot(ctx, id, cfg))
	if got, want := labels(), "XL,L,M,XS,autre,"+empty+",Total"; got != want {
		t.Fatalf("à rebours : %s, voulu %s", got, want)
	}
	// Champ en colonnes, ordre donné par une liste nommée.
	if _, err := d.Paste(ctx, 1, 0, 2, [][]string{{"Jour"}, {"mardi"}, {"lundi"}, {"jeudi"}, {"lundi"}, {"dimanche"}, {"mardi"}}); err != nil {
		t.Fatal(err)
	}
	all, _ := d.AllTables(ctx)
	if len(all[0].Columns) != 3 {
		t.Fatalf("colonne collée hors du tableau : %+v", all)
	}
	cfg.Cols = "Jour"
	cfg.Orders = map[string]CustomOrder{"Jour": {List: "Jours"}}
	must(t, d.SetPivot(ctx, id, cfg))
	l, _ = d.Layout(ctx, sheet)
	if got := fmt.Sprint(l.Pivots[0].Cells[0][1:]); got != "[lundi mardi jeudi dimanche Total]" {
		t.Fatalf("colonnes : %s", got)
	}
	// Champ renommé : l'ordre suit.
	_, err = d.SetCell(ctx, 1, 0, 2, "Jour de la semaine")
	must(t, err)
	l, _ = d.Layout(ctx, sheet)
	if o, ok := l.Pivots[0].Config.Orders["Jour de la semaine"]; !ok || o.List != "Jours" {
		t.Fatalf("ordre après renommage : %+v", l.Pivots[0].Config.Orders)
	}
	if got := fmt.Sprint(l.Pivots[0].Cells[0][1:]); got != "[lundi mardi jeudi dimanche Total]" {
		t.Fatalf("colonnes après renommage : %s", got)
	}
}
