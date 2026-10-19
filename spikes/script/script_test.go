package script

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lalamefine/lattis/internal/store"
)

func openDB(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	err = db.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, nom TEXT, n INTEGER) STRICT;
			INSERT INTO t (nom, n) VALUES ('a', 1), ('b', 2), ('c', 3)`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// gridEdits simule la grille : des saisies en continu pendant que le script
// tourne. Renvoie la pire latence observée.
func gridEdits(t *testing.T, db *store.Store, stop <-chan struct{}) (worst time.Duration, n int) {
	for {
		select {
		case <-stop:
			return
		default:
		}
		t0 := time.Now()
		err := db.Write(context.Background(), func(tx *sql.Tx) error {
			_, err := tx.Exec(`UPDATE t SET n = n + 1 WHERE id = 1`)
			return err
		})
		if err != nil {
			t.Error(err)
			return
		}
		worst = max(worst, time.Since(t0))
		n++
		time.Sleep(5 * time.Millisecond)
	}
}

func runWithGrid(t *testing.T, db *store.Store, ctx context.Context, src string, lim Limits) (Result, time.Duration, int) {
	stop := make(chan struct{})
	var worst time.Duration
	var n int
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); worst, n = gridEdits(t, db, stop) }()
	res := Run(ctx, db, src, lim)
	close(stop)
	wg.Wait()
	return res, worst, n
}

// Critère de sortie : une boucle infinie est interrompue sans geler la grille.
func TestInfiniteLoopTimeout(t *testing.T) {
	db := openDB(t)
	lim := DefaultLimits
	lim.Timeout = 500 * time.Millisecond
	res, worst, n := runWithGrid(t, db, context.Background(), `while true do end`, lim)
	if !errors.Is(res.Err, ErrTimeout) {
		t.Fatalf("erreur %v", res.Err)
	}
	if res.Duration > lim.Timeout+200*time.Millisecond {
		t.Fatalf("arrêt tardif : %v", res.Duration)
	}
	t.Logf("arrêt après %v ; grille : %d saisies, pire %v", res.Duration.Round(time.Millisecond), n, worst)
	if worst > 50*time.Millisecond {
		t.Fatalf("la grille a attendu %v", worst)
	}
}

func TestStopButton(t *testing.T) {
	db := openDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	res := Run(ctx, db, `local i = 0 while true do i = i + 1 end`, DefaultLimits)
	if !errors.Is(res.Err, ErrStopped) || res.Duration > 400*time.Millisecond {
		t.Fatalf("%v après %v", res.Err, res.Duration)
	}
}

func TestMemoryWatchdog(t *testing.T) {
	db := openDB(t)
	lim := DefaultLimits
	lim.MaxHeap = 64 << 20
	res := Run(context.Background(), db, `local t = {} while true do t[#t+1] = string.rep("x", 1000) .. #t end`, lim)
	if !errors.Is(res.Err, ErrMemory) {
		t.Fatalf("erreur %v", res.Err)
	}
	t.Logf("arrêté après %v", res.Duration.Round(time.Millisecond))
}

func TestSandbox(t *testing.T) {
	db := openDB(t)
	res := Run(context.Background(), db, `
		print(os, io, require, dofile, load, loadstring, debug, string.dump)
		print(math.floor(2.5), string.upper("é"), table.concat({1, 2}, "-"))`, DefaultLimits)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if res.Output[0] != strings.Repeat("nil\t", 7)+"nil" {
		t.Fatalf("bibliothèques dangereuses accessibles : %q", res.Output[0])
	}
	for _, src := range []string{`os.execute("x")`, `io.open("x")`, `require("os")`, `dofile("x")`} {
		if r := Run(context.Background(), db, src, DefaultLimits); r.Err == nil {
			t.Errorf("%s accepté", src)
		}
	}
}

func TestDBAPI(t *testing.T) {
	db := openDB(t)
	res := Run(context.Background(), db, `
		local rows = db.query("SELECT nom, n FROM t WHERE n >= ? ORDER BY n", 2)
		for _, r in ipairs(rows) do print(r.nom, r.n) end
		local total = 0
		for i = 1, 100 do total = total + db.execute("UPDATE t SET n = n + 1 WHERE nom = ?", "c") end
		print(total, db.query("SELECT n FROM t WHERE nom = 'c'")[1].n)`, DefaultLimits)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if got := strings.Join(res.Output, "|"); got != "b\t2|c\t3|100\t103" || res.Writes != 100 {
		t.Fatalf("sortie %q, %d écritures", got, res.Writes)
	}
	// Lecture seule : db.query refuse d'écrire.
	if r := Run(context.Background(), db, `db.query("DELETE FROM t")`, DefaultLimits); r.Err == nil {
		t.Fatal("db.query a écrit")
	}
	lim := DefaultLimits
	lim.MaxRows = 2
	if r := Run(context.Background(), db, `db.query("SELECT * FROM t")`, lim); r.Err == nil || !strings.Contains(r.Err.Error(), "LIMIT") {
		t.Fatalf("limite de lignes : %v", r.Err)
	}
}

// Un script qui écrit beaucoup ne bloque pas la grille : ses écritures et
// celles de la grille s'intercalent dans la file.
func TestWritingScriptKeepsGridResponsive(t *testing.T) {
	db := openDB(t)
	res, worst, n := runWithGrid(t, db, context.Background(),
		`for i = 1, 2000 do db.execute("INSERT INTO t (nom, n) VALUES (?, ?)", "s", i) end`, DefaultLimits)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	t.Logf("script : %d écritures en %v ; grille : %d saisies, pire %v", res.Writes, res.Duration.Round(time.Millisecond), n, worst)
	if worst > 100*time.Millisecond {
		t.Fatalf("la grille a attendu %v", worst)
	}
}
