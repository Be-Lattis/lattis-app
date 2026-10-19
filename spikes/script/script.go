// Package script est le prototype du bac à sable Lua (spike 1.4).
//
// gopher-lua (Lua 5.1) est ouvert sans aucune bibliothèque, puis seules
// base (expurgée), string, table et math sont chargées : pas de os, io,
// require, load, dofile ni debug. Le script reçoit une API db.query() et
// db.execute() ; ses écritures passent par la file d'écriture du document,
// une instruction à la fois, pour qu'un script ne bloque jamais la saisie
// dans la grille.
//
// gopher-lua n'a pas de crochet « toutes les N instructions » ; il vérifie
// en revanche le contexte qu'on lui donne (SetContext) pendant l'exécution :
// un délai ou le bouton Arrêter annulent ce contexte, qui est aussi transmis
// aux requêtes SQL en cours. Comme gopher-lua ne borne pas la mémoire, un
// chien de garde surveille le tas du processus et arrête le script au-delà
// d'une limite.
package script

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/store"
	lua "github.com/yuin/gopher-lua"
)

// Limits borne un script.
type Limits struct {
	Timeout  time.Duration // durée maximale
	MaxRows  int           // lignes au plus par db.query
	MaxHeap  uint64        // croissance du tas tolérée, en octets
	MaxPrint int           // lignes de sortie conservées
}

// DefaultLimits sont les bornes par défaut.
var DefaultLimits = Limits{Timeout: 30 * time.Second, MaxRows: 100_000, MaxHeap: 512 << 20, MaxPrint: 1000}

// Result est l'issue d'un script.
type Result struct {
	Output   []string
	Writes   int // instructions db.execute validées
	Duration time.Duration
	Err      error
}

// Raisons d'arrêt.
var (
	ErrTimeout = errors.New("script arrêté : durée maximale dépassée")
	ErrStopped = errors.New("script arrêté par l'utilisateur")
	ErrMemory  = errors.New("script arrêté : trop de mémoire utilisée")
)

// Run exécute un script. Annuler ctx joue le rôle du bouton Arrêter.
func Run(ctx context.Context, db *store.Store, src string, lim Limits) Result {
	start := time.Now()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	if lim.Timeout > 0 {
		var stop context.CancelFunc
		ctx, stop = context.WithTimeoutCause(ctx, lim.Timeout, ErrTimeout)
		defer stop()
	}
	if lim.MaxHeap > 0 {
		go watchHeap(ctx, cancel, lim.MaxHeap)
	}

	L := lua.NewState(lua.Options{SkipOpenLibs: true, CallStackSize: 200, RegistryMaxSize: 1 << 20})
	defer L.Close()
	for _, lib := range []struct {
		name string
		fn   lua.LGFunction
	}{{lua.BaseLibName, lua.OpenBase}, {lua.TabLibName, lua.OpenTable}, {lua.StringLibName, lua.OpenString}, {lua.MathLibName, lua.OpenMath}} {
		L.Push(L.NewFunction(lib.fn))
		L.Push(lua.LString(lib.name))
		L.Call(1, 0)
	}
	for _, name := range []string{"dofile", "loadfile", "load", "loadstring", "require", "module", "setfenv", "getfenv", "newproxy", "collectgarbage"} {
		L.SetGlobal(name, lua.LNil)
	}
	// string.dump renverrait du bytecode chargeable : retiré aussi.
	if s, ok := L.GetGlobal("string").(*lua.LTable); ok {
		s.RawSetString("dump", lua.LNil)
	}

	res := Result{}
	L.SetGlobal("print", L.NewFunction(func(L *lua.LState) int {
		parts := make([]string, L.GetTop())
		for i := range parts {
			parts[i] = L.ToStringMeta(L.Get(i + 1)).String()
		}
		if len(res.Output) < lim.MaxPrint {
			res.Output = append(res.Output, strings.Join(parts, "\t"))
		}
		return 0
	}))
	dbt := L.NewTable()
	dbt.RawSetString("query", L.NewFunction(func(L *lua.LState) int { return query(ctx, L, db, lim.MaxRows) }))
	dbt.RawSetString("execute", L.NewFunction(func(L *lua.LState) int {
		n := execute(ctx, L, db)
		res.Writes++
		return n
	}))
	L.SetGlobal("db", dbt)

	L.SetContext(ctx)
	err := L.DoString(src)
	res.Duration = time.Since(start)
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			if errors.Is(cause, context.Canceled) {
				cause = ErrStopped
			}
			err = cause
		}
		res.Err = err
	}
	return res
}

func watchHeap(ctx context.Context, cancel context.CancelCauseFunc, limit uint64) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	base := ms.HeapAlloc
	t := time.NewTicker(20 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			runtime.ReadMemStats(&ms)
			if ms.HeapAlloc > base+limit {
				cancel(ErrMemory)
				return
			}
		}
	}
}

// args convertit les paramètres Lua d'une requête (à partir de l'indice 2).
func args(L *lua.LState) []any {
	var out []any
	for i := 2; i <= L.GetTop(); i++ {
		switch v := L.Get(i).(type) {
		case lua.LNumber:
			out = append(out, float64(v))
		case lua.LString:
			out = append(out, string(v))
		case lua.LBool:
			out = append(out, bool(v))
		default:
			out = append(out, nil)
		}
	}
	return out
}

func toLua(v any) lua.LValue {
	switch v := v.(type) {
	case int64:
		return lua.LNumber(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []byte:
		return lua.LString(v)
	case bool:
		return lua.LBool(v)
	}
	return lua.LNil
}

// query : db.query(sql, ...) renvoie un tableau de lignes { colonne = valeur }.
// Lecture seule (pool query_only), bornée en nombre de lignes.
func query(ctx context.Context, L *lua.LState, db *store.Store, max int) int {
	rs, err := db.Read().QueryContext(ctx, L.CheckString(1), args(L)...)
	if err != nil {
		L.RaiseError("db.query : %v", err)
	}
	defer rs.Close()
	cols, _ := rs.Columns()
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	out := L.NewTable()
	n := 0
	for rs.Next() {
		if n == max {
			L.RaiseError("db.query : plus de %d lignes, ajoutez un LIMIT", max)
		}
		if err := rs.Scan(ptrs...); err != nil {
			L.RaiseError("db.query : %v", err)
		}
		row := L.NewTable()
		for i, c := range cols {
			row.RawSetString(c, toLua(vals[i]))
		}
		out.Append(row)
		n++
	}
	if err := rs.Err(); err != nil {
		L.RaiseError("db.query : %v", err)
	}
	L.Push(out)
	return 1
}

// execute : db.execute(sql, ...) renvoie le nombre de lignes modifiées.
// Chaque appel est une courte transaction dans la file d'écriture.
func execute(ctx context.Context, L *lua.LState, db *store.Store) int {
	q, a := L.CheckString(1), args(L)
	var n int64
	err := db.Write(ctx, func(tx *sql.Tx) error {
		r, err := tx.ExecContext(ctx, q, a...)
		if err != nil {
			return err
		}
		n, _ = r.RowsAffected()
		return nil
	})
	if err != nil {
		L.RaiseError("db.execute : %v", fmt.Sprint(err))
	}
	L.Push(lua.LNumber(n))
	return 1
}
