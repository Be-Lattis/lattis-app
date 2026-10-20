package transfer

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Les couleurs prises dans le thème (« theme:accent1:+40 ») s'exportent
// résolues avec le thème du classeur, comme à l'écran.
func TestExportThemeColors(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := d.SetCell(ctx, 1, 0, 0, "Thème")
	must(err)
	must(d.ApplyStyle(ctx, 1, 0, 0, 0, 0, canvas.Style{"fg": "theme:accent1", "bg": "theme:accent3:+40"}))
	export := func() string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "theme.xlsx")
		_, err := ExportXLSXFile(ctx, d.Model, p)
		must(err)
		return readZip(t, p)["xl/styles.xml"]
	}
	up := func(hex string) string { return "FF" + strings.ToUpper(strings.TrimPrefix(hex, "#")) }
	// Sans thème : « Vert et bleu ».
	contains(t, "styles sans thème", export(), `<color rgb="FF2F7D55"/>`, `<fgColor rgb="`+up(canvas.TintShade("#e07b24", 40))+`"/>`)
	// Thème « Bleu et orange » : les mêmes cellules prennent ses couleurs.
	must(d.SetTheme(ctx, &canvas.Theme{ID: "bleu-orange", Name: "Bleu et orange", Table: "#2563eb", Source: "#ea580c", Accents: []string{"#16a34a", "#9333ea", "#ca8a04", "#db2777"}}))
	st := export()
	contains(t, "styles du thème", st, `<color rgb="FF2563EB"/>`, `<fgColor rgb="`+up(canvas.TintShade("#16a34a", 40))+`"/>`)
	if strings.Contains(st, "theme:") {
		t.Fatal("référence au thème écrite telle quelle")
	}
}
