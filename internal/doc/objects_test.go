package doc

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"path/filepath"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	must(t, png.Encode(&b, image.NewGray(image.Rect(0, 0, w, h))))
	return b.Bytes()
}

// Images et formes partent dans le fichier ; une image qu'aucun objet ne
// cite plus (supprimé, insertion annulée) n'y part pas, mais reste dans la
// base de travail pour Ctrl+Z.
func TestObjectsSaved(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "images.lattis")
	d, err := New(ctx)
	must(t, err)
	kept, gone := pngBytes(t, 20, 10), pngBytes(t, 30, 30)
	_, err = d.CreateImage(ctx, 1, 1, 1, kept, canvas.ObjectConfig{Alt: "a.png"})
	must(t, err)
	_, err = d.CreateObject(ctx, 1, 6, 1, canvas.ObjectConfig{Kind: canvas.ObjText, Text: "Légende", Italic: true})
	must(t, err)
	_, err = d.CreateImage(ctx, 1, 9, 1, gone, canvas.ObjectConfig{})
	must(t, err)
	_, err = d.Bus.Undo(ctx)
	must(t, err)
	if _, _, err := d.ImageData(ctx, canvas.ImageHash(gone)); err != nil {
		t.Fatalf("image annulée retirée de la base de travail : %v", err)
	}
	must(t, d.Save(ctx, path))
	if _, _, err := d.ImageData(ctx, canvas.ImageHash(gone)); err != nil {
		t.Fatalf("image annulée retirée de la base de travail à l'enregistrement : %v", err)
	}
	must(t, d.Close())

	d, err = Open(ctx, path)
	must(t, err)
	defer d.Close()
	objs, err := d.Objects(ctx, 1)
	must(t, err)
	if len(objs) != 2 || objs[0].Config.Kind != canvas.ObjImage || objs[1].Config.Text != "Légende" || !objs[1].Config.Italic {
		t.Fatalf("objets relus : %+v", objs)
	}
	data, typ, err := d.ImageData(ctx, objs[0].Config.Image)
	must(t, err)
	if typ != "image/png" || !bytes.Equal(data, kept) {
		t.Fatalf("image relue : %s, %d octets", typ, len(data))
	}
	if _, _, err := d.ImageData(ctx, canvas.ImageHash(gone)); err == nil {
		t.Fatal("image sans objet gardée dans le fichier")
	}
	checkOK(t, d)
}
