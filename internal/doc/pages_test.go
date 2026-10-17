package doc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lalamefine/lattis/internal/canvas"
)

// Un onglet page HTML et les hôtes HTTP autorisés s'enregistrent dans le
// fichier .lattis et reviennent à la réouverture.
func TestPageAndHostsSaved(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	d, err := New(ctx)
	must(t, err)
	id, err := d.AddPage(ctx, 1) // avant Feuille1
	must(t, err)
	must(t, d.SetPage(ctx, id, "<h1>Synthèse</h1>"))
	must(t, d.AllowHTTPHost(ctx, "API.example.com"))
	d.SetSessionHost("autre.example.com", true) // pour la session seulement
	path := filepath.Join(t.TempDir(), "pages.lattis")
	must(t, d.Save(ctx, path))
	must(t, d.Close())

	d, err = Open(ctx, path)
	must(t, err)
	defer d.Close()
	l, err := d.Layout(ctx, id)
	must(t, err)
	if len(l.Sheets) != 2 || l.Sheets[0].ID != id || l.Sheets[0].Kind != "html" || l.Sheets[0].Name != "Page1" || l.Sheets[1].Kind != "" {
		t.Fatalf("onglets %+v", l.Sheets)
	}
	src, err := d.Page(ctx, id)
	must(t, err)
	if src != "<h1>Synthèse</h1>" {
		t.Fatalf("page %q", src)
	}
	hosts, err := d.HTTPHosts(ctx)
	must(t, err)
	if len(hosts) != 1 || hosts[0] != "api.example.com" {
		t.Fatalf("hôtes %v", hosts)
	}
	if ok, _, _ := d.HTTPHostAllowed(ctx, "autre.example.com"); ok {
		t.Fatal("autorisation de session enregistrée dans le fichier")
	}
	// L'annulation (historique gardé dans le fichier) rend la page d'origine.
	if _, err := d.Bus.Undo(ctx); err != nil {
		t.Fatal(err)
	}
	if src, _ := d.Page(ctx, id); src == "<h1>Synthèse</h1>" {
		t.Fatal("annulation sans effet sur la page")
	}
}

// Un onglet Documentation est une page d'un autre genre : il garde son
// type à la réouverture, et ses graphiques se calculent sans feuille.
func TestDocSaved(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	d, err := New(ctx)
	must(t, err)
	id, err := d.AddDoc(ctx, 0)
	must(t, err)
	delta := `{"ops":[{"insert":"Notes\n"}]}`
	must(t, d.SetPage(ctx, id, delta))
	path := filepath.Join(t.TempDir(), "doc.lattis")
	must(t, d.Save(ctx, path))
	must(t, d.Close())

	d, err = Open(ctx, path)
	must(t, err)
	defer d.Close()
	l, err := d.Layout(ctx, id)
	must(t, err)
	if len(l.Sheets) != 2 || l.Sheets[1].Kind != "doc" || l.Sheets[1].Name != "Documentation1" {
		t.Fatalf("onglets %+v", l.Sheets)
	}
	if src, _ := d.Page(ctx, id); src != delta {
		t.Fatalf("contenu %q", src)
	}
	if c := d.ChartData(ctx, canvas.ChartConfig{Source: "Inconnu", Kind: "bar"}); c.Error == "" {
		t.Fatal("graphique d'un tableau inconnu sans erreur")
	}
}
