package formula

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Mise en forme des tableaux dynamiques (audit des formules du 30/10/2026) :
// ASSEMB.V, ASSEMB.H, CHOISIRCOLS, CHOISIRLIGNES, VERS.COL, VERS.LIGNE, et
// le 3e argument (colonnes) de PRENDRE et EXCLURE. Le moteur SQL fixe le
// nombre de colonnes à la compilation : les numéros de colonnes, les modes
// de VERS.COL et la source de VERS.LIGNE (une plage ou une constante) sont
// écrits dans la formule ; les deux moteurs appliquent la même règle.

// stackFn : fonctions dont chaque argument est une source, qui peut être un
// tableau nommé (ASSEMB.V(Ventes2025; Ventes2026)).
func stackFn(fn string) bool { return fn == "VSTACK" || fn == "HSTACK" }

// litInt lit un entier écrit dans la formule (2, -1, +3) ; tronqué.
func litInt(n Node) (int, bool) {
	switch m := n.(type) {
	case NumLit:
		return int(math.Trunc(m.V)), true
	case Unary:
		if v, ok := litInt(m.X); ok && (m.Op == "-" || m.Op == "+") {
			if m.Op == "-" {
				v = -v
			}
			return v, true
		}
	case BoolLit:
		return boolInt(m.V), true
	}
	return 0, false
}

// pickIndex : rang 1… ou négatif depuis la fin, parmi n ; -1 si hors limites.
func pickIndex(i, n int) int {
	if i < 0 {
		i += n + 1
	}
	if i < 1 || i > n {
		return -1
	}
	return i - 1
}

// colsPicked lit les numéros de colonnes de CHOISIRCOLS parmi k colonnes.
func colsPicked(args []Node, k int) ([]int, bool) {
	var out []int
	for _, a := range args {
		i, ok := litInt(a)
		if !ok || pickIndex(i, k) < 0 {
			return nil, false
		}
		out = append(out, pickIndex(i, k))
	}
	return out, len(out) > 0
}

// takeCols : colonnes gardées par le 3e argument de PRENDRE / EXCLURE (nil :
// toutes) ; faux si l'argument n'est pas un entier écrit ou ne garde rien.
func takeCols(c Call, k int) ([]int, bool) {
	if len(c.Args) < 3 || c.Args[2] == nil {
		return nil, true
	}
	n, ok := litInt(c.Args[2])
	if !ok {
		return nil, false
	}
	lo, hi := takeRange(c.Fn == "DROP", n, k)
	if lo >= hi {
		return nil, false
	}
	out := make([]int, 0, hi-lo)
	for i := lo; i < hi; i++ {
		out = append(out, i)
	}
	return out, true
}

// tocolModes lit ignorer (0 à 3) et par_colonne de VERS.COL / VERS.LIGNE.
func tocolModes(c Call) (blanks, errs, byCol, ok bool) {
	ign := 0
	if len(c.Args) > 1 && c.Args[1] != nil {
		if ign, ok = litInt(c.Args[1]); !ok || ign < 0 || ign > 3 {
			return false, false, false, false
		}
	}
	if len(c.Args) > 2 && c.Args[2] != nil {
		var b int
		if b, ok = litInt(c.Args[2]); !ok {
			return false, false, false, false
		}
		byCol = b != 0
	}
	return ign&1 != 0, ign&2 != 0, byCol, true
}

// uniqueHeaders numérote les en-têtes répétés (Valeur, Valeur2…) : ASSEMB.H
// peut réunir deux colonnes de même nom.
func uniqueHeaders(cols []string) []string {
	seen := map[string]int{}
	out := make([]string, len(cols))
	for i, c := range cols {
		key := strings.ToLower(c)
		seen[key]++
		out[i] = c
		for n := seen[key]; n > 1; n++ {
			cand := c + strconv.Itoa(n)
			if seen[strings.ToLower(cand)] == 0 {
				seen[strings.ToLower(cand)] = 1
				out[i] = cand
				break
			}
		}
	}
	return out
}

func padHeaders(cols []string, k int) []string {
	out := append([]string(nil), cols...)
	for i := len(out); i < k; i++ {
		out = append(out, fmt.Sprintf("Valeur%d", i+1))
	}
	return uniqueHeaders(out)
}

func isBlankValue(v Value) bool { return fnIsBlank([]Value{v}).Bool }
func fnIsBlank(a []Value) Value { f, _ := lookupFunc("ISBLANK"); return f.impl(a) }

// --- Évaluateur Go ------------------------------------------------------------

// arrayShape évalue les fonctions de ce fichier (voir env.spill).
func (env *Env) arrayShape(c Call) (rel, Value) {
	switch c.Fn {
	case "VSTACK", "HSTACK":
		var srcs []rel
		for _, a := range c.Args {
			r, e := env.spill(a)
			if e.IsErr() {
				return rel{}, e
			}
			if r.long {
				return rel{}, Err(ErrValue)
			}
			srcs = append(srcs, r)
		}
		if c.Fn == "VSTACK" {
			k := 0
			for _, r := range srcs {
				k = max(k, len(r.cols))
			}
			out := rel{cols: padHeaders(srcs[0].cols, k)}
			for _, r := range srcs {
				for _, vals := range r.values {
					row := append([]Value(nil), vals...)
					for len(row) < k {
						row = append(row, Err(ErrNA))
					}
					out.values = append(out.values, row)
				}
			}
			return out, Value{}
		}
		n := 0
		var cols []string
		for _, r := range srcs {
			n = max(n, len(r.values))
			cols = append(cols, r.cols...)
		}
		out := rel{cols: uniqueHeaders(cols)}
		for i := 0; i < n; i++ {
			var row []Value
			for _, r := range srcs {
				if i < len(r.values) {
					row = append(row, r.values[i]...)
					continue
				}
				for range r.cols {
					row = append(row, Err(ErrNA))
				}
			}
			out.values = append(out.values, row)
		}
		return out, Value{}
	case "TOROW":
		vals, e := env.constGrid(c.Args[0])
		if e.IsErr() {
			return rel{}, e
		}
		flat, ok := flatten(vals, c)
		if !ok || len(flat) == 0 {
			return rel{}, Err(ErrValue)
		}
		return rel{cols: seqHeaders(len(flat)), values: [][]Value{flat}}, Value{}
	}
	src, e := env.spill(c.Args[0])
	if e.IsErr() {
		return rel{}, e
	}
	if src.long {
		return rel{}, Err(ErrValue)
	}
	switch c.Fn {
	case "CHOOSECOLS":
		idx, ok := colsPicked(c.Args[1:], len(src.cols))
		if !ok {
			return rel{}, Err(ErrValue)
		}
		return pickCols(src, idx), Value{}
	case "CHOOSEROWS":
		out := rel{cols: src.cols}
		for _, a := range c.Args[1:] {
			v := Eval(a, env)
			f, e := ToNumber(v)
			i := -1
			if !e.IsErr() {
				i = pickIndex(int(math.Trunc(f)), len(src.values))
			}
			row := make([]Value, len(src.cols))
			if i < 0 {
				for j := range row {
					row[j] = Err(ErrValue)
				}
			} else {
				copy(row, src.values[i])
			}
			out.values = append(out.values, row)
		}
		return out, Value{}
	case "TOCOL":
		flat, ok := flatten(src.values, c)
		if !ok {
			return rel{}, Err(ErrValue)
		}
		out := rel{cols: seqHeaders(1)}
		for _, v := range flat {
			out.values = append(out.values, []Value{v})
		}
		return out, Value{}
	}
	return rel{}, Err(ErrValue)
}

// pickCols garde les colonnes idx (dans cet ordre), et les lignes d'un
// tableau source pour FILTRE ou TRIERPAR.
func pickCols(src rel, idx []int) rel {
	out := rel{base: src.base, rows: src.rows}
	for _, i := range idx {
		out.cols = append(out.cols, src.cols[i])
	}
	out.cols = uniqueHeaders(out.cols)
	for _, vals := range src.values {
		row := make([]Value, len(idx))
		for j, i := range idx {
			row[j] = vals[i]
		}
		out.values = append(out.values, row)
	}
	return out
}

// flatten met les valeurs bout à bout (par lignes, ou par colonnes), sans les
// vides ni les erreurs selon le mode de VERS.COL.
func flatten(vals [][]Value, c Call) ([]Value, bool) {
	blanks, errs, byCol, ok := tocolModes(c)
	if !ok {
		return nil, false
	}
	var out []Value
	add := func(v Value) {
		if blanks && isBlankValue(v) || errs && v.IsErr() {
			return
		}
		out = append(out, v)
	}
	if len(vals) == 0 {
		return out, true
	}
	if byCol {
		for j := range vals[0] {
			for _, row := range vals {
				add(row[j])
			}
		}
		return out, true
	}
	for _, row := range vals {
		for _, v := range row {
			add(v)
		}
	}
	return out, true
}

// --- Compilation SQL ----------------------------------------------------------

func (c *compiler) arrayShape(call Call, sc scope) (sqlRel, ErrCode) {
	switch call.Fn {
	case "VSTACK", "HSTACK":
		var srcs []sqlRel
		for _, a := range call.Args {
			r, e := c.spill(a, sc)
			if e != 0 {
				return sqlRel{}, e
			}
			if r.query || r.long {
				return sqlRel{}, ErrValue
			}
			srcs = append(srcs, r)
		}
		if call.Fn == "VSTACK" {
			k := 0
			for _, r := range srcs {
				k = max(k, len(r.cols))
			}
			parts := make([]string, len(srcs))
			for p, r := range srcs {
				sel := make([]string, k)
				for j := range sel {
					if j < len(r.cols) {
						sel[j] = fmt.Sprintf("v%d", j+1)
					} else {
						sel[j] = fmt.Sprintf("%s AS v%d", errSQL(ErrNA), j+1)
					}
				}
				parts[p] = fmt.Sprintf("SELECT %s, %d AS _p, _n AS _q FROM (%s)", strings.Join(sel, ", "), p, r.sql())
			}
			return sqlRel{cols: padHeaders(srcs[0].cols, k), from: fmt.Sprintf(
				"SELECT %s, row_number() OVER (ORDER BY _p, _q) AS _n FROM (%s)", vcols(k, ""), strings.Join(parts, " UNION ALL "))}, 0
		}
		var cols, idx, sel, joins []string
		for _, r := range srcs {
			num := fmt.Sprintf("(SELECT *, row_number() OVER (ORDER BY _n) AS _r FROM (%s))", r.sql())
			al := c.newAlias()
			idx = append(idx, "SELECT _r FROM "+num)
			joins = append(joins, fmt.Sprintf("LEFT JOIN %s AS %s ON %s._r = _i._r", num, al, al))
			for j := range r.cols {
				sel = append(sel, fmt.Sprintf("CASE WHEN %s._r IS NULL THEN %s ELSE %s.v%d END AS v%d", al, errSQL(ErrNA), al, j+1, len(sel)+1))
			}
			cols = append(cols, r.cols...)
		}
		return sqlRel{cols: uniqueHeaders(cols), from: fmt.Sprintf("SELECT %s, _i._r AS _n FROM (%s) AS _i %s",
			strings.Join(sel, ", "), strings.Join(idx, " UNION "), strings.Join(joins, " "))}, 0
	case "TOROW":
		vals, e := c.constGrid(call.Args[0], sc)
		if e != 0 {
			return sqlRel{}, e
		}
		flat, ok := flatten(vals, call)
		if !ok || len(flat) == 0 {
			return sqlRel{}, ErrValue
		}
		return c.constRel([][]Value{flat})
	}
	src, e := c.spill(call.Args[0], sc)
	if e != 0 {
		return sqlRel{}, e
	}
	if src.query || src.long {
		return sqlRel{}, ErrValue
	}
	k := len(src.cols)
	switch call.Fn {
	case "CHOOSECOLS":
		idx, ok := colsPicked(call.Args[1:], k)
		if !ok {
			return sqlRel{}, ErrValue
		}
		return c.pickCols(src, idx), 0
	case "CHOOSEROWS":
		num := fmt.Sprintf("(SELECT *, row_number() OVER (ORDER BY _n) AS _r, count(*) OVER () AS _c FROM (%s))", src.sql())
		var parts []string
		for j, a := range call.Args[1:] {
			i := fmt.Sprintf("CAST(%s AS INTEGER)", c.expr(a, sc))
			al := c.newAlias()
			sel := make([]string, k)
			for m := range sel {
				sel[m] = fmt.Sprintf("CASE WHEN %s._r IS NULL THEN %s ELSE %s.v%d END AS v%d", al, errSQL(ErrValue), al, m+1, m+1)
			}
			parts = append(parts, fmt.Sprintf("SELECT %s, %d AS _n FROM (SELECT 1) LEFT JOIN %s AS %s ON %s._r = CASE WHEN %s < 0 THEN %s._c + 1 + %s ELSE %s END",
				strings.Join(sel, ", "), j+1, num, al, al, i, al, i, i))
		}
		return sqlRel{cols: src.cols, from: strings.Join(parts, " UNION ALL ")}, 0
	case "TOCOL":
		blanks, errs, byCol, ok := tocolModes(call)
		if !ok {
			return sqlRel{}, ErrValue
		}
		var where []string
		if blanks {
			where = append(where, fmt.Sprintf("NOT slt_true(slt_fn(%s, v1))", c.param("ISBLANK")))
		}
		if errs {
			where = append(where, fmt.Sprintf("NOT slt_true(slt_fn(%s, v1))", c.param("ISERROR")))
		}
		parts := make([]string, k)
		for j := range parts {
			parts[j] = fmt.Sprintf("SELECT v%d AS v1, _n AS _a, %d AS _b FROM (%s)", j+1, j, src.sql())
		}
		order := "_a, _b"
		if byCol {
			order = "_b, _a"
		}
		q := "SELECT * FROM (" + strings.Join(parts, " UNION ALL ") + ")"
		if len(where) > 0 {
			q += " WHERE " + strings.Join(where, " AND ")
		}
		return sqlRel{cols: seqHeaders(1), from: fmt.Sprintf("SELECT v1, row_number() OVER (ORDER BY %s) AS _n FROM (%s)", order, q)}, 0
	}
	return sqlRel{}, ErrValue
}

// pickCols : CHOISIRCOLS en SQL ; les lignes d'un tableau restent une
// source pour FILTRE ou TRIERPAR.
func (c *compiler) pickCols(src sqlRel, idx []int) sqlRel {
	var cols []string
	for _, i := range idx {
		cols = append(cols, src.cols[i])
	}
	cols = uniqueHeaders(cols)
	if src.base != nil {
		out := src
		out.cols, out.exprs = cols, nil
		for _, i := range idx {
			out.exprs = append(out.exprs, src.exprs[i])
		}
		return out
	}
	sel := make([]string, len(idx))
	for j, i := range idx {
		sel[j] = fmt.Sprintf("v%d AS v%d", i+1, j+1)
	}
	return sqlRel{cols: cols, from: fmt.Sprintf("SELECT %s, _n FROM (%s)", strings.Join(sel, ", "), src.sql())}
}
