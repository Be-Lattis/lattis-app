package doc

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
)

// Le plan (groupes et état réduit) part dans le fichier et revient.
func TestOutlineSaved(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "plan.lattis")
	d, err := New(ctx)
	must(t, err)
	must(t, d.GroupLines(ctx, 1, true, 3, 6))
	must(t, d.GroupLines(ctx, 1, false, 1, 2))
	must(t, d.FoldGroup(ctx, 1, true, 1, 4, true))
	must(t, d.Save(ctx, path))
	must(t, d.Close())

	d, err = Open(ctx, path)
	must(t, err)
	defer d.Close()
	l, err := d.Layout(ctx, 1)
	must(t, err)
	if l.Outline == nil || len(l.Outline.Rows) != 4 || len(l.Outline.Cols) != 2 || !slices.Equal(l.Outline.HiddenRows, []int{3, 4, 5, 6}) {
		t.Fatalf("plan relu : %+v", l.Outline)
	}
	checkOK(t, d)
}
