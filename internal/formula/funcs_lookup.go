package formula

import (
	"math"
	"strconv"
	"strings"
)

// Recherches du tableur au-delà de la correspondance exacte (audit des
// formules du 30/10/2026) : EQUIVX, RECHERCHEV approchée, RECHERCHEX avec
// ses modes, RECHERCHEV et RECHERCHEH sur des cellules libres.

// xmatch : EQUIVX(valeur; colonne; [mode_correspondance]; [mode_recherche]).
// Mode 0 : égalité (sans jokers) ; -1 : égalité ou la plus grande valeur
// inférieure ; 1 : égalité ou la plus petite valeur supérieure ; 2 : jokers
// * ? ~. Recherche 1 : première trouvée ; -1 : dernière. Les recherches
// dichotomiques (2, -2) parcourent tout : même résultat sur des données
// triées, et juste sur des données qui ne le sont pas.
type xmatch struct {
	key       Value
	mode      int
	last      bool
	err       Value
	pos, best int
	bestV     Value
}

func newXMatch(p []Value) scanner {
	m := &xmatch{key: p[0]}
	if m.key.IsErr() {
		m.err = m.key
	}
	mode, e := modeArg(optParam(p, 1), 0, -1, 0, 1, 2)
	if e.IsErr() && !m.err.IsErr() {
		m.err = e
	}
	search, e := modeArg(optParam(p, 2), 1, 1, -1, 2, -2)
	if e.IsErr() && !m.err.IsErr() {
		m.err = e
	}
	m.mode, m.last = mode, search == -1
	return m
}

// modeArg lit un mode entier parmi allowed (def si vide) ; #VALEUR! sinon.
func modeArg(v Value, def int, allowed ...int) (int, Value) {
	if v.IsEmpty() {
		return def, Value{}
	}
	f, e := ToNumber(v)
	if e.IsErr() {
		return 0, e
	}
	for _, a := range allowed {
		if math.Trunc(f) == float64(a) {
			return a, Value{}
		}
	}
	return 0, Err(ErrValue)
}

func (m *xmatch) add(cols []Value) {
	m.pos++
	if m.err.IsErr() || !m.last && m.best > 0 && (m.mode == 0 || m.mode == 2) {
		return
	}
	v := cols[0]
	switch m.mode {
	case 0, 2:
		hit := sameClass(v, m.key) && cmp(v, m.key) == 0
		if m.mode == 2 && m.key.Kind == Text && v.Kind == Text {
			hit = Wildcard(m.key.Str, v.Str)
		}
		if hit {
			m.best = m.pos
		}
		return
	}
	if !sameClass(v, m.key) {
		return
	}
	c := cmp(v, m.key)
	if m.mode == -1 && c > 0 || m.mode == 1 && c < 0 {
		return
	}
	better := m.best == 0
	if !better {
		d := cmp(v, m.bestV)
		better = d == 0 && m.last || m.mode == -1 && d > 0 || m.mode == 1 && d < 0
	}
	if better {
		m.best, m.bestV = m.pos, v
	}
}

func (m *xmatch) result() Value {
	if m.err.IsErr() {
		return m.err
	}
	if m.best == 0 {
		return Err(ErrNA)
	}
	return Num(float64(m.best))
}

// cellLookup : RECHERCHEV ou RECHERCHEH sur une plage de cellules libres
// (D2:F20 tapée, ou Cell[D2:F20]) devient INDEX(colonne ou ligne renvoyée;
// EQUIV(clé; première colonne ou ligne; 0 ou 1)). Faux si la plage n'est pas
// une plage de cellules.
func cellLookup(c Call, horizontal, exact bool, original string) (Node, bool, error) {
	var r1, c1, r2, c2 int
	var sub func(a, b, x, y int) Node
	switch rg := c.Args[1].(type) {
	case Range:
		var ok bool
		if _, r1, c1, r2, c2, ok = splitA1Range(rg.Text); !ok {
			return nil, false, nil
		}
		prefix := ""
		if k := strings.LastIndex(rg.Text, "!"); k >= 0 {
			prefix = rg.Text[:k+1]
		}
		sub = func(a, b, x, y int) Node {
			return Range{prefix + ColumnLetters(b) + strconv.Itoa(a+1) + ":" + ColumnLetters(y) + strconv.Itoa(x+1)}
		}
	case CellRange:
		r1, c1, r2, c2 = rg.R1, rg.C1, rg.R2, rg.C2
		sub = func(a, b, x, y int) Node {
			return CellRange{Sheet: rg.Sheet, R1: a, C1: b, R2: x, C2: y, Home: rg.Home}
		}
	default:
		return nil, false, nil
	}
	num, ok := c.Args[2].(NumLit)
	k := int(num.V) - 1
	var keys, ret Node
	if horizontal {
		if !ok || num.V < 1 || r1+k > r2 {
			return nil, true, convErr("formula.convert.columnOutOfRange", nil)
		}
		keys, ret = sub(r1, c1, r1, c2), sub(r1+k, c1, r1+k, c2)
	} else {
		if !ok || num.V < 1 || c1+k > c2 {
			return nil, true, convErr("formula.convert.columnOutOfRange", nil)
		}
		keys, ret = sub(r1, c1, r2, c1), sub(r1, c1+k, r2, c1+k)
	}
	typ := NumLit{1}
	if exact {
		typ = NumLit{0}
	}
	match := Call{Fn: "MATCH", Args: []Node{c.Args[0], keys, typ}}
	return Call{Fn: "INDEX", Args: []Node{ret, match}, Origin: original}, true, nil
}

// lookupExact : le 4e argument de RECHERCHEV / RECHERCHEH demande une
// correspondance exacte (FAUX ou 0) ; omis ou VRAI : approchée.
func lookupExact(c Call) bool {
	return len(c.Args) == 4 && c.Args[3] != nil && (c.Args[3] == BoolLit{false} || c.Args[3] == NumLit{0})
}

// hlookup : RECHERCHEH sur des cellules libres ; sur un tableau, refusée
// (ses données sont en colonnes : REFERENCE ou RECHERCHEV).
func hlookup(c Call, original string) (Node, error) {
	out, ok, err := cellLookup(c, true, lookupExact(c), original)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, convErr("formula.convert.hlookup", nil)
	}
	return out, nil
}

// xlookupModes : RECHERCHEX avec un mode de correspondance ou de recherche
// autre que l'égalité depuis le début, ou sur des cellules libres, devient
// INDEX(renvoyée; EQUIVX(clé; cherchée; mode; recherche)). Faux pour
// l'égalité simple sur un tableau (REFERENCE, voir xlookup).
func xlookupModes(c Call, cells bool, original string) (Node, bool) {
	mode, search := Node(nil), Node(nil)
	if len(c.Args) > 4 && c.Args[4] != nil && c.Args[4] != (NumLit{0}) {
		mode = c.Args[4]
	}
	if len(c.Args) > 5 && c.Args[5] != nil && c.Args[5] != (NumLit{1}) {
		search = c.Args[5]
	}
	if mode == nil && search == nil && !cells {
		return nil, false
	}
	args := []Node{c.Args[0], c.Args[1]}
	switch {
	case search != nil:
		if mode == nil {
			mode = NumLit{0}
		}
		args = append(args, mode, search)
	case mode != nil:
		args = append(args, mode)
	}
	var out Node = Call{Fn: "INDEX", Args: []Node{c.Args[2], Call{Fn: "XMATCH", Args: args}}, Origin: original}
	if len(c.Args) > 3 && c.Args[3] != nil {
		out = Call{Fn: "IFNA", Args: []Node{out, c.Args[3]}}
	}
	return out, true
}

// isCells dit si un argument désigne des cellules libres : une plage A1 qui
// ne recoupe aucun tableau, ou Cell[…].
func isCells(n Node, r Resolver) bool {
	switch m := n.(type) {
	case CellRange:
		return true
	case Range:
		if r == nil {
			return false
		}
		_, _, ok := r.Resolve(m.Text)
		if ok {
			return false
		}
		_, _, _, _, _, ok = splitA1Range(m.Text)
		return ok
	}
	return false
}
