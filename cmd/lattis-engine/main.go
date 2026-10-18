// lattis-engine sert le moteur Lattis (paquet engine) et le frontend web à
// un navigateur, sans Wails : l'essai local de la version web. En
// production, lattisd embarque le paquet engine (une session par
// utilisateur) ; voir docs/collaboration/version-web.md.
//
//	cd frontend && npm run build:web
//	go run -tags web ./cmd/lattis-engine --demo med
//	→ http://127.0.0.1:8102/
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/lalamefine/lattis/engine"
	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/demo"
)

const usage = `lattis-engine : moteur Lattis pour le navigateur (essai local de la version web)

  --addr hôte:port   adresse d'écoute (127.0.0.1:8102)
  --static dossier   frontend web à servir (frontend/dist-web, npm run build:web)
  --dir dossier      répertoire de la session (fichiers envoyés, téléchargements)
  --scripting        permet Lua, boutons et pages web (désactivés par défaut)
  --demo [preset]    ouvre un classeur de démonstration (med par défaut)
  --seed n           graine du générateur de la démonstration
`

func main() {
	addr, static, dir, scripting := "127.0.0.1:8102", "frontend/dist-web", "", false
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
		case "--scripting", "-scripting":
			scripting = true
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	config.Init(ctx, nil)

	if dir == "" {
		if dir, err = os.MkdirTemp("", "lattis-engine-"); err != nil {
			fail(err)
		}
	}
	opts := engine.Options{Dir: dir, Scripting: scripting, Demo: preset, Seed: seed,
		OriginPatterns: []string{"localhost:*", "127.0.0.1:*"}}
	if static != "" {
		if st, err := os.Stat(filepath.Join(static, "index.html")); err != nil || st.IsDir() {
			fail(fmt.Errorf("frontend web introuvable dans %s : lancez « npm run build:web » dans frontend", static))
		}
		opts.Static = os.DirFS(static)
	}
	e, err := engine.New(ctx, opts)
	if err != nil {
		fail(err)
	}
	defer e.Close()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fail(err)
	}
	srv := &http.Server{Handler: e.Handler()}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	log.Printf("Lattis (web) : http://%s/  (session : %s)", ln.Addr(), dir)
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "lattis-engine : %v\n\n%s", err, usage)
	os.Exit(2)
}
