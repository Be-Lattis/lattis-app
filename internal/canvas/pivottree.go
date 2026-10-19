package canvas

import (
	"slices"
	"strings"
)

// Disposition compacte d'Excel : avec plusieurs champs en Lignes, les lignes
// se groupent par le premier champ, les champs suivants s'indentent dans la
// même colonne (« Étiquettes de lignes »), chaque groupe porte son
// sous-total et se plie ou se déplie (−/+). Le calcul à plat (pivotFlat :
// une colonne par champ, une ligne par combinaison) reste celui que lisent
// la vue SQL et les graphiques.

// PivotRowInfo décrit une ligne affichée d'un tableau croisé en
// arborescence (sous l'en-tête, dans l'ordre de Cells).
type PivotRowInfo struct {
	// Depth est le niveau dans la hiérarchie (0 : premier champ des lignes).
	Depth int `json:"depth"`
	// Group : ligne d'un groupe (champ qui n'est pas le dernier), avec −/+.
	Group bool `json:"group,omitempty"`
	// Collapsed : groupe plié (ses lignes ne s'affichent pas).
	Collapsed bool `json:"collapsed,omitempty"`
	// Subtotal : ligne « Total X » d'un groupe (sous-totaux en bas).
	Subtotal bool `json:"subtotal,omitempty"`
	// Total : ligne du total général.
	Total bool `json:"total,omitempty"`
	// Path : étiquettes du groupe ou de la ligne, du premier champ au sien.
	Path []string `json:"path,omitempty"`
}

// Emplacement des sous-totaux (PivotConfig.Subtotals).
const (
	SubtotalsTop    = ""
	SubtotalsBottom = "bottom"
	SubtotalsNone   = "none"
)

// PivotRowsLabel est l'étiquette de la colonne des lignes en disposition compacte.
const PivotRowsLabel = "Étiquettes de lignes"

// compact : le TCD s'affiche en arborescence (plusieurs champs en lignes).
func (c PivotConfig) compact() bool { return len(c.RowFields) > 1 }

// folded dit si le groupe path est plié.
func (c PivotConfig) folded(path []string) bool {
	in := func(list [][]string) bool {
		return slices.ContainsFunc(list, func(p []string) bool { return slices.Equal(p, path) })
	}
	if c.CollapseAll {
		return !in(c.Expanded)
	}
	return in(c.Collapsed)
}

// flat retire ce qui ne touche que l'affichage en arborescence : le calcul à
// plat (et son cache) n'en dépend pas.
func (c PivotConfig) flat() PivotConfig {
	c.Subtotals, c.CollapseAll, c.Collapsed, c.Expanded = "", false, nil, nil
	c.Hand = nil // colonnes saisies : ajoutées après le calcul
	return c
}

// truncated garde les n premiers champs des lignes, sans total général : les
// sous-totaux du niveau n-1. Les valeurs gardées par le ▾ d'un champ retiré
// filtrent toujours (un sous-total ne compte que les lignes affichées).
func (c PivotConfig) truncated(n int) PivotConfig {
	s := c.flat()
	s.RowFields = append([]string{}, c.RowFields[:n]...)
	s.NoGrandTotal = true
	deeper := map[string]bool{}
	for _, f := range c.RowFields[n:] {
		deeper[strings.ToLower(f)] = true
	}
	s.Filters = append([]PivotFilter{}, c.Filters...)
	s.Items = nil
	for _, it := range c.Items {
		if deeper[strings.ToLower(it.Field)] {
			s.Filters = append(s.Filters, it)
		} else {
			s.Items = append(s.Items, it)
		}
	}
	return s
}

type pivotView struct {
	cells [][]string
	rows  []PivotRowInfo
}

// pivotViewRaw met le calcul à plat en arborescence (plusieurs champs en
// lignes) ; avec un seul champ, il est rendu tel quel.
func (t *tx) pivotViewRaw(raw PivotConfig) (pivotView, error) {
	cfg := raw.normalized()
	flat, err := t.pivotFlat(cfg)
	if err != nil || !cfg.compact() || len(flat) == 0 {
		return pivotView{cells: flat}, err
	}
	nk := len(cfg.RowFields)
	// Sous-totaux de chaque niveau de groupe, par chemin d'étiquettes.
	subs := make([]map[string][]string, nk-1)
	for d := range nk - 1 {
		cells, err := t.pivotFlat(cfg.truncated(d + 1))
		if err != nil {
			return pivotView{}, err
		}
		m := map[string][]string{}
		for _, r := range cells[1:] {
			if len(r) > d {
				m[strings.Join(r[:d+1], "\x00")] = r[d+1:]
			}
		}
		subs[d] = m
	}
	body := flat[1:]
	var total []string
	if !cfg.NoGrandTotal && len(body) > 0 {
		total, body = body[len(body)-1], body[:len(body)-1]
	}
	// Arbre des groupes, dans l'ordre de première apparition (celui du
	// calcul à plat, tri des étiquettes compris).
	type node struct {
		label string
		path  []string
		vals  []string
		kids  []*node
		index map[string]*node
	}
	root := &node{index: map[string]*node{}}
	for _, r := range body {
		if len(r) < nk {
			continue
		}
		n := root
		for d := range nk {
			k := r[d]
			c := n.index[k]
			if c == nil {
				c = &node{label: k, path: append([]string{}, r[:d+1]...), index: map[string]*node{}}
				n.index[k] = c
				n.kids = append(n.kids, c)
			}
			n = c
		}
		n.vals = r[nk:]
	}
	nm := len(flat[0]) - nk
	blank := func() []string { return make([]string, nm) }
	header := append([]string{PivotRowsLabel}, flat[0][nk:]...)
	out := pivotView{cells: [][]string{header}}
	add := func(label string, vals []string, info PivotRowInfo) {
		if len(vals) != nm {
			vals = blank()
		}
		out.cells = append(out.cells, append([]string{label}, vals...))
		out.rows = append(out.rows, info)
	}
	var emit func(n *node, d int)
	emit = func(n *node, d int) {
		for _, c := range n.kids {
			if d == nk-1 {
				add(c.label, c.vals, PivotRowInfo{Depth: d, Path: c.path})
				continue
			}
			sub := subs[d][strings.Join(c.path, "\x00")]
			info := PivotRowInfo{Depth: d, Group: true, Path: c.path}
			if cfg.folded(c.path) {
				// Plié : le sous-total sur la ligne du groupe, quel que soit
				// l'emplacement choisi (comme Excel).
				info.Collapsed = true
				add(c.label, sub, info)
				continue
			}
			switch cfg.Subtotals {
			case SubtotalsBottom:
				add(c.label, nil, info)
				emit(c, d+1)
				add("Total "+c.label, sub, PivotRowInfo{Depth: d, Subtotal: true, Path: c.path})
			case SubtotalsNone:
				add(c.label, nil, info)
				emit(c, d+1)
			default:
				add(c.label, sub, info)
				emit(c, d+1)
			}
		}
	}
	emit(root, 0)
	if total != nil {
		add(total[0], total[nk:], PivotRowInfo{Total: true})
	}
	return out, nil
}
