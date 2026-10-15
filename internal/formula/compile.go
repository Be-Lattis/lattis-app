package formula

import (
	"fmt"
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
	sql := c.expr(n, scope{table, alias})
	return Compiled{sql, c.args}
}

// CompileHoistedFrom est CompileHoisted avec des paramètres numérotés à
// partir de base+1.
func CompileHoistedFrom(n Node, s *Schema, names map[string]Value, table *Table, alias string, hoist func(Compiled) Value, base int) Compiled {
	c := &compiler{schema: s, names: names, visiting: map[*Column]bool{}, hoist: hoist, base: base}
	sql := c.expr(n, scope{table, alias})
	return Compiled{sql, c.args}
}

// Compile traduit une formule évaluée dans le contexte d'une ligne de table
// (alias alias), ou hors de tout tableau si table est nil.
func Compile(n Node, s *Schema, names map[string]Value, table *Table, alias string) Compiled {
	c := &compiler{schema: s, names: names, visiting: map[*Column]bool{}}
	sql := c.expr(n, scope{table, alias})
	return Compiled{sql, c.args}
}

type scope struct {
	table *Table
	alias string
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
		if col.Type == grid.Boolean {
			return "slt_bool(" + ref + ")"
		}
		return ref
	}
	if c.visiting[col] {
		return errSQL(ErrRef)
	}
	c.visiting[col] = true
	defer delete(c.visiting, col)
	return c.expr(col.Formula, scope{t, alias})
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
	switch fn.kind {
	case aggregate:
		if c.hoist != nil && !rowDependent(n) {
			sub := &compiler{schema: c.schema, names: c.names, visiting: map[*Column]bool{}}
			sql := sub.call(n, scope{})
			return c.param(ToSQL(c.hoist(Compiled{sql, sub.args})))
		}
		parts := []string{c.param(fn.EN)}
		for _, a := range n.Args {
			if r, ok := a.(Ref); ok && !r.Row {
				t, col, e := c.resolve(r, sc)
				if e != 0 {
					parts = append(parts, errSQL(e))
					continue
				}
				al := c.newAlias()
				parts = append(parts, fmt.Sprintf("(SELECT slt_part(%s, %s) FROM %s AS %s)",
					c.param(fn.EN), c.column(t, col, al), t.SQL, al))
				continue
			}
			parts = append(parts, c.expr(a, sc))
		}
		return "slt_final(" + strings.Join(parts, ", ") + ")"
	case special:
		if fn.EN == "REFERENCE" {
			return c.reference(n, sc)
		}
		return errSQL(ErrName)
	}
	parts := []string{c.param(fn.EN)}
	for _, a := range n.Args {
		parts = append(parts, c.expr(a, sc))
	}
	return "slt_fn(" + strings.Join(parts, ", ") + ")"
}

// reference compile REFERENCE(T; Col; clé) en deux sous-requêtes indexées sur
// la colonne clé (EXISTS pour distinguer « absent » de « vide »).
func (c *compiler) reference(n Call, sc scope) string {
	t, i, e := refTarget(c.schema, n)
	if e.IsErr() {
		return errSQL(e.Err)
	}
	k, _ := t.col(t.Key)
	keyCol := &t.Cols[k]
	key := c.expr(n.Args[2], sc)
	al := c.newAlias()
	match := al + "." + keyCol.SQL + " = slt_key(" + key + ", 'n')"
	if keyCol.Type == grid.Text || keyCol.Type == grid.Date {
		match = al + "." + keyCol.SQL + " = slt_key(" + key + ", 't') COLLATE NOCASE"
	}
	val := c.column(t, &t.Cols[i], al)
	return fmt.Sprintf("slt_ref(EXISTS (SELECT 1 FROM %s AS %s WHERE %s), (SELECT %s FROM %s AS %s WHERE %s LIMIT 1), %s)",
		t.SQL, al, match, val, t.SQL, al, match, key)
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
