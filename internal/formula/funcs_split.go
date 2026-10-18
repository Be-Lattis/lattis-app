package formula

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// FRACTIONNER.TEXTE (TEXTSPLIT) et constantes matricielles {…} : deux
// tableaux de valeurs qui se déversent (voir spill.go).
//
// FRACTIONNER.TEXTE(texte; délimiteur_col; [délimiteur_ligne];
// [ignorer_vides]; [respect_casse]; [remplir_avec]) découpe un texte en
// lignes puis en colonnes ; les lignes plus courtes que la plus longue sont
// complétées par #N/A (ou remplir_avec). Un délimiteur peut être une
// constante matricielle ({",";";"}) : chacun de ses textes sépare.
//
// Sa forme (nombre de colonnes) dépend du texte : le moteur SQL, qui fixe
// ses colonnes à la compilation, calcule donc le tableau à la compilation
// quand ses arguments ne dépendent pas d'une ligne (littéraux et cellules,
// ou valeurs précalculées par hoist). Sinon (texte d'une colonne de la
// ligne), le résultat n'a pas de forme connue : il alimente un agrégat
// (NB(FRACTIONNER.TEXTE([Tags]; ","))), valeur par valeur, mais ne se
// déverse pas et ne passe pas par TRIER, PRENDRE… (#VALEUR!).

// maxConstCells borne un tableau calculé à la compilation (paramètres SQL).
const maxConstCells = 20000

// splitOpts : arguments évalués de FRACTIONNER.TEXTE.
type splitOpts struct {
	text, ignore, mode, pad Value
	cols, rows              []Value // délimiteurs
}

// splitArgs évalue les arguments de FRACTIONNER.TEXTE avec ev.
func splitArgs(c Call, ev func(Node) Value) splitOpts {
	arg := func(i int) Node {
		if i < len(c.Args) {
			return c.Args[i]
		}
		return nil
	}
	o := splitOpts{text: ev(arg(0)), ignore: ev(arg(3)), mode: ev(arg(4)), pad: Err(ErrNA)}
	if p := arg(5); p != nil {
		o.pad = ev(p)
	}
	o.cols, o.rows = delimValues(arg(1), ev), delimValues(arg(2), ev)
	return o
}

// delimValues : un délimiteur, ou chaque valeur d'une constante matricielle.
func delimValues(n Node, ev func(Node) Value) []Value {
	switch n := n.(type) {
	case nil:
		return nil
	case ArrayLit:
		var out []Value
		for _, r := range n.Rows {
			for _, x := range r {
				out = append(out, ev(x))
			}
		}
		return out
	}
	return []Value{ev(n)}
}

// delimTexts lit des délimiteurs ; les textes vides sont ignorés.
func delimTexts(vs []Value) ([]string, Value) {
	var out []string
	for _, v := range vs {
		s, e := ToText(v)
		if e.IsErr() {
			return nil, e
		}
		if s != "" {
			out = append(out, s)
		}
	}
	return out, Value{}
}

// textSplit calcule FRACTIONNER.TEXTE : des lignes de même longueur.
func textSplit(o splitOpts) ([][]Value, Value) {
	s, e := ToText(o.text)
	if e.IsErr() {
		return nil, e
	}
	cd, e := delimTexts(o.cols)
	if e.IsErr() {
		return nil, e
	}
	rd, e := delimTexts(o.rows)
	if e.IsErr() {
		return nil, e
	}
	if len(cd) == 0 && len(rd) == 0 {
		return nil, Err(ErrValue) // aucun délimiteur
	}
	ignore := false
	if !o.ignore.IsEmpty() {
		if ignore, e = Truth(o.ignore); e.IsErr() {
			return nil, e
		}
	}
	nocase := false
	if !o.mode.IsEmpty() {
		f, e := ToNumber(o.mode)
		if e.IsErr() {
			return nil, e
		}
		switch f {
		case 0:
		case 1:
			nocase = true
		default:
			return nil, Err(ErrValue)
		}
	}
	lines := []string{s}
	if len(rd) > 0 {
		lines = splitOn(s, rd, nocase)
	}
	var out [][]Value
	width := 0
	for _, l := range lines {
		if ignore && l == "" {
			continue
		}
		parts := []string{l}
		if len(cd) > 0 {
			parts = splitOn(l, cd, nocase)
		}
		var row []Value
		for _, p := range parts {
			if ignore && p == "" {
				continue
			}
			row = append(row, Str(p))
		}
		out = append(out, row)
		width = max(width, len(row))
	}
	if width == 0 {
		return nil, Err(ErrValue) // tableau vide (#CALC! dans Excel)
	}
	for i, r := range out {
		for len(r) < width {
			r = append(r, o.pad)
		}
		out[i] = r
	}
	return out, Value{}
}

// splitOn découpe s à chaque délimiteur ; à une même position, le plus long
// l'emporte. nocase : sans tenir compte de la casse.
func splitOn(s string, delims []string, nocase bool) []string {
	rs := []rune(s)
	ds := make([][]rune, len(delims))
	for i, d := range delims {
		ds[i] = []rune(d)
		if nocase {
			for k, r := range ds[i] {
				ds[i][k] = unicode.ToLower(r)
			}
		}
	}
	var out []string
	start := 0
	for i := 0; i < len(rs); {
		best := 0
		for _, d := range ds {
			if len(d) > best && matchAt(rs, i, d, nocase) {
				best = len(d)
			}
		}
		if best == 0 {
			i++
			continue
		}
		out = append(out, string(rs[start:i]))
		i += best
		start = i
	}
	return append(out, string(rs[start:]))
}

func matchAt(rs []rune, i int, d []rune, nocase bool) bool {
	if i+len(d) > len(rs) {
		return false
	}
	for k, r := range d {
		x := rs[i+k]
		if nocase {
			x = unicode.ToLower(x)
		}
		if x != r {
			return false
		}
	}
	return true
}

// constNode : une valeur qui ne dépend que de littéraux et de cellules,
// calculable sans base de données (Eval avec les valeurs des cellules).
func constNode(n Node, s *Schema) bool {
	switch n := n.(type) {
	case nil, NumLit, StrLit, BoolLit, ErrLit, ArrayLit:
		return true
	case Name:
		if s != nil {
			if _, ok := s.Table(n.Name); ok {
				return false
			}
		}
		return true
	case Unary:
		return constNode(n.X, s)
	case Binary:
		return constNode(n.L, s) && constNode(n.R, s)
	case Call:
		fn, ok := lookupFunc(n.Fn)
		if !ok || fn.kind != scalar {
			return false
		}
		for _, a := range n.Args {
			if !constNode(a, s) {
				return false
			}
		}
		return true
	}
	return false
}

func constArgs(c Call, s *Schema) bool {
	for _, a := range c.Args {
		if !constNode(a, s) {
			return false
		}
	}
	return true
}

// arrayValues donne les valeurs d'une constante matricielle.
func arrayValues(a ArrayLit) [][]Value {
	out := make([][]Value, len(a.Rows))
	for i, r := range a.Rows {
		out[i] = make([]Value, len(r))
		for j, x := range r {
			out[i][j] = Eval(x, &Env{})
		}
	}
	return out
}

// --- Évaluateur Go ------------------------------------------------------------

// textSplit évalue FRACTIONNER.TEXTE. Comme en SQL, un résultat dont les
// arguments dépendent de la ligne n'a pas de forme (long).
func (env *Env) textSplit(c Call) (rel, Value) {
	vals, e := textSplit(splitArgs(c, func(n Node) Value { return Eval(n, env) }))
	if e.IsErr() {
		return rel{}, e
	}
	fixed := env.arr == nil && !rowDependent(c) || env.Table == nil && constArgs(c, env.Schema)
	return rel{cols: seqHeaders(len(vals[0])), values: vals, long: !fixed}, Value{}
}

// --- Compilation SQL ----------------------------------------------------------

// constRel compile un tableau de valeurs connues : une requête VALUES.
func (c *compiler) constRel(vals [][]Value) (sqlRel, ErrCode) {
	k := len(vals[0])
	if len(vals)*k > maxConstCells {
		return sqlRel{}, ErrValue
	}
	rows := make([]string, len(vals))
	for i, r := range vals {
		parts := []string{strconv.Itoa(i + 1)}
		for _, v := range r {
			if v.IsErr() {
				parts = append(parts, errSQL(v.Err))
			} else {
				parts = append(parts, c.param(ToSQL(v)))
			}
		}
		rows[i] = "(" + strings.Join(parts, ", ") + ")"
	}
	sel := make([]string, k)
	for j := range sel {
		sel[j] = fmt.Sprintf("column%d AS v%d", j+2, j+1)
	}
	return sqlRel{cols: seqHeaders(k), from: "SELECT " + strings.Join(sel, ", ") + ", column1 AS _n FROM (VALUES " + strings.Join(rows, ", ") + ")"}, 0
}

// textSplit compile FRACTIONNER.TEXTE : calculé à la compilation si ses
// arguments ne dépendent pas de la ligne, sinon valeur par valeur
// (slt_tsplit sur les rangs donnés par slt_tsplitn), sans forme.
func (c *compiler) textSplit(call Call, sc scope) (sqlRel, ErrCode) {
	var ev func(Node) Value
	switch {
	case sc.table == nil && constArgs(call, c.schema):
		env := &Env{Schema: c.schema, Names: c.names}
		ev = func(n Node) Value { return Eval(n, env) }
	case c.hoist != nil && sc.arr == nil && !rowDependent(call):
		ev = func(n Node) Value {
			if sc.table == nil && constNode(n, c.schema) {
				return Eval(n, &Env{Schema: c.schema, Names: c.names})
			}
			sub := c.sub()
			sql := sub.expr(n, scope{table: sc.table})
			return c.hoist(Compiled{sql, sub.args})
		}
	}
	if ev != nil {
		vals, e := textSplit(splitArgs(call, ev))
		if e.IsErr() {
			return sqlRel{}, e.Err
		}
		return c.constRel(vals)
	}
	arg := func(i int) string {
		if i < len(call.Args) && call.Args[i] != nil {
			return c.expr(call.Args[i], sc)
		}
		return "NULL"
	}
	delims := func(i int) []string {
		var out []string
		if i >= len(call.Args) {
			return nil
		}
		for _, n := range delimNodes(call.Args[i]) {
			out = append(out, c.expr(n, sc))
		}
		return out
	}
	pad := errSQL(ErrNA)
	if len(call.Args) > 5 && call.Args[5] != nil {
		pad = arg(5)
	}
	cols, rows := delims(1), delims(2)
	parts := append([]string{arg(0), arg(3), arg(4), pad, strconv.Itoa(len(cols))}, cols...)
	a := strings.Join(append(parts, rows...), ", ")
	return sqlRel{cols: []string{"Valeur"}, long: true,
		from: fmt.Sprintf("SELECT slt_tsplit(j.key, %s) AS v1, j.key AS _n FROM json_each(slt_tsplitn(%s)) AS j", a, a)}, 0
}

// delimNodes : les nœuds d'un argument délimiteur (ceux d'une constante
// matricielle, ou l'argument lui-même).
func delimNodes(n Node) []Node {
	switch n := n.(type) {
	case nil:
		return nil
	case ArrayLit:
		var out []Node
		for _, r := range n.Rows {
			out = append(out, r...)
		}
		return out
	}
	return []Node{n}
}

// tsplitSQL lit les arguments de slt_tsplit et slt_tsplitn : texte,
// ignorer_vides, respect_casse, remplir_avec, nombre de délimiteurs de
// colonne, ces délimiteurs, puis ceux de ligne.
func tsplitSQL(a []Value) ([][]Value, Value) {
	if len(a) < 5 {
		return nil, Err(ErrValue)
	}
	n := int(a[4].Num)
	if n < 0 || 5+n > len(a) {
		return nil, Err(ErrValue)
	}
	return textSplit(splitOpts{text: a[0], ignore: a[1], mode: a[2], pad: a[3], cols: a[5 : 5+n], rows: a[5+n:]})
}

// tsplitCount : nombre de valeurs de FRACTIONNER.TEXTE (une erreur compte
// pour une valeur).
func tsplitCount(a []Value) int {
	vals, e := tsplitSQL(a)
	if e.IsErr() {
		return 1
	}
	return len(vals) * len(vals[0])
}

// tsplitAt : valeur n° i (lignes à la suite) de FRACTIONNER.TEXTE.
func tsplitAt(i int, a []Value) Value {
	vals, e := tsplitSQL(a)
	if e.IsErr() {
		return e
	}
	w := len(vals[0])
	if i < 0 || i >= len(vals)*w {
		return Value{}
	}
	return vals[i/w][i%w]
}
