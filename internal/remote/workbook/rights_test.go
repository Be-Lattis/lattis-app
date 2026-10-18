package workbook_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/remote/workbook"
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

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
