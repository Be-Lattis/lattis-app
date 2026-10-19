//go:build !windows

package config

import (
	"os"
	"path/filepath"
	"syscall"
)

// secureFile : seul root peut modifier le fichier et son dossier.
var secureFile = func(path string) (bool, error) {
	for _, p := range []string{path, filepath.Dir(path)} {
		fi, err := os.Stat(p)
		if err != nil {
			return false, err
		}
		if fi.Mode().Perm()&0o022 != 0 {
			return false, nil
		}
		if st, ok := fi.Sys().(*syscall.Stat_t); ok && st.Uid != 0 {
			return false, nil
		}
	}
	return true, nil
}
