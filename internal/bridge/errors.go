package bridge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Erreurs de l'interface (plantage d'un composant, promesse rejetée sans
// suite) : écrites dans la console et dans erreurs.log du poste, avec la
// pile et les dernières actions de l'utilisateur, pour que le journal SQL
// ne soit plus la seule trace (demande de Louis).

// maxErrorLog : au-delà, le journal repart de zéro (l'ancien est gardé en .1).
const maxErrorLog = 1 << 20

var errorLogMu sync.Mutex

// errorLogFile est le journal des erreurs ; surchargeable pour les tests.
var errorLogFile = func() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Lattis", "erreurs.log"), nil
}

// ReportError journalise une erreur de l'interface : où (composant, action),
// message, pile et dernières actions (une par ligne). Rend le chemin du
// journal ("" s'il n'a pas pu être écrit).
func (s *DocumentService) ReportError(ctx context.Context, where, message, stack, trail string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== %s · %s\n%s\n", time.Now().Format("2006-01-02 15:04:05"), where, message)
	if stack != "" {
		fmt.Fprintf(&b, "Pile :\n%s\n", strings.TrimRight(stack, "\n"))
	}
	if trail != "" {
		fmt.Fprintf(&b, "Dernières actions :\n%s\n", strings.TrimRight(trail, "\n"))
	}
	entry := b.String()
	fmt.Fprint(os.Stderr, "[ERREUR interface] "+entry)
	p, err := errorLogFile()
	if err != nil {
		return ""
	}
	errorLogMu.Lock()
	defer errorLogMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return ""
	}
	if st, err := os.Stat(p); err == nil && st.Size() > maxErrorLog {
		_ = os.Rename(p, p+".1")
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return ""
	}
	defer f.Close()
	if _, err := f.WriteString(entry + "\n"); err != nil {
		return ""
	}
	return p
}
