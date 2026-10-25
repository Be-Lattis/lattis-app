package transfer

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Les fils de commentaires deviennent des notes du fichier .xlsx (une par
// fil, « Auteur : texte »), sur la feuille et la cellule du fil.
func TestXLSXThreadNotes(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Pomme", "1,5"}, {"Poire", "2"}}); err != nil {
		t.Fatal(err)
	}
	alice, bob := canvas.Who{Name: "Alice", ID: "a"}, canvas.Who{Name: "Bob <b>", ID: "b"}
	id, err := d.AddComment(ctx, 1, 2, 1, "Prix à revoir", alice)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.AddComment(ctx, 1, 2, 1, "D'accord", bob); err != nil {
		t.Fatal(err)
	}
	if err := d.ResolveThread(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	if _, err := d.AddComment(ctx, 1, 5, 3, "Libre", bob); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "notes.xlsx")
	if _, err := ExportXLSXFile(ctx, d.Model, p); err != nil {
		t.Fatal(err)
	}
	parts := readZip(t, p)
	contains(t, "types", parts["[Content_Types].xml"], `PartName="/xl/comments1.xml"`, `Extension="vml"`)
	contains(t, "relations", parts["xl/worksheets/_rels/sheet1.xml.rels"], `Target="../comments1.xml"`, `Target="../drawings/vmlDrawing1.vml"`)
	contains(t, "feuille", parts["xl/worksheets/sheet1.xml"], `<legacyDrawing r:id="rVml1"/>`)
	contains(t, "notes", parts["xl/comments1.xml"], `<comment ref="B3"`, "Alice : Prix à revoir\nBob &lt;b&gt; : D'accord\n✓",`<comment ref="D6"`, "Bob &lt;b&gt; : Libre")
	// Le fichier se relit (les notes ne gênent pas l'import).
	if _, closeFn, err := ReadXLSX(ctx, p); err != nil {
		t.Fatal(err)
	} else if err := closeFn(); err != nil {
		t.Fatal(err)
	}
}
