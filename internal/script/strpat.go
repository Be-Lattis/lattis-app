package script

import (
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/lalamefine/lattis/internal/script/pattern"
)

// Fonctions de motifs de la bibliothèque string (find, match, gmatch, gsub),
// réécrites d'après gopher-lua sur le moteur interruptible du paquet
// pattern : un motif à retour arrière exponentiel s'arrête avec le délai ou
// le bouton Arrêter, comme une boucle Lua.

func installPatterns(s *lua.LTable, L *lua.LState) {
	s.RawSetString("find", L.NewFunction(strFind))
	s.RawSetString("match", L.NewFunction(strMatch))
	s.RawSetString("gmatch", L.NewFunction(strGmatch))
	s.RawSetString("gsub", L.NewFunction(strGsub))
}

// findAll lance le moteur ; une annulation remonte comme une erreur Lua, que
// Run remplace par la cause (délai, Arrêter, mémoire).
func findAll(L *lua.LState, pat, str string, offset, limit int) []*pattern.MatchData {
	mds, err := pattern.Find(L.Context(), pat, []byte(str), offset, limit)
	if err != nil {
		L.RaiseError("%s", err.Error())
	}
	return mds
}

// pushCaptures pousse les captures (ou la correspondance entière s'il n'y en
// a pas et que whole est vrai) et renvoie leur nombre.
func pushCaptures(L *lua.LState, md *pattern.MatchData, str string, whole bool) int {
	if md.CaptureLength() == 2 {
		if !whole {
			return 0
		}
		L.Push(lua.LString(str[md.Capture(0):md.Capture(1)]))
		return 1
	}
	for i := 2; i < md.CaptureLength(); i += 2 {
		if md.IsPosCapture(i) {
			L.Push(lua.LNumber(md.Capture(i)))
		} else {
			L.Push(lua.LString(str[md.Capture(i):md.Capture(i+1)]))
		}
	}
	return md.CaptureLength()/2 - 1
}

func luaIndex(str string, i int, start bool) int {
	if start && i != 0 {
		i--
	}
	l := len(str)
	if i < 0 {
		i = l + i + 1
	}
	i = max(0, i)
	if !start && i > l {
		i = l
	}
	return i
}

func strFind(L *lua.LState) int {
	str := L.CheckString(1)
	pat := L.CheckString(2)
	if len(pat) == 0 {
		L.Push(lua.LNumber(1))
		L.Push(lua.LNumber(0))
		return 2
	}
	init := luaIndex(str, L.OptInt(3, 1), true)
	if L.GetTop() == 4 && lua.LVAsBool(L.Get(4)) {
		if init > len(str) {
			L.Push(lua.LNil)
			return 1
		}
		pos := strings.Index(str[init:], pat)
		if pos < 0 {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(lua.LNumber(init + pos + 1))
		L.Push(lua.LNumber(init + pos + len(pat)))
		return 2
	}
	mds := findAll(L, pat, str, init, 1)
	if len(mds) == 0 {
		L.Push(lua.LNil)
		return 1
	}
	md := mds[0]
	L.Push(lua.LNumber(md.Capture(0) + 1))
	L.Push(lua.LNumber(md.Capture(1)))
	return 2 + pushCaptures(L, md, str, false)
}

func strMatch(L *lua.LState) int {
	str := L.CheckString(1)
	pat := L.CheckString(2)
	offset := L.OptInt(3, 1)
	if offset < 0 {
		offset = len(str) + offset + 1
	}
	offset = max(0, offset-1)
	mds := findAll(L, pat, str, offset, 1)
	if len(mds) == 0 {
		L.Push(lua.LNil)
		return 1
	}
	return pushCaptures(L, mds[0], str, true)
}

func strGmatch(L *lua.LState) int {
	str := L.CheckString(1)
	mds := findAll(L, L.CheckString(2), str, 0, -1)
	i := 0
	L.Push(L.NewFunction(func(L *lua.LState) int {
		if i >= len(mds) {
			return 0
		}
		md := mds[i]
		i++
		return pushCaptures(L, md, str, true)
	}))
	return 1
}

func strGsub(L *lua.LState) int {
	str := L.CheckString(1)
	pat := L.CheckString(2)
	L.CheckTypes(3, lua.LTString, lua.LTNumber, lua.LTTable, lua.LTFunction)
	repl := L.CheckAny(3)
	mds := findAll(L, pat, str, 0, L.OptInt(4, -1))
	if len(mds) == 0 {
		L.SetTop(1)
		L.Push(lua.LNumber(0))
		return 2
	}
	var b strings.Builder
	last := 0
	for _, md := range mds {
		start, end := md.Capture(0), md.Capture(1)
		b.WriteString(str[last:start])
		last = end
		var v lua.LValue
		switch r := repl.(type) {
		case *lua.LTable:
			v = L.GetTable(r, captureValue(L, md, str, 1))
		case *lua.LFunction:
			L.Push(r)
			n := 0
			if md.CaptureLength() > 2 {
				for i := 1; i < md.CaptureLength()/2; i++ {
					L.Push(captureValue(L, md, str, i))
					n++
				}
			} else {
				L.Push(lua.LString(str[start:end]))
				n = 1
			}
			L.Call(n, 1)
			v = L.Get(-1)
			L.Pop(1)
		default:
			v = lua.LString(expand(L, md, str, lua.LVAsString(repl)))
		}
		if lua.LVIsFalse(v) {
			b.WriteString(str[start:end])
		} else {
			b.WriteString(lua.LVAsString(v))
		}
	}
	b.WriteString(str[last:])
	L.Push(lua.LString(b.String()))
	L.Push(lua.LNumber(len(mds)))
	return 2
}

// captureValue renvoie la capture n (1 et suivantes), ou la correspondance
// entière pour n = 1 s'il n'y a pas de capture.
func captureValue(L *lua.LState, md *pattern.MatchData, str string, n int) lua.LValue {
	idx := 2 * n
	if md.CaptureLength() == 2 && n == 1 {
		idx = 0
	} else if idx >= md.CaptureLength() {
		L.RaiseError("indice de capture invalide")
	}
	if md.IsPosCapture(idx) {
		return lua.LNumber(md.Capture(idx))
	}
	return lua.LString(str[md.Capture(idx):md.Capture(idx+1)])
}

// expand remplace %0 à %9 et %% dans la chaîne de remplacement de gsub.
func expand(L *lua.LState, md *pattern.MatchData, str, repl string) string {
	if !strings.Contains(repl, "%") {
		return repl
	}
	var b strings.Builder
	for i := 0; i < len(repl); i++ {
		c := repl[i]
		if c != '%' || i+1 == len(repl) {
			b.WriteByte(c)
			continue
		}
		i++
		c = repl[i]
		switch {
		case c == '0':
			b.WriteString(str[md.Capture(0):md.Capture(1)])
		case c >= '1' && c <= '9':
			v := captureValue(L, md, str, int(c-'0'))
			if n, ok := v.(lua.LNumber); ok {
				b.WriteString(strconv.Itoa(int(n)))
			} else {
				b.WriteString(string(v.(lua.LString)))
			}
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
