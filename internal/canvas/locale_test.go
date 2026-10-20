package canvas

import (
	"context"
	"testing"

	"github.com/lalamefine/lattis/internal/formula"
)

// La locale de la session (langue de l'interface) règle la lecture des
// nombres écrits en texte (Schema.Numbers) ; français par défaut.
func TestModelLocale(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	numbers := func() formula.Locale {
		t.Helper()
		var loc formula.Locale
		if err := d.read(ctx, func(t *tx) error {
			s, err := t.schema()
			if err == nil {
				loc = s.Numbers
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return loc
	}
	if d.Locale() != formula.FR || numbers() != formula.FR {
		t.Fatal("français par défaut")
	}
	d.SetLocale(formula.ForLanguage("en"))
	if numbers() != formula.EN {
		t.Fatalf("anglais : %+v", numbers())
	}
	d.SetLocale(formula.ForLanguage("de"))
	if n := numbers(); n.Dot {
		t.Fatalf("allemand : décimale « , » attendue, %+v", n)
	}
}
