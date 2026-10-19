//go:build windows

package archive

import "golang.org/x/sys/windows"

func hidden(path string) bool {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	a, err := windows.GetFileAttributes(p)
	return err == nil && a&windows.FILE_ATTRIBUTE_HIDDEN != 0
}
