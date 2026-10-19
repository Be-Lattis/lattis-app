package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/console"
	"github.com/lalamefine/lattis/internal/store"
	"github.com/lalamefine/lattis/internal/trace"
)

// debugOpts : options de débogage de la ligne de commande.
type debugOpts struct{ level int }

// parseDebug retire de args les options de débogage et les applique :
//
//	-v, -vv, -vvv, --verbose[=n]   verbosité du journal (internal/trace)
//	--log=doc,sql…                 catégories écrites (toutes par défaut)
//	--log-file=chemin              journal aussi écrit dans ce fichier
//	--sql, --no-sql                journal SQL seul, quel que soit le niveau
//
// LATTIS_VERBOSE=n donne le niveau sans option. Le journal SQL suit le
// niveau (à partir de -vv), même en développement.
func parseDebug(args []string) (debugOpts, []string, error) {
	var o debugOpts
	if v, err := strconv.Atoi(os.Getenv("LATTIS_VERBOSE")); err == nil {
		o.level = v
	}
	var rest []string
	var sql *bool
	var cats []string
	logFile := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, value, inline := strings.Cut(a, "=")
		val := func() string {
			if inline {
				return value
			}
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				return args[i]
			}
			return ""
		}
		if n, ok, err := trace.ParseVerbose(a); ok {
			if err != nil {
				return o, nil, err
			}
			o.level = n
			continue
		}
		switch name {
		case "--sql", "-sql":
			on := true
			sql = &on
		case "--no-sql", "-no-sql":
			off := false
			sql = &off
		case "--log", "-log":
			cats = append(cats, strings.Split(val(), ",")...)
		case "--log-file", "-log-file":
			if logFile = val(); logFile == "" {
				return o, nil, fmt.Errorf("--log-file attend un chemin de fichier")
			}
		default:
			rest = append(rest, a)
		}
	}
	trace.SetLevel(o.level)
	if err := trace.Only(cats); err != nil {
		return o, nil, err
	}
	// Une catégorie demandée sans niveau : -v implicite (-vv pour sql).
	if len(cats) > 0 && o.level == 0 {
		o.level = 1
		if trace.Allowed(trace.SQL) {
			o.level = 2
		}
		trace.SetLevel(o.level)
	}
	on := store.DefaultSQLLog() || o.level >= 2
	if sql != nil {
		on = *sql
	}
	store.SetSQLLog(on)
	if o.level > 0 || on || logFile != "" {
		console.Attach()
	}
	if logFile != "" {
		if _, err := trace.ToFile(logFile); err != nil {
			return o, nil, fmt.Errorf("--log-file : %v", err)
		}
	}
	return o, rest, nil
}

// wailsLevel : journal de Wails, avertissements seulement sauf à -vvv.
func (o debugOpts) wailsLevel() slog.Level {
	if o.level >= 3 {
		return slog.LevelDebug
	}
	return slog.LevelWarn
}
