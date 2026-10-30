package collab_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/remote/pgpool"
	"github.com/lalamefine/lattis/internal/remote/workbook"
)

// Essais de bout en bout contre un vrai lattisd (relais /api/events, jetons
// vérifiés par PostgreSQL avec lattis_oauth) : connexions des postes par
// pgpool et OAUTHBEARER comme dans l'application. Activés par
// LATTIS_LATTISD_URL (voir docs/audits/audit-collaboration-2026-10-30.md,
// « Banc d'essai ») :
//
//	LATTIS_LATTISD_URL      adresse publique de lattisd (émetteur des jetons)
//	LATTIS_LATTISD_KEY      clé de signature de lattisd (signing.pem)
//	LATTIS_LATTISD_PG       hôte:port de PostgreSQL annoncé aux postes
//	LATTIS_LATTISD_SERVICE  URL de lattis_service (création des comptes)
//
// Les jetons sont signés par l'essai avec la clé de lattisd, comme lattisd
// les signe au bout du parcours de connexion.

type ldEnv struct {
	url, pg, svc string
	key          ed25519.PrivateKey
	kid          string
}

func ldSetup(t *testing.T) *ldEnv {
	t.Helper()
	e := &ldEnv{url: strings.TrimSuffix(os.Getenv("LATTIS_LATTISD_URL"), "/"), pg: os.Getenv("LATTIS_LATTISD_PG"), svc: os.Getenv("LATTIS_LATTISD_SERVICE")}
	if e.url == "" {
		t.Skip("LATTIS_LATTISD_URL non défini : essai contre lattisd ignoré")
	}
	if e.pg == "" || e.svc == "" || os.Getenv("LATTIS_LATTISD_KEY") == "" {
		t.Fatal("LATTIS_LATTISD_KEY, LATTIS_LATTISD_PG et LATTIS_LATTISD_SERVICE sont requis avec LATTIS_LATTISD_URL")
	}
	raw, err := os.ReadFile(os.Getenv("LATTIS_LATTISD_KEY"))
	must(t, err)
	b, _ := pem.Decode(raw)
	if b == nil {
		t.Fatal("clé de signature : PEM attendu")
	}
	k, err := x509.ParsePKCS8PrivateKey(b.Bytes)
	must(t, err)
	priv, ok := k.(ed25519.PrivateKey)
	if !ok {
		t.Fatal("clé de signature : Ed25519 attendue")
	}
	e.key = priv
	sum := sha256.Sum256(priv.Public().(ed25519.PublicKey))
	e.kid = base64.RawURLEncoding.EncodeToString(sum[:12])
	useRoot(t)
	return e
}

// exec exécute une instruction sous lattis_service.
func (e *ldEnv) exec(t *testing.T, q string, args ...any) {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, e.svc)
	must(t, err)
	defer c.Close(ctx)
	_, err = c.Exec(ctx, q, args...)
	must(t, err)
}

// text rend la première colonne de la première ligne, en texte.
func (e *ldEnv) text(t *testing.T, q string, args ...any) string {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, e.svc)
	must(t, err)
	defer c.Close(ctx)
	var s string
	must(t, c.QueryRow(ctx, q, args...).Scan(&s))
	return s
}

// user crée un compte et rend son rôle PostgreSQL et son identifiant.
func (e *ldEnv) user(t *testing.T, name string) (role, id string) {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, e.svc)
	must(t, err)
	defer c.Close(ctx)
	email := fmt.Sprintf("%s.%s@example.test", name, uuid.NewString()[:8])
	must(t, c.QueryRow(ctx, `SELECT pg_role, id::text FROM lattis.create_user($1, $2, NULL, NULL)`, email, name).Scan(&role, &id))
	// Première connexion (lattisd) : le rôle reçoit le droit de connexion.
	_, err = c.Exec(ctx, `SELECT lattis.activate_user($1::uuid)`, id)
	must(t, err)
	return role, id
}

// mint signe un jeton d'accès (at+jwt) qui expire à exp.
func (e *ldEnv) mint(role, id string, exp time.Time) string {
	now := time.Now()
	enc := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	var jti [16]byte
	rand.Read(jti[:])
	in := enc(map[string]string{"alg": "EdDSA", "typ": "at+jwt", "kid": e.kid}) + "." + enc(map[string]any{
		"iss": e.url, "sub": id, "aud": "postgres", "iat": now.Unix(), "nbf": now.Add(-time.Minute).Unix(), "exp": exp.Unix(),
		"jti": base64.RawURLEncoding.EncodeToString(jti[:]), "client_id": "lattis-desktop", "scope": "openid lattis", "pg_role": role,
	})
	return in + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(e.key, []byte(in)))
}

// tokens imite auth.Manager.Token : jeton gardé, renouvelé une minute
// avant son expiration, oublié par invalidate (refus du serveur).
type tokens struct {
	e        *ldEnv
	role, id string
	ttl      time.Duration
	mu       sync.Mutex
	cur      string
	exp      time.Time
	mints    atomic.Int32
}

func (k *tokens) token(ctx context.Context) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.cur == "" || !time.Now().Add(time.Minute).Before(k.exp) {
		k.exp = time.Now().Add(k.ttl)
		k.cur = k.e.mint(k.role, k.id, k.exp)
		k.mints.Add(1)
	}
	return k.cur, nil
}

func (k *tokens) invalidate() {
	k.mu.Lock()
	k.cur = ""
	k.mu.Unlock()
}

// stale remplace le jeton gardé par un jeton expiré depuis longtemps, que
// le poste croit encore valable (veille prolongée, horloge du poste en
// retard) : seul le refus du serveur le fait renouveler.
func (k *tokens) stale() {
	k.mu.Lock()
	k.cur = k.e.mint(k.role, k.id, time.Now().Add(-10*time.Minute))
	k.exp = time.Now().Add(time.Hour)
	k.mu.Unlock()
}

// cutter est un relais TCP coupable : cut ferme les connexions en cours et
// refuse les nouvelles (câble débranché, Wi-Fi perdu), mend les accepte de
// nouveau.
type cutter struct {
	ln     net.Listener
	target string
	down   atomic.Bool
	mu     sync.Mutex
	conns  map[net.Conn]bool
}

func newCutter(t *testing.T, target string) *cutter {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	must(t, err)
	c := &cutter{ln: ln, target: target, conns: map[net.Conn]bool{}}
	t.Cleanup(func() { ln.Close(); c.cut() })
	go func() {
		for {
			in, err := ln.Accept()
			if err != nil {
				return
			}
			if c.down.Load() {
				in.Close()
				continue
			}
			out, err := net.Dial("tcp", target)
			if err != nil {
				in.Close()
				continue
			}
			c.mu.Lock()
			c.conns[in], c.conns[out] = true, true
			c.mu.Unlock()
			pipe := func(a, b net.Conn) {
				io.Copy(a, b)
				a.Close()
				b.Close()
				c.mu.Lock()
				delete(c.conns, a)
				delete(c.conns, b)
				c.mu.Unlock()
			}
			go pipe(in, out)
			go pipe(out, in)
		}
	}()
	return c
}

func (c *cutter) addr() string { return c.ln.Addr().String() }

func (c *cutter) cut() {
	c.down.Store(true)
	c.mu.Lock()
	for k := range c.conns {
		k.Close()
	}
	c.mu.Unlock()
}

func (c *cutter) mend() { c.down.Store(false) }

// station est un poste branché sur lattisd comme l'application :
// pgpool (OAUTHBEARER), relais /api/events, jeton renouvelé.
type station struct {
	*client
	role, id string
	tok      *tokens
	pools    *pgpool.Pools
	srv      workbook.Server
	// fsrv : serveur du suivi (relais, ou srv pour LISTEN).
	fsrv workbook.Server
	// net : coupures du poste (nil sans relais coupable).
	pgNet, httpNet *cutter
	// relay : suivi par /api/events (faux : LISTEN sur PostgreSQL).
	relay bool
}

type stationOpts struct {
	ttl      time.Duration
	cuttable bool
	listen   bool // LISTEN au lieu du relais
}

func (e *ldEnv) station(t *testing.T, name, role, id string, o stationOpts) *station {
	t.Helper()
	if o.ttl == 0 {
		o.ttl = 15 * time.Minute
	}
	s := &station{role: role, id: id, tok: &tokens{e: e, role: role, id: id, ttl: o.ttl}, relay: !o.listen}
	pgAddr := e.pg
	if o.cuttable {
		s.pgNet = newCutter(t, e.pg)
		pgAddr = s.pgNet.addr()
		host := strings.TrimPrefix(strings.TrimPrefix(e.url, "http://"), "https://")
		s.httpNet = newCutter(t, host)
	}
	p, err := pgpool.Open(context.Background(), pgpool.Target{RW: pgAddr, SSLMode: "require", User: role, Token: s.tok.token, Version: "charge", Device: uuid.NewString()})
	must(t, err)
	t.Cleanup(p.Close)
	s.pools = p
	// Comme bridge.remoteServer : jeton refusé par PostgreSQL, renouvelé
	// puis un seul nouvel essai.
	srv := workbook.PG(func(ctx context.Context, usage string) (*pgx.Conn, func(), error) {
		c, err := p.Acquire(ctx, usage)
		if pgpool.AuthFailed(err) {
			s.tok.invalidate()
			c, err = p.Acquire(ctx, usage)
		}
		if err != nil {
			return nil, nil, err
		}
		return c.Conn(), c.Release, nil
	})
	s.srv = srv
	s.client = &client{t: t, name: name}
	return s
}

// open charge le classeur wb sur le poste et prépare sa synchronisation.
func (s *station) open(t *testing.T, e *ldEnv, wb string) {
	t.Helper()
	ctx := context.Background()
	d, err := doc.New(ctx)
	must(t, err)
	t.Cleanup(func() { d.Close() })
	_, err = workbook.Load(ctx, s.srv, d.DB(), workbook.Source{Server: server, User: s.role, WorkbookID: wb}, true)
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	s.adopt(t, e, d)
}

// adopt branche la synchronisation du poste sur le classeur d.
func (s *station) adopt(t *testing.T, e *ldEnv, d *doc.Document) {
	s.fsrv = s.followServer(e, d)
	s.client = newClient(t, s.name, d, s.fsrv)
}

// followServer : le serveur du suivi (relais de lattisd ou LISTEN).
func (s *station) followServer(e *ldEnv, d *doc.Document) workbook.Server {
	if !s.relay {
		return s.srv
	}
	base := e.url
	if s.httpNet != nil {
		base = "http://" + s.httpNet.addr()
	}
	info := d.Share.Info()
	es := workbook.WithEvents(s.srv, base+"/api/events", info.WorkbookID, s.tok.token, s.tok.invalidate)
	es.Client = &http.Client{}
	return es
}

// cut coupe le réseau du poste ; mend le rétablit.
func (s *station) cut()  { s.pgNet.cut(); s.httpNet.cut() }
func (s *station) mend() { s.pgNet.mend(); s.httpNet.mend() }

// ldPublish publie depuis le poste d'Alice un classeur collaboratif
// « Ventes » de rows lignes (Produit, Quantité, Prix) et rend son
// identifiant.
func ldPublish(t *testing.T, e *ldEnv, alice *station, rows int) string {
	t.Helper()
	ctx := context.Background()
	d, err := doc.New(ctx)
	must(t, err)
	t.Cleanup(func() { d.Close() })
	data := [][]string{{"Produit", "Quantité", "Prix"}}
	for i := 0; i < rows; i++ {
		data = append(data, []string{fmt.Sprintf("Produit %d", i+1), fmt.Sprint(i), "1,5"})
	}
	_, err = d.Paste(ctx, 1, 0, 0, data)
	must(t, err)
	var first string
	must(t, d.DB().Read().QueryRow(`SELECT name FROM _lattis_tables WHERE sheet = 1 AND row = 0 AND col = 0`).Scan(&first))
	must(t, d.RenameTable(ctx, first, "Ventes"))
	info, err := workbook.Publish(ctx, alice.srv, d.DB(), workbook.Request{Server: server, User: alice.role, Title: "Ventes " + uuid.NewString()[:6], Mode: workbook.Collaborative, Tables: []int64{tableID(t, d)}})
	must(t, err)
	must(t, d.ShareChanged(ctx))
	d.Share.SetOnline(true, "")
	alice.adopt(t, e, d)
	return info.WorkbookID
}
