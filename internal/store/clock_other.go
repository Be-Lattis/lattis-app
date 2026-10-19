//go:build !windows

package store

import "time"

var origin = time.Now()

// clock donne un instant pour since.
func clock() int64 { return int64(time.Since(origin)) }

// since donne la durée écoulée depuis l'instant start (voir clock).
func since(start int64) time.Duration { return time.Duration(clock() - start) }
