// Package engine est le moteur Lattis sans Wails : les services de
// internal/bridge (GridService, DocumentService) servis en JSON-RPC sur
// WebSocket, pour la version web (spécification collaboration § 12).
//
// lattisd l'embarque : un Engine par session d'utilisateur, dont il monte
// Handler derrière son authentification (cookie de session, anti-CSRF).
// cmd/lattis-engine en fait un petit serveur pour l'essayer en local.
// Compilé avec -tags web, il n'importe pas Wails.
//
// Voir docs/collaboration/version-web.md.
package engine

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/lalamefine/lattis/internal/bridge"
	"github.com/lalamefine/lattis/internal/keychain"
	"github.com/lalamefine/lattis/internal/query"
	"github.com/lalamefine/lattis/internal/transport"
)

// Options règle un moteur.
type Options struct {
	// Dir est le répertoire de la session : fichiers envoyés par le
	// navigateur, fichiers enregistrés à télécharger. Obligatoire.
	Dir string
	// BasePath est le préfixe d'URL où Handler est monté (« /engine »), ""
	// pour la racine : /ws, /upload/…, /download/….
	BasePath string
	// Scripting permet Lua, boutons et pages web (server_config.web_scripting,
	// lu par lattisd à l'ouverture de la session). Faux par défaut.
	Scripting bool
	// InternalHosts : hôtes que les requêtes (et les scripts) peuvent
	// joindre bien qu'ils désignent une adresse interne du serveur
	// (bouclage, réseau privé, lien local), « hôte » ou « hôte:port ». Les
	// autres adresses internes sont refusées, et les sources fichier ne
	// lisent que les fichiers de Dir (query.Guard).
	InternalHosts []string
	// Demo et Seed : classeur de démonstration à ouvrir (comme --demo).
	Demo string
	Seed uint64
	// Secrets : coffre de la session (fourni par lattisd, chiffré côté
	// serveur) ; nil : en mémoire, perdu à la fermeture.
	Secrets bridge.SecretStore
	// Static sert le frontend web (sortie de npm run build:web) à la
	// racine ; nil : lattisd le sert lui-même.
	Static fs.FS
	// OriginPatterns : origines permises pour le WebSocket en plus de la
	// même origine (développement : « localhost:* »).
	OriginPatterns []string
	// PublicDemo : démo publique, jetable (lattis-engine --public-demo) :
	// ni scripting, ni connexion sortante (InternalHosts ignoré), ni Serveur
	// Lattis ; Enregistrer télécharge le classeur ; Close efface Dir.
	PublicDemo bool
}

// Engine est un moteur Lattis : un classeur ouvert, ses services et leurs
// connexions.
type Engine struct {
	grid *bridge.GridService
	docs *bridge.DocumentService
	host *WebHost
	hub  *transport.Hub
	mux  *http.ServeMux
	// dispose : répertoire de la session à effacer à la fermeture (démo
	// publique).
	dispose string
}

// New crée et démarre un moteur.
func New(ctx context.Context, o Options) (*Engine, error) {
	if o.Dir == "" {
		return nil, errors.New("engine : Options.Dir manquant")
	}
	// Absolu : les chemins reçus du navigateur sont comparés à Dir
	// (query.Guard.CheckPath).
	dir, err := filepath.Abs(o.Dir)
	if err != nil {
		return nil, err
	}
	o.Dir = dir
	if err := os.MkdirAll(o.Dir, 0o700); err != nil {
		return nil, err
	}
	base := strings.TrimSuffix(o.BasePath, "/")
	e := &Engine{grid: &bridge.GridService{}, docs: &bridge.DocumentService{}}
	reg, err := transport.NewRegistry(e.grid, e.docs)
	if err != nil {
		return nil, err
	}
	e.hub = transport.NewHub(reg)
	e.hub.Accept = &websocket.AcceptOptions{OriginPatterns: o.OriginPatterns}
	// Messages du moteur dans la langue de la session (internal/msg).
	e.hub.Translate = bridge.ErrorTranslator(e.docs)
	secrets := o.Secrets
	if secrets == nil {
		secrets = &bridge.MemSecrets{}
	}
	// Mots de passe des connexions et clés d'API : dans ce coffre, jamais
	// dans le trousseau du compte système du serveur.
	keychain.Use(sessionSecrets{secrets})
	guard := &query.Guard{Dir: o.Dir, Hosts: o.InternalHosts}
	if o.PublicDemo {
		// Rien ne sort d'une démo publique : ni script, ni connexion.
		o.Scripting = false
		// Démo publique : aucune connexion sortante, sauf le fond de carte.
		guard = &query.Guard{Dir: o.Dir, Offline: true, OfflineExcept: []string{"tile.openstreetmap.org"}}
		bridge.UsePublicDemo(true)
		e.dispose = o.Dir
		// Fin de la démo annoncée par lattisd (bridge.SetDemoDeadline) :
		// passé l'heure, plus de modification, lecture et téléchargement
		// seulement.
		e.hub.Guard = func(m *transport.Method) error { return bridge.DemoGuard(m.Short) }
	}
	e.host = &WebHost{
		hub: e.hub, dir: o.Dir, base: base, scripting: o.Scripting, demo: o.PublicDemo, secrets: secrets,
		picks: map[string]chan pickResult{}, downloads: map[string]string{},
		guard: guard,
	}
	e.hub.OnIdle = e.host.cancelPicks
	bridge.UseHost(e.host, e.grid, e.docs)
	if o.Demo != "" {
		e.docs.SetDemo(o.Demo, o.Seed)
	}
	// Pas de démonstration d'un million de lignes (GridService) : réservée
	// à la mesure du bureau.
	if err := bridge.Start(ctx, nil, e.docs); err != nil {
		return nil, err
	}

	e.mux = http.NewServeMux()
	e.mux.Handle("GET "+base+"/ws", e.hub)
	e.mux.HandleFunc("POST "+base+"/upload/{id}", e.host.serveUpload)
	e.mux.HandleFunc("GET "+base+"/download/{token}", e.host.serveDownload)
	if o.Static != nil {
		e.mux.Handle("GET "+base+"/", http.StripPrefix(base, spa(o.Static, base)))
		if base != "" {
			// /app → /app/ : les chemins relatifs de la page (ressources, ws)
			// partent du préfixe.
			e.mux.Handle("GET "+base, http.RedirectHandler(base+"/", http.StatusMovedPermanently))
		}
	}
	return e, nil
}

// Handler sert le WebSocket /ws, l'envoi et le téléchargement de fichiers,
// et le frontend si Options.Static est donné.
func (e *Engine) Handler() http.Handler { return e.mux }

// Documents donne le service des classeurs (lattisd : ouvrir un classeur du
// serveur au démarrage de la session…).
func (e *Engine) Documents() *bridge.DocumentService { return e.docs }

// Emit pousse un évènement aux onglets connectés.
func (e *Engine) Emit(name string, data any) bool { return e.hub.Emit(name, data) }

// Connected dit si un onglet est connecté (lattisd : fermer le moteur après
// une durée d'inactivité).
func (e *Engine) Connected() bool { return e.hub.Connected() }

// Close ferme le classeur et les services. Un classeur modifié et non
// enregistré reste dans le répertoire de travail (récupération), sauf en
// démo publique : tout le répertoire de la session est effacé.
func (e *Engine) Close() error {
	e.host.cancelPicks()
	err := errors.Join(e.docs.ServiceShutdown(), e.grid.ServiceShutdown())
	if e.dispose != "" {
		err = errors.Join(err, removeAll(e.dispose))
	}
	return err
}

// removeAll efface dir ; quelques essais, le temps qu'un fichier tout juste
// fermé soit relâché (Windows).
func removeAll(dir string) error {
	var err error
	for i := 0; i < 10; i++ {
		if err = os.RemoveAll(dir); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

// spa sert les fichiers du frontend, et index.html pour les autres chemins
// (application d'une seule page : adapter-static, fallback index.html).
// Sous un préfixe (--base-path /app, lattisd), la page de repli de
// SvelteKit cite ses ressources en absolu (/_app/…) et une base vide : elles
// sont réécrites sous le préfixe (/app/_app/…, base "/app"), sinon la page
// reste blanche (ressources en 404, lattisd ne route que /app/*).
func spa(files fs.FS, base string) http.Handler {
	fsrv := http.FileServerFS(files)
	var once sync.Once
	var page []byte
	index := func(w http.ResponseWriter, r *http.Request) bool {
		if base == "" {
			return false
		}
		once.Do(func() {
			if b, err := fs.ReadFile(files, "index.html"); err == nil {
				page = rebase(b, base)
			}
		})
		if page == nil {
			return false
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		if r.Method != http.MethodHead {
			w.Write(page)
		}
		return true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if p == "" || p == "index.html" {
			if index(w, r) {
				return
			}
		} else if st, err := fs.Stat(files, p); err != nil || st.IsDir() {
			if index(w, r) {
				return
			}
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fsrv.ServeHTTP(w, r2)
			return
		}
		fsrv.ServeHTTP(w, r)
	})
}

// rebase place sous le préfixe base les chemins absolus de la page de repli
// (/_app/…) et sa base SvelteKit (base: "").
func rebase(page []byte, base string) []byte {
	s := strings.ReplaceAll(string(page), `"/_app/`, `"`+base+`/_app/`)
	s = strings.Replace(s, `base: ""`, `base: "`+base+`"`, 1)
	return []byte(s)
}
