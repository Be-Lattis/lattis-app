package script

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// API expose à un script les commandes de l'application : feuilles,
// tableaux, sélection, boîtes de dialogue, boutons… (internal/api). Les
// mêmes commandes servent le serveur MCP : un script et un assistant voient
// le classeur de la même façon.
type API interface {
	Specs() []Spec
	// Call exécute une commande ; args associe un nom de paramètre à une
	// valeur (nombre, texte, booléen, nil, []any ou map[string]any). Le
	// résultat est converti en Lua comme du JSON.
	Call(ctx context.Context, name string, args map[string]any) (any, error)
}

// Spec décrit une commande pour Lua : "sheets.add" devient la fonction add
// de la table globale sheets.
type Spec struct {
	Name   string
	Params []ParamSpec
	Write  bool // modifie le classeur (le catalogue des tableaux est relu)
}

// ParamSpec est un paramètre, dans l'ordre des arguments positionnels.
type ParamSpec struct {
	Name string
	// Table : le paramètre attend une table Lua (objet ou liste).
	Table bool
}

// installAPI crée les tables globales des commandes. Une commande s'appelle
// avec ses arguments dans l'ordre (sheets.rename("Feuil1", "Ventes")) ou
// avec une seule table d'arguments nommés (sheets.rename{sheet = "Feuil1",
// name = "Ventes"}).
func (r *runner) installAPI(L *lua.LState) {
	if r.env.API == nil {
		return
	}
	specs := r.env.API.Specs()
	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	for _, sp := range specs {
		dot := strings.IndexByte(sp.Name, '.')
		if dot <= 0 {
			continue
		}
		ns, fn := sp.Name[:dot], sp.Name[dot+1:]
		t, ok := L.GetGlobal(ns).(*lua.LTable)
		if !ok {
			t = L.NewTable()
			L.SetGlobal(ns, t)
		}
		if t.RawGetString(fn) != lua.LNil {
			continue // db.query… : la version Lua reste
		}
		t.RawSetString(fn, L.NewFunction(r.command(sp)))
	}
}

func (r *runner) command(sp Spec) lua.LGFunction {
	return func(L *lua.LState) int {
		args := map[string]any{}
		n := L.GetTop()
		named := n == 1 && L.Get(1).Type() == lua.LTTable && (len(sp.Params) == 0 || !sp.Params[0].Table)
		if named {
			L.CheckTable(1).ForEach(func(k, v lua.LValue) {
				s, ok := k.(lua.LString)
				if !ok {
					L.RaiseError("%s : les arguments nommés ont des noms de paramètres", sp.Name)
				}
				args[string(s)] = fromLua(v, 0)
			})
		} else {
			if n > len(sp.Params) {
				L.RaiseError("%s : %d arguments au plus, %d reçus", sp.Name, len(sp.Params), n)
			}
			for i := 1; i <= n; i++ {
				if v := L.Get(i); v != lua.LNil {
					args[sp.Params[i-1].Name] = fromLua(v, 0)
				}
			}
		}
		res, err := r.env.API.Call(r.ctx, sp.Name, args)
		if sp.Write {
			r.writes++
			r.cat = nil
		}
		if err != nil {
			L.RaiseError("%s : %v", sp.Name, err)
		}
		L.Push(jsonToLua(L, res))
		return 1
	}
}

// fromLua convertit une valeur Lua : une table de clés 1..n devient une
// liste, une autre table un objet.
func fromLua(v lua.LValue, depth int) any {
	if depth > 32 {
		return nil
	}
	switch v := v.(type) {
	case lua.LNumber:
		f := float64(v)
		if f == math.Trunc(f) && math.Abs(f) < 1<<53 {
			return int64(f)
		}
		return f
	case lua.LString:
		return string(v)
	case lua.LBool:
		return bool(v)
	case *lua.LTable:
		n := v.Len()
		count := 0
		v.ForEach(func(_, _ lua.LValue) { count++ })
		if n > 0 && n == count {
			out := make([]any, n)
			for i := 1; i <= n; i++ {
				out[i-1] = fromLua(v.RawGetInt(i), depth+1)
			}
			return out
		}
		out := map[string]any{}
		v.ForEach(func(k, val lua.LValue) {
			out[lua.LVAsString(k)] = fromLua(val, depth+1)
		})
		return out
	}
	return nil
}

// jsonToLua convertit un résultat comme du JSON (les champs des structures
// prennent leur nom JSON).
func jsonToLua(L *lua.LState, v any) lua.LValue {
	if v == nil {
		return lua.LNil
	}
	b, err := json.Marshal(v)
	if err != nil {
		L.RaiseError("résultat illisible : %v", err)
	}
	var g any
	if err := json.Unmarshal(b, &g); err != nil {
		L.RaiseError("résultat illisible : %v", err)
	}
	return genericToLua(L, g)
}

func genericToLua(L *lua.LState, v any) lua.LValue {
	switch v := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []any:
		t := L.CreateTable(len(v), 0)
		for _, x := range v {
			t.Append(genericToLua(L, x))
		}
		return t
	case map[string]any:
		t := L.CreateTable(0, len(v))
		for k, x := range v {
			t.RawSetString(k, genericToLua(L, x))
		}
		return t
	}
	return lua.LNil
}

// --- Durée maximale, suspendue pendant une boîte de dialogue ------------------

// clock compte le temps d'exécution d'un script, hors attente de
// l'utilisateur (boîte de dialogue ouverte par ui.dialog…).
type clock struct {
	mu     sync.Mutex
	used   time.Duration
	since  time.Time // zéro : suspendue
	paused int
}

type clockKey struct{}

// Pause suspend le décompte de la durée maximale du script qui porte ctx
// (aucun effet hors script) ; appeler la fonction renvoyée le reprend.
func Pause(ctx context.Context) (resume func()) {
	c, _ := ctx.Value(clockKey{}).(*clock)
	if c == nil {
		return func() {}
	}
	c.mu.Lock()
	if c.paused == 0 {
		c.used += time.Since(c.since)
	}
	c.paused++
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		c.paused--
		if c.paused == 0 {
			c.since = time.Now()
		}
		c.mu.Unlock()
	}
}

func (c *clock) elapsed() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused > 0 {
		return c.used
	}
	return c.used + time.Since(c.since)
}

// watchClock arrête le script quand son temps d'exécution dépasse limit.
func watchClock(ctx context.Context, cancel context.CancelCauseFunc, c *clock, limit time.Duration) {
	t := time.NewTicker(10 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if c.elapsed() > limit {
				cancel(ErrTimeout)
				return
			}
		}
	}
}
