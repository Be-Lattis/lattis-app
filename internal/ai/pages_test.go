package ai

import (
	"strings"
	"testing"
)

// L'assistant ne propose une page web (tableau de bord sur mesure) que
// quand les pages web sont permises (réglage features.htmlPages, coupé par
// défaut, jamais en web) : ni ses outils, ni la consigne qui en parle.
func TestPagesOffered(t *testing.T) {
	for _, mode := range []string{"schema-only", "sample", "full"} {
		for _, on := range []bool{false, true} {
			got := map[string]bool{}
			for _, tool := range ToolsWith(mode, on) {
				got[tool.Name] = true
			}
			if got["pages_add"] != on || got["pages_set"] != on {
				t.Errorf("%s, pages %v : pages_add %v, pages_set %v", mode, on, got["pages_add"], got["pages_set"])
			}
			if strings.Contains(SystemWith(mode, on), "pages_add") != on {
				t.Errorf("%s, pages %v : consigne", mode, on)
			}
		}
		// Par défaut (configuration) : coupées.
		for _, tool := range Tools(mode) {
			if tool.Name == "pages_add" {
				t.Errorf("%s : pages_add proposé par défaut", mode)
			}
		}
		if strings.Contains(System(mode), "pages_add") {
			t.Errorf("%s : consigne des pages par défaut", mode)
		}
	}
}
