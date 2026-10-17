//go:build windows

package store

import (
	"syscall"
	"time"
	"unsafe"
)

// Sous Windows, l'horloge monotone de Go avance par pas de 0,5 à 15 ms :
// les requêtes rapides y duraient « 0 µs ». Le compteur haute résolution
// (QueryPerformanceCounter) mesure à la microseconde.
var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	qpc      = kernel32.NewProc("QueryPerformanceCounter")
	qpf      = kernel32.NewProc("QueryPerformanceFrequency")
	qpcFreq  = func() int64 {
		var f int64
		if r, _, _ := qpf.Call(uintptr(unsafe.Pointer(&f))); r == 0 || f <= 0 {
			return 0
		}
		return f
	}()
)

// clock donne un instant pour since.
func clock() int64 {
	if qpcFreq == 0 {
		return time.Now().UnixNano()
	}
	var c int64
	qpc.Call(uintptr(unsafe.Pointer(&c)))
	return c
}

// since donne la durée écoulée depuis l'instant start (voir clock).
func since(start int64) time.Duration {
	if qpcFreq == 0 {
		return time.Duration(time.Now().UnixNano() - start)
	}
	d := clock() - start
	return time.Duration(d/qpcFreq)*time.Second + time.Duration(d%qpcFreq*int64(time.Second)/qpcFreq)
}
