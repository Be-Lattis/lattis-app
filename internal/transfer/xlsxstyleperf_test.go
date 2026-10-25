package transfer

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// 100 000 lignes mises en forme : les colonnes uniformes prennent un style
// de colonne (aucune cellule une à une), seules les cellules qui s'écartent
// sont écrites, et l'import reste rapide.
func TestXLSXImportStylesLarge(t *testing.T) {
	if testing.Short() {
		t.Skip("gros classeur")
	}
	ctx := context.Background()
	const n = 100_000
	var b strings.Builder
	b.WriteString(`<sheetData><row r="1"><c r="A1" s="7" t="inlineStr"><is><t>Code</t></is></c><c r="B1" s="7" t="inlineStr"><is><t>Prix</t></is></c><c r="C1" s="7" t="inlineStr"><is><t>Date</t></is></c><c r="D1" s="7" t="inlineStr"><is><t>Part</t></is></c></row>`)
	for r := 2; r <= n+1; r++ {
		part := 4
		if r%10_000 == 0 {
			part = 5 // une cellule sur 10 000 s'écarte de sa colonne
		}
		fmt.Fprintf(&b, `<row r="%d"><c r="A%d"><v>%d</v></c><c r="B%d" s="2"><v>%d.5</v></c><c r="C%d" s="3"><v>%d</v></c><c r="D%d" s="%d"><v>0.%d</v></c></row>`,
			r, r, r, r, r%100, r, 45000+r%300, r, part, r%10)
	}
	b.WriteString(`</sheetData>`)
	path := bookParts(t, [][2]string{{"Gros", b.String()}}, styleSheet, nil)
	d := open(t)
	start := time.Now()
	res, err := ImportXLSX(ctx, d.Model, path)
	if err != nil {
		t.Fatal(err)
	}
	took := time.Since(start)
	t.Logf("import de %d lignes mises en forme : %v", n, took)
	l, err := d.Layout(ctx, res.Sheets[0])
	if err != nil {
		t.Fatal(err)
	}
	tb := l.Tables[0]
	for j, want := range []string{"", "€", "date", "%"} {
		if got := styleOf(t, tb.Columns[j].Style)["nf"]; want != "" && got != want {
			t.Errorf("colonne %d : nf = %v, attendu %s", j, got, want)
		}
	}
	cells, err := d.TableCellStyles(ctx, tb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != n/10_000 {
		t.Errorf("%d cellules mises en forme une à une, attendu %d", len(cells), n/10_000)
	}
	if took > 30*time.Second {
		t.Errorf("import trop lent : %v", took)
	}
}
