package bridge

import (
	"fmt"
	"github.com/lalamefine/lattis/internal/msg"
	"os"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"
)

// Paniques du Go (audit 6, M7) : Wails transforme celle d'une méthode liée
// en erreur de l'appel, mais ne l'écrivait que dans son propre journal, et
// une panique dans une goroutine du paquet arrêtait l'application. Toutes
// passent par ReportPanic : erreurs.log (avec la pile) et un message lisible
// dans la barre d'état.

// panicDocs : service qui affiche le message (WatchPanics).
var panicDocs atomic.Pointer[DocumentService]

// WatchPanics désigne le service qui prévient l'utilisateur d'une panique.
func WatchPanics(s *DocumentService) { panicDocs.Store(s) }

// ReportPanic journalise une panique (où, valeur, pile) et prévient
// l'utilisateur. Rend le chemin du journal ("" s'il n'a pas pu être écrit).
func ReportPanic(where string, v any, stack string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== %s · panique · %s\n%v\n", time.Now().Format("2006-01-02 15:04:05"), where, v)
	if stack != "" {
		fmt.Fprintf(&b, "Pile :\n%s\n", strings.TrimRight(stack, "\n"))
	}
	entry := b.String()
	fmt.Fprint(os.Stderr, "[PANIQUE] "+entry)
	p := writeErrorLog(entry)
	if s := panicDocs.Load(); s != nil {
		text := s.say("panic.interrupted", msg.Args{"where": where})
		if p != "" {
			text += s.say("panic.logFile", msg.Args{"path": p})
		}
		host{s}.Notify(text)
	}
	return p
}

// recoverPanic se place en tête d'une goroutine (defer recoverPanic("…")) :
// une panique y est journalisée au lieu d'arrêter l'application.
func recoverPanic(where string) {
	if v := recover(); v != nil {
		ReportPanic(where, v, string(debug.Stack()))
	}
}
