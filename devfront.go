//go:build !production

package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"time"
)

// devFrontend : « go run . --dev » sert l'interface depuis le serveur Vite
// (rechargement à chaud) au lieu de frontend/dist embarqué, sans rebâtir
// Svelte. Vite est lancé s'il ne tourne pas déjà (port 9245, celui de
// wails3 dev), puis arrêté avec l'application. Option de développement
// seulement : ni dans --help ni dans l'aide, absente de la version livrée.
func devFrontend(args []string) ([]string, func()) {
	i := slices.Index(args, "--dev")
	if i < 0 {
		return args, func() {}
	}
	args = slices.Delete(slices.Clone(args), i, i+1)
	if os.Getenv("FRONTEND_DEVSERVER_URL") != "" { // wails3 dev
		return args, func() {}
	}
	const port = 9245
	url := "http://localhost:" + strconv.Itoa(port)
	var cmd *exec.Cmd
	if !reachable(url) {
		cmd = exec.Command("npm", "run", "dev", "--", "--port", strconv.Itoa(port), "--strictPort")
		cmd.Dir = "frontend"
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "lattis --dev : Vite ne démarre pas (%v) ; lancez « npm run dev » dans frontend\n", err)
			os.Exit(1)
		}
		for deadline := time.Now().Add(90 * time.Second); !reachable(url); time.Sleep(300 * time.Millisecond) {
			if time.Now().After(deadline) {
				stop(cmd)
				fmt.Fprintln(os.Stderr, "lattis --dev : Vite ne répond pas sur "+url)
				os.Exit(1)
			}
		}
	}
	os.Setenv("FRONTEND_DEVSERVER_URL", url)
	return args, func() { stop(cmd) }
}

func reachable(url string) bool {
	c := http.Client{Timeout: 500 * time.Millisecond}
	r, err := c.Get(url)
	if err != nil {
		return false
	}
	r.Body.Close()
	return true
}

// stop arrête Vite lancé par devFrontend, avec ses processus enfants.
func stop(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
		return
	}
	_ = cmd.Process.Kill()
}
