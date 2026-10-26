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
	c.ColCollapseAll, c.ColCollapsed, c.ColExpanded = false, nil, nil
	c.Hand = nil   // colonnes saisies : ajoutées après le calcul
	c.Styles = nil // mise en forme : sans effet sur le calcul (ni sur son cache)
	c.Slicers = calcSlicers(c.Slicers)
	return c
}

// cubed : ce dont dépendent les agrégats (pivotCube) : ni les totaux
// affichés, ni le nom ou la langue des libellés.
func (c PivotConfig) cubed() PivotConfig {
	c = c.flat()
	c.NoGrandTotal, c.Name = false, ""
	return c
}

type pivotView struct {
	cells [][]string
	rows  []PivotRowInfo
	// colHead : lignes d'en-tête au-dessus de cells[0] (plusieurs champs en
	// colonnes) ; cols : description de chaque colonne (groupe, sous-total).
	colHead [][]string
	cols    []PivotRowInfo
}

// pivotViewRaw met le TCD en forme : en arborescence avec plusieurs champs
// en lignes, en-têtes imbriqués avec plusieurs champs en colonnes ; sinon,
// le calcul à plat tel quel.
func (t *tx) pivotViewRaw(raw PivotConfig) (pivotView, error) {
	cfg := raw.normalized()
	if !cfg.compact() && len(cfg.ColFields) < 2 {
		flat, err := t.pivotFlat(cfg)
		return pivotView{cells: flat}, err
	}
	cb, err := t.pivotCube(cfg)
	if err != nil {
		return pivotView{}, err
	}
	entries := cb.colEntries(cfg, false)
	measures := cb.header(cfg, entries, false)
	totalLine := len(cfg.Values) > 0 && (cb.nr == 0 || !cfg.NoGrandTotal)
	if !cfg.compact() {
		// Un champ en lignes (ou aucun) : une colonne par champ.
		header := append([]string{}, cfg.RowFields...)
		if cb.nr == 0 {
			header = append(header, "")
		}
		out := pivotView{cells: [][]string{append(header, measures...)}}
		for _, r := range cb.rows {
			out.cells = append(out.cells, append(append([]string{}, r...), cb.line(r, entries)...))
		}
		if totalLine {
			out.cells = append(out.cells, append([]string{"Total"}, cb.line(nil, entries)...))
		}
		out.colHead, out.cols = pivotColHead(cb, entries, len(header)), pivotColsInfo(cb, entries, len(header))
		return out, nil
	}
	nk := cb.nr
	nm := len(measures)
	blank := func() []string { return make([]string, nm) }
	out := pivotView{cells: [][]string{append([]string{PivotRowsLabel}, measures...)}}
	add := func(label string, vals []string, info PivotRowInfo) {
		if len(vals) != nm {
			vals = blank()
		}
		out.cells = append(out.cells, append([]string{label}, vals...))
		out.rows = append(out.rows, info)
	}
	// Arbre des groupes, dans l'ordre des lignes de détail (tri des
	// étiquettes compris).
	var emit func(prefix []string, d int)
	emit = func(prefix []string, d int) {
		for _, k := range cb.children(0, prefix) {
			path := append(append([]string{}, prefix...), k)
			if d == nk-1 {
				add(k, cb.line(path, entries), PivotRowInfo{Depth: d, Path: path})
				continue
			}
			info := PivotRowInfo{Depth: d, Group: true, Path: path}
			if cfg.folded(path) {
				// Plié : le sous-total sur la ligne du groupe, quel que soit
				// l'emplacement choisi (comme un tableur).
				info.Collapsed = true
				add(k, cb.line(path, entries), info)
				continue
			}
			switch cfg.Subtotals {
			case SubtotalsBottom:
				add(k, nil, info)
				emit(path, d+1)
				add("Total "+k, cb.line(path, entries), PivotRowInfo{Depth: d, Subtotal: true, Path: path})
			case SubtotalsNone:
				add(k, nil, info)
				emit(path, d+1)
			default:
				add(k, cb.line(path, entries), info)
				emit(path, d+1)
			}
		}
	}
	emit(nil, 0)
	if totalLine {
		add("Total", cb.line(nil, entries), PivotRowInfo{Total: true})
	}
	out.colHead, out.cols = pivotColHead(cb, entries, 1), pivotColsInfo(cb, entries, 1)
	return out, nil
}

// pivotFullHeader : le nom complet de chaque colonne affichée (« 2024 · T1 »
// avec plusieurs champs en colonnes), comme les nomment les formules.
func pivotFullHeader(v pivotView) []string {
	out := append([]string{}, v.cells[0]...)
	for j := range out {
		if j < len(v.cols) {
			if p := v.cols[j].Path; len(p) > 1 && !v.cols[j].Total {
				out[j] = strings.Join(append(append([]string{}, p[:len(p)-1]...), out[j]), " · ")
			}
		}
	}
	return out
}
