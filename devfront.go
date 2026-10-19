//go:build !production

package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"slices"
)

// devFrontend : « go run . --dev » rebâtit l'interface (npm run build dans
// frontend) avant de démarrer et la sert depuis frontend/dist sur le disque :
// go run embarque le dist de la compilation, qui serait périmé. Option de
// développement seulement : ni dans --help ni dans l'aide, absente de la
// version livrée. Rend nil sans --dev (le dist embarqué sert).
func devFrontend(args []string) ([]string, fs.FS) {
	i := slices.Index(args, "--dev")
	if i < 0 {
		return args, nil
	}
	args = slices.Delete(slices.Clone(args), i, i+1)
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = "frontend"
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "lattis --dev : npm run build a échoué (%v)\n", err)
		os.Exit(1)
	}
	return args, os.DirFS("frontend/dist")
}
