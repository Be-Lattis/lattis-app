//go:build windows

package archive

import (
	"os"

	"golang.org/x/sys/windows"
)

// tryLock pose un verrou exclusif non bloquant sur le fichier ; il disparaît
// avec le processus, même tué.
func tryLock(f *os.File) error {
	ol := new(windows.Overlapped)
	return windows.LockFileEx(windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
}

// syncDir : sous Windows, MoveFileEx est journalisé par NTFS, un répertoire
// ne se synchronise pas.
func syncDir(string) {}
