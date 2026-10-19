// lattis-engine sert le moteur Lattis (paquet engine) et le frontend web à
// un navigateur, sans Wails. Deux usages :
//
//   - essai local de la version web :
//
//     cd frontend && npm run build:web
//     go run -tags web ./cmd/lattis-engine --demo med
//     → http://127.0.0.1:8102/
//
//   - mode serveur, lancé par lattisd (un processus par session
//     d'utilisateur, édition web) : --server, --token-url (et le secret
//     LATTIS_ENGINE_SECRET), --base-path, --dir, --workbook. Il écrit
//     « LISTENING 127.0.0.1:<port> » sur la sortie standard quand il écoute,
//     et s'arrête quand l'entrée standard se ferme ou après --idle sans
//     onglet connecté.
//
// Voir docs/collaboration/version-web.md.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/lalamefine/lattis/engine"
	"github.com/lalamefine/lattis/internal/archive"
	"github.com/lalamefine/lattis/internal/bridge"
	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/demo"
	"github.com/lalamefine/lattis/internal/remote/auth"
)

const usage = `lattis-engine : moteur Lattis pour le navigateur

  --addr hôte:port   adresse d'écoute (127.0.0.1:8102 ; 127.0.0.1:0 en mode serveur)
  --static dossier   frontend web à servir (frontend/dist-web, npm run build:web) ;
                     par défaut le frontend embarqué s'il y en a un
  --dir dossier      répertoire de la session (fichiers envoyés, téléchargements,
                     copies des classeurs du serveur)
  --base-path /app   préfixe des URL (/app/ws, /app/upload/…, /app/download/…)
  --scripting        permet Lua, boutons et pages web (désactivés par défaut)
  --internal-hosts h1,h2
                     hôtes internes (bouclage, réseau privé) que les requêtes et
                     les scripts peuvent joindre ; les autres sont refusés
  --demo [preset]    ouvre un classeur de démonstration (med par défaut)
  --seed n           graine du générateur de la démonstration

Mode serveur (lancé par lattisd) :
  --server url       Serveur Lattis de la session (émetteur) : le seul joignable
  --token-url url    jeton d'accès de l'utilisateur : GET, Authorization: Bearer
                     $LATTIS_ENGINE_SECRET → {"access_token", "pg_role", "expires_in"}
  --workbook id      classeur du serveur à ouvrir au démarrage
  --idle durée       arrêt après cette durée sans onglet connecté (30m)

En mode serveur, « LISTENING hôte:port » est écrit sur la sortie standard dès
l'écoute, et le moteur s'arrête quand l'entrée standard se ferme.
`

func main() {
	addr, static, dir, base, scripting := "", "", "", "", false
	server, tokenURL, workbook := "", "", ""
	var internal []string
	idle := 30 * time.Minute
	var rest []string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		name, value, inline := strings.Cut(args[i], "=")
		val := func() string {
			if inline {
				return value
			}
			if i+1 < len(args) {
				i++
				return args[i]
			}
			fail(fmt.Errorf("%s attend une valeur", name))
			return ""
		}
		switch name {
		case "--addr", "-addr":
			addr = val()
		case "--static", "-static":
			static = val()
		case "--dir", "-dir":
			dir = val()
		case "--base-path", "-base-path":
			base = "/" + strings.Trim(val(), "/")
			if base == "/" {
				base = ""
			}
		case "--scripting", "-scripting":
			scripting = true
		case "--internal-hosts", "-internal-hosts":
			for _, h := range strings.Split(val(), ",") {
				if h = strings.TrimSpace(h); h != "" {
					internal = append(internal, h)
				}
			}
		case "--server", "-server":
			server = val()
		case "--token-url", "-token-url":
			tokenURL = val()
		case "--workbook", "-workbook":
			workbook = val()
		case "--idle", "-idle":
			v := val()
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				fail(fmt.Errorf("--idle attend une durée (30m, 1h), pas « %s »", v))
			}
			idle = d
		default:
			rest = append(rest, args[i])
		}
	}
	preset, seed, err := demo.ParseArgs(rest)
	if errors.Is(err, demo.ErrHelp) {
		fmt.Print(usage)
		return
	}
	if err != nil {
		fail(err)
	}
	serverMode := server != ""
	if serverMode && tokenURL == "" {
		fail(errors.New("--server demande --token-url"))
	}
	if workbook != "" && !serverMode {
		fail(errors.New("--workbook demande --server"))
	}
	if addr == "" {
		addr = "127.0.0.1:8102"
		if serverMode {
			addr = "127.0.0.1:0"
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if dir == "" {
		if dir, err = os.MkdirTemp("", "lattis-engine-"); err != nil {
			fail(err)
		}
	}
	if serverMode {
		// Tout ce que la session écrit reste dans son répertoire : copies de
		// travail (récupération), copies des classeurs du serveur.
		os.Setenv(archive.WorkEnv, filepath.Join(dir, "work"))
		bridge.SetShareCacheDir(filepath.Join(dir, "partages"))
		bridge.HidePaths = true
		if server, err = auth.NormalizeURL(server); err != nil {
			fail(err)
		}
		// Le Serveur Lattis de la session est joignable même s'il est interne.
		if u, err := url.Parse(server); err == nil && u.Host != "" {
			internal = append(internal, u.Host)
		}
		bridge.UseFixedSession(&bridge.FixedSession{
			Server: server,
			Fetch:  bridge.TokenURL(tokenURL, os.Getenv("LATTIS_ENGINE_SECRET"), nil),
		})
		// L'entrée standard fermée (lattisd arrêté ou session finie) : arrêt.
		go func() {
			io.Copy(io.Discard, os.Stdin)
			stop()
		}()
	}
	config.Init(ctx, nil)

	opts := engine.Options{Dir: dir, BasePath: base, Scripting: scripting, Demo: preset, Seed: seed, InternalHosts: internal}
	if !serverMode {
		opts.OriginPatterns = []string{"localhost:*", "127.0.0.1:*"}
	}
	switch {
	case static != "":
		if st, err := os.Stat(filepath.Join(static, "index.html")); err != nil || st.IsDir() {
			fail(fmt.Errorf("frontend web introuvable dans %s : lancez « npm run build:web » dans frontend", static))
		}
		opts.Static = os.DirFS(static)
	case embeddedWeb() != nil:
		opts.Static = embeddedWeb()
	case !serverMode:
		if st, err := os.Stat(filepath.Join("frontend", "dist-web", "index.html")); err == nil && !st.IsDir() {
			opts.Static = os.DirFS(filepath.Join("frontend", "dist-web"))
		}
	}
	if opts.Static != nil {
		if _, err := fs.Stat(opts.Static, "index.html"); err != nil {
			fail(errors.New("frontend web embarqué incomplet (index.html absent)"))
		}
	}
	e, err := engine.New(ctx, opts)
	if err != nil {
		fail(err)
	}
	defer e.Close()
	if workbook != "" {
		if _, err := e.Documents().ShareOpen(ctx, server, workbook, ""); err != nil {
			// Le classeur reste à ouvrir depuis l'interface (« Ouvrir depuis un
			// serveur ») : l'erreur est journalisée, la session démarre.
			log.Printf("lattis-engine : ouverture du classeur %s : %v", workbook, err)
		}
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fail(err)
	}
	srv := &http.Server{Handler: e.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	if serverMode {
		fmt.Fprintf(os.Stdout, "LISTENING %s\n", ln.Addr())
		go watchIdle(ctx, e, idle, stop)
	} else {
		log.Printf("Lattis (web) : http://%s%s/  (session : %s)", ln.Addr(), base, dir)
	}
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fail(err)
	}
}

// idleTick : fréquence du contrôle d'inactivité.
var idleTick = 15 * time.Second

// watchIdle arrête le moteur après idle sans onglet connecté (le délai
// court dès le démarrage : lattisd attend la connexion du navigateur).
func watchIdle(ctx context.Context, e *engine.Engine, idle time.Duration, stop func()) {
	last := time.Now()
	t := time.NewTicker(min(idleTick, idle))
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			if e.Connected() {
				last = now
			} else if now.Sub(last) >= idle {
				log.Printf("lattis-engine : aucun onglet depuis %s, arrêt", idle)
				stop()
				return
			}
		}
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "lattis-engine : %v\n\n%s", err, usage)
	os.Exit(2)
}
