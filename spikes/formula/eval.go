package formula

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/lalamefine/slitable/internal/grid"
)

// Column est une colonne de tableau : stockée (Formula nil) ou calculée.
type Column struct {
	Name    string
	SQL     string
	Type    grid.Type
	Formula Node
}

// Table décrit un tableau pour les formules. Key est la colonne clé
// utilisée par REFERENCE (vide : pas encore de clé).
type Table struct {
	Name string
	SQL  string
	Cols []Column
	Key  string
}

func (t *Table) col(name string) (int, bool) {
	for i, c := range t.Cols {
		if strings.EqualFold(c.Name, name) {
			return i, true
		}
	}
	return -1, false
}

// Schema regroupe les tableaux d'un document.
type Schema struct {
	Tables []*Table
}

func (s *Schema) Table(name string) (*Table, bool) {
	for _, t := range s.Tables {
		if strings.EqualFold(t.Name, name) {
			return t, true
		}
	}
	return nil, false
}

// --- Évaluateur Go ------------------------------------------------------------

// Data est un instantané des valeurs stockées des tableaux, dans l'ordre
// d'affichage : Data[table][ligne][colonne] (colonnes calculées à vide).
type Data map[*Table][][]Value

// LoadData lit les colonnes stockées des tableaux depuis SQLite.
func LoadData(ctx context.Context, db *sql.DB, s *Schema) (Data, error) {
	d := Data{}
	for _, t := range s.Tables {
		var sel []string
		var idx []int
		for i, c := range t.Cols {
			if c.Formula == nil {
				sel = append(sel, c.SQL)
				idx = append(idx, i)
			}
		}
		rs, err := db.QueryContext(ctx, `SELECT `+strings.Join(sel, ", ")+` FROM `+t.SQL+` ORDER BY _pos`)
		if err != nil {
			return nil, err
		}
		raw := make([]any, len(sel))
		ptrs := make([]any, len(sel))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		for rs.Next() {
			if err := rs.Scan(ptrs...); err != nil {
				rs.Close()
				return nil, err
			}
			row := make([]Value, len(t.Cols))
			for k, i := range idx {
				row[i] = FromSQL(raw[k], t.Cols[i].Type)
			}
			d[t] = append(d[t], row)
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// FromSQL convertit une valeur stockée selon le type logique de sa colonne.
func FromSQL(v any, t grid.Type) Value {
	switch v := v.(type) {
	case nil:
		return Value{}
	case int64:
		if t == grid.Boolean {
			return Boolean(v != 0)
		}
		return Num(float64(v))
	case float64:
		return Num(v)
	case string:
		return Str(v)
	case []byte:
		if val, ok := decodeBlob(v); ok {
			return val
		}
		return Str(string(v))
	}
	return Err(ErrValue)
}

// Env est le contexte d'évaluation Go.
type Env struct {
	Schema *Schema
	Data   Data
	Names  map[string]Value // valeurs des cellules nommées (clé en minuscules)
	Table  *Table           // tableau courant (colonne calculée), ou nil
	Row    int              // ligne courante dans Table

	visiting map[[2]any]bool
}

// Eval évalue une formule.
func Eval(n Node, env *Env) Value {
	switch n := n.(type) {
	case nil:
		return Value{}
	case NumLit:
		return Num(n.V)
	case StrLit:
		return Str(n.V)
	case BoolLit:
		return Boolean(n.V)
	case ErrLit:
		return Err(n.V)
	case Name:
		if v, ok := env.Names[strings.ToLower(n.Name)]; ok {
			return v
		}
		return Err(ErrName)
	case Range:
		return Err(ErrRef) // les plages A1 n'existent que pour la conversion
	case Ref:
		t, i, err := env.resolve(n)
		if err.IsErr() {
			return err
		}
		if !n.Row {
			return Err(ErrValue) // une colonne entière hors d'un agrégat
		}
		return env.cell(t, env.Row, i)
	case Unary:
		x := Eval(n.X, env)
		switch n.Op {
		case "-":
			return Neg(x)
		case "%":
			return Percent(x)
		}
		_, e := ToNumber(x)
		if e.IsErr() {
			return e
		}
		return x
	case Binary:
		l, r := Eval(n.L, env), Eval(n.R, env)
		switch n.Op {
		case "&":
			return Concat(l, r)
		case "+", "-", "*", "/", "^":
			return Arith(n.Op[0], l, r)
		}
		return Compare(n.Op, l, r)
	case Call:
		return env.call(n)
	}
	return Err(ErrValue)
}

// resolve trouve le tableau et la colonne d'une référence.
func (env *Env) resolve(r Ref) (*Table, int, Value) {
	t := env.Table
	if r.Table != "" {
		var ok bool
		if t, ok = env.Schema.Table(r.Table); !ok {
			return nil, 0, Err(ErrRef)
		}
	}
	if t == nil || r.Row && t != env.Table {
		return nil, 0, Err(ErrRef)
	}
	i, ok := t.col(r.Col)
	if !ok {
		return nil, 0, Err(ErrRef)
	}
	return t, i, Value{}
}

// cell donne la valeur d'une cellule, en calculant une colonne calculée.
func (env *Env) cell(t *Table, row, i int) Value {
	rows := env.Data[t]
	if row < 0 || row >= len(rows) {
		return Err(ErrRef)
	}
	c := t.Cols[i]
	if c.Formula == nil {
		return rows[row][i]
	}
	key := [2]any{t, i}
	if env.visiting[key] {
		return Err(ErrRef) // référence circulaire entre colonnes calculées
	}
	if env.visiting == nil {
		env.visiting = map[[2]any]bool{}
	}
	env.visiting[key] = true
	defer delete(env.visiting, key)
	sub := *env
	sub.Table, sub.Row = t, row
	return Eval(c.Formula, &sub)
}

func (env *Env) call(n Call) Value {
	fn, ok := lookupFunc(n.Fn)
	if !ok {
		return Err(ErrName)
	}
	switch fn.kind {
	case aggregate:
		acc := Agg{Fn: fn.EN}
		for _, a := range n.Args {
			if r, ok := a.(Ref); ok && !r.Row {
				t, i, err := env.resolve(r)
				if err.IsErr() {
					acc.AddValue(err)
					continue
				}
				for row := range env.Data[t] {
					acc.AddRange(env.cell(t, row, i))
				}
				continue
			}
			acc.AddValue(Eval(a, env))
		}
		return acc.Result()
	case special:
		if fn.EN == "REFERENCE" {
			return env.reference(n)
		}
		return Err(ErrName) // RECHERCHEV doit être convertie (voir Convert)
	}
	args := make([]Value, len(n.Args))
	for i, a := range n.Args {
		args[i] = Eval(a, env)
	}
	return fn.impl(args)
}

// reference : REFERENCE(Tableau; Colonne; clé) cherche la ligne dont la
// colonne clé du tableau vaut clé, et renvoie sa valeur dans Colonne.
func (env *Env) reference(n Call) Value {
	t, col, err := refTarget(env.Schema, n)
	if err.IsErr() {
		return err
	}
	key := Eval(n.Args[2], env)
	if key.IsErr() {
		return key
	}
	k, _ := t.col(t.Key)
	for row := range env.Data[t] {
		if Equal(env.cell(t, row, k), key) {
			return env.cell(t, row, col)
		}
	}
	return Err(ErrNA)
}

// refTarget lit les deux premiers arguments de REFERENCE.
func refTarget(s *Schema, n Call) (*Table, int, Value) {
	tn, ok1 := nameOf(n.Args[0])
	cn, ok2 := nameOf(n.Args[1])
	if !ok1 || !ok2 {
		return nil, 0, Err(ErrValue)
	}
	t, ok := s.Table(tn)
	if !ok || t.Key == "" {
		return nil, 0, Err(ErrRef)
	}
	if _, ok := t.col(t.Key); !ok {
		return nil, 0, Err(ErrRef)
	}
	col, ok := t.col(cn)
	if !ok {
		return nil, 0, Err(ErrRef)
	}
	return t, col, Value{}
}

func nameOf(n Node) (string, bool) {
	switch n := n.(type) {
	case Name:
		return n.Name, true
	case StrLit:
		return n.V, true
	}
	return "", false
}

// String sert au débogage.
func (t *Table) String() string { return fmt.Sprintf("Table(%s)", t.Name) }
