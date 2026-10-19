package script

import (
	"context"
	"runtime"
	"runtime/metrics"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// Le bac à sable a été validé au spike 1.4 (docs/decisions/0001-jalon-m1.md).
//
// gopher-lua (Lua 5.1) est ouvert sans bibliothèque ; seules base, string,
// table et math sont chargées, plus os.time, os.date, os.clock et
// os.difftime. Rien ne touche aux fichiers, aux processus ni au réseau, et
// rien ne charge de code (load, require, string.dump...).
//
// gopher-lua n'a pas de crochet « toutes les N instructions » ; il vérifie en
// revanche le contexte qu'on lui donne (SetContext) : un délai ou le bouton
// Arrêter annulent ce contexte, qui est aussi transmis aux requêtes SQL en
// cours. Les fonctions de motifs de string tournent en Go, hors de la VM :
// elles sont remplacées par des versions qui vérifient aussi ce contexte
// (strpat.go). gopher-lua ne bornant pas la mémoire, un chien de garde surveille
// le tas du processus (voir watchHeap).

// baseAllowed sont les fonctions de base gardées.
var baseAllowed = map[string]bool{
	"assert": true, "error": true, "getmetatable": true, "ipairs": true, "next": true,
	"pairs": true, "pcall": true, "rawequal": true, "rawget": true, "rawset": true,
	"select": true, "setmetatable": true, "tonumber": true, "tostring": true,
	"type": true, "unpack": true, "xpcall": true, "_G": true, "_VERSION": true,
}

var osAllowed = map[string]bool{"time": true, "date": true, "clock": true, "difftime": true}

// maxString borne string.rep, qui allouerait d'un coup avant que le chien de
// garde n'ait le temps de réagir.
const maxString = 16 << 20

func newState(ctx context.Context) *lua.LState {
	L := lua.NewState(lua.Options{SkipOpenLibs: true, CallStackSize: 200, RegistryMaxSize: 1 << 20})
	for _, lib := range []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase}, {lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString}, {lua.MathLibName, lua.OpenMath},
		{lua.OsLibName, lua.OpenOs},
	} {
		L.Push(L.NewFunction(lib.fn))
		L.Push(lua.LString(lib.name))
		L.Call(1, 0)
	}
	keepOnly(L.G.Global, baseAllowed, "string", "table", "math", "os")
	if os, ok := L.GetGlobal("os").(*lua.LTable); ok {
		keepOnly(os, osAllowed)
	}
	if s, ok := L.GetGlobal("string").(*lua.LTable); ok {
		s.RawSetString("dump", lua.LNil) // renverrait du bytecode chargeable
		installPatterns(s, L)
		rep := s.RawGetString("rep").(*lua.LFunction)
		s.RawSetString("rep", L.NewFunction(func(L *lua.LState) int {
			if int64(len(L.CheckString(1)))*int64(L.CheckInt(2)) > maxString {
				L.RaiseError("string.rep : résultat trop grand")
			}
			L.Push(rep)
			L.Push(L.Get(1))
			L.Push(L.Get(2))
			L.Call(2, 1)
			return 1
		}))
	}
	// Le métatable des chaînes donne accès à string : il n'expose que ce qui
	// reste dans la table ci-dessus.
	L.SetContext(ctx)
	return L
}

// keepOnly retire d'une table les clés absentes de allowed et de extra.
func keepOnly(t *lua.LTable, allowed map[string]bool, extra ...string) {
	var drop []lua.LValue
	t.ForEach(func(k, _ lua.LValue) {
		s, ok := k.(lua.LString)
		if !ok {
			return
		}
		if allowed[string(s)] {
			return
		}
		for _, e := range extra {
			if string(s) == e {
				return
			}
		}
		drop = append(drop, k)
	})
	for _, k := range drop {
		t.RawSet(k, lua.LNil)
	}
}

// heapObjects est la mémoire des objets du tas, vivants ou pas encore
// balayés : l'équivalent de runtime.MemStats.HeapAlloc.
const heapObjects = "/memory/classes/heap/objects:bytes"

// heapBytes lit heapObjects. runtime/metrics ne suspend pas le programme,
// contrairement à runtime.ReadMemStats (un « stop the world » à chaque
// lecture, ici 50 fois par seconde pendant tout le script).
func heapBytes(s []metrics.Sample) uint64 {
	metrics.Read(s)
	if s[0].Value.Kind() != metrics.KindUint64 {
		return 0
	}
	return s[0].Value.Uint64()
}

// watchHeap annule le script quand le tas grossit de plus de limit octets.
//
// Go ne compte pas la mémoire par goroutine et gopher-lua n'a ni allocateur
// ni crochet d'allocation à brancher : la mesure reste celle du tas du
// processus, rapportée à ce qu'il était au lancement du script. Pour ne pas
// arrêter un script qui produit beaucoup de déchets sans rien garder (ni
// accuser un script d'objets morts que le ramasse-miettes n'a pas encore
// repris), un dépassement est confirmé après un ramassage complet ; il est
// rare (au-delà de la borne seulement), seule la lecture courante doit être
// légère.
func watchHeap(ctx context.Context, cancel context.CancelCauseFunc, limit uint64) {
	s := []metrics.Sample{{Name: heapObjects}}
	base := heapBytes(s)
	t := time.NewTicker(20 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if heapBytes(s) <= base+limit {
				continue
			}
			runtime.GC()
			if heapBytes(s) > base+limit {
				cancel(ErrMemory)
				return
			}
		}
	}
}
