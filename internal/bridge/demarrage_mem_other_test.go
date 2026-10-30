//go:build !windows

package bridge

import "syscall"

// peakRSS : pic de la mémoire du processus (ru_maxrss : Ko sous Linux).
func peakRSS() uint64 {
	var r syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &r) != nil {
		return 0
	}
	return uint64(r.Maxrss) << 10
}
