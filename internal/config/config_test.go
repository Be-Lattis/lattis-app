package config

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func src(name string, r Rank, trusted bool, t Tree) Source {
	return &TreeSource{N: name, R: r, Trust: trusted, T: t}
}

func resolve(t *testing.T, opts Options, sources ...Source) *Config {
	t.Helper()
	return Resolve(context.Background(), opts, sources...)
}

func TestPrecedenceAndProvenance(t *testing.T) {
	c := resolve(t, Options{},
		src("défaut", RankDefault, false, Defaults()),
		src("machine", RankMachine, true, Tree{"ai": Tree{"endpoint": "https://machine/", "model": "m1"}}),
		src("env", RankEnv, false, Tree{"ai": Tree{"endpoint": "https://env/"}}),
		src("user", RankUser, false, Tree{"ai": Tree{"endpoint": "https://user/", "model": "m2"}}),
	)
	if got := c.String("ai.endpoint", ""); got != "https://env/" {
		t.Fatalf("ai.endpoint = %q", got)
	}
	if got := c.String("ai.model", ""); got != "m2" {
		t.Fatalf("ai.model = %q", got)
	}
	o, _ := c.Origin("ai.endpoint")
	if o.Source != "env" || o.Rank != RankEnv || o.Locked {
		t.Fatalf("origine = %+v", o)
	}
	// Une feuille d'un objet fusionné garde la valeur d'en dessous.
	if !c.Bool("features.allowUserConnections", false) {
		t.Fatal("défaut perdu")
	}
	if len(c.Warnings) != 0 {
		t.Fatalf("avertissements : %v", c.Warnings)
	}
}

func TestPolicyLocks(t *testing.T) {
	layers := []Source{
		src("défaut", RankDefault, false, Defaults()),
		src("env", RankEnv, false, Tree{"features": Tree{"mcp": Tree{"enabled": true}}}),
		src("stratégie", RankPolicy, true, Tree{"features": Tree{"mcp": Tree{"enabled": false}}}),
	}
	c := resolve(t, Options{PolicyOverride: true}, layers...)
	if c.Bool("features.mcp.enabled", true) {
		t.Fatal("l'environnement a passé outre la stratégie")
	}
	if !c.Locked("features.mcp.enabled") || !c.Locked("features.mcp") {
		t.Fatal("clé non verrouillée")
	}
	if c.Locked("ai.enabled") {
		t.Fatal("clé verrouillée à tort")
	}

	// La stratégie autorise elle-même le contournement : l'environnement gagne.
	layers[2] = src("stratégie", RankPolicy, true, Tree{"allowPolicyOverride": true,
		"features": Tree{"mcp": Tree{"enabled": false}}})
	c = resolve(t, Options{PolicyOverride: true}, layers...)
	if !c.Bool("features.mcp.enabled", false) || c.Locked("features.mcp.enabled") {
		t.Fatal("contournement autorisé non appliqué")
	}
	// Sans la demande explicite, la stratégie reste au-dessus.
	c = resolve(t, Options{}, layers...)
	if c.Bool("features.mcp.enabled", true) {
		t.Fatal("contournement appliqué sans demande")
	}
}

func TestNullRemovesAndCaseInsensitiveMerge(t *testing.T) {
	c := resolve(t, Options{},
		src("machine", RankMachine, true, Tree{"connections": Tree{
			"erp":    Tree{"kind": "sqlserver", "config": Tree{"host": "prod"}},
			"ventes": Tree{"kind": "web"},
		}}),
		src("env", RankEnv, false, Tree{"CONNECTIONS": Tree{"ERP": Tree{"config": Tree{"host": "staging"}}, "ventes": nil}}),
	)
	if got := c.String("connections.erp.config.host", ""); got != "staging" {
		t.Fatalf("host = %q", got)
	}
	if got := c.String("connections.erp.kind", ""); got != "sqlserver" {
		t.Fatalf("kind = %q", got)
	}
	if _, ok := c.Lookup("connections.ventes"); ok {
		t.Fatal("null n'a pas retiré la connexion")
	}
	for _, e := range c.Explain() {
		if strings.Contains(strings.ToLower(e.Path), "ventes") {
			t.Fatalf("provenance restée : %v", e)
		}
	}
	s, err := c.Settings()
	if err != nil || s.Connections["erp"].Config["host"] != "staging" {
		t.Fatalf("Settings = %+v, %v", s.Connections, err)
	}
}

func TestInvalidValueFallsBack(t *testing.T) {
	c := resolve(t, Options{},
		src("défaut", RankDefault, false, Defaults()),
		src("machine", RankMachine, true, Tree{"ai": Tree{"endpoint": "https://ok/"}}),
		src("env", RankEnv, false, Tree{"ai": Tree{"endpoint": "pas une adresse", "dataSharing": "tout"}}),
	)
	if got := c.String("ai.endpoint", ""); got != "https://ok/" {
		t.Fatalf("ai.endpoint = %q", got)
	}
	if got := c.String("ai.dataSharing", ""); got != "schema-only" {
		t.Fatalf("ai.dataSharing = %q", got)
	}
	if len(c.Warnings) != 2 {
		t.Fatalf("avertissements : %v", c.Warnings)
	}
}

func TestSecretsRefused(t *testing.T) {
	c := resolve(t, Options{},
		src("machine", RankMachine, true, Tree{
			"connections": Tree{"erp": Tree{"config": Tree{"host": "h", "password": "x"}}},
			"ai":          Tree{"apiKey": "sk-123"},
			"identity": Tree{"providers": Tree{"google": Tree{
				"clientSecret": "non", "desktopClientSecret": "GOCSPX-oui"}}},
		}),
	)
	if _, ok := c.Lookup("connections.erp.config.password"); ok {
		t.Fatal("mot de passe accepté")
	}
	if _, ok := c.Lookup("ai.apiKey"); ok {
		t.Fatal("clé d'API acceptée")
	}
	if _, ok := c.Lookup("identity.providers.google.clientSecret"); ok {
		t.Fatal("secret client accepté")
	}
	if c.String("identity.providers.google.desktopClientSecret", "") != "GOCSPX-oui" {
		t.Fatal("secret de bureau Google refusé")
	}
	b, _ := c.JSON()
	if bytes.Contains(b, []byte("GOCSPX")) {
		t.Fatal("secret de bureau affiché")
	}
	if len(c.Warnings) != 3 {
		t.Fatalf("avertissements : %v", c.Warnings)
	}
}

func TestAdminOnlyKeys(t *testing.T) {
	// Un administrateur pose configUrl : l'utilisateur ne peut pas la changer.
	c := resolve(t, Options{},
		src("machine", RankMachine, true, Tree{"configUrl": "https://acme/config"}),
		src("env", RankEnv, false, Tree{"configUrl": "https://ailleurs/config"}),
	)
	if got := c.String("configUrl", ""); got != "https://acme/config" {
		t.Fatalf("configUrl = %q", got)
	}
	// Sans configuration d'entreprise, l'environnement peut la poser (tests).
	c = resolve(t, Options{}, src("env", RankEnv, false, Tree{"configUrl": "https://test/config"}))
	if got := c.String("configUrl", ""); got != "https://test/config" {
		t.Fatalf("configUrl = %q", got)
	}
	o, _ := c.Origin("configUrl")
	if o.Trusted {
		t.Fatal("environnement tenu pour fiable")
	}
	// allowPolicyOverride n'est lu que dans la stratégie.
	c = resolve(t, Options{}, src("user", RankUser, false, Tree{"allowPolicyOverride": true}))
	if _, ok := c.Lookup("allowPolicyOverride"); ok {
		t.Fatal("allowPolicyOverride lu hors stratégie")
	}
}

func TestEnvSource(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "c.json")
	os.WriteFile(f, []byte(`{"organization":"ACME","ai":{"model":"m"}}`), 0o600)
	env := []string{
		"LATTIS_SQL=1",
		"LATTIS_CONFIG=" + f,
		`LATTIS_CONFIG_JSON={"ai":{"model":"json"}}`,
		"LATTIS_FEATURES__MCP__ENABLED=1",
		"LATTIS_CONNECTIONS__ERP__CONFIG__PORT=1434",
		"LATTIS_CONNECTIONS__ERP__CONFIG__HOST=sql-staging",
		"LATTIS_AI_ENDPOINT=https://ia.acme/",
		"PATH=/bin",
	}
	c := resolve(t, Options{},
		src("défaut", RankDefault, false, Defaults()),
		&EnvSource{Environ: func() []string { return env }},
	)
	checks := map[string]any{
		"organization":                "ACME",
		"ai.model":                    "json",
		"features.mcp.enabled":        true,
		"connections.erp.config.port": float64(1434),
		"connections.erp.config.host": "sql-staging",
		"ai.endpoint":                 "https://ia.acme/",
	}
	for p, want := range checks {
		if got, _ := c.Lookup(p); got != want {
			t.Errorf("%s = %v, attendu %v", p, got, want)
		}
	}
	if _, ok := c.Lookup("sql"); ok {
		t.Error("LATTIS_SQL lue comme configuration")
	}
	// La clé de l'environnement (en minuscules) rejoint celle du défaut.
	var s Settings
	c.Decode("", &s)
	if s.Features.MCP.Enabled == nil || !*s.Features.MCP.Enabled {
		t.Errorf("Settings.Features.MCP = %+v", s.Features.MCP)
	}
}

func TestFlagSource(t *testing.T) {
	sets, rest, err := SplitSetFlags([]string{"--demo", "--set", "ai.model=x", "--set=ai.enabled=true"})
	if err != nil || len(rest) != 1 || rest[0] != "--demo" {
		t.Fatalf("SplitSetFlags : %v %v %v", sets, rest, err)
	}
	c := resolve(t, Options{}, &FlagSource{Pairs: sets})
	if c.String("ai.model", "") != "x" || !c.Bool("ai.enabled", false) {
		t.Fatalf("--set non appliqué : %v", c.Explain())
	}
}

func TestInsecurePolicyFileIgnored(t *testing.T) {
	old := secureFile
	t.Cleanup(func() { secureFile = old })
	dir := t.TempDir()
	p := filepath.Join(dir, "policy.json")
	m := filepath.Join(dir, "config.json")
	os.WriteFile(p, []byte(`{"features":{"mcp":{"enabled":false}}}`), 0o644)
	os.WriteFile(m, []byte(`{"configUrl":"https://acme/config"}`), 0o644)

	secureFile = func(string) (bool, error) { return false, nil }
	c := resolve(t, Options{},
		&FileSource{Path: m, R: RankMachine, AdminFile: true},
		&FileSource{Path: p, R: RankPolicy, AdminFile: true},
	)
	if c.Locked("features.mcp.enabled") {
		t.Fatal("stratégie modifiable par tous appliquée")
	}
	if o, _ := c.Origin("configUrl"); o.Trusted {
		t.Fatal("fichier machine modifiable par tous tenu pour fiable")
	}

	secureFile = func(string) (bool, error) { return true, nil }
	c = resolve(t, Options{},
		&FileSource{Path: m, R: RankMachine, AdminFile: true},
		&FileSource{Path: p, R: RankPolicy, AdminFile: true},
		&FileSource{Path: filepath.Join(dir, "absent.json"), R: RankUser},
	)
	if !c.Locked("features.mcp.enabled") || len(c.Warnings) != 0 {
		t.Fatalf("stratégie non appliquée : %v", c.Warnings)
	}
}

// remoteServer sert un document et compte les requêtes.
type remoteServer struct {
	*httptest.Server
	body  atomic.Value
	etag  atomic.Value
	calls atomic.Int32
}

func newRemoteServer(t *testing.T, body string) *remoteServer {
	rs := &remoteServer{}
	rs.body.Store(body)
	rs.etag.Store(`"v1"`)
	rs.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rs.calls.Add(1)
		etag := rs.etag.Load().(string)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Write([]byte(rs.body.Load().(string)))
	}))
	t.Cleanup(rs.Close)
	return rs
}

func TestRemoteSignedWithPolicy(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	doc := `{"connections":{"erp":{"kind":"sqlserver","config":{"host":"distant"}}},
		"configUrl":"https://autre/","policy":{"features":{"mcp":{"enabled":false}}}}`
	rs := newRemoteServer(t, SignJWS([]byte(doc), priv))
	cache := filepath.Join(t.TempDir(), "cache.json")
	machine := Tree{"configUrl": rs.URL + "/lattis.json",
		"remote": Tree{"publicKey": base64.StdEncoding.EncodeToString(pub)}}
	env := Tree{"features": Tree{"mcp": Tree{"enabled": true}}}
	l := &Loader{
		CacheFile: cache,
		Client:    rs.Client(),
		Environ:   func() []string { return nil },
		Local: func() []Source {
			return []Source{src("machine", RankMachine, true, machine), src("env", RankEnv, false, env)}
		},
	}
	ctx := context.Background()
	c := l.Load(ctx)
	if c.Remote.Cached || c.String("connections.erp.config.host", "") != "" {
		t.Fatal("configuration distante lue sans cache")
	}
	changed, err := l.Refresh(ctx)
	if err != nil || !changed {
		t.Fatalf("Refresh = %v, %v", changed, err)
	}
	c = l.Load(ctx)
	if got := c.String("connections.erp.config.host", ""); got != "distant" {
		t.Fatalf("host = %q (%v)", got, c.Warnings)
	}
	if !c.Remote.Signed || !c.Remote.Cached {
		t.Fatalf("Remote = %+v", c.Remote)
	}
	if c.Bool("features.mcp.enabled", true) || !c.Locked("features.mcp.enabled") {
		t.Fatal("stratégie distante non appliquée")
	}
	if got := c.String("configUrl", ""); got != rs.URL+"/lattis.json" {
		t.Fatalf("la configuration distante a changé configUrl : %q", got)
	}
	// Même ETag : 304, rien ne change.
	if changed, err := l.Refresh(ctx); err != nil || changed {
		t.Fatalf("Refresh (304) = %v, %v", changed, err)
	}
	// Document altéré : refusé, le cache reste.
	rs.etag.Store(`"v2"`)
	rs.body.Store(SignJWS([]byte(doc), priv)[:40] + "x" + SignJWS([]byte(doc), priv)[41:])
	if _, err := l.Refresh(ctx); err == nil {
		t.Fatal("document altéré accepté")
	}
	if got := l.Load(ctx).String("connections.erp.config.host", ""); got != "distant" {
		t.Fatalf("cache perdu : %q", got)
	}
	// Non signé alors qu'une clé est posée : refusé.
	rs.body.Store(`{"organization":"pirate"}`)
	if _, err := l.Refresh(ctx); err == nil {
		t.Fatal("document non signé accepté")
	}
}

func TestRemoteUnsignedHasNoPolicy(t *testing.T) {
	rs := newRemoteServer(t, `{"organization":"ACME","policy":{"ai":{"enabled":false}}}`)
	l := &Loader{
		CacheFile: filepath.Join(t.TempDir(), "cache.json"),
		Client:    rs.Client(),
		Environ:   func() []string { return nil },
		Local: func() []Source {
			return []Source{src("machine", RankMachine, true, Tree{"configUrl": rs.URL})}
		},
	}
	ctx := context.Background()
	l.Load(ctx)
	if _, err := l.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	c := l.Load(ctx)
	if c.String("organization", "") != "ACME" {
		t.Fatalf("organization absente : %v", c.Warnings)
	}
	if c.Locked("ai.enabled") {
		t.Fatal("stratégie lue dans un document non signé")
	}
}

func TestCLISignAndCheck(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "cle.pem")
	var out, errb bytes.Buffer
	if code := Run([]string{"keygen", key}, &out, &errb); code != 0 {
		t.Fatalf("keygen : %s", errb.String())
	}
	var pubLine string
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "remote.publicKey : ") {
			pubLine = strings.TrimPrefix(line, "remote.publicKey : ")
		}
	}
	pub, err := ParsePublicKey(pubLine)
	if err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(dir, "doc.json")
	os.WriteFile(doc, []byte(`{"organization":"ACME"}`), 0o600)
	out.Reset()
	if code := Run([]string{"sign", key, doc}, &out, &errb); code != 0 {
		t.Fatalf("sign : %s", errb.String())
	}
	d, err := ParseRemote(out.Bytes(), pub, true)
	if err != nil || !d.Signed || d.Config["organization"] != "ACME" {
		t.Fatalf("ParseRemote = %+v, %v", d, err)
	}
	out.Reset()
	if code := Run([]string{"check", doc}, &out, &errb); code != 0 {
		t.Fatalf("check : %s %s", out.String(), errb.String())
	}
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte(`{"ai":{"endpoint":"http://non"}}`), 0o600)
	if code := Run([]string{"check", bad}, &out, &errb); code == 0 {
		t.Fatal("check a accepté une adresse http")
	}
}

func TestWebPagesSetting(t *testing.T) {
	c := resolve(t, Options{}, src("défaut", RankDefault, false, Defaults()))
	if v, ok := c.Lookup("features.webPages"); !ok || v != false {
		t.Fatalf("features.webPages par défaut = %v, %v", v, ok)
	}
	c = resolve(t, Options{},
		src("défaut", RankDefault, false, Defaults()),
		src("env", RankEnv, false, Tree{"features": Tree{"webPages": "1"}}),
	)
	if !c.Bool("features.webPages", false) {
		t.Fatal("features.webPages = \"1\" non converti")
	}
	c = resolve(t, Options{},
		src("défaut", RankDefault, false, Defaults()),
		src("user", RankUser, false, Tree{"features": Tree{"webPages": "peut-être"}}),
	)
	if c.Bool("features.webPages", true) || len(c.Warnings) != 1 {
		t.Fatalf("valeur invalide gardée : %v", c.Warnings)
	}
	var s Settings
	c = resolve(t, Options{}, src("défaut", RankDefault, false, Defaults()),
		src("user", RankUser, false, Tree{"features": Tree{"webPages": true}}))
	if err := c.Decode("", &s); err != nil || s.Features.WebPages == nil || !*s.Features.WebPages {
		t.Fatalf("Settings.Features.WebPages = %v, %v", s.Features.WebPages, err)
	}
}

func TestSetUser(t *testing.T) {
	defer SetCurrent(nil)
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "Lattis", "config.json")
	var policy Tree
	l := &Loader{
		Environ:  func() []string { return nil },
		UserFile: file,
		Local: func() []Source {
			out := []Source{&FileSource{Path: file, R: RankUser}}
			if policy != nil {
				out = append(out, src("stratégie", RankPolicy, true, policy))
			}
			return out
		},
	}
	if l.Load(ctx).Bool("features.webPages", true) {
		t.Fatal("features.webPages activé par défaut")
	}
	// Fichier et dossier créés ; la configuration courante suit.
	c, err := l.SetUser(ctx, "features.webPages", true)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Bool("features.webPages", false) || !Current().Bool("features.webPages", false) {
		t.Fatal("réglage non pris en compte")
	}
	st := UserSettings(c)
	if len(st) != 1 || st[0].Value != true || st[0].Locked || st[0].Fixed || st[0].Source != file {
		t.Fatalf("UserSettings = %+v", st)
	}
	// Les autres clés du fichier sont gardées.
	os.WriteFile(file, []byte(`{"organization":"ACME","features":{"webPages":true,"mcp":{"enabled":true}}}`), 0o600)
	if c, err = l.SetUser(ctx, "features.webPages", false); err != nil {
		t.Fatal(err)
	}
	if c.Bool("features.webPages", true) || c.String("organization", "") != "ACME" || !c.Bool("features.mcp.enabled", false) {
		t.Fatalf("fichier mal réécrit : %v", c.Explain())
	}
	// Clé hors de la liste, valeur invalide : refusées.
	if _, err := l.SetUser(ctx, "features.mcp.enabled", true); err == nil {
		t.Fatal("clé hors de la liste acceptée")
	}
	if _, err := l.SetUser(ctx, "features.webPages", "peut-être"); err == nil {
		t.Fatal("valeur invalide acceptée")
	}
	// Fichier illisible : pas écrasé.
	os.WriteFile(file, []byte(`{cassé`), 0o600)
	if _, err := l.SetUser(ctx, "features.webPages", true); err == nil {
		t.Fatal("fichier cassé écrasé")
	}
	if b, _ := os.ReadFile(file); string(b) != `{cassé` {
		t.Fatalf("fichier modifié : %s", b)
	}
	os.Remove(file)
	// Stratégie : verrouillé, refusé, rien d'écrit.
	policy = Tree{"features": Tree{"webPages": false}}
	if _, err := l.SetUser(ctx, "features.webPages", true); err == nil {
		t.Fatal("réglage verrouillé modifié")
	}
	if st := UserSettings(l.Load(ctx)); !st[0].Locked {
		t.Fatalf("UserSettings = %+v", st)
	}
	if _, err := os.Stat(file); err == nil {
		t.Fatal("fichier écrit malgré la stratégie")
	}
}
