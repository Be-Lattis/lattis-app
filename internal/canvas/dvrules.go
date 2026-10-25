package canvas

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
)

// Validation des données au-delà des listes (Données › Validation des
// données d'un tableur) : nombre entier, décimal, date, heure, longueur du
// texte, formule personnalisée, avec message de saisie et alerte d'erreur.
//
// La propriété dv d'une couche de style garde ses deux formes d'origine
// (liste ["Oui", "Non"], plage {"src": "Clients[Ville]"}) et prend une forme
// objet, qui les contient :
//
//	{"type": "whole", "op": "between", "min": "1", "max": "10",
//	 "blank": false, "at": "B2",
//	 "input": {"title": "Quantité", "msg": "De 1 à 10"},
//	 "error": {"style": "warning", "title": "…", "msg": "…"}}
//
// type : any (tout, pour un message de saisie seul), whole, decimal, list
// (list : les valeurs, ou src : la plage), date, time, length, custom
// (formula : vraie quand la valeur est permise). op : between, notbetween,
// "=", "<>", ">", "<", ">=", "<=" ; min porte la valeur des opérateurs à une
// seule valeur. min, max : une constante gardée à la française (« 3,5 »,
// « 19/10/2026 », « 08:30 ») ou une formule (« =B1 », « =[Début] »).
// blank : faux quand « Ignorer si vide » est décoché (vrai par défaut).
// error.style : stop (refuse, par défaut), warning (demande Oui/Non),
// info (prévient, accepte).
//
// Cette cellule. La formule (et les bornes en formule) est écrite pour la
// cellule active au moment de la pose, dont at garde l'adresse : ses
// adresses relatives suivent chaque cellule, comme dans un tableur
// (=ESTNUM(B2) posée en B2 vaut =ESTNUM(B7) en B7). Dans un tableau, la
// formule se réécrit à la pose avec [@Colonne] pour la cellule de la ligne
// (=[@Qté]>0), comme les règles de mise en forme conditionnelle ; [@Col]
// s'y tape aussi directement.
//
// La vérification se fait à la saisie (SetCell sous WithEntryCheck, comme
// la grille), dans la transaction, après l'écriture : une formule comme
// =NB.SI(A:A;A2)=1 voit donc la nouvelle valeur ; un refus annule tout. Le
// collage, la recopie, les scripts et les autres postes d'un classeur
// partagé ne vérifient pas (comme un tableur pour le collage) : les valeurs
// non valides se voient avec « Entourer les données non valides »
// (InvalidCells).

// DVRule est une validation des données sous forme objet.
type DVRule struct {
	Type    string     `json:"type"`
	Op      string     `json:"op,omitempty"`
	Min     string     `json:"min,omitempty"`
	Max     string     `json:"max,omitempty"`
	Formula string     `json:"formula,omitempty"`
	List    []string   `json:"list,omitempty"`
	Src     string     `json:"src,omitempty"`
	Blank   *bool      `json:"blank,omitempty"`
	At      string     `json:"at,omitempty"`
	Input   *DVMessage `json:"input,omitempty"`
	Error   *DVMessage `json:"error,omitempty"`
}

// DVMessage : message de saisie (input) ou alerte d'erreur (error, avec
// son style).
type DVMessage struct {
	Style string `json:"style,omitempty"`
	Title string `json:"title,omitempty"`
	Msg   string `json:"msg,omitempty"`
}

var dvTypes = map[string]bool{"any": true, "whole": true, "decimal": true, "list": true, "date": true, "time": true, "length": true, "custom": true}

var dvOps = map[string]bool{"between": true, "notbetween": true, "=": true, "<>": true, ">": true, "<": true, ">=": true, "<=": true}

// dvOpNames : nom des opérateurs dans le catalogue des messages.
var dvOpNames = map[string]string{"between": "between", "notbetween": "notbetween", "=": "eq", "<>": "ne", ">": "gt", "<": "lt", ">=": "ge", "<=": "le"}

// Longueurs d'un tableur (fichier .xlsx) : titre 32, message 255.
const (
	dvTitleMax = 32
	dvMsgMax   = 255
)

// ignoreBlank : « Ignorer si vide » (coché par défaut).
func (r DVRule) ignoreBlank() bool { return r.Blank == nil || *r.Blank }

// errorStyle : stop, warning ou info.
func (r DVRule) errorStyle() string {
	if r.Error != nil && (r.Error.Style == "warning" || r.Error.Style == "info") {
		return r.Error.Style
	}
	return "stop"
}

// ParseDV lit une propriété dv (liste, plage ou objet) ; faux si ce n'est
// pas une validation.
func ParseDV(v any) (DVRule, bool) {
	switch x := v.(type) {
	case []any:
		var list []string
		for _, e := range x {
			list = append(list, strings.TrimSpace(fmt.Sprint(e)))
		}
		return DVRule{Type: "list", List: list}, len(list) > 0
	case []string:
		return DVRule{Type: "list", List: x}, len(x) > 0
	case map[string]any:
		if _, ok := x["type"]; !ok {
			s, _ := x["src"].(string)
			return DVRule{Type: "list", Src: s}, s != ""
		}
		b, err := json.Marshal(x)
		if err != nil {
			return DVRule{}, false
		}
		var r DVRule
		if json.Unmarshal(b, &r) != nil || !dvTypes[r.Type] {
			return DVRule{}, false
		}
		return r, true
	}
	return DVRule{}, false
}

// dvMap encode une validation objet pour la couche de style.
func dvMap(r DVRule) map[string]any {
	b, _ := json.Marshal(r)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

// DVValues donne la liste (valeurs ou plage source) d'une validation par
// liste, quelle que soit sa forme.
func DVValues(dv any) (list []string, src string, ok bool) {
	r, ok := ParseDV(dv)
	if !ok || r.Type != "list" {
		return nil, "", false
	}
	return r.List, r.Src, true
}

// dvText lit une borne (nombre d'un script, texte) en texte.
func dvText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case float64:
		return formula.FormatNumber(x)
	case int:
		return fmt.Sprint(x)
	case int64:
		return fmt.Sprint(x)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func dvClip(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// normalizeDVRule vérifie et met en forme une validation objet : nil quand
// elle ne demande rien (Tout sans message).
func normalizeDVRule(m map[string]any) (any, error) {
	// Les bornes d'un script peuvent être des nombres.
	for _, k := range []string{"min", "max"} {
		if v, ok := m[k]; ok {
			m[k] = dvText(v)
		}
	}
	r, ok := ParseDV(m)
	if !ok {
		return nil, msg.New("validation.unknownType", msg.Args{"type": fmt.Sprint(m["type"])})
	}
	r.Min, r.Max, r.Formula, r.At = strings.TrimSpace(r.Min), strings.TrimSpace(r.Max), strings.TrimSpace(r.Formula), strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(r.At), "$", ""))
	for _, p := range []**DVMessage{&r.Input, &r.Error} {
		if *p == nil {
			continue
		}
		x := **p
		x.Title, x.Msg = dvClip(x.Title, dvTitleMax), dvClip(x.Msg, dvMsgMax)
		if x.Style != "warning" && x.Style != "info" {
			x.Style = ""
		}
		if p == &r.Input {
			x.Style = ""
		}
		if x == (DVMessage{}) {
			*p = nil
		} else {
			*p = &x
		}
	}
	if r.Blank != nil && *r.Blank {
		r.Blank = nil
	}
	switch r.Type {
	case "any":
		r.Op, r.Min, r.Max, r.Formula, r.List, r.Src, r.Blank, r.Error = "", "", "", "", nil, "", nil, nil
		if r.Input == nil {
			return nil, nil
		}
	case "list":
		r.Op, r.Min, r.Max, r.Formula = "", "", "", ""
		if strings.TrimSpace(r.Src) != "" {
			ref, err := NormalizeDVSource(r.Src)
			if err != nil {
				return nil, err
			}
			r.Src, r.List = ref, nil
		} else {
			r.List = parseDVList(r.List)
			if len(r.List) == 0 {
				return nil, msg.New("validation.emptyList", nil)
			}
		}
	case "custom":
		r.Op, r.Min, r.Max, r.List, r.Src = "", "", "", nil, ""
		if r.Formula == "" {
			return nil, msg.New("validation.emptyFormula", nil)
		}
		if !strings.HasPrefix(r.Formula, "=") {
			r.Formula = "=" + r.Formula
		}
		if _, err := formula.Parse(r.Formula, formula.FR); err != nil {
			return nil, ErrFormula{err}
		}
	default:
		r.Formula, r.List, r.Src = "", nil, ""
		if r.Op == "" {
			r.Op = "between"
		}
		if !dvOps[r.Op] {
			return nil, msg.New("validation.unknownOp", msg.Args{"op": r.Op})
		}
		if r.Op != "between" && r.Op != "notbetween" {
			r.Max = ""
		}
		for _, b := range []*string{&r.Min, &r.Max} {
			if b == &r.Max && r.Max == "" && r.Op != "between" && r.Op != "notbetween" {
				continue
			}
			if *b == "" {
				return nil, msg.New("validation.missingBound", nil)
			}
			if err := checkBound(r.Type, *b); err != nil {
				return nil, err
			}
		}
	}
	if r.At != "" {
		if _, _, _, ok := formula.A1Address(r.At); !ok || !dvRelative(r) {
			r.At = ""
		}
	}
	return dvMap(r), nil
}

// dvRelative : la validation a une formule (ou une borne en formule), qui
// a besoin de l'adresse de la cellule où elle est écrite.
func dvRelative(r DVRule) bool {
	return r.Formula != "" || strings.HasPrefix(r.Min, "=") || strings.HasPrefix(r.Max, "=")
}

func parseDVList(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v != "" && !seen[strings.ToLower(v)] {
			seen[strings.ToLower(v)] = true
			out = append(out, v)
		}
	}
	return out
}

// checkBound vérifie une borne constante d'après le type (une formule est
// seulement analysée).
func checkBound(typ, b string) error {
	if strings.HasPrefix(b, "=") {
		if _, err := formula.Parse(b, formula.FR); err != nil {
			return ErrFormula{err}
		}
		return nil
	}
	n := valueLiteral(b)
	ok := false
	switch v := n.(type) {
	case formula.NumLit:
		ok = typ != "whole" && typ != "length" || v.V == math.Trunc(v.V)
		if typ == "date" {
			ok = true
		}
	case formula.Call, formula.Binary:
		ok = typ == "date" || typ == "time"
	}
	if !ok {
		return msg.New("validation.badBound", msg.Args{"value": b})
	}
	return nil
}

// dvIn : les bornes et la formule d'une validation objet du correctif,
// tapées dans la langue de la session, dans la forme gardée (le correctif
// est copié).
func (u ui) dvIn(patch Style) (Style, error) {
	m, ok := patch["dv"].(map[string]any)
	if u.plain() || !ok || m["type"] == nil {
		return patch, nil
	}
	out := Style{}
	for k, v := range patch {
		out[k] = v
	}
	n := map[string]any{}
	for k, v := range m {
		n[k] = v
	}
	for _, k := range []string{"min", "max", "formula"} {
		s, ok := n[k].(string)
		if !ok || strings.TrimSpace(s) == "" {
			continue
		}
		s = strings.TrimSpace(s)
		if k == "formula" && !strings.HasPrefix(s, "=") {
			s = "=" + s
		}
		if strings.HasPrefix(s, "=") {
			f, err := TranslateA1(s, u.loc, formula.FR)
			if err != nil {
				return nil, ErrFormula{err}
			}
			n[k] = f
		} else {
			n[k] = u.valueIn(s)
		}
	}
	out["dv"] = n
	return out, nil
}

// dvOut : une validation gardée dans l'écriture de la session.
func (u ui) dvOut(r DVRule) DVRule {
	if u.plain() {
		return r
	}
	for _, p := range []*string{&r.Min, &r.Max, &r.Formula} {
		if strings.HasPrefix(*p, "=") {
			if out, err := TranslateA1(*p, formula.FR, u.loc); err == nil {
				*p = out
			}
		} else {
			*p = u.valueOut(*p)
		}
	}
	return r
}

var dvRangeMark = regexp.MustCompile(`lattisdvref(\d+)x\s*:\s*lattisdvref(\d+)x`)

// TranslateA1 est formula.Translate qui garde les adresses A1 telles
// qu'écrites ($ compris) : la formule d'une validation reste relative à
// sa cellule.
func TranslateA1(src string, from, to formula.Locale) (string, error) {
	if from == to {
		return src, nil
	}
	var refs []string
	marked := scanA1(src, func(colAbs bool, col int, rowAbs bool, row int) (string, bool) {
		a := ""
		if colAbs {
			a = "$"
		}
		a += formula.ColumnLetters(col)
		if rowAbs {
			a += "$"
		}
		refs = append(refs, a+strconv.Itoa(row+1))
		return fmt.Sprintf("lattisdvref%dx", len(refs)-1), true
	})
	// Plage A1:B2 : une seule marque.
	marked = dvRangeMark.ReplaceAllStringFunc(marked, func(m string) string {
		k := dvRangeMark.FindStringSubmatch(m)
		a, _ := strconv.Atoi(k[1])
		b, _ := strconv.Atoi(k[2])
		refs = append(refs, refs[a]+":"+refs[b])
		return fmt.Sprintf("lattisdvref%dx", len(refs)-1)
	})
	out, err := formula.Translate(marked, from, to)
	if err != nil {
		return src, err
	}
	for i := len(refs) - 1; i >= 0; i-- {
		out = strings.ReplaceAll(out, fmt.Sprintf("lattisdvref%dx", i), refs[i])
	}
	return out, nil
}

// ShiftFormula décale de dr lignes et dc colonnes les adresses A1
// relatives d'une formule (validation écrite pour une autre cellule).
func ShiftFormula(src string, dr, dc int) string { return shiftA1(src, dr, dc) }

// dvForTable : la validation d'un correctif posée sur une colonne de
// tableau tb (colonne c de la feuille) : les adresses de la formule et des
// bornes, écrites pour la cellule at, le sont pour la première ligne de
// données puis deviennent [@Colonne] ; at n'a plus de raison d'être.
func dvForTable(patch Style, tb *Table, c int) Style {
	m, ok := patch["dv"].(map[string]any)
	if !ok {
		return patch
	}
	r, ok := ParseDV(m)
	if !ok || r.At == "" {
		return patch
	}
	_, ar, ac, ok := formula.A1Address(r.At)
	if !ok {
		return patch
	}
	for _, p := range []*string{&r.Formula, &r.Min, &r.Max} {
		if strings.HasPrefix(*p, "=") {
			*p = rowRefs(shiftA1(*p, tb.Row+1-ar, c-ac), *tb)
		}
	}
	r.At = ""
	out := Style{}
	for k, v := range patch {
		out[k] = v
	}
	out["dv"] = dvMap(r)
	return out
}

// --- Vérification -------------------------------------------------------------

type entryCheckKey struct{}

// WithEntryCheck marque ctx : SetCell vérifie la validation des données de
// la cellule (saisie dans la grille) ; accept : l'utilisateur a répondu Oui
// (Avertissement) ou OK (Informations), la valeur passe.
func WithEntryCheck(ctx context.Context, accept bool) context.Context {
	return context.WithValue(ctx, entryCheckKey{}, !accept)
}

func entryCheck(ctx context.Context) bool {
	on, _ := ctx.Value(entryCheckKey{}).(bool)
	return on
}

// dvCell : une cellule vérifiée, sa validation, son tableau éventuel.
type dvCell struct {
	sheet    int64
	row, col int
	tb       *Table
	rid      int64
	rule     DVRule
}

// dvEnv : ce qu'il faut pour calculer les formules (lu une fois).
type dvEnv struct {
	schema *formula.Schema
	names  map[string]formula.Value
	hoist  func(formula.Compiled) formula.Value
	locs   map[int64]*sheetLocator
}

func (t *tx) dvEnv() (*dvEnv, error) {
	schema, err := t.schema()
	if err != nil {
		return nil, err
	}
	names, err := t.nameValuesWith(schema, true)
	if err != nil {
		return nil, err
	}
	hoist, _ := t.hoister()
	return &dvEnv{schema: schema, names: names, hoist: hoist, locs: map[int64]*sheetLocator{}}, nil
}

func (e *dvEnv) loc(t *tx, sheet int64) *sheetLocator {
	l := e.locs[sheet]
	if l == nil {
		l = t.locator(sheet)
		e.locs[sheet] = l
	}
	return l
}

// dvLayer lit la propriété dv d'une couche (présente : found ; null : rien).
func (t *tx) dvLayer(q string, args ...any) (any, bool, error) {
	var s string
	err := t.row(`SELECT coalesce((`+q+`), '')`, args...).Scan(&s)
	if err != nil {
		return nil, false, err
	}
	if !strings.Contains(s, `"dv"`) {
		return nil, false, nil
	}
	v, ok := parseStyle(s)["dv"]
	return v, ok, nil
}

// dvAt donne la validation de la cellule (row, col) de la feuille : la
// couche la plus proche l'emporte (colonne, puis ligne de la feuille,
// colonne puis cellule du tableau, ou cellule libre), comme la grille.
// L'en-tête d'un tableau n'en a pas.
func (t *tx) dvAt(sheet int64, row, col int, tables []Table) (*dvCell, error) {
	c := &dvCell{sheet: sheet, row: row, col: col}
	for i := range tables {
		tb := &tables[i]
		if col >= tb.Col && col <= tb.Right() && row >= tb.Top() && row <= tb.Bottom() {
			if row <= tb.Row {
				return nil, nil
			}
			ids, err := t.ridsAt(tb, row, row)
			if err != nil {
				return nil, err
			}
			if len(ids) == 0 {
				return nil, nil
			}
			c.tb, c.rid = tb, ids[0]
		}
	}
	type layer struct {
		q    string
		args []any
	}
	layers := []layer{
		{`SELECT style FROM _lattis_col_styles WHERE sheet = ? AND col = ?`, []any{sheet, col}},
		{`SELECT style FROM _lattis_row_styles WHERE sheet = ? AND row = ?`, []any{sheet, row}},
	}
	if c.tb != nil {
		colID := c.tb.Cols[col-c.tb.Col].ID
		layers = append(layers,
			layer{`SELECT style FROM _lattis_styles WHERE tbl = ? AND rid = 0 AND col = ?`, []any{c.tb.ID, colID}},
			layer{`SELECT style FROM _lattis_styles WHERE tbl = ? AND rid = ? AND col = ?`, []any{c.tb.ID, c.rid, colID}})
	} else {
		layers = append(layers, layer{`SELECT style FROM _lattis_cell_styles WHERE sheet = ? AND row = ? AND col = ?`, []any{sheet, row, col}})
	}
	var dv any
	for _, l := range layers {
		v, found, err := t.dvLayer(l.q, l.args...)
		if err != nil {
			return nil, err
		}
		if found {
			dv = v
		}
	}
	r, ok := ParseDV(dv)
	if !ok {
		return nil, nil
	}
	c.rule = r
	return c, nil
}

// dvFormula : le texte d'une formule de la validation pour cette cellule
// (adresses relatives décalées depuis at ; dans un tableau, [@Colonne]
// pour la ligne).
func (c *dvCell) dvFormula(src string) string {
	if _, ar, ac, ok := formula.A1Address(c.rule.At); ok {
		if c.tb != nil {
			return rowRefs(shiftA1(src, c.tb.Row+1-ar, c.col-ac), *c.tb)
		}
		return shiftA1(src, c.row-ar, c.col-ac)
	}
	if c.tb != nil {
		return rowRefs(src, *c.tb)
	}
	return src
}

// eval calcule une formule (« =… », forme gardée) ou une constante pour la
// cellule.
func (t *tx) dvEval(c *dvCell, src string, env *dvEnv) formula.Value {
	var n formula.Node
	if !strings.HasPrefix(src, "=") {
		n = valueLiteral(src)
		cc := formula.CompileHoisted(n, env.schema, env.names, nil, "", env.hoist)
		return t.dvScalar(`SELECT `+cc.SQL, cc.Args)
	}
	src = c.dvFormula(src)
	if c.tb != nil {
		n, err := t.prepareFormula(c.tb.Sheet, src)
		if err != nil {
			return formula.Err(formula.ErrName)
		}
		ft, ok := env.schema.Table(c.tb.Name)
		if !ok {
			return formula.Err(formula.ErrRef)
		}
		cc := formula.CompileHoistedFrom(n, env.schema, env.names, ft, c.tb.sql(), env.hoist, 0)
		return t.dvScalar(fmt.Sprintf(`SELECT %s FROM %s WHERE id = ?%d`, cc.SQL, c.tb.sql(), len(cc.Args)+1), append(cc.Args, c.rid))
	}
	n, err := formula.Parse(src, formula.FR)
	if err != nil {
		return formula.Err(formula.ErrName)
	}
	loc := env.loc(t, c.sheet)
	n, _, err = formula.Convert(n, env.schema, formula.A1Resolver(loc), src)
	if err == nil {
		n, err = formula.ConvertA1(n, loc)
	}
	if err != nil || loc.err != nil {
		loc.err = nil
		return formula.Err(formula.ErrName)
	}
	n = formula.QualifyCells(n, c.sheet)
	cc := formula.CompileHoisted(n, env.schema, env.names, nil, "", env.hoist)
	return t.dvScalar(`SELECT `+cc.SQL, cc.Args)
}

func (t *tx) dvScalar(q string, args []any) formula.Value {
	var v any
	if err := t.row(q, args...).Scan(&v); err != nil {
		return formula.Err(formula.ErrValue)
	}
	return formula.FromResult(v)
}

// dvValue : la valeur de la cellule.
func (t *tx) dvValue(c *dvCell, env *dvEnv) formula.Value {
	if c.tb == nil {
		return env.names[formula.CellKey(c.sheet, c.row, c.col)]
	}
	ft, ok := env.schema.Table(c.tb.Name)
	if !ok {
		return formula.Value{}
	}
	n := formula.Ref{Col: c.tb.Cols[c.col-c.tb.Col].Name, Row: true}
	cc := formula.CompileHoistedFrom(n, env.schema, env.names, ft, c.tb.sql(), env.hoist, 0)
	return t.dvScalar(fmt.Sprintf(`SELECT %s FROM %s WHERE id = ?%d`, cc.SQL, c.tb.sql(), len(cc.Args)+1), append(cc.Args, c.rid))
}

func blankValue(v formula.Value) bool {
	return v.Kind == formula.Empty || (v.Kind == formula.Text && strings.TrimSpace(v.Str) == "")
}

// dvVerdict : la valeur est-elle permise ? Sinon, la règle en clair (pour
// le message par défaut).
func (t *tx) dvVerdict(c *dvCell, v formula.Value, env *dvEnv) (bool, string) {
	r := c.rule
	if blankValue(v) {
		return r.ignoreBlank() || r.Type == "any", ""
	}
	switch r.Type {
	case "any":
		return true, ""
	case "list":
		vals, why := r.List, ""
		if r.Src != "" {
			vals, why = t.validationList(c.sheet, r.Src)
		}
		if why != "" {
			return false, why
		}
		s := strings.ToLower(strings.TrimSpace(v.String()))
		for _, x := range vals {
			if strings.ToLower(strings.TrimSpace(x)) == s {
				return true, ""
			}
		}
		shown := vals
		if len(shown) > 8 {
			shown = append(append([]string{}, shown[:8]...), "…")
		}
		return false, t.say("validation.rule.list", msg.Args{"list": strings.Join(shown, ", ")})
	case "custom":
		f := t.dvEval(c, r.Formula, env)
		return Truthy(f), t.say("validation.rule.custom", msg.Args{"formula": t.ui().formulaOut(c.dvFormula(r.Formula))})
	}
	// Nombre, date, heure, longueur : la valeur comparée.
	x, kindOK := 0.0, false
	switch r.Type {
	case "whole":
		kindOK = v.Kind == formula.Number && v.Num == math.Trunc(v.Num)
		x = v.Num
	case "decimal":
		kindOK, x = v.Kind == formula.Number, v.Num
	case "date", "time":
		kindOK, x = v.Kind == formula.Number || v.Kind == formula.Date, v.Num
	case "length":
		kindOK, x = !v.IsErr(), float64(utf8.RuneCountInString(v.String()))
	}
	lo, hi := t.dvEval(c, r.Min, env), formula.Value{}
	if r.Op == "between" || r.Op == "notbetween" {
		hi = t.dvEval(c, r.Max, env)
	}
	rule := t.dvRuleText(r, lo, hi)
	if !kindOK {
		return false, rule
	}
	// Borne vide (référence à une cellule vide) : ignorée si « Ignorer si
	// vide » est coché, comme un tableur.
	bound := func(b formula.Value) (float64, bool) {
		if b.Kind == formula.Number || b.Kind == formula.Date {
			return b.Num, true
		}
		return 0, false
	}
	if r.ignoreBlank() && (lo.IsEmpty() || ((r.Op == "between" || r.Op == "notbetween") && hi.IsEmpty())) {
		return true, ""
	}
	a, ok := bound(lo)
	if !ok {
		return false, rule
	}
	b := 0.0
	if r.Op == "between" || r.Op == "notbetween" {
		if b, ok = bound(hi); !ok {
			return false, rule
		}
	}
	const eps = 1e-9 // heures : fractions de jour
	switch r.Op {
	case "between":
		return x >= a-eps && x <= b+eps, rule
	case "notbetween":
		return x < a-eps || x > b+eps, rule
	case "=":
		return math.Abs(x-a) <= eps, rule
	case "<>":
		return math.Abs(x-a) > eps, rule
	case ">":
		return x > a+eps, rule
	case "<":
		return x < a-eps, rule
	case ">=":
		return x >= a-eps, rule
	case "<=":
		return x <= a+eps, rule
	}
	return true, ""
}

// dvRuleText : la règle en clair (« nombre entier entre 1 et 10 »).
func (t *tx) dvRuleText(r DVRule, lo, hi formula.Value) string {
	u := t.ui()
	show := func(v formula.Value, typ string) string {
		switch {
		case v.Kind == formula.Number && typ == "time":
			return formatTimeOfDay(v.Num)
		case v.Kind == formula.Number && typ == "date":
			return u.valueOut(formula.DateSerial(v.Num).String())
		}
		return u.result(v.String())
	}
	kind := t.say(msg.ID("validation.kind."+r.Type), nil)
	cond := t.say(msg.ID("validation.op."+dvOpNames[r.Op]), msg.Args{"min": show(lo, r.Type), "max": show(hi, r.Type)})
	return kind + " " + cond
}

// formatTimeOfDay écrit une fraction de jour hh:mm (hh:mm:ss au besoin).
func formatTimeOfDay(f float64) string {
	s := int(math.Round((f - math.Floor(f)) * 86400))
	h, m, sec := s/3600, s/60%60, s%60
	if sec != 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%02d:%02d", h, m)
}

// checkEntry vérifie la cellule saisie (dans la transaction de la saisie) :
// une erreur validation.refusedEntry (style, titre, texte) si sa valeur
// n'est pas permise.
func (t *tx) checkEntry(sheet int64, row, col int) error {
	tables, err := t.tables(sheet)
	if err != nil {
		return err
	}
	c, err := t.dvAt(sheet, row, col, tables)
	if err != nil || c == nil {
		return err
	}
	env, err := t.dvEnv()
	if err != nil {
		return err
	}
	v := t.dvValue(c, env)
	ok, rule := t.dvVerdict(c, v, env)
	if ok {
		return nil
	}
	shown := t.ui().result(v.String())
	text := ""
	title := ""
	if e := c.rule.Error; e != nil {
		text, title = e.Msg, e.Title
	}
	if text == "" && blankValue(v) {
		text = t.say("validation.invalidBlank", nil)
	} else if text == "" {
		text = t.say("validation.invalid", msg.Args{"value": shown, "rule": rule})
	}
	return msg.New("validation.refusedEntry", msg.Args{"text": text, "title": title, "style": c.rule.errorStyle(), "value": shown})
}

// --- Entourer les données non valides -------------------------------------------

// CellPos : une cellule de la feuille.
type CellPos struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// maxDVChecks borne les cellules vérifiées par InvalidCells.
const maxDVChecks = 20000

// InvalidCells donne les cellules non vides d'une feuille dont la valeur ne
// respecte pas leur validation (Entourer les données non valides) ; au plus
// maxDVChecks cellules vérifiées.
func (m *Model) InvalidCells(ctx context.Context, sheet int64) ([]CellPos, error) {
	out := []CellPos{}
	err := m.read(ctx, func(t *tx) error {
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		var env *dvEnv
		seen := map[[2]int]bool{}
		checks := 0
		check := func(row, col int) error {
			p := [2]int{row, col}
			if seen[p] || checks >= maxDVChecks {
				return nil
			}
			seen[p] = true
			checks++
			c, err := t.dvAt(sheet, row, col, tables)
			if err != nil || c == nil {
				return err
			}
			if env == nil {
				if env, err = t.dvEnv(); err != nil {
					return err
				}
			}
			v := t.dvValue(c, env)
			if blankValue(v) {
				return nil
			}
			if ok, _ := t.dvVerdict(c, v, env); !ok {
				out = append(out, CellPos{row, col})
			}
			return nil
		}
		// Cellules libres : celles qui ont une validation, et les valeurs des
		// lignes et colonnes entières qui en ont une.
		pos, err := t.dvPositions(`SELECT row, col FROM _lattis_cell_styles WHERE sheet = ? AND style LIKE '%"dv"%'
			UNION SELECT c.row, c.col FROM _lattis_cells c WHERE c.sheet = ? AND (
				EXISTS (SELECT 1 FROM _lattis_row_styles r WHERE r.sheet = c.sheet AND r.row = c.row AND r.style LIKE '%"dv"%')
				OR EXISTS (SELECT 1 FROM _lattis_col_styles k WHERE k.sheet = c.sheet AND k.col = c.col AND k.style LIKE '%"dv"%'))
			ORDER BY 1, 2`, sheet, sheet)
		if err != nil {
			return err
		}
		for _, p := range pos {
			if !inTable(tables, p[0], p[1]) {
				if err := check(p[0], p[1]); err != nil {
					return err
				}
			}
		}
		// Tableaux : les colonnes dont une couche porte une validation.
		lineDV := func(table, key string, i int) bool {
			var n int
			_ = t.row(`SELECT count(*) FROM `+table+` WHERE sheet = ? AND `+key+` = ? AND style LIKE '%"dv"%'`, sheet, i).Scan(&n)
			return n > 0
		}
		for i := range tables {
			tb := &tables[i]
			var rows int
			if err := t.row(`SELECT count(*) FROM ` + tb.view()).Scan(&rows); err != nil {
				return err
			}
			for k, col := range tb.Cols {
				var n int
				if err := t.row(`SELECT count(*) FROM _lattis_styles WHERE tbl = ? AND col = ? AND rid >= 0 AND style LIKE '%"dv"%'`, tb.ID, col.ID).Scan(&n); err != nil {
					return err
				}
				anyRow := false
				for r := tb.Row + 1; r <= tb.Row+rows && !anyRow && r-tb.Row < 2000; r++ {
					anyRow = lineDV("_lattis_row_styles", "row", r)
				}
				if n == 0 && !anyRow && !lineDV("_lattis_col_styles", "col", tb.Col+k) {
					continue
				}
				for r := 0; r < rows && checks < maxDVChecks; r++ {
					if err := check(tb.Row+1+r, tb.Col+k); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	return out, err
}

func (t *tx) dvPositions(q string, args ...any) ([][2]int, error) {
	rs, err := t.query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out [][2]int
	for rs.Next() {
		var p [2]int
		if err := rs.Scan(&p[0], &p[1]); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rs.Err()
}

// ValidationAt donne la validation de la cellule (row, col) dans
// l'écriture de la session (dialogue Validation des données) ; Type vide :
// aucune.
func (m *Model) ValidationAt(ctx context.Context, sheet int64, row, col int) (DVRule, error) {
	var out DVRule
	u := m.ui(ctx)
	err := m.read(ctx, func(t *tx) error {
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		c, err := t.dvAt(sheet, row, col, tables)
		if err != nil || c == nil {
			return err
		}
		r := c.rule
		// Formule montrée pour la cellule active, comme un tableur.
		if r.At != "" {
			for _, p := range []*string{&r.Formula, &r.Min, &r.Max} {
				if strings.HasPrefix(*p, "=") && c.tb == nil {
					*p = c.dvFormula(*p)
				}
			}
			if c.tb == nil {
				r.At = formula.ColumnLetters(col) + fmt.Sprint(row+1)
			}
		}
		out = u.dvOut(r)
		return nil
	})
	return out, err
}
