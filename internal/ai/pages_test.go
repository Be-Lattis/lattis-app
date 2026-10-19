package ai

import "testing"

// L'assistant peut créer et corriger une page web (tableau de bord sur
// mesure) dans tous les modes de partage, même si les pages web sont
// coupées dans les réglages de l'utilisateur.
func TestPagesOffered(t *testing.T) {
	for _, mode := range []string{"schema-only", "sample", "full"} {
		got := map[string]bool{}
		for _, tool := range Tools(mode) {
			got[tool.Name] = true
		}
		if !got["pages_add"] || !got["pages_set"] {
			t.Errorf("%s : pages_add et pages_set absents", mode)
		}
	}
}
