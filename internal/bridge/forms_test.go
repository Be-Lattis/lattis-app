package bridge

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
	"github.com/lalamefine/lattis/protocol"
)

// TestFormInfo : conditions du §2.2 de partager-comme-formulaire.md
// (classeur local, tableau d'une requête, tableau privé, lecteur, hors
// ligne, serveur sans formulaires), colonnes données au concepteur (clé
// numérotée, listes fixes et tirées d'un tableau publié), création, colonne
// « Reçu le » ajoutée par la structure, image.
func TestFormInfo(t *testing.T) {
	ctx := context.Background()
	fake := workbooktest.New()
	oldServer, oldDir, oldRetry, oldDebounce, oldDisc := shareServer, shareCacheDir, shareRetry, collab.Debounce, formsDiscovery
	cache := t.TempDir()
	shareCacheDir = func() (string, error) { return cache, nil }
	shareRetry, collab.Debounce = 20*time.Millisecond, 10*time.Millisecond
	disc := protocol.Discovery{Name: "Serveur des nids", Forms: true}
	formsDiscovery = func(ctx context.Context, server string) (protocol.Discovery, error) { return disc, nil }
	t.Cleanup(func() {
		shareServer, shareCacheDir, shareRetry, collab.Debounce, formsDiscovery = oldServer, oldDir, oldRetry, oldDebounce, oldDisc
	})
	user := "alice"
	shareServer = func(ctx context.Context, server string) (workbook.Server, auth.User, string, error) {
		return fake.As(user), auth.User{PGRole: user}, "Serveur de test", nil
	}

	// Classeur d'Alice : Espèces (liste), Nids (clé numérotée, listes),
	// Notes (laissé sur le poste).
	ad, err := doc.New(ctx)
	mustShare(t, err)
	t.Cleanup(func() { ad.Close() })
	_, err = ad.Paste(ctx, 1, 0, 0, [][]string{{"ID", "Commune", "Espèce", "Statut", "Œufs"}, {"1", "Brest", "Pie", "Vu", "3"}, {"2", "Quimper", "Merle", "Couvé", "4"}})
	mustShare(t, err)
	mustShare(t, ad.RenameTable(ctx, "Feuille1", "Nids"))
	_, err = ad.Paste(ctx, 1, 0, 8, [][]string{{"Nom", "Code"}, {"Pie", "P"}, {"Merle", "M"}, {"Geai", "G"}})
	mustShare(t, err)
	_, err = ad.Paste(ctx, 1, 0, 11, [][]string{{"Note", "N"}, {"a", "1"}})
	mustShare(t, err)
	names := map[int]string{}
	rs, err := ad.DB().Read().Query(`SELECT col, name FROM _lattis_tables`)
	mustShare(t, err)
	for rs.Next() {
		var c int
		var n string
		mustShare(t, rs.Scan(&c, &n))
		names[c] = n
	}
	rs.Close()
	mustShare(t, ad.RenameTable(ctx, names[8], "Espèces"))
	mustShare(t, ad.RenameTable(ctx, names[11], "Notes"))
	mustShare(t, ad.ApplyStyle(ctx, 1, 1, 2, 2, 2, canvas.Style{"dv": "=Espèces[Nom]"}))
	mustShare(t, ad.ApplyStyle(ctx, 1, 1, 3, 2, 3, canvas.Style{"dv": []any{"Vu", "Couvé", "Éclos"}}))

	// Classeur local : publier d'abord.
	l, _ := shareService(t)
	_, err = l.New(ctx)
	mustShare(t, err)
	_, err = l.Paste(ctx, 1, 0, 0, [][]string{{"A", "B"}, {"x", "1"}})
	mustShare(t, err)
	fi, err := l.FormInfo(ctx, "Feuille1")
	mustShare(t, err)
	if fi.Reason != FormLocal || !strings.Contains(fi.Message, "publiez d'abord") {
		t.Fatalf("classeur local : %+v", fi)
	}

	var nids, especes int64
	mustShare(t, ad.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE name = 'Nids'`).Scan(&nids))
	mustShare(t, ad.DB().Read().QueryRow(`SELECT id FROM _lattis_tables WHERE name = 'Espèces'`).Scan(&especes))
	info, err := workbook.Publish(ctx, fake.As("alice"), ad.DB(), workbook.Request{Server: "https://lattis.example.test", User: "alice", Title: "Nids", Mode: workbook.Collaborative, Tables: []int64{nids, especes}})
	mustShare(t, err)
	mustShare(t, fake.Grant(info.WorkbookID, "bob", "reader"))
	fake.As("bob")

	a, ha := shareService(t)
	_, err = a.ShareOpen(ctx, "https://lattis.example.test", info.WorkbookID, "")
	mustShare(t, err)
	waitShare(t, ha, "Alice en ligne", func(s workbook.State) bool { return s.Online && s.CanManage })

	// Tableau créé sur le poste après la publication : privé.
	_, err = a.Paste(ctx, 1, 20, 0, [][]string{{"Note", "N"}, {"a", "1"}})
	mustShare(t, err)
	rv, err := a.ShareRights(ctx, false)
	mustShare(t, err)
	if len(rv.LocalTables) != 1 {
		t.Fatalf("tableaux privés : %+v", rv.LocalTables)
	}
	if fi, err = a.FormInfo(ctx, rv.LocalTables[0]); err != nil || fi.Reason != FormPrivate {
		t.Fatalf("tableau privé : %+v %v", fi, err)
	}
	fi, err = a.FormInfo(ctx, "Nids")
	mustShare(t, err)
	if fi.Reason != "" || fi.TableID == "" || fi.ServerName != "Serveur des nids" || len(fi.Columns) != 5 {
		t.Fatalf("Nids : %+v", fi)
	}
	col := map[string]FormColumn{}
	for _, c := range fi.Columns {
		col[c.Name] = c
	}
	if c := col["ID"]; !c.Key || !c.Auto || c.ColumnID == "" {
		t.Fatalf("clé numérotée : %+v", c)
	}
	if l := col["Espèce"].List; l == nil || l.TableID == "" || l.ColumnID == "" || strings.Join(l.Values, ",") != "Pie,Merle,Geai" {
		t.Fatalf("liste tirée d'Espèces : %+v", l)
	}
	if l := col["Statut"].List; l == nil || !l.Fixed || strings.Join(l.Values, ",") != "Vu,Couvé,Éclos" {
		t.Fatalf("liste fixe : %+v", l)
	}
	if col["Commune"].AvgLength < 5 || col["Œufs"].Type != "integer" {
		t.Fatalf("colonnes : %+v", fi.Columns)
	}

	// Création : définition par column_id, valeur posée par le serveur.
	names2 := map[string]string{}
	for _, c := range fi.Columns {
		names2[c.ColumnID] = c.Name
	}
	def := workbook.FormDefinition{Format: 1, Lang: "fr", Appearance: workbook.FormAppearance{Title: "Relevé des nids", Accent: "green", SubmitLabel: "Envoyer", Thanks: "Merci !"},
		Fields: []workbook.FormField{
			{Column: col["Commune"].ColumnID, Label: "Commune", Required: true, Widget: "text"},
			{Column: col["Espèce"].ColumnID, Label: "Espèce", Widget: "radio", List: &workbook.FormList{TableID: col["Espèce"].List.TableID, Column: col["Espèce"].List.ColumnID}},
		},
		ServerValues: []workbook.FormServerValue{{Column: col["ID"].ColumnID, Value: workbook.FormValue{Token: workbook.TokenNextNumber}}}}
	bad := def
	bad.Fields = append([]workbook.FormField{}, def.Fields...)
	bad.Fields[0].List = &workbook.FormList{}
	if _, err := a.FormSave(ctx, FormSave{Server: fi.Server, WorkbookID: fi.WorkbookID, TableID: fi.TableID, Spec: workbook.FormSpec{Definition: bad, Access: workbook.FormMembers}, Names: names2}); err == nil || !strings.Contains(err.Error(), "Commune : liste vide") {
		t.Fatalf("refus avec le nom de la colonne : %v", err)
	}
	saved, err := a.FormSave(ctx, FormSave{Server: fi.Server, WorkbookID: fi.WorkbookID, TableID: fi.TableID, Spec: workbook.FormSpec{Definition: def, Access: workbook.FormAudience},
		Add: []DataAudienceEntry{{Email: "bob@example.test"}, {Email: "zoe@example.test"}}, Names: names2})
	mustShare(t, err)
	if saved.ID == "" || saved.URL == "" || saved.Version != 1 || len(saved.Refused) != 1 || !strings.Contains(saved.Refused[0], "zoe") {
		t.Fatalf("création : %+v", saved)
	}
	mustShare(t, a.FormSetState(ctx, fi.Server, saved.ID, workbook.FormOpen, names2))
	if u, err := a.FormPreviewURL(ctx, fi.Server, saved.ID); err != nil || !strings.Contains(u, "?p=") {
		t.Fatalf("aperçu : %q %v", u, err)
	}
	id, err := a.FormImage(ctx, fi.Server, saved.ID, "logo", "logo.png", base64.StdEncoding.EncodeToString([]byte("png")))
	if err != nil || id == "" || fake.LastFormImage != "logo:image/png" {
		t.Fatalf("logo : %q %v %q", id, err, fake.LastFormImage)
	}
	if _, err := a.FormImage(ctx, fi.Server, saved.ID, "logo", "logo.gif", ""); err == nil {
		t.Fatal("GIF accepté")
	}
	if fi, err = a.FormInfo(ctx, "Nids"); err != nil || len(fi.Forms) != 1 || fi.Forms[0].State != workbook.FormOpen {
		t.Fatalf("formulaires du tableau : %+v %v", fi.Forms, err)
	}

	// « Ajouter une colonne Reçu le » : opération de structure, envoyée.
	fi, err = a.FormAddColumn(ctx, "Nids", "Reçu le", "date")
	mustShare(t, err)
	if c := fi.Columns[len(fi.Columns)-1]; c.Name != "Reçu le" || c.ColumnID == "" || c.Type != "date" {
		t.Fatalf("colonne Reçu le : %+v", fi.Columns)
	}

	// Serveur sans formulaires : découverte, puis 42883.
	disc.Forms = false
	if fi, _ = a.FormInfo(ctx, "Nids"); fi.Reason != FormNoServer {
		t.Fatalf("découverte sans forms : %+v", fi)
	}
	disc.Forms = true
	fake.NoForms = true
	if fi, _ = a.FormInfo(ctx, "Nids"); fi.Reason != FormNoServer || fi.Message != "Ce serveur ne propose pas encore les formulaires." {
		t.Fatalf("serveur sans formulaires : %+v", fi)
	}
	fake.NoForms = false

	// Hors ligne.
	fake.SetDown(true)
	waitShare(t, ha, "Alice hors ligne", func(s workbook.State) bool { return !s.Online })
	if fi, _ = a.FormInfo(ctx, "Nids"); fi.Reason != FormOffline {
		t.Fatalf("hors ligne : %+v", fi)
	}
	fake.SetDown(false)
	a.ShareRetry()
	waitShare(t, ha, "Alice de retour", func(s workbook.State) bool { return s.Online })

	// Bob, lecteur : réservé, mais il voit le formulaire ouvert à lui.
	user = "bob"
	bobCache := t.TempDir()
	shareCacheDir = func() (string, error) { return bobCache, nil }
	b, hb := shareService(t)
	_, err = b.ShareOpen(ctx, "https://lattis.example.test", info.WorkbookID, "")
	mustShare(t, err)
	waitShare(t, hb, "Bob en ligne", func(s workbook.State) bool { return s.Online })
	fi, err = b.FormInfo(ctx, "Nids")
	mustShare(t, err)
	if fi.Reason != FormRights || len(fi.Forms) != 1 || fi.Forms[0].URL == "" || fi.Forms[0].Manageable || len(fi.Columns) != 0 {
		t.Fatalf("lecteur : %+v", fi)
	}
}
