package formula

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

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
	// Visible : source SQL des lignes affichées quand un filtre en masque
	// (sous-requête), vide sinon. SOUS.TOTAL ne compte que celles-là.
	Visible string
	Cols    []Column
	Key     string
	// Sheet : feuille du tableau, où ses colonnes calculées lisent les
	// adresses Cell_A1 (0 : inconnue).
	Sheet int64
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

	// Now fixe AUJOURDHUI et MAINTENANT (heure courante si zéro) et Seed les
	// tirages d'ALEA : le document les renouvelle à l'ouverture et sur F9, pas
	// à chaque lecture de la grille.
	Now  time.Time
	Seed uint64
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
		if t == grid.Date {
			if f, ok := ParseDate(v); ok {
				return DateSerial(f)
			}
		}
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

	// arr et arrRow : ligne parcourue par une expression sur colonnes
	// (Ventes[Prix]*Ventes[Qté] dans SOMMEPROD) ; Ventes[Prix] y désigne la
	// valeur de cette ligne.
	arr    *Table
	arrRow int

	// ranges : plages Cell[A1:B3] parcourues, leurs valeurs et la position
	// courante.
	ranges    []CellRange
	rangeVals [][]Value
	rangePos  int

	visiting map[[2]any]bool
	rands    *int // rang du prochain appel à ALEA
}

// inArray donne le contexte d'une ligne d'un tableau parcouru.
func (env *Env) inArray(t *Table, row int) *Env {
	sub := *env
	sub.arr, sub.arrRow = t, row
	return &sub
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
	case CellRange:
		for i, r := range env.ranges {
			if r == n {
				return env.rangeVals[i][env.rangePos]
			}
		}
		return Err(ErrValue) // une plage hors d'un agrégat
	case RowRange:
		if env.arr != nil {
			if t, i, err := env.resolve(n.Ref); !err.IsErr() && t == env.arr {
				return env.cell(t, env.arrRow, i)
			}
		}
		return env.rowValue(n)
	case Ref:
		t, i, err := env.resolve(n)
		if err.IsErr() {
			return err
		}
		if !n.Row {
			if t == env.arr {
				return env.cell(t, env.arrRow, i)
			}
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
	sub.arr = nil
	return Eval(c.Formula, &sub)
}

func (env *Env) call(n Call) Value {
	if _, ok := relativeOffset(n); ok {
		// [Col][-1] ne se calcule qu'en SQL, dans une colonne calculée.
		return Err(ErrRef)
	}
	if n.Fn == "ITEM" {
		it, ok := itemCall(n, env.Schema, env.Table)
		if !ok {
			return Err(ErrRef)
		}
		n = it
	}
	fn, ok := lookupFunc(n.Fn)
	if !ok {
		return Err(ErrName)
	}
	switch fn.kind {
	case aggregate:
		return env.aggregate(fn.EN, n.Args)
	case scan:
		return env.scan(scanOf(n, env.Schema, env.Table))
	case special:
		return env.special(fn.EN, n)
	case spill:
		return Err(ErrValue) // un tableau dynamique ne tient pas dans une cellule
	}
	args := make([]Value, len(n.Args))
	for i, a := range n.Args {
		args[i] = Eval(a, env)
	}
	return fn.impl(args)
}

// aggregate calcule un agrégat : une colonne entière ne compte que ses
// nombres, une expression sur colonnes est évaluée ligne à ligne (même
// règle), une autre valeur est convertie.
func (env *Env) aggregate(fn string, args []Node) Value {
	acc := Agg{Fn: fn}
	lead := aggLead[fn]
	var leadV []Value
	for k, a := range args {
		if k < lead {
			leadV = append(leadV, Eval(a, env))
			continue
		}
		if sp, ok, e := spanOf([]Node{a}, env.Schema, env.Table); ok {
			if e != 0 {
				acc.AddValue(Err(e))
				continue
			}
			env.eachSpan(sp, func(sub *Env) { acc.AddRange(Eval(a, sub)) })
			continue
		}
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
		if IsSpill(a) {
			r, err := env.spill(a)
			if err.IsErr() {
				acc.AddValue(err)
				continue
			}
			for _, vals := range r.values {
				for _, v := range vals {
					acc.AddRange(v)
				}
			}
			continue
		}
		t, e := arrayTable([]Node{a}, env.Schema, env.Table)
		switch {
		case e != 0:
			acc.AddValue(Err(e))
		case t != nil:
			for row := range env.Data[t] {
				acc.AddRange(Eval(a, env.inArray(t, row)))
			}
		default:
			acc.AddValue(Eval(a, env))
		}
	}
	if fn == "TEXTJOIN" {
		return acc.TextResult(leadV[0], leadV[1])
	}
	return acc.Result()
}

// scan exécute une fonction de parcours (voir scan.go).
func (env *Env) scan(sc scanCall) Value {
	if sc.err != 0 {
		return Err(sc.err)
	}
	params := make([]Value, len(sc.params))
	for i, p := range sc.params {
		params[i] = Eval(p, env)
	}
	s := sc.def.make(params)
	if len(sc.cols) == 0 {
		return s.result()
	}
	vals := make([]Value, len(sc.cols))
	row := func(e *Env) {
		for i, c := range sc.cols {
			vals[i] = Eval(c, e)
		}
		s.add(vals)
	}
	if sp, ok, e := spanOf(sc.cols, env.Schema, env.Table); ok {
		if e != 0 {
			return Err(e)
		}
		env.eachSpan(sp, row)
		return s.result()
	}
	t, e := arrayTable(sc.cols, env.Schema, env.Table)
	if e != 0 {
		return Err(e)
	}
	if t == nil {
		row(env) // valeurs simples : une seule « ligne »
	} else {
		for r := range env.Data[t] {
			row(env.inArray(t, r))
		}
	}
	return s.result()
}

// special évalue les fonctions propres à chaque backend.
func (env *Env) special(fn string, n Call) Value {
	switch fn {
	case "LET":
		body, ok := expandLet(n)
		if !ok {
			return Err(ErrValue)
		}
		return Eval(body, env)
	case "REFERENCE":
		return env.reference(n)
	case "REFERENCE.APPROX":
		return env.scan(scanOf(n, env.Schema, env.Table))
	case "EXISTS":
		t, k, err := existsTarget(env.Schema, n)
		if err.IsErr() {
			return err
		}
		key := Eval(n.Args[1], env)
		if key.IsErr() {
			return key
		}
		for row := range env.Data[t] {
			if Equal(env.cell(t, row, k), key) {
				return Boolean(true)
			}
		}
		return Boolean(false)
	case "SUBTOTAL":
		agg, err := subtotalOf(n)
		if err.IsErr() {
			return err
		}
		return env.aggregate(agg, n.Args[1:])
	case "PREVIOUS", "NEXT":
		if env.Table == nil {
			return Err(ErrRef)
		}
		r := env.Row - 1
		if fn == "NEXT" {
			r = env.Row + 1
		}
		if r < 0 || r >= len(env.Data[env.Table]) {
			if len(n.Args) > 1 {
				return Eval(n.Args[1], env)
			}
			return Value{}
		}
		sub := *env
		sub.Row = r
		return Eval(n.Args[0], &sub)
	case "RUNNINGSUM":
		if env.Table == nil {
			return Err(ErrRef)
		}
		acc := Agg{Fn: "SUM"}
		for r := 0; r <= env.Row && r < len(env.Data[env.Table]); r++ {
			sub := *env
			sub.Row = r
			acc.AddRange(Eval(n.Args[0], &sub))
		}
		return acc.Result()
	case "TODAY":
		return DateSerial(math.Floor(env.Schema.nowSerial()))
	case "NOW":
		return DateSerial(env.Schema.nowSerial())
	case "RAND", "RANDBETWEEN":
		if env.rands == nil {
			env.rands = new(int)
		}
		k := *env.rands
		*env.rands++
		var row int64
		if env.Table != nil {
			row = int64(env.Row + 1)
		}
		r := Num(randAt(env.Schema.seed(), row, k))
		if fn == "RAND" {
			return r
		}
		return fnRandBetween([]Value{Eval(n.Args[0], env), Eval(n.Args[1], env), r})
	}
	return Err(ErrName) // RECHERCHEV, RECHERCHEX, RECHERCHEH : à convertir (voir Convert)
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
	if len(n.Args) != 3 { // REFERENCE(T[Col]; clé) est relue en (T; Col; clé)
		return nil, 0, Err(ErrValue)
	}
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
