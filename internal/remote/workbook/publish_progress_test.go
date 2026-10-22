package workbook_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
	"github.com/lalamefine/lattis/internal/work"
)

// steps suit une publication comme la barre d'état : toutes les étapes
// émises (work.Delay et work.Every à zéro) ; stopAt : « Annuler » dès que
// des lignes sont parties.
type steps struct {
	mu     sync.Mutex
	all    []work.Step
	tr     *work.Tracker
	stopAt bool
}

func (s *steps) start(t *testing.T, ctx context.Context) context.Context {
	oldDelay, oldEvery := work.Delay, work.Every
	work.Delay, work.Every = 0, 0
	t.Cleanup(func() { work.Delay, work.Every = oldDelay, oldEvery })
	ctx, s.tr = work.Start(ctx, s.emit, true)
	return ctx
}

func (s *steps) emit(st work.Step) {
	s.mu.Lock()
	s.all = append(s.all, st)
	stop := s.stopAt && st.Phase == "send" && st.Done > 0
	s.mu.Unlock()
	if stop {
		s.tr.Cancel()
	}
}

func (s *steps) phases() map[string]work.Step {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]work.Step{}
	for _, st := range s.all {
		if st.Phase != "" {
			out[st.Phase] = st
		}
	}
	return out
}

// Publication suivie : étapes dites (vérification, envoi des lignes avec
// leur nombre, enregistrement des identifiants), « Annuler » proposé
// jusqu'au point de non-retour puis retiré. Annulée pendant l'envoi : rien
// sur le serveur, rien de noté dans le classeur, qui se publie ensuite.
func TestPublishProgressAndCancel(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	fake := workbooktest.New()
	d := newDoc(t)
	const n = 10000
	bigTable(t, d, n)
	id := tableID(t, d, "Gros")
	req := workbook.Request{Server: server, User: "alice", Title: "Gros", Mode: workbook.Collaborative, Tables: []int64{id}}

	stopped := &steps{stopAt: true}
	_, err := workbook.Publish(stopped.start(t, ctx), fake.As("alice"), d.DB(), req)
	stopped.tr.End()
	if err == nil || !stopped.tr.Cancelled() {
		t.Fatalf("publication annulée : %v", err)
	}
	if list, err := fake.As("alice").Workbooks(ctx); err != nil || len(list) != 0 {
		t.Fatalf("classeurs sur le serveur après l'annulation : %+v %v", list, err)
	}
	notPublished(t, d)

	followed := &steps{}
	info, err := workbook.Publish(followed.start(t, ctx), fake.As("alice"), d.DB(), req)
	followed.tr.End()
	must(t, err)
	if rows := fake.Rows(info.WorkbookID, "Gros"); len(rows) != n {
		t.Fatalf("%d lignes publiées", len(rows))
	}
	got := followed.phases()
	for _, p := range []string{"check", "order", "prepare", "send", "layout", "finish", "record"} {
		if _, ok := got[p]; !ok {
			t.Fatalf("étape %q non dite : %+v", p, got)
		}
	}
	if s := got["send"]; s.Total != n || s.Op != "publish" || s.Name != "Gros" {
		t.Fatalf("envoi : %+v", s)
	}
	if s := got["record"]; s.Done != n || s.Total != n || s.Cancel {
		t.Fatalf("enregistrement des identifiants : %+v", s)
	}
	if !got["send"].Cancel {
		t.Fatal("« Annuler » doit être proposé pendant l'envoi")
	}
	if followed.tr.Cancel() {
		t.Fatal("après le point de non-retour, Annuler ne fait plus rien")
	}
	var ids, distinct int
	must(t, d.DB().Read().QueryRow(`SELECT count(*), count(DISTINCT lattis_id) FROM _lattis_row_ids WHERE tbl = ?`, id).Scan(&ids, &distinct))
	if ids != n || distinct != n {
		t.Fatalf("identifiants partagés : %d (%d distincts)", ids, distinct)
	}
}

// notPublished : le classeur est resté local (ni description du partage, ni
// identifiants partagés, ni types figés).
func notPublished(t *testing.T, d *doc.Document) {
	t.Helper()
	info, err := workbook.ReadInfo(context.Background(), d.DB().Read())
	must(t, err)
	if info != nil {
		t.Fatalf("classeur noté publié : %+v", info)
	}
	var n int
	must(t, d.DB().Read().QueryRow(`SELECT (SELECT count(*) FROM _lattis_row_ids) + (SELECT count(*) FROM _lattis_columns WHERE fixed = 1)`).Scan(&n))
	if n != 0 {
		t.Fatalf("%d traces de publication dans le classeur", n)
	}
}

// Contre PostgreSQL : l'annulation pendant la copie abandonne la
// transaction du serveur (aucun classeur créé), et la publication refaite
// aboutit.
func TestPostgresPublishCancel(t *testing.T) {
	env := pgSetup(t)
	useRoot(t)
	ctx := context.Background()
	alice, _ := env.user(t, "alice")
	d := newDoc(t)
	const n = 20000
	bigTable(t, d, n)
	req := workbook.Request{Server: server, User: alice, Title: "Gros", Mode: workbook.Frozen, Tables: []int64{tableID(t, d, "Gros")}}

	stopped := &steps{stopAt: true}
	start := time.Now()
	_, err := workbook.Publish(stopped.start(t, ctx), env.as(alice), d.DB(), req)
	stopped.tr.End()
	if err == nil || !stopped.tr.Cancelled() {
		t.Fatalf("publication annulée : %v", err)
	}
	t.Logf("annulée en %v", time.Since(start))
	if list, err := env.as(alice).Workbooks(ctx); err != nil || len(list) != 0 {
		t.Fatalf("classeurs sur le serveur après l'annulation : %+v %v", list, err)
	}
	if c := env.Text(t, `SELECT count(*)::text FROM lattis.workbooks`); c != "0" {
		t.Fatalf("%s classeurs dans lattis.workbooks", c)
	}
	notPublished(t, d)

	info, err := workbook.Publish(ctx, env.as(alice), d.DB(), req)
	must(t, err)
	if c := env.Text(t, `SELECT count(*)::text FROM `+quoteIdent(info.Schema)+`."Gros"`); c != "20000" {
		t.Fatalf("%s lignes publiées", c)
	}
}
