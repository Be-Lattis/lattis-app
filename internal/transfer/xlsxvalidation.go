package transfer

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// Validations des données d'un fichier .xlsx (<dataValidation>) et
// propriété dv de Lattis (internal/canvas/dvrules.go), dans les deux sens.
// La lecture (validationFromXlsx) est appelée par l'import ; l'écriture
// (sheetValidationsXML) par l'export, juste avant les liens.

// XlsxValidation est un élément <dataValidation> lu : attributs et
// formules telles qu'écrites dans le fichier (anglais, adresses A1, sans
// « = »), TopLeft : la première cellule de sqref (formules relatives à
// elle).
type XlsxValidation struct {
	Type, Operator                   string
	AllowBlank, ShowInput, ShowError bool
	ErrorStyle, ErrorTitle, Error    string
	PromptTitle, Prompt              string
	Formula1, Formula2               string
	TopLeft                          string
}

var xlsxDVTypes = map[string]string{"whole": "whole", "decimal": "decimal", "list": "list", "date": "date", "time": "time", "textLength": "length", "custom": "custom"}

var xlsxDVOps = map[string]string{"between": "between", "notBetween": "notbetween", "equal": "=", "notEqual": "<>",
	"greaterThan": ">", "lessThan": "<", "greaterThanOrEqual": ">=", "lessThanOrEqual": "<="}

// validationFromXlsx traduit un <dataValidation> en propriété dv (forme
// gardée : formules en français, valeurs « 3,5 », « 19/10/2026 ») ; faux
// quand rien n'est à garder (type « any » sans message, formule illisible).
func validationFromXlsx(v XlsxValidation) (map[string]any, bool) {
	typ := xlsxDVTypes[v.Type]
	if v.Type == "" || v.Type == "none" {
		typ = "any"
	}
	if typ == "" {
		return nil, false
	}
	dv := map[string]any{"type": typ}
	if !v.AllowBlank {
		dv["blank"] = false
	}
	if v.ShowInput && (v.PromptTitle != "" || v.Prompt != "") {
		dv["input"] = map[string]any{"title": v.PromptTitle, "msg": v.Prompt}
	}
	if v.ShowError && (v.ErrorStyle == "warning" || v.ErrorStyle == "information" || v.ErrorTitle != "" || v.Error != "") {
		e := map[string]any{"title": v.ErrorTitle, "msg": v.Error}
		switch v.ErrorStyle {
		case "warning":
			e["style"] = "warning"
		case "information":
			e["style"] = "info"
		}
		dv["error"] = e
	}
	if !v.ShowError {
		// Sans alerte, un tableur laisse tout passer : on garde seulement le
		// message de saisie.
		if dv["input"] == nil {
			return nil, false
		}
		return map[string]any{"type": "any", "input": dv["input"]}, true
	}
	relative := false
	switch typ {
	case "any":
		if dv["input"] == nil {
			return nil, false
		}
		delete(dv, "blank")
		delete(dv, "error")
		return dv, true
	case "list":
		f := strings.TrimSpace(v.Formula1)
		if strings.HasPrefix(f, `"`) && strings.HasSuffix(f, `"`) && len(f) >= 2 {
			var list []any
			for _, p := range strings.Split(strings.ReplaceAll(f[1:len(f)-1], `""`, `"`), ",") {
				if p = strings.TrimSpace(p); p != "" {
					list = append(list, p)
				}
			}
			if len(list) == 0 {
				return nil, false
			}
			dv["list"] = list
		} else {
			src, err := canvas.NormalizeDVSource(strings.ReplaceAll(f, "$", ""))
			if err != nil {
				return nil, false
			}
			dv["src"] = src
		}
	case "custom":
		f, ok := xlsxFormulaIn(v.Formula1)
		if !ok {
			return nil, false
		}
		dv["formula"], relative = f, true
	default:
		op := xlsxDVOps[v.Operator]
		if op == "" {
			op = "between"
		}
		dv["op"] = op
		bounds := []string{v.Formula1}
		if op == "between" || op == "notbetween" {
			bounds = append(bounds, v.Formula2)
		}
		for i, b := range bounds {
			s, isF, ok := xlsxBoundIn(typ, b)
			if !ok {
				return nil, false
			}
			relative = relative || isF
			dv[[]string{"min", "max"}[i]] = s
		}
	}
	if relative && v.TopLeft != "" {
		dv["at"] = strings.ReplaceAll(v.TopLeft, "$", "")
	}
	return dv, true
}

// xlsxFormulaIn : une formule du fichier (anglais, sans « = ») en français.
func xlsxFormulaIn(f string) (string, bool) {
	f = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(f), "="))
	if f == "" {
		return "", false
	}
	out, err := canvas.TranslateA1("="+f, formula.EN, formula.FR)
	return out, err == nil
}

// xlsxBoundIn : une borne du fichier, constante (nombre, numéro de série
// d'une date ou d'une heure) ou formule.
func xlsxBoundIn(typ, f string) (string, bool, bool) {
	f = strings.TrimSpace(f)
	n, err := strconv.ParseFloat(f, 64)
	if err != nil {
		s, ok := xlsxFormulaIn(f)
		return s, true, ok
	}
	switch typ {
	case "date":
		return formula.DateSerial(math.Floor(n)).String(), false, true
	case "time":
		s := int(math.Round((n - math.Floor(n)) * 86400))
		out := fmt.Sprintf("%02d:%02d", s/3600, s/60%60)
		if s%60 != 0 {
			out += fmt.Sprintf(":%02d", s%60)
		}
		return out, false, true
	}
	return formula.FormatNumber(n), false, true
}

// validationToXlsx écrit la propriété dv en <dataValidation> pour les
// cellules sqref, dont la première est (row, col) ; faux si elle ne
// s'écrit pas (formule sans équivalent).
func validationToXlsx(dv any, sqref string, row, col int) (string, bool) {
	r, ok := canvas.ParseDV(dv)
	if !ok {
		return "", false
	}
	var b strings.Builder
	typ := r.Type
	for k, v := range xlsxDVTypes {
		if v == r.Type {
			typ = k
		}
	}
	if r.Type == "any" {
		typ = "none"
	}
	fmt.Fprintf(&b, `<dataValidation type="%s"`, typ)
	if op := r.Op; op != "" && op != "between" {
		for k, v := range xlsxDVOps {
			if v == op {
				fmt.Fprintf(&b, ` operator="%s"`, k)
			}
		}
	}
	if s := r.Error; s != nil && s.Style != "" {
		fmt.Fprintf(&b, ` errorStyle="%s"`, map[string]string{"warning": "warning", "info": "information"}[s.Style])
	}
	if r.Blank == nil || *r.Blank {
		b.WriteString(` allowBlank="1"`)
	}
	if r.Type == "list" && r.Src == "" && len(r.List) == 0 {
		return "", false
	}
	if r.Input != nil {
		fmt.Fprintf(&b, ` showInputMessage="1" promptTitle="%s" prompt="%s"`, xmlEscape(truncateRunes(r.Input.Title, 32)), xmlEscape(truncateRunes(r.Input.Msg, 255)))
	}
	if r.Type != "any" {
		b.WriteString(` showErrorMessage="1"`)
		if e := r.Error; e != nil {
			fmt.Fprintf(&b, ` errorTitle="%s" error="%s"`, xmlEscape(truncateRunes(e.Title, 32)), xmlEscape(truncateRunes(e.Msg, 255)))
		}
	}
	fmt.Fprintf(&b, ` sqref="%s">`, sqref)
	formulaOut := func(src string) (string, bool) {
		if !strings.HasPrefix(src, "=") {
			return xlsxConstOut(r.Type, src)
		}
		if _, ar, ac, ok := formula.A1Address(r.At); ok {
			src = canvas.ShiftFormula(src, row-ar, col-ac)
		}
		out, err := canvas.TranslateA1(src, formula.FR, formula.EN)
		if err != nil || strings.Contains(out, "[") {
			return "", false // références structurées : sans équivalent
		}
		return (&book{target: toXLSX}).separators(strings.TrimPrefix(out, "=")), true
	}
	var fs []string
	switch r.Type {
	case "list":
		if r.Src != "" {
			rg, ok := xlsxRange(r.Src)
			if !ok {
				return "", false
			}
			fs = []string{rg}
		} else {
			fs = []string{`"` + strings.ReplaceAll(strings.Join(r.List, ","), `"`, `""`) + `"`}
		}
	case "custom":
		fs = []string{r.Formula}
	case "any":
	default:
		fs = []string{r.Min}
		if r.Op == "between" || r.Op == "notbetween" {
			fs = append(fs, r.Max)
		}
	}
	for i, f := range fs {
		if r.Type != "list" {
			var ok bool
			if f, ok = formulaOut(f); !ok {
				return "", false
			}
		}
		fmt.Fprintf(&b, `<formula%d>%s</formula%d>`, i+1, xmlEscape(f), i+1)
	}
	b.WriteString(`</dataValidation>`)
	return b.String(), true
}

// xlsxConstOut : une borne constante (forme gardée) en nombre du fichier.
func xlsxConstOut(typ, s string) (string, bool) {
	s = strings.TrimSpace(s)
	if typ == "time" {
		if m := regexp.MustCompile(`^(\d{1,2}):(\d{2})(?::(\d{2}))?$`).FindStringSubmatch(s); m != nil {
			h, _ := strconv.Atoi(m[1])
			mn, _ := strconv.Atoi(m[2])
			sec, _ := strconv.Atoi(m[3] + "0")
			return strconv.FormatFloat(float64(h*3600+mn*60+sec/10)/86400, 'f', -1, 64), true
		}
	}
	if d, _, ok := grid.ParseDateTime(s); ok && strings.ContainsAny(s, "/-") {
		return strconv.FormatFloat(math.Floor(formula.TimeSerial(d)), 'f', -1, 64), true
	}
	v, err := grid.Parse(grid.Decimal, s)
	if err != nil || v == nil {
		return "", false
	}
	return strconv.FormatFloat(v.(float64), 'f', -1, 64), true
}

// xlsxRange : la source d'une liste (Cell[A1:A10], Feuille!Cell[A1:A10])
// en plage du fichier ; une colonne de tableau ne s'écrit pas ici.
func xlsxRange(src string) (string, bool) {
	n, err := formula.Parse(src, formula.FR)
	if err != nil {
		return "", false
	}
	rg, ok := n.(formula.CellRange)
	if !ok {
		return "", false
	}
	out := fmt.Sprintf("$%s$%d:$%s$%d", colName(rg.C1), rg.R1+1, colName(rg.C2), rg.R2+1)
	if rg.Sheet != "" {
		out = "'" + strings.ReplaceAll(rg.Sheet, "'", "''") + "'!" + out
	}
	return out, true
}

// sheetValidationsXML écrit les validations d'une feuille (cellules libres,
// lignes et colonnes entières, colonnes de tableau) : une entrée par
// validation, ses plages dans sqref ; skipped compte celles qui ne
// s'écrivent pas.
func sheetValidationsXML(l canvas.Layout) (xml string, skipped int) {
	type group struct {
		dv     any
		ranges []string
		r, c   int
	}
	groups := map[string]*group{}
	var order []string
	add := func(style string, ref string, r, c int) {
		if !strings.Contains(style, `"dv"`) {
			return
		}
		var st map[string]any
		if json.Unmarshal([]byte(style), &st) != nil || st["dv"] == nil {
			return
		}
		k, _ := json.Marshal(st["dv"])
		g := groups[string(k)]
		if g == nil {
			g = &group{dv: st["dv"], r: r, c: c}
			groups[string(k)] = g
			order = append(order, string(k))
		}
		g.ranges = append(g.ranges, ref)
	}
	for _, s := range l.Styles {
		if s.Row < ExcelRows && s.Col < ExcelCols {
			add(s.Style, colName(s.Col)+strconv.Itoa(s.Row+1), s.Row, s.Col)
		}
	}
	for _, s := range l.RowStyles {
		if s.Index < ExcelRows {
			add(s.Style, fmt.Sprintf("A%d:XFD%d", s.Index+1, s.Index+1), s.Index, 0)
		}
	}
	for _, s := range l.ColStyles {
		if s.Index < ExcelCols {
			add(s.Style, fmt.Sprintf("%s1:%s1048576", colName(s.Index), colName(s.Index)), 0, s.Index)
		}
	}
	for _, tb := range l.Tables {
		last := min(tb.Row+max(tb.Total, 1), ExcelRows-1)
		for k, c := range tb.Columns {
			col := tb.Col + k
			if col >= ExcelCols || tb.Row+1 >= ExcelRows {
				continue
			}
			// [@Colonne] : la cellule de la première ligne de données.
			style := c.Style
			if strings.Contains(style, "[@") {
				for j, o := range tb.Columns {
					a := colName(tb.Col+j) + strconv.Itoa(tb.Row+2)
					style = strings.ReplaceAll(style, "[@"+o.Name+"]", a)
				}
				if strings.Contains(style, `"dv"`) && !strings.Contains(style, `"at"`) {
					style = strings.Replace(style, `"dv":{`, `"dv":{"at":"`+colName(col)+strconv.Itoa(tb.Row+2)+`",`, 1)
				}
			}
			add(style, fmt.Sprintf("%s%d:%s%d", colName(col), tb.Row+2, colName(col), last+1), tb.Row+1, col)
		}
	}
	var out []string
	for _, k := range order {
		g := groups[k]
		if x, ok := validationToXlsx(g.dv, strings.Join(g.ranges, " "), g.r, g.c); ok {
			out = append(out, x)
		} else {
			skipped++
		}
	}
	if len(out) == 0 {
		return "", skipped
	}
	return fmt.Sprintf(`<dataValidations count="%d">%s</dataValidations>`, len(out), strings.Join(out, "")), skipped
}
