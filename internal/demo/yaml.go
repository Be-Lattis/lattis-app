package demo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/lalamefine/lattis/internal/canvas"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// Plans de démonstration écrits en YAML (lattis --demo plan.yaml) : la
// même description que les presets écrits en Go (Plan), lue d'un fichier.
// Format : docs/demo.md ; exemple : docs/demo/exemple.yaml.

type yPlan struct {
	Seed    uint64    `yaml:"seed"`
	Sheets  []ySheet  `yaml:"sheets"`
	Scripts []yScript `yaml:"scripts"`
	Events  []yEvent  `yaml:"events"`
}

type yScript struct {
	Name   string `yaml:"name"`
	Source string `yaml:"source"`
}

type yEvent struct {
	Event    string `yaml:"event"`
	Script   string `yaml:"script"`
	Function string `yaml:"function"`
}

type ySheet struct {
	Name     string           `yaml:"name"`
	Tables   []yTable         `yaml:"tables"`
	Cells    []yCell          `yaml:"cells"`
	Pivots   []map[string]any `yaml:"pivots"`
	Charts   []map[string]any `yaml:"charts"`
	Controls []map[string]any `yaml:"controls"`
	Rules    []yRule          `yaml:"rules"`
	Styles   []yStyle         `yaml:"styles"`
	Widths   map[string]int   `yaml:"widths"`
	Query    *yQuery          `yaml:"query"`
	Doc      []map[string]any `yaml:"doc"`
	HTML     string           `yaml:"html"`
}

type yTable struct {
	Name    string    `yaml:"name"`
	Cell    string    `yaml:"cell"`
	Rows    int       `yaml:"rows"`
	Key     any       `yaml:"key"`
	Columns []yColumn `yaml:"columns"`
}

type yColumn struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Formula  string   `yaml:"formula"`
	Gen      string   `yaml:"gen"`
	Of       string   `yaml:"of"`
	Values   []any    `yaml:"values"`
	Value    any      `yaml:"value"`
	Min      *float64 `yaml:"min"`
	Max      *float64 `yaml:"max"`
	Decimals *int     `yaml:"decimals"`
	Start    *float64 `yaml:"start"`
	Step     *float64 `yaml:"step"`
	From     string   `yaml:"from"`
	To       string   `yaml:"to"`
	Ordered  bool     `yaml:"ordered"`
	Skew     bool     `yaml:"skew"`
	P        *float64 `yaml:"p"`
	Prefix   string   `yaml:"prefix"`
	Digits   int      `yaml:"digits"`
	Domain   string   `yaml:"domain"`
	Blank    float64  `yaml:"blank"`
	Pattern  string   `yaml:"pattern"` // gen: regex
}

type yCell struct {
	Cell  string `yaml:"cell"`
	Value any    `yaml:"value"`
}

type yRule struct {
	Table  string           `yaml:"table"`
	Column string           `yaml:"column"`
	Rules  []map[string]any `yaml:"rules"`
}

type yStyle struct {
	Range string         `yaml:"range"`
	Style map[string]any `yaml:"style"`
}

type yQuery struct {
	SQL    string           `yaml:"sql"`
	Source map[string]any   `yaml:"source"`
	Steps  []map[string]any `yaml:"steps"`
	Table  string           `yaml:"table"`
}

// yfile retrouve la ligne d'un élément du fichier pour les messages.
type yfile struct {
	name string
	root *yaml.Node
}

// at donne « fichier:ligne » de l'élément désigné par path (clés et rangs) ;
// à défaut, du plus proche parent trouvé.
func (f *yfile) at(path ...any) string {
	n := f.root
	if n != nil && n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	line := 1
	for _, p := range path {
		if n == nil {
			break
		}
		if n.Line > 0 {
			line = n.Line
		}
		var next *yaml.Node
		switch p := p.(type) {
		case string:
			if n.Kind == yaml.MappingNode {
				for i := 0; i+1 < len(n.Content); i += 2 {
					if n.Content[i].Value == p {
						next = n.Content[i+1]
						if next.Line == 0 || next.Kind != yaml.ScalarNode {
							line = n.Content[i].Line
						}
					}
				}
			}
		case int:
			if n.Kind == yaml.SequenceNode && p < len(n.Content) {
				next = n.Content[p]
			}
		}
		n = next
	}
	if n != nil && n.Line > 0 {
		line = n.Line
	}
	return f.name + ":" + strconv.Itoa(line)
}

func (f *yfile) errf(path []any, format string, args ...any) error {
	return fmt.Errorf("%s : %s", f.at(path...), fmt.Sprintf(format, args...))
}

func pathOf(base []any, more ...any) []any {
	return append(append([]any(nil), base...), more...)
}

var (
	reUnknownField = regexp.MustCompile(`field (\S+) not found in type \S+`)
	reCannot       = regexp.MustCompile("cannot unmarshal !!(\\w+) `([^`]*)` into (\\S+)")
	reJSONUnknown  = regexp.MustCompile(`json: unknown field "([^"]+)"`)
	reJSONType     = regexp.MustCompile(`json: cannot unmarshal (\w+) into Go (?:struct field|value) (?:\S+\.)?(\S*) of type (\S+)`)
)

// yamlError traduit une erreur de lecture YAML, avec fichier et ligne.
func yamlError(name string, err error) error {
	var te *yaml.TypeError
	if errors.As(err, &te) {
		var msgs []string
		for _, e := range te.Errors {
			msg := e.Err.Error()
			if m := reUnknownField.FindStringSubmatch(msg); m != nil {
				msg = "clé « " + m[1] + " » inconnue"
			} else if m := reCannot.FindStringSubmatch(msg); m != nil {
				msg = fmt.Sprintf("valeur « %s » invalide ici (%s attendu)", m[2], goType(m[3]))
			}
			msgs = append(msgs, fmt.Sprintf("%s:%d : %s", name, e.Line, msg))
		}
		return errors.New(strings.Join(msgs, "\n"))
	}
	var pe *yaml.ParserError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s:%d : YAML illisible : %s", name, pe.Line, pe.Message)
	}
	return fmt.Errorf("%s : YAML illisible : %v", name, err)
}

func goType(t string) string {
	switch {
	case strings.Contains(t, "int"), strings.Contains(t, "float"):
		return "nombre"
	case t == "bool":
		return "vrai/faux (true, false)"
	case t == "string":
		return "texte"
	case strings.HasPrefix(t, "[]"):
		return "liste"
	case strings.HasPrefix(t, "map"), strings.HasPrefix(t, "demo."):
		return "objet (clé: valeur)"
	}
	return t
}

// decodeJSON convertit un objet YAML en une structure de l'application
// (mêmes noms de clés que son JSON), en refusant les clés inconnues.
func (f *yfile) decodeJSON(path []any, in any, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return f.errf(path, "%v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		msg := err.Error()
		if m := reJSONUnknown.FindStringSubmatch(msg); m != nil {
			return f.errf(pathOf(path, m[1]), "clé « %s » inconnue", m[1])
		}
		if m := reJSONType.FindStringSubmatch(msg); m != nil {
			return f.errf(pathOf(path, m[2]), "« %s » : valeur invalide (%s attendu)", m[2], goType(m[3]))
		}
		return f.errf(path, "%s", msg)
	}
	return nil
}

// LoadFile lit un plan de démonstration YAML.
func LoadFile(path string) (Plan, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Plan{}, fmt.Errorf("fichier de démonstration illisible : %w", err)
	}
	return ParsePlan(filepath.Base(path), b)
}

// ParsePlan lit un plan de démonstration YAML ; name nomme le fichier dans
// les messages d'erreur.
func ParsePlan(name string, b []byte) (Plan, error) {
	var y yPlan
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&y); err != nil {
		if errors.Is(err, io.EOF) {
			return Plan{}, fmt.Errorf("%s : fichier vide", name)
		}
		return Plan{}, yamlError(name, err)
	}
	f := &yfile{name: name, root: &yaml.Node{}}
	if err := yaml.Unmarshal(b, f.root); err != nil {
		return Plan{}, yamlError(name, err)
	}
	if len(y.Sheets) == 0 {
		return Plan{}, f.errf(nil, "aucun onglet (sheets)")
	}
	p := Plan{Seed: y.Seed}
	names := map[string]bool{}
	for i, ys := range y.Sheets {
		path := []any{"sheets", i}
		s, err := f.sheet(path, ys)
		if err != nil {
			return Plan{}, err
		}
		if names[strings.ToLower(s.Name)] {
			return Plan{}, f.errf(pathOf(path, "name"), "onglet « %s » en double", s.Name)
		}
		names[strings.ToLower(s.Name)] = true
		p.Sheets = append(p.Sheets, s)
	}
	for i, s := range y.Scripts {
		if strings.TrimSpace(s.Name) == "" || strings.TrimSpace(s.Source) == "" {
			return Plan{}, f.errf([]any{"scripts", i}, "un script a un nom (name) et un code (source)")
		}
		p.Scripts = append(p.Scripts, ScriptSpec{Name: s.Name, Source: s.Source})
	}
	for i, e := range y.Events {
		if e.Event == "" || e.Script == "" || e.Function == "" {
			return Plan{}, f.errf([]any{"events", i}, "un écouteur a un évènement (event), un script et une fonction (function)")
		}
		p.Events = append(p.Events, canvas.Listener{Event: e.Event, Script: e.Script, Function: e.Function})
	}
	return p, nil
}

// a1 lit une adresse « B3 » (ligne et colonne à partir de 0).
func a1(s string) (row, col int, ok bool) {
	return formula.CellRef("Cell_" + strings.ToUpper(strings.TrimSpace(s)))
}

// colIndex lit une lettre de colonne (« A », « AB »).
func colIndex(s string) (int, bool) {
	_, c, ok := a1(s + "1")
	return c, ok
}

// takeCell retire la clé « cell » d'un objet et la lit (A1 par défaut).
func (f *yfile) takeCell(path []any, m map[string]any) (row, col int, err error) {
	v, ok := m["cell"]
	if !ok {
		return 0, 0, nil
	}
	delete(m, "cell")
	s, _ := v.(string)
	r, c, good := a1(s)
	if !good {
		return 0, 0, f.errf(pathOf(path, "cell"), "adresse « %v » invalide (« B3 » par exemple)", v)
	}
	return r, c, nil
}

func scalar(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		if v {
			return "vrai"
		}
		return "faux"
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

func (f *yfile) sheet(path []any, ys ySheet) (SheetSpec, error) {
	s := SheetSpec{Name: strings.TrimSpace(ys.Name), HTML: ys.HTML}
	if s.Name == "" {
		return s, f.errf(path, "onglet sans nom (name)")
	}
	kinds := 0
	for _, b := range []bool{len(ys.Tables) > 0 || len(ys.Cells) > 0, ys.Query != nil, ys.Doc != nil, ys.HTML != ""} {
		if b {
			kinds++
		}
	}
	if kinds > 1 {
		return s, f.errf(path, "onglet « %s » : choisissez un seul contenu (tables/cells, query, doc ou html)", s.Name)
	}
	for j, yt := range ys.Tables {
		t, err := f.table(pathOf(path, "tables", j), yt)
		if err != nil {
			return s, err
		}
		s.Tables = append(s.Tables, t)
	}
	for j, yc := range ys.Cells {
		r, c, ok := a1(yc.Cell)
		if !ok {
			return s, f.errf(pathOf(path, "cells", j), "adresse « %s » invalide (« B3 » par exemple)", yc.Cell)
		}
		s.Cells = append(s.Cells, canvas.ImportCell{Row: r, Col: c, Raw: scalar(yc.Value)})
	}
	for j, m := range ys.Pivots {
		pp := pathOf(path, "pivots", j)
		r, c, err := f.takeCell(pp, m)
		if err != nil {
			return s, err
		}
		var cfg canvas.PivotConfig
		if err := f.decodeJSON(pp, m, &cfg); err != nil {
			return s, err
		}
		s.Pivots = append(s.Pivots, PivotPlacement{Row: r, Col: c, Config: cfg})
	}
	for j, m := range ys.Charts {
		pp := pathOf(path, "charts", j)
		r, c, err := f.takeCell(pp, m)
		if err != nil {
			return s, err
		}
		var cfg canvas.ChartConfig
		if err := f.decodeJSON(pp, m, &cfg); err != nil {
			return s, err
		}
		s.Charts = append(s.Charts, ChartSpec{Row: r, Col: c, Config: cfg})
	}
	for j, m := range ys.Controls {
		pp := pathOf(path, "controls", j)
		r, c, err := f.takeCell(pp, m)
		if err != nil {
			return s, err
		}
		var cfg canvas.ButtonConfig
		if err := f.decodeJSON(pp, m, &cfg); err != nil {
			return s, err
		}
		s.Controls = append(s.Controls, ControlSpec{Row: r, Col: c, Config: cfg})
	}
	for j, yr := range ys.Rules {
		pp := pathOf(path, "rules", j)
		if yr.Table == "" || yr.Column == "" {
			return s, f.errf(pp, "une règle désigne un tableau (table) et une colonne (column)")
		}
		var rules []canvas.Rule
		if err := f.decodeJSON(pathOf(pp, "rules"), yr.Rules, &rules); err != nil {
			return s, err
		}
		s.Rules = append(s.Rules, RuleSpec{Table: yr.Table, Column: yr.Column, Rules: rules})
	}
	for j, st := range ys.Styles {
		pp := pathOf(path, "styles", j)
		from, to, _ := strings.Cut(st.Range, ":")
		if to == "" {
			to = from
		}
		r0, c0, ok0 := a1(from)
		r1, c1, ok1 := a1(to)
		if !ok0 || !ok1 {
			return s, f.errf(pathOf(pp, "range"), "plage « %s » invalide (« B3 » ou « B3:D5 »)", st.Range)
		}
		s.Styles = append(s.Styles, StyleSpec{R0: min(r0, r1), C0: min(c0, c1), R1: max(r0, r1), C1: max(c0, c1), Style: canvas.Style(st.Style)})
	}
	for col, w := range ys.Widths {
		c, ok := colIndex(col)
		if !ok || w < 0 {
			return s, f.errf(pathOf(path, "widths", col), "largeur de colonne « %s » invalide", col)
		}
		s.Widths = append(s.Widths, WidthSpec{Col: c, Width: w})
	}
	if q := ys.Query; q != nil {
		pp := pathOf(path, "query")
		qs := &QuerySpec{SQL: q.SQL, Table: q.Table}
		if (q.SQL == "") == (q.Source == nil) {
			return s, f.errf(pp, "une requête a soit du SQL (sql), soit une source (source)")
		}
		if q.Source != nil {
			if err := f.decodeJSON(pathOf(pp, "source"), q.Source, &qs.Source); err != nil {
				return s, err
			}
		}
		if q.Steps != nil {
			if err := f.decodeJSON(pathOf(pp, "steps"), q.Steps, &qs.Steps); err != nil {
				return s, err
			}
		}
		s.Query = qs
	}
	if ys.Doc != nil {
		s.Doc = []DocBlock{}
		for j, m := range ys.Doc {
			b, err := f.docBlock(pathOf(path, "doc", j), m)
			if err != nil {
				return s, err
			}
			s.Doc = append(s.Doc, b)
		}
	}
	return s, nil
}

func (f *yfile) docBlock(path []any, m map[string]any) (DocBlock, error) {
	var b DocBlock
	for k, v := range m {
		switch k {
		case "h1", "h2", "h3", "p", "bullet", "number", "quote":
			if b.Kind != "" {
				return b, f.errf(path, "un bloc n'a qu'un genre (%s ou %s)", b.Kind, k)
			}
			b.Kind, b.Text = k, scalar(v)
		case "chart":
			if b.Kind != "" {
				return b, f.errf(path, "un bloc n'a qu'un genre (%s ou %s)", b.Kind, k)
			}
			b.Kind, b.Chart = k, &canvas.ChartConfig{}
			if err := f.decodeJSON(pathOf(path, k), v, b.Chart); err != nil {
				return b, err
			}
		case "pivot":
			if b.Kind != "" {
				return b, f.errf(path, "un bloc n'a qu'un genre (%s ou %s)", b.Kind, k)
			}
			b.Kind, b.Pivot = k, &canvas.PivotConfig{}
			if err := f.decodeJSON(pathOf(path, k), v, b.Pivot); err != nil {
				return b, err
			}
		case "button":
			b.Button = &DocButton{}
			if err := f.decodeJSON(pathOf(path, k), v, b.Button); err != nil {
				return b, err
			}
		case "control":
			b.Control = &canvas.ButtonConfig{}
			if err := f.decodeJSON(pathOf(path, k), v, b.Control); err != nil {
				return b, err
			}
		default:
			return b, f.errf(pathOf(path, k), "bloc « %s » inconnu (h1, h2, h3, p, bullet, number, quote, chart, pivot ; button, control)", k)
		}
	}
	if b.Kind == "" {
		b.Kind = "p" // un bouton ou un champ seul sur sa ligne
	}
	return b, nil
}

// Types des colonnes, en français ou en anglais.
var yTypes = map[string]grid.Type{
	"integer": grid.Integer, "entier": grid.Integer, "decimal": grid.Decimal, "décimal": grid.Decimal, "nombre": grid.Decimal,
	"text": grid.Text, "texte": grid.Text, "boolean": grid.Boolean, "booléen": grid.Boolean, "date": grid.Date,
}

// Générateurs de valeurs et type par défaut de leur colonne.
var yGens = map[string]grid.Type{
	"sequence": grid.Integer, "code": grid.Text, "int": grid.Integer, "decimal": grid.Decimal, "date": grid.Date,
	"bool": grid.Boolean, "pick": grid.Text, "cycle": grid.Text, "constant": grid.Text, "regex": grid.Text,
	"firstname": grid.Text, "lastname": grid.Text, "person": grid.Text, "company": grid.Text,
	"email": grid.Text, "phone": grid.Text,
	"city": grid.Text, "postcode": grid.Text, "region": grid.Text,
	"latitude": grid.Decimal, "longitude": grid.Decimal, "population": grid.Integer,
	"product": grid.Text, "category": grid.Text, "price": grid.Decimal,
}

// genNames : liste des générateurs pour les messages.
const genNames = "sequence, code, int, decimal, date, bool, pick, cycle, constant, regex, firstname, lastname, person, company, email, phone, city, postcode, region, latitude, longitude, population, product, category, price"

func (f *yfile) table(path []any, yt yTable) (TableSpec, error) {
	t := TableSpec{Name: strings.TrimSpace(yt.Name), Rows: yt.Rows}
	if t.Name == "" {
		return t, f.errf(path, "tableau sans nom (name)")
	}
	if yt.Rows < 0 || yt.Rows > 10_000_000 {
		return t, f.errf(pathOf(path, "rows"), "nombre de lignes entre 0 et 10 000 000")
	}
	if yt.Cell != "" {
		r, c, ok := a1(yt.Cell)
		if !ok {
			return t, f.errf(pathOf(path, "cell"), "adresse « %s » invalide (« B3 » par exemple)", yt.Cell)
		}
		t.Row, t.Col = r, c
	}
	if len(yt.Columns) == 0 {
		return t, f.errf(path, "tableau « %s » sans colonne (columns)", t.Name)
	}
	index := map[string]int{}
	for k, yc := range yt.Columns {
		cp := pathOf(path, "columns", k)
		col, err := f.column(cp, yc, index, t.Rows)
		if err != nil {
			return t, err
		}
		if _, dup := index[strings.ToLower(col.Name)]; dup {
			return t, f.errf(pathOf(cp, "name"), "colonne « %s » en double", col.Name)
		}
		index[strings.ToLower(col.Name)] = k
		t.Cols = append(t.Cols, col)
	}
	switch k := yt.Key.(type) {
	case nil:
	case int:
		if k < 1 || k > len(t.Cols) {
			return t, f.errf(pathOf(path, "key"), "clé : rang de colonne entre 1 et %d", len(t.Cols))
		}
		t.Key = k
	case string:
		i, ok := index[strings.ToLower(strings.TrimSpace(k))]
		if !ok {
			return t, f.errf(pathOf(path, "key"), "clé : colonne « %s » introuvable", k)
		}
		t.Key = i + 1
	default:
		return t, f.errf(pathOf(path, "key"), "clé : nom ou rang de colonne attendu")
	}
	return t, nil
}

func (f *yfile) column(path []any, yc yColumn, index map[string]int, rows int) (Column, error) {
	col := Column{Name: strings.TrimSpace(yc.Name)}
	if col.Name == "" {
		return col, f.errf(path, "colonne sans nom (name)")
	}
	if yc.Type != "" {
		t, ok := yTypes[strings.ToLower(yc.Type)]
		if !ok {
			return col, f.errf(pathOf(path, "type"), "type « %s » inconnu (integer, decimal, text, boolean, date)", yc.Type)
		}
		col.Type = t
	}
	if yc.Formula != "" {
		if yc.Gen != "" {
			return col, f.errf(path, "colonne « %s » : une formule ou un générateur (gen), pas les deux", col.Name)
		}
		col.Formula = yc.Formula
		if !strings.HasPrefix(col.Formula, "=") {
			col.Formula = "=" + col.Formula
		}
		if col.Type == "" {
			col.Type = grid.Decimal
		}
		return col, nil
	}
	if yc.Gen == "" {
		return col, f.errf(path, "colonne « %s » : générateur (gen) ou formule (formula) attendu", col.Name)
	}
	def, ok := yGens[yc.Gen]
	if !ok {
		return col, f.errf(pathOf(path, "gen"), "générateur « %s » inconnu ; générateurs : %s", yc.Gen, genNames)
	}
	if col.Type == "" {
		col.Type = def
	}
	of := -1
	if yc.Of != "" {
		i, ok := index[strings.ToLower(strings.TrimSpace(yc.Of))]
		if !ok {
			return col, f.errf(pathOf(path, "of"), "colonne « %s » introuvable parmi les colonnes précédentes", yc.Of)
		}
		of = i
	}
	v, err := f.generator(path, yc, of, rows)
	if err != nil {
		return col, err
	}
	if yc.Blank < 0 || yc.Blank > 1 {
		return col, f.errf(pathOf(path, "blank"), "blank : proportion de cellules vides entre 0 et 1")
	}
	if yc.Blank > 0 {
		inner, p := v, yc.Blank
		v = func(g *Gen, i int, r []any) any {
			if g.Chance(p) {
				return nil
			}
			return inner(g, i, r)
		}
	}
	col.Value = v
	return col, nil
}

func num(p *float64, d float64) float64 {
	if p == nil {
		return d
	}
	return *p
}

// pickIndex tire un rang dans [0, n[ (les premiers plus souvent avec skew).
func pickIndex(g *Gen, n int, skew bool) int {
	if skew {
		return g.Skewed(n)
	}
	return g.Int(0, n-1)
}

func (f *yfile) generator(path []any, yc yColumn, of, rows int) (func(g *Gen, i int, r []any) any, error) {
	lo, hi := num(yc.Min, 0), num(yc.Max, 100)
	if lo > hi {
		return nil, f.errf(path, "colonne « %s » : min plus grand que max", yc.Name)
	}
	needOf := func(what string) error {
		if of < 0 {
			return f.errf(path, "colonne « %s » : %s demande la colonne d'où il est tiré (of)", yc.Name, what)
		}
		return nil
	}
	cityOf := func(g *Gen, r []any) int {
		if of >= 0 {
			if k, ok := villeOf[str(r[of])]; ok {
				return k
			}
		}
		return pickIndex(g, len(villes), yc.Skew)
	}
	productOf := func(g *Gen, r []any) int {
		if of >= 0 {
			if k, ok := produitOf[str(r[of])]; ok {
				return k
			}
		}
		return pickIndex(g, len(produits), yc.Skew)
	}
	switch yc.Gen {
	case "sequence":
		start, step := num(yc.Start, 1), num(yc.Step, 1)
		return func(g *Gen, i int, _ []any) any { return int64(start + step*float64(i)) }, nil
	case "code":
		start, digits := int(num(yc.Start, 1)), yc.Digits
		if digits <= 0 {
			digits = 4
		}
		return func(g *Gen, i int, _ []any) any { return fmt.Sprintf("%s%0*d", yc.Prefix, digits, start+i) }, nil
	case "int":
		a, b := int(math.Round(lo)), int(math.Round(hi))
		return func(g *Gen, i int, _ []any) any {
			if yc.Skew {
				return int64(a + g.Skewed(b-a+1))
			}
			return int64(g.Int(a, b))
		}, nil
	case "decimal":
		d := 2
		if yc.Decimals != nil {
			d = *yc.Decimals
		}
		return func(g *Gen, i int, _ []any) any {
			if yc.Skew {
				u := float64(g.Skewed(10_000)) / 10_000
				return Round(lo+u*(hi-lo), d)
			}
			return Round(g.Float(lo, hi), d)
		}, nil
	case "date":
		from, to := today-365, today-1
		var err error
		if yc.From != "" {
			if from, err = dayArg(yc.From); err != nil {
				return nil, f.errf(pathOf(path, "from"), "date « %s » invalide (aaaa-mm-jj)", yc.From)
			}
		}
		if yc.To != "" {
			if to, err = dayArg(yc.To); err != nil {
				return nil, f.errf(pathOf(path, "to"), "date « %s » invalide (aaaa-mm-jj)", yc.To)
			}
		}
		if from > to {
			return nil, f.errf(path, "colonne « %s » : from après to", yc.Name)
		}
		if yc.Ordered { // dates croissantes, de from à to
			n := max(rows-1, 1)
			return func(g *Gen, i int, _ []any) any { return Day(from + (to-from)*i/n) }, nil
		}
		return func(g *Gen, i int, _ []any) any { return g.Date(from, to) }, nil
	case "bool":
		p := num(yc.P, 0.5)
		return func(g *Gen, i int, _ []any) any { return Bool(g.Chance(p)) }, nil
	case "pick":
		if len(yc.Values) == 0 {
			return nil, f.errf(path, "colonne « %s » : pick demande des valeurs (values)", yc.Name)
		}
		vals := make([]any, len(yc.Values))
		for k, v := range yc.Values {
			vals[k] = plainValue(v)
		}
		return func(g *Gen, i int, _ []any) any { return vals[pickIndex(g, len(vals), yc.Skew)] }, nil
	case "cycle":
		if len(yc.Values) == 0 {
			return nil, f.errf(path, "colonne « %s » : cycle demande des valeurs (values)", yc.Name)
		}
		vals := make([]any, len(yc.Values))
		for k, v := range yc.Values {
			vals[k] = plainValue(v)
		}
		return func(g *Gen, i int, _ []any) any { return vals[i%len(vals)] }, nil
	case "regex":
		if yc.Pattern == "" {
			return nil, f.errf(path, "colonne « %s » : regex demande une expression (pattern)", yc.Name)
		}
		re, err := compileRegexGen(yc.Pattern)
		if err != nil {
			return nil, f.errf(pathOf(path, "pattern"), "colonne « %s » : %v", yc.Name, err)
		}
		return func(g *Gen, i int, _ []any) any { return g.Regex(re) }, nil
	case "constant":
		v := plainValue(yc.Value)
		return func(g *Gen, i int, _ []any) any { return v }, nil
	case "firstname":
		return func(g *Gen, i int, _ []any) any { return Pick(g, prenoms) }, nil
	case "lastname":
		return func(g *Gen, i int, _ []any) any { return Pick(g, noms) }, nil
	case "person":
		return func(g *Gen, i int, _ []any) any { a, b := g.Person(); return a + " " + b }, nil
	case "company":
		return func(g *Gen, i int, _ []any) any { return entreprises[pickIndex(g, len(entreprises), yc.Skew)] }, nil
	case "email":
		domain := yc.Domain
		return func(g *Gen, i int, r []any) any {
			first, last := g.Person()
			if of >= 0 {
				if a, b, ok := strings.Cut(str(r[of]), " "); ok {
					first, last = a, b
				}
			}
			d := domain
			if d == "" {
				d = Pick(g, []string{"gmail.com", "orange.fr", "free.fr", "laposte.net", "outlook.fr"})
			}
			return Email(first, last, d)
		}, nil
	case "phone":
		return func(g *Gen, i int, _ []any) any { return g.Phone() }, nil
	case "city":
		return func(g *Gen, i int, _ []any) any { return villes[pickIndex(g, len(villes), yc.Skew)].Nom }, nil
	case "postcode":
		return func(g *Gen, i int, r []any) any { return villes[cityOf(g, r)].CP }, nil
	case "region":
		return func(g *Gen, i int, r []any) any { return villes[cityOf(g, r)].Region }, nil
	case "latitude":
		return func(g *Gen, i int, r []any) any { return villes[cityOf(g, r)].Lat }, nil
	case "longitude":
		return func(g *Gen, i int, r []any) any { return villes[cityOf(g, r)].Lon }, nil
	case "population":
		return func(g *Gen, i int, r []any) any { return int64(villes[cityOf(g, r)].Pop) }, nil
	case "product":
		return func(g *Gen, i int, _ []any) any { return produits[pickIndex(g, len(produits), yc.Skew)].Nom }, nil
	case "category":
		if err := needOf("category"); err != nil {
			return nil, err
		}
		return func(g *Gen, i int, r []any) any { return produits[productOf(g, r)].Categorie }, nil
	case "price":
		return func(g *Gen, i int, r []any) any { return produits[productOf(g, r)].Prix }, nil
	}
	return nil, f.errf(pathOf(path, "gen"), "générateur « %s » inconnu", yc.Gen)
}

func dayArg(s string) (int, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return 0, err
	}
	return int(t.Sub(day0).Hours() / 24), nil
}

// plainValue ramène une valeur YAML au type de stockage d'une cellule.
func plainValue(v any) any {
	switch v := v.(type) {
	case int:
		return int64(v)
	case bool:
		return Bool(v)
	case float64, string, nil:
		return v
	}
	return fmt.Sprint(v)
}
