package formula

// Fonctions de parcours : elles lisent les lignes d'un tableau, dans l'ordre
// d'affichage si besoin (EQUIV, INDEX). Leurs arguments sont de deux sortes :
//
//   - les paramètres, évalués une fois dans le contexte de la formule (le
//     critère de NB.SI, la valeur cherchée par EQUIV) ;
//   - les colonnes, évaluées sur chaque ligne du tableau parcouru : une
//     colonne (Ventes[Prix]) ou une expression sur les colonnes d'un même
//     tableau (Ventes[Prix]*Ventes[Qté], (Ventes[Prix]>10)*1).
//
// Le scanner (même code Go pour les deux backends) reçoit les paramètres,
// puis les valeurs des colonnes ligne par ligne. En SQL, il devient
// l'agrégat slt_scan, ce qui garde la sémantique en un seul endroit.

type role uint8

const (
	param role = iota
	column
)

type scanner interface {
	add(cols []Value)
	result() Value
}

type scanDef struct {
	// roles donne le rôle de chaque argument, ou false si le nombre
	// d'arguments ne convient pas (#VALEUR!).
	roles   func(n int) ([]role, bool)
	make    func(params []Value) scanner
	ordered bool // les lignes doivent arriver dans l'ordre d'affichage

	// crit donne, pour le critère n° i (parmi les paramètres), la colonne
	// qu'il teste ; nil hors des familles .SI (voir grouped.go).
	crit func(i int) int
}

func critSame(i int) int  { return i }
func critAfter(i int) int { return i + 1 }

func roles(rs ...role) []role { return rs }

// colParam : (colonne; paramètre), twoCols : deux colonnes du même tableau.
func colParam(n int) ([]role, bool) { return roles(column, param), n == 2 }
func twoCols(n int) ([]role, bool)  { return roles(column, column), n == 2 }

// optParam donne le paramètre facultatif n° i, vide s'il est absent.
func optParam(p []Value, i int) Value {
	if i < len(p) {
		return p[i]
	}
	return Value{}
}

// pairRoles : (plage; critère) répétés, après un préfixe.
func pairRoles(prefix ...role) func(int) ([]role, bool) {
	return func(n int) ([]role, bool) {
		if (n-len(prefix))%2 != 0 || n-len(prefix) < 2 {
			return nil, false
		}
		out := append([]role(nil), prefix...)
		for len(out) < n {
			out = append(out, column, param)
		}
		return out, true
	}
}

var scans = map[string]scanDef{
	"SUMIF":     {roles: ifRoles, make: ifScanner("SUM"), crit: critSame},
	"AVERAGEIF": {roles: ifRoles, make: ifScanner("AVERAGE"), crit: critSame},
	"COUNTIF": {roles: func(n int) ([]role, bool) { return roles(column, param), n == 2 },
		make: func(p []Value) scanner { return newCond("COUNT", p, []int{0}, -1) }, crit: critSame},
	"SUMIFS":     {roles: pairRoles(column), make: ifsScanner("SUM"), crit: critAfter},
	"AVERAGEIFS": {roles: pairRoles(column), make: ifsScanner("AVERAGE"), crit: critAfter},
	"MAXIFS":     {roles: pairRoles(column), make: ifsScanner("MAX"), crit: critAfter},
	"MINIFS":     {roles: pairRoles(column), make: ifsScanner("MIN"), crit: critAfter},
	"COUNTIFS": {roles: pairRoles(), make: func(p []Value) scanner {
		idx := make([]int, len(p))
		for i := range idx {
			idx[i] = i
		}
		return newCond("COUNT", p, idx, -1)
	}, crit: critSame},
	"SUMPRODUCT": {roles: func(n int) ([]role, bool) {
		out := make([]role, n)
		for i := range out {
			out[i] = column
		}
		return out, true
	}, make: func([]Value) scanner { return &sumProduct{} }},
	"MATCH": {roles: func(n int) ([]role, bool) { return roles(param, column, param)[:n], true },
		make: newMatch, ordered: true},
	"INDEX": {roles: func(n int) ([]role, bool) { return roles(column, param), n == 2 },
		make: func(p []Value) scanner { return &index{n: p[0]} }, ordered: true},
	"RANK": {roles: func(n int) ([]role, bool) { return roles(param, column, param)[:n], n >= 2 },
		make: newRank},
	"REFERENCE.APPROX": {roles: func(n int) ([]role, bool) { return roles(param, column, column), n == 3 },
		make: func(p []Value) scanner { return &approx{key: p[0]} }},
	"LARGE": {roles: func(n int) ([]role, bool) { return roles(column, param), n == 2 }, make: newKth(true)},
	"SMALL": {roles: func(n int) ([]role, bool) { return roles(column, param), n == 2 }, make: newKth(false)},

	// Statistiques (funcs_stats.go)
	"PERCENTILE":     {roles: colParam, make: newQuantile(false, false)},
	"PERCENTILE.INC": {roles: colParam, make: newQuantile(false, false)},
	"PERCENTILE.EXC": {roles: colParam, make: newQuantile(false, true)},
	"QUARTILE":       {roles: colParam, make: newQuantile(true, false)},
	"QUARTILE.INC":   {roles: colParam, make: newQuantile(true, false)},
	"QUARTILE.EXC":   {roles: colParam, make: newQuantile(true, true)},
	"CORREL":         {roles: twoCols, make: func([]Value) scanner { return &pairStats{fn: "CORREL"} }},
	"SLOPE":          {roles: twoCols, make: func([]Value) scanner { return &pairStats{fn: "SLOPE"} }},
	"INTERCEPT":      {roles: twoCols, make: func([]Value) scanner { return &pairStats{fn: "INTERCEPT"} }},
	"IRR": {roles: func(n int) ([]role, bool) { return roles(column, param)[:n], true },
		make: func(p []Value) scanner { return &irr{guess: optParam(p, 0)} }, ordered: true},

	"NETWORKDAYS": {roles: func(n int) ([]role, bool) { return roles(param, param, column)[:n], true },
		make: func(p []Value) scanner { return newWorkdays(p, false, false) }},
	"WORKDAY": {roles: func(n int) ([]role, bool) { return roles(param, param, column)[:n], true },
		make: func(p []Value) scanner { return newWorkdays(p, true, false) }},
	"NETWORKDAYS.INTL": {roles: func(n int) ([]role, bool) { return roles(param, param, param, column)[:n], true },
		make: func(p []Value) scanner { return newWorkdays(p, false, true) }},
	"WORKDAY.INTL": {roles: func(n int) ([]role, bool) { return roles(param, param, param, column)[:n], true },
		make: func(p []Value) scanner { return newWorkdays(p, true, true) }},
}

// --- Familles .SI et .SI.ENS --------------------------------------------------

// ifRoles : SOMME.SI(plage; critère; [plage_somme]).
func ifRoles(n int) ([]role, bool) { return roles(column, param, column)[:n], true }

func ifScanner(fn string) func([]Value) scanner {
	return func(p []Value) scanner { return newCond(fn, p, []int{0}, -2) }
}

// ifsScanner : SOMME.SI.ENS(plage_somme; plage1; critère1; …).
func ifsScanner(fn string) func([]Value) scanner {
	return func(p []Value) scanner {
		idx := make([]int, len(p))
		for i := range idx {
			idx[i] = i + 1
		}
		return newCond(fn, p, idx, 0)
	}
}

// cond agrège les valeurs des lignes qui satisfont tous les critères.
type cond struct {
	crits []criterion
	cols  []int // colonne testée par chaque critère
	value int   // colonne agrégée ; -1 : on compte les lignes ; -2 : la dernière colonne
	count int
	agg   Agg
}

func newCond(fn string, params []Value, cols []int, value int) *cond {
	c := &cond{cols: cols, value: value, agg: Agg{Fn: fn}}
	for _, p := range params {
		c.crits = append(c.crits, parseCriterion(p))
	}
	return c
}

func (c *cond) add(cols []Value) {
	for i, cr := range c.crits {
		if cr.err.IsErr() || !cr.match(cols[c.cols[i]]) {
			return
		}
	}
	switch c.value {
	case -1:
		c.count++
	case -2:
		c.agg.AddRange(cols[len(cols)-1])
	default:
		c.agg.AddRange(cols[c.value])
	}
}

func (c *cond) result() Value {
	for _, cr := range c.crits {
		if cr.err.IsErr() {
			return cr.err
		}
	}
	if c.value == -1 {
		return Num(float64(c.count))
	}
	return c.agg.Result()
}

// --- SOMMEPROD -------------------------------------------------------------

// sumProduct : somme des produits ligne à ligne. Comme dans Excel, une valeur
// non numérique (texte, booléen, vide) compte pour 0 ; une erreur est propagée.
type sumProduct struct {
	agg Agg
	err Value
}

func (s *sumProduct) add(cols []Value) {
	if s.err.IsErr() {
		return
	}
	p := 1.0
	for _, v := range cols {
		switch v.Kind {
		case Error:
			s.err = v
			return
		case Number, Date:
			p *= v.Num
		default:
			p = 0
		}
	}
	s.agg.addSum(p)
}

func (s *sumProduct) result() Value {
	if s.err.IsErr() {
		return s.err
	}
	return number(s.agg.Sum + s.agg.Comp)
}

// --- EQUIV et INDEX ---------------------------------------------------------

// match : EQUIV(valeur; colonne; [type]). Type 0 : première valeur égale
// (jokers permis dans un texte) ; 1 (défaut) : la plus grande valeur
// inférieure ou égale ; -1 : la plus petite valeur supérieure ou égale.
// Position dans l'ordre d'affichage, à partir de 1.
type match struct {
	key       Value
	mode      int
	err       Value
	pos, best int
	bestV     Value
}

func newMatch(p []Value) scanner {
	m := &match{key: p[0], mode: 1}
	if m.key.IsErr() {
		m.err = m.key
	}
	if len(p) > 1 && !p[1].IsEmpty() {
		f, e := ToNumber(p[1])
		switch {
		case e.IsErr():
			m.err = e
		case f > 0:
			m.mode = 1
		case f < 0:
			m.mode = -1
		default:
			m.mode = 0
		}
	}
	return m
}

func sameClass(a, b Value) bool {
	return a.Kind != Empty && b.Kind != Empty && !a.IsErr() && !b.IsErr() && order(a.Kind) == order(b.Kind)
}

func (m *match) add(cols []Value) {
	m.pos++
	if m.err.IsErr() || m.mode == 0 && m.best > 0 {
		return
	}
	v := cols[0]
	if m.mode == 0 {
		if m.key.Kind == Text && v.Kind == Text && Wildcard(m.key.Str, v.Str) || sameClass(v, m.key) && cmp(v, m.key) == 0 {
			m.best = m.pos
		}
		return
	}
	if !sameClass(v, m.key) {
		return
	}
	c := cmp(v, m.key)
	if m.mode == 1 && c <= 0 && (m.best == 0 || cmp(v, m.bestV) >= 0) ||
		m.mode == -1 && c >= 0 && (m.best == 0 || cmp(v, m.bestV) <= 0) {
		m.best, m.bestV = m.pos, v
	}
}

func (m *match) result() Value {
	if m.err.IsErr() {
		return m.err
	}
	if m.best == 0 {
		return Err(ErrNA)
	}
	return Num(float64(m.best))
}

// index : INDEX(colonne; n), la n-ième valeur dans l'ordre d'affichage.
type index struct {
	n     Value
	pos   int
	found Value
	hit   bool
}

func (x *index) add(cols []Value) {
	x.pos++
	if f, e := ToNumber(x.n); !e.IsErr() && int(f) == x.pos {
		x.found, x.hit = cols[0], true
	}
}

func (x *index) result() Value {
	if x.n.IsErr() {
		return x.n
	}
	if _, e := ToNumber(x.n); e.IsErr() {
		return e
	}
	if !x.hit {
		return Err(ErrRef)
	}
	return x.found
}

// --- RANG -------------------------------------------------------------------

// rank : RANG(valeur; colonne; [ordre]) : 1 + le nombre de valeurs plus
// grandes (ordre 0, défaut) ou plus petites (ordre non nul). #N/A si la
// valeur n'est pas dans la colonne.
type rank struct {
	v     Value
	asc   bool
	err   Value
	n     int
	found bool
}

func newRank(p []Value) scanner {
	r := &rank{v: p[0]}
	switch {
	case p[0].IsErr():
		r.err = p[0]
	case p[0].Kind != Number && p[0].Kind != Date:
		r.err = Err(ErrValue)
	}
	if len(p) > 1 {
		b, e := Truth(p[1])
		if e.IsErr() {
			r.err = e
		}
		r.asc = b
	}
	return r
}

func (r *rank) add(cols []Value) {
	v := cols[0]
	if v.Kind != Number && v.Kind != Date {
		return
	}
	switch {
	case v.Num == r.v.Num:
		r.found = true
	case !r.asc && v.Num > r.v.Num, r.asc && v.Num < r.v.Num:
		r.n++
	}
}

func (r *rank) result() Value {
	if r.err.IsErr() {
		return r.err
	}
	if !r.found {
		return Err(ErrNA)
	}
	return Num(float64(r.n + 1))
}

// --- REFERENCE.APPROCHEE ------------------------------------------------------

// approx : la ligne dont la clé est la plus grande valeur inférieure ou égale
// à la valeur cherchée (tranches, barèmes). #N/A si toutes les clés sont plus
// grandes.
type approx struct {
	key   Value
	bestK Value
	bestV Value
	hit   bool
}

func (a *approx) add(cols []Value) {
	k := cols[0]
	if !sameClass(k, a.key) || cmp(k, a.key) > 0 {
		return
	}
	if !a.hit || cmp(k, a.bestK) > 0 {
		a.bestK, a.bestV, a.hit = k, cols[1], true
	}
}

func (a *approx) result() Value {
	if a.key.IsErr() {
		return a.key
	}
	if !a.hit {
		return Err(ErrNA)
	}
	return a.bestV
}

// --- Jours ouvrés -------------------------------------------------------------

type workdayScan struct {
	from, n Value
	forward bool // SERIE.JOUR.OUVRE (sinon NB.JOURS.OUVRES)
	days    workdays
	err     Value
}

// newWorkdays : NB.JOURS.OUVRES et SERIE.JOUR.OUVRE ; intl : leurs formes
// .INTL, dont le 3e paramètre donne les jours de week-end.
func newWorkdays(p []Value, forward, intl bool) scanner {
	w := &workdayScan{from: p[0], n: p[1], forward: forward, days: newWorkdaySet()}
	if intl {
		w.err = w.days.weekend(optParam(p, 2))
	}
	return w
}

func (w *workdayScan) add(cols []Value) {
	if e := w.days.add(cols[0]); e.IsErr() && !w.err.IsErr() {
		w.err = e
	}
}

func (w *workdayScan) result() Value {
	if w.err.IsErr() {
		return w.err
	}
	if w.forward {
		return w.days.workday(w.from, w.n)
	}
	return w.days.networkdays(w.from, w.n)
}
