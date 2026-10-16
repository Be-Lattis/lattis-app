package demo

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/slitable/internal/canvas"
	"github.com/lalamefine/slitable/internal/doc"
	"github.com/lalamefine/slitable/internal/formula"
)

func build(t *testing.T, preset string, seed uint64) (*doc.Document, Result) {
	t.Helper()
	start := time.Now()
	d, res, err := New(context.Background(), preset, Options{Seed: seed})
	if err != nil {
		t.Fatalf("%s : %v", preset, err)
	}
	t.Cleanup(func() { d.Close() })
	t.Logf("%s : %d lignes, %d onglets en %v", preset, res.Rows, len(res.Sheets), time.Since(start).Round(time.Millisecond))
	if len(res.Warnings) > 0 {
		t.Fatalf("%s : avertissements %v", preset, res.Warnings)
	}
	return d, res
}

// check vérifie que chaque onglet s'affiche sans erreur : TCD, graphiques,
// cellules libres et colonnes calculées.
func check(t *testing.T, d *doc.Document) {
	t.Helper()
	ctx := context.Background()
	l, err := d.Layout(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range l.Sheets {
		sl, err := d.Layout(ctx, s.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range sl.Pivots {
			if p.Error != "" || len(p.Cells) < 3 {
				t.Errorf("%s : TCD %s : %q, %d lignes", s.Name, p.Config.Name, p.Error, len(p.Cells))
			}
		}
		for _, c := range sl.Charts {
			if c.Error != "" || len(c.Labels) == 0 {
				t.Errorf("%s : graphique %q : %q, %d points", s.Name, c.Config.Title, c.Error, len(c.Labels))
			}
		}
		for _, c := range sl.Cells {
			if c.Error || strings.HasPrefix(c.Value, "#") {
				t.Errorf("%s : cellule %s = %q", s.Name, c.Raw, c.Value)
			}
		}
		for _, tb := range sl.Tables {
			n := 0
			err := d.ExportTable(ctx, tb.Name, func([]canvas.ExportColumn) error { return nil }, func(row []any) error {
				for k, v := range row {
					if fv, ok := v.(formula.Value); ok && fv.Kind == formula.Error {
						return fmt.Errorf("ligne %d, colonne %s : %s", n+1, tb.Columns[k].Name, fv.String())
					}
				}
				n++
				return nil
			})
			if err != nil {
				t.Errorf("%s[%s] : %v", s.Name, tb.Name, err)
			}
			if n != tb.Total {
				t.Errorf("%s[%s] : %d lignes lues sur %d", s.Name, tb.Name, n, tb.Total)
			}
		}
	}
}

func shape(t *testing.T, d *doc.Document) []string {
	t.Helper()
	ctx := context.Background()
	l, err := d.Layout(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, s := range l.Sheets {
		sl, err := d.Layout(ctx, s.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, tb := range sl.Tables {
			out = append(out, fmt.Sprintf("%s/%s:%dx%d", s.Name, tb.Name, tb.Total, len(tb.Columns)))
		}
		for range sl.Pivots {
			out = append(out, s.Name+"/TCD")
		}
		for range sl.Charts {
			out = append(out, s.Name+"/graphique")
		}
	}
	return out
}

func TestPresetsPetits(t *testing.T) {
	for _, p := range []string{"sml", "med", "refs", "1Kx10", "1Kx100"} {
		t.Run(p, func(t *testing.T) {
			d, _ := build(t, p, 0)
			check(t, d)
			if dirty, _ := d.Dirty(context.Background()); dirty {
				t.Error("le classeur de démonstration ne doit pas être marqué modifié")
			}
			t.Log(shape(t, d))
		})
	}
}

func TestFormes(t *testing.T) {
	want := map[string]string{
		"sml":    "[Commandes/Commandes:100x10]",
		"1Kx100": "[Relevés/Relevés:1000x100]",
		"med":    "[Commandes/Commandes:100x10 Clients/Clients:1000x10 Employés/Employés:1200x10 Analyse/TCD Analyse/graphique Analyse/graphique]",
		"refs":   "[Référentiel/Catégories:7x4 Référentiel/Produits:24x9 Clients/Clients:150x8 Ventes/Commandes:600x7 Ventes/Lignes:2000x9]",
	}
	for p, w := range want {
		d, _ := build(t, p, 0)
		if got := fmt.Sprint(shape(t, d)); got != w {
			t.Errorf("%s :\n got %s\nwant %s", p, got, w)
		}
	}
}

func TestBigPlan(t *testing.T) {
	p, _ := Lookup("big")
	plan := p.Plan()
	if len(plan.Sheets) != 3+BigSheets || len(plan.Pivots) != 1 {
		t.Fatalf("%d onglets, %d TCD", len(plan.Sheets), len(plan.Pivots))
	}
	var got []string
	for _, s := range plan.Sheets[3:] {
		got = append(got, fmt.Sprintf("%dx%d", s.Tables[0].Rows, len(s.Tables[0].Cols)))
	}
	if g := strings.Join(got, " "); g != "100x26 268x5 720x50 1931x10 5179x36 13895x13 37276x7 100000x19" {
		t.Errorf("tailles : %s", g)
	}
}

func TestDeterministe(t *testing.T) {
	ctx := context.Background()
	dump := func(seed uint64) string {
		d, _ := build(t, "refs", seed)
		var b strings.Builder
		err := d.ExportTable(ctx, "Lignes", func([]canvas.ExportColumn) error { return nil }, func(row []any) error {
			fmt.Fprintln(&b, row...)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	a, b, c := dump(1), dump(1), dump(2)
	if a != b {
		t.Error("même graine, données différentes")
	}
	if a == c {
		t.Error("graines différentes, mêmes données")
	}
}

func TestPresetInconnu(t *testing.T) {
	if _, _, err := New(context.Background(), "énorme", Options{}); err == nil || !strings.Contains(err.Error(), "1Kx10") {
		t.Fatalf("erreur attendue avec la liste des presets, obtenu %v", err)
	}
	if _, ok := Lookup("1kX100"); !ok {
		t.Error("les noms ne tiennent pas compte de la casse")
	}
}

// TestPresetsLourds fabrique les gros presets (SLITABLE_DEMO_LOURD=1, ou
// =big,100Kx100 pour en choisir).
func TestPresetsLourds(t *testing.T) {
	v := os.Getenv("SLITABLE_DEMO_LOURD")
	if v == "" {
		t.Skip("SLITABLE_DEMO_LOURD non défini")
	}
	list := []string{"10Kx10", "10Kx100", "100Kx10", "big"}
	if v != "1" {
		list = strings.Split(v, ",")
	}
	for _, p := range list {
		t.Run(p, func(t *testing.T) { build(t, p, 0) })
	}
}
