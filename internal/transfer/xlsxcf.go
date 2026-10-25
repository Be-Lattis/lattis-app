package transfer

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/formula"
)

// Mise en forme conditionnelle d'un fichier .xlsx, dans les deux sens : les
// règles d'Excel (comparaisons, texte, dates, doublons, 10 premiers,
// moyenne, formules, barres de données, nuances de couleurs, jeux
// d'icônes) deviennent les règles de Lattis (canvas.Rule), posées sur les
// colonnes des tableaux, les lignes ou colonnes entières ou les cellules
// libres que leur plage couvre ; une règle sans équivalent (comparaison à
// une formule, seuil calculé) est comptée comme non importée.

var cfOps = map[string]string{"greaterThan": ">", "greaterThanOrEqual": ">=", "lessThan": "<", "lessThanOrEqual": "<=",
	"equal": "=", "notEqual": "<>", "between": "between", "notBetween": "notbetween"}

var cfPeriods = map[string]string{"today": "today", "yesterday": "yesterday", "tomorrow": "tomorrow", "last7Days": "last7",
	"thisWeek": "thisweek", "lastWeek": "lastweek", "nextWeek": "nextweek", "thisMonth": "thismonth", "lastMonth": "lastmonth", "nextMonth": "nextmonth"}

var cfIcons = map[string]string{"3Arrows": "3arrows", "3ArrowsGray": "3arrowsgray", "3Triangles": "3triangles", "4Arrows": "4arrows",
	"4ArrowsGray": "4arrowsgray", "5Arrows": "5arrows", "5ArrowsGray": "5arrowsgray", "3TrafficLights1": "3traffic",
	"3TrafficLights2": "3trafficrim", "3Signs": "3signs", "4TrafficLights": "4traffic", "4RedToBlack": "4redblack",
	"3Symbols": "3symbols", "3Symbols2": "3symbols2", "3Flags": "3flags", "3Stars": "3stars", "4Rating": "4rating",
	"5Rating": "5rating", "5Quarters": "5quarters", "5Boxes": "5boxes"}

var cfvoKinds = map[string]string{"min": "min", "max": "max", "num": "num", "percent": "pct", "percentile": "pctl", "autoMin": "auto", "autoMax": "auto"}

// cfConst lit une valeur de comparaison : un nombre ou un texte entre
// guillemets ; faux pour une référence ou une formule.
func cfConst(f string) (string, bool) {
	f = strings.TrimSpace(f)
	if _, err := strconv.ParseFloat(f, 64); err == nil {
		return f, true
	}
	if len(f) >= 2 && f[0] == '"' && f[len(f)-1] == '"' {
		return strings.ReplaceAll(f[1:len(f)-1], `""`, `"`), true
	}
	switch strings.ToUpper(f) {
	case "TRUE":
		return "VRAI", true
	case "FALSE":
		return "FAUX", true
	}
	return "", false
}

// cfvo lit un seuil (barre, nuance, icône) ; faux pour un seuil calculé.
func cfvo(m map[string]string) (map[string]any, bool) {
	k, ok := cfvoKinds[m["type"]]
	if !ok {
		if m["type"] != "formula" {
			return nil, false
		}
		k = "num" // une formule constante (=10) se lit comme un nombre
	}
	out := map[string]any{"kind": k}
	if k == "num" || k == "pct" || k == "pctl" {
		v, err := strconv.ParseFloat(strings.TrimPrefix(m["val"], "="), 64)
		if err != nil {
			return nil, false
		}
		out["value"] = v
	}
	return out, true
}

func (x *xlsxFile) hexColor(c xColor) string {
	if h := x.st.color(&c); h != "" {
		return "#" + strings.ToLower(h)
	}
	return ""
}

// cfRule traduit une règle d'Excel ; faux si elle n'a pas d'équivalent.
func (x *xlsxFile) cfRule(r *xCFRule) (canvas.Rule, bool) {
	a := r.attrs
	out := canvas.Rule{ID: "x" + uuid.NewString()[:8], Stop: a["stopIfTrue"] == "1" || a["stopIfTrue"] == "true", Style: canvas.Style{}}
	if id, err := strconv.Atoi(a["dxfId"]); err == nil {
		out.Style = x.st.dxf(id)
	}
	f := func(i int) string {
		if i < len(r.formulas) {
			return r.formulas[i]
		}
		return ""
	}
	switch a["type"] {
	case "cellIs":
		op, ok := cfOps[a["operator"]]
		if !ok {
			return out, false
		}
		out.Op = op
		if out.Value, ok = cfConst(f(0)); !ok {
			return out, false
		}
		if op == "between" || op == "notbetween" {
			if out.To, ok = cfConst(f(1)); !ok {
				return out, false
			}
		}
	case "containsText":
		out.Op, out.Value = "contains", a["text"]
	case "notContainsText":
		out.Op, out.Value = "notcontains", a["text"]
	case "beginsWith":
		out.Op, out.Value = "begins", a["text"]
	case "endsWith":
		out.Op, out.Value = "ends", a["text"]
	case "containsBlanks":
		out.Op = "empty"
	case "notContainsBlanks":
		out.Op = "notempty"
	case "containsErrors":
		out.Op = "error"
	case "notContainsErrors":
		out.Op = "noerror"
	case "timePeriod":
		p, ok := cfPeriods[a["timePeriod"]]
		if !ok {
			return out, false
		}
		out.Op, out.Value = "date", p
	case "duplicateValues":
		out.Op = "dup"
	case "uniqueValues":
		out.Op = "unique"
	case "top10":
		out.Op, out.Value, out.Pct = "top", a["rank"], a["percent"] == "1"
		if a["bottom"] == "1" {
			out.Op = "bottom"
		}
		if out.Value == "" {
			out.Value = "10"
		}
	case "aboveAverage":
		out.Op = "above"
		if a["aboveAverage"] == "0" {
			out.Op = "below"
		}
		switch {
		case a["stdDev"] != "" && a["stdDev"] != "0":
			out.Value = a["stdDev"]
		case a["equalAverage"] == "1":
			out.Value = "="
		}
	case "expression":
		src, ok := translateRule(f(0), formula.EN, formula.FR)
		if !ok {
			return out, false
		}
		out.Op, out.Value = "formula", src
	case "colorScale":
		if len(r.cfvos) < 2 || len(r.cfvos) > 3 || len(r.colors) < len(r.cfvos) {
			return out, false
		}
		out.Op, out.Style = "scale", canvas.Style{}
		for i, v := range r.cfvos {
			p, ok := cfvo(v)
			if !ok {
				return out, false
			}
			p["color"] = x.hexColor(r.colors[i])
			out.Scale = append(out.Scale, p)
		}
	case "dataBar":
		if len(r.cfvos) < 2 || len(r.colors) < 1 {
			return out, false
		}
		lo, ok1 := cfvo(r.cfvos[0])
		hi, ok2 := cfvo(r.cfvos[1])
		if !ok1 || !ok2 {
			return out, false
		}
		out.Op, out.Style = "bar", canvas.Style{}
		out.Bar = map[string]any{"color": x.hexColor(r.colors[0]), "min": lo, "max": hi}
		if a["dataBar.showValue"] == "0" {
			out.Bar["only"] = true
		}
	case "iconSet":
		name := a["iconSet.iconSet"]
		if name == "" {
			name = "3TrafficLights1"
		}
		set, ok := cfIcons[name]
		if !ok || len(r.cfvos) < 2 {
			return out, false
		}
		out.Op, out.Style = "icons", canvas.Style{}
		var pts []any
		for _, v := range r.cfvos[1:] {
			p, ok := cfvo(v)
			if !ok || p["kind"] == "min" || p["kind"] == "max" || p["kind"] == "auto" {
				return out, false
			}
			if v["gte"] == "0" {
				p["gt"] = true
			}
			pts = append(pts, p)
		}
		out.Icons = map[string]any{"set": set, "points": pts}
		if a["iconSet.reverse"] == "1" {
			out.Icons["reverse"] = true
		}
		if a["iconSet.showValue"] == "0" {
			out.Icons["only"] = true
		}
	default:
		return out, false
	}
	return out, true
}

// sqrefParts lit une plage d'Excel (« A1:B5 D2 », « C:C », « 3:4 ») :
// rectangles, colonnes entières, lignes entières.
func sqrefParts(sq string) (rects []rect, cols, rows [][2]int) {
	for _, p := range strings.Fields(strings.ReplaceAll(sq, "$", "")) {
		if a, b, ok := strings.Cut(p, ":"); ok {
			if ca, cb := colIndex(a), colIndex(b); ca >= 0 && cb >= 0 {
				cols = append(cols, [2]int{min(ca, cb), max(ca, cb)})
				continue
			}
			ra, ea := strconv.Atoi(a)
			rb, eb := strconv.Atoi(b)
			if ea == nil && eb == nil && ra > 0 && rb > 0 {
				rows = append(rows, [2]int{min(ra, rb) - 1, max(ra, rb) - 1})
				continue
			}
		}
		r, ok := rangeRect(p)
		if !ok {
			continue
		}
		switch {
		case r.r0 == 0 && r.r1 >= ExcelRows-1:
			cols = append(cols, [2]int{r.c0, r.c1})
		case r.c0 == 0 && r.c1 >= ExcelCols-1:
			rows = append(rows, [2]int{r.r0, r.r1})
		default:
			rects = append(rects, r)
		}
	}
	return
}

// colIndex lit des lettres de colonne seules (« C ») ; -1 sinon.
func colIndex(s string) int {
	if s == "" || len(s) > 3 {
		return -1
	}
	c := 0
	for i := 0; i < len(s); i++ {
		l := s[i] | 0x20
		if l < 'a' || l > 'z' {
			return -1
		}
		c = c*26 + int(l-'a'+1)
	}
	return c - 1
}

type prioRule struct {
	prio int
	seq  int
	rule canvas.Rule
}

// placeRules pose les règles de la feuille sur ses tableaux, ses lignes et
// colonnes et ses cellules libres.
func (x *xlsxFile) placeRules(sh xlsxSheet, sb *sheetBuild) {
	type tcol struct {
		tb *tableBuild
		j  int
	}
	byCol := map[tcol][]prioRule{}
	byLine := map[[2]int][]prioRule{} // (0 ligne / 1 colonne, rang)
	byCell := map[[2]int][]prioRule{}
	lost, seq := 0, 0
	for _, cf := range sh.ext.cfs {
		rects, cols, rows := sqrefParts(cf.sqref)
		for _, xr := range cf.rules {
			rule, ok := x.cfRule(xr)
			if !ok {
				lost++
				continue
			}
			prio, _ := strconv.Atoi(xr.attrs["priority"])
			seq++
			pr := prioRule{prio, seq, rule}
			placed := false
			for _, c := range cols {
				for i := c[0]; i <= min(c[1], c[0]+canvas.MaxStyleLines-1); i++ {
					byLine[[2]int{1, i}] = append(byLine[[2]int{1, i}], pr)
					placed = true
				}
			}
			for _, r := range rows {
				for i := r[0]; i <= min(r[1], r[0]+canvas.MaxStyleLines-1); i++ {
					byLine[[2]int{0, i}] = append(byLine[[2]int{0, i}], pr)
					placed = true
				}
			}
			for _, r := range rects {
				cells := 0
				for _, tb := range sb.tables {
					is := tb.is
					if r.r1 <= is.r0 || r.r0 > is.r1 || r.c1 < is.c0 || r.c0 > is.c1 {
						continue
					}
					for c := max(r.c0, is.c0); c <= min(r.c1, is.c1); c++ {
						k := tcol{tb, c - is.c0}
						byCol[k] = append(byCol[k], pr)
						placed = true
					}
				}
				if (r.r1-r.r0+1)*(r.c1-r.c0+1) > canvas.MaxStyleCells {
					continue
				}
				for rr := r.r0; rr <= r.r1; rr++ {
					for c := r.c0; c <= r.c1; c++ {
						if sb.tableAt(rr, c) != nil {
							continue
						}
						if cells++; cells <= canvas.MaxStyleCells {
							byCell[[2]int{rr, c}] = append(byCell[[2]int{rr, c}], pr)
							placed = true
						}
					}
				}
			}
			if !placed {
				lost++
			}
		}
	}
	sorted := func(ps []prioRule) []canvas.Rule {
		slices.SortFunc(ps, func(a, b prioRule) int { return cmp.Or(cmp.Compare(a.prio, b.prio), cmp.Compare(a.seq, b.seq)) })
		out := make([]canvas.Rule, len(ps))
		for i, p := range ps {
			out[i] = p.rule
		}
		return out
	}
	for k, ps := range byCol {
		tf := k.tb.tf
		if tf.Rules == nil {
			tf.Rules = make([][]canvas.Rule, len(tf.Cols))
		}
		tf.Rules[k.j] = sorted(ps)
	}
	for k, ps := range byLine {
		l := canvas.ImportLineRules{Index: k[1], Rules: sorted(ps)}
		if k[0] == 0 {
			sb.f.RowRules = append(sb.f.RowRules, l)
		} else {
			sb.f.ColRules = append(sb.f.ColRules, l)
		}
	}
	for k, ps := range byCell {
		sb.patch(k[0], k[1], canvas.Style{"cf": sorted(ps)})
	}
	x.imp.lose(&x.imp.rules, sh.name, lost)
}
