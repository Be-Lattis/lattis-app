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
	Agg   string `json:"agg"` // sum, count, countDistinct, avg, min, max
	// Show : « Afficher les valeurs » (pivotshow.go) : "", pctTotal, pctCol,
	// pctRow, pctOf, pctParentRow, pctParentCol, pctParent, diff, pctDiff,
	// runTotal, pctRunTotal, rankAsc, rankDesc, index.
	Show string `json:"show,omitempty"`
	// BaseField : champ de base (en lignes ou en colonnes) des calculs qui en
	// demandent un ; BaseItem son élément de base (pctOf, diff, pctDiff), ou
	// BasePos "prev" / "next" (élément précédent, suivant).
	BaseField string `json:"baseField,omitempty"`
	BaseItem  string `json:"baseItem,omitempty"`
	BasePos   string `json:"basePos,omitempty"`
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
	// Period : filtre d'une chronologie (pivotslicer.go) : Values sont des
	// périodes ("2024", "2024-T2", "2024-03", "2024-03-15") de ce niveau
	// (years, quarters, months, days) de la date Field.
	Period string `json:"period,omitempty"`
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
	Name string `json:"name,omitempty"`
	// Lang : langue des libellés que le TCD écrit lui-même (« Somme de
	// Ventes », « Sum of Sales »), fixée à sa création dans la langue de la
	// session et gardée ensuite : les en-têtes, que citent les formules des
	// colonnes saisies et les graphiques, ne changent pas d'un poste à
	// l'autre. Vide : le français (TCD d'avant, scripts).
	Lang      string   `json:"lang,omitempty"`
	Source    string   `json:"source"`
	RowFields []string `json:"rowFields,omitempty"`
	// ColFields : champs en colonnes, du plus large au plus fin (en-têtes
	// imbriqués, pivotcube.go). Cols est l'ancien format (un seul champ).
	ColFields []string      `json:"colFields,omitempty"`
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
	// Même chose pour les groupes des champs en colonnes.
	ColCollapseAll bool       `json:"colCollapseAll,omitempty"`
	ColCollapsed   [][]string `json:"colCollapsed,omitempty"`
	ColExpanded    [][]string `json:"colExpanded,omitempty"`

	// Groups : champs groupés (« Grouper… », pivotgroup.go) : dates par
	// années, trimestres, mois, jours ; nombres par intervalles.
	Groups []PivotGroup `json:"groups,omitempty"`

	// Slicers : segments et chronologies posés sur la feuille du TCD
	// (pivotslicer.go) ; leur sélection filtre le TCD, et les autres TCD de
	// même source qu'ils désignent (Linked, posé à la lecture).
	Slicers []PivotSlicer `json:"slicers,omitempty"`
	Linked  []PivotFilter `json:"linked,omitempty"`

	// Hand : colonnes saisies à droite du TCD (pivothand.go, décision 0008) :
	// leurs noms et formules ; les valeurs sont rangées par étiquettes de
	// lignes dans _lattis_hand_kept (owner « pivot:<id> »).
	Hand []PivotHandCol `json:"hand,omitempty"`

	// Styles : mise en forme des colonnes du TCD (pivotstyle.go), par
	// colonne (champ des lignes, valeur, colonne saisie) : elle suit la
	// colonne quand le TCD est actualisé, trié, s'élargit ou rétrécit.
	Styles map[string]PivotColStyle `json:"styles,omitempty"`

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
	c.Choice, c.Linked = nil, nil
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
	if len(c.ColFields) == 0 && c.Cols != "" {
		c.ColFields = []string{c.Cols}
	}
	c.Rows, c.Value, c.Agg, c.Cols = "", "", "", ""
	return c
}

// created : la configuration d'un tableau croisé qu'on crée (CreatePivot,
// CreatePivotAt : ruban, scripts, assistant). Sans valeur, il compte les
// lignes (« Nombre de lignes »), pour montrer un résultat tout de suite ;
// des valeurs choisies sont gardées telles quelles.
func (c PivotConfig) created() PivotConfig {
	c = c.normalized()
	if len(c.Values) == 0 {
		c.Values = []PivotValue{{Agg: "count"}}
	}
	return c
}

// pivotAggs : les fonctions des valeurs, et leur nom dans les libellés
// (msg pivot.caption.*).
var pivotAggs = map[string]msg.ID{
	"sum":           "pivot.caption.sum",
	"count":         "pivot.caption.count",
	"countDistinct": "pivot.caption.countDistinct",
	"avg":           "pivot.caption.avg",
	"min":           "pivot.caption.min",
	"max":           "pivot.caption.max",
}

var pivotShows = map[string]msg.ID{
	"": "", "pctTotal": "pivot.caption.pctTotal", "pctRow": "pivot.caption.pctRow", "pctCol": "pivot.caption.pctCol",
	"pctOf": "pivot.caption.pctOf", "pctParentRow": "pivot.caption.pctParentRow", "pctParentCol": "pivot.caption.pctParentCol",
	"pctParent": "pivot.caption.pctParent", "diff": "pivot.caption.diff", "pctDiff": "pivot.caption.pctDiff",
	"runTotal": "pivot.caption.runTotal", "pctRunTotal": "pivot.caption.pctRunTotal",
	"rankAsc": "pivot.caption.rankAsc", "rankDesc": "pivot.caption.rankDesc", "index": "pivot.caption.index",
}

// showNeedsBase : calculs qui demandent un champ de base (et, pour pctOf,
// diff et pctDiff, un élément de base).
func showNeedsBase(show string) bool {
	switch show {
	case "pctOf", "diff", "pctDiff", "pctParent", "runTotal", "pctRunTotal", "rankAsc", "rankDesc":
		return true
	}
	return false
}

// captionLang : la langue des libellés d'un TCD qu'on crée (PivotConfig.Lang) :
// vide pour le français, comme les TCD d'avant.
func (t *tx) captionLang() string {
	if l := t.nameLang(); l != msg.FR {
		return string(l)
	}
	return ""
}

type pivotPlan struct {
	tb      Table
	rows    []*Column
	cols    []*Column // champs en colonnes, du plus large au plus fin
	vals    []*Column // nil : nombre de lignes
	cfg     PivotConfig
	filters []string
	args    []any
	// extra : expressions des champs calculés et des groupes
	// (« expr AS c_N »), lues dans la source (from) sous les colonnes calc.
	extra []string
	calc  map[int64]bool
	// source : colonne calculée du tableau derrière un champ de calc.
	source map[int64]Column
	// exprs : expression de chaque colonne de extra (un groupe la relit :
	// un alias ne se lit pas dans le même SELECT) ; labels : texte affiché
	// d'une colonne de groupe (SQL sur sa clé c_N), rangée dans l'ordre de
	// sa clé (pivotgroup.go).
	exprs  map[int64]string
	labels map[int64]string
	// computed : requête de la source avec les valeurs de ses colonnes
	// calculées (computedSource), quand l'une d'elles sert de champ.
	computed string
	// bases : champ de base de chaque valeur (pivotshow.go).
	bases []pivotBase
}

// pivotBase désigne le champ de base d'une valeur : son axe (colonnes ou
// lignes) et son rang ; level -1 : aucun (ou absent du TCD).
type pivotBase struct {
	col   bool
	level int
}

// pivotCalcBase numérote les colonnes des champs calculés d'un TCD, au-delà
// de celles d'un tableau.
const pivotCalcBase = 1 << 40

// from est la source des requêtes du TCD : le tableau (avec ses colonnes
// calculées si l'une sert de champ), et les valeurs de ses champs calculés.
func (p pivotPlan) from() string {
	base := p.tb.sql()
	if p.computed != "" {
		base = "(" + p.computed + ") AS " + p.tb.sql()
	}
	if len(p.extra) == 0 {
		return base
	}
	return "(SELECT *, " + strings.Join(p.extra, ", ") + " FROM " + base + ") AS " + p.tb.sql()
}

// text est le texte affiché d'un champ (voir textExpr : direct pour un
// texte ; slt_label pour un champ calculé ; l'étiquette d'un groupe).
func (p pivotPlan) text(c *Column) string {
	if l, ok := p.labels[c.ID]; ok {
		return "coalesce(" + l + ", '')"
	}
	if p.calc[c.ID] {
		return formula.LabelFunc + "(" + colSQL(c.ID) + ")"
	}
	return textExpr(*c)
}

// addExtra ajoute à la source une colonne d'expression e ; rend son numéro.
func (p *pivotPlan) addExtra(e string) int64 {
	id := int64(pivotCalcBase + len(p.extra))
	p.extra = append(p.extra, e+" AS "+colSQL(id))
	if p.exprs == nil {
		p.exprs = map[int64]string{}
	}
	p.exprs[id] = e
	return id
}

// exprOf : l'expression d'une colonne dans la source (en ligne pour une
// colonne de extra).
func (p pivotPlan) exprOf(c *Column) string {
	if e, ok := p.exprs[c.ID]; ok {
		return "(" + e + ")"
	}
	return colSQL(c.ID)
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
	id := p.addExtra(e)
	if p.calc == nil {
		p.calc = map[int64]bool{}
	}
	p.calc[id] = true
	return &Column{ID: id, Name: name, Type: grid.Text}, nil
}

// computedField : une colonne calculée du tableau source comme champ. La
// source est lue avec les valeurs de ses colonnes calculées (comme le tri et
// le filtre d'une colonne calculée, computedfilter.go) : toute formule de
// colonne s'y prête (ligne précédente, recherche dans un autre tableau…).
// La colonne garde son numéro (et donc sa mise en forme), au type de son
// résultat.
func (t *tx) computedField(p *pivotPlan, c Column) (*Column, error) {
	if p.computed == "" {
		src, ok, err := t.computedSource(p.tb)
		if err != nil || !ok {
			return nil, msg.Wrap(ErrPivot, "pivot.computedNotField", msg.Args{"column": c.Name})
		}
		p.computed = src
	}
	typ := t.computedResult(p.tb, c)
	if typ == "" {
		typ = grid.Text
	}
	c.Formula, c.Type = "", typ
	return &c, nil
}

func (t *tx) pivotPlan(raw PivotConfig) (pivotPlan, error) {
	cfg := raw.normalized()
	p := pivotPlan{cfg: cfg}
	tb, err := t.tableByName(cfg.Source)
	if err != nil {
		return p, msg.Wrap(ErrPivot, "pivot.tableNotFound", msg.Args{"table": cfg.Source})
	}
	p.tb = tb
	known := map[string]*Column{}
	// base : un champ calculé ou une colonne du tableau (sans les groupes).
	base := func(name string) (*Column, error) {
		for _, f := range cfg.Calc {
			if !strings.EqualFold(f.Name, name) {
				continue
			}
			if c, ok := known[strings.ToLower(f.Name)]; ok {
				return c, nil
			}
			c, err := t.calcColumn(&p, f.Name, f.Formula)
			if err != nil {
				return nil, err
			}
			known[strings.ToLower(f.Name)] = c
			return c, nil
		}
		for i := range tb.Cols {
			if !strings.EqualFold(tb.Cols[i].Name, name) {
				continue
			}
			if tb.Cols[i].Formula == "" {
				return &tb.Cols[i], nil
			}
			key := "=" + strings.ToLower(tb.Cols[i].Name)
			if c, ok := known[key]; ok {
				return c, nil
			}
			c, err := t.computedField(&p, tb.Cols[i])
			if err != nil {
				return nil, err
			}
			known[key] = c
			return c, nil
		}
		return nil, msg.Wrap(ErrPivot, "pivot.columnNotFound", msg.Args{"column": name, "table": tb.Name})
	}
	// find : base, ou un champ groupé (Date par mois, Années…).
	find := func(name string) (*Column, error) {
		if c, ok, err := t.groupField(&p, name, base, known); ok || err != nil {
			return c, err
		}
		return base(name)
	}
	for _, f := range cfg.RowFields {
		c, err := find(f)
		if err != nil {
			return p, err
		}
		p.rows = append(p.rows, c)
	}
	for _, f := range cfg.ColFields {
		c, err := find(f)
		if err != nil {
			return p, err
		}
		p.cols = append(p.cols, c)
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
			// Un champ groupé s'agrège sur ses valeurs d'origine (comme un tableur).
			if c, err = base(v.Field); err != nil {
				if c, err = find(v.Field); err != nil {
					return p, err
				}
			}
		} else if v.Agg != "count" {
			return p, msg.Wrap(ErrPivot, "pivot.chooseValues", nil)
		}
		p.vals = append(p.vals, c)
		b := pivotBase{level: -1}
		if showNeedsBase(v.Show) {
			if i := foldIndex(cfg.RowFields, v.BaseField); i >= 0 {
				b.level = i
			} else if i := foldIndex(cfg.ColFields, v.BaseField); i >= 0 {
				b = pivotBase{col: true, level: i}
			}
		}
		p.bases = append(p.bases, b)
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
	shown := map[string]bool{}
	for _, f := range append(append([]string{}, cfg.RowFields...), cfg.ColFields...) {
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
	// Segments et chronologies : les leurs, et ceux d'autres TCD reliés.
	all = append(all, slicerFilters(cfg.Slicers)...)
	all = append(all, cfg.Linked...)
	for _, f := range all {
		if len(f.Values) == 0 {
			continue // aucune valeur choisie : toutes les lignes
		}
		var e string
		switch {
		case f.Formula != "":
			c, err := t.calcColumn(&p, f.Field, f.Formula)
			if err != nil {
				continue // la source n'a pas les colonnes : pas de filtre
			}
			e = p.text(c)
		case f.Period != "":
			c, err := base(f.Field)
			if err != nil {
				continue // champ disparu : la chronologie ne filtre plus
			}
			e = periodSQL(f.Period, pivotDateSQL(p.exprOf(c)))
		default:
			c, err := find(f.Field)
			if err != nil {
				return p, err
			}
			e = p.text(c)
		}
		marks := make([]string, len(f.Values))
		for i, v := range f.Values {
			marks[i] = "?"
			p.args = append(p.args, v)
		}
		p.filters = append(p.filters, e+" IN ("+strings.Join(marks, ", ")+")")
	}
	return p, nil
}

// foldIndex : rang de name dans list (sans tenir compte de la casse), -1.
func foldIndex(list []string, name string) int {
	if name == "" {
		return -1
	}
	for i, s := range list {
		if strings.EqualFold(s, name) {
			return i
		}
	}
	return -1
}

// pivotOrder est le tri des étiquettes d'un champ (ORDER BY, valeurs
// d'origine : 9 avant 10), ou "" pour l'ordre de la source. Un champ groupé
// suit l'ordre de ses groupes (2024 avant 2025, janvier avant février).
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
	if _, ok := p.labels[c.ID]; ok {
		return "min(" + colSQL(c.ID) + ") ASC"
	}
	return ""
}

// sorted : un champ au moins a un tri choisi (ou suit l'ordre de ses groupes).
func (p pivotPlan) sorted() bool {
	return len(p.cfg.Sort) > 0 || len(p.cfg.Orders) > 0 || len(p.labels) > 0
}

// axisOrder : ORDER BY des étiquettes des champs d'un axe (alias names),
// champ par champ dans la hiérarchie : un champ non trié garde l'ordre de
// première apparition de son groupe ; les vides de chaque champ viennent en
// dernier dans leur groupe, quel que soit le tri (comme un tableur). La
// requête rend min(_pos) sous l'alias o.
func (p pivotPlan) axisOrder(fields []*Column, names []string) []string {
	if len(fields) == 1 && !p.sorted() {
		return []string{names[0] + " = ''", "o"}
	}
	var order []string
	for i, c := range fields {
		order = append(order, names[i]+" = ''")
		if o := p.pivotOrder(c); o != "" {
			order = append(order, o)
		} else {
			order = append(order, "min(min(_pos)) OVER (PARTITION BY "+strings.Join(names[:i+1], ", ")+")")
		}
	}
	return append(order, "o")
}

// pivotLabel : le nom affiché d'une valeur (« Somme de Ventes »), dans la
// langue lang des libellés du TCD (PivotConfig.Lang, vide : français).
func pivotLabel(v PivotValue, lang string) string {
	if l := strings.TrimSpace(v.Label); l != "" {
		return l
	}
	ln := pivotLang(lang)
	label := v.Agg
	if id, ok := pivotAggs[v.Agg]; ok {
		label = msg.Format(ln, id, nil)
	}
	if v.Field != "" {
		label = msg.Format(ln, "pivot.caption.of", msg.Args{"agg": label, "field": v.Field})
	}
	if id := pivotShows[v.Show]; id != "" {
		label = msg.Format(ln, "pivot.caption.show", msg.Args{"label": label, "show": msg.Format(ln, id, nil)})
	}
	return label
}

// pivotLang : la langue des libellés d'un TCD (vide : français).
func pivotLang(lang string) msg.Lang {
	ln, ok := msg.Parse(lang)
	if !ok {
		return msg.FR
	}
	return ln
}

func pivotAgg(agg, v string) string {
	switch agg {
	case "countDistinct":
		return "count(DISTINCT " + v + ")"
	}
	return agg + "(" + v + ")"
}

// percent écrit une part à la française : 0.125 → « 12,5 % ».
func percent(f float64) string {
	s := strconv.FormatFloat(f*100, 'f', 1, 64)
	s = strings.TrimSuffix(strings.TrimSuffix(s, "0"), ".")
	return strings.ReplaceAll(s, ".", ",") + " %"
}
