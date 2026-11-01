package workbook_test

import (
	"context"
	"testing"

	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
	"github.com/lalamefine/lattis/internal/work"
)

// loadSteps suit un chargement comme la barre d'état (work.Delay et
// work.Every à zéro) ; stop : « Annuler » dès que des lignes sont lues.
func loadSteps(t *testing.T, ctx context.Context, stop bool) (context.Context, *steps) {
	t.Helper()
	oldDelay, oldEvery := work.Delay, work.Every
	work.Delay, work.Every = 0, 0
	t.Cleanup(func() { work.Delay, work.Every = oldDelay, oldEvery })
	s := &steps{}
	ctx, s.tr = work.Start(ctx, func(st work.Step) {
		s.emit(st)
		if stop && st.Phase == "rows" && st.Done > 0 {
			s.tr.Cancel()
		}
	}, true)
	return ctx, s
}

// Chargement suivi : mise en page, puis les lignes de chaque tableau,
// comptées, puis l'écriture sur le disque.
func TestLoadProgress(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	fake := workbooktest.New()
	src := newDoc(t)
	const n = 500
	bigTable(t, src, n)
	id := tableID(t, src, "Gros")
	info, err := workbook.Publish(ctx, fake.As("alice"), src.DB(), workbook.Request{Server: server, User: "alice", Title: "Gros", Mode: workbook.Frozen, Tables: []int64{id}})
	must(t, err)

	d := newDoc(t)
	wctx, s := loadSteps(t, ctx, false)
	_, err = workbook.Load(wctx, fake.As("alice"), d.DB(), workbook.Source{Server: server, User: "alice", WorkbookID: info.WorkbookID}, true)
	s.tr.End()
	must(t, err)
	got := s.phases()
	for _, p := range []string{"readLayout", "rows", "place", "commit"} {
		if _, ok := got[p]; !ok {
			t.Fatalf("étape %s absente : %+v", p, got)
		}
	}
	if r := got["rows"]; r.Item != "Gros" || r.Done != n || r.Total != 0 {
		t.Fatalf("lignes lues : %+v", r)
	}
}

// Annulé pendant la lecture des lignes : un classeur neuf reste vide, un
// classeur déjà chargé reste tel quel (une seule transaction locale).
func TestLoadCancelLeavesNothing(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	fake := workbooktest.New()
	src := newDoc(t)
	bigTable(t, src, 500)
	id := tableID(t, src, "Gros")
	info, err := workbook.Publish(ctx, fake.As("alice"), src.DB(), workbook.Request{Server: server, User: "alice", Title: "Gros", Mode: workbook.Frozen, Tables: []int64{id}})
	must(t, err)
	source := workbook.Source{Server: server, User: "alice", WorkbookID: info.WorkbookID}

	d := newDoc(t)
	wctx, s := loadSteps(t, ctx, true)
	_, err = workbook.Load(wctx, fake.As("alice"), d.DB(), source, true)
	s.tr.End()
	if err == nil || !s.tr.Cancelled() {
		t.Fatalf("chargement annulé : %v", err)
	}
	var tables, marks int
	must(t, d.DB().Read().QueryRow(`SELECT (SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name GLOB 't_[0-9]*'), (SELECT count(*) FROM _lattis_row_ids)`).Scan(&tables, &marks))
	if tables != 0 || marks != 0 {
		t.Fatalf("restes du chargement annulé : %d tableaux, %d lignes", tables, marks)
	}
	if old, err := workbook.ReadInfo(ctx, d.DB().Read()); err != nil || old != nil {
		t.Fatalf("classeur noté partagé : %+v %v", old, err)
	}

	// Rechargement annulé : ce qui était là reste.
	_, err = workbook.Load(ctx, fake.As("alice"), d.DB(), source, true)
	must(t, err)
	var before, after int
	must(t, d.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_row_ids`).Scan(&before))
	wctx, s = loadSteps(t, ctx, true)
	_, err = workbook.Load(wctx, fake.As("alice"), d.DB(), source, false)
	s.tr.End()
	if err == nil || !s.tr.Cancelled() {
		t.Fatalf("rechargement annulé : %v", err)
	}
	must(t, d.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_row_ids`).Scan(&after))
	if before != 500 || after != before {
		t.Fatalf("lignes avant %d, après %d", before, after)
	}
}

// Contre PostgreSQL : l'annulation pendant la lecture des lignes rend la main
// (requête interrompue), le classeur neuf reste vide, et le chargement refait
// aboutit sur la même connexion du pool.
func TestPostgresLoadCancel(t *testing.T) {
	env := pgSetup(t)
	useRoot(t)
	ctx := context.Background()
	alice, _ := env.user(t, "alice")
	src := newDoc(t)
	const n = 5000
	bigTable(t, src, n)
	info, err := workbook.Publish(ctx, env.as(alice), src.DB(), workbook.Request{Server: server, User: alice, Title: "Gros", Mode: workbook.Frozen, Tables: []int64{tableID(t, src, "Gros")}})
	must(t, err)
	source := workbook.Source{Server: server, User: alice, WorkbookID: info.WorkbookID}

	d := newDoc(t)
	wctx, s := loadSteps(t, ctx, true)
	_, err = workbook.Load(wctx, env.as(alice), d.DB(), source, true)
	s.tr.End()
	if err == nil || !s.tr.Cancelled() {
		t.Fatalf("chargement annulé : %v", err)
	}
	var tables int
	must(t, d.DB().Read().QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name GLOB 't_[0-9]*'`).Scan(&tables))
	if tables != 0 {
		t.Fatalf("%d tableaux après l'annulation", tables)
	}
	_, err = workbook.Load(ctx, env.as(alice), d.DB(), source, true)
	must(t, err)
	var got int
	must(t, d.DB().Read().QueryRow(`SELECT count(*) FROM _lattis_row_ids`).Scan(&got))
	if got != n {
		t.Fatalf("lignes chargées : %d", got)
	}
}
