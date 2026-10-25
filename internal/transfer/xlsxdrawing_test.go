package transfer

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"path/filepath"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Images et formes partent dans le .xlsx (parties de dessin) et en
// reviennent à l'import : place, taille, couleurs, texte.
func TestXLSXDrawingRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewGray(image.Rect(0, 0, 30, 20))); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateImage(ctx, 1, 2, 1, b.Bytes(), canvas.ObjectConfig{Alt: "logo", OffX: 4, OffY: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateObject(ctx, 1, 6, 2, canvas.ObjectConfig{Kind: canvas.ObjRoundRect, Text: "Objectif\natteint", Bold: true, Fill: "#ff0000", Transparency: 40, Align: "center", VAlign: "middle"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateObject(ctx, 1, 9, 0, canvas.ObjectConfig{Kind: canvas.ObjArrow, Width: 100, Height: 30, FlipH: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateObject(ctx, 1, 12, 0, canvas.ObjectConfig{Kind: canvas.ObjText, Text: "Note"}); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "objets.xlsx")
	if _, err := ExportXLSXFile(ctx, d.Model, p); err != nil {
		t.Fatal(err)
	}
	z := readZip(t, p)
	contains(t, "feuille", z["xl/worksheets/sheet1.xml"], `<drawing r:id="rDrawing1"/>`)
	contains(t, "dessin", z["xl/drawings/drawing1.xml"], `r:embed="rId1"`, `prst="roundRect"`, `<a:alpha val="60000"/>`, `<a:tailEnd type="triangle"/>`, `txBox="1"`, `flipH="1"`)
	contains(t, "types", z["[Content_Types].xml"], `Extension="png"`, `/xl/drawings/drawing1.xml`)
	contains(t, "relations", z["xl/worksheets/_rels/sheet1.xml.rels"], `Id="rDrawing1"`, `Target="../drawings/drawing1.xml"`)

	r := open(t)
	res, err := ImportXLSX(ctx, r.Model, p)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range res.Warnings {
		t.Logf("avertissement : %s", w)
	}
	objs, err := r.Objects(ctx, res.Sheets[0])
	if err != nil || len(objs) != 4 {
		t.Fatalf("objets importés : %+v %v", objs, err)
	}
	img, rr, arrow, tb := objs[0], objs[1], objs[2].Config, objs[3].Config
	if img.Config.Kind != canvas.ObjImage || img.Row != 2 || img.Col != 1 || img.Config.OffX != 4 || img.Config.OffY != 3 || img.Config.Width != 30 || img.Config.Alt != "logo" {
		t.Fatalf("image : %+v", img)
	}
	if c := rr.Config; c.Kind != canvas.ObjRoundRect || c.Text != "Objectif\natteint" || !c.Bold || c.Fill != "#ff0000" || c.Transparency != 40 || c.Align != "center" || c.VAlign != "middle" || c.Width != 120 {
		t.Fatalf("forme : %+v", c)
	}
	if arrow.Kind != canvas.ObjArrow || !arrow.FlipH || arrow.FlipV || arrow.Width != 100 {
		t.Fatalf("flèche : %+v", arrow)
	}
	if tb.Kind != canvas.ObjText || tb.Text != "Note" {
		t.Fatalf("zone de texte : %+v", tb)
	}
}
