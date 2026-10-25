package canvas

import (
	"context"
	"testing"
)

func TestNormalizeLink(t *testing.T) {
	ok := map[string]string{
		"https://lattis.be/a b":  "https://lattis.be/a%20b",
		"HTTP://exemple.fr":      "HTTP://exemple.fr",
		"www.exemple.fr":         "http://www.exemple.fr",
		"exemple.fr/page?x=1":    "http://exemple.fr/page?x=1",
		"jean@exemple.fr":        "mailto:jean@exemple.fr",
		"mailto:jean@exemple.fr": "mailto:jean@exemple.fr",
		"#B3":                    "#B3",
		"#$b$3:c4":               "#B3:C4",
		"#Feuille2!A1":           "#Feuille2!A1",
		"#'Ma feuille'!A1":       "#'Ma feuille'!A1",
		"#'L''été'!A1":           "#'L''été'!A1",
		"#Ventes":                "#Ventes",
		"  ":                     "",
	}
	for in, want := range ok {
		got, err := NormalizeLink(in)
		if err != nil || got != want {
			t.Errorf("NormalizeLink(%q) = %q, %v ; attendu %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"javascript:alert(1)", "JavaScript:alert(1)", "file:///C:/x.txt", "data:text/html,x", "vbscript:x",
		"ftp://exemple.fr", "http:x", "mailto:", "#", "#Feuille2!", "#A1 B2", "bonjour", "a\x00b"} {
		if got, err := NormalizeLink(bad); err == nil {
			t.Errorf("NormalizeLink(%q) = %q : attendu un refus", bad, got)
		}
	}
	if SafeLink("javascript:alert(1)") != "" || SafeLink("https://a.b") != "https://a.b" {
		t.Error("SafeLink")
	}
	// Renommages : feuille (citée entre apostrophes au besoin), tableau, nom.
	for _, c := range []struct{ in, want string }{
		{"#Feuille2!B3", "#'Mes chiffres'!B3"},
		{"#'feuille2'!B3", "#'Mes chiffres'!B3"},
		{"#Autre!B3", "#Autre!B3"},
		{"#Ventes", "#Achats"},
		{"#Objectif", "#Cible"},
		{"https://a.b", "https://a.b"},
	} {
		got, _ := renameLink(c.in, refChange{}.Renames)
		if c.in != "https://a.b" {
			rc := refChange{}
			rc.Sheets = lowerMap("Feuille2", "Mes chiffres")
			rc.Tables = lowerMap("Ventes", "Achats")
			rc.Names = lowerMap("Objectif", "Cible")
			got, _ = renameLink(c.in, rc.Renames)
		}
		if got != c.want {
			t.Errorf("renameLink(%q) = %q ; attendu %q", c.in, got, c.want)
		}
	}
}

func TestSetLink(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 0, 0, [][]string{{"Ville", "Pop"}, {"Lyon", "520"}, {"Paris", "2100"}, {"Nice", "340"}}); err != nil {
		t.Fatal(err)
	}
	before := dump(t, d)
	free := func(r, c int) string {
		var s string
		d.db.Read().QueryRow(`SELECT style FROM _lattis_cell_styles WHERE sheet = 1 AND row = ? AND col = ?`, r, c).Scan(&s)
		return s
	}
	// Cellule libre : texte affiché et lien, une seule étape.
	if err := d.SetLink(ctx, 1, 6, 4, "www.lattis.be", "Site"); err != nil {
		t.Fatal(err)
	}
	if got := free(6, 4); got != `{"ln":"http://www.lattis.be"}` {
		t.Fatalf("lien libre : %q", got)
	}
	l, _ := d.Layout(ctx, 1)
	if len(l.Cells) != 1 || l.Cells[0].Raw != "Site" {
		t.Fatalf("texte affiché : %+v", l.Cells)
	}
	// Schéma refusé : rien ne change.
	if err := d.SetLink(ctx, 1, 6, 4, "javascript:alert(1)", ""); err == nil {
		t.Fatal("javascript: accepté")
	}
	if err := d.ApplyStyle(ctx, 1, 6, 4, 6, 4, Style{"ln": "file:///etc/passwd"}); err == nil {
		t.Fatal("file: accepté par la mise en forme")
	}
	// Le lien suit l'insertion de lignes.
	if err := d.InsertSheetRows(ctx, 1, 5, 1); err != nil {
		t.Fatal(err)
	}
	if got := free(7, 4); got != `{"ln":"http://www.lattis.be"}` {
		t.Fatalf("lien après insertion : %q", got)
	}
	// Cellule de tableau (Paris, Pop) : le lien suit sa ligne au tri.
	if err := d.SetLink(ctx, 1, 2, 1, "#Feuille1!E8", ""); err != nil {
		t.Fatal(err)
	}
	if err := d.SortTable(ctx, "Feuille1", 1, true); err != nil {
		t.Fatal(err)
	}
	var paris int64
	var s string
	d.db.Read().QueryRow(`SELECT id FROM t_1 WHERE c_1 = 'Paris'`).Scan(&paris)
	d.db.Read().QueryRow(`SELECT style FROM _lattis_styles WHERE rid = ? AND col = 2`, paris).Scan(&s)
	if s != `{"ln":"#Feuille1!E8"}` {
		t.Fatalf("lien du tableau : %q", s)
	}
	// Renommer la feuille et le tableau : les adresses internes suivent.
	if err := d.SetLink(ctx, 1, 9, 4, "#Feuille1", "Aller"); err != nil {
		t.Fatal(err)
	}
	if err := d.RenameSheet(ctx, 1, "Mes chiffres"); err != nil {
		t.Fatal(err)
	}
	if err := d.RenameTable(ctx, "Feuille1", "Villes"); err != nil {
		t.Fatal(err)
	}
	d.db.Read().QueryRow(`SELECT style FROM _lattis_styles WHERE rid = ? AND col = 2`, paris).Scan(&s)
	if s != `{"ln":"#'Mes chiffres'!E8"}` {
		t.Fatalf("lien après renommage de la feuille : %q", s)
	}
	if got := free(9, 4); got != `{"ln":"#Villes"}` {
		t.Fatalf("lien après renommage du tableau : %q", got)
	}
	// Retirer le lien.
	if err := d.SetLink(ctx, 1, 9, 4, "", ""); err != nil {
		t.Fatal(err)
	}
	if got := free(9, 4); got != "" {
		t.Fatalf("lien retiré : %q", got)
	}
	check(t, d, "liens")
	for range 8 {
		d.bus.Undo(ctx)
	}
	if got := dump(t, d); got != before {
		t.Fatalf("annulation incomplète :\n%s", diff(before, got))
	}
}

func TestLinkPlace(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.Paste(ctx, 1, 2, 1, [][]string{{"Ville", "Pop"}, {"Lyon", "520"}}); err != nil {
		t.Fatal(err)
	}
	id, err := d.AddSheet(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.RenameSheet(ctx, id, "Ma feuille"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, id, 4, 4, "100"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetName(ctx, id, 4, 4, "Objectif"); err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string]LinkPlace{
		"#C4":              {1, 3, 2, 3, 2},
		"#b2:a1":           {1, 0, 0, 1, 1},
		"#'ma feuille'!E5": {id, 4, 4, 4, 4},
		"#Feuille1":        {1, 2, 1, 3, 2},
		"#objectif":        {id, 4, 4, 4, 4},
	} {
		got, err := d.LinkPlace(ctx, 1, in)
		if err != nil || got != want {
			t.Errorf("LinkPlace(%q) = %+v, %v ; attendu %+v", in, got, err, want)
		}
	}
	for _, bad := range []string{"#Inconnue!A1", "#Personne", "https://a.b"} {
		if _, err := d.LinkPlace(ctx, 1, bad); err == nil {
			t.Errorf("LinkPlace(%q) : attendu une erreur", bad)
		}
	}
}

func TestHyperlinkFormula(t *testing.T) {
	ctx := context.Background()
	d := open(t)
	if _, err := d.SetCell(ctx, 1, 0, 0, "lattis.be"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 0, 2, `=LIEN_HYPERTEXTE("https://"&Cell[A1]; "Site")`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 1, 2, `=HYPERLINK("javascript:alert(1)")`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetCell(ctx, 1, 2, 2, `=NBCAR(LIEN_HYPERTEXTE("https://a.b"; "abc"))`); err != nil {
		t.Fatal(err)
	}
	l, err := d.Layout(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int]CellInfo{}
	for _, c := range l.Cells {
		if c.Col == 2 {
			got[c.Row] = c
		}
	}
	if c := got[0]; c.Value != "Site" || c.Link != "https://lattis.be" {
		t.Fatalf("LIEN_HYPERTEXTE : %+v", c)
	}
	if c := got[1]; c.Value != "javascript:alert(1)" || c.Link != "" {
		t.Fatalf("schéma refusé : %+v", c)
	}
	if c := got[2]; c.Value != "3" || c.Link != "" {
		t.Fatalf("dans une autre fonction : %+v", c)
	}
}
