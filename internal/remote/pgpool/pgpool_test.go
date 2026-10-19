package pgpool

import (
	"context"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgproto3"
)

// fakePG imite un PostgreSQL 18 configuré en « oauth » (pg_hba) : il
// annonce OAUTHBEARER, vérifie le jeton porteur et retient les paramètres
// de démarrage.
type fakePG struct {
	ln    net.Listener
	token string

	mu       sync.Mutex
	startups []map[string]string
	appNames []string // application_name posés par SET
	devices  []string // lattis.device posés par SET
	tokens   []string
}

func newFakePG(t *testing.T, token string) *fakePG {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakePG{ln: ln, token: token}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c)
		}
	}()
	return f
}

func (f *fakePG) addr() string { return f.ln.Addr().String() }

func (f *fakePG) serve(c net.Conn) {
	defer c.Close()
	b := pgproto3.NewBackend(c, c)
	msg, err := b.ReceiveStartupMessage()
	if err != nil {
		return
	}
	if _, ok := msg.(*pgproto3.SSLRequest); ok {
		c.Write([]byte("N"))
		if msg, err = b.ReceiveStartupMessage(); err != nil {
			return
		}
	}
	sm, ok := msg.(*pgproto3.StartupMessage)
	if !ok {
		return
	}
	f.mu.Lock()
	f.startups = append(f.startups, sm.Parameters)
	f.mu.Unlock()
	b.Send(&pgproto3.AuthenticationSASL{AuthMechanisms: []string{"OAUTHBEARER"}})
	b.Flush()
	b.SetAuthType(pgproto3.AuthTypeSASL)
	m, err := b.Receive()
	if err != nil {
		return
	}
	ir, ok := m.(*pgproto3.SASLInitialResponse)
	if !ok || ir.AuthMechanism != "OAUTHBEARER" {
		return
	}
	// RFC 7628 §3.1 : gs2-header, puis auth=Bearer <jeton>, séparés par ^A.
	data := string(ir.Data)
	tok := ""
	if strings.HasPrefix(data, "n,,\x01") {
		for _, kv := range strings.Split(strings.TrimPrefix(data, "n,,\x01"), "\x01") {
			if v, ok := strings.CutPrefix(kv, "auth=Bearer "); ok {
				tok = v
			}
		}
	}
	f.mu.Lock()
	f.tokens = append(f.tokens, tok)
	f.mu.Unlock()
	if tok != f.token {
		b.Send(&pgproto3.ErrorResponse{Severity: "FATAL", Code: "28000", Message: "OAuth bearer authentication failed"})
		b.Flush()
		return
	}
	b.Send(&pgproto3.AuthenticationOk{})
	b.Send(&pgproto3.ParameterStatus{Name: "application_name", Value: sm.Parameters["application_name"]})
	b.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "18.0"})
	b.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 2}})
	b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	b.Flush()
	for {
		m, err := b.Receive()
		if err != nil {
			return
		}
		switch q := m.(type) {
		case *pgproto3.Query:
			if v, ok := strings.CutPrefix(q.String, "SET application_name = "); ok {
				v = strings.ReplaceAll(strings.Trim(v, "'"), "''", "'")
				f.mu.Lock()
				f.appNames = append(f.appNames, v)
				f.mu.Unlock()
				b.Send(&pgproto3.ParameterStatus{Name: "application_name", Value: v})
				b.Send(&pgproto3.CommandComplete{CommandTag: []byte("SET")})
			} else if v, ok := strings.CutPrefix(q.String, "SET lattis.device = "); ok {
				f.mu.Lock()
				f.devices = append(f.devices, strings.ReplaceAll(strings.Trim(v, "'"), "''", "'"))
				f.mu.Unlock()
				b.Send(&pgproto3.CommandComplete{CommandTag: []byte("SET")})
			} else {
				b.Send(&pgproto3.EmptyQueryResponse{})
			}
			b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
			b.Flush()
		case *pgproto3.Terminate:
			return
		}
	}
}

func target(rw, ro string, tok TokenFunc) Target {
	t := Target{RW: rw, Database: "lattis", SSLMode: "disable", User: "alice@acme.fr", Token: tok, Version: "1.2.0"}
	if ro != "" {
		t.RO = []string{ro}
	}
	return t
}

func TestAppNameAndRouting(t *testing.T) {
	if AppName("1.2.0", UsagePivot) != "Lattis/1.2.0/pivot" || AppName("", UsageWrite) != "Lattis/dev/write" {
		t.Fatal(AppName("1.2.0", UsagePivot))
	}
	if len(AppName(strings.Repeat("9", 80), UsageSQL)) != 63 {
		t.Fatal("application_name non tronqué")
	}
	for u, ro := range map[string]bool{UsageInitialLoad: true, UsagePivot: true, UsageSQL: true, UsagePowerQuery: true, UsageDelta: false, UsageWrite: false} {
		if ReadUsage(u) != ro {
			t.Errorf("ReadUsage(%s) = %v", u, !ro)
		}
	}
	h, p, err := hostsPorts([]string{"pg1.acme.fr", "pg2.acme.fr:6543", "[::1]:5433"}, 0)
	if err != nil || h != "pg1.acme.fr,pg2.acme.fr,::1" || p != "5432,6543,5433" {
		t.Fatalf("hostsPorts = %s %s %v", h, p, err)
	}
	if _, err := (Target{RW: "x", Token: func(context.Context) (string, error) { return "", nil }}).dsn([]string{"x"}, UsageSQL); err == nil {
		t.Fatal("connexion sans rôle PostgreSQL acceptée")
	}
}

func TestOAuthBearerOpenDB(t *testing.T) {
	rw := newFakePG(t, "jeton-rw")
	ro := newFakePG(t, "jeton-rw")
	var calls atomic.Int32
	tok := func(context.Context) (string, error) { calls.Add(1); return "jeton-rw", nil }
	db, err := target(rw.addr(), ro.addr(), tok).OpenDB(UsagePowerQuery)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(ro.startups) != 1 || len(rw.startups) != 0 {
		t.Fatalf("Power Query lu sur ro : rw %d, ro %d", len(rw.startups), len(ro.startups))
	}
	p := ro.startups[0]
	if p["user"] != "alice@acme.fr" || p["database"] != "lattis" || p["application_name"] != "Lattis/1.2.0/power_query" {
		t.Fatalf("paramètres de démarrage = %v", p)
	}
	if calls.Load() != 1 || ro.tokens[0] != "jeton-rw" {
		t.Fatalf("jeton : %d appels, reçu %v", calls.Load(), ro.tokens)
	}

	bad, _ := target(rw.addr(), "", func(context.Context) (string, error) { return "périmé", nil }).OpenDB(UsageWrite)
	defer bad.Close()
	if err := bad.PingContext(context.Background()); err == nil || !strings.Contains(err.Error(), "28000") {
		t.Fatalf("jeton refusé : %v", err)
	}
}

func TestPools(t *testing.T) {
	rw := newFakePG(t, "j")
	ro := newFakePG(t, "j")
	ctx := context.Background()
	p, err := Open(ctx, target(rw.addr(), ro.addr(), func(context.Context) (string, error) { return "j", nil }))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	c, err := p.Acquire(ctx, UsagePivot)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Conn().PgConn().ParameterStatus("application_name"); got != "Lattis/1.2.0/pivot" {
		t.Fatalf("application_name = %q", got)
	}
	c.Release()
	w, err := p.Acquire(ctx, UsageDelta)
	if err != nil {
		t.Fatal(err)
	}
	w.Release()
	rw.mu.Lock()
	defer rw.mu.Unlock()
	ro.mu.Lock()
	defer ro.mu.Unlock()
	if len(ro.startups) != 1 || ro.startups[0]["application_name"] != "Lattis/1.2.0/initial_load" || len(ro.appNames) != 1 {
		t.Fatalf("ro : %v %v", ro.startups, ro.appNames)
	}
	if len(rw.startups) != 1 || rw.startups[0]["application_name"] != "Lattis/1.2.0/write" || len(rw.appNames) != 1 || rw.appNames[0] != "Lattis/1.2.0/delta" {
		t.Fatalf("rw : %v %v", rw.startups, rw.appNames)
	}

	// Sans réplica, les lectures passent par rw.
	p2, err := Open(ctx, target(rw.addr(), "", func(context.Context) (string, error) { return "j", nil }))
	if err != nil {
		t.Fatal(err)
	}
	defer p2.Close()
	if p2.RO != nil {
		t.Fatal("pool ro sans réplica")
	}
	if len(rw.devices) != 0 {
		t.Fatalf("lattis.device posé sans identifiant de poste : %v", rw.devices)
	}
}

// Identifiant du poste (contrat client §6) : posé sur chaque connexion du
// pool rw (écritures, deltas), jamais sur les réplicas.
func TestPoolsDevice(t *testing.T) {
	rw := newFakePG(t, "j")
	ro := newFakePG(t, "j")
	ctx := context.Background()
	tg := target(rw.addr(), ro.addr(), func(context.Context) (string, error) { return "j", nil })
	tg.Device = "poste-d'Alice"
	p, err := Open(ctx, tg)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for _, u := range []string{UsageWrite, UsagePivot} {
		c, err := p.Acquire(ctx, u)
		if err != nil {
			t.Fatal(err)
		}
		c.Release()
	}
	rw.mu.Lock()
	defer rw.mu.Unlock()
	ro.mu.Lock()
	defer ro.mu.Unlock()
	if len(rw.devices) != 1 || rw.devices[0] != "poste-d'Alice" || len(ro.devices) != 0 {
		t.Fatalf("lattis.device : rw %v, ro %v", rw.devices, ro.devices)
	}
}
