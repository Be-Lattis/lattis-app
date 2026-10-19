//go:build !windows

package archive

import "golang.org/x/sys/unix"

// freeBytes donne l'espace disponible pour l'utilisateur sur le volume de dir.
func freeBytes(dir string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
