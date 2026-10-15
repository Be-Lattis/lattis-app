// Slitable : tableur relationnel local-first (Go + Wails v3 + SvelteKit).
//
// Le point d'entrée reste à la racine du module : les tâches de build Wails
// (Taskfile, bindings, embed de frontend/dist) l'attendent ici.
package main

import (
	"embed"
	"log"

	"github.com/lalamefine/slitable/internal/bridge"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
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
		BackgroundColour: application.NewRGB(255, 255, 255),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
