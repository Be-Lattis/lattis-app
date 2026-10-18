//go:build windows

package bridge

import (
	"os/exec"
	"syscall"
)

// hideConsole évite la fenêtre de console d'un programme lancé par Lattis
// (commande claude).
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}
