package formula

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

// Fonctions ajoutées après la revue n° 2 (point 24) : JOURS, GRANDE.VALEUR,
// PETITE.VALEUR, CHOISIR, NO.SEMAINE, PLAFOND, PLANCHER, TEMPS, LET et
// SEQUENCE.

// --- Scalaires ----------------------------------------------------------------

// fnDays : JOURS(date_fin; date_début), nombre de jours entre deux dates
// (parties entières, comme Excel).
func fnDays(a []Value) Value {
	end, e := toSerial(a[0])
	if e.IsErr() {
		return e
	}
	start, e := toSerial(a[1])
	if e.IsErr() {
		return e
	}
	return Num(math.Floor(end) - math.Floor(start))
}

// fnChoose : CHOISIR(no_index; valeur1; valeur2…).
func fnChoose(a []Value) Value {
	f, e := ToNumber(a[0])
	if e.IsErr() {
		return e
	}
	i := int(math.Trunc(f))
	if i < 1 || i >= len(a) {
		return Err(ErrValue)
	}
	return a[i]
}

// fnWeeknum : NO.SEMAINE(date; [type]). La semaine 1 contient le 1er janvier ;
// le type donne le premier jour de la semaine (1 ou 17 : dimanche, 2 ou 11 :
// lundi, 12 à 16 : mardi à samedi) ; 21 : semaine ISO.
func fnWeeknum(a []Value) Value {
	t, e := dateArg(a[0])
	if e.IsErr() {
		return e
	}
	mode := 1
	if len(a) > 1 && !a[1].IsEmpty() {
		m, e := ToNumber(a[1])
		if e.IsErr() {
			return e
		}
		mode = int(m)
	}
	var first int // premier jour de la semaine (0 = dimanche)
	switch {
	case mode == 1 || mode == 17:
		first = 0
	case mode == 2 || mode == 11:
		first = 1
	case mode >= 12 && mode <= 16:
		first = mode - 10
	case mode == 21:
		_, w := t.ISOWeek()
		return Num(float64(w))
	default:
		return Err(ErrNum)
	}
	jan1 := time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	off := (int(jan1.Weekday()) - first + 7) % 7
	return Num(float64((t.YearDay()-1+off)/7 + 1))
}

// multiple lit les arguments de PLAFOND et PLANCHER : le nombre, le multiple
// (1 si omis) et leur quotient corrigé de la représentation binaire
// (15 chiffres significatifs, comme ARRONDI).
func multiple(a []Value) (x, sig, q float64, err Value) {
	x, e := ToNumber(a[0])
	if e.IsErr() {
		return 0, 0, 0, e
	}
	sig = 1
	if len(a) > 1 && !a[1].IsEmpty() {
		if sig, e = ToNumber(a[1]); e.IsErr() {
			return 0, 0, 0, e
		}
	}
	if x > 0 && sig < 0 {
		return 0, 0, 0, Err(ErrNum)
	}
	if sig != 0 {
		q, _ = parseNumber(FormatNumber(x / sig))
	}
	return x, sig, q, Value{}
}

// fnCeiling : PLAFOND(nombre; [précision]), arrondi au multiple supérieur
// (au multiple qui s'éloigne de zéro pour un nombre et un multiple négatifs).
func fnCeiling(a []Value) Value {
	_, sig, q, e := multiple(a)
	switch {
	case e.IsErr():
		return e
	case sig == 0:
		return Num(0)
	}
	// Multiple négatif (nombre alors négatif ou nul) : q est positif, son
	// plafond multiplié par sig s'éloigne de zéro.
	return number(math.Ceil(q) * sig)
}

// fnFloor : PLANCHER(nombre; [précision]), arrondi au multiple inférieur
// (vers zéro pour un nombre et un multiple négatifs).
func fnFloor(a []Value) Value {
	_, sig, q, e := multiple(a)
	switch {
	case e.IsErr():
		return e
	case sig == 0:
		return Err(ErrDiv0)
	}
	return number(math.Floor(q) * sig)
}

// fnTime : TEMPS(heure; minute; seconde), fraction de journée (0,5 = midi).
func fnTime(a []Value) Value {
	n, e := intArgs(a)
	if e.IsErr() {
		return e
	}
	secs := n[0]*3600 + n[1]*60 + n[2]
	if secs < 0 {
		return Err(ErrNum)
	}
	return Num(float64(secs%86400) / 86400)
}

// --- GRANDE.VALEUR et PETITE.VALEUR -------------------------------------------

// kth : k-ième plus grande (ou plus petite) valeur numérique d'une colonne ;
// textes, booléens et vides sont ignorés, une erreur est propagée.
type kth struct {
	k       Value
	largest bool
	vals    []Value
	err     Value
	numRule
}

func newKth(largest bool) func([]Value) scanner {
	return func(p []Value) scanner { return &kth{k: p[0], largest: largest} }
}

func (s *kth) add(cols []Value) {
	switch v := s.coerce(cols[0]); v.Kind {
	case Number, Date:
		s.vals = append(s.vals, v)
	case Error:
		if !s.err.IsErr() {
			s.err = v
		}
	}
}

func (s *kth) result() Value {
	if s.err.IsErr() {
		return s.err
	}
	f, e := ToNumber(s.k)
	if e.IsErr() {
		return e
	}
	k := int(math.Ceil(f))
	if k < 1 || k > len(s.vals) {
		return Err(ErrNum)
	}
	slices.SortStableFunc(s.vals, func(a, b Value) int {
		if s.largest {
			return cmpFloat(b.Num, a.Num)
		}
		return cmpFloat(a.Num, b.Num)
	})
	return s.vals[k-1]
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// --- LET ----------------------------------------------------------------------

// expandLet remplace LET(nom1; valeur1; …; calcul) par son calcul, où chaque
// nom est remplacé par l'expression de sa valeur : les deux backends
// l'évaluent alors comme n'importe quelle formule. Le résultat est celui
// d'une valeur calculée une fois : les tirages d'ALEA d'une valeur reçoivent
// leur rang ici (next donne le rang suivant de la formule ; nil : pas de
// rang, pour la seule forme du calcul), si bien qu'un nom utilisé deux fois
// donne deux fois le même nombre.
func expandLet(c Call, next func() int) (Node, bool) {
	if len(c.Args) < 3 || len(c.Args)%2 == 0 {
		return nil, false
	}
	bind := map[string]Node{}
	for i := 0; i+1 < len(c.Args); i += 2 {
		name, ok := letName(c.Args[i])
		if !ok {
			return nil, false
		}
		v := substNames(c.Args[i+1], bind)
		if next != nil {
			v = pinRands(v, next)
		}
		bind[name] = v
	}
	return substNames(c.Args[len(c.Args)-1], bind), true
}

// pinRands donne un rang fixe aux tirages d'ALEA (et ALEA.ENTRE.BORNES) qui
// n'en ont pas encore, dans l'ordre de la formule.
func pinRands(n Node, next func() int) Node {
	switch n := n.(type) {
	case Unary:
		return Unary{Op: n.Op, X: pinRands(n.X, next)}
	case Binary:
		return Binary{Op: n.Op, L: pinRands(n.L, next), R: pinRands(n.R, next)}
	case Call:
		out := n
		if (n.Fn == "RAND" || n.Fn == "RANDBETWEEN") && n.rank == 0 {
			out.rank = next() + 1
		}
		out.Args = make([]Node, len(n.Args))
		for i, a := range n.Args {
			out.Args[i] = pinRands(a, next)
		}
		return out
	}
	return n
}

// letName lit un nom de LET : un identifiant qui n'est ni une adresse de
// cellule, ni une référence à une autre feuille, ni un nom de fonction.
func letName(n Node) (string, bool) {
	nm, ok := n.(Name)
	if !ok || nm.Name == "" || strings.ContainsAny(nm.Name, ".!#") {
		return "", false
	}
	if _, _, ok := CellRef(nm.Name); ok {
		return "", false
	}
	if _, _, _, ok := A1Address(nm.Name); ok {
		return "", false // B2, AB12 : une adresse à la Excel
	}
	// Noms anglais et français seulement : les noms allemands et espagnols
	// (Y, O, TAG, WERT…) restent des noms de LET possibles, comme avant
	// qu'ils soient au catalogue.
	if f, ok := lookupFunc(nm.Name); ok && (strings.EqualFold(nm.Name, f.EN) || strings.EqualFold(nm.Name, f.FR)) {
		return "", false
	}
	return strings.ToLower(nm.Name), true
}

// substNames remplace les noms liés par leurs expressions. Un LET imbriqué qui
// redéfinit un nom le masque pour la suite de ses arguments.
func substNames(n Node, bind map[string]Node) Node {
	if len(bind) == 0 {
		return n
	}
	switch n := n.(type) {
	case Name:
		if v, ok := bind[strings.ToLower(n.Name)]; ok {
			return v
		}
		return n
	case Unary:
		return Unary{Op: n.Op, X: substNames(n.X, bind)}
	case Binary:
		return Binary{Op: n.Op, L: substNames(n.L, bind), R: substNames(n.R, bind)}
	case Call:
		args := make([]Node, len(n.Args))
		if n.Fn == "LET" {
			inner := bind
			for i, a := range n.Args {
				if i%2 == 0 && i+1 < len(n.Args) {
					if name, ok := letName(a); ok {
						args[i] = a
						if _, shadow := inner[name]; shadow {
							inner = mapsWithout(inner, name)
						}
						continue
					}
				}
				args[i] = substNames(a, inner)
			}
			return Call{Fn: n.Fn, Args: args, Origin: n.Origin, rank: n.rank}
		}
		skip := 0
		switch n.Fn {
		case "REFERENCE", "REFERENCE.APPROX":
			skip = 2 // tableau et colonne : des noms, pas des valeurs
		case "EXISTS":
			skip = 1
		}
		for i, a := range n.Args {
			if i < skip {
				args[i] = a
			} else {
				args[i] = substNames(a, bind)
			}
		}
		return Call{Fn: n.Fn, Args: args, Origin: n.Origin, rank: n.rank}
	}
	return n
}

func mapsWithout(m map[string]Node, key string) map[string]Node {
	out := make(map[string]Node, len(m))
	for k, v := range m {
		if k != key {
			out[k] = v
		}
	}
	return out
}

// --- SEQUENCE -----------------------------------------------------------------

// seqMax borne le nombre de valeurs d'une SEQUENCE.
const seqMax = 1_000_000

// seqCols lit le nombre de colonnes de SEQUENCE : il fixe la forme du
// résultat, il doit donc être un nombre écrit dans la formule (1 si omis).
func seqCols(c Call) (int, ErrCode) {
	if len(c.Args) < 2 || c.Args[1] == nil {
		return 1, 0
	}
	lit, ok := c.Args[1].(NumLit)
	if !ok || lit.V < 1 || lit.V > 1000 {
		return 0, ErrValue
	}
	return int(lit.V), 0
}

// seqRows lit le nombre de lignes de SEQUENCE ; -1 s'il ne convient pas.
func seqRows(v Value, cols int) int {
	f, e := ToNumber(v)
	if e.IsErr() || f < 1 || math.Trunc(f)*float64(cols) > seqMax {
		return -1
	}
	return int(math.Trunc(f))
}

func seqHeaders(k int) []string {
	if k == 1 {
		return []string{"Valeur"}
	}
	out := make([]string, k)
	for i := range out {
		out[i] = fmt.Sprintf("Valeur%d", i+1)
	}
	return out
}

// seqValue : valeur n° i (à partir de 0) d'une SEQUENCE.
func seqValue(start, step Value, i int) Value {
	return Arith('+', start, Arith('*', step, Num(float64(i))))
}

// seqArgs donne le début et le pas (1 si omis).
func seqArgs(c Call) (Node, Node) {
	var start, step Node = NumLit{1}, NumLit{1}
	if len(c.Args) > 2 && c.Args[2] != nil {
		start = c.Args[2]
	}
	if len(c.Args) > 3 && c.Args[3] != nil {
		step = c.Args[3]
	}
	return start, step
}

// sequence évalue SEQUENCE(lignes; [colonnes]; [début]; [pas]).
func (env *Env) sequence(c Call) (rel, Value) {
	k, e := seqCols(c)
	if e != 0 {
		return rel{}, Err(e)
	}
	n := seqRows(Eval(c.Args[0], env), k)
	if n < 0 {
		return rel{}, Err(ErrValue)
	}
	s, st := seqArgs(c)
	start, step := Eval(s, env), Eval(st, env)
	out := rel{cols: seqHeaders(k)}
	for r := 0; r < n; r++ {
		row := make([]Value, k)
		for j := range row {
			row[j] = seqValue(start, step, r*k+j)
		}
		out.values = append(out.values, row)
	}
	return out, Value{}
}

// sequence compile SEQUENCE : une suite récursive de numéros de ligne, et les
// valeurs calculées par slt_seq. Un nombre de lignes qui ne convient pas
// donne une seule ligne #VALEUR!.
func (c *compiler) sequence(call Call, sc scope) (sqlRel, ErrCode) {
	k, e := seqCols(call)
	if e != 0 {
		return sqlRel{}, e
	}
	s, st := seqArgs(call)
	count := fmt.Sprintf("slt_seqn(%s, %d)", c.expr(call.Args[0], sc), k)
	start, step := c.expr(s, sc), c.expr(st, sc)
	cte := c.newAlias()
	vals := make([]string, k)
	for j := range vals {
		vals[j] = fmt.Sprintf("slt_seq(%s, %s, %s, (_n - 1) * %d + %d) AS v%d", count, start, step, k, j, j+1)
	}
	from := fmt.Sprintf("SELECT %s, _n FROM (WITH RECURSIVE %s(_n) AS (SELECT 1 UNION ALL SELECT _n + 1 FROM %s WHERE _n < %s) SELECT _n FROM %s)",
		strings.Join(vals, ", "), cte, cte, count, cte)
	return sqlRel{cols: seqHeaders(k), from: from}, 0
}
