package console

import (
	"os"
	"syscall"
)

// Attach rattache le programme à la console qui l'a lancé : l'exécutable
// Windows est construit sans console (-H windowsgui), et --help doit pourtant
// écrire dans le terminal.
func Attach() {
	const attachParent = ^uintptr(0) // ATTACH_PARENT_PROCESS
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole")
	if r, _, _ := proc.Call(attachParent); r == 0 {
		return // déjà une console (go run), ou lancé hors terminal
	}
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout, os.Stderr = f, f
	}
}
