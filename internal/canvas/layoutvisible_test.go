package canvas

import (
	"context"
	"strconv"
	"testing"
)

// Grande feuille de cellules libres : d'abord les lignes demandées (formules
// calculées pareil), la mise en page complète ne change pas. Petite feuille :
// entière d'emblée.
func TestLayoutVisible(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Import(ctx, "Importer", []ImportSheet{{Name: "Petite", Cells: []ImportCell{{Row: 500, Col: 1, Raw: "x"}}}}); err != nil {
		t.Fatal(err)
	}
	var cells []ImportCell
	for r := range visibleCells {
		cells = append(cells, ImportCell{Row: r, Col: 0, Raw: "L" + strconv.Itoa(r)}, ImportCell{Row: r, Col: 3, Raw: "=1+" + strconv.Itoa(r)})
	}
	if _, err := d.Import(ctx, "Grande", []ImportSheet{{Name: "Grande", Cells: cells}}); err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	full, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range full.Sheets {
		ids[s.Name] = s.ID
	}

	small, err := d.LayoutVisible(ctx, ids["Petite"], 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if small.Partial || len(small.Cells) != 1 {
		t.Fatalf("petite feuille : partielle %v, %d cellules", small.Partial, len(small.Cells))
	}

	vis, err := d.LayoutVisible(ctx, ids["Grande"], 10, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !vis.Partial || !vis.Pending || len(vis.Cells) != 10 || vis.UsedRow != visibleCells-1 || vis.UsedCol != 3 {
		t.Fatalf("visible : partielle %v, en attente %v, %d cellules, jusqu'à %d,%d", vis.Partial, vis.Pending, len(vis.Cells), vis.UsedRow, vis.UsedCol)
	}
	whole, err := d.Layout(ctx, ids["Grande"])
	if err != nil {
		t.Fatal(err)
	}
	if whole.Partial || len(whole.Cells) != 2*visibleCells {
		t.Fatalf("complète : partielle %v, %d cellules", whole.Partial, len(whole.Cells))
	}
	byPos := map[[2]int]CellInfo{}
	for _, c := range whole.Cells {
		byPos[[2]int{c.Row, c.Col}] = c
	}
	for _, c := range vis.Cells {
		w := byPos[[2]int{c.Row, c.Col}]
		if c.Raw != w.Raw || c.Value != w.Value || c.Error != w.Error {
			t.Fatalf("cellule %d,%d : %+v et %+v", c.Row, c.Col, c, w)
		}
	}
}
