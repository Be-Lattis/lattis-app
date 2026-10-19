package workbook_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// remoteColumns lit les colonnes du serveur d'un tableau publié (nom →
// column_id) et son identifiant.
func remoteColumns(t *testing.T, srv workbook.Server, info *workbook.Info, name string) (string, map[string]string) {
	t.Helper()
	ctx := context.Background()
	var id string
	cols := map[string]string{}
	must(t, srv.Read(ctx, func(r workbook.Reader) error {
		ts, err := r.Tables(ctx, info.Schema)
		for _, tb := range ts {
			if tb.Name == name {
				id = tb.ID
				for _, c := range tb.Columns {
					cols[c.Name] = c.ID
				}
			}
		}
		return err
	}))
	if id == "" {
		t.Fatalf("tableau %s absent du serveur", name)
	}
	return id, cols
}

func formDef(cols map[string]string) workbook.FormDefinition {
	return workbook.FormDefinition{Format: 1, Lang: "fr",
		Appearance: workbook.FormAppearance{Title: "Commandes", Accent: "teal", SubmitLabel: "Envoyer", Thanks: "**Merci** !"},
		Fields: []workbook.FormField{
			{Kind: workbook.FormFieldSection, Title: "Quoi ?"},
			{Column: cols["Produit"], Label: "Produit", Required: true, Widget: "list", List: &workbook.FormList{Values: []string{"Café", "Thé"}}, Param: "produit", Locked: true},
			{Column: cols["Quantité"], Label: "Quantité", Widget: "number"},
		},
		ServerValues: []workbook.FormServerValue{{Column: cols["Livré le"], Value: workbook.FormValue{Token: workbook.TokenReceivedAt}}},
	}
}

// TestForms : création (brouillon), refus (droits, classeur figé, tableau
// inconnu, définition invalide, lien public, serveur sans formulaires),
// version, états, liste vue par un membre, audience, aperçu, suppression.
func TestForms(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	fake := workbooktest.New()
	alice := fake.As("alice")
	d := author(t)
	info := publish(t, d, alice, workbook.Collaborative, "Ventes")
	tableID, cols := remoteColumns(t, alice, info, "Ventes")
	bob := fake.As("bob")
	must(t, fake.Grant(info.WorkbookID, "bob", "reader"))

	spec := workbook.FormSpec{Definition: formDef(cols), Access: workbook.FormMembers}
	if _, err := bob.CreateForm(ctx, info.WorkbookID, tableID, spec); workbook.FormError(err) == nil || !strings.Contains(workbook.FormError(err).Error(), "propriétaires et concepteurs") {
		t.Fatalf("lecteur : %v", err)
	}
	if _, err := alice.CreateForm(ctx, info.WorkbookID, "inconnu", spec); err == nil || !strings.HasPrefix(workbook.FormError(err).Error(), "Introuvable") {
		t.Fatalf("tableau inconnu : %v", err)
	}
	bad := spec
	bad.Definition = formDef(cols)
	bad.Definition.Appearance.Accent = "rose"
	bad.Definition.Fields = append(bad.Definition.Fields, workbook.FormField{Column: "absente", Label: "?"})
	_, err := alice.CreateForm(ctx, info.WorkbookID, tableID, bad)
	var inv *workbook.FormInvalid
	if !errors.As(workbook.FormError(err), &inv) || inv.Fields["absente"] != "colonne inconnue" || !strings.Contains(inv.Error(), "palette") {
		t.Fatalf("définition invalide : %v %+v", err, inv)
	}
	pub := spec
	pub.Access = workbook.FormPublic
	if _, err := alice.CreateForm(ctx, info.WorkbookID, tableID, pub); err == nil || !strings.Contains(workbook.FormError(err).Error(), "lien public") {
		t.Fatalf("lien public refusé : %v", err)
	}

	created, err := alice.CreateForm(ctx, info.WorkbookID, tableID, spec)
	must(t, err)
	if created.ID == "" || len(created.Code) != 10 || !strings.HasSuffix(created.URL, "/f/"+created.Code) {
		t.Fatalf("création : %+v", created)
	}
	forms, err := alice.ListForms(ctx, info.WorkbookID)
	must(t, err)
	if len(forms) != 1 || forms[0].State != workbook.FormDraft || !forms[0].Manageable || forms[0].Version != 1 || forms[0].Definition == nil || forms[0].Definition.Fields[1].Param != "produit" {
		t.Fatalf("liste : %+v", forms)
	}
	// Un membre ne voit pas un brouillon.
	if l, err := bob.ListForms(ctx, info.WorkbookID); err != nil || len(l) != 0 {
		t.Fatalf("liste du lecteur (brouillon) : %+v %v", l, err)
	}

	// Version : 40001 si elle a changé.
	title := formDef(cols)
	title.Appearance.Title = "Commandes 2026"
	if _, err := alice.UpdateForm(ctx, created.ID, workbook.FormChange{Definition: &title}, 7); !errors.Is(workbook.FormError(err), workbook.ErrFormModified) {
		t.Fatalf("version périmée : %v", err)
	}
	v, err := alice.UpdateForm(ctx, created.ID, workbook.FormChange{Definition: &title}, 1)
	must(t, err)
	if v != 2 {
		t.Fatalf("version : %d", v)
	}

	// États : un brouillon ne se met pas en pause ; ouvert, il apparaît au
	// lecteur (mode membres), sans les chiffres.
	if err := alice.SetFormState(ctx, created.ID, workbook.FormPaused); err == nil {
		t.Fatal("pause d'un brouillon acceptée")
	}
	must(t, alice.SetFormState(ctx, created.ID, workbook.FormOpen))
	l, err := bob.ListForms(ctx, info.WorkbookID)
	must(t, err)
	if len(l) != 1 || l[0].Manageable || l[0].Definition != nil || l[0].Title != "Commandes 2026" || l[0].URL == "" {
		t.Fatalf("liste du lecteur : %+v", l)
	}
	if err := bob.SetFormState(ctx, created.ID, workbook.FormClosed); !workbook.Privilege(err) {
		t.Fatalf("lecteur qui ferme : %v", err)
	}
	must(t, alice.SetFormState(ctx, created.ID, workbook.FormPaused))
	if err := alice.SetFormState(ctx, created.ID, "fini"); err == nil {
		t.Fatal("état inconnu accepté")
	}

	// Audience : personnes et équipes du serveur ; adresse sans compte refusée.
	audience := workbook.FormAudience
	_, err = alice.UpdateForm(ctx, created.ID, workbook.FormChange{Access: &audience}, 2)
	must(t, err)
	if err := alice.FormAudienceAddByEmail(ctx, created.ID, "zoe@example.test"); err == nil || !strings.HasPrefix(workbook.FormError(err).Error(), "Introuvable") {
		t.Fatalf("adresse sans compte : %v", err)
	}
	must(t, alice.FormAudienceAddByEmail(ctx, created.ID, "bob@example.test"))
	team := fake.CreateTeam("Nord")
	must(t, alice.FormAudienceAdd(ctx, created.ID, team))
	forms, err = alice.ListForms(ctx, info.WorkbookID)
	must(t, err)
	if len(forms[0].Audience) != 2 || forms[0].Audience[1].Kind != "team" || forms[0].Audience[1].Name != "Nord" || forms[0].Access != workbook.FormAudience {
		t.Fatalf("audience : %+v", forms[0])
	}
	must(t, alice.FormAudienceRemove(ctx, created.ID, team))

	u, err := alice.FormPreviewURL(ctx, created.ID)
	must(t, err)
	if !strings.Contains(u, "/f/"+created.Code+"?p=") {
		t.Fatalf("aperçu : %s", u)
	}

	must(t, alice.DropForm(ctx, created.ID))
	if forms, _ := alice.ListForms(ctx, info.WorkbookID); len(forms) != 0 {
		t.Fatalf("après suppression : %+v", forms)
	}
	if err := alice.DropForm(ctx, created.ID); err == nil {
		t.Fatal("suppression d'un formulaire supprimé acceptée")
	}

	// Serveur sans les formulaires (42883).
	fake.NoForms = true
	if _, err := alice.ListForms(ctx, info.WorkbookID); !errors.Is(workbook.FormError(err), workbook.ErrNoForms) {
		t.Fatalf("serveur sans formulaires : %v", err)
	}
	fake.NoForms = false

	// Classeur figé : 55000.
	f2 := workbooktest.New()
	d2 := author(t)
	info2 := publish(t, d2, f2.As("alice"), workbook.Frozen, "Ventes")
	id2, cols2 := remoteColumns(t, f2.As("alice"), info2, "Ventes")
	if _, err := f2.As("alice").CreateForm(ctx, info2.WorkbookID, id2, workbook.FormSpec{Definition: formDef(cols2), Access: workbook.FormMembers}); err == nil || !strings.Contains(workbook.FormError(err).Error(), "collaboratif") {
		t.Fatalf("classeur figé : %v", err)
	}
}

func TestFormImageClient(t *testing.T) {
	ctx := context.Background()
	var kind, ctype string
	var size int
	status, tries := http.StatusCreated, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tries++
		if r.Header.Get("Authorization") != "Bearer neuf" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/api/forms/f1/images" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		b, _ := io.ReadAll(r.Body)
		kind, ctype, size = r.URL.Query().Get("kind"), r.Header.Get("Content-Type"), len(b)
		w.WriteHeader(status)
		w.Write([]byte(`{"id":"img-1"}`))
	}))
	defer srv.Close()
	tok := "vieux"
	c := workbook.FormImageClient{Issuer: srv.URL + "/", Token: func(context.Context) (string, error) { return tok, nil }, Invalidate: func() { tok = "neuf" }}
	id, err := c.UploadFormImage(ctx, "f1", "logo", "image/png", []byte("png"))
	must(t, err)
	if id != "img-1" || kind != "logo" || ctype != "image/png" || size != 3 || tries != 2 {
		t.Fatalf("envoi : %s %s %s %d %d", id, kind, ctype, size, tries)
	}
	status = http.StatusRequestEntityTooLarge
	if _, err := c.UploadFormImage(ctx, "f1", "banner", "image/jpeg", []byte("x")); err == nil || !strings.Contains(err.Error(), "trop lourde") {
		t.Fatalf("413 : %v", err)
	}
	if _, err := c.UploadFormImage(ctx, "f2", "banner", "image/jpeg", []byte("x")); !errors.Is(err, workbook.ErrNoForms) {
		t.Fatalf("404 : %v", err)
	}
}
