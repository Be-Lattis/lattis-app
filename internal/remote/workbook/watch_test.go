package workbook_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

func TestWatchFollowsServer(t *testing.T) {
	useRoot(t)
	ctx := context.Background()
	fake := workbooktest.New()
	a := author(t)
	info := publish(t, a, fake.As("alice"), workbook.Frozen, "Ventes")
	must(t, fake.Grant(info.WorkbookID, "bob", "reader"))
	r := open(t, fake.As("bob"), info.WorkbookID, "bob")
	r.Share.SetOnline(false, "")

	changes := make(chan workbook.State, 64)
	wctx, stop := context.WithCancel(ctx)
	defer stop()
	go workbook.Watch(wctx, fake.As("bob"), r.Share, 20*time.Millisecond, func() { changes <- r.Share.State() })
	wait := func(what string, ok func(workbook.State) bool) {
		t.Helper()
		for {
			select {
			case st := <-changes:
				if ok(st) {
					return
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("%s : %+v", what, r.Share.State())
			}
		}
	}
	wait("en ligne", func(s workbook.State) bool { return s.Online && s.ReadOnly == "Classeur publié en lecture seule" })
	_, err := workbook.Republish(ctx, fake.As("alice"), a.DB(), nil)
	must(t, err)
	wait("nouvelle version", func(s workbook.State) bool { return s.NewVersion == 2 })
	fake.SetDown(true)
	wait("hors ligne", func(s workbook.State) bool { return !s.Online && s.ReadOnly == "Hors ligne, lecture seule" })
	if _, err := r.SetCell(ctx, 1, 1, 0, "x"); err == nil || !strings.Contains(err.Error(), "Hors ligne") {
		t.Fatalf("saisie hors ligne : %v", err)
	}
	fake.SetDown(false)
	wait("de retour", func(s workbook.State) bool { return s.Online })
	if v, ok := workbook.VersionNotice("*version:12"); !ok || v != 12 {
		t.Fatal("VersionNotice")
	}
	if _, ok := workbook.VersionNotice("*meta"); ok {
		t.Fatal("VersionNotice *meta")
	}
}
