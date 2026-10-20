package canvas

import (
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// PivotValue est un champ de la zone Valeurs : la colonne agrégée (vide :
// nombre de lignes), la fonction et l'affichage (valeur ou pourcentage).
type PivotValue struct {
	Field string `json:"field,omitempty"`
	Agg   string `json:"agg"`            // sum, count, countDistinct, avg, min, max
	Show  string `json:"show,omitempty"` // "", pctTotal, pctRow, pctCol
	// Label remplace le nom affiché (« Somme de Ventes ») : le nom
	// personnalisé d'Excel.
	Label string `json:"label,omitempty"`
	// Format est le format de nombre imposé (style JSON : nf, dp), à la place
	// de celui repris de la colonne source. Sans effet sur un pourcentage.
	Format string `json:"format,omitempty"`
}

// PivotFilter est un champ de la zone Filtres : les valeurs gardées.
type PivotFilter struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
	// Formula : filtre par une formule par ligne (anglais canonique) plutôt
	// que par le champ Field : celui de l'onglet à choix par une formule
	// (Choice), Field n'en est que le nom.
	Formula string `json:"formula,omitempty"`
}

// PivotCalcField est un champ calculé (« Champ calculé… ») : une formule
// sur les colonnes de chaque ligne du tableau source (=ANNEE([Date]),
// =GAUCHE([Code];2)), en anglais canonique. Il se place en lignes, en
// colonnes, en filtres ou en valeurs comme une colonne ; les lignes sont
// regroupées par son résultat (choiceformula.go).
type PivotCalcField struct {
	Name    string `json:"name"`
	Formula string `json:"formula,omitempty"`
	// Text : la formule en français, rendue à la lecture ; à l'écriture
	// (volet, API), une formule saisie qui remplace Formula.
	Text string `json:"text,omitempty"`
}

// PivotConfig décrit un tableau croisé dynamique, comme le volet « Champs de
// tableau croisé » d'Excel : zones Lignes, Colonnes, Valeurs et Filtres. Les
// champs sont désignés par le nom de leur colonne dans le tableau source.
type PivotConfig struct {
	// Name nomme le TCD (vue SQL, source de graphique) : « TCD1 »…
	Name      string        `json:"name,omitempty"`
	Source    string        `json:"source"`
	RowFields []string      `json:"rowFields,omitempty"`
	Cols      string        `json:"cols,omitempty"`
	Values    []PivotValue  `json:"values,omitempty"`
	Filters   []PivotFilter `json:"filters,omitempty"`
	// Calc : champs calculés par une formule (PivotCalcField), désignés
	// par leur nom dans les zones comme une colonne.
	Calc []PivotCalcField `json:"calc,omitempty"`
	// Items garde des valeurs des champs des lignes et des colonnes (le ▾ de
	// leurs étiquettes, comme Excel) : un filtre qui n'est pas un filtre de
	// rapport.
	Items []PivotFilter `json:"items,omitempty"`
	// Sort ordonne les étiquettes d'un champ des lignes ou des colonnes :
	// "asc" ou "desc" (absent : ordre de la source, première apparition).
	Sort map[string]string `json:"sort,omitempty"`
	// Orders donne à un champ un ordre personnalisé (Tri personnalisé…),
	// qui l'emporte sur Sort : les valeurs de la liste d'abord, les autres
	// ensuite (ordre croissant), les vides toujours à la fin.
	Orders map[string]CustomOrder `json:"orders,omitempty"`
	// NoGrandTotal masque la ligne et la colonne de total général.
	NoGrandTotal bool `json:"noGrandTotal,omitempty"`
	// Subtotals place les sous-totaux des groupes (plusieurs champs en
	// lignes) : "" sur la ligne du groupe (en haut), "bottom" sur une ligne
	// « Total X » sous le groupe, "none" aucun.
	Subtotals string `json:"subtotals,omitempty"`
	// Groupes pliés (−/+), désignés par leurs étiquettes du premier champ au
	// leur : Collapsed liste les groupes pliés ; avec CollapseAll, tous le
	// sont sauf ceux d'Expanded (Réduire tout). Par défaut, tout est déplié.
	CollapseAll bool       `json:"collapseAll,omitempty"`
	Collapsed   [][]string `json:"collapsed,omitempty"`
	Expanded    [][]string `json:"expanded,omitempty"`

	// Hand : colonnes saisies à droite du TCD (pivothand.go, décision 0008) :
	// leurs noms et formules ; les valeurs sont rangées par étiquettes de
	// lignes dans _lattis_hand_kept (owner « pivot:<id> »).
	Hand []PivotHandCol `json:"hand,omitempty"`

	// Choice : filtre imposé par l'onglet à choix de la feuille du TCD
	// (choice.go), posé à la lecture, jamais enregistré (stored le retire).
	Choice *PivotFilter `json:"choice,omitempty"`

	// Ancien format (un champ de lignes, une valeur) : relu puis converti.
	Rows  string `json:"rows,omitempty"`
	Value string `json:"value,omitempty"`
	Agg   string `json:"agg,omitempty"`
}

// stored : la configuration telle qu'on l'enregistre (sans le filtre de
// l'onglet à choix, qui se repose à chaque lecture).
func (c PivotConfig) stored() PivotConfig {
	c.Choice = nil
	return c
}

// normalized convertit l'ancien format.
func (c PivotConfig) normalized() PivotConfig {
	if len(c.RowFields) == 0 && c.Rows != "" {
		c.RowFields = []string{c.Rows}
	}
	if len(c.Values) == 0 && (c.Value != "" || c.Agg != "") {
		c.Values = []PivotValue{{Field: c.Value, Agg: c.Agg}}
	}
	c.Rows, c.Value, c.Agg = "", "", ""
	return c
}

var pivotAggs = map[string]struct{ label string }{
	"sum":           {"Somme"},
	"count":         {"Nombre"},
	"countDistinct": {"Nombre distinct"},
	"avg":           {"Moyenne"},
	"min":           {"Min"},
	"max":           {"Max"},
}

var pivotShows = map[string]string{"": "", "pctTotal": "% du total", "pctRow": "% de la ligne", "pctCol": "% de la colonne"}

type pivotPlan struct {
	tb      Table
	rows    []*Column
	cols    *Column
	vals    []*Column // nil : nombre de lignes
	cfg     PivotConfig
	filters []string
	args    []any
	// extra : expressions des champs calculés (« expr AS c_N »), lues
	// dans la source (from) sous les colonnes calc.
	extra []string
	calc  map[int64]bool
	// source : colonne calculée du tableau derrière un champ de calc.
	source map[int64]Column
}

// pivotCalcBase numérote les colonnes des champs calculés d'un TCD, au-delà
// de celles d'un tableau.
const pivotCalcBase = 1 << 40

// from est la source des requêtes du TCD : le tableau, avec les valeurs de
// ses champs calculés.
func (p pivotPlan) from() string {
	if len(p.extra) == 0 {
		return p.tb.sql()
	}
	return "(SELECT *, " + strings.Join(p.extra, ", ") + " FROM " + p.tb.sql() + ") AS " + p.tb.sql()
}

// text est le texte affiché d'un champ (voir textExpr : direct pour un
// texte ; slt_label pour un champ calculé).
func (p pivotPlan) text(c *Column) string {
	if p.calc[c.ID] {
		return formula.LabelFunc + "(" + colSQL(c.ID) + ")"
	}
	return textExpr(*c)
}

// calcColumn ajoute à la source la valeur de la formule en (anglais
// canonique) pour chaque ligne, sous une colonne nommée name.
func (t *tx) calcColumn(p *pivotPlan, name, en string) (*Column, error) {
	e, ok, err := t.rowValueSQL(p.tb, en)
	if err != nil {
		return nil, err
	}
	if !ok {
		n, _ := formula.Parse(en, formula.EN)
		miss := []string{}
		if n != nil {
			miss = missingColumns(p.tb, formulaColumns(n))
		}
		if len(miss) == 0 {
			return nil, msg.Wrap(ErrPivot, "pivot.fieldNotComputable", msg.Args{"field": name})
		}
		return nil, msg.Wrap(ErrPivot, "pivot.fieldMissingColumns", msg.Args{"field": name, "columns": quoteList(miss), "table": p.tb.Name})
	}
	id := int64(pivotCalcBase + len(p.extra))
	p.extra = append(p.extra, e+" AS "+colSQL(id))
	if p.calc == nil {
		p.calc = map[int64]bool{}
	}
	p.calc[id] = true
	return &Column{ID: id, Name: name, Type: grid.Text}, nil
}

func (t *tx) pivotPlan(raw PivotConfig) (pivotPlan, error) {
	cfg := raw.normalized()
	p := pivotPlan{cfg: cfg}
	tb, err := t.tableByName(cfg.Source)
	if err != nil {
		return p, msg.Wrap(ErrPivot, "pivot.tableNotFound", msg.Args{"table": cfg.Source})
	}
	p.tb = tb
	calc := map[string]*Column{}
	find := func(name string) (*Column, error) {
		for _, f := range cfg.Calc {
			if !strings.EqualFold(f.Name, name) {
				continue
			}
			if c, ok := calc[strings.ToLower(f.Name)]; ok {
				return c, nil
			}
			c, err := t.calcColumn(&p, f.Name, f.Formula)
			if err != nil {
				return nil, err
			}
			calc[strings.ToLower(f.Name)] = c
			return c, nil
		}
		for i := range tb.Cols {
			if strings.EqualFold(tb.Cols[i].Name, name) {
				if tb.Cols[i].Formula != "" {
					// Colonne calculée : sa formule, ligne par ligne, comme un
					// champ calculé du TCD ; sa mise en forme reste celle de la
					// colonne (format €).
					key := "=" + strings.ToLower(tb.Cols[i].Name)
					if c, ok := calc[key]; ok {
						return c, nil
					}
					c, err := t.calcColumn(&p, tb.Cols[i].Name, tb.Cols[i].Formula)
					if err != nil {
						return nil, msg.Wrap(ErrPivot, "pivot.computedNotField", msg.Args{"column": name})
					}
					if p.source == nil {
						p.source = map[int64]Column{}
					}
					p.source[c.ID] = tb.Cols[i]
					calc[key] = c
					return c, nil
				}
				return &tb.Cols[i], nil
			}
		}
		return nil, msg.Wrap(ErrPivot, "pivot.columnNotFound", msg.Args{"column": name, "table": tb.Name})
	}
	for _, f := range cfg.RowFields {
		c, err := find(f)
		if err != nil {
			return p, err
		}
		p.rows = append(p.rows, c)
	}
	if cfg.Cols != "" {
		if p.cols, err = find(cfg.Cols); err != nil {
			return p, err
		}
	}
	for _, v := range cfg.Values {
		if _, ok := pivotAggs[v.Agg]; !ok {
			return p, msg.Wrap(ErrPivot, "pivot.unknownFunction", msg.Args{"function": fmt.Sprintf("%q", v.Agg)})
		}
		if _, ok := pivotShows[v.Show]; !ok {
			return p, msg.Wrap(ErrPivot, "pivot.unknownShow", msg.Args{"show": fmt.Sprintf("%q", v.Show)})
		}
		var c *Column
		if v.Field != "" {
			if c, err = find(v.Field); err != nil {
				return p, err
			}
		} else if v.Agg != "count" {
			return p, msg.Wrap(ErrPivot, "pivot.chooseValues", nil)
		}
		p.vals = append(p.vals, c)
	}
	// Ordres personnalisés donnés par le nom d'une liste.
	for k, o := range p.cfg.Orders {
		if len(o.Values) == 0 && o.List != "" {
			r, err := t.resolveOrder(o)
			if err != nil {
				return p, fmt.Errorf("%w : %v", ErrPivot, err)
			}
			orders := maps.Clone(p.cfg.Orders)
			orders[k] = r
			p.cfg.Orders = orders
		}
	}
	// Filtres d'étiquettes : seulement ceux des champs encore en lignes ou en
	// colonnes (un champ déplacé ailleurs ne filtre plus par son ▾).
	shown := map[string]bool{strings.ToLower(cfg.Cols): cfg.Cols != ""}
	for _, f := range cfg.RowFields {
		shown[strings.ToLower(f)] = true
	}
	all := append([]PivotFilter{}, cfg.Filters...)
	if cfg.Choice != nil {
		all = append(all, *cfg.Choice)
	}
	for _, f := range cfg.Items {
		if shown[strings.ToLower(f.Field)] {
			all = append(all, f)
		}
	}
	for _, f := range all {
		var c *Column
		if f.Formula != "" {
			if c, err = t.calcColumn(&p, f.Field, f.Formula); err != nil {
				continue // la source n'a pas les colonnes : pas de filtre
			}
		} else if c, err = find(f.Field); err != nil {
			return p, err
		}
		marks := make([]string, len(f.Values))
		for i, v := range f.Values {
			marks[i] = "?"
			p.args = append(p.args, v)
		}
		if len(marks) == 0 {
			continue // aucune valeur choisie : toutes les lignes
		}
		p.filters = append(p.filters, p.text(c)+" IN ("+strings.Join(marks, ", ")+")")
	}
	return p, nil
}

// pivotOrder est le tri des étiquettes d'un champ (ORDER BY, valeurs
// d'origine : 9 avant 10), ou "" pour l'ordre de la source.
func (p pivotPlan) pivotOrder(c *Column) string {
	if k, ok := foldKey(p.cfg.Orders, c.Name); ok && len(p.cfg.Orders[k].Values) > 0 {
		return OrderBy(p.text(c), "min("+colSQL(c.ID)+")", p.cfg.Orders[k])
	}
	switch p.cfg.Sort[c.Name] {
	case "asc":
		return "min(" + colSQL(c.ID) + ") ASC"
	case "desc":
		return "min(" + colSQL(c.ID) + ") DESC"
	}
	return ""
}

// sorted : un champ au moins a un tri choisi.
func (p pivotPlan) sorted() bool { return len(p.cfg.Sort) > 0 || len(p.cfg.Orders) > 0 }

func pivotLabel(v PivotValue) string {
	if l := strings.TrimSpace(v.Label); l != "" {
		return l
	}
	label := pivotAggs[v.Agg].label
	if v.Field != "" {
		label += " de " + v.Field
	}
	if s := pivotShows[v.Show]; s != "" {
		label += " (" + s + ")"
	}
	return label
}

func pivotAgg(agg, v string) string {
	switch agg {
	case "countDistinct":
		return "count(DISTINCT " + v + ")"
	}
	return agg + "(" + v + ")"
}

// pivotCells calcule le tableau croisé en deux passes : les valeurs distinctes
// du champ des colonnes, puis un agrégat par colonne et par valeur
// (CASE WHEN), groupé par les champs des lignes.
func (t *tx) pivotCellsRaw(raw PivotConfig) ([][]string, error) {
	p, err := t.pivotPlan(raw)
	if err != nil {
		return nil, err
	}
	cfg := p.cfg
	where := ""
	if len(p.filters) > 0 {
		where = " WHERE " + strings.Join(p.filters, " AND ")
	}
	// Valeurs distinctes du champ des colonnes.
	var keys []string
	if p.cols != nil {
		// Les vides en dernier, quel que soit le tri (comme Excel).
		order := "k = '', min(_pos)"
		if o := p.pivotOrder(p.cols); o != "" {
			order = "k = '', " + o + ", min(_pos)"
		}
		rs, err := t.query(`SELECT `+p.text(p.cols)+` AS k FROM `+p.from()+where+` GROUP BY k ORDER BY `+order+` LIMIT ?`, append(append([]any{}, p.args...), MaxPivotCols+1)...)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var k string
			if err := rs.Scan(&k); err != nil {
				rs.Close()
				return nil, err
			}
			keys = append(keys, k)
		}
		rs.Close()
		if len(keys) > MaxPivotCols {
			return nil, msg.Wrap(ErrPivot, "pivot.tooManyColumns", msg.Args{"max": MaxPivotCols, "column": p.cols.Name})
		}
	}
	// Colonnes de mesure : (clé de colonne, valeur), puis les totaux.
	type measure struct {
		v     int     // rang dans Values
		key   *string // nil : toutes les colonnes (total)
		label string
	}
	var ms []measure
	vals := cfg.Values
	for _, k := range keys {
		for vi := range vals {
			name := k
			if name == "" {
				name = "(vide)"
			}
			if len(vals) > 1 {
				name += " · " + pivotLabel(vals[vi])
			}
			ms = append(ms, measure{vi, &k, name})
		}
	}
	if p.cols == nil || !cfg.NoGrandTotal {
		for vi := range vals {
			name := pivotLabel(vals[vi])
			if p.cols != nil {
				name = "Total"
				if len(vals) > 1 {
					name += " · " + pivotLabel(vals[vi])
				}
			}
			ms = append(ms, measure{vi, nil, name})
		}
	}
	// Une passe groupée par (lignes, colonne) plutôt qu'un CASE WHEN par
	// valeur du champ des colonnes : chaque agrégat ne lit la source qu'une fois.
	plain := make([]string, len(vals))
	for vi := range vals {
		v := "1"
		if c := p.vals[vi]; c != nil {
			v = colSQL(c.ID)
		}
		plain[vi] = pivotAgg(vals[vi].Agg, v)
	}
	var exprs []string
	var totals []int // rang dans ms des colonnes de total
	for i, m := range ms {
		if m.key == nil {
			exprs = append(exprs, plain[m.v])
			totals = append(totals, i)
		}
	}
	header := make([]string, 0, len(p.rows)+len(ms))
	for _, c := range p.rows {
		header = append(header, c.Name)
	}
	if len(p.rows) == 0 {
		header = append(header, "")
	}
	for _, m := range ms {
		header = append(header, m.label)
	}
	nk := len(p.rows)
	label := func(k any) string {
		s := formula.FromResult(k).String()
		if s == "" {
			s = "(vide)"
		}
		return s
	}
	// byCol[lignes][colonne] : agrégats de chaque valeur.
	byCol := map[string]map[string][]any{}
	if len(keys) > 0 {
		sel := make([]string, 0, nk+1+len(plain))
		group := make([]string, 0, nk+1)
		for i, c := range p.rows {
			sel = append(sel, p.text(c)+" AS k"+strconv.Itoa(i))
			group = append(group, "k"+strconv.Itoa(i))
		}
		sel = append(sel, p.text(p.cols)+" AS kc")
		group = append(group, "kc")
		sel = append(sel, plain...)
		rs, err := t.query(`SELECT `+strings.Join(sel, ", ")+` FROM `+p.from()+where+` GROUP BY `+strings.Join(group, ", "), p.args...)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			row := make([]any, len(sel))
			ptrs := make([]any, len(sel))
			for i := range row {
				ptrs[i] = &row[i]
			}
			if err := rs.Scan(ptrs...); err != nil {
				rs.Close()
				return nil, err
			}
			ks := make([]string, nk)
			for i := range nk {
				ks[i] = label(row[i])
			}
			rk := strings.Join(ks, "\x00")
			if byCol[rk] == nil {
				byCol[rk] = map[string][]any{}
			}
			byCol[rk][formula.FromResult(row[nk]).String()] = row[nk+1:]
		}
		err = rs.Err()
		rs.Close()
		if err != nil {
			return nil, err
		}
	}
	// Lignes : groupes, puis total général.
	type line struct {
		keys []string
		nums []any
	}
	var lines []line
	// assemble range les mesures dans l'ordre des colonnes affichées.
	assemble := func(rk string, tot []any) []any {
		nums := make([]any, len(ms))
		for j, i := range totals {
			nums[i] = tot[j]
		}
		for i, m := range ms {
			if m.key == nil {
				continue
			}
			if vs, ok := byCol[rk][*m.key]; ok {
				nums[i] = vs[m.v]
			} else if a := vals[m.v].Agg; a == "count" || a == "countDistinct" {
				nums[i] = int64(0)
			}
		}
		return nums
	}
	scan := func(q string, args []any, withKeys bool) error {
		rs, err := t.query(q, args...)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			n := len(exprs)
			if withKeys {
				n += nk + 1
			}
			row := make([]any, n)
			ptrs := make([]any, n)
			for i := range row {
				ptrs[i] = &row[i]
			}
			if err := rs.Scan(ptrs...); err != nil {
				return err
			}
			l := line{}
			if withKeys {
				for _, k := range row[:nk] {
					l.keys = append(l.keys, label(k))
				}
				l.nums = assemble(strings.Join(l.keys, "\x00"), row[nk+1:])
			} else {
				l.keys = []string{"Total"}
				l.nums = row
			}
			lines = append(lines, l)
		}
		return rs.Err()
	}
	if nk > 0 {
		sel := make([]string, nk)
		group := make([]string, nk)
		for i, c := range p.rows {
			sel[i] = p.text(c) + " AS k" + strconv.Itoa(i)
			group[i] = "k" + strconv.Itoa(i)
		}
		// Ordre de la source (première apparition), sauf tri choisi sur un
		// champ (▾ de son étiquette).
		cols := append(append(sel, "min(_pos) AS o"), exprs...)
		// Champ par champ, dans la hiérarchie : un champ non trié garde
		// l'ordre de première apparition de son groupe ; les vides de
		// chaque champ viennent en dernier dans leur groupe, quel que soit
		// le tri (comme Excel).
		order := []string{"k0 = ''", "o"}
		if nk > 1 || p.sorted() {
			order = order[:0]
			for i, c := range p.rows {
				order = append(order, group[i]+" = ''")
				if o := p.pivotOrder(c); o != "" {
					order = append(order, o)
				} else {
					order = append(order, "min(min(_pos)) OVER (PARTITION BY "+strings.Join(group[:i+1], ", ")+")")
				}
			}
			order = append(order, "o")
		}
		args := append(append([]any{}, p.args...), MaxPivotRows+1)
		if err := scan(`SELECT `+strings.Join(cols, ", ")+` FROM `+p.from()+where+` GROUP BY `+strings.Join(group, ", ")+` ORDER BY `+strings.Join(order, ", ")+` LIMIT ?`, args, true); err != nil {
			return nil, err
		}
		if len(lines) > MaxPivotRows {
			return nil, msg.Wrap(ErrPivot, "pivot.tooManyRows", msg.Args{"max": MaxPivotRows})
		}
	}
	var total *line
	if len(ms) > 0 && (nk == 0 || !cfg.NoGrandTotal || hasPct(vals)) {
		// Total général : par valeur du champ des colonnes, puis tout confondu.
		grand := map[string][]any{}
		if len(keys) > 0 {
			rs, err := t.query(`SELECT `+p.text(p.cols)+` AS kc, `+strings.Join(plain, ", ")+` FROM `+p.from()+where+` GROUP BY kc`, p.args...)
			if err != nil {
				return nil, err
			}
			for rs.Next() {
				row := make([]any, 1+len(plain))
				ptrs := make([]any, len(row))
				for i := range row {
					ptrs[i] = &row[i]
				}
				if err := rs.Scan(ptrs...); err != nil {
					rs.Close()
					return nil, err
				}
				grand[formula.FromResult(row[0]).String()] = row[1:]
			}
			err = rs.Err()
			rs.Close()
			if err != nil {
				return nil, err
			}
		}
		tot := make([]any, len(exprs))
		if len(exprs) > 0 {
			ptrs := make([]any, len(tot))
			for i := range tot {
				ptrs[i] = &tot[i]
			}
			if err := t.row(`SELECT `+strings.Join(exprs, ", ")+` FROM `+p.from()+where, p.args...).Scan(ptrs...); err != nil {
				return nil, err
			}
		}
		saved := byCol
		byCol = map[string]map[string][]any{"\x01": grand}
		lines = append(lines, line{keys: []string{"Total"}, nums: assemble("\x01", tot)})
		byCol = saved
		total = &lines[len(lines)-1]
		if nk > 0 && cfg.NoGrandTotal {
			lines = lines[:len(lines)-1]
		}
	}
	// Mise en forme, avec les pourcentages.
	num := func(v any) (float64, bool) {
		f := formula.FromResult(v)
		return f.Num, f.Kind == formula.Number
	}
	// Colonne du total de ligne pour une valeur donnée.
	rowTotal := map[int]int{}
	for i, m := range ms {
		if m.key == nil {
			rowTotal[m.v] = i
		}
	}
	out := [][]string{header}
	for _, l := range lines {
		row := append([]string{}, l.keys...)
		if nk > 1 && len(l.keys) == 1 {
			for len(row) < nk {
				row = append(row, "")
			}
		}
		for i, v := range l.nums {
			m := ms[i]
			show := vals[m.v].Show
			x, ok := num(v)
			if show == "" || !ok {
				row = append(row, formula.FromResult(v).String())
				continue
			}
			var base float64
			switch show {
			case "pctTotal":
				if total != nil {
					if j, ok := rowTotal[m.v]; ok {
						base, _ = num(total.nums[j])
					} else {
						base, _ = num(total.nums[i])
					}
				}
			case "pctCol":
				if total != nil {
					base, _ = num(total.nums[i])
				}
			case "pctRow":
				if j, ok := rowTotal[m.v]; ok {
					base, _ = num(l.nums[j])
				} else {
					base = x
				}
			}
			if base == 0 {
				row = append(row, "")
				continue
			}
			row = append(row, percent(x/base))
		}
		out = append(out, row)
	}
	return out, nil
}

func hasPct(vals []PivotValue) bool {
	for _, v := range vals {
		if v.Show != "" {
			return true
		}
	}
	return false
}

// percent écrit une part à la française : 0.125 → « 12,5 % ».
func percent(f float64) string {
	s := strconv.FormatFloat(f*100, 'f', 1, 64)
	s = strings.TrimSuffix(strings.TrimSuffix(s, "0"), ".")
	return strings.ReplaceAll(s, ".", ",") + " %"
}
