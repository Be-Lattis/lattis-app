package query

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// Compilation des étapes en SQL (SQLite) : chaque étape devient un CTE qui
// lit le précédent, sous la forme
//
//	WITH q1_s0 AS (SELECT * FROM src_3),
//	     q1_s1 AS (SELECT * FROM q1_s0 WHERE "Ville" = 'Paris'),
//	     q1_s2 AS (SELECT "Ville", sum("Montant") AS "Total" FROM q1_s1 GROUP BY "Ville")
//	SELECT * FROM q1_s2
//
// Une requête citée (source, fusion, ajout) est compilée de même et posée
// en sous-requête. Les colonnes portent leur nom affiché, entre guillemets.

// MaxPivotValues borne les colonnes créées par un pivot.
const MaxPivotValues = 50

// pipeline est une requête en cours de compilation.
type pipeline struct {
	ctes  []string
	names []string   // nom de chaque CTE
	at    [][]Column // colonnes de chaque CTE
	last  string
	cols  []Column
}

// upTo donne le pipeline arrêté au CTE k (0 : la source).
func (p *pipeline) upTo(k int) *pipeline {
	return &pipeline{ctes: p.ctes[:k+1], names: p.names[:k+1], at: p.at[:k+1], last: p.names[k], cols: p.at[k]}
}

func (p *pipeline) sql() string { return p.with("SELECT * FROM " + p.last) }

// with précède sel des CTE de la requête.
func (p *pipeline) with(sel string) string {
	return "WITH " + strings.Join(p.ctes, ",\n") + "\n" + sel
}

func (p *pipeline) push(name, sel string, cols []Column) {
	p.ctes = append(p.ctes, name+" AS ("+sel+")")
	p.names = append(p.names, name)
	p.at = append(p.at, cols)
	p.last, p.cols = name, cols
}

// qi écrit un identifiant SQLite entre guillemets.
func qi(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

// qs écrit une chaîne SQL.
func qs(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// lit écrit une valeur stockée (int64, float64, string, nil) en SQL.
func lit(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		if math.IsInf(x, 0) || math.IsNaN(x) {
			return "NULL"
		}
		s := strconv.FormatFloat(x, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
		return s
	case string:
		return qs(x)
	}
	return qs(fmt.Sprint(v))
}

// literal lit une valeur saisie (français) au type d'une colonne.
func literal(raw string, t grid.Type) (string, error) {
	if t == "" || t == grid.Text {
		return qs(raw), nil
	}
	v, err := grid.Parse(t, raw)
	if err != nil {
		return "", msg.New("query.notOfType", msg.Args{"value": raw, "type": TypeLabel(t)})
	}
	return lit(v), nil
}

// find cherche une colonne par son nom (la casse exacte d'abord).
func find(cols []Column, name string) (int, error) {
	for i, c := range cols {
		if c.Name == name {
			return i, nil
		}
	}
	for i, c := range cols {
		if strings.EqualFold(c.Name, name) {
			return i, nil
		}
	}
	return -1, msg.New("query.columnNotFound", msg.Args{"column": name})
}

// unique rend un nom de colonne unique parmi cols (« Nom », « Nom 2 »…).
func unique(cols []Column, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Colonne"
	}
	cand := name
	for n := 2; ; n++ {
		if _, err := find(cols, cand); err != nil {
			return cand
		}
		cand = name + " " + strconv.Itoa(n)
	}
}

// joinType donne le type d'une colonne qui reçoit des valeurs des deux
// types ; un type à inférer le reste.
func joinType(a, b grid.Type) grid.Type {
	if a == "" || b == "" {
		return ""
	}
	return canvas.Join(a, b)
}

func selectAll(cols []Column) string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = qi(c.Name)
	}
	return strings.Join(out, ", ")
}

// step compile une étape qui lit le CTE prev (colonnes cols).
func (r *run) step(p *pipeline, s Step) (string, []Column, error) {
	prev, cols := p.last, p.cols
	switch s.Op {
	case OpFilter:
		if len(s.Conditions) == 0 {
			return "", nil, msg.New("query.filterNoCondition", nil)
		}
		var conds []string
		for _, c := range s.Conditions {
			e, err := condition(cols, c)
			if err != nil {
				return "", nil, err
			}
			conds = append(conds, e)
		}
		op := " AND "
		if s.Any {
			op = " OR "
		}
		return "SELECT * FROM " + prev + " WHERE " + strings.Join(conds, op), cols, nil

	case OpRename:
		i, err := find(cols, s.Column)
		if err != nil {
			return "", nil, err
		}
		name := strings.TrimSpace(s.Name)
		if name == "" {
			return "", nil, msg.New("query.emptyNewName", nil)
		}
		if j, err := find(cols, name); err == nil && j != i {
			return "", nil, msg.New("column.nameTaken", msg.Args{"column": name})
		}
		out := append([]Column{}, cols...)
		out[i].Name = name
		return r.project(prev, cols, out, map[int]string{}), out, nil

	case OpType:
		i, err := find(cols, s.Column)
		if err != nil {
			return "", nil, err
		}
		switch s.Type {
		case grid.Integer, grid.Decimal, grid.Text, grid.Boolean, grid.Date:
		default:
			return "", nil, msg.New("query.unknownType", msg.Args{"type": fmt.Sprintf("%q", s.Type)})
		}
		from := cols[i].Type
		if from == "" {
			from = grid.Text
		}
		out := append([]Column{}, cols...)
		out[i].Type = s.Type
		expr := qi(cols[i].Name)
		if from != s.Type || cols[i].Type == "" {
			expr = fmt.Sprintf("slt_convert(%s, '%s', '%s')", expr, from, s.Type)
		}
		return r.project(prev, cols, out, map[int]string{i: expr}), out, nil

	case OpRemove, OpSelect:
		if len(s.Columns) == 0 {
			return "", nil, msg.New("query.noColumnChosen", nil)
		}
		idx := map[int]bool{}
		var order []int
		for _, n := range s.Columns {
			i, err := find(cols, n)
			if err != nil {
				return "", nil, err
			}
			if !idx[i] {
				idx[i] = true
				order = append(order, i)
			}
		}
		var out []Column
		if s.Op == OpSelect {
			for _, i := range order {
				out = append(out, cols[i])
			}
		} else {
			for i, c := range cols {
				if !idx[i] {
					out = append(out, c)
				}
			}
			if len(out) == 0 {
				return "", nil, msg.New("query.keepOneColumn", nil)
			}
		}
		return "SELECT " + selectAll(out) + " FROM " + prev, out, nil

	case OpSplit:
		i, err := find(cols, s.Column)
		if err != nil {
			return "", nil, err
		}
		if s.Delimiter == "" {
			return "", nil, msg.New("query.emptyDelimiter", nil)
		}
		parts := s.Parts
		if parts < 2 {
			parts = 2
		}
		if parts > 100 {
			return "", nil, msg.New("query.maxColumns", nil)
		}
		var out []Column
		var sel []string
		rest := append(append([]Column{}, cols[:i]...), cols[i+1:]...)
		for j, c := range cols {
			if j != i {
				out = append(out, c)
				sel = append(sel, qi(c.Name))
				continue
			}
			for k := 1; k <= parts; k++ {
				name := unique(rest, fmt.Sprintf("%s.%d", c.Name, k))
				rest = append(rest, Column{Name: name})
				out = append(out, Column{Name: name})
				sel = append(sel, fmt.Sprintf("slt_split(%s, %s, %d) AS %s", qi(c.Name), qs(s.Delimiter), k, qi(name)))
			}
		}
		return "SELECT " + strings.Join(sel, ", ") + " FROM " + prev, out, nil

	case OpReplace:
		i, err := find(cols, s.Column)
		if err != nil {
			return "", nil, err
		}
		c := cols[i]
		var expr string
		if c.Type == grid.Text || c.Type == "" {
			if s.Value == "" {
				return "", nil, msg.New("query.emptySearch", nil)
			}
			expr = fmt.Sprintf("replace(%s, %s, %s)", qi(c.Name), qs(s.Value), qs(s.With))
		} else {
			find, err := literal(s.Value, c.Type)
			if err != nil {
				return "", nil, err
			}
			with := "NULL"
			if strings.TrimSpace(s.With) != "" {
				if with, err = literal(s.With, c.Type); err != nil {
					return "", nil, err
				}
			}
			expr = fmt.Sprintf("CASE WHEN %s = %s THEN %s ELSE %s END", qi(c.Name), find, with, qi(c.Name))
		}
		return r.project(prev, cols, cols, map[int]string{i: expr}), cols, nil

	case OpTransform:
		i, err := find(cols, s.Column)
		if err != nil {
			return "", nil, err
		}
		fn := map[string]string{"trim": "trim", "upper": "upper", "lower": "lower"}[s.Fn]
		if fn == "" {
			return "", nil, msg.New("query.unknownTransform", msg.Args{"transform": fmt.Sprintf("%q", s.Fn)})
		}
		out := append([]Column{}, cols...)
		if out[i].Type != grid.Text {
			out[i].Type = ""
		}
		return r.project(prev, cols, out, map[int]string{i: fmt.Sprintf("%s(%s)", fn, qi(cols[i].Name))}), out, nil

	case OpAddColumn:
		name := strings.TrimSpace(s.Name)
		if name == "" {
			return "", nil, msg.New("query.emptyColumnName", nil)
		}
		if _, err := find(cols, name); err == nil {
			return "", nil, msg.New("column.nameTaken", msg.Args{"column": name})
		}
		expr, err := expression(cols, s.Expr)
		if err != nil {
			return "", nil, err
		}
		if s.Type != "" {
			expr = fmt.Sprintf("slt_convert(%s, 'text', '%s')", expr, s.Type)
		}
		out := append(append([]Column{}, cols...), Column{Name: name, Type: s.Type})
		return "SELECT *, " + expr + " AS " + qi(name) + " FROM " + prev, out, nil

	case OpIndex:
		name := strings.TrimSpace(s.Name)
		if name == "" {
			name = "Index"
		}
		name = unique(cols, name)
		out := append(append([]Column{}, cols...), Column{Name: name, Type: grid.Integer})
		return fmt.Sprintf("SELECT *, row_number() OVER () + %d AS %s FROM %s", s.Start-1, qi(name), prev), out, nil

	case OpMerge:
		return r.merge(p, s)

	case OpAppend:
		if s.Other == nil {
			return "", nil, msg.New("query.missingAppend", nil)
		}
		from, other, err := r.from(*s.Other)
		if err != nil {
			return "", nil, err
		}
		out := append([]Column{}, cols...)
		for _, c := range other {
			if i, err := find(out, c.Name); err == nil {
				out[i].Type = joinType(out[i].Type, c.Type)
			} else {
				out = append(out, c)
			}
		}
		side := func(have []Column) string {
			var sel []string
			for _, c := range out {
				if _, err := find(have, c.Name); err == nil {
					sel = append(sel, qi(c.Name))
				} else {
					sel = append(sel, "NULL AS "+qi(c.Name))
				}
			}
			return strings.Join(sel, ", ")
		}
		return "SELECT " + side(cols) + " FROM " + prev + " UNION ALL SELECT " + side(other) + " FROM " + from, out, nil

	case OpGroup:
		var keys, sel []string
		var out []Column
		for _, n := range s.Columns {
			i, err := find(cols, n)
			if err != nil {
				return "", nil, err
			}
			keys = append(keys, qi(cols[i].Name))
			sel = append(sel, qi(cols[i].Name))
			out = append(out, cols[i])
		}
		if len(s.Aggs) == 0 && len(keys) == 0 {
			return "", nil, msg.New("query.groupEmpty", nil)
		}
		for _, a := range s.Aggs {
			e, t, err := aggregate(cols, a.Op, a.Column)
			if err != nil {
				return "", nil, err
			}
			name := strings.TrimSpace(a.Name)
			if name == "" {
				name = aggName(a.Op, a.Column)
			}
			if _, err := find(out, name); err == nil {
				return "", nil, msg.New("column.nameTaken", msg.Args{"column": name})
			}
			sel = append(sel, e+" AS "+qi(name))
			out = append(out, Column{Name: name, Type: t})
		}
		q := "SELECT " + strings.Join(sel, ", ") + " FROM " + prev
		if len(keys) > 0 {
			q += " GROUP BY " + strings.Join(keys, ", ") + " ORDER BY " + strings.Join(keys, ", ")
		}
		return q, out, nil

	case OpSort:
		if len(s.Sort) == 0 {
			return "", nil, msg.New("query.noSortKey", nil)
		}
		var by []string
		for _, k := range s.Sort {
			i, err := find(cols, k.Column)
			if err != nil {
				return "", nil, err
			}
			e := qi(cols[i].Name)
			text := cols[i].Type == grid.Text || cols[i].Type == ""
			if text {
				e += " COLLATE NOCASE"
			}
			if len(k.Order) > 0 {
				v := qi(cols[i].Name)
				if !text {
					v = "CAST(" + v + " AS TEXT)"
				}
				// Les vides à la fin, puis la liste, puis les autres.
				by = append(by, qi(cols[i].Name)+" IS NULL", canvas.OrderBy(v, e, canvas.CustomOrder{Values: k.Order, Desc: k.Desc}))
				continue
			}
			if k.Desc {
				e += " DESC"
			}
			by = append(by, e)
		}
		return "SELECT * FROM " + prev + " ORDER BY " + strings.Join(by, ", "), cols, nil

	case OpDistinct:
		if len(s.Columns) == 0 {
			return "SELECT DISTINCT * FROM " + prev, cols, nil
		}
		var keys []string
		for _, n := range s.Columns {
			i, err := find(cols, n)
			if err != nil {
				return "", nil, err
			}
			keys = append(keys, qi(cols[i].Name))
		}
		return fmt.Sprintf(`SELECT %s FROM (SELECT *, row_number() OVER (PARTITION BY %s) AS "__rn", row_number() OVER () AS "__ord" FROM %s) WHERE "__rn" = 1 ORDER BY "__ord"`,
			selectAll(cols), strings.Join(keys, ", "), prev), cols, nil

	case OpTop:
		if s.Count < 0 {
			return "", nil, msg.New("query.negativeRows", nil)
		}
		return fmt.Sprintf("SELECT * FROM %s LIMIT %d", prev, s.Count), cols, nil

	case OpUnpivot:
		return unpivot(prev, cols, s)

	case OpPivot:
		return r.pivot(p, s)
	}
	return "", nil, msg.New("query.unknownStep", msg.Args{"step": fmt.Sprintf("%q", s.Op)})
}

// project recopie les colonnes de cols sous les noms de out, avec des
// expressions remplacées (par rang).
func (r *run) project(prev string, cols, out []Column, exprs map[int]string) string {
	sel := make([]string, len(cols))
	for i, c := range cols {
		e, ok := exprs[i]
		if !ok {
			e = qi(c.Name)
		}
		sel[i] = e + " AS " + qi(out[i].Name)
	}
	return "SELECT " + strings.Join(sel, ", ") + " FROM " + prev
}

// condition compile une condition de filtre.
func condition(cols []Column, c Cond) (string, error) {
	i, err := find(cols, c.Column)
	if err != nil {
		return "", err
	}
	col := cols[i]
	x := qi(col.Name)
	text := "CAST(" + x + " AS TEXT)"
	low := func() string { return "lower(" + qs(c.Value) + ")" }
	switch c.Cmp {
	case "empty":
		return "(" + x + " IS NULL OR " + x + " = '')", nil
	case "notempty":
		return "(" + x + " IS NOT NULL AND " + x + " <> '')", nil
	case "contains":
		return "instr(lower(" + text + "), " + low() + ") > 0", nil
	case "notcontains":
		return "(" + x + " IS NULL OR instr(lower(" + text + "), " + low() + ") = 0)", nil
	case "begins":
		return "substr(lower(" + text + "), 1, length(" + qs(c.Value) + ")) = " + low(), nil
	case "ends":
		return "substr(lower(" + text + "), -length(" + qs(c.Value) + ")) = " + low(), nil
	case "=", "<>", "<", "<=", ">", ">=":
		v, err := literal(c.Value, col.Type)
		if err != nil {
			return "", msg.Wrap(err, "query.filterError", msg.Args{"column": col.Name, "err": err})
		}
		if c.Cmp == "<>" {
			return "(" + x + " IS NULL OR " + x + " <> " + v + ")", nil
		}
		return x + " " + c.Cmp + " " + v, nil
	}
	return "", msg.New("query.unknownComparison", msg.Args{"comparison": fmt.Sprintf("%q", c.Cmp)})
}

// expression traduit une expression de colonne personnalisée : SQLite, où
// [Colonne] désigne une colonne (les chaînes '...' ne sont pas touchées).
func expression(cols []Column, src string) (string, error) {
	if strings.TrimSpace(src) == "" {
		return "", msg.New("query.emptyExpression", nil)
	}
	if strings.Contains(src, ";") {
		return "", msg.New("query.expressionSemicolon", nil)
	}
	var b strings.Builder
	rs := []rune(src)
	for i := 0; i < len(rs); i++ {
		switch r := rs[i]; r {
		case '\'', '"':
			j := i + 1
			for j < len(rs) && rs[j] != r {
				j++
			}
			if j >= len(rs) {
				return "", msg.New("query.unclosedString", nil)
			}
			b.WriteString(string(rs[i : j+1]))
			i = j
		case '[':
			j := i + 1
			for j < len(rs) && rs[j] != ']' {
				j++
			}
			if j >= len(rs) {
				return "", msg.New("query.unbalancedBracket", nil)
			}
			k, err := find(cols, string(rs[i+1:j]))
			if err != nil {
				return "", err
			}
			b.WriteString(qi(cols[k].Name))
			i = j
		default:
			b.WriteRune(r)
		}
	}
	return "(" + b.String() + ")", nil
}

// aggregate compile un agrégat et donne son type.
func aggregate(cols []Column, op, column string) (string, grid.Type, error) {
	if op == "count" {
		return "count(*)", grid.Integer, nil
	}
	i, err := find(cols, column)
	if err != nil {
		return "", "", err
	}
	c := cols[i]
	x := qi(c.Name)
	numeric := c.Type == grid.Integer || c.Type == grid.Decimal || c.Type == grid.Boolean || c.Type == ""
	switch op {
	case "countdistinct":
		return "count(DISTINCT " + x + ")", grid.Integer, nil
	case "sum":
		if !numeric {
			return "", "", msg.New("query.sumType", msg.Args{"column": c.Name, "type": TypeLabel(c.Type)})
		}
		if c.Type == grid.Integer || c.Type == grid.Boolean {
			return "sum(" + x + ")", grid.Integer, nil
		}
		return "total(" + x + ")", grid.Decimal, nil
	case "avg":
		if !numeric {
			return "", "", msg.New("query.averageType", msg.Args{"column": c.Name, "type": TypeLabel(c.Type)})
		}
		return "avg(" + x + ")", grid.Decimal, nil
	case "min", "max":
		return op + "(" + x + ")", c.Type, nil
	}
	return "", "", msg.New("query.unknownAggregate", msg.Args{"aggregate": fmt.Sprintf("%q", op)})
}

// aggName nomme une colonne d'agrégat sans nom.
func aggName(op, column string) string {
	switch op {
	case "count":
		return "Nombre"
	case "countdistinct":
		return "Nombre distinct de " + column
	case "sum":
		return "Somme de " + column
	case "avg":
		return "Moyenne de " + column
	case "min":
		return "Min de " + column
	case "max":
		return "Max de " + column
	}
	return column
}

// merge compile une fusion avec une autre requête ou un tableau.
func (r *run) merge(p *pipeline, s Step) (string, []Column, error) {
	prev, cols := p.last, p.cols
	if s.Other == nil {
		return "", nil, msg.New("query.missingMerge", nil)
	}
	if len(s.Keys) == 0 || len(s.Keys) != len(s.OtherKeys) {
		return "", nil, msg.New("query.mergeColumnsCount", nil)
	}
	from, other, err := r.from(*s.Other)
	if err != nil {
		return "", nil, err
	}
	var on []string
	otherKey := map[int]bool{}
	for k := range s.Keys {
		i, err := find(cols, s.Keys[k])
		if err != nil {
			return "", nil, err
		}
		j, err := find(other, s.OtherKeys[k])
		if err != nil {
			return "", nil, fmt.Errorf("%s : %w", SourceLabel(*s.Other), err)
		}
		otherKey[j] = true
		a, b := "l."+qi(cols[i].Name), "r."+qi(other[j].Name)
		if cols[i].Type != other[j].Type {
			a, b = "CAST("+a+" AS TEXT)", "CAST("+b+" AS TEXT)"
		}
		on = append(on, a+" = "+b)
	}
	cond := strings.Join(on, " AND ")
	var sel []string
	for _, c := range cols {
		sel = append(sel, "l."+qi(c.Name))
	}
	join := s.Join
	if join == "" {
		join = "left"
	}
	if join == "anti" {
		return "SELECT " + strings.Join(sel, ", ") + " FROM " + prev + " l WHERE NOT EXISTS (SELECT 1 FROM " + from + " r WHERE " + cond + ")", cols, nil
	}
	kw := map[string]string{"left": "LEFT JOIN", "inner": "JOIN", "full": "FULL JOIN", "right": "RIGHT JOIN"}[join]
	if kw == "" {
		return "", nil, msg.New("query.unknownJoin", msg.Args{"join": fmt.Sprintf("%q", join)})
	}
	out := append([]Column{}, cols...)
	var bring []int
	if len(s.Columns) > 0 {
		for _, n := range s.Columns {
			j, err := find(other, n)
			if err != nil {
				return "", nil, fmt.Errorf("%s : %w", SourceLabel(*s.Other), err)
			}
			bring = append(bring, j)
		}
	} else {
		for j := range other {
			if !otherKey[j] {
				bring = append(bring, j)
			}
		}
	}
	prefix := otherName(*s.Other)
	for _, j := range bring {
		name := other[j].Name
		if _, err := find(out, name); err == nil {
			name = unique(out, prefix+"."+name)
		}
		out = append(out, Column{Name: name, Type: other[j].Type})
		sel = append(sel, "r."+qi(other[j].Name)+" AS "+qi(name))
	}
	return "SELECT " + strings.Join(sel, ", ") + " FROM " + prev + " l " + kw + " " + from + " r ON " + cond, out, nil
}

func otherName(s Source) string {
	switch s.Kind {
	case SourceQuery:
		return s.Query
	case SourceTable:
		return s.Table
	}
	return "Autre"
}

// unpivot compile un dépivotage : chaque colonne choisie devient une ligne
// (attribut, valeur) ; les valeurs vides sont omises, comme Power Query.
func unpivot(prev string, cols []Column, s Step) (string, []Column, error) {
	if len(s.Columns) == 0 {
		return "", nil, msg.New("query.nothingToUnpivot", nil)
	}
	chosen := map[int]bool{}
	var vt grid.Type = "-"
	for _, n := range s.Columns {
		i, err := find(cols, n)
		if err != nil {
			return "", nil, err
		}
		chosen[i] = true
		if vt == "-" {
			vt = cols[i].Type
		} else {
			vt = joinType(vt, cols[i].Type)
		}
	}
	var keep []Column
	for i, c := range cols {
		if !chosen[i] {
			keep = append(keep, c)
		}
	}
	attr, val := strings.TrimSpace(s.Name), strings.TrimSpace(s.ValueName)
	if attr == "" {
		attr = "Attribut"
	}
	if val == "" {
		val = "Valeur"
	}
	attr = unique(keep, attr)
	val = unique(append(keep, Column{Name: attr}), val)
	out := append(append([]Column{}, keep...), Column{Name: attr, Type: grid.Text}, Column{Name: val, Type: vt})
	var parts []string
	for i, c := range cols {
		if !chosen[i] {
			continue
		}
		v := qi(c.Name)
		if vt == grid.Text && c.Type != grid.Text {
			from := c.Type
			if from == "" {
				from = grid.Text
			}
			v = fmt.Sprintf("slt_convert(%s, '%s', 'text')", v, from)
		}
		sel := selectAll(keep)
		if sel != "" {
			sel += ", "
		}
		parts = append(parts, fmt.Sprintf("SELECT %s%s AS %s, %s AS %s FROM %s WHERE %s IS NOT NULL", sel, qs(c.Name), qi(attr), v, qi(val), prev, qi(c.Name)))
	}
	return strings.Join(parts, " UNION ALL "), out, nil
}

// distinct lit au plus n valeurs distinctes (non vides) d'une expression du
// dernier CTE, triées.
func (r *run) distinct(p *pipeline, expr string, n int) ([]any, error) {
	rs, err := r.s.db.QueryContext(r.ctx, p.with(fmt.Sprintf("SELECT DISTINCT %s FROM %s WHERE %s IS NOT NULL ORDER BY 1 LIMIT %d", expr, p.last, expr, n)))
	if err != nil {
		return nil, errors.New(sqlMessage(err))
	}
	defer rs.Close()
	var out []any
	for rs.Next() {
		var v any
		if err := rs.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, plain(v))
	}
	return out, rs.Err()
}

// pivot compile un pivot : chaque valeur distincte de la colonne attribut
// devient une colonne, qui agrège la colonne valeur ; les autres colonnes
// regroupent les lignes.
func (r *run) pivot(p *pipeline, s Step) (string, []Column, error) {
	prev, cols := p.last, p.cols
	ai, err := find(cols, s.Column)
	if err != nil {
		return "", nil, err
	}
	vi, err := find(cols, s.ValueColumn)
	if err != nil {
		return "", nil, err
	}
	if ai == vi {
		return "", nil, msg.New("query.pivotValuesSame", nil)
	}
	op := s.Agg
	if op == "" {
		op = "sum"
	}
	if op == "count" {
		op = "countv"
	}
	values, err := r.distinct(p, qi(cols[ai].Name), MaxPivotValues+1)
	if err != nil {
		return "", nil, err
	}
	if len(values) > MaxPivotValues {
		return "", nil, msg.New("query.pivotTooManyValues", msg.Args{"max": MaxPivotValues, "column": cols[ai].Name})
	}
	var keep []Column
	for i, c := range cols {
		if i != ai && i != vi {
			keep = append(keep, c)
		}
	}
	out := append([]Column{}, keep...)
	sel := []string{}
	if len(keep) > 0 {
		sel = append(sel, selectAll(keep))
	}
	v := cols[vi]
	for _, val := range values {
		name := unique(out, display(val, cols[ai].Type))
		when := "CASE WHEN " + qi(cols[ai].Name) + " = " + lit(val) + " THEN " + qi(v.Name) + " END"
		var e string
		var t grid.Type
		switch op {
		case "countv":
			e, t = "count("+when+")", grid.Integer
		default:
			agg, at, err := aggregate([]Column{{Name: "__v", Type: v.Type}}, op, "__v")
			if err != nil {
				return "", nil, err
			}
			e, t = strings.Replace(agg, qi("__v"), when, 1), at
		}
		sel = append(sel, e+" AS "+qi(name))
		out = append(out, Column{Name: name, Type: t})
	}
	if len(values) == 0 {
		return "SELECT " + selectAll(keep) + " FROM " + prev + " GROUP BY " + selectAll(keep), keep, nil
	}
	q := "SELECT " + strings.Join(sel, ", ") + " FROM " + prev
	if len(keep) > 0 {
		q += " GROUP BY " + selectAll(keep) + " ORDER BY " + selectAll(keep)
	}
	return q, out, nil
}
