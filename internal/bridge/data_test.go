package bridge

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/query"
	"github.com/lalamefine/lattis/internal/remote/auth"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
	"github.com/lalamefine/lattis/protocol"
)

const dataServer = "https://lattis.example.test"

// deltaSpy note les relevés demandés au serveur (point de reprise, relu en
// entier ou non).
type deltaSpy struct {
	workbook.Server
	mu    sync.Mutex
	full  []bool
	since []string
	// direct : seuls les relevés appelés par l'essai (contexte sans fin)
	// atteignent le serveur ; le suivi en direct (contexte annulable),
	// lancé par le premier chargement, n'en fait aucun.
	direct bool
}

var errFollowOff = errors.New("suivi en direct coupé pour l'essai")

func (s *deltaSpy) DataCatalog(ctx context.Context) (protocol.DataCatalog, error) {
	if s.direct && ctx.Done() != nil {
		return protocol.DataCatalog{}, errFollowOff
	}
	return s.Server.DataCatalog(ctx)
}

func (s *deltaSpy) DatasetDelta(ctx context.Context, id, sqlName, since string) (workbook.DataDelta, error) {
	if s.direct && ctx.Done() != nil {
		return workbook.DataDelta{}, errFollowOff
	}
	d, err := s.Server.DatasetDelta(ctx, id, sqlName, since)
	s.mu.Lock()
	s.full = append(s.full, d.Full)
	s.since = append(s.since, since)
	s.mu.Unlock()
	return d, err
}

func (s *deltaSpy) last() (since string, full bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.since[len(s.since)-1], s.full[len(s.full)-1]
}

// dataFixture : un serveur de test avec la donnée « clients » (Nom, CA,
// Actif), et un service dont le classeur a la requête qui la lit, chargée
// comme le ferait InsertDataset (premier chargement par la requête).
type dataFixture struct {
	fake  *workbooktest.Fake
	srv   *deltaSpy
	s     *DocumentService
	d     *doc.Document
	ds    protocol.Dataset
	query int64
}

func newDataFixture(t *testing.T, share func(s *DocumentService)) *dataFixture {
	t.Helper()
	return newDataFixtureWith(t, share, false)
}

// newDataFixtureWith : direct, le suivi en direct ne relève rien (les
// relevés sont faits par l'essai seul, deltaSpy.direct).
func newDataFixtureWith(t *testing.T, share func(s *DocumentService), direct bool) *dataFixture {
	t.Helper()
	ctx := context.Background()
	f := &dataFixture{fake: workbooktest.New()}
	f.srv = &deltaSpy{Server: f.fake.As("bob"), direct: direct}
	old, oldRetry := shareServer, dataRetry
	shareServer = func(ctx context.Context, server string) (workbook.Server, auth.User, string, error) {
		return f.srv, auth.User{PGRole: "bob"}, "Serveur de test", nil
	}
	dataRetry = 20 * time.Millisecond
	t.Cleanup(func() { shareServer, dataRetry = old, oldRetry })
	id := f.fake.SeedDataset("alice", "clients", []workbook.RemoteColumn{{Name: "Nom", Type: "text"}, {Name: "CA", Type: "decimal"}, {Name: "Actif", Type: "boolean"}})
	f.fake.DataPut(id, "r1", "a0", "Dupont", 10.5, true)
	f.fake.DataPut(id, "r2", "a1", "Martin", 3.0, false)

	f.s, _ = shareService(t)
	_, err := f.s.New(ctx)
	mustShare(t, err)
	if share != nil {
		share(f.s)
	}
	f.d, err = f.s.current()
	mustShare(t, err)
	cat, err := f.srv.DataCatalog(ctx)
	mustShare(t, err)
	for _, ds := range cat.Datasets {
		if ds.ID == id {
			f.ds = ds
		}
	}
	def, err := query.EncodeConnection(query.Connection{Name: "Serveur de test", Kind: query.KindLattis, Config: query.DBConfig{URL: dataServer}})
	mustShare(t, err)
	_, err = f.d.SaveConnection(ctx, def)
	mustShare(t, err)
	f.query, err = f.s.SaveQuery(ctx, query.Query{Name: "clients", Steps: []query.Step{},
		Source: query.Source{Kind: query.SourceLattis, Connection: "Serveur de test", SQL: datasetSQL(f.ds), Dataset: f.ds.ID, Place: &canvas.QueryAnchor{Sheet: 1, Row: 0, Col: 0}}})
	mustShare(t, err)
	// Premier chargement : ce que la requête aurait lu (sans identifiants).
	rows := [][]any{{"Dupont", 10.5, int64(1)}, {"Martin", 3.0, int64(0)}}
	_, err = f.d.Model.LoadQuery(ctx, f.query, canvas.ImportTable{
		Headers: []string{"Nom", "CA", "Actif"}, Types: []grid.Type{grid.Text, grid.Decimal, grid.Boolean}, Count: len(rows),
		At: &canvas.QueryAnchor{Sheet: 1},
		Rows: func(yield func([]any) error) error {
			for _, r := range rows {
				if err := yield(r); err != nil {
					return err
				}
			}
			return nil
		},
	})
	mustShare(t, err)
	_, err = f.s.changed(ctx)
	mustShare(t, err)
	return f
}

// sync relève la donnée comme sur une notification.
func (f *dataFixture) sync(t *testing.T, full bool) {
	t.Helper()
	mustShare(t, f.s.syncDataset(context.Background(), f.d, f.srv, linkedData{query: f.query, name: "clients", dataset: f.ds.ID}, f.ds, full))
}

// rows lit le tableau lié, dans l'ordre des lignes.
func (f *dataFixture) rows(t *testing.T) string {
	t.Helper()
	tt, ok, err := f.d.Model.DatasetTable(context.Background(), f.query)
	mustShare(t, err)
	if !ok {
		t.Fatal("tableau lié absent")
	}
	rs, err := f.d.DB().Read().Query(fmt.Sprintf(`SELECT c_1, c_2, c_3 FROM t_%d ORDER BY _pos`, tt.Table))
	mustShare(t, err)
	defer rs.Close()
	var out []string
	for rs.Next() {
		var nom string
		var ca float64
		var actif int64
		mustShare(t, rs.Scan(&nom, &ca, &actif))
		out = append(out, fmt.Sprintf("%s:%g:%d", nom, ca, actif))
	}
	return strings.Join(out, " ")
}

func (f *dataFixture) undo(t *testing.T) string {
	t.Helper()
	st, err := f.s.State(context.Background())
	mustShare(t, err)
	return st.Undo
}

func TestDatasetIncremental(t *testing.T) {
	// Relevés appelés ici, pas par le suivi en direct : lancé par le premier
	// chargement, il pouvait faire le premier relevé avant d'être arrêté
	// (machine rapide : « premier relevé : since "1003" »).
	f := newDataFixtureWith(t, nil, true)
	f.s.watchData(nil)
	undo := f.undo(t)
	if undo == "" {
		t.Fatal("le premier chargement doit pouvoir s'annuler")
	}

	// Premier relevé : pas de point de reprise, tout est relu et les lignes
	// chargées par la requête reçoivent leur identifiant.
	f.sync(t, false)
	if since, full := f.srv.last(); since != "" || !full {
		t.Fatalf("premier relevé : since %q, full %v", since, full)
	}
	if got := f.rows(t); got != "Dupont:10.5:1 Martin:3:0" {
		t.Fatalf("après le premier relevé : %s", got)
	}
	tt, _, _ := f.d.Model.DatasetTable(context.Background(), f.query)
	if tt.Bound != 2 || tt.Since == "" || tt.Dataset != f.ds.ID {
		t.Fatalf("état du relevé : %+v", tt)
	}

	// Ajout, modification, suppression : seules les lignes changées sont lues.
	f.fake.DataPut(f.ds.ID, "r3", "a2", "Durand", 7.0, true)
	f.fake.DataPut(f.ds.ID, "r1", "", "Dupont SA", 12.0, true)
	f.fake.DataDelete(f.ds.ID, "r2")
	f.sync(t, false)
	if since, full := f.srv.last(); since != tt.Since || full {
		t.Fatalf("relevé incrémental : since %q (attendu %q), full %v", since, tt.Since, full)
	}
	if got := f.rows(t); got != "Dupont SA:12:1 Durand:7:1" {
		t.Fatalf("après le relevé incrémental : %s", got)
	}

	// Rien de nouveau : rien ne change.
	f.sync(t, false)
	if got := f.rows(t); got != "Dupont SA:12:1 Durand:7:1" {
		t.Fatalf("relevé sans changement : %s", got)
	}

	// Pierres tombales purgées : le point de reprise n'est plus couvert,
	// tout est relu.
	f.fake.DataDelete(f.ds.ID, "r3")
	f.fake.PurgeTombstones()
	f.sync(t, false)
	if _, full := f.srv.last(); !full {
		t.Fatal("point de reprise purgé : relecture complète attendue")
	}
	if got := f.rows(t); got != "Dupont SA:12:1" {
		t.Fatalf("après la relecture complète : %s", got)
	}

	// Une ligne sortie de l'audience ne laisse pas de pierre tombale : le
	// compte de la vue ne correspond plus, tout est relu.
	f.fake.DataPut(f.ds.ID, "r4", "a3", "Petit", 1.0, false)
	f.sync(t, false)
	if got := f.rows(t); got != "Dupont SA:12:1 Petit:1:0" {
		t.Fatalf("avant la sortie de l'audience : %s", got)
	}
	f.fake.DataHide(f.ds.ID, "r4")
	f.sync(t, false)
	if _, full := f.srv.last(); full {
		t.Fatal("ligne sortie de l'audience : relevé incrémental attendu (dataset_changed_ids)")
	}
	if got := f.rows(t); got != "Dupont SA:12:1" {
		t.Fatalf("après la sortie de l'audience : %s", got)
	}

	// Vue refaite : relecture complète demandée.
	f.sync(t, true)
	if since, full := f.srv.last(); since != "" || !full {
		t.Fatalf("vue refaite : since %q, full %v", since, full)
	}

	// Aucun relevé ne crée d'étape d'annulation.
	if got := f.undo(t); got != undo {
		t.Fatalf("annulation après les relevés : %q (attendu %q)", got, undo)
	}
}

func TestDatasetFollowLive(t *testing.T) {
	f := newDataFixture(t, nil)
	undo := f.undo(t)
	// Le suivi (relancé par changed après le chargement) relève à la
	// notification du canal du classeur source.
	wait := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for f.rows(t) != want {
			if time.Now().After(deadline) {
				t.Fatalf("attendu %q, lu %q", want, f.rows(t))
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	wait("Dupont:10.5:1 Martin:3:0")
	deadline := time.Now().Add(5 * time.Second)
	for {
		tt, _, _ := f.d.Model.DatasetTable(context.Background(), f.query)
		if tt.Bound == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("premier relevé du suivi")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// La notification peut partir avant que l'écoute soit posée : on
	// réessaie jusqu'à la voir appliquée.
	for i := 0; ; i++ {
		f.fake.DataPut(f.ds.ID, "r2", "", "Martin", float64(4+i), false)
		ok := false
		for end := time.Now().Add(300 * time.Millisecond); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
			if strings.HasPrefix(f.rows(t), "Dupont:10.5:1 Martin:") && !strings.HasSuffix(f.rows(t), "Martin:3:0") {
				ok = true
				break
			}
		}
		if ok {
			break
		}
		if i > 20 {
			t.Fatalf("notification non suivie : %s", f.rows(t))
		}
	}
	if got := f.undo(t); got != undo {
		t.Fatalf("annulation après le suivi : %q (attendu %q)", got, undo)
	}
}

func TestDatasetBatchShape(t *testing.T) {
	tt := canvas.DatasetTable{Table: 7, Columns: []canvas.Column{{ID: 1, Name: "Nom", Type: grid.Text}, {ID: 2, Name: "CA", Type: grid.Decimal}}}
	delta := workbook.DataDelta{Columns: []string{"CA", "Nom", "Nouvelle"}, Types: []string{"NUMERIC", "TEXT", "INT8"}, Since: "12", Count: 1,
		Rows: []workbook.DataRow{{ID: "r1", Pos: "a0", Values: []any{"12.50", "Dupont", int64(3)}}}, Deleted: []string{"r9"}}
	b, err := datasetBatch(tt, "d1", delta)
	mustShare(t, err)
	if len(b.Rows) != 2 || b.Rows[0].Values[1] != "Dupont" || b.Rows[0].Values[2] != 12.5 || !b.Rows[1].Deleted || b.Since != "12" || b.Count != 1 {
		t.Fatalf("relevé converti : %+v", b)
	}
	// Colonne disparue, ou de type changé : la requête est actualisée.
	if _, err := datasetBatch(tt, "d1", workbook.DataDelta{Columns: []string{"Nom"}, Types: []string{"TEXT"}}); err != errDataShape {
		t.Fatalf("colonne disparue : %v", err)
	}
	if _, err := datasetBatch(tt, "d1", workbook.DataDelta{Columns: []string{"Nom", "CA"}, Types: []string{"TEXT", "TEXT"}}); err != errDataShape {
		t.Fatalf("type changé : %v", err)
	}
	ds := protocol.Dataset{SQLName: "clients", Columns: []protocol.DataColumn{{Name: "Nom"}}}
	if !followable(query.Query{Source: query.Source{Kind: query.SourceLattis, SQL: datasetSQL(ds)}}, ds) {
		t.Fatal("requête d'InsertDataset non suivie")
	}
	if followable(query.Query{Source: query.Source{Kind: query.SourceLattis, SQL: datasetSQL(ds)}, Steps: []query.Step{{}}}, ds) ||
		followable(query.Query{Source: query.Source{Kind: query.SourceLattis, SQL: `SELECT * FROM donnees."clients" WHERE "Nom" = 'x'`}}, ds) {
		t.Fatal("requête modifiée suivie par relevés")
	}
}

func TestDatasetUnlink(t *testing.T) {
	ctx := context.Background()
	var wbID string
	f := newDataFixture(t, func(s *DocumentService) {
		_, err := s.Paste(ctx, 1, 10, 0, [][]string{{"Produit", "Prix"}, {"Café", "4,5"}})
		mustShare(t, err)
		check, err := s.ShareCheck(ctx)
		mustShare(t, err)
		st, err := s.SharePublish(ctx, ShareRequest{Server: dataServer, Title: "Ventes", Mode: workbook.Frozen, Tables: []int64{check.Tables[0].ID}})
		mustShare(t, err)
		if !st.Shared || !st.CanDesign {
			t.Fatalf("publication : %+v", st)
		}
	})
	wbID = f.d.Share.Info().WorkbookID
	usedBy := func() []string {
		cat, err := f.fake.As("alice").DataCatalog(ctx)
		mustShare(t, err)
		var out []string
		for _, ds := range cat.Datasets {
			if ds.ID == f.ds.ID {
				for _, u := range ds.UsedBy {
					out = append(out, u.WorkbookID)
				}
			}
		}
		return out
	}
	waitUsed := func(what string, want bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for slices.Contains(usedBy(), wbID) != want {
			if time.Now().After(deadline) {
				t.Fatalf("%s : utilisée par %v", what, usedBy())
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitUsed("donnée chargée : lien annoncé", true)

	// Tableau lié supprimé : lien retiré ; Annuler le rétablit.
	tt, _, _ := f.d.Model.DatasetTable(ctx, f.query)
	var name string
	mustShare(t, f.d.DB().Read().QueryRow(`SELECT name FROM _lattis_tables WHERE id = ?`, tt.Table).Scan(&name))
	_, err := f.s.DeleteTables(ctx, []string{name})
	mustShare(t, err)
	waitUsed("tableau supprimé", false)
	_, err = f.s.Undo(ctx)
	mustShare(t, err)
	waitUsed("suppression annulée", true)

	// Requête supprimée : lien retiré.
	_, err = f.s.DeleteQuery(ctx, f.query)
	mustShare(t, err)
	waitUsed("requête supprimée", false)
}

func TestQueryData(t *testing.T) {
	f := newDataFixture(t, nil)
	f.s.watchData(nil)
	r, err := f.s.QueryData(context.Background(), dataServer, "SELECT * FROM donnees.clients;")
	mustShare(t, err)
	if strings.Join(r.Columns, ",") != "Nom,CA,Actif" || len(r.Rows) != 2 || r.Rows[0][0] != "Dupont" {
		t.Fatalf("résultat : %+v", r)
	}
	if _, err := f.s.QueryData(context.Background(), dataServer, "SELECT * FROM donnees.inconnue"); err == nil || !strings.Contains(err.Error(), "inconnue") {
		t.Fatalf("vue inconnue : %v", err)
	}
	if _, err := f.s.QueryData(context.Background(), dataServer, "  ; "); err == nil {
		t.Fatal("requête vide acceptée")
	}
}
