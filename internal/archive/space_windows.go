//go:build windows

package archive

import "golang.org/x/sys/windows"

// freeBytes donne l'espace disponible pour l'utilisateur sur le volume de dir.
func freeBytes(dir string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var free, total, all uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, &all); err != nil {
		return 0, err
	}
	return free, nil
}
