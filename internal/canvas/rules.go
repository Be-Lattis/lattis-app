package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
)

// Mise en forme conditionnelle, comme dans Excel. Les règles se posent à
// trois endroits (même JSON partout, voir frontend/src/lib/grid/rules.ts) :
//
//   - sur une colonne de tableau (_lattis_columns.rules), pour toutes ses
//     lignes, présentes et à venir ;
//   - sur des lignes ou des colonnes entières de la feuille (linerules.go) ;
//   - sur des cellules libres, dans leur couche de style (propriété « cf »,
//     comme les notes et les validations) : elles suivent ainsi l'insertion
//     de lignes, le déplacement et la copie des cellules, l'annulation.
//
// Une règle porte un identifiant (id) : posée sur plusieurs colonnes, lignes
// ou cellules, c'est la même règle (« S'applique à » d'Excel), dont les
// statistiques (10 premiers, moyenne, barres, nuances…) se calculent sur
// l'ensemble. La liste d'un emplacement est dans l'ordre de priorité.

// Rule est une règle de mise en forme conditionnelle.
//
//	op : ">", ">=", "<", "<=", "=", "<>", "between", "notbetween",
//	     "contains", "notcontains", "begins", "ends", "empty", "notempty",
//	     "error" (saisie hors type ou erreur de formule), "noerror",
//	     "date" (Value : yesterday, today, tomorrow, last7, lastweek,
//	     thisweek, nextweek, lastmonth, thismonth, nextmonth),
//	     "dup", "unique", "top", "bottom" (Value : N, Pct : N %),
//	     "above", "below" (Value : "", "=" ou écart type "1", "2", "3"),
//	     "formula" (Value : formule), "bar", "scale", "icons"
type Rule struct {
	ID    string `json:"id,omitempty"`
	Op    string `json:"op"`
	Value string `json:"value,omitempty"`
	To    string `json:"to,omitempty"` // borne haute de "between"
	Pct   bool   `json:"pct,omitempty"`
	// Stop : « Interrompre si vrai », les règles suivantes ne s'appliquent
	// plus à la cellule.
	Stop  bool  `json:"stop,omitempty"`
	Style Style `json:"style"`
	// Barres de données, nuances de couleurs, jeux d'icônes : réglages lus
	// par l'interface seulement.
	Bar   map[string]any   `json:"bar,omitempty"`
	Scale []map[string]any `json:"scale,omitempty"`
	Icons map[string]any   `json:"icons,omitempty"`
	// F : formule d'une règle "formula" d'une colonne de tableau, analysée
	// (anglais, références structurées), pour le calcul par ligne en SQL.
	F string `json:"f,omitempty"`
}

var ruleOps = map[string]bool{">": true, ">=": true, "<": true, "<=": true, "=": true, "<>": true,
	"between": true, "notbetween": true, "contains": true, "notcontains": true, "begins": true, "ends": true,
	"empty": true, "notempty": true, "error": true, "noerror": true, "date": true, "dup": true, "unique": true,
	"top": true, "bottom": true, "above": true, "below": true, "formula": true, "bar": true, "scale": true, "icons": true}

var dateWhens = map[string]bool{"yesterday": true, "today": true, "tomorrow": true, "last7": true, "lastweek": true,
	"thisweek": true, "nextweek": true, "lastmonth": true, "thismonth": true, "nextmonth": true}

// checkRules vérifie une liste de règles.
func checkRules(rules []Rule) error {
	for _, r := range rules {
		if !ruleOps[r.Op] {
			return fmt.Errorf("règle : condition %q inconnue", r.Op)
		}
		switch r.Op {
		case "date":
			if !dateWhens[r.Value] {
				return fmt.Errorf("règle : période %q inconnue", r.Value)
			}
		case "top", "bottom":
			n, err := strconv.Atoi(strings.TrimSpace(r.Value))
			if err != nil || n < 1 || n > 1000 || (r.Pct && n > 100) {
				return fmt.Errorf("règle : rang %q invalide (1 à %d)", r.Value, map[bool]int{false: 1000, true: 100}[r.Pct])
			}
		case "scale":
			if len(r.Scale) < 2 || len(r.Scale) > 3 {
				return errors.New("règle : une nuance de couleurs a deux ou trois couleurs")
			}
		case "bar":
			if r.Bar == nil {
				return errors.New("règle : barre de données sans réglages")
			}
		case "icons":
			if r.Icons == nil {
				return errors.New("règle : jeu d'icônes sans réglages")
			}
		case "formula":
			src := strings.TrimSpace(r.Value)
			if src == "" {
				return errors.New("règle : formule vide")
			}
			if !strings.HasPrefix(src, "=") {
				src = "=" + src
			}
			if _, err := formula.Parse(src, formula.FR); err != nil {
				return ErrFormula{err.Error()}
			}
		}
	}
	return nil
}

// encodeRules encode une liste de règles (nil : aucune).
func encodeRules(rules []Rule) (any, error) {
	if len(rules) == 0 {
		return nil, nil
	}
	for i := range rules {
		if rules[i].Style == nil {
			rules[i].Style = Style{}
		}
	}
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false) // garder « > » lisible
	if err := enc.Encode(rules); err != nil {
		return nil, err
	}
	return strings.TrimSpace(b.String()), nil
}

// SetRules remplace les règles de mise en forme conditionnelle de la colonne
// i (liste vide : aucune). La première règle a la priorité la plus haute.
func (m *Model) SetRules(ctx context.Context, table string, i int, rules []Rule) error {
	if err := checkRules(rules); err != nil {
		return err
	}
	return m.Do(ctx, "Mise en forme conditionnelle", func(t *tx) error {
		tb, err := t.viewTable(table)
		if err != nil {
			return err
		}
		if err := t.guardFormat(tb.Sheet); err != nil {
			return err
		}
		return t.setColumnRules(tb, i, rules)
	})
}

// ColumnRules : règles d'une colonne de tableau (SetSheetRules).
type ColumnRules struct {
	Table string `json:"table"`
	Col   int    `json:"col"`
	Rules []Rule `json:"rules"`
}

// LineRuleSet : règles de lignes (Rows) ou de colonnes entières I0 à I1.
type LineRuleSet struct {
	Rows  bool   `json:"rows"`
	I0    int    `json:"i0"`
	I1    int    `json:"i1"`
	Rules []Rule `json:"rules"`
}

// RangeRules : règles des cellules libres d'un rectangle.
type RangeRules struct {
	R0    int    `json:"r0"`
	C0    int    `json:"c0"`
	R1    int    `json:"r1"`
	C1    int    `json:"c1"`
	Rules []Rule `json:"rules"`
}

// RuleChanges regroupe des changements de règles d'une feuille, appliqués
// en une seule étape d'annulation (gestionnaire des règles, Effacer les
// règles, nouvelle règle sur une sélection qui mêle tableaux et cellules).
type RuleChanges struct {
	Columns []ColumnRules `json:"columns"`
	Lines   []LineRuleSet `json:"lines"`
	Cells   []RangeRules  `json:"cells"`
}

// SetSheetRules applique des changements de règles à une feuille : chaque
// emplacement cité reçoit exactement la liste donnée (vide : plus aucune).
func (m *Model) SetSheetRules(ctx context.Context, sheet int64, ch RuleChanges) error {
	for _, c := range ch.Columns {
		if err := checkRules(c.Rules); err != nil {
			return err
		}
	}
	for _, l := range ch.Lines {
		if err := checkRules(l.Rules); err != nil {
			return err
		}
	}
	for _, c := range ch.Cells {
		if err := checkRules(c.Rules); err != nil {
			return err
		}
	}
	return m.Do(ctx, "Mise en forme conditionnelle", func(t *tx) error {
		if err := t.guardFormat(sheet); err != nil {
			return err
		}
		for _, c := range ch.Columns {
			tb, err := t.tableByName(c.Table)
			if err != nil {
				return err
			}
			if tb.Sheet != sheet {
				return fmt.Errorf("%w : le tableau « %s » n'est pas sur cette feuille", ErrTable, tb.Name)
			}
			if err := t.setColumnRules(tb, c.Col, c.Rules); err != nil {
				return err
			}
		}
		for _, l := range ch.Lines {
			if err := t.setLineRules(sheet, l.Rows, l.I0, l.I1, l.Rules); err != nil {
				return err
			}
		}
		if len(ch.Cells) == 0 {
			return nil
		}
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		cells := 0
		for _, c := range ch.Cells {
			r0, r1, c0, c1 := min(c.R0, c.R1), max(c.R0, c.R1), min(c.C0, c.C1), max(c.C0, c.C1)
			if r0 < 0 || c0 < 0 {
				return errors.New("règles hors de la feuille")
			}
			if cells += (r1 - r0 + 1) * (c1 - c0 + 1); cells > MaxStyleCells {
				return fmt.Errorf("mise en forme conditionnelle limitée à %d cellules : sélectionnez des lignes ou des colonnes entières", MaxStyleCells)
			}
			var v any // nil : retire la propriété
			if len(c.Rules) > 0 {
				rs := append([]Rule{}, c.Rules...)
				for i := range rs {
					rs[i].F = ""
					if rs[i].Style == nil {
						rs[i].Style = Style{}
					}
				}
				v = rs
			}
			for r := r0; r <= r1; r++ {
				for col := c0; col <= c1; col++ {
					if inTable(tables, r, col) {
						continue // les cellules de tableau suivent les règles de leur colonne
					}
					if err := t.styleFree(sheet, r, col, Style{"cf": v}, nil); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

// setColumnRules remplace les règles de la colonne i du tableau. Les
// formules sont analysées pour le calcul par ligne (F).
func (t *tx) setColumnRules(tb Table, i int, rules []Rule) error {
	if i < 0 || i >= len(tb.Cols) {
		return fmt.Errorf("colonne %d hors du tableau", i)
	}
	rules = append([]Rule{}, rules...)
	for k := range rules {
		if rules[k].Op != "formula" {
			rules[k].F = ""
			continue
		}
		src := strings.TrimSpace(rules[k].Value)
		if !strings.HasPrefix(src, "=") {
			src = "=" + src
		}
		n, err := t.prepareFormula(tb.Sheet, rowRefs(src, tb))
		if err != nil {
			return err
		}
		rules[k].F = formula.Format(n, formula.EN)
	}
	c := tb.Cols[i]
	var old sql.NullString
	if err := t.row(`SELECT rules FROM _lattis_columns WHERE tbl = ? AND id = ?`, tb.ID, c.ID).Scan(&old); err != nil {
		return err
	}
	neu, err := encodeRules(rules)
	if err != nil {
		return err
	}
	var prev any
	if old.Valid {
		prev = old.String
	}
	if prev == neu {
		return nil
	}
	q := `UPDATE _lattis_columns SET rules = ? WHERE tbl = ? AND id = ?`
	return t.apply([]stmt{st(q, neu, tb.ID, c.ID)}, []stmt{st(q, prev, tb.ID, c.ID)})
}

// columnRules lit les règles d'une colonne (JSON brut, vide s'il n'y en a pas).
func (t *tx) columnRules(tbl, col int64) (string, error) {
	var s sql.NullString
	err := t.row(`SELECT rules FROM _lattis_columns WHERE tbl = ? AND id = ?`, tbl, col).Scan(&s)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return s.String, err
}

// parseRuleList lit une liste de règles JSON (vide ou illisible : aucune).
func parseRuleList(s string) []Rule {
	var rs []Rule
	if s == "" || json.Unmarshal([]byte(s), &rs) != nil {
		return nil
	}
	return rs
}

// --- Adresses A1 des formules de règles ------------------------------------

// a1Token repère une adresse A1 dans une formule (lettres de colonne, $
// éventuels, numéro de ligne) ; scanA1 vérifie ce qui l'entoure.
func scanA1(src string, fn func(colAbs bool, col int, rowAbs bool, row int) (string, bool)) string {
	var b strings.Builder
	n := len(src)
	for i := 0; i < n; {
		ch := src[i]
		// Textes "…", noms de feuille '…' et crochets […] : recopiés tels quels.
		if ch == '"' || ch == '\'' || ch == '[' {
			end := byte(ch)
			if ch == '[' {
				end = ']'
			}
			j := i + 1
			for j < n && src[j] != end {
				j++
			}
			if j < n {
				j++
			}
			b.WriteString(src[i:j])
			i = j
			continue
		}
		if i > 0 && (isWord(src[i-1]) || src[i-1] == '$') {
			b.WriteByte(ch)
			i++
			continue
		}
		j := i
		colAbs := j < n && src[j] == '$'
		if colAbs {
			j++
		}
		k := j
		for k < n && k-j < 3 && isLetter(src[k]) {
			k++
		}
		if k == j || (k < n && isLetter(src[k])) {
			b.WriteByte(ch)
			i++
			continue
		}
		letters := strings.ToUpper(src[j:k])
		rowAbs := k < n && src[k] == '$'
		if rowAbs {
			k++
		}
		d := k
		for d < n && src[d] >= '0' && src[d] <= '9' {
			d++
		}
		if d == k || (d < n && (isWord(src[d]) || src[d] == '(')) {
			b.WriteByte(ch)
			i++
			continue
		}
		col := 0
		for _, c := range letters {
			col = col*26 + int(c-'A'+1)
		}
		row, _ := strconv.Atoi(src[k:d])
		if out, ok := fn(colAbs, col-1, rowAbs, row-1); ok && col <= 16384 && row >= 1 {
			b.WriteString(out)
		} else {
			b.WriteString(src[i:d])
		}
		i = d
	}
	return b.String()
}

func isLetter(c byte) bool { return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') }
func isWord(c byte) bool {
	return isLetter(c) || (c >= '0' && c <= '9') || c == '_' || c == '.' || c >= 0x80
}

// shiftA1 décale de dr lignes et dc colonnes les adresses relatives d'une
// formule (copie de la formule d'une règle vers une autre cellule).
func shiftA1(src string, dr, dc int) string {
	if dr == 0 && dc == 0 {
		return src
	}
	return scanA1(src, func(colAbs bool, col int, rowAbs bool, row int) (string, bool) {
		if !colAbs {
			col += dc
		}
		if !rowAbs {
			row += dr
		}
		if col < 0 || row < 0 {
			return "#REF!", true
		}
		out := ""
		if colAbs {
			out += "$"
		}
		out += formula.ColumnLetters(col)
		if rowAbs {
			out += "$"
		}
		return out + strconv.Itoa(row+1), true
	})
}

// rowRefs écrit [@Colonne] les adresses de la première ligne de données
// d'un tableau dans la formule d'une règle de ce tableau : comme dans Excel,
// la formule est écrite pour la cellule en haut de la plage (=$C2>100) et
// vaut pour chaque ligne.
func rowRefs(src string, tb Table) string {
	return scanA1(src, func(_ bool, col int, rowAbs bool, row int) (string, bool) {
		if rowAbs || row != tb.Row+1 || col < tb.Col || col > tb.Right() {
			return "", false
		}
		return "[@" + tb.Cols[col-tb.Col].Name + "]", true
	})
}

// --- Formules des règles ----------------------------------------------------

// RuleExprs compile les règles « formule » des colonnes d'un tableau, pour la
// lecture par fenêtres : une expression SQL par règle (vraie quand elle
// s'applique), avec la colonne affichée et le rang de la règle. Les
// paramètres sont numérotés à partir de base+1.
func (m *Model) RuleExprs(ctx context.Context, table int64, base int) (exprs []string, args []any, cols, rules []int, err error) {
	err = m.read(ctx, func(t *tx) error {
		tb, err := t.table(table)
		if err != nil {
			return err
		}
		var schema *formula.Schema
		var ft *formula.Table
		var names map[string]formula.Value
		var hoist func(formula.Compiled) formula.Value
		for i, c := range tb.Cols {
			s, err := t.columnRules(tb.ID, c.ID)
			if err != nil {
				return err
			}
			for k, r := range parseRuleList(s) {
				if r.Op != "formula" || r.F == "" {
					continue
				}
				n, err := formula.Parse(r.F, formula.EN)
				if err != nil {
					continue
				}
				if schema == nil {
					if schema, err = t.schema(); err != nil {
						return err
					}
					var ok bool
					if ft, ok = schema.Table(tb.Name); !ok {
						return errors.New("tableau introuvable")
					}
					if names, err = t.nameValuesWith(schema, true); err != nil {
						return err
					}
					hoist, _ = t.hoister()
				}
				cc := formula.CompileHoistedFrom(n, schema, names, ft, tb.sql(), hoist, base+len(args))
				exprs = append(exprs, cc.SQL)
				args = append(args, cc.Args...)
				cols, rules = append(cols, i), append(rules, k)
			}
		}
		return nil
	})
	return exprs, args, cols, rules, err
}

// Truthy dit si le résultat d'une formule de règle est vrai (VRAI ou un
// nombre non nul, comme Excel).
func Truthy(v formula.Value) bool {
	switch v.Kind {
	case formula.Bool:
		return v.Bool
	case formula.Number:
		return v.Num != 0
	}
	return false
}

// RuleMark : la règle Rule (identifiant) s'applique à la cellule libre.
type RuleMark struct {
	Row  int    `json:"row"`
	Col  int    `json:"col"`
	Rule string `json:"rule"`
}

// maxRuleEvals borne le nombre de cellules libres évaluées par les règles
// « formule » d'une feuille à chaque mise en page.
const maxRuleEvals = 5000

// ruleMarks évalue les règles « formule » des cellules libres d'une feuille
// (règles de cellules et de lignes ou colonnes entières) : la formule est
// écrite pour la cellule en haut à gauche de la plage de la règle, ses
// adresses relatives suivent chaque cellule (comme Excel).
func (t *tx) ruleMarks(sheet int64, styles []CellStyle, rowRules, colRules []LineRules, cells []CellInfo, tables []TableInfo) ([]RuleMark, error) {
	type use struct {
		src     string
		r0, c0  int
		targets [][2]int
	}
	uses := map[string]*use{}
	var order []string
	add := func(r Rule, row, col int) {
		if r.Op != "formula" || strings.TrimSpace(r.Value) == "" {
			return
		}
		id := r.ID
		if id == "" {
			id = r.Value
		}
		u := uses[id]
		if u == nil {
			u = &use{src: r.Value, r0: row, c0: col}
			uses[id] = u
			order = append(order, id)
		}
		u.r0, u.c0 = min(u.r0, row), min(u.c0, col)
	}
	for _, s := range styles {
		var st struct {
			CF []Rule `json:"cf"`
		}
		if json.Unmarshal([]byte(s.Style), &st) != nil {
			continue
		}
		for _, r := range st.CF {
			add(r, s.Row, s.Col)
			if u := uses[ruleKey(r)]; u != nil {
				u.targets = append(u.targets, [2]int{s.Row, s.Col})
			}
		}
	}
	// Lignes et colonnes entières : les cellules libres qui ont une valeur.
	for _, q := range []struct {
		lines []LineRules
		rows  bool
	}{{rowRules, true}, {colRules, false}} {
		for _, l := range q.lines {
			for _, r := range parseRuleList(l.Rules) {
				if r.Op != "formula" {
					continue
				}
				if q.rows {
					add(r, l.Index, 0)
				} else {
					add(r, 0, l.Index)
				}
				u := uses[ruleKey(r)]
				if u == nil {
					continue
				}
				for _, c := range cells {
					if (q.rows && c.Row == l.Index) || (!q.rows && c.Col == l.Index) {
						u.targets = append(u.targets, [2]int{c.Row, c.Col})
					}
				}
			}
		}
	}
	if len(uses) == 0 {
		return nil, nil
	}
	schema, err := t.schema()
	if err != nil {
		return nil, err
	}
	names, err := t.nameValuesWith(schema, true)
	if err != nil {
		return nil, err
	}
	hoist, _ := t.hoister()
	loc := t.locator(sheet)
	var out []RuleMark
	evals := 0
	for _, id := range order {
		u := uses[id]
		src := strings.TrimSpace(u.src)
		if !strings.HasPrefix(src, "=") {
			src = "=" + src
		}
		seen := map[[2]int]bool{}
		for _, p := range u.targets {
			if seen[p] || inTableInfo(tables, p[0], p[1]) {
				continue
			}
			seen[p] = true
			if evals++; evals > maxRuleEvals {
				return out, nil
			}
			if t.ruleFormula(sheet, shiftA1(src, p[0]-u.r0, p[1]-u.c0), schema, names, hoist, loc) {
				out = append(out, RuleMark{Row: p[0], Col: p[1], Rule: id})
			}
		}
	}
	return out, nil
}

// ruleKey : identifiant d'une règle « formule » pour ruleMarks.
func ruleKey(r Rule) string {
	if r.ID != "" {
		return r.ID
	}
	return r.Value
}

func inTableInfo(tables []TableInfo, r, c int) bool {
	for _, tb := range tables {
		if r >= tb.Row && r <= tb.Row+tb.Rows && c >= tb.Col && c < tb.Col+len(tb.Columns) {
			return true
		}
	}
	return false
}

// ruleFormula calcule la formule d'une règle pour une cellule libre.
func (t *tx) ruleFormula(sheet int64, src string, schema *formula.Schema, names map[string]formula.Value, hoist func(formula.Compiled) formula.Value, loc *sheetLocator) bool {
	v, _ := cached(t, fmt.Sprintf("cfrule:%d:%s", sheet, src), func() (formula.Value, error) {
		n, err := formula.Parse(src, formula.FR)
		if err != nil {
			return formula.Err(formula.ErrName), nil
		}
		n, _, err = formula.Convert(n, schema, formula.A1Resolver(loc), src)
		if err == nil {
			n, err = formula.ConvertA1(n, loc)
		}
		if err != nil || loc.err != nil {
			loc.err = nil
			return formula.Err(formula.ErrName), nil
		}
		n = formula.QualifyCells(n, sheet)
		c := formula.CompileHoisted(n, schema, names, nil, "", hoist)
		var v any
		if err := t.row(`SELECT `+c.SQL, c.Args...).Scan(&v); err != nil {
			return formula.Err(formula.ErrValue), nil
		}
		return formula.FromResult(v), nil
	})
	return Truthy(v)
}
