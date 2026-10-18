//go:build !windows

package archive

import (
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
