package canvas

import (
	"sync/atomic"
	"time"
)

var lastUniq atomic.Int64

// uniq rend un nombre strictement croissant, tiré de l'horloge, pour nommer
// les tables de corbeille (_lattis_trash_…) : deux appels dans la même
// transaction ne donnent jamais le même nom, même quand l'horloge de Windows
// rend deux fois la même valeur.
func uniq() int64 {
	for {
		now, last := time.Now().UnixNano(), lastUniq.Load()
		if now <= last {
			now = last + 1
		}
		if lastUniq.CompareAndSwap(last, now) {
			return now
		}
	}
}
