//go:build !windows

package bridge

import "os/exec"

// hideConsole : rien à cacher hors de Windows.
func hideConsole(*exec.Cmd) {}
