// Package trace est le journal de débogage de Lattis, réglé au lancement :
//
//	-v     ce que fait l'application : classeurs (ouverture, enregistrement,
//	       récupération), modifications (étapes d'annulation), scripts,
//	       évènements
//	-vv    en plus, chaque requête SQL jouée contre le SQLite local
//	-vvv   en plus, les paramètres des requêtes, le détail des évènements et
//	       des recalculs, journal interne de Wails
//
// --log=sql,doc limite le journal à ces catégories, --log-file=chemin
// l'écrit aussi dans un fichier. Les lignes vont sur la sortie d'erreur,
// préfixées de l'heure et de la catégorie : « 21:42:07.315 [DOC] … ».
package trace

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Catégories du journal.
const (
	Doc    = "DOC"    // classeurs : ouverture, enregistrement, récupération, fermeture
	Edit   = "EDIT"   // modifications : étapes d'annulation, annuler, rétablir
	Script = "SCRIPT" // scripts Lua : exécution, durée, erreur
	Event  = "EVENT"  // évènements du classeur et leurs écouteurs
	Calc   = "CALC"   // recalcul des noms et formules
	SQL    = "SQL"    // requêtes SQL (écrites par le paquet store)
)

// Categories liste les catégories, pour l'aide et la validation de --log.
var Categories = []string{Doc, Edit, Script, Event, Calc, SQL}

var (
	level atomic.Int32
	mu    sync.Mutex
	only  map[string]bool // nil : toutes
	out   io.Writer       = os.Stderr
)

// SetLevel fixe la verbosité (0 : silencieux, 1 à 3 : -v à -vvv).
func SetLevel(n int) { level.Store(int32(max(0, min(n, 3)))) }

// Level donne la verbosité.
func Level() int { return int(level.Load()) }

// Only limite le journal aux catégories données (vide : toutes). Une
// catégorie inconnue est une erreur.
func Only(cats []string) error {
	mu.Lock()
	defer mu.Unlock()
	if len(cats) == 0 {
		only = nil
		return nil
	}
	m := map[string]bool{}
	for _, c := range cats {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		known := false
		for _, k := range Categories {
			known = known || k == c
		}
		if !known {
			return fmt.Errorf("catégorie de journal inconnue : %s (au choix : %s)", strings.ToLower(c), strings.ToLower(strings.Join(Categories, ", ")))
		}
		m[c] = true
	}
	if len(m) == 0 {
		m = nil
	}
	only = m
	return nil
}

// Allowed dit si la catégorie n'est pas exclue par --log.
func Allowed(cat string) bool {
	mu.Lock()
	defer mu.Unlock()
	return only == nil || only[cat]
}

// On dit si une ligne de niveau n de la catégorie cat est écrite.
func On(n int, cat string) bool { return Level() >= n && Allowed(cat) }

// ToFile écrit aussi le journal dans un fichier (ajouté à la fin) ; la sortie
// d'erreur le reçoit toujours.
func ToFile(path string) (io.Closer, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	mu.Lock()
	out = io.MultiWriter(os.Stderr, f)
	mu.Unlock()
	fmt.Fprintf(f, "--- Lattis, %s ---\n", time.Now().Format("2006-01-02 15:04:05"))
	return f, nil
}

// Writer est la sortie du journal (sortie d'erreur, et le fichier de
// --log-file), pour un journal qui écrit lui-même ses lignes.
func Writer() io.Writer {
	mu.Lock()
	defer mu.Unlock()
	return out
}

// Logf écrit une ligne de niveau n dans la catégorie cat.
func Logf(n int, cat, format string, args ...any) {
	if !On(n, cat) {
		return
	}
	line := fmt.Sprintf("%s [%s] %s\n", time.Now().Format("15:04:05.000"), cat, fmt.Sprintf(format, args...))
	mu.Lock()
	io.WriteString(out, line)
	mu.Unlock()
}

// Since écrit une ligne de niveau n terminée par la durée depuis start et,
// s'il y en a une, l'erreur : à appeler en defer.
//
//	defer trace.Since(1, trace.Doc, time.Now(), &err, "ouverture de %s", path)
func Since(n int, cat string, start time.Time, err *error, format string, args ...any) {
	if !On(n, cat) {
		return
	}
	msg := fmt.Sprintf(format, args...) + " (" + Dur(time.Since(start)) + ")"
	if err != nil && *err != nil {
		msg += " : ERREUR " + (*err).Error()
	}
	Logf(n, cat, "%s", msg)
}

// Dur écrit une durée lisible : 850 µs, 12.4 ms, 1.20 s.
func Dur(d time.Duration) string {
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%.2f s", d.Seconds())
	case d >= time.Millisecond:
		return fmt.Sprintf("%.1f ms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%d µs", d.Microseconds())
}

// ParseVerbose lit une option de verbosité : -v, -vv, -vvv (n = nombre de
// v), --verbose (1) ou --verbose=n. ok : c'en est une.
func ParseVerbose(arg string) (n int, ok bool, err error) {
	if name, v, has := strings.Cut(arg, "="); name == "--verbose" || name == "-verbose" {
		if !has {
			return 1, true, nil
		}
		if _, e := fmt.Sscan(v, &n); e != nil || n < 0 || n > 3 {
			return 0, true, fmt.Errorf("--verbose attend un niveau de 0 à 3, pas « %s »", v)
		}
		return n, true, nil
	}
	if len(arg) >= 2 && arg[0] == '-' && strings.Trim(arg[1:], "v") == "" {
		if len(arg)-1 > 3 {
			return 3, true, nil
		}
		return len(arg) - 1, true, nil
	}
	return 0, false, nil
}
