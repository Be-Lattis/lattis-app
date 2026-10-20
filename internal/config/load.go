package config

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"github.com/lalamefine/lattis/internal/msg"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// RemoteStatus décrit la configuration distante utilisée.
type RemoteStatus struct {
	URL       string    `json:"url,omitempty"`
	FetchedAt time.Time `json:"fetchedAt,omitzero"`
	Signed    bool      `json:"signed,omitempty"`
	// Cached : un document est en cache pour cette adresse.
	Cached bool `json:"cached,omitempty"`
}

// Loader construit la configuration de l'application et rafraîchit la
// configuration distante.
type Loader struct {
	// SetFlags : options --set chemin=valeur.
	SetFlags []string
	// Environ remplace os.Environ (tests).
	Environ func() []string
	// Local remplace les couches locales du système (tests).
	Local func() []Source
	// CacheFile remplace le cache de la configuration distante (tests).
	CacheFile string
	// Client remplace le client HTTP (tests).
	Client *http.Client
	// UserFile remplace le config.json de l'utilisateur écrit par SetUser
	// (tests ; Local doit alors le lire).
	UserFile string

	mu            sync.Mutex
	url           string
	key           ed25519.PublicKey
	policyAllowed bool
}

func (l *Loader) environ() []string {
	if l.Environ != nil {
		return l.Environ()
	}
	return os.Environ()
}

func (l *Loader) getenv(k string) string {
	for _, kv := range l.environ() {
		if len(kv) > len(k) && kv[len(k)] == '=' && kv[:len(k)] == k {
			return kv[len(k)+1:]
		}
	}
	return ""
}

func (l *Loader) cacheFile() string {
	if l.CacheFile != "" {
		return l.CacheFile
	}
	return cacheFile()
}

func (l *Loader) client() *http.Client {
	if l.Client != nil {
		return l.Client
	}
	return &http.Client{Timeout: 15 * time.Second, CheckRedirect: httpsOnly}
}

func (l *Loader) local() []Source {
	if l.Local != nil {
		return l.Local()
	}
	return localSources(l.SetFlags, l.Environ)
}

// Load résout la configuration, avec la configuration distante en cache.
// Elle ne fait aucun accès réseau.
func (l *Loader) Load(ctx context.Context) *Config {
	opts := Options{PolicyOverride: l.getenv("LATTIS_ALLOW_POLICY_OVERRIDE") == "1"}
	defaults := &TreeSource{N: "valeurs par défaut", R: RankDefault, T: Defaults()}
	local := l.local()
	base := Resolve(ctx, opts, append([]Source{defaults}, local...)...)

	url := base.String("configUrl", "")
	var key ed25519.PublicKey
	policyAllowed := false
	if s := base.String("remote.publicKey", ""); s != "" {
		key, _ = ParsePublicKey(s) // déjà validée
		o, _ := base.Origin("remote.publicKey")
		policyAllowed = key != nil && o.Trusted
	}
	l.mu.Lock()
	l.url, l.key, l.policyAllowed = url, key, policyAllowed
	l.mu.Unlock()
	if url == "" {
		return base
	}

	status := RemoteStatus{URL: url}
	var warnings []Warning
	var remote []Source
	var remotePolicy []Source
	if cache, err := readCache(l.cacheFile()); err == nil && cache.URL == url {
		doc, err := ParseRemote([]byte(cache.Body), key, policyAllowed)
		if err != nil {
			warnings = append(warnings, Warning{Source: "configuration distante (cache)", Msg: err.Error()})
		} else {
			status.Cached, status.FetchedAt, status.Signed = true, cache.FetchedAt, doc.Signed
			name := "configuration distante " + url
			// Une configuration distante ne redirige pas vers une autre.
			for _, k := range []string{"configUrl", "remote"} {
				if x, ok := findKey(doc.Config, k); ok {
					delete(doc.Config, x)
					warnings = append(warnings, Warning{Source: name, Path: k, Msg: "ignorée dans une configuration distante"})
				}
			}
			remote = append(remote, &TreeSource{N: name, R: RankRemote, T: doc.Config})
			if doc.Policy != nil {
				remotePolicy = append(remotePolicy, &TreeSource{N: "stratégie distante " + url, R: RankPolicy, Trust: true, T: doc.Policy})
			}
		}
	}
	all := append([]Source{defaults}, remote...)
	all = append(all, remotePolicy...) // avant les stratégies locales : elles l'emportent
	all = append(all, local...)
	c := Resolve(ctx, opts, all...)
	c.Warnings = append(c.Warnings, warnings...)
	c.Remote = status
	return c
}

// Refresh télécharge la configuration distante et la met en cache si elle a
// changé et qu'elle est valide. Un document invalide ne remplace jamais le
// cache. changed : un nouveau document est en cache (Load le prendra).
func (l *Loader) Refresh(ctx context.Context) (changed bool, err error) {
	l.mu.Lock()
	url, key, policyAllowed := l.url, l.key, l.policyAllowed
	l.mu.Unlock()
	if url == "" {
		return false, nil
	}
	path := l.cacheFile()
	if path == "" {
		return false, msg.New("config.cacheDirNotFound", nil)
	}
	etag := ""
	old, _ := readCache(path)
	if old != nil && old.URL == url {
		etag = old.ETag
	} else {
		old = nil
	}
	body, newETag, err := fetchRemote(ctx, l.client(), url, etag)
	if errors.Is(err, errNotModified) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := ParseRemote(body, key, policyAllowed); err != nil {
		return false, err
	}
	if old != nil && bytes.Equal(bytes.TrimSpace([]byte(old.Body)), bytes.TrimSpace(body)) {
		return false, nil
	}
	err = writeCache(path, &remoteCache{URL: url, ETag: newETag, FetchedAt: time.Now().UTC(), Body: string(body)})
	return err == nil, err
}

// StartRefresh rafraîchit la configuration distante tout de suite puis à
// intervalle régulier (remote.refreshMinutes), jusqu'à la fin de ctx. Une
// nouvelle configuration devient la configuration courante et est passée à
// onChange (qui peut être nil).
func (l *Loader) StartRefresh(ctx context.Context, onChange func(*Config)) {
	go func() {
		for {
			if changed, err := l.Refresh(ctx); err == nil && changed {
				c := l.Load(ctx)
				SetCurrent(c)
				if onChange != nil {
					onChange(c)
				}
			}
			minutes := 60
			var s Settings
			if Current().Decode("remote", &s.Remote) == nil && s.Remote.RefreshMinutes > 0 {
				minutes = max(s.Remote.RefreshMinutes, 5)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(minutes) * time.Minute):
			}
		}
	}()
}

var current atomic.Pointer[Config]

// Current donne la configuration courante (les valeurs par défaut avant
// Init).
func Current() *Config {
	if c := current.Load(); c != nil {
		return c
	}
	return Resolve(context.Background(), Options{}, &TreeSource{N: "valeurs par défaut", R: RankDefault, T: Defaults()})
}

// SetCurrent remplace la configuration courante.
func SetCurrent(c *Config) { current.Store(c) }

// Init résout la configuration de l'application au démarrage (sans réseau)
// et lance le rafraîchissement de la configuration distante.
func Init(ctx context.Context, setFlags []string) *Loader {
	l := &Loader{SetFlags: setFlags}
	active.Store(l)
	SetCurrent(l.Load(ctx))
	l.StartRefresh(ctx, nil)
	return l
}
