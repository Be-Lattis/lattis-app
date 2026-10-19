package canvas

import (
	"context"
	"errors"
	"testing"
)

// Règles des noms de tableau, vérifiées aussi dans l'interface avant l'envoi
// (frontend/src/lib/ui/names.ts, tableNameProblem) : mêmes cas des deux
// côtés, pour que la boîte « nom invalide » dise la même chose que le moteur.
func TestTableNameRules(t *testing.T) {
	valid := func(n string) bool { return n != "" && identifier(n) == n && !A1Like(n) }
	for _, n := range []string{"Ventes", "Ventes_2024", "_brouillon", "Élèves", "T1x", "F10_"} {
		if !valid(n) {
			t.Errorf("%q devrait être un nom de tableau", n)
		}
	}
	var m Model
	for _, n := range []string{"2024 ventes", "Mes ventes", "Ventes-2024", "2024", "Prix€", "F10", "AB12", "r1c1", "", "   "} {
		if valid(n) {
			t.Errorf("%q ne devrait pas être un nom de tableau", n)
		}
		// Refusé avant toute écriture (le modèle vide n'est pas touché).
		if err := m.RenameTable(context.Background(), "Tableau1", n); !errors.Is(err, ErrTable) {
			t.Errorf("RenameTable(%q) : %v, ErrTable attendu", n, err)
		}
	}
}
