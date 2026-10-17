// Lattis : tableur relationnel local-first (Go + Wails v3 + SvelteKit).
//
// Le point d'entrée reste à la racine du module : les tâches de build Wails
// (Taskfile, bindings, embed de frontend/dist) l'attendent ici.
package main

import (
	"embed"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"runtime"

	"github.com/lalamefine/lattis/internal/bridge"
	"github.com/lalamefine/lattis/internal/console"
	"github.com/lalamefine/lattis/internal/demo"
	"github.com/lalamefine/lattis/internal/mcp"
	"github.com/lalamefine/lattis/internal/store"
	"github.com/wailsapp/wails/v3/pkg/application"
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

	documents := &bridge.DocumentService{}
	// -h, --help : l'aide dans le terminal, sans ouvrir la fenêtre ; une
	// option inconnue l'affiche aussi, avec l'erreur.
	// --sql / --no-sql : journal des requêtes SQL dans la console.
	var args []string
	for _, a := range os.Args[1:] {
		switch a {
		case "--sql", "-sql":
			store.SetSQLLog(true)
			console.Attach()
		case "--no-sql", "-no-sql":
			store.SetSQLLog(false)
		default:
			args = append(args, a)
		}
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
		// par fichier servi, « [AssetFileServerFS] Handling request »).
		LogLevel: slog.LevelWarn,
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

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Lattis",
		Width:            1280,
		Height:           800,
		MinWidth:         640,
		MinHeight:        400,
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

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
