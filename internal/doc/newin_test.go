package doc

import (
	"context"
	"testing"

	"github.com/lalamefine/lattis/internal/msg"
)

// Un classeur nouveau nomme sa feuille dans la langue de l'interface, sans
// étape d'annulation ni modification à enregistrer.
func TestNewInLanguage(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	for lang, want := range map[msg.Lang]string{msg.FR: "Feuille1", msg.EN: "Sheet1", msg.DE: "Tabelle1", msg.ES: "Hoja1", msg.ZH: "Sheet1"} {
		d, err := NewIn(ctx, lang)
		must(t, err)
		l, err := d.Layout(ctx, 0)
		must(t, err)
		if len(l.Sheets) != 1 || l.Sheets[0].Name != want {
			t.Errorf("%s : feuilles %+v, voulu %s", lang, l.Sheets, want)
		}
		if dirty, err := d.Dirty(ctx); err != nil || dirty || !d.Pristine() {
			t.Errorf("%s : classeur modifié (%v, %v, vierge %v)", lang, dirty, err, d.Pristine())
		}
		if st, err := d.Bus.State(ctx); err != nil || st.Undo != "" {
			t.Errorf("%s : annulation %q (%v)", lang, st.Undo, err)
		}
		must(t, d.Close())
	}
}
