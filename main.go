// Lattis : tableur relationnel local-first (Go + Wails v3 + SvelteKit).
//
// Le point d'entrée reste à la racine du module : les tâches de build Wails
// (Taskfile, bindings, embed de frontend/dist) l'attendent ici.
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"runtime"

	"github.com/lalamefine/lattis/internal/bridge"
	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/console"
	"github.com/lalamefine/lattis/internal/demo"
	"github.com/lalamefine/lattis/internal/instance"
	"github.com/lalamefine/lattis/internal/mcp"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// « lattis mcp » : relais stdio du serveur MCP vers l'application
	// ouverte, pour les clients qui lancent un programme (Claude Desktop…).
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		if err := mcp.Proxy(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// « lattis config » : configuration effective (provenance, stratégies),
	// validation et signature des fichiers de configuration d'entreprise.
	if len(os.Args) > 1 && os.Args[1] == "config" {
		console.Attach()
		os.Exit(config.Run(os.Args[2:], os.Stdout, os.Stderr))
	}

	// Configuration en cascade (stratégie, environnement, Registre, fichiers,
	// configuration distante en cache) : résolue sans réseau avant la
	// fenêtre ; la configuration distante se rafraîchit en arrière-plan.
	sets, cliArgs, err := config.SplitSetFlags(os.Args[1:])
	if err != nil {
		console.Attach()
		fmt.Fprintf(os.Stderr, "lattis : %v\n", err)
		os.Exit(2)
	}
	config.Init(context.Background(), sets)

	// --dev (développement) : l'interface est rebâtie puis servie depuis le disque.
	var ui fs.FS = assets
	cliArgs, devUI := devFrontend(cliArgs)
	if devUI != nil {
		ui = devUI
	}

	documents := &bridge.DocumentService{}
	bridge.WatchPanics(documents)
	// -h, --help : l'aide dans le terminal, sans ouvrir la fenêtre ; une
	// option inconnue l'affiche aussi, avec l'erreur.
	// -v, -vv, -vvv, --log, --log-file : journal de débogage (internal/trace) ;
	// --sql / --no-sql : journal des requêtes SQL seul.
	debug, cliArgs, err := parseDebug(cliArgs)
	if err != nil {
		console.Attach()
		fmt.Fprintf(os.Stderr, "lattis : %v\n", err)
		os.Exit(2)
	}
	var args []string
	for i, a := range cliArgs {
		// « lattis lattis://open?… » : lien vers un classeur partagé (navigateur,
		// schéma déclaré à l'installation). Une instance déjà ouverte le reçoit
		// et ce lancement s'arrête ; sinon, il l'ouvre lui-même.
		if bridge.IsDesktopLink(a) {
			if instance.Forward(a) == nil {
				return
			}
			documents.RequestLink(a)
			continue
		}
		a = bridge.FileURI(a) // file:///… : %U du .desktop (Linux)
		// « lattis [classeur] » : le classeur à ouvrir (aussi l'association
		// de fichiers, double-clic sur un .lattis sous Windows ou Linux).
		// La valeur de --demo ou --seed n'en est pas un.
		afterOpt := i > 0 && (cliArgs[i-1] == "--demo" || cliArgs[i-1] == "-demo" || cliArgs[i-1] == "--seed" || cliArgs[i-1] == "-seed")
		if path, ok := bridge.LaunchArg(a); ok && !afterOpt {
			if path == "" {
				console.Attach()
				fmt.Fprintf(os.Stderr, "lattis : classeur introuvable : %s\n", a)
				os.Exit(2)
			}
			documents.RequestOpen(path)
			continue
		}
		args = append(args, a)
	}
	preset, seed, err := demo.ParseArgs(args)
	if errors.Is(err, demo.ErrHelp) {
		console.Attach()
		fmt.Fprint(os.Stdout, demo.Usage())
		return
	}
	if err != nil {
		console.Attach()
		fmt.Fprintf(os.Stderr, "lattis : %v\n\n", err)
		fmt.Fprint(os.Stderr, demo.Usage())
		os.Exit(2)
	}
	if preset != "" {
		documents.SetDemo(preset, seed)
	}

	app := application.New(application.Options{
		Name:        "Lattis",
		Description: "Tableur relationnel local-first",
		// Journal de Wails : avertissements et erreurs seulement (pas une ligne
		// par fichier servi, « [AssetFileServerFS] Handling request ») ; tout à -vvv.
		LogLevel: debug.wailsLevel(),
		// Une panique est journalisée dans erreurs.log et signalée dans la
		// barre d'état, sans arrêter l'application.
		PanicHandler: bridge.PanicHandler,
		// Fenêtre sans cadre : le runtime JavaScript de Wails repère les bords
		// à saisir pour redimensionner. Il lit ces clés telles quelles (avec
		// le point) et prend 5 px sinon, une bande trop fine à attraper ;
		// les coins sont plus larges encore.
		Flags: map[string]any{
			"system.resizeHandleWidth":  8,
			"system.resizeHandleHeight": 8,
			"resizeCornerExtra":         16,
		},
		Services: []application.Service{
			application.NewService(&bridge.GridService{}),
			application.NewService(documents),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(ui),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Lattis",
		Width:            1280,
		Height:           800,
		MinWidth:         400,
		MinHeight:        300,
		BackgroundColour: application.NewRGB(255, 255, 255),
		URL:              "/",
		// Barre de titre dessinée par la page (frontend/src/lib/shell/TitleBar.svelte).
		// Windows garde l'ombre, les coins arrondis et le redimensionnement par les
		// bords ; sous Linux, le runtime Wails gère les bords en JavaScript.
		Frameless: runtime.GOOS != "darwin",
		// macOS : barre native masquée, feux tricolores conservés.
		Mac: application.MacWindow{
			TitleBar:                application.MacTitleBarHidden,
			InvisibleTitleBarHeight: 32,
		},
	})

	// Fermer la fenêtre passe par les écouteurs before_close du classeur
	// (évènements des scripts Lua), qui peuvent l'annuler.
	bridge.GuardClose(documents, win)

	// macOS : un .lattis ouvert depuis le Finder arrive par un événement,
	// au lancement comme pendant que l'application tourne.
	app.Event.OnApplicationEvent(events.Common.ApplicationOpenedWithFile, func(e *application.ApplicationEvent) {
		if p := e.Context().Filename(); bridge.IsWorkbook(p) {
			documents.RequestOpen(p)
		}
	})

	// Liens lattis:// transmis par un second lancement (instance.Forward) ;
	// macOS les annonce par un événement.
	if stop, err := instance.Serve(func(link string) {
		documents.RequestLink(link)
		application.InvokeSync(func() {
			win.Show()
			win.Focus()
		})
	}); err == nil {
		defer stop()
	}
	if runtime.GOOS == "darwin" {
		app.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(e *application.ApplicationEvent) {
			if u := e.Context().URL(); bridge.IsDesktopLink(u) {
				documents.RequestLink(u)
			}
		})
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
