package canvas

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// Textes échangés avec l'interface dans la langue de la session
// (docs/i18n.md, § 5) : l'utilisateur tape et lit ses formules et ses
// valeurs dans sa langue (noms des fonctions, séparateurs, décimale, dates,
// VRAI/FAUX) ; le classeur, lui, les garde sous une forme qui ne dépend pas
// de la langue :
//
//   - français pour le texte brut des cellules libres (_lattis_cells.raw,
//     formules comprises), les formules des règles, des validations, des
//     onglets à choix, des champs calculés et des colonnes saisies à côté
//     d'un tableau croisé, et les valeurs des règles (« 3,5 »,
//     « 19/10/2026 ») : c'est la forme de toujours, les classeurs existants
//     se relisent sans migration ;
//   - anglais (formula.EN) pour les colonnes calculées, comme avant.
//
// La conversion se fait au bord du modèle (méthodes publiques) : la saisie
// passe de la langue de la session à la forme gardée (in), l'affichage de
// la forme gardée à la langue (out). Ce qui circule entre les postes d'un
// classeur partagé (journal des opérations, coédition, serveur) est donc
// toujours la forme gardée. L'API des scripts et de l'assistant
// (internal/api) travaille directement sur la forme gardée (StorageForm) :
// un script donne le même résultat quelle que soit la langue du poste.

type storageKey struct{}

// StorageForm marque ctx : les textes reçus et rendus y sont dans la forme
// gardée par le classeur (français), quelle que soit la langue de la
// session.
func StorageForm(ctx context.Context) context.Context {
	return context.WithValue(ctx, storageKey{}, true)
}

// ui : l'écriture des formules et des valeurs de la session.
type ui struct {
	loc   formula.Locale
	dates grid.DateStyle
}

// stored : l'écriture gardée par le classeur.
var stored = ui{loc: formula.FR, dates: grid.FrenchDates}

// ui : l'écriture de la session pour un appel (la forme gardée sous
// StorageForm).
func (m *Model) ui(ctx context.Context) ui {
	if ctx != nil && ctx.Value(storageKey{}) != nil {
		return stored
	}
	m.locMu.Lock()
	loc, lang := m.loc, m.lang
	m.locMu.Unlock()
	return ui{loc: loc, dates: grid.DateStyleFor(string(lang))}
}

// ui : l'écriture de la session de l'appel qui a ouvert la transaction.
func (t *tx) ui() ui { return t.m.ui(t.Ctx) }

// plain : la session écrit comme le classeur, rien à convertir.
func (u ui) plain() bool { return u == stored }

// formulaIn : une formule tapée dans la session, dans la forme gardée
// (français). Formule illisible : ErrFormula, qui situe l'erreur dans le
// texte tapé.
func (u ui) formulaIn(src string) (string, error) {
	if u.plain() {
		return src, nil
	}
	out, err := formula.Translate(src, u.loc, formula.FR)
	if err != nil {
		return src, ErrFormula{err}
	}
	return out, nil
}

// formulaOut : une formule gardée (français) dans la langue de la session ;
// illisible, telle quelle.
func (u ui) formulaOut(fr string) string {
	if u.plain() || fr == "" {
		return fr
	}
	out, _ := formula.Translate(fr, formula.FR, u.loc)
	return out
}

// columnOut : la formule d'une colonne calculée (gardée en anglais) dans la
// langue de la session.
func (u ui) columnOut(en string) string {
	if en == "" {
		return ""
	}
	out, err := formula.Translate(en, formula.EN, u.loc)
	if err != nil {
		return en
	}
	return out
}

// input : une saisie (formule ou valeur) dans la forme gardée.
func (u ui) input(raw string) (string, error) {
	if u.plain() {
		return raw, nil
	}
	if isFormula(raw) {
		return u.formulaIn(raw)
	}
	return u.valueIn(raw), nil
}

// show : le texte brut gardé d'une cellule libre dans la langue de la
// session.
func (u ui) show(raw string) string {
	if u.plain() {
		return raw
	}
	if isFormula(raw) {
		return u.formulaOut(raw)
	}
	return u.valueOut(raw)
}

// result : la valeur affichée d'une formule (Value.String, en français)
// dans la langue de la session, codes d'erreur compris.
func (u ui) result(s string) string {
	if u.plain() {
		return s
	}
	return formula.ErrorText(u.valueOut(s), u.loc)
}

var (
	// Nombres écrits avec la décimale « . » (anglais, chinois) : 3.5,
	// 1,234.5, -0.25 %.
	dotNumberRe = regexp.MustCompile(`^([+-]?)(\d{1,3}(?:,\d{3})+|\d*)(?:\.(\d+))?(\s*[%€])?$`)
	// Milliers séparés par « . » (allemand, espagnol) : 1.234,5.
	dotGroupRe = regexp.MustCompile(`^([+-]?)(\d{1,3}(?:\.\d{3})+)(,\d+)?(\s*[%€])?$`)
	// Nombres gardés à la française : 3,5, 1 234,5, -0,25 %.
	frNumberRe = regexp.MustCompile(`^([+-]?)(\d{1,3}(?:[ \x{a0}\x{202f}]\d{3})+|\d+)(?:,(\d+))?(\s*[%€])?$`)
)

// valueIn : une valeur tapée dans la session (nombre, date, booléen) dans
// la forme gardée ; un texte ne change pas. Les écritures françaises
// restent acceptées (« 1,5 » se lit 1,5 partout).
func (u ui) valueIn(raw string) string {
	if u.plain() {
		return raw
	}
	s := strings.TrimSpace(raw)
	if s == "" || strings.HasPrefix(s, "'") {
		return raw
	}
	if u.loc.Names != formula.NamesFR {
		switch strings.ToUpper(s) {
		case "TRUE":
			return "VRAI"
		case "FALSE":
			return "FAUX"
		}
	}
	if u.loc.Dot {
		if m := dotNumberRe.FindStringSubmatch(s); m != nil && m[2] != "" && (m[3] != "" || strings.Contains(m[2], ",")) {
			out := m[1] + strings.ReplaceAll(m[2], ",", "")
			if m[3] != "" {
				out += "," + m[3]
			}
			return out + m[4]
		}
	} else if m := dotGroupRe.FindStringSubmatch(s); m != nil {
		return m[1] + strings.ReplaceAll(m[2], ".", "") + m[3] + m[4]
	}
	if d := u.dates.Input(s); d != s {
		return d
	}
	return raw
}

// valueOut : une valeur gardée (nombre, date, VRAI/FAUX) dans l'écriture de
// la session ; un texte ne change pas.
func (u ui) valueOut(s string) string {
	if u.plain() || s == "" {
		return s
	}
	if u.loc.Names != formula.NamesFR {
		switch s {
		case "VRAI":
			return "TRUE"
		case "FAUX":
			return "FALSE"
		}
	}
	if u.loc.Dot {
		if m := frNumberRe.FindStringSubmatch(s); m != nil && (m[3] != "" || len(m[2]) > 3) {
			out := m[1] + strings.Map(func(r rune) rune {
				if r == ' ' || r == 0xa0 || r == 0x202f {
					return ','
				}
				return r
			}, m[2])
			if m[3] != "" {
				out += "." + m[3]
			}
			return out + m[4]
		}
	}
	return u.dates.Show(s)
}

// blockIn : un bloc collé (formules et valeurs) dans la forme gardée.
func (u ui) blockIn(block [][]string) ([][]string, error) {
	if u.plain() {
		return block, nil
	}
	out := make([][]string, len(block))
	for r, row := range block {
		out[r] = make([]string, len(row))
		for c, s := range row {
			v, err := u.input(s)
			if err != nil {
				return nil, err
			}
			out[r][c] = v
		}
	}
	return out, nil
}

// valuesIn : un bloc de valeurs affichées (sans formules) dans la forme
// gardée.
func (u ui) valuesIn(block [][]string) [][]string {
	if u.plain() {
		return block
	}
	out := make([][]string, len(block))
	for r, row := range block {
		out[r] = make([]string, len(row))
		for c, s := range row {
			out[r][c] = u.valueIn(formula.ErrorInput(s))
		}
	}
	return out
}

// typing : l'écriture à poser sur une transaction (tx.typed) pour une
// saisie de l'interface ; nil si elle est déjà la forme gardée.
func (u ui) typing() *ui {
	if u.plain() {
		return nil
	}
	return &u
}

// cellText : le texte à saisir dans la colonne i de tb : la valeur lue dans
// la forme gardée (val), ou le texte tapé dans une colonne Texte (typed).
func (t *tx) cellText(tb Table, i int, val, typed string) string {
	if i >= 0 && i < len(tb.Cols) && tb.Cols[i].Type == grid.Text && tb.Cols[i].Formula == "" {
		return typed
	}
	return val
}

// rulesIn : les règles reçues de l'interface, formules et valeurs dans la
// forme gardée.
func (u ui) rulesIn(rules []Rule) ([]Rule, error) {
	if u.plain() || len(rules) == 0 {
		return rules, nil
	}
	out := append([]Rule{}, rules...)
	for i, r := range out {
		if r.Op == "formula" {
			src := strings.TrimSpace(r.Value)
			if src == "" {
				continue
			}
			if !strings.HasPrefix(src, "=") {
				src = "=" + src
			}
			f, err := u.formulaIn(src)
			if err != nil {
				return nil, err
			}
			out[i].Value = f
			continue
		}
		if ruleValueOps[r.Op] {
			out[i].Value, out[i].To = u.valueIn(r.Value), u.valueIn(r.To)
		}
	}
	return out, nil
}

// rulesOut : les règles gardées dans l'écriture de la session.
func (u ui) rulesOut(rules []Rule) []Rule {
	if u.plain() || len(rules) == 0 {
		return rules
	}
	out := append([]Rule{}, rules...)
	for i, r := range out {
		switch {
		case r.Op == "formula":
			out[i].Value = u.formulaOut(r.Value)
		case ruleValueOps[r.Op]:
			out[i].Value, out[i].To = u.valueOut(r.Value), u.valueOut(r.To)
		}
	}
	return out
}

// changesIn : les changements de règles d'une feuille dans la forme gardée
// (les listes sont copiées, pas modifiées sur place).
func (u ui) changesIn(ch *RuleChanges) error {
	if u.plain() {
		return nil
	}
	var err error
	cols := append([]ColumnRules{}, ch.Columns...)
	for i := range cols {
		if cols[i].Rules, err = u.rulesIn(cols[i].Rules); err != nil {
			return err
		}
	}
	lines := append([]LineRuleSet{}, ch.Lines...)
	for i := range lines {
		if lines[i].Rules, err = u.rulesIn(lines[i].Rules); err != nil {
			return err
		}
	}
	cells := append([]RangeRules{}, ch.Cells...)
	for i := range cells {
		if cells[i].Rules, err = u.rulesIn(cells[i].Rules); err != nil {
			return err
		}
	}
	ch.Columns, ch.Lines, ch.Cells = cols, lines, cells
	return nil
}

// rulesJSONOut : une liste de règles encodée (règles d'une colonne, de
// lignes) dans l'écriture de la session.
func (u ui) rulesJSONOut(s string) string {
	if u.plain() || s == "" {
		return s
	}
	var rules []Rule
	if json.Unmarshal([]byte(s), &rules) != nil {
		return s
	}
	out, err := encodeRules(u.rulesOut(rules))
	if err != nil || out == nil {
		return s
	}
	return out.(string)
}

// styleOut : une mise en forme encodée (JSON) dont les règles (cf) passent
// dans l'écriture de la session.
func (u ui) styleOut(s string) string {
	if u.plain() || !strings.Contains(s, `"cf"`) {
		return s
	}
	var st map[string]any
	if json.Unmarshal([]byte(s), &st) != nil {
		return s
	}
	rules, ok := styleRules(st["cf"])
	if !ok {
		return s
	}
	st["cf"] = u.rulesOut(rules)
	b, err := json.Marshal(st)
	if err != nil {
		return s
	}
	return string(b)
}

// styleIn : les règles (cf) d'un correctif de mise en forme dans la forme
// gardée (le correctif est copié).
func (u ui) styleIn(patch Style) (Style, error) {
	if u.plain() {
		return patch, nil
	}
	rules, ok := styleRules(patch["cf"])
	if !ok {
		return patch, nil
	}
	rules, err := u.rulesIn(rules)
	if err != nil {
		return nil, err
	}
	out := Style{}
	for k, v := range patch {
		out[k] = v
	}
	out["cf"] = rules
	return out, nil
}

// styleRules lit les règles cf d'une mise en forme décodée.
func styleRules(v any) ([]Rule, bool) {
	if v == nil {
		return nil, false
	}
	if r, ok := v.([]Rule); ok {
		return r, true
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var rules []Rule
	if json.Unmarshal(b, &rules) != nil || len(rules) == 0 {
		return nil, false
	}
	return rules, true
}

// filterOrderOps : conditions de filtre qui comparent à une valeur (nombre,
// date) tapée comme dans une cellule.
var filterOrderOps = map[string]bool{"gt": true, "ge": true, "lt": true, "le": true, "between": true}

// filterIn : un filtre de colonne reçu de l'interface, valeurs des
// conditions d'ordre dans la forme gardée (copié).
func (u ui) filterIn(f *ColumnFilter) *ColumnFilter {
	if u.plain() || f == nil || len(f.Conds) == 0 {
		return f
	}
	out := *f
	out.Conds = u.condsMap(f.Conds, u.valueIn)
	return &out
}

func (u ui) condsMap(cs []FilterCond, fn func(string) string) []FilterCond {
	out := append([]FilterCond{}, cs...)
	for i, c := range out {
		if filterOrderOps[c.Op] {
			out[i].Value, out[i].To = fn(c.Value), fn(c.To)
		}
	}
	return out
}

// filterJSONOut : les filtres d'un tableau (JSON, par colonne) dans
// l'écriture de la session.
func (u ui) filterJSONOut(s string) string {
	if u.plain() || !strings.Contains(s, `"conds"`) {
		return s
	}
	var fs map[string]*ColumnFilter
	if json.Unmarshal([]byte(s), &fs) != nil {
		return s
	}
	for k, f := range fs {
		if f != nil && len(f.Conds) > 0 {
			c := *f
			c.Conds = u.condsMap(f.Conds, u.valueOut)
			fs[k] = &c
		}
	}
	b, err := json.Marshal(fs)
	if err != nil {
		return s
	}
	return string(b)
}

// layoutOut met une mise en page dans l'écriture de la session : texte
// brut et valeurs des cellules libres, règles des colonnes, des lignes et
// des cellules (les formules des colonnes calculées, des onglets à choix et
// des tableaux croisés y sont déjà). Les listes sont recopiées.
func (u ui) layoutOut(l *Layout) {
	if u.plain() {
		return
	}
	cells := make([]CellInfo, len(l.Cells))
	for i, c := range l.Cells {
		c.Raw = u.show(c.Raw)
		if c.Value != "" {
			c.Value = u.result(c.Value)
		}
		if c.Spill != nil {
			spill := make([][]string, len(c.Spill))
			for r, row := range c.Spill {
				spill[r] = make([]string, len(row))
				for k, v := range row {
					spill[r][k] = u.result(v)
				}
			}
			c.Spill = spill
		}
		cells[i] = c
	}
	l.Cells = cells
	tables := make([]TableInfo, len(l.Tables))
	for i, tb := range l.Tables {
		cols := make([]ColumnInfo, len(tb.Columns))
		for k, c := range tb.Columns {
			c.Rules = u.rulesJSONOut(c.Rules)
			c.Style = u.styleOut(c.Style)
			cols[k] = c
		}
		tb.Columns = cols
		tb.Filter = u.filterJSONOut(tb.Filter)
		tables[i] = tb
	}
	l.Tables = tables
	lines := func(in []LineRules) []LineRules {
		out := make([]LineRules, len(in))
		for i, r := range in {
			r.Rules = u.rulesJSONOut(r.Rules)
			out[i] = r
		}
		return out
	}
	l.RowRules, l.ColRules = lines(l.RowRules), lines(l.ColRules)
	styles := make([]CellStyle, len(l.Styles))
	for i, s := range l.Styles {
		s.Style = u.styleOut(s.Style)
		styles[i] = s
	}
	l.Styles = styles
}

// ruleValueOps : règles dont la valeur (et la borne To) est une valeur de
// cellule comparée (nombre, date) ; les autres (texte cherché, rang,
// période) ne changent pas.
var ruleValueOps = map[string]bool{">": true, ">=": true, "<": true, "<=": true, "=": true, "<>": true, "between": true, "notbetween": true}
