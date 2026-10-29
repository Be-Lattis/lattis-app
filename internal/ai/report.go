package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/mcp"
)

// Bilans rendus au modèle après une écriture, et erreurs qui aident à se
// corriger. Une écriture rend « ok » (ou ce qu'elle a provoqué : tableau
// créé, colonne élargie…), sans les champs vides ; quand elle a posé une
// formule (cells_set, cells_paste), Lattis vérifie aussitôt les cellules et
// les colonnes calculées touchées et joint leurs erreurs par code, la
// première cellule en erreur et la cause probable (fonction, tableau ou
// colonne inconnus, avec les noms proches) : le modèle se corrige sans
// devoir penser à appeler formulas_errors. Aucune valeur n'est rendue :
// des codes d'erreur, des comptes et des positions.

// compactResult : le résultat d'une écriture sans ses champs vides ; « ok »
// s'il ne reste rien.
func compactResult(res any) string {
	s := encode(res)
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "{") {
		return s
	}
	var m map[string]any
	if json.Unmarshal([]byte(t), &m) != nil {
		return s
	}
	for k, v := range m {
		switch x := v.(type) {
		case nil:
			delete(m, k)
		case string:
			if x == "" {
				delete(m, k)
			}
		case bool:
			if !x {
				delete(m, k)
			}
		case float64:
			if x == 0 {
				delete(m, k)
			}
		case []any:
			if len(x) == 0 {
				delete(m, k)
			}
		case map[string]any:
			if len(x) == 0 {
				delete(m, k)
			}
		}
	}
	if len(m) == 0 {
		return "ok"
	}
	return encode(m)
}

// hasFormula dit si les paramètres d'un outil contiennent une formule.
func hasFormula(v any) bool {
	switch x := v.(type) {
	case string:
		return strings.HasPrefix(strings.TrimSpace(x), "=")
	case map[string]any:
		for k, e := range x {
			if !rawKeys[k] && hasFormula(e) {
				return true
			}
		}
	case []any:
		for _, e := range x {
			if hasFormula(e) {
				return true
			}
		}
	}
	return false
}

// MaxReportCells borne les cellules libres en erreur citées dans un bilan.
const MaxReportCells = 5

// formulaCheck vérifie les formules qu'une saisie (cells.set ou
// cells.paste) vient de poser : colonnes calculées des tableaux touchés,
// cellules libres. Vide si rien n'est à vérifier.
func formulaCheck(ctx context.Context, host api.Host, cmd string, args map[string]any, loc formula.Locale) string {
	if cmd != "cells.set" && cmd != "cells.paste" {
		return ""
	}
	row, _ := args["row"].(float64)
	col, _ := args["col"].(float64)
	if row < 1 || col < 1 {
		return ""
	}
	// Cellules écrites avec une formule (rangs de la feuille, à partir de 1).
	type cell struct{ r, c int }
	var written []cell
	if cmd == "cells.set" {
		if hasFormula(args["value"]) {
			written = append(written, cell{int(row), int(col)})
		}
	} else if rows, ok := args["values"].([]any); ok {
		for i, r := range rows {
			if line, ok := r.([]any); ok {
				for j, v := range line {
					if hasFormula(v) {
						written = append(written, cell{int(row) + i, int(col) + j})
					}
				}
			}
		}
	}
	if len(written) == 0 {
		return ""
	}
	q := map[string]any{}
	if s, ok := args["sheet"]; ok {
		q["sheet"] = s
	}
	v, err := api.Invoke(ctx, host, "sheets.get", q)
	if err != nil {
		return ""
	}
	view, ok := v.(api.SheetView)
	if !ok {
		return ""
	}
	names := knownNames(ctx, host)
	var out []string
	seen := map[string]bool{}
	var free []string
	freeOK := 0
	for _, w := range written {
		inTable := false
		for _, t := range view.Tables {
			first := t.Row + 1
			if t.TwoLevel {
				first++
			}
			last := t.Row + t.Rows + t.Hidden
			if t.TwoLevel {
				last++
			}
			k := w.c - t.Col
			if w.r < t.Row || w.r > last || k < 0 || k >= len(t.Columns) {
				continue
			}
			inTable = true
			c := t.Columns[k]
			if seen[t.Name+"\x00"+c.Name] {
				break
			}
			seen[t.Name+"\x00"+c.Name] = true
			if c.Formula == "" {
				// Collage qui crée un tableau : les formules y restent du
				// texte.
				out = append(out, fmt.Sprintf("la colonne « %s » de %s garde la formule comme du texte : pour une colonne calculée, cells_set de la formule dans sa première ligne de données (%s%d)", c.Name, t.Name, formula.ColumnLetters(t.Col-1+k), first))
				break
			}
			out = append(out, columnCheck(ctx, host, t, k, first, names, loc))
			break
		}
		if inTable {
			continue
		}
		for _, c := range view.Cells {
			if c.Row != w.r || c.Col != w.c {
				continue
			}
			addr := formula.ColumnLetters(c.Col-1) + fmt.Sprint(c.Row)
			if c.Error || strings.HasPrefix(c.Value, "#") {
				if len(free) < MaxReportCells {
					line := "cellule " + addr + " : " + formula.ErrorText(c.Value, loc)
					if why := diagnose(c.Raw, "", names, loc); why != "" {
						line += " (" + why + ")"
					}
					free = append(free, line)
				}
			} else {
				freeOK++
			}
		}
	}
	out = append(out, free...)
	if len(free) == 0 && freeOK > 0 {
		out = append(out, fmt.Sprintf("%d cellule(s) avec formule, sans erreur", freeOK))
	}
	if len(out) == 0 {
		return ""
	}
	return "Vérification : " + strings.Join(out, " ; ")
}

// columnCheck : le bilan d'une colonne calculée (rang k du tableau t).
func columnCheck(ctx context.Context, host api.Host, t api.Table, k, first int, names nameSet, loc formula.Locale) string {
	c := t.Columns[k]
	head := fmt.Sprintf("colonne calculée « %s » %s", c.Name, formulaOut(c.Formula, loc))
	st, err := resultStats(ctx, host, t.Name, c.Name)
	if err != nil {
		return head + " : vérification impossible (" + err.Error() + ")"
	}
	errs := errorSummary(st.errors, loc)
	if errs == "" {
		res := ""
		if st.result != "" {
			res = " (résultat : " + st.result + ")"
		}
		return head + " : aucune erreur" + res
	}
	line := fmt.Sprintf("%s : %s sur %d ligne(s)", head, errs, t.Rows+t.Hidden)
	if n := firstErrorRow(ctx, host, t.Name, c.Name); n > 0 {
		line += fmt.Sprintf(", la première en %s%d", formula.ColumnLetters(t.Col-1+k), first+n-1)
	}
	if why := diagnose(c.Formula, t.Name, names, loc); why != "" {
		line += " ; " + why
	}
	return line + " ; corrigez la formule"
}

// firstErrorRow : le rang (à partir de 1, dans l'ordre d'affichage) de la
// première ligne en erreur d'une colonne calculée ; 0 si inconnu.
func firstErrorRow(ctx context.Context, host api.Host, table, column string) int {
	q := quoteIdent(column)
	isErr := "typeof(v) = 'blob' AND substr(hex(v), 1, 4) = '" + blobError + "'"
	for _, order := range []string{"ORDER BY _pos", ""} {
		sql := "SELECT n FROM (SELECT ROW_NUMBER() OVER (" + order + ") AS n, " + q + " AS v FROM " + quoteIdent(table) + ") WHERE " + isErr + " LIMIT 1"
		res, err := api.Invoke(ctx, host, "db.query", map[string]any{"sql": sql})
		if err != nil {
			continue
		}
		b, _ := json.Marshal(res)
		var r struct {
			Rows [][]any `json:"rows"`
		}
		if json.Unmarshal(b, &r) == nil && len(r.Rows) == 1 && len(r.Rows[0]) == 1 {
			if n, ok := r.Rows[0][0].(float64); ok {
				return int(n)
			}
		}
		return 0
	}
	return 0
}

// nameSet : les noms du classeur, pour les suggestions.
type nameSet struct {
	tables  map[string][]string // tableau (minuscules) → colonnes
	display map[string]string   // tableau (minuscules) → nom affiché
	sheets  []string
}

func knownNames(ctx context.Context, host api.Host) nameSet {
	n := nameSet{tables: map[string][]string{}, display: map[string]string{}}
	if res, err := api.Invoke(ctx, host, "tables.list", nil); err == nil {
		if l, ok := res.([]api.TableEntry); ok {
			for _, t := range l {
				n.tables[strings.ToLower(t.Name)] = t.Columns
				n.display[strings.ToLower(t.Name)] = t.Name
			}
		}
	}
	if res, err := api.Invoke(ctx, host, "sheets.list", nil); err == nil {
		var l []digestSheet
		if b, err := json.Marshal(res); err == nil && json.Unmarshal(b, &l) == nil {
			for _, s := range l {
				n.sheets = append(n.sheets, s.Name)
			}
		}
	}
	return n
}

func (n nameSet) tableNames() []string {
	out := make([]string, 0, len(n.display))
	for _, d := range n.display {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// diagnose : la cause probable d'une formule en erreur (gardée en
// français), en cherchant les fonctions, tableaux et colonnes inconnus ;
// table est le tableau de la formule (vide pour une cellule libre).
func diagnose(src, table string, names nameSet, loc formula.Locale) string {
	n, err := formula.Parse(src, formula.FR)
	if err != nil {
		return ""
	}
	var why []string
	seen := map[string]bool{}
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			why = append(why, s)
		}
	}
	checkRef := func(r formula.Ref) {
		t := r.Table
		if t == "" {
			t = table
		}
		if t == "" {
			return
		}
		cols, ok := names.tables[strings.ToLower(t)]
		if !ok {
			add("tableau inconnu « " + t + " »" + closeTo(t, names.tableNames()))
			return
		}
		if r.Col == "" {
			return
		}
		for _, c := range cols {
			if strings.EqualFold(c, r.Col) {
				return
			}
		}
		add("colonne inconnue [" + r.Col + "] dans " + names.display[strings.ToLower(t)] + closeTo(r.Col, cols))
	}
	var walk func(formula.Node)
	walk = func(n formula.Node) {
		switch x := n.(type) {
		case formula.Ref:
			checkRef(x)
		case formula.RowRange:
			checkRef(x.Ref)
		case formula.Unary:
			walk(x.X)
		case formula.Binary:
			walk(x.L)
			walk(x.R)
		case formula.Call:
			if !formula.Known(x.Fn) && !formula.BareFunction(x.Fn) {
				var fns []string
				for _, f := range formula.Functions(loc) {
					fns = append(fns, f.Name)
				}
				add("fonction inconnue " + x.Fn + closeTo(x.Fn, fns) + ", à chercher avec formulas_functions")
			}
			for _, a := range x.Args {
				walk(a)
			}
		}
	}
	walk(n)
	return strings.Join(why, " ; ")
}

// closeTo : « (proche : A, B) » pour les noms de list proches de name.
func closeTo(name string, list []string) string {
	if c := closest(name, list, 3); len(c) > 0 {
		return " (proche : " + strings.Join(c, ", ") + ")"
	}
	return ""
}

// closest : au plus max noms de list proches de name (sans tenir compte
// des majuscules ni des accents), du plus proche au moins proche.
func closest(name string, list []string, max int) []string {
	q := formula.Fold(name)
	if q == "" {
		return nil
	}
	type hit struct {
		s string
		d int
	}
	var hits []hit
	seen := map[string]bool{}
	for _, s := range list {
		f := formula.Fold(s)
		if seen[f] || f == q {
			continue
		}
		seen[f] = true
		d := levenshtein(q, f)
		limit := max3(2, len([]rune(q))/3)
		if d <= limit || (len(q) >= 3 && (strings.Contains(f, q) || strings.Contains(q, f) && len(f) >= 3)) {
			hits = append(hits, hit{s, d})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].d < hits[j].d })
	var out []string
	for _, h := range hits {
		if len(out) == max {
			break
		}
		out = append(out, h.s)
	}
	return out
}

func max3(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// toolNamer réécrit les noms des commandes (tables.set_key) dans la forme
// des outils que voit le modèle (tables_set_key).
var toolNamer = func() *strings.Replacer {
	var names []string
	for _, c := range api.Commands() {
		if strings.Contains(c.Name, ".") {
			names = append(names, c.Name)
		}
	}
	// Les plus longs d'abord : tables.set_key avant tables.set.
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	var pairs []string
	for _, n := range names {
		pairs = append(pairs, n, mcp.ToolName(n))
	}
	return strings.NewReplacer(pairs...)
}()

// explain : le texte d'une erreur d'outil pour le modèle, avec ce qui aide
// à la corriger : l'écriture des formules pour une formule illisible, les
// noms proches d'un tableau, d'une colonne ou d'une feuille inconnus, les
// noms d'outils tels qu'il les voit.
func explain(ctx context.Context, host api.Host, err error, loc formula.Locale) string {
	text := toolNamer.Replace(err.Error())
	var se *formula.SyntaxError
	var fe canvas.ErrFormula
	if errors.As(err, &se) || (errors.As(err, &fe) && strings.Contains(text, "position ")) {
		text += " — " + syntaxHint(loc)
	}
	if s := suggestNames(ctx, host, text); s != "" {
		text += " — " + s
	}
	return text
}

// suggestNames : pour chaque nom entre « » d'un message d'erreur qui n'est
// ni un tableau, ni une colonne, ni une feuille, les noms proches ; à
// défaut, la liste des tableaux quand le message parle d'un tableau.
func suggestNames(ctx context.Context, host api.Host, text string) string {
	quoted := quotedNames(text)
	if len(quoted) == 0 {
		return ""
	}
	names := knownNames(ctx, host)
	var all []string
	all = append(all, names.tableNames()...)
	all = append(all, names.sheets...)
	for _, cols := range names.tables {
		all = append(all, cols...)
	}
	known := map[string]bool{}
	for _, s := range all {
		known[strings.ToLower(s)] = true
	}
	var out []string
	for _, q := range quoted {
		if known[strings.ToLower(q)] {
			continue
		}
		if c := closest(q, all, 3); len(c) > 0 {
			out = append(out, "« "+q+" » : vouliez-vous "+strings.Join(c, ", ")+" ?")
		} else if strings.Contains(strings.ToLower(text), "tableau") {
			list := names.tableNames()
			if len(list) > 15 {
				list = append(list[:15], "…")
			}
			if len(list) > 0 {
				out = append(out, "tableaux du classeur : "+strings.Join(list, ", "))
			}
		}
	}
	return strings.Join(out, " ; ")
}

// quotedNames : les textes entre « et » d'un message.
func quotedNames(s string) []string {
	var out []string
	for {
		i := strings.Index(s, "«")
		if i < 0 {
			return out
		}
		s = s[i+len("«"):]
		j := strings.Index(s, "»")
		if j < 0 {
			return out
		}
		if q := strings.TrimSpace(s[:j]); q != "" && len(q) < 80 {
			out = append(out, q)
		}
		s = s[j+len("»"):]
	}
}
