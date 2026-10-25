package canvas

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// pngOf fabrique une image PNG w × h d'une couleur.
func pngOf(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func imageCount(t *testing.T, d *fixture) int {
	t.Helper()
	var n int
	if err := d.read(context.Background(), func(tx *tx) error {
		return tx.row(`SELECT count(*) FROM _lattis_images`).Scan(&n)
	}); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestObjectConfigCheck(t *testing.T) {
	for _, tc := range []struct {
		cfg ObjectConfig
		bad string
	}{
		{ObjectConfig{Kind: ObjRect, Width: 100, Height: 50}, ""},
		{ObjectConfig{Kind: "star", Width: 100, Height: 50}, "inconnu"},
		{ObjectConfig{Kind: ObjImage, Width: 100, Height: 50}, "aucune image"},
		{ObjectConfig{Kind: ObjRect, Width: 2, Height: 50}, "taille invalide"},
		{ObjectConfig{Kind: ObjLine, Width: 100, Height: 0}, ""}, // ligne horizontale : boîte plate
		{ObjectConfig{Kind: ObjLine, Width: 1, Height: 1}, "taille invalide"},
		{ObjectConfig{Kind: ObjRect, Width: 10, Height: 10, Fill: "red"}, "couleur"},
		{ObjectConfig{Kind: ObjRect, Width: 10, Height: 10, Transparency: 120}, "transparence"},
		{ObjectConfig{Kind: ObjText, Width: 10, Height: 10, Size: 300}, "taille du texte"},
		{ObjectConfig{Kind: ObjText, Width: 10, Height: 10, Align: "justify"}, "alignement"},
	} {
		cfg := tc.cfg
		err := cfg.check()
		switch {
		case tc.bad == "" && err != nil:
			t.Errorf("%+v : %v", tc.cfg, err)
		case tc.bad != "" && (err == nil || !strings.Contains(err.Error(), tc.bad)):
			t.Errorf("%+v : erreur %v, attendu « %s »", tc.cfg, err, tc.bad)
		}
	}
}

func TestCheckImage(t *testing.T) {
	info, err := CheckImage(pngOf(t, 30, 20, color.White))
	if err != nil || info.Type != "image/png" || info.Width != 30 || info.Height != 20 || len(info.Hash) != 64 {
		t.Fatalf("PNG : %+v %v", info, err)
	}
	// SVG : jamais gardé tel quel (l'interface le convertit en PNG).
	if _, err := CheckImage([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)); err == nil {
		t.Fatal("SVG accepté")
	}
	if _, err := CheckImage([]byte("pas une image")); err == nil {
		t.Fatal("texte accepté")
	}
	if _, err := CheckImage(make([]byte, MaxImageBytes+1)); err == nil || !strings.Contains(err.Error(), "10 Mo") {
		t.Fatalf("image trop lourde : %v", err)
	}
	// WebP sans perte (VP8L) 300 × 200 : en-tête seul.
	webp := []byte("RIFF\x00\x00\x00\x00WEBPVP8L\x00\x00\x00\x00\x2f")
	v := uint32(300-1) | uint32(200-1)<<14
	webp = append(webp, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
	webp = append(webp, make([]byte, 16)...)
	if w, h, ok := webpSize(webp); !ok || w != 300 || h != 200 {
		t.Fatalf("WebP : %d × %d %v", w, h, ok)
	}
}

func TestFitImage(t *testing.T) {
	if w, h := FitImage(1000, 500, 480); w != 480 || h != 240 {
		t.Fatalf("réduite : %d × %d", w, h)
	}
	if w, h := FitImage(100, 50, 480); w != 100 || h != 50 {
		t.Fatalf("gardée : %d × %d", w, h)
	}
}

// Une image posée deux fois n'est enregistrée qu'une fois ; chaque pose est
// une étape d'annulation, l'image reste pour le rétablissement.
func TestImageDedupUndo(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	data := pngOf(t, 1200, 600, color.RGBA{200, 30, 30, 255})
	before := dump(t, d)
	id, err := d.CreateImage(ctx, 1, 2, 3, data, ObjectConfig{Alt: "logo.png"})
	if err != nil {
		t.Fatal(err)
	}
	o, err := d.Object(ctx, id)
	if err != nil || o.Row != 2 || o.Col != 3 || o.Config.Width != DefaultImageWidth || o.Config.Height != DefaultImageWidth/2 || o.Config.Alt != "logo.png" {
		t.Fatalf("image posée : %+v %v", o, err)
	}
	mid := dump(t, d)
	id2, err := d.CreateImage(ctx, 1, 10, 0, data, ObjectConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if n := imageCount(t, d); n != 1 {
		t.Fatalf("%d images enregistrées, attendu 1 (même contenu)", n)
	}
	second := dump(t, d)
	// Copier-coller : un objet qui cite l'image déjà enregistrée.
	o2, _ := d.Object(ctx, id2)
	if _, err := d.CreateObject(ctx, 1, 20, 0, o2.Config); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateObject(ctx, 1, 20, 0, ObjectConfig{Kind: ObjImage, Image: strings.Repeat("0", 64), Width: 10, Height: 10}); err == nil {
		t.Fatal("image inconnue acceptée")
	}
	undoOnce(t, d, second, "copie") // l'objet refusé n'a rien laissé à annuler
	undoOnce(t, d, mid, "deuxième image")
	undoOnce(t, d, before, "première image")
	if _, err := d.bus.Redo(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _, err := d.ImageData(ctx, ImageHash(data)); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("image relue : %v", err)
	}
	if objs, _ := d.Objects(ctx, 1); len(objs) != 1 || objs[0].ID != id {
		t.Fatalf("rétablie : %+v", objs)
	}
}

// Les objets suivent les lignes et colonnes insérées ou supprimées avant
// eux, comme les graphiques ; une suppression qui les contient les ramène
// au début de la plage.
func TestObjectsMoveWithCells(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	id, err := d.CreateObject(ctx, 1, 5, 4, ObjectConfig{Kind: ObjEllipse, Text: "Ici"})
	if err != nil {
		t.Fatal(err)
	}
	at := func() (int, int) {
		o, err := d.Object(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return o.Row, o.Col
	}
	before := dump(t, d)
	if err := d.InsertSheetRows(ctx, 1, 2, 3); err != nil {
		t.Fatal(err)
	}
	if r, c := at(); r != 8 || c != 4 {
		t.Fatalf("après 3 lignes insérées : %d, %d", r, c)
	}
	undoOnce(t, d, before, "insertion de lignes")
	if err := d.InsertSheetColumns(ctx, 1, 6, 2); err != nil { // après l'objet : il ne bouge pas
		t.Fatal(err)
	}
	if r, c := at(); r != 5 || c != 4 {
		t.Fatalf("colonnes insérées après : %d, %d", r, c)
	}
	if err := d.DeleteSheetColumns(ctx, 1, 0, 1); err != nil {
		t.Fatal(err)
	}
	if r, c := at(); r != 5 || c != 2 {
		t.Fatalf("après 2 colonnes supprimées : %d, %d", r, c)
	}
	if err := d.DeleteSheetRows(ctx, 1, 3, 9); err != nil {
		t.Fatal(err)
	}
	if r, _ := at(); r != 3 {
		t.Fatalf("ligne d'ancrage supprimée : %d, attendu 3", r)
	}
}

func TestObjectPlaceOrderStyle(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	a, _ := d.CreateObject(ctx, 1, 0, 0, ObjectConfig{Kind: ObjRect})
	b, _ := d.CreateObject(ctx, 1, 0, 0, ObjectConfig{Kind: ObjText, Text: "Note"})
	c, err := d.CreateObject(ctx, 1, 0, 0, ObjectConfig{Kind: ObjArrow})
	if err != nil {
		t.Fatal(err)
	}
	order := func() []int64 {
		objs, err := d.Objects(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		var ids []int64
		for _, o := range objs {
			ids = append(ids, o.ID)
		}
		return ids
	}
	if got := order(); got[0] != a || got[2] != c {
		t.Fatalf("ordre de pose : %v", got)
	}
	before := dump(t, d)
	if err := d.OrderObject(ctx, a, true); err != nil {
		t.Fatal(err)
	}
	if got := order(); got[2] != a {
		t.Fatalf("premier plan : %v", got)
	}
	undoOnce(t, d, before, "premier plan")
	if err := d.OrderObject(ctx, c, false); err != nil {
		t.Fatal(err)
	}
	if got := order(); got[0] != c {
		t.Fatalf("arrière-plan : %v", got)
	}
	before = dump(t, d)
	if err := d.PlaceObject(ctx, b, 3, 2, 5, 7, 200, 90); err != nil {
		t.Fatal(err)
	}
	o, _ := d.Object(ctx, b)
	if o.Row != 3 || o.Col != 2 || o.Config.OffX != 5 || o.Config.OffY != 7 || o.Config.Width != 200 || o.Config.Height != 90 || o.Config.Text != "Note" {
		t.Fatalf("placé : %+v", o)
	}
	undoOnce(t, d, before, "déplacement")
	// Présentation : la place, la taille et le rang sont gardés.
	cfg := o.Config
	cfg.Fill, cfg.Transparency, cfg.Bold, cfg.Text, cfg.Width = "#ffee00", 40, true, "Note 2", 999
	if err := d.SetObject(ctx, b, cfg); err != nil {
		t.Fatal(err)
	}
	o2, _ := d.Object(ctx, b)
	if o2.Config.Fill != "#ffee00" || o2.Config.Transparency != 40 || !o2.Config.Bold || o2.Config.Text != "Note 2" || o2.Config.Width != 160 || o2.Config.Z != o.Config.Z {
		t.Fatalf("présentation : %+v", o2.Config)
	}
	if err := d.PlaceLine(ctx, c, 0, 0, 0, 0, 160, 0, true, false); err != nil {
		t.Fatal(err)
	}
	if o, _ := d.Object(ctx, c); !o.Config.FlipH {
		t.Fatal("flèche non retournée")
	}
	before = dump(t, d)
	if err := d.DeleteObject(ctx, b); err != nil {
		t.Fatal(err)
	}
	undoOnce(t, d, before, "suppression")
}

// Copier une feuille copie ses objets ; la supprimer les emporte, Ctrl+Z
// les rend.
func TestObjectsSheetCopyDelete(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	data := pngOf(t, 40, 40, color.Black)
	if _, err := d.CreateImage(ctx, 1, 1, 1, data, ObjectConfig{}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateObject(ctx, 1, 4, 1, ObjectConfig{Kind: ObjTriangle}); err != nil {
		t.Fatal(err)
	}
	cp, err := d.CopySheet(ctx, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	objs, _ := d.Objects(ctx, cp)
	if len(objs) != 2 || objs[0].Config.Image != ImageHash(data) {
		t.Fatalf("copie : %+v", objs)
	}
	before := dump(t, d)
	if err := d.DeleteSheet(ctx, cp); err != nil {
		t.Fatal(err)
	}
	if objs, _ := d.Objects(ctx, cp); len(objs) != 0 {
		t.Fatal("objets restés après la suppression de la feuille")
	}
	undoOnce(t, d, before, "suppression de la feuille")
}

func TestObjectHTML(t *testing.T) {
	cfg := ObjectConfig{Kind: ObjRect, Width: 100, Height: 40, Text: "<b>Total</b>", Bold: true}
	cfg.Defaults()
	h := ObjectHTML(cfg, "")
	if !strings.Contains(h, "<rect") || !strings.Contains(h, "&lt;b&gt;Total&lt;/b&gt;") || !strings.Contains(h, "font-weight:bold") {
		t.Fatalf("forme : %s", h)
	}
	a := ObjectConfig{Kind: ObjArrow, Width: 100, Height: 0, FlipH: true}
	a.Defaults()
	if s := ShapeSVG(a); !strings.Contains(s, `x1="100"`) || !strings.Contains(s, "<polygon") {
		t.Fatalf("flèche : %s", s)
	}
	img := ObjectHTML(ObjectConfig{Kind: ObjImage, Width: 10, Height: 10, Alt: `"x"`}, "data:image/png;base64,AA")
	if !strings.Contains(img, `src="data:image/png;base64,AA"`) || !strings.Contains(img, `alt="&#34;x&#34;"`) {
		t.Fatalf("image : %s", img)
	}
}

func TestPrintObjects(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.CreateImage(ctx, 1, 0, 0, pngOf(t, 8, 8, color.White), ObjectConfig{}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateObject(ctx, 1, 2, 3, ObjectConfig{Kind: ObjText, Text: "Bonjour", OffX: 4}); err != nil {
		t.Fatal(err)
	}
	ps, err := d.PrintObjects(ctx, 1)
	if err != nil || len(ps) != 2 {
		t.Fatalf("%+v %v", ps, err)
	}
	if !strings.Contains(ps[0].HTML, "data:image/png;base64,") || ps[1].Row != 2 || ps[1].Col != 3 || !strings.Contains(ps[1].HTML, "Bonjour") {
		t.Fatalf("%+v", ps)
	}
}
