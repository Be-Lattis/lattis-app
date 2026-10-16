package formula

import (
	"fmt"
	"math"
	"strings"

	"github.com/lalamefine/slitable/internal/grid"
)

// Compiled est une expression SQL et ses paramètres.
type Compiled struct {
	SQL  string
	Args []any
}

// compiler traduit un AST en expression SQL. Chaque tableau parcouru reçoit
// un alias : la ligne courante est l'alias du tableau courant, et chaque
// sous-requête (agrégat, REFERENCE) ouvre son propre alias, ce qui permet
// une recherche dans le tableau courant lui-même.
type compiler struct {
	schema   *Schema
	names    map[string]Value
	args     []any
	aliases  int
	rands    int
	visiting map[*Column]bool

	// hoist, s'il est défini, calcule une fois un agrégat qui ne dépend pas de
	// la ligne (SOMME(T[Col]) dans une colonne calculée) ; sa valeur est
	// ensuite passée en paramètre au lieu d'être recalculée à chaque requête.
	hoist func(Compiled) Value

	// base décale la numérotation des paramètres (?base+1…), pour assembler
	// plusieurs expressions compilées dans une même requête.
	base int
}

// CompileHoisted est Compile avec des agrégats calculés à part (voir hoist).
func CompileHoisted(n Node, s *Schema, names map[string]Value, table *Table, alias string, hoist func(Compiled) Value) Compiled {
	c := &compiler{schema: s, names: names, visiting: map[*Column]bool{}, hoist: hoist}
	sql := c.expr(n, scope{table: table, alias: alias})
	return Compiled{sql, c.args}
}

// CompileHoistedFrom est CompileHoisted avec des paramètres numérotés à
// partir de base+1.
func CompileHoistedFrom(n Node, s *Schema, names map[string]Value, table *Table, alias string, hoist func(Compiled) Value, base int) Compiled {
	c := &compiler{schema: s, names: names, visiting: map[*Column]bool{}, hoist: hoist, base: base}
	sql := c.expr(n, scope{table: table, alias: alias})
	return Compiled{sql, c.args}
}

// Compile traduit une formule évaluée dans le contexte d'une ligne de table
// (alias alias), ou hors de tout tableau si table est nil.
func Compile(n Node, s *Schema, names map[string]Value, table *Table, alias string) Compiled {
	c := &compiler{schema: s, names: names, visiting: map[*Column]bool{}}
	sql := c.expr(n, scope{table: table, alias: alias})
	return Compiled{sql, c.args}
}

type scope struct {
	table *Table
	alias string

	// arr et arrAlias : tableau parcouru par une expression sur colonnes
	// (SOMMEPROD, agrégats, familles .SI) ; Tableau[Col] y lit la ligne arrAlias.
	arr      *Table
	arrAlias string
}

// inArray donne la portée d'une expression évaluée sur chaque ligne de t.
func (sc scope) inArray(t *Table, alias string) scope {
	return scope{table: sc.table, alias: sc.alias, arr: t, arrAlias: alias}
}

func errSQL(e ErrCode) string { return fmt.Sprintf("slt_err(%d)", e) }

// param ajoute un paramètre numéroté (?N) : une expression peut être recopiée
// plusieurs fois dans la requête sans décaler les paramètres.
func (c *compiler) param(v any) string {
	c.args = append(c.args, v)
	return fmt.Sprintf("?%d", c.base+len(c.args))
}

func (c *compiler) expr(n Node, sc scope) string {
	switch n := n.(type) {
	case nil:
		return "NULL"
	case NumLit:
		return c.param(n.V)
	case StrLit:
		return c.param(n.V)
	case BoolLit:
		return fmt.Sprintf("slt_bool(%d)", boolInt(n.V))
	case ErrLit:
		return errSQL(n.V)
	case Name:
		v, ok := c.names[strings.ToLower(n.Name)]
		if !ok {
			return errSQL(ErrName)
		}
		return c.param(ToSQL(v))
	case Range:
		return errSQL(ErrRef)
	case Ref:
		t, col, e := c.resolve(n, sc)
		if e != 0 {
			return errSQL(e)
		}
		if !n.Row {
			if t == sc.arr {
				return c.column(t, col, sc.arrAlias)
			}
			return errSQL(ErrValue)
		}
		return c.column(t, col, sc.alias)
	case Unary:
		x := c.expr(n.X, sc)
		switch n.Op {
		case "-":
			return "slt_neg(" + x + ")"
		case "%":
			return "slt_pct(" + x + ")"
		}
		return "slt_plus(" + x + ")"
	case Binary:
		l, r := c.expr(n.L, sc), c.expr(n.R, sc)
		fn := map[string]string{
			"+": "slt_add", "-": "slt_sub", "*": "slt_mul", "/": "slt_div", "^": "slt_pow", "&": "slt_cat",
			"=": "slt_eq", "<>": "slt_ne", "<": "slt_lt", "<=": "slt_le", ">": "slt_gt", ">=": "slt_ge",
		}[n.Op]
		return fn + "(" + l + ", " + r + ")"
	case Call:
		return c.call(n, sc)
	}
	return errSQL(ErrValue)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (c *compiler) resolve(r Ref, sc scope) (*Table, *Column, ErrCode) {
	t := sc.table
	if r.Table != "" {
		var ok bool
		if t, ok = c.schema.Table(r.Table); !ok {
			return nil, nil, ErrRef
		}
	}
	if t == nil || r.Row && t != sc.table {
		return nil, nil, ErrRef
	}
	i, ok := t.col(r.Col)
	if !ok {
		return nil, nil, ErrRef
	}
	return t, &t.Cols[i], 0
}

// column donne la valeur d'une colonne sur la ligne alias : la colonne
// stockée (convertie selon son type), ou la formule d'une colonne calculée
// recopiée en ligne.
func (c *compiler) column(t *Table, col *Column, alias string) string {
	if col.Formula == nil {
		ref := alias + "." + col.SQL
		switch col.Type {
		case grid.Boolean:
			return "slt_bool(" + ref + ")"
		case grid.Date:
			return "slt_date(" + ref + ")"
		}
		return ref
	}
	if c.visiting[col] {
		return errSQL(ErrRef)
	}
	c.visiting[col] = true
	defer delete(c.visiting, col)
	return c.expr(col.Formula, scope{table: t, alias: alias})
}

func (c *compiler) newAlias() string {
	c.aliases++
	return fmt.Sprintf("s%d", c.aliases)
}

func (c *compiler) call(n Call, sc scope) string {
	fn, ok := lookupFunc(n.Fn)
	if !ok {
		return errSQL(ErrName)
	}
	if c.hoist != nil && sc.arr == nil && hoistable(fn) && !rowDependent(n) {
		sub := &compiler{schema: c.schema, names: c.names, visiting: map[*Column]bool{}}
		sql := sub.call(n, scope{table: sc.table})
		return c.param(ToSQL(c.hoist(Compiled{sql, sub.args})))
	}
	switch fn.kind {
	case aggregate:
		return c.aggregate(fn.EN, n.Args, sc, false)
	case scan:
		return c.scan(scanOf(n, c.schema, sc.table), sc)
	case special:
		return c.special(fn.EN, n, sc)
	case spill:
		return errSQL(ErrValue)
	}
	parts := []string{c.param(fn.EN)}
	for _, a := range n.Args {
		parts = append(parts, c.expr(a, sc))
	}
	return "slt_fn(" + strings.Join(parts, ", ") + ")"
}

// hoistable : fonctions qui lisent des tableaux entiers, à calculer une fois
// quand elles ne dépendent pas de la ligne.
func hoistable(fn funcDef) bool {
	switch fn.kind {
	case aggregate, scan:
		return true
	}
	return fn.EN == "SUBTOTAL" || fn.EN == "EXISTS" || fn.EN == "REFERENCE.APPROX"
}

// aggregate compile un agrégat : chaque colonne ou expression sur colonnes
// devient une sous-requête qui rend un état partiel (slt_part), combiné par
// slt_final avec les valeurs directes. visible (SOUS.TOTAL) : seulement les
// lignes qu'un filtre laisse affichées.
func (c *compiler) aggregate(fn string, args []Node, sc scope, visible bool) string {
	parts := []string{c.param(fn)}
	lead := aggLead[fn]
	order := ""
	for k, a := range args {
		if k < lead {
			parts = append(parts, c.expr(a, sc))
			continue
		}
		if IsSpill(a) {
			parts = append(parts, c.spillAggregate(fn, a, sc))
			continue
		}
		var t *Table
		var val string
		al := c.newAlias()
		if fn == "TEXTJOIN" {
			order = " ORDER BY " + al + "._pos"
		}
		if r, ok := a.(Ref); ok && !r.Row {
			tt, col, e := c.resolve(r, sc)
			if e != 0 {
				parts = append(parts, errSQL(e))
				continue
			}
			t, val = tt, c.column(tt, col, al)
		} else {
			tt, e := arrayTable([]Node{a}, c.schema, sc.table)
			if e != 0 {
				parts = append(parts, errSQL(e))
				continue
			}
			if tt == nil {
				parts = append(parts, c.expr(a, sc))
				continue
			}
			t, val = tt, c.expr(a, sc.inArray(tt, al))
		}
		src := t.SQL
		if visible && t.Visible != "" {
			src = t.Visible
		}
		parts = append(parts, fmt.Sprintf("(SELECT slt_part(%s, %s%s) FROM %s AS %s)", c.param(fn), val, order, src, al))
	}
	return "slt_final(" + strings.Join(parts, ", ") + ")"
}

// scan compile une fonction de parcours : l'agrégat slt_scan reçoit les
// paramètres puis les colonnes de chaque ligne ; slt_scanfin donne le
// résultat, y compris sur un tableau vide.
func (c *compiler) scan(s scanCall, sc scope) string {
	if s.err != 0 {
		return errSQL(s.err)
	}
	t, e := arrayTable(s.cols, c.schema, sc.table)
	if e != 0 {
		return errSQL(e)
	}
	head := []string{c.param(s.name), fmt.Sprint(len(s.params))}
	for _, p := range s.params {
		head = append(head, c.expr(p, sc))
	}
	h := strings.Join(head, ", ")
	if len(s.cols) == 0 {
		return "slt_scanfin(" + h + ", NULL)"
	}
	var cols []string
	if t == nil {
		for _, col := range s.cols {
			cols = append(cols, c.expr(col, sc))
		}
		return fmt.Sprintf("slt_scanfin(%s, (SELECT slt_scan(%s, %s)))", h, h, strings.Join(cols, ", "))
	}
	al := c.newAlias()
	for _, col := range s.cols {
		cols = append(cols, c.expr(col, sc.inArray(t, al)))
	}
	order := ""
	if s.def.ordered {
		order = " ORDER BY " + al + "._pos"
	}
	full := fmt.Sprintf("(SELECT slt_scan(%s, %s%s) FROM %s AS %s)", h, strings.Join(cols, ", "), order, t.SQL, al)
	if r, ok := c.rankPrecomputed(s, t, sc); ok {
		full = fmt.Sprintf("coalesce(%s, %s)", r, full)
	} else if g, ok := c.grouped(s, t, sc); ok {
		// Famille .SI dont un critère dépend de la ligne (NB.SI(Ventes[Client];
		// [@Client])) : les totaux par valeur sont calculés une fois ; chaque
		// ligne les lit, et ne parcourt le tableau que si son critère n'est pas
		// une égalité simple (">5", "Dur*"…).
		full = fmt.Sprintf("coalesce(%s, %s)", g, full)
	}
	return fmt.Sprintf("slt_scanfin(%s, %s)", h, full)
}

// special compile les fonctions propres à chaque backend.
func (c *compiler) special(fn string, n Call, sc scope) string {
	switch fn {
	case "REFERENCE":
		return c.reference(n, sc)
	case "REFERENCE.APPROX":
		return c.scan(scanOf(n, c.schema, sc.table), sc)
	case "EXISTS":
		t, k, e := existsTarget(c.schema, n)
		if e.IsErr() {
			return errSQL(e.Err)
		}
		key := c.expr(n.Args[1], sc)
		al := c.newAlias()
		return fmt.Sprintf("slt_exists(EXISTS (SELECT 1 FROM %s AS %s WHERE %s), %s)", t.SQL, al, keyMatch(&t.Cols[k], al, key), key)
	case "SUBTOTAL":
		agg, e := subtotalOf(n)
		if e.IsErr() {
			return errSQL(e.Err)
		}
		return c.aggregate(agg, n.Args[1:], sc, true)
	case "PREVIOUS", "NEXT":
		if sc.table == nil || sc.alias == "" {
			return errSQL(ErrRef)
		}
		al := c.newAlias()
		where := fmt.Sprintf("%s._pos < %s._pos", al, sc.alias)
		dir := "DESC"
		if fn == "NEXT" {
			where = fmt.Sprintf("%s._pos > %s._pos", al, sc.alias)
			dir = "ASC"
		}
		val := c.expr(n.Args[0], scope{table: sc.table, alias: al})
		def := "NULL"
		if len(n.Args) > 1 {
			def = c.expr(n.Args[1], sc)
		}
		t := sc.table.SQL
		return fmt.Sprintf("slt_pick(EXISTS (SELECT 1 FROM %s AS %s WHERE %s), (SELECT %s FROM %s AS %s WHERE %s ORDER BY %s._pos %s LIMIT 1), %s)",
			t, al, where, val, t, al, where, al, dir, def)
	case "RUNNINGSUM":
		if sc.table == nil || sc.alias == "" {
			return errSQL(ErrRef)
		}
		al := c.newAlias()
		val := c.expr(n.Args[0], scope{table: sc.table, alias: al})
		direct := fmt.Sprintf("slt_final(%s, (SELECT slt_part(%s, %s) FROM %s AS %s WHERE %s._pos <= %s._pos))",
			c.param("SUM"), c.param("SUM"), val, sc.table.SQL, al, al, sc.alias)
		if pre, ok := c.cumulPrecomputed(n.Args[0], sc); ok {
			// Cumuls de toutes les lignes calculés en un parcours (sinon une
			// somme de toutes les lignes précédentes, à chaque ligne).
			return fmt.Sprintf("coalesce(%s, %s)", pre, direct)
		}
		return direct
	case "TODAY":
		return c.param(ToSQL(DateSerial(math.Floor(c.schema.nowSerial()))))
	case "NOW":
		return c.param(ToSQL(DateSerial(c.schema.nowSerial())))
	case "RAND", "RANDBETWEEN":
		row := "0"
		if sc.table != nil && sc.alias != "" {
			row = sc.alias + ".id"
		}
		r := fmt.Sprintf("slt_rand(%s, %s, %d)", c.param(int64(c.schema.seed())), row, c.rands)
		c.rands++
		if fn == "RAND" {
			return r
		}
		return fmt.Sprintf("slt_fn(%s, %s, %s, %s)", c.param(fn), c.expr(n.Args[0], sc), c.expr(n.Args[1], sc), r)
	}
	return errSQL(ErrName)
}

// reference compile REFERENCE(T; Col; clé) en deux sous-requêtes indexées sur
// la colonne clé (EXISTS pour distinguer « absent » de « vide »).
func (c *compiler) reference(n Call, sc scope) string {
	t, i, e := refTarget(c.schema, n)
	if e.IsErr() {
		return errSQL(e.Err)
	}
	k, _ := t.col(t.Key)
	key := c.expr(n.Args[2], sc)
	al := c.newAlias()
	match := keyMatch(&t.Cols[k], al, key)
	val := c.column(t, &t.Cols[i], al)
	return fmt.Sprintf("slt_ref(EXISTS (SELECT 1 FROM %s AS %s WHERE %s), (SELECT %s FROM %s AS %s WHERE %s LIMIT 1), %s)",
		t.SQL, al, match, val, t.SQL, al, match, key)
}

// keyMatch : condition « la colonne clé vaut key » sur la ligne alias, qui
// utilise l'index de la clé.
func keyMatch(keyCol *Column, alias, key string) string {
	if keyCol.Type == grid.Text || keyCol.Type == grid.Date {
		return alias + "." + keyCol.SQL + " = slt_key(" + key + ", 't') COLLATE NOCASE"
	}
	return alias + "." + keyCol.SQL + " = slt_key(" + key + ", 'n')"
}

// rowDependent indique si une expression lit la ligne courante.
func rowDependent(n Node) bool {
	switch n := n.(type) {
	case Ref:
		return n.Row
	case Unary:
		return rowDependent(n.X)
	case Binary:
		return rowDependent(n.L) || rowDependent(n.R)
	case Call:
		for _, a := range n.Args {
			if rowDependent(a) {
				return true
			}
		}
	}
	return false
}
