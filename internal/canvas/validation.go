package canvas

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
)

// Validation des données tirée d'une plage (comme la source « =$A$1:$A$10 »
// d'une liste d'Excel).
//
// La propriété dv d'une couche de style est soit la liste des valeurs
// permises (["Oui", "Non"], forme d'origine, inchangée), soit un objet
// {"src": "Clients[Ville]"} : la liste est alors relue à chaque ouverture de
// la liste déroulante et à chaque saisie. La source est une colonne de
// tableau (Clients[Ville], ou ses lignes Clients[Ville][1:5]), une plage de
// cellules libres (Cell[A1:A10], Feuille2!Cell[A1:A10]), une cellule
// (Cell[A1]) ou une cellule nommée. Les valeurs sont distinctes (sans
// casse), dans l'ordre de la source, sans les vides ni les erreurs.
//
// La source suit les renommages (tableau, colonne, feuille, cellule nommée :
// rewriteRefs) et les déplacements de cellules (insertion et suppression de
// lignes ou colonnes, couper-coller : rewriteCellRefs). Une source disparue
// (tableau supprimé, plage effacée par une suppression de lignes : #REF!)
// donne une erreur, montrée dans le dialogue et dans la liste.

// MaxValidationValues borne une liste de validation tirée d'une plage.
const MaxValidationValues = 1000

// ValidationList est la liste d'une validation tirée d'une plage : les
// valeurs, ou pourquoi la source ne se lit pas.
type ValidationList struct {
	Values []string `json:"values"`
	Error  string   `json:"error,omitempty"`
}

// DVSource rend la source d'une validation (propriété dv) tirée d'une plage.
func DVSource(dv any) (string, bool) {
	m, ok := dv.(map[string]any)
	if !ok {
		return "", false
	}
	s, ok := m["src"].(string)
	return s, ok && s != ""
}

// normalizeDV met en forme la propriété dv d'un correctif : un texte « =Ref »
// devient {"src": "Ref"}, une plage tapée à la Excel (A1:A10,
// Feuille2!A1:A10) s'écrit Cell[A1:A10] ; une source qui n'est pas une
// référence est refusée. Une liste reste telle quelle.
func normalizeDV(patch Style) error {
	v, ok := patch["dv"]
	if !ok || empty(v) {
		return nil
	}
	src := ""
	switch x := v.(type) {
	case string:
		if !strings.HasPrefix(strings.TrimSpace(x), "=") {
			// Ancienne forme tapée « a;b;c » : une liste.
			var list []any
			for _, p := range strings.Split(x, ";") {
				if p = strings.TrimSpace(p); p != "" {
					list = append(list, p)
				}
			}
			patch["dv"] = list
			return nil
		}
		src = x
	case map[string]any:
		if x["type"] != nil {
			// Forme objet (dvrules.go) : nombre, date, formule, messages…
			dv, err := normalizeDVRule(x)
			if err != nil {
				return err
			}
			patch["dv"] = dv
			return nil
		}
		s, _ := x["src"].(string)
		if strings.TrimSpace(s) == "" {
			return msg.New("validation.emptySource", nil)
		}
		src = s
	default:
		return nil
	}
	ref, err := NormalizeDVSource(src)
	if err != nil {
		return err
	}
	patch["dv"] = map[string]any{"src": ref}
	return nil
}

// NormalizeDVSource lit la source d'une validation (avec ou sans « = ») et
// la rend dans la nomenclature de Lattis ; erreur si ce n'est pas une
// colonne de tableau, une plage ou une cellule, ou une cellule nommée.
func NormalizeDVSource(src string) (string, error) {
	s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(src), "="))
	if s == "" {
		return "", msg.New("validation.emptySource", nil)
	}
	n, err := formula.Parse(s, formula.FR)
	if err != nil {
		return "", msg.New("validation.notReference", msg.Args{"value": s})
	}
	switch v := n.(type) {
	case formula.Ref:
		if v.Table != "" && !v.Row {
			return s, nil
		}
	case formula.RowRange:
		if v.Ref.Table != "" && !v.Ref.Row {
			return s, nil
		}
	case formula.CellRange, formula.Name:
		return s, nil
	case formula.Range:
		// Plage à la Excel : A1:A10, Feuille2!A1:A10.
		if r, ok := excelRange(v.Text); ok {
			return r.String(), nil
		}
	}
	return "", msg.New("validation.notRange", msg.Args{"value": s})
}

// excelRange lit « A1:B3 » ou « Feuille!A1:B3 » (sans $).
func excelRange(text string) (formula.CellRange, bool) {
	text = strings.ReplaceAll(text, "$", "")
	sheet, rest := "", text
	if k := strings.LastIndex(text, "!"); k >= 0 {
		sheet, rest = strings.Trim(text[:k], "'"), text[k+1:]
	}
	a, b, ok := strings.Cut(rest, ":")
	if !ok {
		return formula.CellRange{}, false
	}
	_, r1, c1, ok1 := formula.A1Address(a)
	_, r2, c2, ok2 := formula.A1Address(b)
	if !ok1 || !ok2 {
		return formula.CellRange{}, false
	}
	return formula.CellRange{Sheet: sheet, R1: min(r1, r2), C1: min(c1, c2), R2: max(r1, r2), C2: max(c1, c2)}, true
}

// ValidationList lit la liste d'une validation tirée d'une plage, pour une
// cellule de la feuille sheet (une adresse sans feuille désigne celle-ci).
func (m *Model) ValidationList(ctx context.Context, sheet int64, src string) (ValidationList, error) {
	var out ValidationList
	err := m.read(ctx, func(t *tx) error {
		vals, why := t.validationList(sheet, src)
		out = ValidationList{Values: vals, Error: why}
		if out.Values == nil {
			out.Values = []string{}
		}
		// Choisies, les valeurs sont saisies comme tapées dans la langue
		// de la session : elles s'y écrivent (dates, décimale, VRAI).
		u := t.ui()
		for i, v := range out.Values {
			out.Values[i] = u.valueOut(v)
		}
		return nil
	})
	return out, err
}

// validationList rend les valeurs de la source, ou le message qui dit
// pourquoi elle ne se lit pas.
func (t *tx) validationList(sheet int64, src string) ([]string, string) {
	s, err := NormalizeDVSource(src)
	if err != nil {
		if strings.Contains(src, "#REF!") {
			return nil, t.say("validation.sourceDeleted", msg.Args{"ref": "#REF!"})
		}
		return nil, strings.TrimPrefix(err.Error(), "validation : ")
	}
	n, err := formula.Parse(s, formula.FR)
	if err != nil {
		return nil, t.say("validation.unreadableRef", msg.Args{"ref": s})
	}
	switch v := n.(type) {
	case formula.Ref:
		return t.columnList(sheet, v, 1, 0)
	case formula.RowRange:
		return t.columnList(sheet, v.Ref, v.From, v.To)
	case formula.CellRange:
		return t.rangeList(sheet, v)
	case formula.Name:
		sh, addr := "", v.Name
		if k := strings.LastIndex(v.Name, "."); k >= 0 {
			sh, addr = v.Name[:k], v.Name[k+1:]
		}
		if r, c, ok := formula.CellRef(addr); ok {
			return t.rangeList(sheet, formula.CellRange{Sheet: sh, R1: r, C1: c, R2: r, C2: c})
		}
		return t.namedList(v.Name)
	}
	return nil, t.say("validation.notARange", msg.Args{"ref": s})
}

// columnList : valeurs distinctes d'une colonne de tableau (ou de ses
// lignes from à to, rangs d'affichage à partir de 1 ; to = 0 : toutes),
// dans l'ordre des lignes.
func (t *tx) columnList(sheet int64, ref formula.Ref, from, to int) ([]string, string) {
	schema, err := t.schema()
	if err != nil {
		return nil, err.Error()
	}
	tb, ok := schema.Table(ref.Table)
	if !ok {
		return nil, t.say("validation.tableGone", msg.Args{"table": ref.Table})
	}
	found := false
	for _, c := range tb.Cols {
		found = found || strings.EqualFold(c.Name, strings.TrimSpace(ref.Col))
	}
	if !found {
		return nil, t.say("validation.columnGone", msg.Args{"column": ref.Col, "table": tb.Name})
	}
	col := ref.Table + "[" + ref.Col + "]"
	raw := "=UNIQUE(" + col + ")"
	if to > 0 {
		raw = fmt.Sprintf("=PRENDRE(%s;%d)", col, to)
	}
	rows, ok, e := t.spillFreeRaw(sheet, raw)
	if !ok || e.IsErr() {
		return nil, t.say("validation.unreadableSource", msg.Args{"err": e.String()})
	}
	var vals []formula.Value
	for i, r := range rows {
		if len(r) > 0 && i+1 >= from {
			vals = append(vals, formula.Str(r[0]))
		}
	}
	return distinctValues(vals), ""
}

// rangeList : valeurs d'une plage de cellules libres, ligne par ligne.
func (t *tx) rangeList(sheet int64, rg formula.CellRange) ([]string, string) {
	home := sheet
	if rg.Sheet != "" {
		ids, err := t.sheetIDs()
		if err != nil {
			return nil, err.Error()
		}
		id, ok := ids[strings.ToLower(rg.Sheet)]
		if !ok {
			return nil, t.say("validation.sheetGone", msg.Args{"sheet": rg.Sheet})
		}
		home = id
	}
	if r, c := rg.Size(); r*c > MaxSpillRows {
		return nil, fmt.Sprintf("plage trop grande (%d cellules au plus)", MaxSpillRows)
	}
	values, err := t.allCellValues()
	if err != nil {
		return nil, err.Error()
	}
	var vals []formula.Value
	for r := rg.R1; r <= rg.R2; r++ {
		for c := rg.C1; c <= rg.C2; c++ {
			if v, ok := values[formula.CellKey(home, r, c)]; ok {
				vals = append(vals, v)
			}
		}
	}
	return distinctValues(vals), ""
}

// namedList : la valeur d'une cellule nommée.
func (t *tx) namedList(name string) ([]string, string) {
	values, err := t.allCellValues()
	if err != nil {
		return nil, err.Error()
	}
	v, ok := values[strings.ToLower(name)]
	if !ok {
		return nil, t.say("validation.notNameOrRange", msg.Args{"name": name})
	}
	return distinctValues([]formula.Value{v}), ""
}

// allCellValues : valeurs de toutes les cellules libres et nommées.
func (t *tx) allCellValues() (map[string]formula.Value, error) {
	schema, err := t.schema()
	if err != nil {
		return nil, err
	}
	return t.cellValues(schema)
}

// distinctValues : textes distincts (sans casse), dans l'ordre, sans vides
// ni erreurs, MaxValidationValues au plus.
func distinctValues(vals []formula.Value) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range vals {
		if v.IsEmpty() || v.IsErr() {
			continue
		}
		s := strings.TrimSpace(v.String())
		k := strings.ToLower(s)
		if s == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
		if len(out) >= MaxValidationValues {
			break
		}
	}
	return out
}

// Réécriture des sources --------------------------------------------------

// dvStyleTables : couches de style qui portent une validation, avec la
// feuille où se lit une adresse sans feuille.
var dvStyleTables = []struct{ table, keys, sheet, self string }{
	{"_lattis_cell_styles", "sheet, row, col", "sheet", "''"},
	{"_lattis_row_styles", "sheet, row, -1", "sheet", "''"},
	{"_lattis_col_styles", "sheet, -1, col", "sheet", "''"},
	{"_lattis_styles", "tbl, rid, col", "(SELECT sheet FROM _lattis_tables WHERE id = tbl)", "coalesce((SELECT name FROM _lattis_tables WHERE id = tbl), '')"},
}

// rewriteDVSources applique fn (source, feuille de la couche) aux sources
// des validations tirées d'une plage ; fn rend la nouvelle source et vrai si
// elle change.
func (t *tx) rewriteDVSources(fn func(src string, sheet int64) (string, bool)) error {
	return t.rewriteDVTexts(func(src string, sheet int64, _ string) (string, bool) { return fn(src, sheet) })
}

// rewriteDVTexts est rewriteDVSources avec, pour une couche de tableau, le
// nom du tableau (ses [Colonne] et [@Colonne]).
func (t *tx) rewriteDVTexts(fn func(src string, sheet int64, self string) (string, bool)) error {
	for _, q := range dvStyleTables {
		rs, err := t.query(`SELECT ` + q.keys + `, coalesce(` + q.sheet + `, 0), ` + q.self + `, style FROM ` + q.table + ` WHERE style LIKE '%"dv":{%'`)
		if err != nil {
			return err
		}
		type up struct {
			k          [3]int64
			old, style string
		}
		var ups []up
		for rs.Next() {
			var k [3]int64
			var sheet int64
			var self, s string
			if err := rs.Scan(&k[0], &k[1], &k[2], &sheet, &self, &s); err != nil {
				rs.Close()
				return err
			}
			var st map[string]any
			if json.Unmarshal([]byte(s), &st) != nil {
				continue
			}
			// La source d'une liste, et les formules d'une validation objet
			// (bornes « =… », formule personnalisée) : les autres champs restent.
			dv, ok := st["dv"].(map[string]any)
			if !ok {
				continue
			}
			changed := false
			for _, k := range []string{"src", "formula", "min", "max"} {
				s, ok := dv[k].(string)
				if !ok || s == "" || (k != "src" && !strings.HasPrefix(s, "=")) {
					continue
				}
				if neu, ok := fn(s, sheet, self); ok && neu != s {
					dv[k], changed = neu, true
				}
			}
			if !changed {
				continue
			}
			st["dv"] = dv
			if enc, err := encodeJSON(st); err == nil {
				ups = append(ups, up{k, s, enc})
			}
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return err
		}
		var do, undo []stmt
		for _, u := range ups {
			where, args := dvWhere(q.table, u.k)
			do = append(do, st(`UPDATE `+q.table+` SET style = ? WHERE `+where, append([]any{u.style}, args...)...))
			undo = append(undo, st(`UPDATE `+q.table+` SET style = ? WHERE `+where, append([]any{u.old}, args...)...))
		}
		if len(do) > 0 {
			if err := t.apply(do, undo); err != nil {
				return err
			}
		}
	}
	return nil
}

func dvWhere(table string, k [3]int64) (string, []any) {
	switch table {
	case "_lattis_row_styles":
		return `sheet = ? AND row = ?`, []any{k[0], k[1]}
	case "_lattis_col_styles":
		return `sheet = ? AND col = ?`, []any{k[0], k[2]}
	case "_lattis_styles":
		return `tbl = ? AND rid = ? AND col = ?`, []any{k[0], k[1], k[2]}
	}
	return `sheet = ? AND row = ? AND col = ?`, []any{k[0], k[1], k[2]}
}

// rewriteValidations : renommages de tableaux, colonnes, feuilles et
// cellules nommées dans les sources des validations.
func (t *tx) rewriteValidations(rc refChange) error {
	return t.rewriteDVTexts(func(src string, _ int64, self string) (string, bool) {
		return formula.RenameRefs(src, formula.FR, rc.Renames, self)
	})
}
