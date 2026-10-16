package formula

import "strings"

// criterion est un critère de NB.SI, SOMME.SI… : une valeur (égalité), ou un
// texte comme ">=100", "<>Paris", "Dur*", "" (vide), "<>" (non vide).
type criterion struct {
	op   string // =, <>, <, <=, >, >=
	val  Value  // opérande : nombre, date, booléen, texte, ou vide
	wild bool   // le texte contient * ou ? (égalité par motif)
	err  Value
}

// parseCriterion lit un critère. Comme dans Excel, une référence à une cellule
// vide vaut 0, et un texte numérique ("10", ">=2,5") compare des nombres.
func parseCriterion(v Value) criterion {
	switch v.Kind {
	case Error:
		return criterion{err: v}
	case Empty:
		return criterion{op: "=", val: Num(0)}
	case Text:
	default:
		return criterion{op: "=", val: v}
	}
	s := v.Str
	op := "="
	for _, o := range []string{"<=", ">=", "<>", "<", ">", "="} {
		if strings.HasPrefix(s, o) {
			op, s = o, s[len(o):]
			break
		}
	}
	c := criterion{op: op}
	switch {
	case s == "":
		c.val = Value{} // "" et "=" : cellules vides ; "<>" : non vides
	case strings.EqualFold(s, "VRAI") || strings.EqualFold(s, "TRUE"):
		c.val = Boolean(true)
	case strings.EqualFold(s, "FAUX") || strings.EqualFold(s, "FALSE"):
		c.val = Boolean(false)
	default:
		if f, ok := parseNumber(s); ok {
			c.val = Num(f)
		} else if f, ok := ParseDate(s); ok {
			c.val = DateSerial(f)
		} else if e, ok := errorByName(s); ok {
			c.val = Err(e)
		} else {
			c.val = Str(s)
			c.wild = strings.ContainsAny(s, "*?") && (op == "=" || op == "<>")
		}
	}
	return c
}

func errorByName(s string) (ErrCode, bool) {
	s = strings.ToUpper(s)
	for code, name := range errNames {
		if name == s {
			return code, true
		}
	}
	code, ok := errAliases[s]
	return code, ok
}

// match dit si une valeur de la plage satisfait le critère.
func (c criterion) match(v Value) bool {
	eq, comparable := c.compare(v)
	switch c.op {
	case "=":
		return comparable && eq == 0
	case "<>":
		return !comparable || eq != 0
	case "<":
		return comparable && eq < 0
	case "<=":
		return comparable && eq <= 0
	case ">":
		return comparable && eq > 0
	case ">=":
		return comparable && eq >= 0
	}
	return false
}

// compare compare v à l'opérande ; comparable est faux quand les types ne se
// comparent pas (un texte face à ">5" n'est ni plus grand ni plus petit).
func (c criterion) compare(v Value) (int, bool) {
	want := c.val
	switch want.Kind {
	case Empty:
		if c.op == "=" || c.op == "<>" {
			if blank(v) {
				return 0, true
			}
			return 1, true
		}
		return 0, false
	case Number, Date:
		if v.Kind == Text {
			// "10" dans la plage correspond au critère 10, pour l'égalité.
			if f, ok := parseNumber(v.Str); ok && (c.op == "=" || c.op == "<>") {
				return cmp(Num(f), Num(want.Num)), true
			}
			return 0, false
		}
		if v.Kind != Number && v.Kind != Date {
			return 0, false
		}
		return cmp(Num(v.Num), Num(want.Num)), true
	case Bool:
		if v.Kind != Bool {
			return 0, false
		}
		return cmp(v, want), true
	case Error:
		if v.Kind == Error && v.Err == want.Err {
			return 0, true
		}
		return 1, c.op == "=" || c.op == "<>"
	case Text:
		if v.Kind != Text {
			return 0, false
		}
		if c.wild {
			if Wildcard(want.Str, v.Str) {
				return 0, true
			}
			return 1, true
		}
		return cmp(v, want), true
	}
	return 0, false
}

// Wildcard dit si s correspond au motif (sans tenir compte de la casse) :
// * pour une suite quelconque, ? pour un caractère, ~ pour échapper.
func Wildcard(pattern, s string) bool {
	p := []rune(strings.ToLower(pattern))
	t := []rune(strings.ToLower(s))
	return wildFrom(p, t, true)
}

// wildFrom compare le motif au texte ; anchored exige que le texte entier
// corresponde (sinon : le motif doit correspondre à un début du texte).
func wildFrom(p, t []rune, anchored bool) bool {
	// Programmation dynamique simple : motifs et textes de cellule sont courts.
	var rec func(i, j int) bool
	memo := map[[2]int]bool{}
	rec = func(i, j int) bool {
		key := [2]int{i, j}
		if v, ok := memo[key]; ok {
			return v
		}
		var ok bool
		switch {
		case i == len(p):
			ok = !anchored || j == len(t)
		case p[i] == '*':
			ok = rec(i+1, j) || j < len(t) && rec(i, j+1)
		case j == len(t):
			ok = false
		case p[i] == '?':
			ok = rec(i+1, j+1)
		case p[i] == '~' && i+1 < len(p):
			ok = p[i+1] == t[j] && rec(i+2, j+1)
		default:
			ok = p[i] == t[j] && rec(i+1, j+1)
		}
		memo[key] = ok
		return ok
	}
	return rec(0, 0)
}

// searchText cherche un motif dans un texte à partir d'une position (en
// caractères, 0 = début) et renvoie la position trouvée, ou -1. Sans tenir
// compte de la casse et avec jokers (CHERCHE).
func searchText(pattern, s string, from int) int {
	p := []rune(strings.ToLower(pattern))
	t := []rune(strings.ToLower(s))
	for i := from; i <= len(t); i++ {
		if wildFrom(p, t[i:], false) {
			return i
		}
	}
	return -1
}
