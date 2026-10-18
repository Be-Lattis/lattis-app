package doc

import (
	"context"
	"path/filepath"
	"testing"
)

// Un tableau converti en plage reste une plage après enregistrement et
// réouverture : écrire juste dessous ne refait pas de tableau.
func TestRangeSurvivesReopen(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "plage.lattis")
	d, err := New(ctx)
	must(t, err)
	set(t, d, 0, 0, "Produit")
	set(t, d, 0, 1, "Prix")
	set(t, d, 1, 0, "Clavier")
	set(t, d, 1, 1, "12,5")
	l, err := d.Layout(ctx, 1)
	must(t, err)
	if len(l.Tables) != 1 {
		t.Fatalf("tableau attendu : %+v", l.Tables)
	}
	must(t, d.ConvertToRange(ctx, l.Tables[0].Name))
	must(t, d.Save(ctx, path))
	saved := view(t, d)
	must(t, d.Close())

	d, err = Open(ctx, path)
	must(t, err)
	defer d.Close()
	if got := view(t, d); got != saved {
		t.Fatalf("contenu rouvert :\n%s\nattendu\n%s", got, saved)
	}
	l, err = d.Layout(ctx, 1)
	must(t, err)
	if len(l.Tables) != 0 || len(l.Ranges) != 1 {
		t.Fatalf("rouvert : tableaux %+v, plages %+v", l.Tables, l.Ranges)
	}
	set(t, d, 2, 0, "Souris")
	set(t, d, 2, 1, "8")
	l, err = d.Layout(ctx, 1)
	must(t, err)
	if len(l.Tables) != 0 {
		t.Fatalf("saisie sous la plage rouverte : tableau créé %+v", l.Tables)
	}
	checkOK(t, d)
	// L'historique aussi : annuler les saisies puis la conversion.
	for range 3 {
		_, err := d.Bus.Undo(ctx)
		must(t, err)
	}
	l, err = d.Layout(ctx, 1)
	must(t, err)
	if len(l.Tables) != 1 || len(l.Ranges) != 0 {
		t.Fatalf("conversion annulée après réouverture : %+v, %+v", l.Tables, l.Ranges)
	}
}
