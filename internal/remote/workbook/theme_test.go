package workbook_test

import (
	"context"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// Le thème de couleur du classeur part avec la mise en page, seul de
// _lattis_info : le lecteur voit les couleurs de l'auteur.
func TestThemePublished(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	fake := workbooktest.New()
	a := author(t)
	must(t, a.SetTheme(ctx, &canvas.Theme{ID: "bleu-orange", Name: "Bleu et orange", Table: "#2563eb", Source: "#ea580c"}))
	info := publish(t, a, fake.As("alice"), workbook.Frozen, "Ventes")
	lay := string(fake.Meta(info.WorkbookID, "layout"))
	if !strings.Contains(lay, `"theme":{"id":"bleu-orange"`) || strings.Contains(lay, "_lattis_info") {
		t.Fatalf("mise en page publiée : %s", lay)
	}
	must(t, fake.Grant(info.WorkbookID, "bob", "reader"))
	r := open(t, fake.As("bob"), info.WorkbookID, "bob")
	th, err := r.Theme(ctx)
	must(t, err)
	if th == nil || th.Name != "Bleu et orange" || th.Source != "#ea580c" {
		t.Fatalf("thème reçu : %+v", th)
	}
}
