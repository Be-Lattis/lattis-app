package canvas

import (
	"context"
	"testing"
)

func TestFavorites(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	before := dump(t, d)

	id, err := d.AddFavorite(ctx, "", "-- ventes\nSELECT *\nFROM Ventes\nLIMIT 100")
	if err != nil || id != 1 {
		t.Fatalf("AddFavorite = %d, %v", id, err)
	}
	// Même requête (aux espaces près) : pas de doublon.
	if again, err := d.AddFavorite(ctx, "Autre", "SELECT * FROM Ventes LIMIT 100\n-- ventes"); err != nil || again != 2 {
		t.Fatalf("requête différente (commentaire déplacé) : %d, %v", again, err)
	}
	if again, err := d.AddFavorite(ctx, "x", "-- ventes\nSELECT  *  FROM Ventes\n LIMIT 100"); err != nil || again != id {
		t.Fatalf("doublon : %d, %v", again, err)
	}
	if _, err := d.AddFavorite(ctx, "Vide", "  \n"); err == nil {
		t.Fatal("requête vide acceptée")
	}
	list, _ := d.Favorites(ctx)
	if len(list) != 2 || list[0].Name != "SELECT *" || list[1].Name != "Autre" {
		t.Fatalf("favoris : %+v", list)
	}
	if err := d.RenameFavorite(ctx, id, "  Toutes les ventes "); err != nil {
		t.Fatal(err)
	}
	if err := d.RenameFavorite(ctx, id, " "); err == nil {
		t.Fatal("nom vide accepté")
	}
	if err := d.DeleteFavorite(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteFavorite(ctx, 99); err == nil {
		t.Fatal("favori inconnu supprimé")
	}
	list, _ = d.Favorites(ctx)
	if len(list) != 1 || list[0].Name != "Toutes les ventes" || list[0].SQL != "-- ventes\nSELECT *\nFROM Ventes\nLIMIT 100" {
		t.Fatalf("après renommage et suppression : %+v", list)
	}
	check(t, d, "favoris")

	// Tout s'annule : ajout, ajout, renommage, suppression.
	for range 4 {
		if _, err := d.bus.Undo(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
	if st, _ := d.bus.State(ctx); st.Undo != "" {
		t.Fatalf("reste à annuler : %q", st.Undo)
	}
	for range 4 {
		if _, err := d.bus.Redo(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if list, _ = d.Favorites(ctx); len(list) != 1 || list[0].Name != "Toutes les ventes" {
		t.Fatalf("après rétablissement : %+v", list)
	}
}
