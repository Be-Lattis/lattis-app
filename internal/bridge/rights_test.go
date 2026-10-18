package bridge

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// TestShareRights : écran « Partager » par le pont (propriétaire), puis un
// participant : lignes des autres verrouillées dans la fenêtre de la grille,
// droits relus sur notification.
func TestShareRights(t *testing.T) {
	ctx := context.Background()
	fake := workbooktest.New()
	oldServer, oldDir, oldRetry, oldDebounce := shareServer, shareCacheDir, shareRetry, collab.Debounce
	cache := t.TempDir()
	shareCacheDir = func() (string, error) { return cache, nil }
	shareRetry, collab.Debounce = 20*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() {
		shareServer, shareCacheDir, shareRetry, collab.Debounce = oldServer, oldDir, oldRetry, oldDebounce
	})
	user := "alice"
	shareServer = func(ctx context.Context, server string) (workbook.Server, auth.User, string, error) {
		return fake.As(user), auth.User{PGRole: user}, "Serveur de test", nil
	}

	ad, err := doc.New(ctx)
	mustShare(t, err)
	t.Cleanup(func() { ad.Close() })
	_, err = ad.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Café", "4,5"}, {"Thé", "2"}})
	mustShare(t, err)
	var tbl int64
	mustShare(t, ad.DB().Read().QueryRow(`SELECT id FROM _lattis_tables`).Scan(&tbl))
	info, err := workbook.Publish(ctx, fake.As("alice"), ad.DB(), workbook.Request{Server: "https://lattis.example.test", User: "alice", Title: "Prix", Mode: workbook.Collaborative, Tables: []int64{tbl}})
	mustShare(t, err)
	fake.As("bob")

	// Alice (propriétaire) ouvre le classeur par le pont.
	a, ha := shareService(t)
	_, err = a.ShareOpen(ctx, "https://lattis.example.test", info.WorkbookID, "")
	mustShare(t, err)
	waitShare(t, ha, "Alice en ligne", func(s workbook.State) bool { return s.Online && s.CanManage })
	v, err := a.ShareRights(ctx, true)
	mustShare(t, err)
	if !v.CanManage || v.ACL == nil || len(v.Tables) != 1 || v.Tables[0].SheetName != "Feuille1" || len(v.Tables[0].Columns) != 2 {
		t.Fatalf("droits : %+v", v)
	}
	v, err = a.ShareInvite(ctx, "bob@example.test", []string{"participant"})
	mustShare(t, err)
	v, err = a.ShareCreateRole(ctx, "Nord")
	mustShare(t, err)
	var nord string
	for _, r := range v.ACL.Roles {
		if r.Name == "Nord" {
			nord = r.ID
		}
	}
	// Niveau d'une feuille : la feuille est connue du serveur (create_table
	// avec sheet_id), le niveau y est posé.
	if v.Tables[0].SheetKey != "1" {
		t.Fatalf("feuille du serveur : %+v", v.Tables[0])
	}
	v, err = a.ShareSetLevels(ctx, []workbook.Level{{RoleID: nord, NodeKind: workbook.NodeSheet, NodeID: "1", Level: "reader"}})
	mustShare(t, err)
	for _, r := range v.ACL.Roles {
		if r.ID == nord && !slices.ContainsFunc(r.Grants, func(g workbook.Grant) bool { return g.NodeKind == "sheet" && g.NodeID == "1" && g.Level == "reader" }) {
			t.Fatalf("niveaux de Nord : %+v", r.Grants)
		}
	}
	// Invitation à un rôle personnalisé (par son nom, sans casse) ; membres
	// lus par workbook_members.
	v, err = a.ShareInvite(ctx, "carol@example.test", []string{"nord"})
	mustShare(t, err)
	if !v.CustomInvite || len(v.ACL.Invitations) != 1 || !slices.Equal(v.ACL.Invitations[0].Roles, []string{"nord"}) {
		t.Fatalf("invitation à Nord : %+v %+v", v.CustomInvite, v.ACL.Invitations)
	}
	if len(v.Members) != 2 || v.Members[1].UserID != "bob" || !slices.Equal(v.Members[1].Roles, []string{"participant"}) {
		t.Fatalf("membres : %+v", v.Members)
	}
	if _, err := a.ShareSetLevels(ctx, []workbook.Level{{RoleID: nord, NodeKind: workbook.NodeTable, NodeID: v.Tables[0].RemoteID, Level: "reader", ReadFilter: "1; DROP TABLE x"}}); err == nil || strings.Contains(err.Error(), "SQLSTATE") {
		t.Fatalf("filtre refusé : %v", err)
	}
	v, err = a.ShareAddRestriction(ctx, workbook.RestrictionRule{TableID: v.Tables[0].RemoteID, Command: "delete", Predicate: `"Prix" < 10`, Label: "Pas les chers"})
	mustShare(t, err)
	if len(v.ACL.Restrictions) != 1 {
		t.Fatalf("restrictions : %+v", v.ACL.Restrictions)
	}
	p, err := a.SharePreview(ctx, "bob", tbl)
	mustShare(t, err)
	if !slices.Equal(p.Columns, []string{"Produit", "Prix", "Auteur"}) || len(p.Rows) != 2 || p.Rows[0][0] != "Café" || p.Rows[0][2] != "alice" {
		t.Fatalf("aperçu : %+v", p)
	}
	// Écarts : « Recompiler les droits » appelle compile_acl.
	fake.SetDrift(info.WorkbookID, []workbook.Drift{{TableID: v.Tables[0].RemoteID, Name: "Tableau1"}})
	v, err = a.ShareRecompile(ctx)
	mustShare(t, err)
	if fake.Calls["compile_acl"] != 1 || len(v.Drift) != 0 {
		t.Fatalf("recompilation : %d appel(s), écarts %+v", fake.Calls["compile_acl"], v.Drift)
	}

	// Bob (participant) : lignes d'Alice verrouillées, refus clair.
	user = "bob"
	bobCache := t.TempDir() // un autre poste
	shareCacheDir = func() (string, error) { return bobCache, nil }
	b, hb := shareService(t)
	_, err = b.ShareOpen(ctx, "https://lattis.example.test", info.WorkbookID, "")
	mustShare(t, err)
	waitShare(t, hb, "Bob en ligne", func(s workbook.State) bool { return s.Online })
	l, err := b.Layout(ctx, 1)
	mustShare(t, err)
	if len(l.Tables) != 1 || l.Tables[0].Access != "participant" {
		t.Fatalf("mise en page : %+v", l.Tables)
	}
	w, err := b.Window(ctx, 1, l.Tables[0].ID, 0, 10)
	mustShare(t, err)
	if !slices.Equal(w.Locked, []int{0, 1}) {
		t.Fatalf("lignes verrouillées : %v", w.Locked)
	}
	if _, err := b.SetCell(ctx, 1, 1, 1, "5"); err == nil || !strings.Contains(err.Error(), "participant") {
		t.Fatalf("participant : %v", err)
	}
	if _, err := b.ShareCreateRole(ctx, "Sud"); err == nil {
		t.Fatal("un participant gère les droits")
	}
	if v, err := b.ShareRights(ctx, true); err != nil || v.ACL != nil || v.Tables[0].Level != "participant" {
		t.Fatalf("droits de Bob : %+v %v", v, err)
	}

	// Alice fait de Bob un éditeur : notification *acl, plus de verrou.
	user = "alice"
	acl, err := fake.As("alice").ACL(ctx, info.WorkbookID)
	mustShare(t, err)
	for _, r := range acl.Roles {
		if r.Builtin == "editor" {
			mustShare(t, fake.As("alice").GrantRole(ctx, r.ID, "bob"))
		}
	}
	waitFor(t, "droits de Bob relus", func() bool {
		l, err := b.Layout(ctx, 1)
		return err == nil && len(l.Tables) == 1 && l.Tables[0].Access == "editor"
	})
	w, err = b.Window(ctx, 1, l.Tables[0].ID, 0, 10)
	mustShare(t, err)
	if len(w.Locked) != 0 {
		t.Fatalf("lignes verrouillées pour un éditeur : %v", w.Locked)
	}
}

// TestSheetLevels : niveau de feuille posé sur la feuille quand le serveur
// la connaît pour tous ses tableaux, sinon sur chacun d'eux (tableaux
// publiés avant la migration 0009).
func TestSheetLevels(t *testing.T) {
	sheet := []workbook.Level{{RoleID: "r", NodeKind: workbook.NodeSheet, NodeID: "2", Level: "editor"}}
	tables := []RightsTable{{Sheet: 2, RemoteID: "a", SheetKey: "2"}, {Sheet: 2, RemoteID: "b", SheetKey: "2"}, {Sheet: 3, RemoteID: "c", SheetKey: "3"}}
	out, err := sheetLevels(sheet, tables)
	if err != nil || len(out) != 1 || out[0].NodeKind != "sheet" || out[0].NodeID != "2" {
		t.Fatalf("feuille connue : %+v %v", out, err)
	}
	tables[1].SheetKey = ""
	out, err = sheetLevels(sheet, tables)
	if err != nil || len(out) != 2 || out[0].NodeKind != "table" || out[0].NodeID != "a" || out[1].NodeID != "b" || out[1].Level != "editor" {
		t.Fatalf("feuille inconnue : %+v %v", out, err)
	}
	if _, err := sheetLevels([]workbook.Level{{RoleID: "r", NodeKind: workbook.NodeSheet, NodeID: "9", Level: "editor"}}, tables); err == nil {
		t.Fatal("feuille sans tableau publié")
	}
}
