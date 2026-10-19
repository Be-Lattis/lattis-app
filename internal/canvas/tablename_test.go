package canvas

import "testing"

// TestA1Like : les noms qui se lisent comme une adresse de cellule.
func TestA1Like(t *testing.T) {
	for _, n := range []string{"A1", "F10", "ab12", "ZZZZ999999999", "Data1", "R1C1", "r10c3"} {
		if !A1Like(n) {
			t.Errorf("%s : devrait être une adresse", n)
		}
	}
	for _, n := range []string{"Tableau1", "Ventes", "A0", "A01", "ABCDE1", "A1B", "F10_", "_F10", "A1234567890", "RC", "Feuille1"} {
		if A1Like(n) {
			t.Errorf("%s : n'est pas une adresse", n)
		}
	}
}
