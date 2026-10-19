package canvas

import (
	"context"
	"reflect"
	"testing"
)

func TestPageSetup(t *testing.T) {
	d := open(t)
	ctx := context.Background()
	get := func(sheet int64) PageSetup {
		t.Helper()
		l, err := d.Layout(ctx, sheet)
		if err != nil {
			t.Fatal(err)
		}
		return l.PageSetup
	}
	if !get(1).IsZero() {
		t.Fatalf("mise en page par défaut : %+v", get(1))
	}
	p := PageSetup{Paper: "A3", Landscape: true, Margins: &MarginsWide, FitWide: 1,
		Area: &Rect{R0: 9, C0: 3, R1: 1, C1: 0}, TitleRows: &Span{From: 1, To: 0},
		RowBreaks: []int{20, 5, 5, 0}, Header: HeaderFooter{Center: "&[Onglet]"}, Footer: HeaderFooter{Right: "Page &[Page] sur &[Pages]"}}
	if err := d.SetPageSetup(ctx, 1, p); err != nil {
		t.Fatal(err)
	}
	got := get(1)
	if *got.Area != (Rect{R0: 1, C0: 0, R1: 9, C1: 3}) || *got.TitleRows != (Span{0, 1}) || !reflect.DeepEqual(got.RowBreaks, []int{5, 20}) || got.Footer.Right == "" {
		t.Fatalf("mise en page relue : %+v", got)
	}
	state := func() string { s, _ := d.bus.State(ctx); return s.Undo + "|" + s.Redo }
	if s := state(); s != "Mise en page|" {
		t.Fatalf("étape : %q", s)
	}
	// Même mise en page : pas d'étape (Rétablir reste possible).
	d.bus.Undo(ctx)
	if err := d.SetPageSetup(ctx, 1, PageSetup{Paper: "A4", Scale: 100}); err != nil || state() != "|Mise en page" {
		t.Fatalf("même mise en page : %v %s", err, state())
	}
	d.bus.Redo(ctx)

	// Insérer deux lignes au-dessus : zone, titres et sauts suivent ;
	// supprimer la ligne du titre la retire.
	if err := d.InsertSheetRows(ctx, 1, 0, 2); err != nil {
		t.Fatal(err)
	}
	got = get(1)
	if got.Area.R0 != 3 || got.Area.R1 != 11 || *got.TitleRows != (Span{2, 3}) || !reflect.DeepEqual(got.RowBreaks, []int{7, 22}) {
		t.Fatalf("après insertion : %+v %+v %v", got.Area, got.TitleRows, got.RowBreaks)
	}
	if err := d.DeleteSheetRows(ctx, 1, 2, 3); err != nil {
		t.Fatal(err)
	}
	if got = get(1); got.TitleRows != nil || got.Area.R0 != 2 || got.Area.R1 != 9 || !reflect.DeepEqual(got.RowBreaks, []int{5, 20}) {
		t.Fatalf("après suppression : %+v %+v %v", got.Area, got.TitleRows, got.RowBreaks)
	}
	if err := d.DeleteSheetColumns(ctx, 1, 0, 0); err != nil {
		t.Fatal(err)
	}
	if got = get(1); got.Area.C0 != 0 || got.Area.C1 != 2 {
		t.Fatalf("après suppression de colonne : %+v", got.Area)
	}
	d.bus.Undo(ctx)
	d.bus.Undo(ctx)
	d.bus.Undo(ctx)
	if got = get(1); got.Area.R0 != 1 || *got.TitleRows != (Span{0, 1}) {
		t.Fatalf("après annulations : %+v %+v", got.Area, got.TitleRows)
	}

	// Copie et suppression de la feuille.
	cp, err := d.CopySheet(ctx, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := get(cp); got.Paper != "A3" || !got.Landscape {
		t.Fatalf("mise en page de la copie : %+v", got)
	}
	if err := d.DeleteSheet(ctx, cp); err != nil {
		t.Fatal(err)
	}
	d.bus.Undo(ctx)
	if got := get(cp); got.Paper != "A3" {
		t.Fatalf("mise en page de la feuille rendue : %+v", got)
	}

	// Retour aux réglages par défaut : la ligne part ; l'annulation la rend.
	if err := d.SetPageSetup(ctx, 1, PageSetup{}); err != nil {
		t.Fatal(err)
	}
	if !get(1).IsZero() {
		t.Fatal("mise en page non effacée")
	}
	d.bus.Undo(ctx)
	if get(1).Paper != "A3" {
		t.Fatal("annulation de l'effacement")
	}
	for _, bad := range []PageSetup{{Paper: "A0"}, {Scale: 5}, {FitWide: -1}, {Margins: &Margins{Left: 15, Right: 15}}} {
		if err := d.SetPageSetup(ctx, 1, bad); err == nil {
			t.Errorf("mise en page %+v acceptée", bad)
		}
	}
	if err := d.SetPageSetup(ctx, 99, p); err == nil {
		t.Error("feuille inconnue acceptée")
	}
	check(t, d, "mise en page")
}

func TestPageSetupA1(t *testing.T) {
	r, err := ParseA1Range("$B$2:A10")
	if err != nil || r != (Rect{R0: 1, C0: 0, R1: 9, C1: 1}) || r.A1() != "A2:B10" {
		t.Fatalf("plage : %+v %v", r, err)
	}
	s, err := ParseLineSpan("$1:$2", true)
	if err != nil || s != (Span{0, 1}) || s.RowsA1() != "1:2" {
		t.Fatalf("lignes : %+v %v", s, err)
	}
	s, err = ParseLineSpan("c:aa", false)
	if err != nil || s != (Span{2, 26}) || s.ColsA1() != "C:AA" {
		t.Fatalf("colonnes : %+v %v", s, err)
	}
	for _, bad := range []string{"", "A", "1", "A0", "A1:"} {
		if _, err := ParseA1Range(bad); err == nil {
			t.Errorf("plage %q acceptée", bad)
		}
	}
}
