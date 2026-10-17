// Command sqlite-drivers compare les trois drivers SQLite Go envisagés pour
// Lattis sur les opérations dont dépend la grille : chargement en masse,
// lecture de fenêtres de lignes par clé d'ordre, édition d'une cellule et appel
// d'une fonction SQL écrite en Go.
//
// Un driver par binaire, choisi par build tag (mattn, modernc, ncruces) :
//
//	go run -tags modernc . -rows 1000000
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const cols = 20

func main() {
	rows := flag.Int("rows", 1_000_000, "nombre de lignes")
	windows := flag.Int("windows", 1000, "nombre de fenêtres lues")
	window := flag.Int("window", 200, "lignes par fenêtre")
	updates := flag.Int("updates", 1000, "nombre d'éditions de cellule")
	dir := flag.String("dir", os.TempDir(), "répertoire de la base de test")
	flag.Parse()

	path := filepath.Join(*dir, fmt.Sprintf("bench-%s.sqlite", driverName))
	for _, suffix := range []string{"", "-wal", "-shm"} {
		os.Remove(path + suffix)
	}
	db, err := openDB(path)
	check(err)
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, p := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
	} {
		_, err := db.Exec(p)
		check(err)
	}
	var version string
	check(db.QueryRow("SELECT sqlite_version()").Scan(&version))
	fmt.Printf("driver=%s sqlite=%s rows=%d\n", driverName, version, *rows)

	check(createTable(db))

	start := time.Now()
	check(load(db, *rows))
	report("chargement", time.Since(start))

	start = time.Now()
	_, err = db.Exec(`CREATE INDEX t_pos ON t(_pos)`)
	check(err)
	report("index _pos", time.Since(start))

	start = time.Now()
	anchors, err := buildAnchors(db, 1000)
	check(err)
	report(fmt.Sprintf("ancres (%d)", len(anchors)), time.Since(start))

	readWindows(db, anchors, *rows, *windows, *window)
	editCells(db, *rows, *updates)

	start = time.Now()
	var sum float64
	check(db.QueryRow(`SELECT total(slt_div(c5, c0)) FROM t`).Scan(&sum))
	report("slt_div sur toute la table", time.Since(start))

	start = time.Now()
	var n int
	check(db.QueryRow(`SELECT count(*) FROM t WHERE c10 LIKE 'nom 12%'`).Scan(&n))
	report("scan LIKE sur toute la table", time.Since(start))

	st, err := os.Stat(path)
	check(err)
	fmt.Printf("%-34s %8.1f Mo\n", "taille du fichier", float64(st.Size())/1e6)
}

func createTable(db *sql.DB) error {
	var b strings.Builder
	b.WriteString("CREATE TABLE t (id INTEGER PRIMARY KEY, _pos TEXT NOT NULL")
	for c := range cols {
		fmt.Fprintf(&b, ", c%d %s", c, colType(c))
	}
	b.WriteString(") STRICT")
	_, err := db.Exec(b.String())
	return err
}

// colType donne un mélange réaliste : entiers, réels, textes, booléen, date.
func colType(c int) string {
	switch {
	case c < 5:
		return "INTEGER"
	case c < 10:
		return "REAL"
	case c == 18:
		return "INTEGER"
	default:
		return "TEXT"
	}
}

func value(c, i int) any {
	switch {
	case c < 5:
		return int64(i*(c+1)) % 100_000
	case c < 10:
		return float64(i%10_000) / float64(c+1)
	case c == 18:
		return int64(i % 2)
	case c == 19:
		return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i%2000).Format("2006-01-02")
	default:
		return fmt.Sprintf("nom %d colonne %d", i, c)
	}
}

// posKey produit des clés d'ordre de largeur fixe, triées comme les entiers,
// avec de la place entre deux clés pour des insertions ultérieures.
func posKey(i int) string {
	const digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	n := uint64(i+1) << 16
	buf := make([]byte, 8)
	for k := 7; k >= 0; k-- {
		buf[k] = digits[n%62]
		n /= 62
	}
	return string(buf)
}

func load(db *sql.DB, rows int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	var names, marks strings.Builder
	for c := range cols {
		fmt.Fprintf(&names, ", c%d", c)
		marks.WriteString(", ?")
	}
	q := "INSERT INTO t (_pos" + names.String() + ") VALUES (?" + marks.String() + ")"
	stmt, err := tx.Prepare(q)
	if err != nil {
		return err
	}
	args := make([]any, cols+1)
	for i := range rows {
		args[0] = posKey(i)
		for c := range cols {
			args[c+1] = value(c, i)
		}
		if _, err := stmt.Exec(args...); err != nil {
			return err
		}
	}
	stmt.Close()
	return tx.Commit()
}

// buildAnchors relève une clé d'ordre toutes les `every` lignes. Pour lire la
// ligne n, on part de l'ancre n/every puis on saute au plus every-1 lignes :
// la pagination reste par clé, avec un OFFSET borné.
func buildAnchors(db *sql.DB, every int) ([]string, error) {
	rs, err := db.Query(`SELECT _pos FROM t ORDER BY _pos`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var anchors []string
	i := 0
	for rs.Next() {
		var p string
		if err := rs.Scan(&p); err != nil {
			return nil, err
		}
		if i%every == 0 {
			anchors = append(anchors, p)
		}
		i++
	}
	return anchors, rs.Err()
}

func readWindows(db *sql.DB, anchors []string, rows, n, size int) {
	var names strings.Builder
	names.WriteString("id")
	for c := range cols {
		fmt.Fprintf(&names, ", c%d", c)
	}
	stmt, err := db.Prepare(`SELECT ` + names.String() + ` FROM t WHERE _pos >= ? ORDER BY _pos LIMIT ? OFFSET ?`)
	check(err)
	defer stmt.Close()

	var query, encode []time.Duration
	var bytes int
	for range n {
		first := rand.IntN(rows - size)
		start := time.Now()
		rs, err := stmt.Query(anchors[first/1000], size, first%1000)
		check(err)
		out := make([][]any, 0, size)
		for rs.Next() {
			row := make([]any, cols+1)
			ptrs := make([]any, cols+1)
			for k := range row {
				ptrs[k] = &row[k]
			}
			check(rs.Scan(ptrs...))
			out = append(out, row)
		}
		check(rs.Err())
		rs.Close()
		query = append(query, time.Since(start))

		start = time.Now()
		b, err := json.Marshal(out)
		check(err)
		encode = append(encode, time.Since(start))
		bytes = len(b)
	}
	reportDist(fmt.Sprintf("fenêtre de %d lignes", size), query)
	reportDist("encodage JSON de la fenêtre", encode)
	fmt.Printf("%-34s %8.1f Ko\n", "taille JSON d'une fenêtre", float64(bytes)/1e3)
}

func editCells(db *sql.DB, rows, n int) {
	var d []time.Duration
	for i := range n {
		id := rand.IntN(rows) + 1
		start := time.Now()
		_, err := db.Exec(`UPDATE t SET c10 = ? WHERE id = ?`, fmt.Sprintf("édité %d", i), id)
		check(err)
		d = append(d, time.Since(start))
	}
	reportDist("édition d'une cellule (commit)", d)
}

func report(name string, d time.Duration) {
	fmt.Printf("%-34s %8.1f ms\n", name, float64(d.Microseconds())/1000)
}

func reportDist(name string, d []time.Duration) {
	slices.Sort(d)
	p := func(q float64) float64 { return float64(d[int(q*float64(len(d)-1))].Microseconds()) / 1000 }
	fmt.Printf("%-34s p50 %6.2f ms  p95 %6.2f ms  max %6.2f ms\n", name, p(0.5), p(0.95), p(1))
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "erreur:", err)
		os.Exit(1)
	}
}

// sltDiv reproduit la division d'Excel : toujours réelle, NULL (futur #DIV/0!)
// sur une division par zéro.
func sltDiv(a, b float64) (float64, bool) {
	if b == 0 {
		return 0, false
	}
	return a / b, true
}
