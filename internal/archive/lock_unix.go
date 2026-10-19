//go:build !windows

package archive

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// tryLock pose un verrou exclusif non bloquant sur le fichier ; il disparaît
// avec le processus, même tué.
func tryLock(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
}

// syncDir met sur disque le répertoire (le renommage d'un enregistrement).
func syncDir(dir string) {
	if f, err := os.Open(dir); err == nil {
		f.Sync()
		f.Close()
	}
}

// hide : rien hors de Windows (seul un nom commençant par un point y est
// caché).
func hide(string) {}

// processAlive dit si le processus pid existe encore sur ce poste.
func processAlive(pid int) bool {
	err := unix.Kill(pid, 0)
	return err == nil || errors.Is(err, unix.EPERM)
}
