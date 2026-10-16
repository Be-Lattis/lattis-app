// Slitable : tableur relationnel local-first (Go + Wails v3 + SvelteKit).
//
// Le point d'entrée reste à la racine du module : les tâches de build Wails
// (Taskfile, bindings, embed de frontend/dist) l'attendent ici.
package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/lalamefine/slitable/internal/bridge"
	"github.com/lalamefine/slitable/internal/mcp"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// « slitable mcp » : relais stdio du serveur MCP vers l'application
	// ouverte, pour les clients qui lancent un programme (Claude Desktop…).
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		if err := mcp.Proxy(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	app := application.New(application.Options{
		Name:        "Slitable",
		Description: "Tableur relationnel local-first",
		Services: []application.Service{
			application.NewService(&bridge.GridService{}),
			application.NewService(&bridge.DocumentService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Slitable",
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
