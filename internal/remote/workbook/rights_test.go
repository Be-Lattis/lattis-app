package workbook_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

func TestHTTPPreview(t *testing.T) {
	ctx := context.Background()
	var got map[string]string
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer jeton" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(status)
		if status == http.StatusOK {
			w.Write([]byte(`{"rows":[{"Produit":"Café","Quantité":3,"lattis_author":"alice"}],"truncated":false}`))
		} else {
			w.Write([]byte(`{"error":"forbidden","error_description":"réservé"}`))
		}
	}))
	defer srv.Close()
	tok := func(context.Context) (string, error) { return "jeton", nil }
	p := workbook.HTTPPreview(nil, srv.URL, tok)
	res, err := p.Preview(ctx, "wb", "u", "t")
	must(t, err)
	if got["workbook_id"] != "wb" || got["user_id"] != "u" || got["table_id"] != "t" {
		t.Fatalf("corps : %v", got)
	}
	if len(res.Rows) != 1 || res.Rows[0]["Produit"] != "Café" || res.Rows[0]["Quantité"] != json.Number("3") {
		t.Fatalf("lignes : %+v", res)
	}
	status = http.StatusForbidden
	if _, err := p.Preview(ctx, "wb", "u", "t"); err == nil || !strings.Contains(err.Error(), "propriétaires et concepteurs") {
		t.Fatalf("403 : %v", err)
	}
	bad := workbook.HTTPPreview(nil, srv.URL, func(context.Context) (string, error) { return "autre", nil })
	if _, err := bad.Preview(ctx, "wb", "u", "t"); err == nil || !strings.Contains(err.Error(), "jeton") {
		t.Fatalf("401 : %v", err)
	}
	if _, err := workbook.HTTPPreview(nil, "", tok).Preview(ctx, "wb", "u", "t"); err != workbook.ErrNoPreview {
		t.Fatalf("sans adresse : %v", err)
	}
}

func TestCheckLevelAndRestriction(t *testing.T) {
	for _, l := range []workbook.Level{
		{NodeKind: "colonne", Level: "reader"},
		{NodeKind: "table", Level: "chef"},
		{NodeKind: "sheet", Level: "reader", ReadFilter: "x = 1"},
	} {
		if workbook.CheckLevel(l) == nil {
			t.Fatalf("accepté : %+v", l)
		}
	}
	must(t, workbook.CheckLevel(workbook.Level{NodeKind: "table", Level: "", ReadFilter: "x = 1"}))
	if workbook.CheckRestriction(workbook.RestrictionRule{TableID: "t", Command: "truncate", Predicate: "true"}) == nil {
		t.Fatal("commande inconnue acceptée")
	}
	if workbook.CheckRestriction(workbook.RestrictionRule{TableID: "t", Command: "all"}) == nil {
		t.Fatal("restriction sans condition acceptée")
	}
}

// TestFakeTeams : équipes Lattis du faux serveur (comme la migration 0012) :
// rôle donné à une équipe, ses membres le reçoivent (capacités), membres
// par l'équipe, *acl quand l'équipe change ; serveur sans 0012 : ancien
// comportement.
func TestFakeTeams(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	fake := workbooktest.New()
	as := fake.As("alice")
	info := publish(t, author(t), as, workbook.Collaborative, "Ventes")
	nord := fake.CreateTeam("Nord")
	fake.AddTeamMember(nord, "dave")
	teams, err := as.Teams(ctx)
	must(t, err)
	if len(teams) != 1 || teams[0].Name != "Nord" || teams[0].MemberCount != 1 || teams[0].Members != nil {
		t.Fatalf("équipes : %+v", teams)
	}
	fake.ServerOwner("alice")
	if teams, _ := as.Teams(ctx); len(teams[0].Members) != 1 || teams[0].Members[0].UserID != "dave" {
		t.Fatalf("équipes vues par un propriétaire du serveur : %+v", teams)
	}
	if _, err := fake.As("dave").Capabilities(ctx, info.WorkbookID); err == nil {
		t.Fatal("Dave a accès sans rôle")
	}
	acl, err := as.ACL(ctx, info.WorkbookID)
	must(t, err)
	editor := acl.Roles[slices.IndexFunc(acl.Roles, func(r workbook.Role) bool { return r.Builtin == "editor" })].ID
	notified := make(chan string, 16)
	lctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ready := make(chan struct{})
	go as.Listen(lctx, info.Channel, func() { close(ready) }, func(p string) { notified <- p })
	<-ready
	must(t, as.GrantRole(ctx, editor, nord))
	caps, err := fake.As("dave").Capabilities(ctx, info.WorkbookID)
	must(t, err)
	if !slices.Contains(caps.Roles, "editor") {
		t.Fatalf("capacités de Dave par son équipe : %+v", caps)
	}
	acl, err = as.ACL(ctx, info.WorkbookID)
	must(t, err)
	for _, r := range acl.Roles {
		if want := r.ID == editor; want != (len(r.Teams) == 1 && r.Teams[0].TeamID == nord && r.Teams[0].Name == "Nord") || len(r.Members) > 0 && r.Builtin != "owner" {
			t.Fatalf("rôle %s : %+v", r.Name, r)
		}
	}
	members, err := as.Members(ctx, info.WorkbookID)
	must(t, err)
	if len(members) != 2 || members[1].UserID != "dave" || !slices.Equal(members[1].Roles, []string{"editor"}) || !slices.Equal(members[1].Teams, []string{"Nord"}) || len(members[0].Teams) != 0 {
		t.Fatalf("membres : %+v", members)
	}
	// Un membre ajouté à l'équipe : *acl sur le canal du classeur.
	for len(notified) > 0 {
		<-notified
	}
	fake.AddTeamMember(nord, "erin")
	select {
	case p := <-notified:
		if p != workbook.NoticeACL {
			t.Fatalf("notification : %q", p)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pas de *acl pour un membre ajouté à l'équipe")
	}
	must(t, as.RevokeRole(ctx, editor, nord))
	if _, err := fake.As("dave").Capabilities(ctx, info.WorkbookID); err == nil {
		t.Fatal("Dave garde l'accès après le retrait du rôle de son équipe")
	}

	// Serveur sans la migration 0012.
	fake.Before12 = true
	if _, err := as.Teams(ctx); !errors.Is(err, workbook.ErrUnsupported) {
		t.Fatalf("équipes sans 0012 : %v", err)
	}
	if err := as.GrantRole(ctx, editor, nord); err == nil {
		t.Fatal("rôle donné à une équipe sans 0012")
	}
	acl, err = as.ACL(ctx, info.WorkbookID)
	must(t, err)
	if acl.Roles[0].Teams != nil {
		t.Fatalf("équipes d'un rôle sans 0012 : %+v", acl.Roles[0])
	}
}

// Droits contre PostgreSQL 18 et les migrations du serveur : liste des
// droits, rôle personnalisé avec filtre de lecture (une ligne invisible
// n'est pas dans le cache), restriction, écarts, invitations, capacités
// relues, notification *acl (migration 0008).
func TestPostgresACL(t *testing.T) {
	env := pgSetup(t)
	useRoot(t)
	ctx := context.Background()
	alice, _ := env.user(t, "alice")
	carol, carolID := env.user(t, "carol")
	a := author(t)
	as := env.as(alice)
	info, err := workbook.Publish(ctx, as, a.DB(), workbook.Request{Server: server, User: alice, Title: "Droits", Mode: workbook.Collaborative, Tables: []int64{tableID(t, a, "Ventes")}})
	must(t, err)
	must(t, a.ShareChanged(ctx))
	env.addMember(t, info.WorkbookID, "participant", carolID)

	acl, err := as.ACL(ctx, info.WorkbookID)
	must(t, err)
	if acl.Workbook.ID != info.WorkbookID || len(acl.Tables) != 1 || acl.Tables[0].Name != "Ventes" {
		t.Fatalf("ACL : %+v", acl)
	}
	role := func(key string) workbook.Role {
		acl, err := as.ACL(ctx, info.WorkbookID)
		must(t, err)
		for _, r := range acl.Roles {
			if r.Builtin == key || r.Name == key {
				return r
			}
		}
		t.Fatalf("rôle %q introuvable : %+v", key, acl.Roles)
		return workbook.Role{}
	}
	if p := role("participant"); len(p.Members) != 1 || p.Members[0].UserID != carolID {
		t.Fatalf("participants : %+v", p)
	}
	if _, err := env.as(carol).ACL(ctx, info.WorkbookID); !workbook.Privilege(err) {
		t.Fatalf("ACL par Carol : %v", err)
	}
	rt := acl.Tables[0].TableID

	// Carol ouvre en participante ; ses capacités le disent.
	c := open(t, env.as(carol), info.WorkbookID, carol)
	ctbl := tableID(t, c, "Ventes")
	if lv, filt := c.Share.TableAccess(ctbl); lv != workbook.LevelParticipant || filt {
		t.Fatalf("Carol : %q %v", lv, filt)
	}
	locked, err := c.Share.LockedRows(ctx, ctbl, []int64{1, 2, 3})
	must(t, err)
	if len(locked) != 3 {
		t.Fatalf("lignes d'Alice verrouillées : %v", locked)
	}
	if _, err := c.SetCell(ctx, 1, 1, 1, "99"); err == nil || !strings.Contains(err.Error(), "participant") {
		t.Fatalf("participante : %v", err)
	}

	// Écoute du canal (notification *acl si le serveur la donne).
	notified := make(chan string, 16)
	lctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ready := make(chan struct{})
	go env.as(carol).Listen(lctx, info.Channel, func() { close(ready) }, func(p string) { notified <- p })
	<-ready

	// Rôle personnalisé « Nord » : éditeur du tableau, filtre de lecture.
	nord, err := as.CreateRole(ctx, info.WorkbookID, "Nord")
	must(t, err)
	err = as.SetLevels(ctx, []workbook.Level{{RoleID: nord, NodeKind: workbook.NodeTable, NodeID: rt, Level: "editor", ReadFilter: `"Produit"; DROP TABLE x`}})
	if code, _ := workbook.Rejected(err); code != "22023" {
		t.Fatalf("filtre dangereux : %v", err)
	}
	must(t, as.SetLevels(ctx, []workbook.Level{{RoleID: nord, NodeKind: workbook.NodeTable, NodeID: rt, Level: "editor", ReadFilter: `"Produit" = 'Café'`}}))
	must(t, as.GrantRole(ctx, nord, carolID))
	must(t, as.RevokeRole(ctx, role("participant").ID, carolID))
	if g := role("Nord").Grants; !slices.ContainsFunc(g, func(g workbook.Grant) bool { return g.NodeID == rt && g.ReadFilter != "" }) {
		t.Fatalf("niveaux de Nord : %+v", g)
	}
	changed, err := workbook.RefreshCapabilities(ctx, env.as(carol), c.Share)
	must(t, err)
	if lv, filt := c.Share.TableAccess(ctbl); !changed || lv != workbook.LevelEditor || !filt {
		t.Fatalf("Carol après Nord : %v %q %v", changed, lv, filt)
	}
	l, err := c.Layout(ctx, 1)
	must(t, err)
	for _, ti := range l.Tables {
		if ti.Name == "Ventes" && (!ti.Filtered || ti.Access != "editor") {
			t.Fatalf("mise en page : %+v", ti)
		}
	}
	// Lecture filtrée : seules les lignes visibles arrivent dans le cache
	// (REFERENCE vers une autre ligne : #N/A, comme une ligne absente).
	c2 := open(t, env.as(carol), info.WorkbookID, carol)
	if got := column(t, c2, "Ventes", "Produit"); !slices.Equal(got, []string{"Café"}) {
		t.Fatalf("lignes vues par Carol : %v", got)
	}

	// Restriction de lecture pour Nord : plus rien.
	rid, err := as.AddRestriction(ctx, info.WorkbookID, workbook.RestrictionRule{TableID: rt, Command: "select", AppliesTo: []string{nord}, Predicate: `"Quantité" > 5`, Label: "Grosses quantités"})
	must(t, err)
	c3 := open(t, env.as(carol), info.WorkbookID, carol)
	if got := column(t, c3, "Ventes", "Produit"); len(got) != 0 {
		t.Fatalf("après restriction : %v", got)
	}
	if r := c3.Share.Restrictions(tableID(t, c3, "Ventes")); len(r) != 1 || r[0].Label != "Grosses quantités" {
		t.Fatalf("restrictions de Carol : %+v", r)
	}
	must(t, as.DropRestriction(ctx, rid))

	// Écarts : une règle posée à la main.
	d, err := as.Drift(ctx, info.WorkbookID)
	must(t, err)
	if len(d) != 0 {
		t.Fatalf("écarts : %+v", d)
	}
	env.Exec(t, `CREATE POLICY main_libre ON `+quoteIdent(info.Schema)+`."Ventes" FOR SELECT USING (true)`)
	d, err = as.Drift(ctx, info.WorkbookID)
	must(t, err)
	if len(d) != 1 || !slices.Contains(d[0].ForeignPolicies, "main_libre") {
		t.Fatalf("écarts : %+v", d)
	}

	if has0009() {
		rights0009(t, env, as, info.WorkbookID, rt, carol, carolID)
	} else if _, err := as.Members(ctx, info.WorkbookID); !errors.Is(err, workbook.ErrUnsupported) {
		t.Fatalf("membres sans 0009 : %v", err)
	}
	if hasMigration("0012") {
		rights0012(t, env, as, alice, info.WorkbookID, rt)
	} else if _, err := as.Teams(ctx); !errors.Is(err, workbook.ErrUnsupported) {
		t.Fatalf("équipes sans 0012 : %v", err)
	}

	// Invitations.
	inv, err := as.Invite(ctx, info.WorkbookID, "dan@example.test", []string{"reader"})
	must(t, err)
	acl, err = as.ACL(ctx, info.WorkbookID)
	must(t, err)
	if len(acl.Invitations) != 1 || acl.Invitations[0].Email != "dan@example.test" {
		t.Fatalf("invitations : %+v", acl.Invitations)
	}
	must(t, as.RevokeInvitation(ctx, inv))
	must(t, as.DropRole(ctx, nord))

	// Notification *acl (migration 0008 du serveur).
	if has0008() {
		deadline := time.After(5 * time.Second)
		for {
			select {
			case p := <-notified:
				if p == workbook.NoticeACL {
					return
				}
			case <-deadline:
				t.Fatal("pas de notification *acl")
			}
		}
	}
}

func has0008() bool {
	m, _ := filepath.Glob(filepath.Join(os.Getenv("LATTIS_SERVER_SCHEMA"), "0008_*.sql"))
	return len(m) > 0
}

func has0009() bool { return hasMigration("0009") }

// hasMigration : le schéma du serveur essayé a la migration n (« 0012 »).
func hasMigration(n string) bool {
	m, _ := filepath.Glob(filepath.Join(os.Getenv("LATTIS_SERVER_SCHEMA"), n+"_*.sql"))
	return len(m) > 0
}

// rights0012 : équipes Lattis (migration 0012) : liste des équipes (membres
// pour un propriétaire du serveur seulement), rôle donné à une équipe
// (workbook_acl.teams, workbook_members avec « teams »), filtre « mon
// équipe » (IN (SELECT lattis.my_teams())) appliqué à la lecture d'un
// membre de l'équipe, rôle retiré.
func rights0012(t *testing.T, env *pgEnv, as workbook.Server, alice, wb, rt string) {
	t.Helper()
	ctx := context.Background()
	erin, erinID := env.user(t, "erin")
	// L'équipe porte le nom d'un produit : « mon équipe » le compare à la
	// colonne Produit.
	team := env.Text(t, `SELECT lattis.create_team('Café')::text`)
	env.Exec(t, `SELECT lattis.add_team_member($1::uuid, $2::uuid)`, team, erinID)
	teams, err := as.Teams(ctx)
	must(t, err)
	if len(teams) != 1 || teams[0].ID != team || teams[0].Name != "Café" || teams[0].MemberCount != 1 || teams[0].Members != nil {
		t.Fatalf("équipes vues par Alice : %+v", teams)
	}
	role, err := as.CreateRole(ctx, wb, "Par équipe")
	must(t, err)
	must(t, as.SetLevels(ctx, []workbook.Level{{RoleID: role, NodeKind: workbook.NodeTable, NodeID: rt, Level: "reader", ReadFilter: `"Produit" IN (SELECT lattis.my_teams())`}}))
	must(t, as.GrantRole(ctx, role, team))
	acl, err := as.ACL(ctx, wb)
	must(t, err)
	i := slices.IndexFunc(acl.Roles, func(r workbook.Role) bool { return r.ID == role })
	if i < 0 || len(acl.Roles[i].Teams) != 1 || acl.Roles[i].Teams[0].TeamID != team || acl.Roles[i].Teams[0].Name != "Café" || len(acl.Roles[i].Members) != 0 {
		t.Fatalf("rôle de l'équipe : %+v", acl.Roles)
	}
	members, err := as.Members(ctx, wb)
	must(t, err)
	j := slices.IndexFunc(members, func(m workbook.WorkbookMember) bool { return m.UserID == erinID })
	if j < 0 || !slices.Equal(members[j].Teams, []string{"Café"}) || !slices.Equal(members[j].Roles, []string{"Par équipe"}) {
		t.Fatalf("membres par l'équipe : %+v", members)
	}
	// Erin lit Ventes par son équipe : seules les lignes « Café ».
	e := open(t, env.as(erin), wb, erin)
	if got := column(t, e, "Ventes", "Produit"); len(got) == 0 || slices.ContainsFunc(got, func(p string) bool { return p != "Café" }) {
		t.Fatalf("lignes vues par Erin (mon équipe) : %v", got)
	}
	// Propriétaire du serveur : les membres de l'équipe.
	env.Exec(t, `GRANT lattis_server_owners TO `+quoteIdent(alice))
	teams, err = as.Teams(ctx)
	must(t, err)
	if len(teams) != 1 || len(teams[0].Members) != 1 || teams[0].Members[0].UserID != erinID {
		t.Fatalf("équipes vues par un propriétaire du serveur : %+v", teams)
	}
	env.Exec(t, `REVOKE lattis_server_owners FROM `+quoteIdent(alice))
	must(t, as.RevokeRole(ctx, role, team))
	acl, err = as.ACL(ctx, wb)
	must(t, err)
	if i := slices.IndexFunc(acl.Roles, func(r workbook.Role) bool { return r.ID == role }); i < 0 || len(acl.Roles[i].Teams) != 0 {
		t.Fatalf("rôle retiré à l'équipe : %+v", acl.Roles)
	}
	members, err = as.Members(ctx, wb)
	must(t, err)
	if slices.ContainsFunc(members, func(m workbook.WorkbookMember) bool { return m.UserID == erinID }) {
		t.Fatalf("Erin encore membre : %+v", members)
	}
	must(t, as.DropRole(ctx, role))
}

// rights0009 : retours du serveur sur les droits (migration 0009) : feuille
// des tableaux (create_table), niveau de feuille, lecture filtrée détaillée,
// membres, invitation à un rôle personnalisé, recompilation.
func rights0009(t *testing.T, env *pgEnv, as workbook.Server, wb, rt, carol, carolID string) {
	t.Helper()
	ctx := context.Background()
	// Recompiler (avant tout set_level, qui recompile aussi) : l'empreinte
	// est reprise ; la règle posée à la main reste dans PostgreSQL et, avec
	// la migration 0011 à jour, reste signalée (droits.md §4) tant qu'elle
	// n'est pas retirée.
	must(t, as.Recompile(ctx, wb))
	d, err := as.Drift(ctx, wb)
	must(t, err)
	if len(d) > 1 || len(d) == 1 && !slices.Equal(d[0].ForeignPolicies, []string{"main_libre"}) {
		t.Fatalf("écarts après recompilation : %+v", d)
	}
	acl0, err := as.ACL(ctx, wb)
	must(t, err)
	env.Exec(t, `DROP POLICY main_libre ON `+quoteIdent(acl0.Workbook.Schema)+`."Ventes"`)
	d, err = as.Drift(ctx, wb)
	must(t, err)
	if len(d) != 0 {
		t.Fatalf("écarts après le retrait de la règle : %+v", d)
	}
	acl, err := as.ACL(ctx, wb)
	must(t, err)
	if acl.Tables[0].SheetID != "1" {
		t.Fatalf("feuille de Ventes : %+v", acl.Tables)
	}
	// Carol (Nord, filtre de lecture) : read_filtered, pas write_filtered.
	caps, err := env.as(carol).Capabilities(ctx, wb)
	must(t, err)
	if a := caps.Tables[rt]; a.SheetID != "1" || a.ReadFiltered == nil || !*a.ReadFiltered || a.WriteFiltered == nil || *a.WriteFiltered {
		t.Fatalf("capacités de Carol : %+v", a)
	}
	// Niveau posé sur la feuille : hérité par Ventes.
	dave, daveID := env.user(t, "dave")
	sud, err := as.CreateRole(ctx, wb, "Sud")
	must(t, err)
	must(t, as.SetLevels(ctx, []workbook.Level{{RoleID: sud, NodeKind: workbook.NodeSheet, NodeID: "1", Level: "reader"}}))
	must(t, as.GrantRole(ctx, sud, daveID))
	caps, err = env.as(dave).Capabilities(ctx, wb)
	must(t, err)
	if a := caps.Tables[rt]; a.Level != workbook.LevelReader {
		t.Fatalf("niveau de feuille pour Dave : %+v", a)
	}
	// Membres (propriétaire), rôles par nom.
	members, err := as.Members(ctx, wb)
	must(t, err)
	i := slices.IndexFunc(members, func(m workbook.WorkbookMember) bool { return m.UserID == carolID })
	if i < 0 || !slices.Contains(members[i].Roles, "Nord") || members[i].PGRole != carol {
		t.Fatalf("membres : %+v", members)
	}
	if _, err := env.as(dave).Members(ctx, wb); !workbook.Privilege(err) {
		t.Fatalf("membres lus par Dave : %v", err)
	}
	// Invitation à un rôle personnalisé, par son nom (sans casse).
	inv, err := as.Invite(ctx, wb, "erin@example.test", []string{"sud"})
	must(t, err)
	must(t, as.RevokeInvitation(ctx, inv))
	if err := env.as(dave).Recompile(ctx, wb); !workbook.Privilege(err) {
		t.Fatalf("recompilation par Dave : %v", err)
	}
	must(t, as.DropRole(ctx, sud))
}

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
