package canvas

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// searchBook : Feuille1 avec un tableau (Ville, Prix, Double calculée), une
// cellule libre, une formule libre et des notes ; Feuille2 avec du texte.
func searchBook(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Prix"}, {"Paris", "10"}, {"Évry", "25"}, {"paris-sud", "7"}, {"Lyon*", "3"}}); err != nil {
		t.Fatal(err)
	}
	if err := promoteIfData(ctx, d, "Feuille1"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		row, col int
		raw      string
	}{{0, 2, "Double"}, {1, 2, "=[Prix]*2"}, {7, 0, "voir Paris"}, {8, 0, "=SOMME(Feuille1[Prix])"}, {9, 0, "a?b"}} {
		if _, err := d.SetCell(ctx, 1, c.row, c.col, c.raw); err != nil {
			t.Fatalf("%d,%d %q : %v", c.row, c.col, c.raw, err)
		}
	}
	if err := d.SetNote(ctx, 1, 2, 1, "prix de Paris"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetNote(ctx, 1, 7, 0, "à revoir"); err != nil {
		t.Fatal(err)
	}
	s2, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, s2, 3, 3, "Paris encore"); err != nil {
		t.Fatal(err)
	}
	return d
}

func foundCells(r SearchResult) string {
	var b []string
	for _, f := range r.Found {
		b = append(b, fmt.Sprintf("%d:%d,%d", f.Sheet, f.Row, f.Col))
	}
	return strings.Join(b, " ")
}

func TestSearchOptions(t *testing.T) {
	ctx := context.Background()
	d := searchBook(t)
	find := func(q SearchQuery) SearchResult {
		t.Helper()
		if q.Sheet == 0 && !q.Book {
			q.Sheet = 1
		}
		r, err := d.Search(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	for _, c := range []struct {
		name string
		q    SearchQuery
		want string
	}{
		{"sans la casse", SearchQuery{Text: "paris"}, "1:1,0 1:3,0 1:7,0"},
		{"avec la casse", SearchQuery{Text: "paris", Case: true}, "1:3,0"},
		{"cellule entière", SearchQuery{Text: "paris", Whole: true}, "1:1,0"},
		{"joker *", SearchQuery{Text: "p*s", Whole: true}, "1:1,0"},
		{"joker ?", SearchQuery{Text: "?yon", Whole: false}, "1:4,0"},
		{"~* littéral", SearchQuery{Text: "n~*", Whole: false}, "1:4,0"},
		{"~? littéral", SearchQuery{Text: "a~?b"}, "1:9,0"},
		{"? sans ~ : un caractère", SearchQuery{Text: "a?b", Whole: true}, "1:9,0"},
		{"accent sans la casse", SearchQuery{Text: "évry"}, "1:2,0"},
		{"accent avec la casse", SearchQuery{Text: "évry", Case: true}, ""},
		{"nombres", SearchQuery{Text: "25"}, "1:2,1"},
		{"en-tête", SearchQuery{Text: "doub"}, "1:0,2"},
		// Formules : celle de la cellule libre, celle de la colonne calculée (une fois).
		{"formules", SearchQuery{Text: "prix", Look: LookFormulas}, "1:0,1 1:1,2 1:8,0"},
		// Valeurs : les résultats (20, 50, 14, 6 ; somme 45).
		{"valeurs calculées", SearchQuery{Text: "50", Look: LookValues}, "1:2,2"},
		{"valeur d'une formule libre", SearchQuery{Text: "45", Look: LookValues, Whole: true}, "1:8,0"},
		{"pas de formule dans les valeurs", SearchQuery{Text: "somme", Look: LookValues}, ""},
		{"notes", SearchQuery{Text: "paris", Look: LookNotes}, "1:2,1"},
		{"notes libres", SearchQuery{Text: "revoir", Look: LookNotes}, "1:7,0"},
		{"par colonnes", SearchQuery{Text: "r", ByCols: true}, "1:1,0 1:2,0 1:3,0 1:7,0 1:8,0 1:0,1 1:1,2"},
		{"dans la sélection", SearchQuery{Text: "paris", In: &Rect{R0: 2, C0: 0, R1: 8, C1: 0}}, "1:3,0 1:7,0"},
	} {
		if got := foundCells(find(c.q)); got != c.want {
			t.Errorf("%s : %q, attendu %q", c.name, got, c.want)
		}
	}
	// Tout le classeur : une feuille par appel, puis la suivante.
	r := find(SearchQuery{Text: "paris", Book: true, Whole: false})
	if r.Sheet != 1 || r.Next == 0 || r.Total != 3 || r.Name != "Feuille1" {
		t.Fatalf("première feuille : %+v", r)
	}
	r2 := find(SearchQuery{Text: "paris", Book: true, Sheet: r.Next})
	if foundCells(r2) != fmt.Sprintf("%d:3,3", r.Next) || r2.Next != 0 {
		t.Fatalf("seconde feuille : %+v", r2)
	}
	if f := find(SearchQuery{Text: "paris"}).Found[0]; f.Table != "Feuille1" || f.Text != "Paris" {
		t.Fatalf("cellule trouvée : %+v", f)
	}
}

// Les tableaux croisés ne sont pas parcourus.
func TestSearchSkipsPivots(t *testing.T) {
	ctx := context.Background()
	d := searchBook(t)
	if _, err := d.CreatePivotAt(ctx, 1, 20, 0, PivotConfig{Source: "Feuille1", RowFields: []string{"Ville"}, Values: []PivotValue{{Field: "Prix", Agg: "sum"}}}); err != nil {
		t.Fatal(err)
	}
	r, err := d.Search(ctx, SearchQuery{Sheet: 1, Text: "lyon"})
	if err != nil || foundCells(r) != "1:4,0" {
		t.Fatalf("TCD parcouru : %q %v", foundCells(r), err)
	}
}

// Au-delà de la limite, la liste s'arrête et le total compte tout.
func TestSearchLimitCount(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	rows := [][]string{{"Nom", "Code"}}
	for i := range MaxMatches + 300 {
		rows = append(rows, []string{fmt.Sprintf("x%d", i), "ab"})
	}
	if _, err := d.Paste(ctx, 1, 0, 0, rows); err != nil {
		t.Fatal(err)
	}
	for _, byCols := range []bool{false, true} {
		r, err := d.Search(ctx, SearchQuery{Sheet: 1, Text: "*", ByCols: byCols})
		if err != nil {
			t.Fatal(err)
		}
		want := 2*(MaxMatches+300) + 2
		if len(r.Found) != MaxMatches || r.Total != want {
			t.Fatalf("par colonnes %v : %d trouvées, total %d (attendu %d)", byCols, len(r.Found), r.Total, want)
		}
		if byCols && r.Found[1].Col != 0 || !byCols && r.Found[1].Col != 1 {
			t.Fatalf("ordre (par colonnes %v) : %+v", byCols, r.Found[:3])
		}
	}
}

func TestReplaceAllOptions(t *testing.T) {
	ctx := context.Background()
	d := searchBook(t)
	before := dump(t, d)
	// Tout le classeur, sans la casse : tableau, cellule libre, autre feuille ;
	// une seule étape d'annulation.
	res, err := d.ReplaceAll(ctx, SearchQuery{Text: "paris", Book: true}, "Nice")
	if err != nil || res.Replaced != 4 || res.Skipped != 0 {
		t.Fatalf("ReplaceAll = %+v, %v", res, err)
	}
	got := view(t, d)
	if !strings.Contains(got, "Nice") || strings.Contains(strings.ToLower(got), "paris") {
		t.Fatalf("feuille :\n%s", got)
	}
	if n, _ := d.Search(ctx, SearchQuery{Sheet: 2, Text: "Nice encore", Whole: true}); len(n.Found) != 1 {
		t.Fatalf("Feuille2 non remplacée : %+v", n)
	}
	check(t, d, "remplacer tout")
	d.bus.Undo(ctx)
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
	// Jokers : la partie trouvée est remplacée.
	if res, err := d.ReplaceAll(ctx, SearchQuery{Sheet: 1, Text: "p*s", Whole: true}, "Lille"); err != nil || res.Replaced != 1 {
		t.Fatalf("joker = %+v, %v", res, err)
	}
	d.bus.Undo(ctx)
	// Formules : la colonne calculée et la formule libre sont réécrites.
	res, err = d.ReplaceAll(ctx, SearchQuery{Sheet: 1, Text: "~*2", Look: LookFormulas, Whole: false}, "*3")
	if err != nil || res.Replaced != 1 {
		t.Fatalf("formule de colonne = %+v, %v", res, err)
	}
	r, _ := d.Search(ctx, SearchQuery{Sheet: 1, Text: "30", Look: LookValues, Whole: true})
	if foundCells(r) != "1:1,2" {
		t.Fatalf("colonne recalculée : %q", foundCells(r))
	}
	if res, err := d.ReplaceAll(ctx, SearchQuery{Sheet: 1, Text: "somme", Look: LookFormulas}, "MAX"); err != nil || res.Replaced != 1 {
		t.Fatalf("formule libre = %+v, %v", res, err)
	}
	if r, _ := d.Search(ctx, SearchQuery{Sheet: 1, Text: "25", Look: LookValues, Whole: true}); foundCells(r) != "1:2,1 1:8,0" {
		t.Fatalf("formule libre recalculée : %q", foundCells(r))
	}
	// Une formule devenue illisible est laissée et comptée.
	if res, err := d.ReplaceAll(ctx, SearchQuery{Sheet: 1, Text: "MAX(", Look: LookFormulas}, "MAX(("); err != nil || res.Replaced != 0 || res.Skipped != 1 || res.Reason == "" {
		t.Fatalf("formule refusée = %+v, %v", res, err)
	}
	// Dans les valeurs : ce sont les formules qui se réécrivent.
	d.bus.Undo(ctx)
	d.bus.Undo(ctx)
	if got := dump(t, d); got != before {
		t.Fatalf("annulation des formules :\n%s", diff(before, got))
	}
	// Notes.
	if res, err := d.ReplaceAll(ctx, SearchQuery{Sheet: 1, Text: "paris", Look: LookNotes}, "Lyon"); err != nil || res.Replaced != 1 {
		t.Fatalf("notes = %+v, %v", res, err)
	}
	if r, _ := d.Search(ctx, SearchQuery{Sheet: 1, Text: "prix de Lyon", Look: LookNotes, Whole: true}); foundCells(r) != "1:2,1" {
		t.Fatalf("note remplacée : %q", foundCells(r))
	}
	d.bus.Undo(ctx)
	// Une seule cellule (Remplacer) : In sur elle.
	if res, err := d.ReplaceAll(ctx, SearchQuery{Sheet: 1, Text: "paris", In: &Rect{R0: 3, C0: 0, R1: 3, C1: 0}}, "Orly"); err != nil || res.Replaced != 1 {
		t.Fatalf("une cellule = %+v, %v", res, err)
	}
	d.bus.Undo(ctx)
	if got := dump(t, d); got != before {
		t.Fatalf("annulation :\n%s", diff(before, got))
	}
}

// Feuille protégée : ses cellules sont laissées et comptées, les autres
// feuilles remplacées.
func TestReplaceAllSkipsProtected(t *testing.T) {
	ctx := context.Background()
	d := searchBook(t)
	if err := d.ProtectSheet(ctx, 1, ""); err != nil {
		t.Fatal(err)
	}
	res, err := d.ReplaceAll(ctx, SearchQuery{Text: "paris", Book: true}, "Nice")
	if err != nil || res.Replaced != 1 || res.Skipped != 3 || res.Reason == "" {
		t.Fatalf("ReplaceAll = %+v, %v", res, err)
	}
	if r, _ := d.Search(ctx, SearchQuery{Sheet: 1, Text: "paris"}); len(r.Found) != 3 {
		t.Fatalf("feuille protégée modifiée : %+v", r)
	}
}

// Classeur partagé : un tableau en lecture seule est laissé ; sans droit de
// conception, seules les données des tableaux se remplacent.
func TestReplaceAllSharedRights(t *testing.T) {
	ctx := context.Background()
	d := searchBook(t)
	d.SetSharing(lockAll{})
	res, err := d.ReplaceAll(ctx, SearchQuery{Sheet: 1, Text: "paris"}, "Nice")
	if err != nil || res.Replaced != 1 || res.Skipped != 2 || !strings.Contains(res.Reason, "lecture seule") {
		t.Fatalf("lecture seule = %+v, %v", res, err)
	}
	d.bus.Undo(ctx)
	d.SetSharing(layoutLocked{})
	res, err = d.ReplaceAll(ctx, SearchQuery{Sheet: 1, Text: "paris"}, "Nice")
	if err != nil || res.Replaced != 2 || res.Skipped != 1 {
		t.Fatalf("sans conception = %+v, %v", res, err)
	}
}
