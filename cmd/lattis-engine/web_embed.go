//go:build embedweb

package main

import (
	"embed"
	"io/fs"
)

// Frontend web embarqué (image du moteur pour lattisd) : frontend/dist-web
// copié dans cmd/lattis-engine/dist-web avant la compilation
// (go build -tags web,embedweb ; voir build/engine/Dockerfile).
//
//go:embed all:dist-web
var embedded embed.FS

func embeddedWeb() fs.FS {
	sub, err := fs.Sub(embedded, "dist-web")
	if err != nil {
		return nil
	}
	return sub
}
