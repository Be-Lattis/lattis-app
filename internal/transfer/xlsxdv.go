package transfer

import (
	"strings"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/formula"
)

// Validations des données d'un fichier .xlsx : une liste de valeurs écrite
// dans la règle (« Oui,Non ») ou tirée d'une plage (=$D$2:$D$9,
// =Feuille2!$A$1:$A$20, un nom défini) devient la propriété dv de Lattis :
// la liste, ou la source (colonne du tableau importé à cet endroit, sinon
// plage de cellules). Les autres validations sont comptées comme non
// importées.

// dvValue donne la propriété dv d'une validation ; faux si elle n'a pas
// d'équivalent.
func (x *xlsxFile) dvValue(dv *xDV, sheet string, rv *resolver) (any, bool) {
	a := dv.attrs
	on := func(k string) bool { return a[k] == "1" || a[k] == "true" }
	top, _, _ := strings.Cut(strings.Fields(dv.sqref + " ")[0], ":")
	v, ok := validationFromXlsx(XlsxValidation{Type: a["type"], Operator: a["operator"], AllowBlank: on("allowBlank"),
		ShowInput: on("showInputMessage"), ShowError: on("showErrorMessage"), ErrorStyle: a["errorStyle"], ErrorTitle: a["errorTitle"],
		Error: a["error"], PromptTitle: a["promptTitle"], Prompt: a["prompt"], Formula1: dv.f1, Formula2: dv.f2, TopLeft: top})
	if a["type"] != "list" {
		return v, ok // xlsxvalidation.go
	}
	// Liste : la source est retrouvée dans les tableaux importés (colonne de
	// tableau, nom défini) ; messages et alerte gardés.
	l, lok := x.listValidation(dv.f1, sheet, rv)
	if !lok {
		return v, ok && (v["list"] != nil || v["src"] != nil)
	}
	if !ok || v["type"] != "list" {
		return l, true // sans alerte : la liste déroulante seule
	}
	delete(v, "list")
	delete(v, "src")
	switch l := l.(type) {
	case []any:
		v["list"] = l
	case map[string]any:
		v["src"] = l["src"]
	}
	return v, true
}

// listValidation lit la source d'une liste.
func (x *xlsxFile) listValidation(f1, sheet string, rv *resolver) (any, bool) {
	f := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(f1), "="))
	if len(f) >= 2 && f[0] == '"' && f[len(f)-1] == '"' {
		var list []any
		for _, v := range strings.Split(strings.ReplaceAll(f[1:len(f)-1], `""`, `"`), ",") {
			if v = strings.TrimSpace(v); v != "" {
				list = append(list, v)
			}
		}
		return list, len(list) > 0
	}
	if v, ok := x.names[strings.ToLower(f)]; ok {
		f = strings.TrimPrefix(strings.TrimSpace(v), "=")
	}
	src, ok := x.rangeSource(f, sheet, rv)
	if !ok {
		return nil, false
	}
	return map[string]any{"src": src}, true
}

// rangeSource écrit une plage d'Excel (A1, $A:$A, Feuille!A1:A9) en source
// de Lattis : la colonne d'un tableau importé quand la plage y tombe
// (Tableau[Colonne]), sinon la plage de cellules.
func (x *xlsxFile) rangeSource(ref, sheet string, rv *resolver) (string, bool) {
	ref = strings.ReplaceAll(strings.TrimSpace(ref), "$", "")
	explicit := false
	if k := strings.LastIndex(ref, "!"); k >= 0 {
		sheet, ref, explicit = ref[:k], ref[k+1:], true
	}
	sk := sheetKey(sheet)
	r, ok := rangeRect(ref)
	if !ok {
		a, b, two := strings.Cut(ref, ":")
		ca, cb := colIndex(a), colIndex(b)
		if !two || ca < 0 || ca != cb {
			return "", false
		}
		r = rect{r0: 0, c0: ca, r1: ExcelRows - 1, c1: ca}
	}
	whole := r.r1 == ExcelRows-1
	if r.c0 == r.c1 {
		// Une colonne de tableau : la colonne entière, ou ses lignes
		// (en-tête compris ou non, plage allongée pour les lignes à venir).
		for _, is := range rv.bySheet[sk] {
			if !is.table() || r.c0 < is.c0 || r.c0 > is.c1 {
				continue
			}
			if whole || (r.r0 == is.r0 || r.r0 == is.r0+1) && r.r1 > is.r0 && r.r1 <= is.r1+10_000 {
				return is.name + "[" + is.headers[r.c0-is.c0] + "]", true
			}
		}
	}
	if whole {
		return "", false
	}
	cr := formula.CellRange{R1: r.r0, C1: r.c0, R2: r.r1, C2: r.c1}
	if explicit {
		cr.Sheet = sheet
		if len(sheet) >= 2 && sheet[0] == '\'' {
			cr.Sheet = strings.ReplaceAll(sheet[1:len(sheet)-1], "''", "'")
		}
	}
	s, err := canvas.NormalizeDVSource(cr.String())
	return s, err == nil
}

// placeValidations pose les validations de la feuille.
func (x *xlsxFile) placeValidations(sh xlsxSheet, sb *sheetBuild, rv *resolver) []string {
	lost := 0
	for _, dv := range sh.ext.dvs {
		v, ok := x.dvValue(dv, sh.name, rv)
		if !ok {
			lost++
			continue
		}
		p := canvas.Style{"dv": v}
		rects, cols, _ := sqrefParts(dv.sqref)
		placed := false
		for _, c := range cols {
			for _, tb := range sb.tables {
				for j := max(c[0], tb.is.c0); j <= min(c[1], tb.is.c1); j++ {
					sb.patchColumn(tb, j-tb.is.c0, p)
					placed = true
				}
			}
		}
		for _, r := range rects {
			for _, tb := range sb.tables {
				is := tb.is
				if r.r1 <= is.r0 || r.r0 > is.r1 || r.c1 < is.c0 || r.c0 > is.c1 {
					continue
				}
				for j := max(r.c0, is.c0); j <= min(r.c1, is.c1); j++ {
					sb.patchColumn(tb, j-is.c0, p)
					placed = true
				}
			}
			if (r.r1-r.r0+1)*(r.c1-r.c0+1) > canvas.MaxStyleCells {
				continue
			}
			for rr := r.r0; rr <= r.r1; rr++ {
				for c := r.c0; c <= r.c1; c++ {
					if sb.tableAt(rr, c) == nil {
						sb.patch(rr, c, p)
						placed = true
					}
				}
			}
		}
		if !placed {
			lost++
		}
	}
	x.imp.lose(&x.imp.lists, sh.name, lost)
	return nil
}
