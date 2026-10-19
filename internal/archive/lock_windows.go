//go:build windows

package archive

import (
	"errors"
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

// hide cache le fichier (verrou ~$ à côté du classeur).
func hide(path string) {
	if p, err := windows.UTF16PtrFromString(path); err == nil {
		if a, err := windows.GetFileAttributes(p); err == nil {
			windows.SetFileAttributes(p, a|windows.FILE_ATTRIBUTE_HIDDEN)
		}
	}
}

// processAlive dit si le processus pid existe encore sur ce poste.
func processAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return errors.Is(err, windows.ERROR_ACCESS_DENIED) // existe, mais à un autre
	}
	defer windows.CloseHandle(h)
	var code uint32
	if windows.GetExitCodeProcess(h, &code) != nil {
		return true
	}
	return code == 259 // STILL_ACTIVE
}
