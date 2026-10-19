package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lalamefine/lattis/internal/trace"
	"github.com/ncruces/go-sqlite3"
	sqlite "github.com/ncruces/go-sqlite3/driver"
)

// Journal SQL : chaque requête jouée contre le SQLite local (classeur, base
// de travail de Power Query, fichier SQLite source) est écrite dans le
// journal de débogage (trace : sortie d'erreur et --log-file), avec sa durée
// et la connexion. Actif avec -vv ou --sql, jamais par
// défaut (même en développement) ; LATTIS_SQL=1 ou 0 force ; --log sans
// sql le coupe. À -vvv, les paramètres des requêtes préparées suivent la requête.
// Une requête répétée à l'identique (insertion ligne à ligne d'un import)
// n'est écrite qu'une fois, suivie du nombre de répétitions.
//
// Le journal enveloppe le pilote (database/sql) : le traçage de SQLite ne
// voit pas les requêtes sans paramètre que le pilote exécute directement.

// defaultSQLLog : journal SQL coupé sans option.
const defaultSQLLog = false

var sqlLog atomic.Bool

func init() {
	sqlLog.Store(defaultSQLLog)
	switch os.Getenv("LATTIS_SQL") {
	case "1", "true", "oui":
		sqlLog.Store(true)
	case "0", "false", "non":
		sqlLog.Store(false)
	}
}

// SetSQLLog active ou coupe le journal SQL (option --sql).
func SetSQLLog(on bool) { sqlLog.Store(on) }

// SQLLog dit si le journal SQL est actif.
func SQLLog() bool { return sqlLog.Load() }

// DefaultSQLLog dit si le journal SQL est actif sans option : seulement
// imposé par LATTIS_SQL.
func DefaultSQLLog() bool {
	switch os.Getenv("LATTIS_SQL") {
	case "1", "true", "oui":
		return true
	case "0", "false", "non":
		return false
	}
	return defaultSQLLog
}

func logging() bool { return sqlLog.Load() && trace.Allowed(trace.SQL) }

// maxLogSQL borne la longueur d'une requête écrite.
const maxLogSQL = 4000

var logMu sync.Mutex
var last struct {
	sql    string
	kind   string
	repeat int
	total  time.Duration
}

// stamp : l'heure, comme les autres lignes du journal de débogage.
func stamp() string { return time.Now().Format("15:04:05.000") }

func flushRepeat() {
	if last.repeat > 0 {
		fmt.Fprintf(trace.Writer(), "%s [SQL %s]   … répétée %d fois de plus (%s au total)\n", stamp(), last.kind, last.repeat, fmtDur(last.total))
	}
	last.repeat, last.total = 0, 0
}

func fmtDur(d time.Duration) string { return trace.Dur(d) }

// fmtArgs écrit les paramètres d'une requête (-vvv), bornés.
func fmtArgs(args []driver.NamedValue) string {
	if len(args) == 0 || trace.Level() < 3 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n  paramètres :")
	for i, a := range args {
		if i == 20 {
			fmt.Fprintf(&b, " … (%d en tout)", len(args))
			break
		}
		v := fmt.Sprintf("%v", a.Value)
		if s, ok := a.Value.(string); ok {
			if len(s) > 80 {
				s = s[:80] + "…"
			}
			v = fmt.Sprintf("%q", s)
		}
		if a.Name != "" {
			fmt.Fprintf(&b, " :%s=%s", a.Name, v)
		} else {
			fmt.Fprintf(&b, " ?%d=%s", a.Ordinal, v)
		}
	}
	return b.String()
}

// logSQL écrit une requête jouée (kind : connexion), avec sa durée.
func logSQL(kind, q string, d time.Duration, err error) { logSQLArgs(kind, q, nil, d, err) }

// logSQLArgs : logSQL, avec les paramètres de la requête (écrits à -vvv).
func logSQLArgs(kind, q string, args []driver.NamedValue, d time.Duration, err error) {
	if !logging() {
		return
	}
	params := fmtArgs(args)
	q = strings.TrimSpace(q)
	logMu.Lock()
	defer logMu.Unlock()
	if q == last.sql && kind == last.kind && err == nil && params == "" {
		last.repeat++
		last.total += d
		return
	}
	flushRepeat()
	last.sql, last.kind = q, kind
	// Sur une ligne : les requêtes écrites sur plusieurs lignes restent lisibles.
	shown := strings.Join(strings.Fields(q), " ")
	if len(shown) > maxLogSQL {
		shown = shown[:maxLogSQL] + fmt.Sprintf(" … (%d caractères)", len(shown))
	}
	if err != nil {
		fmt.Fprintf(trace.Writer(), "%s [SQL %s %s] %s%s\n  ERREUR : %v\n", stamp(), kind, fmtDur(d), shown, params, err)
		return
	}
	fmt.Fprintf(trace.Writer(), "%s [SQL %s %s] %s%s\n", stamp(), kind, fmtDur(d), shown, params)
}

// logSQLRows écrit une lecture terminée : durée jusqu'à la dernière ligne et
// nombre de lignes lues.
func logSQLRows(kind, q string, args []driver.NamedValue, d time.Duration, rows int) {
	logSQLArgs(kind+fmt.Sprintf(" %d ligne(s)", rows), q, args, d, nil)
}

// OpenDB ouvre une base SQLite (dsn du pilote) dont les requêtes passent par
// le journal SQL ; kind nomme la connexion dans le journal ; init configure
// chaque nouvelle connexion (pragmas, fonctions).
func OpenDB(dsn, kind string, init func(*sqlite3.Conn) error) (*sql.DB, error) {
	return openDB(dsn, kind, init, nil)
}

// openDB : OpenDB, et onWrite reçoit chaque requête exécutée (suivi des
// tableaux modifiés, voir Store.TakeTouched).
func openDB(dsn, kind string, init func(*sqlite3.Conn) error, onWrite func(string)) (*sql.DB, error) {
	inner, err := (&sqlite.SQLite{}).OpenConnector(dsn)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(&logConnector{inner: inner, kind: kind, init: init, onWrite: onWrite}), nil
}

type logConnector struct {
	inner   driver.Connector
	kind    string
	init    func(*sqlite3.Conn) error
	onWrite func(string)
}

func (c *logConnector) Driver() driver.Driver { return c.inner.Driver() }

func (c *logConnector) Connect(ctx context.Context) (driver.Conn, error) {
	cn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	sc, ok := cn.(sqlite.Conn)
	if !ok {
		cn.Close()
		return nil, fmt.Errorf("store : connexion SQLite inattendue (%T)", cn)
	}
	if c.init != nil {
		if err := c.init(sc.Raw()); err != nil {
			cn.Close()
			return nil, err
		}
	}
	return &logConn{Conn: sc, kind: c.kind, onWrite: c.onWrite}, nil
}

// logConn enveloppe une connexion du pilote : les exécutions directes et
// les requêtes préparées passent par le journal.
type logConn struct {
	sqlite.Conn
	kind    string
	onWrite func(string)
}

func (c *logConn) Raw() *sqlite3.Conn { return c.Conn.Raw() }

func (c *logConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	ex, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	start := clock()
	r, err := ex.ExecContext(ctx, q, args)
	if err == driver.ErrSkip { // requête avec paramètres : préparée, journalisée par logStmt
		return nil, err
	}
	logSQL(c.kind, q, since(start), err)
	if c.onWrite != nil {
		c.onWrite(q)
	}
	return r, err
}

func (c *logConn) Prepare(q string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), q)
}

func (c *logConn) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	s, err := c.Conn.PrepareContext(ctx, q)
	if err != nil {
		return nil, err
	}
	return &logStmt{Stmt: s, q: q, kind: c.kind, onWrite: c.onWrite}, nil
}

type logStmt struct {
	driver.Stmt
	q       string
	kind    string
	onWrite func(string)
}

func (s *logStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	start := clock()
	r, err := s.Stmt.(driver.StmtExecContext).ExecContext(ctx, args)
	logSQLArgs(s.kind, s.q, args, since(start), err)
	if s.onWrite != nil {
		s.onWrite(s.q)
	}
	return r, err
}

func (s *logStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	start := clock()
	r, err := s.Stmt.(driver.StmtQueryContext).QueryContext(ctx, args)
	if err != nil || !logging() {
		logSQLArgs(s.kind, s.q, args, since(start), err)
		return r, err
	}
	// Lecture : journalisée à sa fin (Close), durée et nombre de lignes compris.
	return &logRows{Rows: r, start: start, kind: s.kind, q: s.q, args: args}, nil
}

// logRows mesure une lecture jusqu'à sa dernière ligne.
type logRows struct {
	driver.Rows
	start int64
	kind  string
	q     string
	args  []driver.NamedValue
	n     int
	done  bool
}

func (r *logRows) Next(dest []driver.Value) error {
	err := r.Rows.Next(dest)
	if err == nil {
		r.n++
	}
	return err
}

func (r *logRows) Close() error {
	err := r.Rows.Close()
	if !r.done {
		r.done = true
		logSQLRows(r.kind, r.q, r.args, since(r.start), r.n)
	}
	return err
}

func (r *logRows) ColumnTypeDatabaseTypeName(i int) string {
	if c, ok := r.Rows.(driver.RowsColumnTypeDatabaseTypeName); ok {
		return c.ColumnTypeDatabaseTypeName(i)
	}
	return ""
}

func (r *logRows) ColumnTypeNullable(i int) (nullable, ok bool) {
	if c, ok := r.Rows.(driver.RowsColumnTypeNullable); ok {
		return c.ColumnTypeNullable(i)
	}
	return false, false
}

func (r *logRows) ColumnTypeScanType(i int) reflect.Type {
	if c, ok := r.Rows.(driver.RowsColumnTypeScanType); ok {
		return c.ColumnTypeScanType(i)
	}
	return reflect.TypeFor[any]()
}

func (s *logStmt) CheckNamedValue(arg *driver.NamedValue) error {
	if ck, ok := s.Stmt.(driver.NamedValueChecker); ok {
		return ck.CheckNamedValue(arg)
	}
	return driver.ErrSkip
}

func (c *logConn) CheckNamedValue(arg *driver.NamedValue) error {
	if ck, ok := c.Conn.(driver.NamedValueChecker); ok {
		return ck.CheckNamedValue(arg)
	}
	return driver.ErrSkip
}
