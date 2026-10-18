package formula

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"slices"
	"strings"
)

// Tableaux dynamiques (catalogue 2.7) : FILTRE, TRIER, TRIERPAR, UNIQUE,
// PRENDRE, EXCLURE, GROUPER.PAR et REQUETE rendent un tableau de valeurs qui
// se déverse sur la feuille (EvalSpill, CompileSpill), ou alimente un agrégat
// (SOMME(FILTRE(Ventes[Montant]; Ventes[Qté]>1))).
//
// Une source est un tableau (Ventes : toutes ses colonnes) ou une colonne
// (Ventes[Montant]). FILTRE et TRIERPAR évaluent leurs conditions et clés sur
// les lignes de ce tableau : leur source doit être un tableau ou une colonne,
// éventuellement déjà filtrés.

// IsSpill dit si une formule rend un tableau dynamique.
func IsSpill(n Node) bool {
	c, ok := n.(Call)
	if !ok {
		return false
	}
	if c.Fn == "LET" {
		body, ok := expandLet(c)
		return ok && IsSpill(body)
	}
	fn, ok := lookupFunc(c.Fn)
	return ok && fn.kind == spill
}

// sortKey donne une clé d'ordre et d'égalité commune aux deux backends :
// nombres < textes (sans casse) < booléens < erreurs ; nil pour une valeur
// vide (toujours classée en dernier).
func sortKey(v Value) []byte {
	switch v.Kind {
	case Number, Date:
		b := math.Float64bits(v.Num + 0) // + 0 : -0 devient 0
		if b>>63 == 1 {
			b = ^b
		} else {
			b |= 1 << 63
		}
		return binary.BigEndian.AppendUint64([]byte{0}, b)
	case Text:
		if v.Str == "" {
			return nil
		}
		return append([]byte{1}, strings.ToLower(v.Str)...)
	case Bool:
		return []byte{2, byte(boolInt(v.Bool))}
	case Error:
		return []byte{3, byte(v.Err)}
	}
	return nil
}

// compareKeys compare deux clés, les vides en dernier quel que soit le sens.
func compareKeys(a, b []byte, desc bool) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	}
	c := bytes.Compare(a, b)
	if desc {
		return -c
	}
	return c
}

// spillOrder lit le sens d'un tri : 1 (croissant, défaut) ou -1.
func spillOrder(v Value) (bool, Value) {
	if v.IsEmpty() {
		return false, Value{}
	}
	f, e := ToNumber(v)
	if e.IsErr() {
		return false, e
	}
	switch f {
	case 1:
		return false, Value{}
	case -1:
		return true, Value{}
	}
	return false, Err(ErrValue)
}

// spillCount lit un nombre de lignes (PRENDRE, EXCLURE, colonne de TRIER).
func spillCount(v Value) (int, Value) {
	f, e := ToNumber(v)
	if e.IsErr() {
		return 0, e
	}
	return int(math.Trunc(f)), Value{}
}

// groupAgg lit la fonction de GROUPER.PAR (SOMME, MOYENNE, NB…).
func groupAgg(n Node) (string, bool) {
	name, ok := n.(Name)
	if !ok {
		return "", false
	}
	fn, ok := lookupFunc(name.Name)
	if !ok || fn.kind != aggregate || aggLead[fn.EN] > 0 {
		return "", false
	}
	return fn.EN, true
}

// sourceOf lit une source : Tableau ou Tableau[Col].
func sourceOf(n Node, s *Schema, current *Table) (*Table, []int, bool) {
	switch src := n.(type) {
	case Name:
		if s == nil {
			return nil, nil, false
		}
		t, ok := s.Table(src.Name)
		if !ok {
			return nil, nil, false
		}
		cols := make([]int, len(t.Cols))
		for i := range cols {
			cols[i] = i
		}
		return t, cols, true
	case Ref:
		if src.Row {
			return nil, nil, false
		}
		t := current
		if src.Table != "" {
			var ok bool
			if s == nil {
				return nil, nil, false
			}
			if t, ok = s.Table(src.Table); !ok {
				return nil, nil, false
			}
		}
		if t == nil {
			return nil, nil, false
		}
		i, ok := t.col(src.Col)
		if !ok {
			return nil, nil, false
		}
		return t, []int{i}, true
	}
	return nil, nil, false
}

func colNames(t *Table, cols []int) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = t.Cols[c].Name
	}
	return out
}

// --- Évaluateur Go ------------------------------------------------------------

// rel est un tableau de valeurs. base et rows (indices dans base) sont
// renseignés tant que ses lignes sont celles d'un tableau, dans l'ordre :
// FILTRE et TRIERPAR peuvent alors évaluer des expressions sur ces lignes.
type rel struct {
	cols   []string
	values [][]Value
	base   *Table
	rows   []int
}

// EvalSpill évalue un tableau dynamique : en-têtes et lignes.
func EvalSpill(n Node, env *Env) ([]string, [][]Value, Value) {
	r, e := env.spill(n)
	if e.IsErr() {
		return nil, nil, e
	}
	return r.cols, r.values, Value{}
}

func (env *Env) spill(n Node) (rel, Value) {
	if t, cols, ok := sourceOf(n, env.Schema, env.Table); ok {
		r := rel{cols: colNames(t, cols), base: t}
		for row := range env.Data[t] {
			vals := make([]Value, len(cols))
			for i, c := range cols {
				vals[i] = env.cell(t, row, c)
			}
			r.values = append(r.values, vals)
			r.rows = append(r.rows, row)
		}
		return r, Value{}
	}
	c, ok := n.(Call)
	if !ok || !IsSpill(c) {
		return rel{}, Err(ErrValue)
	}
	arg := func(i int) Value {
		if i < len(c.Args) {
			return Eval(c.Args[i], env)
		}
		return Value{}
	}
	switch c.Fn {
	case "GROUPBY":
		return env.groupBy(c)
	case "SEQUENCE":
		return env.sequence(c)
	case "LET":
		body, _ := expandLet(c)
		return env.spill(body)
	}
	if c.Fn == "QUERY" {
		return rel{}, Err(ErrValue) // SQL libre : seulement dans le moteur SQL
	}
	src, e := env.spill(c.Args[0])
	if e.IsErr() {
		return rel{}, e
	}
	switch c.Fn {
	case "FILTER":
		if src.base == nil {
			return rel{}, Err(ErrValue)
		}
		out := rel{cols: src.cols, base: src.base}
		for k, row := range src.rows {
			keep, e := Truth(Eval(c.Args[1], env.inArray(src.base, row)))
			if e.IsErr() {
				return rel{}, e
			}
			if keep {
				out.values = append(out.values, src.values[k])
				out.rows = append(out.rows, row)
			}
		}
		if len(out.values) == 0 && len(c.Args) > 2 {
			out.values = [][]Value{padRow(arg(2), len(out.cols))}
			out.base, out.rows = nil, nil
		}
		return out, Value{}
	case "SORT":
		idx := 1
		if v := arg(1); !v.IsEmpty() {
			if idx, e = spillCount(v); e.IsErr() {
				return rel{}, e
			}
		}
		desc, e := spillOrder(arg(2))
		if e.IsErr() {
			return rel{}, e
		}
		if idx < 1 || idx > len(src.cols) || len(c.Args) > 3 && c.Args[3] != nil && c.Args[3] != (BoolLit{false}) {
			return rel{}, Err(ErrValue)
		}
		keys := make([][][]byte, len(src.values))
		for k, vals := range src.values {
			keys[k] = [][]byte{sortKey(vals[idx-1])}
		}
		return sortRel(rel{cols: src.cols, values: src.values}, keys, []bool{desc}), Value{}
	case "SORTBY":
		if src.base == nil || len(c.Args)%2 != 0 && len(c.Args) < 2 {
			return rel{}, Err(ErrValue)
		}
		var descs []bool
		var bys []Node
		for i := 1; i < len(c.Args); i += 2 {
			bys = append(bys, c.Args[i])
			d, e := spillOrder(arg(i + 1))
			if e.IsErr() {
				return rel{}, e
			}
			descs = append(descs, d)
		}
		keys := make([][][]byte, len(src.values))
		for k, row := range src.rows {
			for _, by := range bys {
				keys[k] = append(keys[k], sortKey(Eval(by, env.inArray(src.base, row))))
			}
		}
		return sortRel(src, keys, descs), Value{}
	case "UNIQUE":
		if len(c.Args) > 1 && c.Args[1] != nil && c.Args[1] != (BoolLit{false}) {
			return rel{}, Err(ErrValue) // comparaison par colonnes : non prise en charge
		}
		once, e := Truth(arg(2))
		if e.IsErr() {
			return rel{}, e
		}
		count := map[string]int{}
		var order []string
		first := map[string][]Value{}
		for _, vals := range src.values {
			var k []byte
			for _, v := range vals {
				k = append(append(k, byte(len(sortKey(v)))), sortKey(v)...)
			}
			if count[string(k)] == 0 {
				order = append(order, string(k))
				first[string(k)] = vals
			}
			count[string(k)]++
		}
		out := rel{cols: src.cols}
		for _, k := range order {
			if !once || count[k] == 1 {
				out.values = append(out.values, first[k])
			}
		}
		return out, Value{}
	case "TAKE", "DROP":
		if len(c.Args) > 2 && c.Args[2] != nil {
			return rel{}, Err(ErrValue)
		}
		n, e := spillCount(arg(1))
		if e.IsErr() {
			return rel{}, e
		}
		lo, hi := takeRange(c.Fn == "DROP", n, len(src.values))
		out := rel{cols: src.cols, values: src.values[lo:hi]}
		if src.base != nil {
			out.base, out.rows = src.base, src.rows[lo:hi]
		}
		return out, Value{}
	}
	return rel{}, Err(ErrValue)
}

// takeRange donne les lignes gardées par PRENDRE(n) ou EXCLURE(n) (n négatif :
// en partant de la fin).
func takeRange(drop bool, n, total int) (int, int) {
	switch {
	case !drop && n >= 0:
		return 0, min(n, total)
	case !drop:
		return max(total+n, 0), total
	case n >= 0:
		return min(n, total), total
	}
	return 0, max(total+n, 0)
}

func padRow(v Value, n int) []Value {
	row := make([]Value, max(n, 1))
	row[0] = v
	return row
}

func sortRel(r rel, keys [][][]byte, descs []bool) rel {
	idx := make([]int, len(r.values))
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(a, b int) int {
		for k, d := range descs {
			if c := compareKeys(keys[a][k], keys[b][k], d); c != 0 {
				return c
			}
		}
		return 0
	})
	out := rel{cols: r.cols, base: r.base}
	for _, i := range idx {
		out.values = append(out.values, r.values[i])
		if r.base != nil {
			out.rows = append(out.rows, r.rows[i])
		}
	}
	return out
}

// groupBy : GROUPER.PAR(Tableau[Clé]; Tableau[Valeur]; SOMME), une ligne par
// clé distincte (sans tenir compte de la casse), triée par clé.
func (env *Env) groupBy(c Call) (rel, Value) {
	fn, ok := groupAgg(c.Args[2])
	if !ok {
		return rel{}, Err(ErrValue)
	}
	t, e := arrayTable(c.Args[:2], env.Schema, env.Table)
	if e != 0 || t == nil {
		return rel{}, Err(ErrValue)
	}
	type group struct {
		key Value
		agg Agg
		k   []byte
	}
	var groups []*group
	byKey := map[string]*group{}
	for row := range env.Data[t] {
		sub := env.inArray(t, row)
		key := Eval(c.Args[0], sub)
		k := sortKey(key)
		g, ok := byKey[string(k)]
		if !ok {
			g = &group{key: key, agg: Agg{Fn: fn}, k: k}
			byKey[string(k)] = g
			groups = append(groups, g)
		}
		g.agg.AddRange(Eval(c.Args[1], sub))
	}
	slices.SortStableFunc(groups, func(a, b *group) int { return compareKeys(a.k, b.k, false) })
	out := rel{cols: []string{headerOf(c.Args[0]), headerOf(c.Args[1])}}
	for _, g := range groups {
		out.values = append(out.values, []Value{g.key, g.agg.Result()})
	}
	return out, Value{}
}

// headerOf donne l'en-tête d'une colonne de GROUPER.PAR.
func headerOf(n Node) string {
	if r, ok := n.(Ref); ok {
		return r.Col
	}
	return strings.TrimPrefix(Format(n, FR), "=")
}

// --- Compilation SQL ----------------------------------------------------------

// Spill est un tableau dynamique compilé : une requête qui rend ses lignes
// dans l'ordre (colonnes v1, v2…), et les en-têtes (nil pour REQUETE : ceux de
// la requête).
type Spill struct {
	SQL     string
	Args    []any
	Columns []string
}

// CompileSpill compile un tableau dynamique pour la lecture par la grille.
func CompileSpill(n Node, s *Schema, names map[string]Value) (sp Spill, fail Value) {
	defer func() {
		if r := recover(); r != nil { // voir compiler.top
			sp, fail = Spill{}, Err(ErrValue)
		}
	}()
	c := &compiler{schema: s, names: names, visiting: map[*Column]bool{}}
	r, e := c.spill(n, scope{})
	if e != 0 {
		return Spill{}, Err(e)
	}
	if r.query {
		return Spill{SQL: r.sql(), Args: c.args}, Value{}
	}
	vs := make([]string, len(r.cols))
	for i := range vs {
		vs[i] = fmt.Sprintf("v%d", i+1)
	}
	return Spill{SQL: "SELECT " + strings.Join(vs, ", ") + " FROM (" + r.sql() + ") ORDER BY _n", Args: c.args, Columns: r.cols}, Value{}
}

// sqlRel est un tableau dynamique en SQL : une requête aux colonnes v1…vk et
// _n (ordre). Tant que ses lignes sont celles d'un tableau (base), elle
// garde son alias, ses expressions et sa condition pour FILTRE et TRIERPAR.
type sqlRel struct {
	cols  []string
	base  *Table
	alias string
	exprs []string
	where []string
	order string // clé d'ordre de _n
	from  string // requête dérivée (base nil)
	query bool   // REQUETE : colonnes inconnues
}

func (r sqlRel) sql() string {
	if r.base == nil {
		return r.from
	}
	sel := make([]string, len(r.exprs))
	for i, e := range r.exprs {
		sel[i] = fmt.Sprintf("%s AS v%d", e, i+1)
	}
	q := "SELECT " + strings.Join(sel, ", ") + ", " + r.order + " AS _n FROM " + r.base.SQL + " AS " + r.alias
	if len(r.where) > 0 {
		q += " WHERE " + strings.Join(r.where, " AND ")
	}
	return q
}

func vcols(n int, prefix string) string {
	vs := make([]string, n)
	for i := range vs {
		vs[i] = fmt.Sprintf("%sv%d", prefix, i+1)
	}
	return strings.Join(vs, ", ")
}

func (c *compiler) spill(n Node, sc scope) (sqlRel, ErrCode) {
	if t, cols, ok := sourceOf(n, c.schema, sc.table); ok {
		al := c.newAlias()
		r := sqlRel{cols: colNames(t, cols), base: t, alias: al, order: al + "._pos"}
		for _, i := range cols {
			r.exprs = append(r.exprs, c.column(t, &t.Cols[i], al))
		}
		return r, 0
	}
	call, ok := n.(Call)
	if !ok || !IsSpill(call) {
		return sqlRel{}, ErrValue
	}
	arg := func(i int) string {
		if i < len(call.Args) && call.Args[i] != nil {
			return c.expr(call.Args[i], sc)
		}
		return "NULL"
	}
	switch call.Fn {
	case "SEQUENCE":
		return c.sequence(call, sc)
	case "LET":
		body, _ := expandLet(call)
		return c.spill(body, sc)
	case "QUERY":
		q, ok := call.Args[0].(StrLit)
		if !ok {
			return sqlRel{}, ErrValue
		}
		return sqlRel{from: "SELECT * FROM (" + q.V + ")", query: true}, 0
	case "GROUPBY":
		fn, ok := groupAgg(call.Args[2])
		t, e := arrayTable(call.Args[:2], c.schema, sc.table)
		if !ok || e != 0 || t == nil {
			return sqlRel{}, ErrValue
		}
		al := c.newAlias()
		in := sc.inArray(t, al)
		key, val := c.expr(call.Args[0], in), c.expr(call.Args[1], in)
		from := fmt.Sprintf("SELECT %s AS v1, slt_final(%s, slt_part(%s, %s)) AS v2, slt_sortkey(%s) AS _k FROM %s AS %s GROUP BY _k",
			key, c.param(fn), c.param(fn), val, key, t.SQL, al)
		return sqlRel{cols: []string{headerOf(call.Args[0]), headerOf(call.Args[1])},
			from: "SELECT v1, v2, row_number() OVER (ORDER BY _k IS NULL, _k) AS _n FROM (" + from + ")"}, 0
	}
	src, e := c.spill(call.Args[0], sc)
	if e != 0 {
		return sqlRel{}, e
	}
	if src.query {
		return sqlRel{}, ErrValue
	}
	k := len(src.cols)
	switch call.Fn {
	case "FILTER":
		if src.base == nil {
			return sqlRel{}, ErrValue
		}
		cond := c.expr(call.Args[1], sc.inArray(src.base, src.alias))
		out := src
		out.where = append(slices.Clone(src.where), "slt_true("+cond+")")
		if len(call.Args) > 2 && call.Args[2] != nil {
			// Valeur si vide : une ligne de remplacement quand le filtre ne garde rien.
			q := out.sql()
			fill := arg(2)
			for i := 1; i < k; i++ {
				fill += ", NULL"
			}
			return sqlRel{cols: src.cols, from: fmt.Sprintf("SELECT * FROM (%s) UNION ALL SELECT %s, 0 WHERE NOT EXISTS (%s)", q, fill, q)}, 0
		}
		return out, 0
	case "SORT":
		idx := 1
		if len(call.Args) > 1 && call.Args[1] != nil {
			lit, ok := call.Args[1].(NumLit)
			if !ok {
				return sqlRel{}, ErrValue
			}
			idx = int(lit.V)
		}
		if idx < 1 || idx > k || len(call.Args) > 3 && call.Args[3] != nil && call.Args[3] != (BoolLit{false}) {
			return sqlRel{}, ErrValue
		}
		order := orderSQL(fmt.Sprintf("slt_sortkey(v%d)", idx), arg(2))
		return sqlRel{cols: src.cols, from: fmt.Sprintf("SELECT %s, row_number() OVER (ORDER BY %s, _n) AS _n FROM (%s)", vcols(k, ""), order, src.sql())}, 0
	case "SORTBY":
		if src.base == nil {
			return sqlRel{}, ErrValue
		}
		in := sc.inArray(src.base, src.alias)
		var keys []string
		for i := 1; i < len(call.Args); i += 2 {
			keys = append(keys, orderSQL("slt_sortkey("+c.expr(call.Args[i], in)+")", arg(i+1)))
		}
		out := src
		out.order = "row_number() OVER (ORDER BY " + strings.Join(keys, ", ") + ", " + src.alias + "._pos)"
		return out, 0
	case "UNIQUE":
		if len(call.Args) > 1 && call.Args[1] != nil && call.Args[1] != (BoolLit{false}) {
			return sqlRel{}, ErrValue
		}
		var keys []string
		for i := 1; i <= k; i++ {
			keys = append(keys, fmt.Sprintf("slt_sortkey(v%d)", i))
		}
		having := ""
		if len(call.Args) > 2 && call.Args[2] != nil {
			having = " HAVING NOT slt_true(" + arg(2) + ") OR count(*) = 1"
		}
		return sqlRel{cols: src.cols, from: fmt.Sprintf("SELECT %s, min(_n) AS _n FROM (%s) GROUP BY %s%s", vcols(k, ""), src.sql(), strings.Join(keys, ", "), having)}, 0
	case "TAKE", "DROP":
		if len(call.Args) > 2 && call.Args[2] != nil {
			return sqlRel{}, ErrValue
		}
		drop := 0
		if call.Fn == "DROP" {
			drop = 1
		}
		return sqlRel{cols: src.cols, from: fmt.Sprintf(
			"SELECT %s, _n FROM (SELECT *, row_number() OVER (ORDER BY _n) AS _r, count(*) OVER () AS _c FROM (%s)) WHERE slt_take(_r, _c, %s, %d)",
			vcols(k, ""), src.sql(), arg(1), drop)}, 0
	}
	return sqlRel{}, ErrValue
}

// orderSQL : clé de tri SQL, les vides en dernier ; sens -1 : décroissant.
func orderSQL(key, dir string) string {
	return fmt.Sprintf("%s IS NULL, CASE WHEN slt_desc(%s) THEN NULL ELSE %s END, CASE WHEN slt_desc(%s) THEN %s END DESC", key, dir, key, dir, key)
}

// spillAggregate compile un agrégat sur un tableau dynamique : toutes ses
// valeurs, ligne par ligne.
func (c *compiler) spillAggregate(fn string, n Node, sc scope) string {
	r, e := c.spill(n, sc)
	if e != 0 || r.query {
		return errSQL(ErrValue)
	}
	var parts []string
	for i := range r.cols {
		parts = append(parts, fmt.Sprintf("SELECT v%d AS v, _n, %d AS _i FROM (%s)", i+1, i, r.sql()))
	}
	return fmt.Sprintf("(SELECT slt_part(%s, v ORDER BY _n, _i) FROM (%s))", c.param(fn), strings.Join(parts, " UNION ALL "))
}
