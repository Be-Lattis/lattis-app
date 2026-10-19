package formula

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/ncruces/go-sqlite3"
)

// Totaux par valeur pour la famille .SI.
//
// Dans une colonne calculée, NB.SI(Ventes[Client]; [@Client]) parcourt tout
// le tableau à chaque ligne : une fenêtre de 200 lignes d'un tableau de
// 100 000 lignes demande 20 millions de tests. Quand le critère d'une ligne
// est une égalité simple (une valeur, et non ">5" ou "Dur*"), le résultat ne
// dépend que de cette valeur : un seul parcours calcule les totaux de chaque
// valeur (slt_group, sorti de la requête comme les autres agrégats), et
// chaque ligne lit le sien (slt_grouped). Une ligne dont le critère n'est pas
// une égalité simple reprend le parcours complet.
//
// Égalité « simple » : celle de criterion.match. Un texte est comparé sans
// casse (clé sortKey) ; un nombre ou une date correspond aussi aux textes qui
// s'écrivent comme ce nombre (« 10 » pour 10), rangés à part (préfixe '#').

// critKey donne la clé d'un critère d'égalité simple, ou false.
func critKey(v Value) (string, bool) {
	c := parseCriterion(v)
	if c.err.IsErr() || c.op != "=" || c.wild {
		return "", false
	}
	switch c.val.Kind {
	case Number, Date:
		return string(sortKey(Num(c.val.Num))), true
	case Text, Bool:
		return string(sortKey(c.val)), true
	}
	return "", false // vide, erreur : parcours complet
}

// cellKeys donne les clés sous lesquelles ranger une valeur de la plage.
func cellKeys(v Value) []string {
	switch v.Kind {
	case Number, Date:
		return []string{string(sortKey(Num(v.Num)))}
	case Bool:
		return []string{string(sortKey(v))}
	case Text:
		if v.Str == "" {
			return nil
		}
		keys := []string{string(sortKey(v))}
		if f, ok := parseNumber(v.Str); ok {
			keys = append(keys, "#"+string(sortKey(Num(f))))
		}
		return keys
	}
	return nil
}

// grouped est le résultat d'un parcours : un accumulateur par combinaison de
// clés des critères qui dépendent de la ligne.
type grouped struct {
	tmpl   *cond // critères constants, colonnes, agrégat
	rowDep []int // indices des critères qui dépendent de la ligne
	groups map[string]*cond
}

// tupleKey joint des clés (préfixées de leur longueur).
func tupleKey(keys []string) string {
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%d:%s", len(k), k)
	}
	return b.String()
}

func (g *grouped) add(cols []Value) {
	t := g.tmpl
	for i, cr := range t.crits {
		if !isIn(i, g.rowDep) && (cr.err.IsErr() || !cr.match(cols[t.cols[i]])) {
			return
		}
	}
	// Toutes les combinaisons de clés (une valeur peut en avoir deux).
	combos := [][]string{nil}
	for _, i := range g.rowDep {
		keys := cellKeys(cols[t.cols[i]])
		var next [][]string
		for _, c := range combos {
			for _, k := range keys {
				next = append(next, append(append([]string(nil), c...), k))
			}
		}
		combos = next
	}
	for _, c := range combos {
		k := tupleKey(c)
		acc, ok := g.groups[k]
		if !ok {
			acc = &cond{value: t.value, agg: newAgg(t.agg.Fn, t.agg.rule)}
			g.groups[k] = acc
		}
		acc.addValue(cols)
	}
}

// addValue ajoute une ligne retenue (sans tester les critères).
func (c *cond) addValue(cols []Value) {
	switch c.value {
	case -1:
		c.count++
	case -2:
		c.agg.AddRange(cols[len(cols)-1])
	default:
		c.agg.AddRange(cols[c.value])
	}
}

// lookup donne le résultat pour les critères d'une ligne, ou false si l'un
// d'eux n'est pas une égalité simple.
func (g *grouped) lookup(crits []Value) (Value, bool) {
	combos := [][]string{nil}
	for _, v := range crits {
		k, ok := critKey(v)
		if !ok {
			return Value{}, false
		}
		alts := []string{k}
		if c := parseCriterion(v); c.val.Kind == Number || c.val.Kind == Date {
			alts = append(alts, "#"+k)
		}
		var next [][]string
		for _, c := range combos {
			for _, a := range alts {
				next = append(next, append(append([]string(nil), c...), a))
			}
		}
		combos = next
	}
	out := &cond{value: g.tmpl.value, agg: newAgg(g.tmpl.agg.Fn, g.tmpl.agg.rule)}
	for _, c := range combos {
		if acc, ok := g.groups[tupleKey(c)]; ok {
			out.count += acc.count
			out.agg.Merge(acc.agg)
		}
	}
	if out.value == -1 {
		return Num(float64(out.count)), true
	}
	return out.agg.Result(), true
}

func isIn(i int, list []int) bool {
	for _, x := range list {
		if x == i {
			return true
		}
	}
	return false
}

// Les index précalculés (totaux par valeur, valeurs triées de RANG, cumuls)
// sont gardés dans un registre borné : la requête de la grille ne transporte
// que leur numéro.
var precomputed = struct {
	sync.Mutex
	next  int64
	m     map[int64]any
	order []int64
}{m: map[int64]any{}}

const precomputedCap = 32

func register(x any) int64 {
	r := &precomputed
	r.Lock()
	defer r.Unlock()
	r.next++
	r.m[r.next] = x
	r.order = append(r.order, r.next)
	for len(r.order) > precomputedCap {
		delete(r.m, r.order[0])
		r.order = r.order[1:]
	}
	return r.next
}

func lookupPrecomputed(id int64) any {
	r := &precomputed
	r.Lock()
	defer r.Unlock()
	return r.m[id]
}

// grouped compile, quand c'est possible, la lecture des totaux par valeur
// d'une fonction de la famille .SI (expression NULL : parcours complet).
func (c *compiler) grouped(s scanCall, t *Table, sc scope) (string, bool) {
	if s.def.crit == nil || c.hoist == nil || sc.arr != nil {
		return "", false
	}
	for _, col := range s.cols {
		if rowDependent(col) {
			return "", false
		}
	}
	var rowDep []int
	for i, p := range s.params {
		if rowDependent(p) {
			rowDep = append(rowDep, i)
		}
	}
	if len(rowDep) == 0 || len(rowDep) > 3 {
		return "", false
	}
	// Le parcours, sans dépendance à la ligne : les critères qui en dépendent
	// sont remplacés par NULL et désignés dans spec.
	sub := c.sub()
	spec := make([]byte, len(s.params))
	head := []string{sub.param(c.schema.rule(false).key(s.name)), "", fmt.Sprint(len(s.params))}
	for i, p := range s.params {
		spec[i] = 'c'
		if isIn(i, rowDep) {
			spec[i] = 'r'
			head = append(head, "NULL")
			continue
		}
		head = append(head, sub.expr(p, scope{table: sc.table}))
	}
	head[1] = sub.param(string(spec))
	al := sub.newAlias()
	var cols []string
	for _, col := range s.cols {
		cols = append(cols, sub.expr(col, scope{table: sc.table}.inArray(t, al)))
	}
	q := fmt.Sprintf("(SELECT slt_group(%s, %s) FROM %s AS %s)", strings.Join(head, ", "), strings.Join(cols, ", "), t.SQL, al)
	id := c.hoist(Compiled{q, sub.args})
	if id.Kind != Number {
		return "", false
	}
	args := []string{c.param(id.Num)}
	for _, i := range rowDep {
		args = append(args, c.expr(s.params[i], sc))
	}
	return "slt_grouped(" + strings.Join(args, ", ") + ")", true
}

// groupAggFn construit le parcours (slt_group) : nom, spec, nparams,
// paramètres…, colonnes….
type groupAggFn struct{ g *grouped }

func (a *groupAggFn) Step(ctx sqlite3.Context, arg ...sqlite3.Value) {
	np := arg[2].Int()
	if a.g == nil {
		name, rule := splitKey(arg[0].Text())
		def, ok := scans[name]
		if !ok || def.crit == nil {
			return
		}
		spec := arg[1].Text()
		params := args(arg[3 : 3+np])
		tmpl, ok := newScanner(def, params, rule).(*cond)
		if !ok {
			return
		}
		a.g = &grouped{tmpl: tmpl, groups: map[string]*cond{}}
		for i := range spec {
			if spec[i] == 'r' {
				a.g.rowDep = append(a.g.rowDep, i)
			}
		}
	}
	a.g.add(args(arg[3+np:]))
}

func (a *groupAggFn) Value(ctx sqlite3.Context) {
	if a.g == nil {
		// Tableau vide : aucun groupe, chaque ligne lit le total vide.
		ctx.ResultNull()
		return
	}
	ctx.ResultInt64(register(a.g))
}

func registerGrouped(c *sqlite3.Conn) error {
	if err := registerRankCumul(c); err != nil {
		return err
	}
	if err := registerSeq(c); err != nil {
		return err
	}
	if err := c.CreateWindowFunction("slt_group", -1, sqlite3.DETERMINISTIC|sqlite3.INNOCUOUS, func() sqlite3.AggregateFunction { return &groupAggFn{} }); err != nil {
		return err
	}
	// slt_grouped(numéro, critères…) : le total de la ligne (enveloppé comme
	// celui de slt_scan), ou NULL pour reprendre le parcours complet. Pas DETERMINISTIC : le registre change.
	return c.CreateFunction("slt_grouped", -1, sqlite3.INNOCUOUS, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		g, _ := lookupPrecomputed(arg[0].Int64()).(*grouped)
		if g == nil {
			ctx.ResultNull()
			return
		}
		v, ok := g.lookup(args(arg[1:]))
		if !ok {
			ctx.ResultNull()
			return
		}
		ctx.ResultBlob(encodeValue(v)) // enveloppé comme slt_scan, pour slt_scanfin
	})
}

// --- RANG et CUMUL par ligne ---------------------------------------------------

// rankIndex : les nombres d'une colonne, triés, pour RANG([@Score]) : une
// recherche dichotomique par ligne au lieu d'un parcours.
type rankIndex struct{ vals []float64 }

func (x *rankIndex) rank(v, order Value) Value {
	r := newRank([]Value{v, order}).(*rank)
	if r.err.IsErr() {
		return r.err
	}
	lo, _ := slices.BinarySearch(x.vals, r.v.Num)
	hi := lo
	for hi < len(x.vals) && x.vals[hi] == r.v.Num {
		hi++
	}
	if lo == hi {
		return Err(ErrNA)
	}
	if r.asc {
		return Num(float64(lo + 1))
	}
	return Num(float64(len(x.vals) - hi + 1))
}

type rankAgg struct{ x *rankIndex }

func (a *rankAgg) Step(ctx sqlite3.Context, arg ...sqlite3.Value) {
	if a.x == nil {
		a.x = &rankIndex{}
	}
	if v := fromArg(arg[0]); v.Kind == Number || v.Kind == Date {
		a.x.vals = append(a.x.vals, v.Num)
	}
}

func (a *rankAgg) Value(ctx sqlite3.Context) {
	if a.x == nil {
		a.x = &rankIndex{}
	}
	slices.Sort(a.x.vals)
	ctx.ResultInt64(register(a.x))
}

// cumulIndex : CUMUL de chaque ligne (par identifiant), calculé en un seul
// parcours dans l'ordre du tableau.
type cumulIndex struct {
	sums map[int64]float64
	errs map[int64]ErrCode
}

type cumulAgg struct {
	x   *cumulIndex
	acc Agg
}

func (a *cumulAgg) Step(ctx sqlite3.Context, arg ...sqlite3.Value) {
	if a.x == nil {
		a.x = &cumulIndex{sums: map[int64]float64{}, errs: map[int64]ErrCode{}}
		a.acc = aggOf(arg[2].Text())
	}
	a.acc.AddRange(fromArg(arg[1]))
	id := arg[0].Int64()
	if a.acc.Err != 0 {
		a.x.errs[id] = a.acc.Err
	} else {
		a.x.sums[id] = a.acc.Sum + a.acc.Comp
	}
}

func (a *cumulAgg) Value(ctx sqlite3.Context) {
	if a.x == nil {
		ctx.ResultNull()
		return
	}
	ctx.ResultInt64(register(a.x))
}

func registerRankCumul(c *sqlite3.Conn) error {
	if err := c.CreateWindowFunction("slt_rankidx", 1, sqlite3.DETERMINISTIC|sqlite3.INNOCUOUS, func() sqlite3.AggregateFunction { return &rankAgg{} }); err != nil {
		return err
	}
	// slt_rankof(numéro, valeur, ordre) : RANG, rendu enveloppé comme slt_scan.
	if err := c.CreateFunction("slt_rankof", 3, sqlite3.INNOCUOUS, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		x, _ := lookupPrecomputed(arg[0].Int64()).(*rankIndex)
		if x == nil {
			ctx.ResultNull()
			return
		}
		ctx.ResultBlob(encodeValue(x.rank(fromArg(arg[1]), fromArg(arg[2]))))
	}); err != nil {
		return err
	}
	if err := c.CreateWindowFunction("slt_cumidx", 3, sqlite3.DETERMINISTIC|sqlite3.INNOCUOUS, func() sqlite3.AggregateFunction { return &cumulAgg{} }); err != nil {
		return err
	}
	// slt_cumof(numéro, identifiant de ligne) : CUMUL, ou NULL (calcul direct).
	return c.CreateFunction("slt_cumof", 2, sqlite3.INNOCUOUS, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		x, _ := lookupPrecomputed(arg[0].Int64()).(*cumulIndex)
		if x == nil {
			ctx.ResultNull()
			return
		}
		id := arg[1].Int64()
		if e, ok := x.errs[id]; ok {
			result(ctx, Err(e))
		} else if f, ok := x.sums[id]; ok {
			result(ctx, number(f))
		} else {
			ctx.ResultNull()
		}
	})
}

// rankPrecomputed compile RANG([@Score]) avec les valeurs triées calculées
// une fois.
func (c *compiler) rankPrecomputed(s scanCall, t *Table, sc scope) (string, bool) {
	if s.name != "RANK" || c.hoist == nil || sc.arr != nil || len(s.cols) != 1 || rowDependent(s.cols[0]) {
		return "", false
	}
	if !rowDependent(s.params[0]) || len(s.params) > 1 && rowDependent(s.params[1]) {
		return "", false
	}
	sub := c.sub()
	al := sub.newAlias()
	col := sub.expr(s.cols[0], scope{table: sc.table}.inArray(t, al))
	id := c.hoist(Compiled{fmt.Sprintf("(SELECT slt_rankidx(%s) FROM %s AS %s)", col, t.SQL, al), sub.args})
	if id.Kind != Number {
		return "", false
	}
	order := "NULL"
	if len(s.params) > 1 && s.params[1] != nil {
		order = c.expr(s.params[1], sc)
	}
	return fmt.Sprintf("slt_rankof(%s, %s, %s)", c.param(id.Num), c.expr(s.params[0], sc), order), true
}

// cumulPrecomputed compile CUMUL([@Montant]) avec les cumuls de toutes les
// lignes calculés en un parcours.
func (c *compiler) cumulPrecomputed(arg Node, sc scope) (string, bool) {
	if c.hoist == nil || sc.arr != nil {
		return "", false
	}
	sub := c.sub()
	al := sub.newAlias()
	val := sub.expr(arg, scope{table: sc.table, alias: al})
	key := sub.param(c.schema.rule(false).key("SUM"))
	id := c.hoist(Compiled{fmt.Sprintf("(SELECT slt_cumidx(%s.id, %s, %s ORDER BY %s._pos) FROM %s AS %s)", al, val, key, al, sc.table.SQL, al), sub.args})
	if id.Kind != Number {
		return "", false
	}
	return fmt.Sprintf("slt_cumof(%s, %s.id)", c.param(id.Num), sc.alias), true
}
