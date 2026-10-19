package script

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// Les fonctions de motifs réécrites gardent le comportement de Lua 5.1.
func TestPatterns(t *testing.T) {
	f := open(t)
	out := f.mustRun(t, `
		print(string.find("hello world", "o w"))
		print(string.find("hello world", "(o)(r)"))
		print(string.find("a.b", ".", 1, true), string.find("abc", "x"))
		print(string.find("abc", "b", 10, true))
		print(string.match("cle=valeur", "(%w+)=(%w+)"))
		print(string.match("abc 123", "%d+"), string.match("abc", "%d"))
		print(string.match("hello", "()ll()"))
		local t = {}
		for k, v in string.gmatch("a=1, b=2", "(%w+)=(%w+)") do t[#t+1] = k .. v end
		for w in ("un deux"):gmatch("%a+") do t[#t+1] = w end
		print(table.concat(t, ","))
		print(string.gsub("hello world", "o", "0"))
		print(string.gsub("hello world", "(%w+)", "<%1>"))
		print(string.gsub("abc", "%w", "%0%0", 2))
		print(string.gsub("$nom a $age", "%$(%w+)", {nom = "Léa", age = 30}))
		print(string.gsub("1 2 3", "%d", function(d) return d * 2 end))
		print(string.gsub("x y", "%a", function() return nil end))
		print(("50%"):gsub("%%", "%%%%"))
	`)
	want := []string{
		"5\t7",
		"8\t9\to\tr",
		"2\tnil",
		"nil",
		"cle\tvaleur",
		"123\tnil",
		"3\t5",
		"a1,b2,un,deux",
		"hell0 w0rld\t2",
		"<hello> <world>\t2",
		"aabbc\t2",
		"Léa a 30\t2",
		"2 4 6\t3",
		"x y\t2",
		"50%%\t1",
	}
	if strings.Join(out, "\n") != strings.Join(want, "\n") {
		t.Fatalf("obtenu :\n%s\nattendu :\n%s", strings.Join(out, "\n"), strings.Join(want, "\n"))
	}
}

// Un motif à retour arrière exponentiel ne bloque plus le délai ni Arrêter
// (audit 6, I3 : 75 s pour un délai d'une seconde).
func TestPatternInterrupted(t *testing.T) {
	f := open(t)
	for _, fn := range []string{"find", "match", "gmatch", "gsub"} {
		src := `local s = string.rep("a", 3000) .. "c"
		string.` + fn + `(s, "(.-)(.-)(.-)b"` + map[bool]string{true: `, "x"`, false: ``}[fn == "gsub"] + `)`
		lim := DefaultLimits
		lim.Timeout = 200 * time.Millisecond
		res := Run(context.Background(), Env{Read: f.db.Read()}, src, lim)
		if !errors.Is(res.Err, ErrTimeout) || res.Duration > 3*time.Second {
			t.Fatalf("%s, délai : %v en %v", fn, res.Err, res.Duration)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	res := Run(ctx, Env{Read: f.db.Read()}, `
		local ok = pcall(string.find, string.rep("a", 3000), "(.-)(.-)(.-)b")
		while true do end`, DefaultLimits)
	if !errors.Is(res.Err, ErrStopped) || res.Duration > 3*time.Second {
		t.Fatalf("Arrêter : %v en %v", res.Err, res.Duration)
	}
}
