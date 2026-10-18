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
	"log"
	"os"
	"runtime"

	"github.com/lalamefine/lattis/internal/bridge"
	"github.com/lalamefine/lattis/internal/config"
	"github.com/lalamefine/lattis/internal/console"
	"github.com/lalamefine/lattis/internal/demo"
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

	documents := &bridge.DocumentService{}
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
			Handler: application.AssetFileServerFS(assets),
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

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
