package collab_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lalamefine/lattis/internal/remote/collab"
)

// TestCharge : LATTIS_CHARGE=N postes suivent le même classeur
// collaboratif sur un vrai lattisd (LATTIS_LATTISD_URL…, voir
// lattisd_test.go), comme l'application : pgpool, relais /api/events (ou
// LISTEN avec LATTIS_CHARGE_LISTEN=1). Mesures, dans le journal de
// l'essai :
//
//   - propagation : une saisie d'un poste, visible chez tous les autres
//     (médiane, 95e centile, pire), regroupement de collab.Debounce compris ;
//   - rafale : W postes saisissent en même temps, délai jusqu'à ce que tous
//     les postes aient tout ;
//   - connexions PostgreSQL des postes (pg_stat_activity), transactions par
//     saisie propagée, mémoire de lattisd (LATTIS_CHARGE_METRICS : adresse
//     de /metrics) et de l'application par poste.
//
// Essai long (une à quelques minutes) : jamais lancé sans LATTIS_CHARGE.
func TestCharge(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("LATTIS_CHARGE"))
	if n <= 0 {
		t.Skip("LATTIS_CHARGE non défini : essai de charge ignoré")
	}
	e := ldSetup(t)
	listen := os.Getenv("LATTIS_CHARGE_LISTEN") == "1"
	edits, burst := 20, min(30, n)
	ctx, cancel := context.WithCancel(context.Background())

	ar, ai := e.user(t, "alice")
	alice := e.station(t, "alice", ar, ai, stationOpts{listen: listen})
	wb := ldPublish(t, e, alice, burst+1)

	// Comptes et membres en une connexion.
	type acct struct{ role, id string }
	accts := make([]acct, n)
	func() {
		c, err := pgx.Connect(ctx, e.svc)
		must(t, err)
		defer c.Close(ctx)
		for i := range accts {
			email := fmt.Sprintf("poste%d.%d@example.test", i, time.Now().UnixNano())
			must(t, c.QueryRow(ctx, `SELECT pg_role, id::text FROM lattis.create_user($1, $2, NULL, NULL)`, email, fmt.Sprint("poste ", i)).Scan(&accts[i].role, &accts[i].id))
			_, err = c.Exec(ctx, `SELECT lattis.activate_user($1::uuid), lattis.add_member($2::uuid, 'editor', $1::uuid)`, accts[i].id, wb)
			must(t, err)
		}
	}()

	heap0, rss0 := memNow()
	// Arrivées : edit → poste → instant où la valeur attendue est vue.
	var mu sync.Mutex
	want := map[string]int{} // valeur attendue → numéro de saisie (Quantité, ligne 0)
	seen := make([]map[int]time.Time, n)
	burstWant := map[int]string{} // ligne → valeur de la rafale
	burstSeen := make([]map[int]bool, n)
	burstDone := make([]time.Time, n)
	stations := make([]*station, n)
	opened := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < 16; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range opened {
				s := e.station(t, fmt.Sprint("poste ", i), accts[i].role, accts[i].id, stationOpts{listen: listen})
				s.open(t, e, wb)
				stations[i] = s
			}
		}()
	}
	start := time.Now()
	for i := range n {
		opened <- i
	}
	close(opened)
	wg.Wait()
	t.Logf("%d postes ouverts en %v", n, time.Since(start).Round(time.Millisecond))

	for i, s := range stations {
		seen[i], burstSeen[i] = map[int]time.Time{}, map[int]bool{}
		s.follow(ctx, s.fsrv, time.Second, func(ev collab.Event) {
			if !ev.Data {
				return
			}
			now := time.Now()
			q := s.col("Quantité")
			mu.Lock()
			defer mu.Unlock()
			if k, ok := want[q[0]]; ok {
				if _, done := seen[i][k]; !done {
					seen[i][k] = now
				}
			}
			for row, v := range burstWant {
				if row < len(q) && q[row] == v {
					burstSeen[i][row] = true
				}
			}
			if len(burstWant) > 0 && len(burstSeen[i]) == len(burstWant) && burstDone[i].IsZero() {
				burstDone[i] = now
			}
		})
	}
	alice.follow(ctx, alice.fsrv, time.Second, nil)
	t.Cleanup(cancel) // après les follow : le suivi s'arrête avant leurs attentes
	start = time.Now()
	for _, s := range stations {
		waitOnline(t, s.client)
	}
	waitOnline(t, alice.client)
	t.Logf("tous en ligne en %v", time.Since(start).Round(time.Millisecond))
	time.Sleep(2 * time.Second) // fin des premiers tours
	runtime.GC()
	heap1, rss1 := memNow()
	conns := e.pgConns(t)
	lattisd := metrics(os.Getenv("LATTIS_CHARGE_METRICS"))
	mode := "relais /api/events"
	if listen {
		mode = "LISTEN"
	}
	t.Logf("mode %s ; connexions PostgreSQL des postes après ouverture : %s", mode, conns)
	t.Logf("mémoire de l'essai : tas Go +%.1f Mo (%.0f Ko par poste), processus +%.1f Mo (%.0f Ko par poste)",
		mb(heap1-heap0), kb(heap1-heap0)/float64(n), mb(rss1-rss0), kb(rss1-rss0)/float64(n))
	t.Logf("lattisd : %s", lattisd)

	// 1. Propagation d'une saisie d'Alice à tous les postes.
	var lat []time.Duration
	var lastPer []time.Duration
	xact0 := e.xacts(t)
	for k := range edits {
		v := strconv.Itoa(1000 + k)
		mu.Lock()
		want[v] = k
		mu.Unlock()
		t0 := time.Now()
		alice.mustSet(0, 1, v)
		alice.e.Kick()
		waitFor(t, "saisie "+v+" chez tous", func() bool {
			mu.Lock()
			defer mu.Unlock()
			for i := range seen {
				if _, ok := seen[i][k]; !ok {
					return false
				}
			}
			return true
		})
		var last time.Duration
		mu.Lock()
		for i := range seen {
			d := seen[i][k].Sub(t0)
			lat = append(lat, d)
			last = max(last, d)
		}
		mu.Unlock()
		lastPer = append(lastPer, last)
		time.Sleep(300 * time.Millisecond)
	}
	xacts := e.xacts(t) - xact0
	t.Logf("propagation (%d saisies × %d postes) : médiane %v, 95e centile %v, pire %v ; dernier poste servi : médiane %v, pire %v",
		edits, n, pct(lat, 50), pct(lat, 95), pct(lat, 100), pct(lastPer, 50), pct(lastPer, 100))
	t.Logf("transactions PostgreSQL : %d pour %d saisies (%.1f par saisie, %.2f par poste et par saisie)",
		xacts, edits, float64(xacts)/float64(edits), float64(xacts)/float64(edits*n))

	// 2. Rafale : burst postes saisissent chacun une ligne en même temps.
	mu.Lock()
	for r := 1; r <= burst; r++ {
		burstWant[r] = strconv.Itoa(5000 + r)
	}
	mu.Unlock()
	t0 := time.Now()
	for r := 1; r <= burst; r++ {
		go func(s *station, r int) {
			s.mustSet(r, 1, strconv.Itoa(5000+r))
			s.e.Kick()
		}(stations[r-1], r)
	}
	waitFor(t, "rafale chez tous", func() bool {
		mu.Lock()
		defer mu.Unlock()
		for i := range burstDone {
			if burstDone[i].IsZero() && len(burstSeen[i]) < burst {
				return false
			}
		}
		return true
	})
	var bl []time.Duration
	mu.Lock()
	for i := range burstDone {
		if !burstDone[i].IsZero() {
			bl = append(bl, burstDone[i].Sub(t0))
		}
	}
	mu.Unlock()
	t.Logf("rafale de %d saisies simultanées : tout reçu partout en %v (médiane par poste %v, 95e centile %v)",
		burst, time.Since(t0).Round(time.Millisecond), pct(bl, 50), pct(bl, 95))

	// 3. Au repos : connexions gardées par les postes.
	time.Sleep(5 * time.Second)
	t.Logf("connexions PostgreSQL des postes au repos (5 s) : %s", e.pgConns(t))
	t.Logf("lattisd après les mesures : %s", metrics(os.Getenv("LATTIS_CHARGE_METRICS")))
}

// pgConns : connexions des postes (application_name Lattis/charge/…) par
// état, et total de la base.
func (e *ldEnv) pgConns(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, e.svc)
	must(t, err)
	defer c.Close(ctx)
	rows, err := c.Query(ctx, `SELECT coalesce(state, '?'), count(*) FROM pg_stat_activity
		WHERE datname = current_database() AND application_name LIKE 'Lattis/charge/%' GROUP BY 1 ORDER BY 1`)
	must(t, err)
	var parts []string
	total := 0
	for rows.Next() {
		var s string
		var k int
		must(t, rows.Scan(&s, &k))
		parts = append(parts, fmt.Sprintf("%s %d", s, k))
		total += k
	}
	must(t, rows.Err())
	// lattisd : pools par utilisateur (lattisd/admin : droits vérifiés à
	// l'abonnement au relais), service et écoute.
	var lattisd, all int
	must(t, c.QueryRow(ctx, `SELECT count(*) FILTER (WHERE application_name = 'lattisd/admin'), count(*) FROM pg_stat_activity WHERE datname = current_database()`).Scan(&lattisd, &all))
	return fmt.Sprintf("%d (%s) ; lattisd par utilisateur %d ; %d sur la base", total, strings.Join(parts, ", "), lattisd, all)
}

// xacts : transactions validées de la base (pg_stat_database).
func (e *ldEnv) xacts(t *testing.T) int64 {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, e.svc)
	must(t, err)
	defer c.Close(ctx)
	// Statistiques cumulées : vues après le prochain envoi des compteurs.
	_, _ = c.Exec(ctx, `SELECT pg_stat_force_next_flush()`)
	time.Sleep(1100 * time.Millisecond)
	var x int64
	must(t, c.QueryRow(ctx, `SELECT xact_commit + xact_rollback FROM pg_stat_database WHERE datname = current_database()`).Scan(&x))
	return x
}

// metrics résume la mémoire et les goroutines de lattisd (/metrics).
func metrics(url string) string {
	if url == "" {
		return "LATTIS_CHARGE_METRICS non défini"
	}
	resp, err := http.Get(url)
	if err != nil {
		return err.Error()
	}
	defer resp.Body.Close()
	vals := map[string]float64{}
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 {
			if v, err := strconv.ParseFloat(f[1], 64); err == nil {
				vals[f[0]] = v
			}
		}
	}
	io.Copy(io.Discard, resp.Body)
	return fmt.Sprintf("tas %.1f Mo, mémoire du processus %.1f Mo, %d goroutines",
		mb(int64(vals["go_memstats_heap_inuse_bytes"])), mb(int64(vals["process_resident_memory_bytes"])), int(vals["go_goroutines"]))
}

// memNow : tas Go en usage et mémoire du processus (octets).
func memNow() (heap, rss int64) {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.HeapInuse), rssNow()
}

// rssNow : mémoire résidente du processus (Linux : /proc ; Windows :
// tasklist) ; 0 si inconnue.
func rssNow() int64 {
	if b, err := os.ReadFile("/proc/self/status"); err == nil {
		for _, l := range strings.Split(string(b), "\n") {
			if f := strings.Fields(l); len(f) >= 2 && f[0] == "VmRSS:" {
				k, _ := strconv.ParseInt(f[1], 10, 64)
				return k << 10
			}
		}
	}
	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", os.Getpid()), "/FO", "CSV", "/NH").Output()
		if err == nil {
			f := strings.Split(string(out), "\",\"")
			if len(f) >= 5 {
				digits := strings.Map(func(r rune) rune {
					if r >= '0' && r <= '9' {
						return r
					}
					return -1
				}, f[4])
				k, _ := strconv.ParseInt(digits, 10, 64)
				return k << 10
			}
		}
	}
	return 0
}

func mb(b int64) float64 { return float64(b) / (1 << 20) }
func kb(b int64) float64 { return float64(b) / (1 << 10) }

// pct : centile p (100 : pire) d'une série de durées.
func pct(ds []time.Duration, p int) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	s := slices.Clone(ds)
	slices.Sort(s)
	i := (len(s)*p + 99) / 100
	i = max(0, min(len(s)-1, i-1))
	return s[i].Round(time.Millisecond)
}
