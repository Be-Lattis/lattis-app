package canvas

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/grid"
)

// TestMesureCriteres chronomètre les colonnes calculées dont le critère dépend
// de la ligne avec une comparaison (SOMME.SI(T[Qté];">"&[@Qté];…), RANG,
// cumul par client), puis un agrégat de ces colonnes dans une cellule libre :
// LATTIS_MESURE_CRITERES=30000 go test ./internal/canvas -run TestMesureCriteres -v
func TestMesureCriteres(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("LATTIS_MESURE_CRITERES"))
	if n == 0 {
		t.Skip("LATTIS_MESURE_CRITERES non défini")
	}
	ctx := context.Background()
	for i, f := range []string{
		`=SOMME.SI(T[Qté];">"&[@Qté];T[Qté])`,
		`=NB.SI(T[Qté];"<="&[@Qté])`,
		`=RANG([@Qté];T[Qté])`,
		`=SOMME.SI.ENS(T[Qté];T[Client];[@Client];T[Date];"<="&[@Date])`,
		`=NB.SI(T[Client];[@Client])`,
	} {
		d, sheet := criteresFixture(t, n)
		col := 3
		start := time.Now()
		if _, err := d.SetCell(ctx, sheet, 0, col, "K"+strconv.Itoa(i)); err != nil {
			t.Fatal(err)
		}
		if _, err := d.SetCell(ctx, sheet, 1, col, f); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Layout(ctx, sheet); err != nil {
			t.Fatal(err)
		}
		t.Logf("%-62s colonne + mise en page %8.0f ms", f, float64(time.Since(start).Microseconds())/1000)
		start = time.Now()
		if _, err := d.SetCell(ctx, sheet, 0, 12, fmt.Sprintf("=MOYENNE(T[K%d])", i)); err != nil {
			t.Fatal(err)
		}
		lay, err := d.Layout(ctx, sheet)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%-62s MOYENNE de la colonne    %8.0f ms (%v)", "", float64(time.Since(start).Microseconds())/1000, cellValue(lay, 12))
	}
}

// cellValue rend la valeur affichée de la cellule libre (ligne 0, colonne col).
func cellValue(l Layout, col int) any {
	for _, c := range l.Cells {
		if c.Row == 0 && c.Col == col {
			return c.Value
		}
	}
	return nil
}

// criteresFixture : un tableau T (Client, Qté, Date) de n lignes.
func criteresFixture(t *testing.T, n int) (*fixture, int64) {
	ctx := context.Background()
	d := open(t)
	_, err := d.Import(ctx, "Importer", []ImportSheet{{Name: "Données", Tables: []ImportTable{{
		Name: "T", Headers: []string{"Client", "Qté", "Date"},
		Types: []grid.Type{grid.Text, grid.Integer, grid.Date},
		Count: n,
		Rows: func(yield func([]any) error) error {
			for i := range n {
				if err := yield([]any{"C" + strconv.Itoa(i%500), int64(i*7919%1000 + 1), fmt.Sprintf("2026-%02d-%02d", i%12+1, i%28+1)}); err != nil {
					return err
				}
			}
			return nil
		},
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	var sheet int64
	d.read(ctx, func(t *tx) error {
		return t.row(`SELECT sheet FROM _lattis_tables WHERE name = 'T'`).Scan(&sheet)
	})
	return d, sheet
}
