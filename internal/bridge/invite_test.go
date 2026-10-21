package bridge

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
	"github.com/lalamefine/lattis/protocol"
)

// TestShareInvitePerson : « Inviter une personne » de Gérer l'accès par le
// pont (migration 0046) : recherche, compte connu (rôles donnés), adresse
// inconnue (lien à transmettre, ou courriel), invitations en attente avec
// leur auteur, réglage « mail » de la découverte ; serveur sans 0046 :
// lattis.invite.
func TestShareInvitePerson(t *testing.T) {
	ctx := context.Background()
	fake := workbooktest.New()
	oldServer, oldDir, oldRetry, oldDebounce, oldDisc := shareServer, shareCacheDir, shareRetry, collab.Debounce, serverDiscovery
	cache := t.TempDir()
	shareCacheDir = func() (string, error) { return cache, nil }
	shareRetry, collab.Debounce = 20*time.Millisecond, 10*time.Millisecond
	disc := protocol.Discovery{}
	serverDiscovery = func(ctx context.Context, server string) (protocol.Discovery, error) { return disc, nil }
	t.Cleanup(func() {
		shareServer, shareCacheDir, shareRetry, collab.Debounce, serverDiscovery = oldServer, oldDir, oldRetry, oldDebounce, oldDisc
	})
	shareServer = func(ctx context.Context, server string) (workbook.Server, auth.User, string, error) {
		return fake.As("alice"), auth.User{PGRole: "alice"}, "Serveur de test", nil
	}
	ad, err := doc.New(ctx)
	mustShare(t, err)
	t.Cleanup(func() { ad.Close() })
	_, err = ad.Paste(ctx, 1, 0, 0, [][]string{{"Produit", "Prix"}, {"Café", "4,5"}})
	mustShare(t, err)
	var tbl int64
	mustShare(t, ad.DB().Read().QueryRow(`SELECT id FROM _lattis_tables`).Scan(&tbl))
	info, err := workbook.Publish(ctx, fake.As("alice"), ad.DB(), workbook.Request{Server: "https://lattis.example.test", User: "alice", Title: "Prix", Mode: workbook.Collaborative, Tables: []int64{tbl}})
	mustShare(t, err)
	fake.As("bob")
	fake.SetName("bob", "Bob Martin")

	a, ha := shareService(t)
	_, err = a.ShareOpen(ctx, "https://lattis.example.test", info.WorkbookID, "")
	mustShare(t, err)
	waitShare(t, ha, "Alice en ligne", func(s workbook.State) bool { return s.Online && s.CanManage })

	// Recherche : rien sous 2 caractères.
	if p, err := a.ShareSearchPeople(ctx, "m"); err != nil || len(p) != 0 {
		t.Fatalf("recherche d'un caractère : %+v %v", p, err)
	}
	p, err := a.ShareSearchPeople(ctx, "mart")
	mustShare(t, err)
	if len(p) != 1 || p[0].Email != "bob@example.test" || p[0].DisplayName != "Bob Martin" {
		t.Fatalf("recherche : %+v", p)
	}

	// Compte connu : membre tout de suite.
	out, err := a.ShareInvitePerson(ctx, "bob@example.test", []string{"editor"})
	mustShare(t, err)
	if out.Status != workbook.InviteGranted || out.Link != "" || !slices.ContainsFunc(out.View.Members, func(m workbook.WorkbookMember) bool { return m.UserID == "bob" }) {
		t.Fatalf("Bob : %+v", out)
	}
	// Adresse inconnue, pas de courriel : lien à transmettre.
	out, err = a.ShareInvitePerson(ctx, "carol@example.test", []string{"reader"})
	mustShare(t, err)
	if out.Status != workbook.InviteInvited || out.Mailed || out.Link != "https://lattis.example.test/w/"+info.WorkbookID || out.View.Mail {
		t.Fatalf("Carol : %+v", out)
	}
	if inv := out.View.ACL.Invitations; len(inv) != 1 || inv[0].Email != "carol@example.test" || inv[0].InvitedBy != "alice@example.test" {
		t.Fatalf("invitations : %+v", inv)
	}
	// Serveur qui envoie les courriels (découverte « mail ») : pas de lien.
	disc.Mail, fake.Mailer = true, true
	out, err = a.ShareInvitePerson(ctx, "dan@example.test", []string{"participant"})
	mustShare(t, err)
	if out.Status != workbook.InviteInvited || !out.Mailed || out.Link != "" || !out.View.Mail {
		t.Fatalf("Dan : %+v", out)
	}
	v, err := a.ShareRevokeInvitation(ctx, out.View.ACL.Invitations[1].ID)
	mustShare(t, err)
	if len(v.ACL.Invitations) != 1 {
		t.Fatalf("invitations après retrait : %+v", v.ACL.Invitations)
	}

	// Serveur sans 0046 : recherche vide, invitation par lattis.invite.
	fake.Before46 = true
	if p, err := a.ShareSearchPeople(ctx, "bob"); err != nil || len(p) != 0 {
		t.Fatalf("recherche sans 0046 : %+v %v", p, err)
	}
	out, err = a.ShareInvitePerson(ctx, "erin@example.test", []string{"reader"})
	mustShare(t, err)
	if out.Status != "" || len(out.View.ACL.Invitations) != 2 || fake.Calls["invite"] != 1 {
		t.Fatalf("invitation sans 0046 : %+v", out)
	}
	if _, err := a.ShareInvitePerson(ctx, "erin", []string{"reader"}); err == nil {
		t.Fatal("adresse sans @ acceptée")
	}
}

func TestInviteLink(t *testing.T) {
	for _, c := range []struct{ server, path, want string }{
		{"https://lattis.example.test/", "/w/abc", "https://lattis.example.test/w/abc"},
		{"https://lattis.example.test", "w/abc", "https://lattis.example.test/w/abc"},
		{"https://lattis.example.test", "", "https://lattis.example.test/w/wb"},
		{"https://lattis.example.test", "https://autre.example.test/w/abc", "https://autre.example.test/w/abc"},
	} {
		if got := inviteLink(c.server, "wb", c.path); got != c.want {
			t.Errorf("inviteLink(%q, %q) = %q, attendu %q", c.server, c.path, got, c.want)
		}
	}
}
