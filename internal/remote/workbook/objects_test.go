package workbook_test

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// pngNoise : PNG de bruit (incompressible), environ 4 × w × h octets.
func pngNoise(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewPCG(1, 2))
	for i := range img.Pix {
		img.Pix[i] = byte(r.Uint32())
	}
	var b bytes.Buffer
	must(t, png.Encode(&b, img))
	return b.Bytes()
}

// Images, formes et zones de texte partent avec la mise en page ; une
// image de plus de 1 Mo reste sur le poste de l'auteur (cadre vide chez
// les autres) et, le classeur partagé, une telle image est refusée.
func TestObjectsPublished(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	fake := workbooktest.New()
	a := author(t)
	small, big := pngNoise(t, 40, 40), pngNoise(t, 600, 600)
	if len(big) <= canvas.MaxSharedImageBytes {
		t.Fatalf("image de test trop légère : %d octets", len(big))
	}
	_, err := a.CreateImage(ctx, 1, 12, 0, small, canvas.ObjectConfig{Alt: "logo"})
	must(t, err)
	_, err = a.CreateImage(ctx, 1, 12, 4, big, canvas.ObjectConfig{})
	must(t, err)
	_, err = a.CreateObject(ctx, 1, 20, 0, canvas.ObjectConfig{Kind: canvas.ObjRoundRect, Text: "À lire"})
	must(t, err)
	info := publish(t, a, fake.As("alice"), workbook.Frozen, "Ventes")
	lay := fake.Meta(info.WorkbookID, "layout")
	if !strings.Contains(string(lay), "_lattis_objects") || !strings.Contains(string(lay), canvas.ImageHash(small)) || strings.Contains(string(lay), canvas.ImageHash(big)+`",`) {
		t.Fatalf("mise en page publiée sans les objets, ou avec la grande image (%d octets)", len(lay))
	}
	if len(lay) > 1<<20 {
		t.Fatalf("mise en page de %d octets", len(lay))
	}
	must(t, fake.Grant(info.WorkbookID, "bob", "reader"))
	r := open(t, fake.As("bob"), info.WorkbookID, "bob")
	objs, err := r.Objects(ctx, 1)
	must(t, err)
	if len(objs) != 3 || objs[2].Config.Text != "À lire" {
		t.Fatalf("objets reçus : %+v", objs)
	}
	if data, _, err := r.ImageData(ctx, canvas.ImageHash(small)); err != nil || !bytes.Equal(data, small) {
		t.Fatalf("petite image reçue : %v", err)
	}
	if _, _, err := r.ImageData(ctx, canvas.ImageHash(big)); err == nil {
		t.Fatal("grande image publiée")
	}
	// Classeur partagé : une image de plus de 1 Mo est refusée.
	if _, err := a.CreateImage(ctx, 1, 30, 0, pngNoise(t, 601, 600), canvas.ObjectConfig{}); err == nil || !strings.Contains(err.Error(), "1 Mo") {
		t.Fatalf("grande image acceptée dans un classeur partagé : %v", err)
	}
}
