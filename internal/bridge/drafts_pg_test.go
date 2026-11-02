package bridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/remote/workbook/workbooktest"
)

// Contrat des brouillons contre le vrai schéma du serveur (LATTIS_PG_TEST=1,
// voir workbooktest.PGSetup).
func TestPostgresDrafts(t *testing.T) {
	env := workbooktest.PGSetup(t)
	ctx := context.Background()
	alice, _ := env.User(t, "alice")
	bob, _ := env.User(t, "bob")
	a, b := workbook.PGDrafts(env.Dial(alice)), workbook.PGDrafts(env.Dial(bob))
	const id = "00000000-0000-0000-0000-00000000000a"
	at, err := a.Save(ctx, id, "Ventes", []byte{0, 1, 2, 0xff})
	mustShare(t, err)
	if at.IsZero() {
		t.Fatal("heure vide")
	}
	l, err := a.List(ctx)
	mustShare(t, err)
	if len(l) != 1 || l[0].ID != id || l[0].Title != "Ventes" || l[0].Size != 4 || l[0].UpdatedAt.IsZero() {
		t.Fatalf("liste = %+v", l)
	}
	got, err := a.Read(ctx, id)
	mustShare(t, err)
	if !bytes.Equal(got, []byte{0, 1, 2, 0xff}) {
		t.Fatalf("lu = %v", got)
	}
	// Chacun les siens.
	if l, err := b.List(ctx); err != nil || len(l) != 0 {
		t.Fatalf("liste de bob = %+v, %v", l, err)
	}
	if _, err := b.Read(ctx, id); !errors.Is(err, workbook.ErrDraftGone) {
		t.Fatalf("lecture de bob : %v", err)
	}
	// Limite : LT001, et le précédent intact.
	env.Exec(t, `UPDATE lattis.server_config SET value = '1' WHERE key = 'draft_quota_mb'`)
	if _, err := a.Save(ctx, id, "Ventes", make([]byte, 2<<20)); !workbook.DraftOverflow(err) {
		t.Fatalf("au-delà de la limite : %v", err)
	}
	if got, err := a.Read(ctx, id); err != nil || len(got) != 4 {
		t.Fatalf("après refus : %d octets, %v", len(got), err)
	}
	mustShare(t, a.Delete(ctx, id))
	mustShare(t, a.Delete(ctx, id)) // sans erreur s'il n'existe plus
	if _, err := a.Read(ctx, id); !errors.Is(err, workbook.ErrDraftGone) {
		t.Fatalf("après retrait : %v", err)
	}
}

func draftRows(name, def string) []int {
	var out []int
	for _, f := range strings.Split(cmpOr(os.Getenv(name), def), ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(f)); err == nil && n > 0 {
			out = append(out, n)
		}
	}
	return out
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// saisies : n modifications de cellule, durée de chacune.
func draftTyping(t *testing.T, s *DocumentService, n int, every time.Duration) []time.Duration {
	t.Helper()
	ctx := context.Background()
	out := make([]time.Duration, 0, n)
	for i := range n {
		start := time.Now()
		_, err := s.SetCell(ctx, 1, i%50, 0, fmt.Sprint("v", i, time.Now().UnixNano()))
		mustShare(t, err)
		out = append(out, time.Since(start))
		time.Sleep(every)
	}
	return out
}

func draftPct(d []time.Duration) string {
	if len(d) == 0 {
		return "aucune saisie"
	}
	d = slices.Clone(d)
	slices.Sort(d)
	return fmt.Sprintf("médiane %v, p95 %v, max %v", d[len(d)/2].Round(100*time.Microsecond), d[len(d)*95/100].Round(100*time.Microsecond), d[len(d)-1].Round(100*time.Microsecond))
}

func draftBig(t *testing.T, store DraftStore, rows int) (*DocumentService, *doc.Document) {
	t.Helper()
	ctx := context.Background()
	s := &DocumentService{}
	_, err := s.New(ctx)
	mustShare(t, err)
	d, err := s.current()
	mustShare(t, err)
	mustShare(t, grid.CreateDemo(ctx, d.DB(), rows, nil))
	UseDrafts(s, store, t.TempDir())
	t.Cleanup(func() {
		s.stopDraft(false)
		s.CloseDocument(ctx)
	})
	return s, d
}

// TestMesureDrafts : taille et durée d'un brouillon, saisie pendant l'envoi,
// restauration (LATTIS_PG_TEST=1 LATTIS_DRAFT_MESURE=1000,100000,1000000).
func TestMesureDrafts(t *testing.T) {
	rows := draftRows("LATTIS_DRAFT_MESURE", "")
	if len(rows) == 0 {
		t.Skip("LATTIS_DRAFT_MESURE=<lignes,…> pour lancer la mesure")
	}
	env := workbooktest.PGSetup(t)
	env.Exec(t, `UPDATE lattis.server_config SET value = '0' WHERE key = 'draft_quota_mb'`)
	role, _ := env.User(t, "mesure")
	store := workbook.PGDrafts(env.Dial(role))
	workRoot(t)
	ctx := context.Background()
	for _, n := range rows {
		s, d := draftBig(t, store, n)
		start := time.Now()
		data, _, err := d.Draft(ctx, false, nil)
		mustShare(t, err)
		snap := time.Since(start)
		id := draftID(d.ID())
		start = time.Now()
		_, err = store.Save(ctx, id, "mesure", data)
		mustShare(t, err)
		up := time.Since(start)
		start = time.Now()
		back, err := store.Read(ctx, id)
		mustShare(t, err)
		rd := time.Since(start)
		start = time.Now()
		o, err := archive.RestoreDraft(back, "")
		mustShare(t, err)
		rest := time.Since(start)
		mustShare(t, archive.Discard(o))
		t.Logf("%d lignes : archive %.1f Mo ; instantané + compression %v ; envoi %v ; lecture %v ; extraction %v", n, float64(len(data))/1e6, snap.Round(time.Millisecond), up.Round(time.Millisecond), rd.Round(time.Millisecond), rest.Round(time.Millisecond))

		base := draftTyping(t, s, 100, 5*time.Millisecond)
		t.Logf("%d lignes : saisie sans envoi : %s", n, draftPct(base))
		// Envois en boucle, comme pendant une saisie continue.
		stop := make(chan struct{})
		done := make(chan int)
		go func() {
			count := 0
			for {
				select {
				case <-stop:
					done <- count
					return
				default:
				}
				data, _, err := d.Draft(ctx, false, nil)
				if err == nil {
					_, err = store.Save(ctx, id, "mesure", data)
				}
				if err != nil {
					t.Error(err)
					done <- count
					return
				}
				count++
			}
		}()
		during := draftTyping(t, s, max(100, 50), 20*time.Millisecond)
		for snap+up > 0 && len(during) < 300 && time.Duration(len(during))*20*time.Millisecond < 2*(snap+up) {
			during = append(during, draftTyping(t, s, 50, 20*time.Millisecond)...)
		}
		close(stop)
		count := <-done
		t.Logf("%d lignes : saisie pendant les envois (%d envois) : %s", n, count, draftPct(during))
		// Pour comparer : l'instantané d'un enregistrement, dans la file d'écriture.
		path := t.TempDir() + "/m.lattis"
		fin := make(chan struct{})
		go func() {
			d.Save(ctx, path)
			close(fin)
		}()
		time.Sleep(10 * time.Millisecond)
		var queued []time.Duration
	loop:
		for {
			select {
			case <-fin:
				break loop
			default:
				queued = append(queued, draftTyping(t, s, 1, 10*time.Millisecond)...)
			}
		}
		t.Logf("%d lignes : saisie pendant un Ctrl+S : %s", n, draftPct(queued))
		mustShare(t, store.Delete(ctx, id))
		s.stopDraft(false)
		s.CloseDocument(ctx)
	}
}

// TestMesureDraftsWAL : volume du WAL du serveur pour une personne qui saisit
// en continu, avec le vrai rythme des envois (LATTIS_PG_TEST=1
// LATTIS_DRAFT_WAL=0,100000 : lignes du classeur ; 0 : petit classeur ;
// LATTIS_DRAFT_WAL_SEC : durée de chaque mesure, 60 s par défaut).
func TestMesureDraftsWAL(t *testing.T) {
	spec := os.Getenv("LATTIS_DRAFT_WAL")
	if spec == "" {
		t.Skip("LATTIS_DRAFT_WAL=<lignes,…> pour lancer la mesure")
	}
	env := workbooktest.PGSetup(t)
	env.Exec(t, `UPDATE lattis.server_config SET value = '0' WHERE key = 'draft_quota_mb'`)
	role, _ := env.User(t, "mesure")
	counted := &draftCounter{DraftStore: workbook.PGDrafts(env.Dial(role))}
	workRoot(t)
	secs, _ := strconv.Atoi(os.Getenv("LATTIS_DRAFT_WAL_SEC"))
	if secs <= 0 {
		secs = 60
	}
	for _, f := range strings.Split(spec, ",") {
		n, _ := strconv.Atoi(strings.TrimSpace(f))
		counted.saves, counted.bytes = 0, 0
		var s *DocumentService
		var d *doc.Document
		if n > 0 {
			s, d = draftBig(t, counted, n)
		} else {
			s = &DocumentService{}
			UseDrafts(s, counted, t.TempDir())
			_, err := s.New(context.Background())
			mustShare(t, err)
			d, err = s.current()
			mustShare(t, err)
		}
		s.attachDraft(d)
		from := env.Text(t, `SELECT pg_current_wal_lsn()::text`)
		// 4 saisies par seconde.
		typed := draftTyping(t, s, secs*4, 250*time.Millisecond)
		env2 := env.Text(t, `SELECT pg_wal_lsn_diff(pg_current_wal_lsn(), $1::pg_lsn)::text`, from)
		wal, _ := strconv.ParseFloat(env2, 64)
		t.Logf("%d lignes : %d s de saisie : %d envois, %.2f Mo envoyés (dernier %.2f Mo), WAL %.1f Mo, soit %.2f Go/h ; saisie %s", n, secs, counted.saves, float64(counted.bytes)/1e6, float64(counted.last)/1e6, wal/1e6, wal/1e9*3600/float64(secs), draftPct(typed))
		s.DraftForget()
		s.CloseDocument(context.Background())
	}
}

type draftCounter struct {
	DraftStore
	saves       int
	bytes, last int64
}

func (c *draftCounter) Save(ctx context.Context, id, title string, data []byte) (time.Time, error) {
	c.saves++
	c.bytes += int64(len(data))
	c.last = int64(len(data))
	return c.DraftStore.Save(ctx, id, title, data)
}
