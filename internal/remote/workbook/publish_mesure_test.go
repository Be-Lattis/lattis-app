package workbook_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/remote/workbook"
	"github.com/lalamefine/lattis/internal/work"
)

// TestMesurePublication chronomètre la publication d'un gros tableau
// (10 colonnes, LATTIS_MESURE lignes) sur un vrai PostgreSQL, étape par
// étape, dans les deux modes
// (docs/audits/publication-gros-tableau-2026-10-22.md) :
//
//	LATTIS_PG_TEST=1 LATTIS_TEST_PG_URL=… LATTIS_SERVER_SCHEMA=… \
//	LATTIS_MESURE=1000000 go test ./internal/remote/workbook -run TestMesurePublication -v -timeout 0
//
// LATTIS_MESURE_MODE : frozen ou collaborative (les deux par défaut).
func TestMesurePublication(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("LATTIS_MESURE"))
	if n == 0 {
		t.Skip("LATTIS_MESURE non défini")
	}
	env := pgSetup(t)
	useRoot(t)
	modes := []string{workbook.Frozen, workbook.Collaborative}
	if m := os.Getenv("LATTIS_MESURE_MODE"); m != "" {
		modes = []string{m}
	}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			alice, _ := env.user(t, "alice")
			d := newDoc(t)
			start := time.Now()
			bigTable(t, d, n)
			t.Logf("import %d × 10 : %v", n, time.Since(start))

			start = time.Now()
			check, err := workbook.Inspect(ctx, d.DB())
			must(t, err)
			t.Logf("%-40s %8.2f s", "vérifications (ShareCheck)", time.Since(start).Seconds())
			id := check.Tables[0].ID

			// Étapes dites par la publication (internal/work), toutes
			// émises : chacune est datée à son début.
			oldDelay, oldEvery := work.Delay, work.Every
			work.Delay, work.Every = 0, 0
			defer func() { work.Delay, work.Every = oldDelay, oldEvery }()
			var mu sync.Mutex
			var last string
			var lastAt time.Time
			var lastDone int64
			t0 := time.Now()
			step := func(s work.Step) {
				mu.Lock()
				defer mu.Unlock()
				if s.Phase == last && !s.End {
					lastDone = s.Done
					return
				}
				if last != "" {
					t.Logf("  %-38s %8.2f s %s", last, time.Since(lastAt).Seconds(), doneText(lastDone))
				}
				last, lastAt, lastDone = s.Phase, time.Now(), s.Done
			}
			wctx, tr := work.Start(ctx, step, true)
			srv := &timedServer{Server: env.as(alice)}
			info, err := workbook.Publish(wctx, srv, d.DB(), workbook.Request{Server: server, User: alice, Title: "Gros " + mode, Mode: mode, Tables: []int64{id}})
			tr.End()
			must(t, err)
			t.Logf("%-40s %8.2f s", "publication (Publish)", time.Since(t0).Seconds())
			t.Logf("  COPY : %.2f s dont %.2f s à lire les lignes du classeur (poste)", srv.copy.Seconds(), srv.read.Seconds())
			start = time.Now()
			must(t, d.ShareChanged(ctx))
			t.Logf("%-40s %8.2f s", "état relu (ShareChanged)", time.Since(start).Seconds())
			start = time.Now()
			must(t, d.Save(ctx, t.TempDir()+"/gros.lattis"))
			t.Logf("%-40s %8.2f s", "enregistrement du fichier (Save)", time.Since(start).Seconds())
			var count int64
			env.Exec(t, "SELECT 1")
			c := env.Text(t, fmt.Sprintf(`SELECT count(*)::text FROM %q."Gros"`, info.Schema))
			count, _ = strconv.ParseInt(c, 10, 64)
			if count != int64(n) {
				t.Fatalf("%d lignes sur le serveur, %d attendues", count, n)
			}
		})
	}
}

func doneText(n int64) string {
	if n == 0 {
		return ""
	}
	return "(" + strconv.FormatInt(n, 10) + ")"
}

// bigTable importe un tableau « Gros » de n lignes × 10 colonnes (entiers
// et textes, comme TestMesureVue).
func bigTable(t *testing.T, d *doc.Document, n int) {
	t.Helper()
	headers := make([]string, 10)
	types := make([]grid.Type, 10)
	for i := range headers {
		headers[i] = fmt.Sprintf("Col%d", i+1)
		types[i] = grid.Text
		if i%2 == 1 {
			types[i] = grid.Integer
		}
	}
	types[0] = grid.Integer
	_, err := d.Import(context.Background(), "Importer", []canvas.ImportSheet{{Name: "Données", Tables: []canvas.ImportTable{{
		Name: "Gros", Headers: headers, Types: types, Count: n,
		Rows: func(yield func([]any) error) error {
			row := make([]any, 10)
			for i := range n {
				for c := range row {
					if types[c] == grid.Integer {
						row[c] = int64(i*(c+1)) % 100003
					} else {
						row[c] = "Texte " + strconv.Itoa(i%7919+c)
					}
				}
				row[0] = int64(i + 1)
				if err := yield(row); err != nil {
					return err
				}
			}
			return nil
		},
	}}}})
	must(t, err)
}

// timedServer chronomètre la copie (COPY) : temps total et temps passé à
// lire les lignes du classeur (Next, côté poste) ; le reste est l'envoi et
// le travail du serveur.
type timedServer struct {
	workbook.Server
	copy, read time.Duration
}

func (s *timedServer) Publish(ctx context.Context, fn func(workbook.Publisher) error) error {
	return s.Server.Publish(ctx, func(p workbook.Publisher) error { return fn(timedPublisher{p, s}) })
}

type timedPublisher struct {
	workbook.Publisher
	s *timedServer
}

func (p timedPublisher) Copy(ctx context.Context, schema, table string, cols []string, rows workbook.CopySource) (int64, error) {
	start := time.Now()
	n, err := p.Publisher.Copy(ctx, schema, table, cols, &timedRows{rows, p.s})
	p.s.copy += time.Since(start)
	return n, err
}

type timedRows struct {
	workbook.CopySource
	s *timedServer
}

func (r *timedRows) Next() bool {
	start := time.Now()
	ok := r.CopySource.Next()
	r.s.read += time.Since(start)
	return ok
}
