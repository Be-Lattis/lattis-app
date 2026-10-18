package collab_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/remote/collab"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Droits dans l'application (§8.1) : capacités relues, participant,
// lecture filtrée, restrictions, notification *acl.

// roleID cherche un rôle du classeur par son rôle fourni ou son nom.
func roleID(t *testing.T, srv workbook.Server, id, key string) string {
	t.Helper()
	acl, err := srv.ACL(context.Background(), id)
	must(t, err)
	for _, r := range acl.Roles {
		if r.Builtin == key || r.Name == key {
			return r.ID
		}
	}
	t.Fatalf("rôle %q introuvable", key)
	return ""
}

// aclTableID donne l'identifiant du serveur du tableau Ventes.
func aclTableID(t *testing.T, srv workbook.Server, id string) string {
	t.Helper()
	acl, err := srv.ACL(context.Background(), id)
	must(t, err)
	for _, tb := range acl.Tables {
		if tb.Name == "Ventes" {
			return tb.TableID
		}
	}
	t.Fatal("Ventes introuvable")
	return ""
}

func (c *client) access() (string, bool) {
	return c.d.Share.TableAccess(tableID(c.t, c.d))
}

func refresh(t *testing.T, c *client) bool {
	t.Helper()
	ch, err := c.e.RefreshRights(context.Background())
	must(t, err)
	return ch
}

func TestRightsParticipant(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	as := f.As("alice")
	// Bob passe d'éditeur à participant.
	must(t, as.RevokeRole(ctx, roleID(t, as, id, "editor"), "bob"))
	must(t, as.GrantRole(ctx, roleID(t, as, id, "participant"), "bob"))
	if !refresh(t, bob) {
		t.Fatal("droits de Bob inchangés")
	}
	if refresh(t, bob) {
		t.Fatal("seconde relecture : rien ne devrait changer")
	}
	if lv, _ := bob.access(); lv != workbook.LevelParticipant {
		t.Fatalf("niveau : %q", lv)
	}
	l, err := bob.d.Layout(ctx, 1)
	must(t, err)
	if len(l.Tables) != 1 || l.Tables[0].Access != "participant" || l.Tables[0].ReadOnly != "" {
		t.Fatalf("mise en page : %+v", l.Tables)
	}
	// Lignes d'Alice : en lecture seule (refus local, rien ne part).
	tbl := tableID(t, bob.d)
	locked, err := bob.d.Share.LockedRows(ctx, tbl, []int64{1, 2, 3})
	must(t, err)
	if len(locked) != 3 {
		t.Fatalf("lignes verrouillées : %v", locked)
	}
	if err := bob.set(0, 1, "99"); !errors.Is(err, workbook.ErrNotYours) && (err == nil || !strings.Contains(err.Error(), "participant")) {
		t.Fatalf("modification d'une ligne d'Alice : %v", err)
	}
	if n := bob.states()["pending"]; n != 0 {
		t.Fatalf("file : %v", bob.states())
	}
	eq(t, "Bob", bob.col("Quantité"), []string{"3", "10", "1"})
	// Ajout d'une ligne : permis, puis modifiable par Bob.
	bob.mustSet(3, 0, "Tisane")
	r := bob.send()
	if r.Rejected != 0 {
		t.Fatalf("ajout refusé : %+v", r)
	}
	bob.mustSet(3, 1, "7")
	if r := bob.send(); r.Rejected != 0 {
		t.Fatalf("modification de sa ligne refusée : %+v", r)
	}
	bob.receive()
	locked, err = bob.d.Share.LockedRows(ctx, tbl, []int64{1, 2, 3, 4})
	must(t, err)
	if len(locked) != 3 || locked[4] {
		t.Fatalf("lignes verrouillées après ajout : %v", locked)
	}
	// Alice (propriétaire, éditrice) n'a aucune ligne verrouillée.
	if locked, _ := alice.d.Share.LockedRows(ctx, tableID(t, alice.d), []int64{1, 2, 3}); len(locked) != 0 {
		t.Fatalf("Alice : %v", locked)
	}
}

func TestRightsReaderAndRevoked(t *testing.T) {
	ctx := context.Background()
	f, _, bob, id := setup(t)
	as := f.As("alice")
	must(t, as.RevokeRole(ctx, roleID(t, as, id, "editor"), "bob"))
	must(t, as.GrantRole(ctx, roleID(t, as, id, "reader"), "bob"))
	refresh(t, bob)
	if err := bob.set(0, 1, "99"); err == nil || !strings.Contains(err.Error(), "lecture seule") {
		t.Fatalf("lecteur : %v", err)
	}
	// Plus aucun rôle : tous les tableaux publiés passent à « aucun ».
	must(t, as.RevokeRole(ctx, roleID(t, as, id, "reader"), "bob"))
	if !refresh(t, bob) {
		t.Fatal("révocation non vue")
	}
	if lv, _ := bob.access(); lv != workbook.LevelNone {
		t.Fatalf("niveau : %q", lv)
	}
	if err := bob.set(0, 1, "99"); !errors.Is(err, workbook.ErrNoAccess) && (err == nil || !strings.Contains(err.Error(), "inaccessible")) {
		t.Fatalf("sans accès : %v", err)
	}
}

func TestRightsCustomRoleFilterRestriction(t *testing.T) {
	ctx := context.Background()
	f, alice, bob, id := setup(t)
	as := f.As("alice")
	rt := aclTableID(t, as, id)
	nord, err := as.CreateRole(ctx, id, "Nord")
	must(t, err)
	if _, err := as.CreateRole(ctx, id, "nord"); err == nil {
		t.Fatal("rôle en double accepté")
	}
	// Filtre refusé (sortie de l'expression).
	if err := as.SetLevels(ctx, []workbook.Level{{RoleID: nord, NodeKind: workbook.NodeTable, NodeID: rt, Level: "reader", ReadFilter: `true; DROP TABLE x`}}); err == nil {
		t.Fatal("filtre dangereux accepté")
	}
	if err := as.SetLevels(ctx, []workbook.Level{{RoleID: nord, NodeKind: workbook.NodeWorkbook, Level: "reader", ReadFilter: `"Produit" = 'Café'`}}); err == nil {
		t.Fatal("filtre sur le classeur accepté")
	}
	must(t, as.SetLevels(ctx, []workbook.Level{{RoleID: nord, NodeKind: workbook.NodeTable, NodeID: rt, Level: "editor", ReadFilter: `"Produit" = 'Café'`}}))
	must(t, as.RevokeRole(ctx, roleID(t, as, id, "editor"), "bob"))
	must(t, as.GrantRole(ctx, nord, "bob"))
	refresh(t, bob)
	lv, filt := bob.access()
	if lv != workbook.LevelEditor || !filt {
		t.Fatalf("Nord : %q filtré %v", lv, filt)
	}
	l, err := bob.d.Layout(ctx, 1)
	must(t, err)
	if !l.Tables[0].Filtered {
		t.Fatalf("indicateur : %+v", l.Tables[0])
	}
	acl, err := as.ACL(ctx, id)
	must(t, err)
	i := slices.IndexFunc(acl.Roles, func(r workbook.Role) bool { return r.ID == nord })
	if i < 0 || len(acl.Roles[i].Members) != 1 || acl.Roles[i].Members[0].UserID != "bob" {
		t.Fatalf("membres de Nord : %+v", acl.Roles)
	}
	// Restriction de lecture pour tous : vue filtrée aussi pour un éditeur
	// sans filtre.
	must(t, as.SetLevels(ctx, []workbook.Level{{RoleID: nord, NodeKind: workbook.NodeTable, NodeID: rt, Level: "editor"}}))
	refresh(t, bob)
	if _, filt := bob.access(); filt {
		t.Fatal("filtre retiré : vue encore filtrée")
	}
	rid, err := as.AddRestriction(ctx, id, workbook.RestrictionRule{TableID: rt, Command: "select", Predicate: `"Quantité" < 100`, Label: "Petites quantités"})
	must(t, err)
	refresh(t, bob)
	if _, filt := bob.access(); !filt {
		t.Fatal("restriction de lecture : vue non filtrée")
	}
	if r := bob.d.Share.Restrictions(tableID(t, bob.d)); len(r) != 1 || r[0].Label != "Petites quantités" {
		t.Fatalf("restrictions : %+v", r)
	}
	// « Tous les rôles » ne vise pas la propriétaire (read_filtered).
	refresh(t, alice)
	if _, filt := alice.access(); filt {
		t.Fatal("restriction pour tous : vue filtrée pour la propriétaire")
	}
	must(t, as.DropRestriction(ctx, rid))
	// Filtre d'écriture seul : la lecture n'est pas filtrée.
	must(t, as.SetLevels(ctx, []workbook.Level{{RoleID: nord, NodeKind: workbook.NodeTable, NodeID: rt, Level: "editor", WriteFilter: `"Quantité" < 100`}}))
	refresh(t, bob)
	if _, filt := bob.access(); filt {
		t.Fatal("filtre d'écriture seul : vue filtrée")
	}
	// Serveur sans la migration 0009 : « filtered » (lecture ou écriture).
	f.Before09 = true
	refresh(t, bob)
	if _, filt := bob.access(); !filt {
		t.Fatal("serveur sans 0009 : filtre non vu")
	}
	f.Before09 = false
	must(t, as.DropRole(ctx, nord))
	refresh(t, bob)
	if lv, _ := bob.access(); lv != workbook.LevelNone {
		t.Fatalf("rôle supprimé : %q", lv)
	}
	// Bob ne gère pas les droits.
	if _, err := f.As("bob").ACL(ctx, id); !workbook.Privilege(err) {
		t.Fatalf("ACL par Bob : %v", err)
	}
}

// Niveau posé sur une feuille (migration 0009 : create_table reçoit la
// feuille) : hérité par ses tableaux, sauf niveau propre du tableau.
func TestRightsSheetLevel(t *testing.T) {
	ctx := context.Background()
	f, _, bob, id := setup(t)
	as := f.As("alice")
	rt := aclTableID(t, as, id)
	acl, err := as.ACL(ctx, id)
	must(t, err)
	if acl.Tables[0].SheetID != "1" {
		t.Fatalf("feuille du tableau : %+v", acl.Tables)
	}
	nord, err := as.CreateRole(ctx, id, "Nord")
	must(t, err)
	must(t, as.RevokeRole(ctx, roleID(t, as, id, "editor"), "bob"))
	must(t, as.GrantRole(ctx, nord, "bob"))
	must(t, as.SetLevels(ctx, []workbook.Level{{RoleID: nord, NodeKind: workbook.NodeSheet, NodeID: "1", Level: "reader"}}))
	refresh(t, bob)
	if lv, _ := bob.access(); lv != workbook.LevelReader {
		t.Fatalf("niveau de feuille : %q", lv)
	}
	must(t, as.SetLevels(ctx, []workbook.Level{{RoleID: nord, NodeKind: workbook.NodeTable, NodeID: rt, Level: "editor"}}))
	refresh(t, bob)
	if lv, _ := bob.access(); lv != workbook.LevelEditor {
		t.Fatalf("niveau propre du tableau : %q", lv)
	}
	caps, err := f.As("bob").Capabilities(ctx, id)
	must(t, err)
	if a := caps.Tables[rt]; a.SheetID != "1" || a.ReadFiltered == nil || *a.ReadFiltered {
		t.Fatalf("capacités : %+v", a)
	}
}

func TestRightsInvitations(t *testing.T) {
	ctx := context.Background()
	f, _, _, id := setup(t)
	as := f.As("alice")
	inv, err := as.Invite(ctx, id, "carol@example.test", []string{"participant"})
	must(t, err)
	acl, err := as.ACL(ctx, id)
	must(t, err)
	if len(acl.Invitations) != 1 || acl.Invitations[0].Email != "carol@example.test" {
		t.Fatalf("invitations : %+v", acl.Invitations)
	}
	must(t, as.RevokeInvitation(ctx, inv))
	if _, err := as.Invite(ctx, id, "dan@example.test", []string{"Nord"}); err == nil {
		t.Fatal("rôle personnalisé inexistant accepté à l'invitation")
	}
	// Rôle personnalisé par son nom (migration 0009) : adresse inconnue,
	// invitation et compte en attente (grant_role vaut ensuite) ; adresse
	// connue, rôle donné tout de suite.
	nord, err := as.CreateRole(ctx, id, "Nord")
	must(t, err)
	_, err = as.Invite(ctx, id, "dan@example.test", []string{"nord"})
	must(t, err)
	must(t, as.GrantRole(ctx, nord, "dan"))
	f.As("frank")
	_, err = as.Invite(ctx, id, "frank@example.test", []string{"NORD", "reader"})
	must(t, err)
	members, err := as.Members(ctx, id)
	must(t, err)
	i := slices.IndexFunc(members, func(m workbook.WorkbookMember) bool { return m.UserID == "frank" })
	if i < 0 || !slices.Equal(members[i].Roles, []string{"Nord", "reader"}) {
		t.Fatalf("membres : %+v", members)
	}
	if _, err := f.As("bob").Members(ctx, id); !workbook.Privilege(err) {
		t.Fatalf("membres lus par un éditeur : %v", err)
	}
	f.Before09 = true
	if _, err := as.Invite(ctx, id, "gina@example.test", []string{"Nord"}); err == nil {
		t.Fatal("serveur sans 0009 : rôle personnalisé accepté")
	}
	if _, err := as.Members(ctx, id); !errors.Is(err, workbook.ErrUnsupported) {
		t.Fatalf("serveur sans 0009 : membres %v", err)
	}
	f.Before09 = false
	// Utilisateur déjà connu : rôle donné tout de suite.
	f.As("erin")
	_, err = as.Invite(ctx, id, "erin@example.test", []string{"reader"})
	must(t, err)
	caps, err := f.As("erin").Capabilities(ctx, id)
	must(t, err)
	if !slices.Equal(caps.Roles, []string{"reader"}) {
		t.Fatalf("Erin : %+v", caps.Roles)
	}
	// Dernier propriétaire : gardé.
	if err := as.RevokeRole(ctx, roleID(t, as, id, "owner"), "alice"); err == nil {
		t.Fatal("dernier propriétaire retiré")
	}
}

// Les droits changés sur le serveur arrivent par la notification *acl :
// la grille grise le tableau sans attendre un refus.
func TestRightsNotified(t *testing.T) {
	old := collab.Debounce
	collab.Debounce = 20 * time.Millisecond
	t.Cleanup(func() { collab.Debounce = old })
	f, _, bob, id := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notices := make(chan []string, 8)
	go bob.e.Follow(ctx, f.As("bob"), time.Hour, func(ev collab.Event) {
		if len(ev.Notices) > 0 {
			notices <- ev.Notices
		}
	})
	waitOnline(t, bob)
	as := f.As("alice")
	must(t, as.RevokeRole(ctx, roleID(t, as, id, "editor"), "bob"))
	must(t, as.GrantRole(ctx, roleID(t, as, id, "reader"), "bob"))
	deadline := time.After(5 * time.Second)
	for {
		if lv, _ := bob.access(); lv == workbook.LevelReader {
			break
		}
		select {
		case <-deadline:
			t.Fatal("changement de droits jamais reçu")
		case <-time.After(10 * time.Millisecond):
		}
	}
	select {
	case n := <-notices:
		if !slices.Contains(n, collab.RightsNotice) {
			t.Fatalf("message : %v", n)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pas de message")
	}
	if err := bob.set(0, 1, "99"); err == nil {
		t.Fatal("lecteur : saisie acceptée")
	}
}

func TestRightsDriftAndPreview(t *testing.T) {
	ctx := context.Background()
	f, _, _, id := setup(t)
	as := f.As("alice")
	d, err := as.Drift(ctx, id)
	must(t, err)
	if len(d) != 0 {
		t.Fatalf("écarts : %+v", d)
	}
	f.SetDrift(id, []workbook.Drift{{TableID: "x", Name: "Ventes", ForeignPolicies: []string{"main_libre"}}})
	if d, _ := as.Drift(ctx, id); len(d) != 1 || d[0].ForeignPolicies[0] != "main_libre" {
		t.Fatalf("écarts : %+v", d)
	}
	if _, err := f.As("bob").Drift(ctx, id); err == nil {
		t.Fatal("écarts lus par un éditeur")
	}
	// Recompiler (compile_acl) : empreinte reprise, plus d'écart.
	must(t, as.Recompile(ctx, id))
	if d, _ := as.Drift(ctx, id); len(d) != 0 {
		t.Fatalf("écarts après recompilation : %+v", d)
	}
	if err := f.As("bob").Recompile(ctx, id); !workbook.Privilege(err) {
		t.Fatalf("recompilation par un éditeur : %v", err)
	}
	p := as.(workbook.Previewer)
	res, err := p.Preview(ctx, id, "bob", aclTableID(t, as, id))
	must(t, err)
	if len(res.Rows) != 3 || res.Rows[0]["Produit"] != "Café" {
		t.Fatalf("aperçu : %+v", res)
	}
	if _, err := f.As("bob").(workbook.Previewer).Preview(ctx, id, "alice", aclTableID(t, as, id)); err == nil {
		t.Fatal("aperçu par un éditeur")
	}
}
