package formula

import (
	"slices"
	"testing"
)

// TestPositionDeps : LIGNES(Ventes) dépend du tableau (recalcul quand on
// ajoute une ligne), LIGNE(Cell[B3]) de la cellule.
func TestPositionDeps(t *testing.T) {
	names, tables := Deps(mustParse(t, `=LIGNES(Ventes)+COLONNES(Clients[Nom])`, FR))
	if len(names) != 0 || !slices.Equal(tables, []string{"ventes", "clients"}) {
		t.Errorf("LIGNES : %v %v", names, tables)
	}
	names, tables = Deps(mustParse(t, `=LIGNE(Cell[B3])`, FR))
	if len(names) != 1 || len(tables) != 0 {
		t.Errorf("LIGNE : %v %v", names, tables)
	}
}
