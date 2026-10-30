package canvas

import (
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
)

// Calcul d'un tableau croisé : un « cube » d'agrégats, un par combinaison
// d'un niveau des champs en lignes (0 : tout confondu, jusqu'au dernier
// champ) et d'un niveau des champs en colonnes. Chaque niveau est une passe
// SQL groupée (GROUP BY) sur la source : les sous-totaux et les totaux sont
// de vrais agrégats (la moyenne d'un groupe, pas la moyenne des moyennes ;
// un nombre distinct). Sans nombre distinct, une seule passe au niveau le
// plus fin suffit, les niveaux au-dessus en sont déduits en Go
// (pivotone.go) ; ce calcul en plusieurs passes reste celui des cas
// rares. Les lignes et les colonnes de détail sont lues dans leur ordre
// (tri des étiquettes, première apparition) par les passes du dernier
// niveau de chaque axe, bornées (MaxPivotRows, MaxPivotCols) avant les
// autres. Le reste (en-têtes imbriqués, arborescence, « Afficher les
// valeurs ») se calcule sur ce résultat, au plus quelques milliers de
// lignes, sans relire la source.

// pivotCube : les agrégats d'un TCD (voir plus haut).
type pivotCube struct {
	nr, nc int
	// rows, cols : étiquettes des lignes et des colonnes de détail, dans
	// l'ordre ; vides sans champ sur l'axe.
	rows, cols [][]string
	vals       []PivotValue
	bases      []pivotBase
	cells      map[string][]any
	// kids : étiquettes des enfants de chaque groupe, dans l'ordre, par axe
	// (0 : lignes, 1 : colonnes) et par chemin (pathKey).
	kids [2]map[string][]string
}

func pathKey(p []string) string { return strconv.Itoa(len(p)) + ":" + strings.Join(p, "\x00") }

func cubeKey(r, c []string) string { return pathKey(r) + "\x01" + pathKey(c) }

// get : les agrégats du groupe de lignes r et de colonnes c (nil : aucune
// ligne de la source).
func (cb *pivotCube) get(r, c []string) []any { return cb.cells[cubeKey(r, c)] }

// value : l'agrégat de la valeur vi (un nombre de lignes absent vaut 0).
func (cb *pivotCube) value(vi int, r, c []string) any {
	if v := cb.get(r, c); v != nil {
		return v[vi]
	}
	if a := cb.vals[vi].Agg; a == "count" || a == "countDistinct" {
		return int64(0)
	}
	return nil
}

// children : les étiquettes sous le groupe prefix de l'axe (0 lignes, 1 colonnes).
func (cb *pivotCube) children(axis int, prefix []string) []string {
	return cb.kids[axis][pathKey(prefix)]
}

// pivotKeyText : le texte d'une étiquette lue en base.
func pivotKeyText(k any) string { return formula.FromResult(k).String() }

// pivotKeyLabel : l'étiquette affichée (« (vide) » pour une valeur vide).
func pivotKeyLabel(k any) string {
	if s := pivotKeyText(k); s != "" {
		return s
	}
	return "(vide)"
}

func (t *tx) pivotCubeRaw(raw PivotConfig) (*pivotCube, error) {
	p, err := t.pivotPlan(raw)
	if err != nil {
		return nil, err
	}
	cfg := p.cfg
	where := ""
	if len(p.filters) > 0 {
		where = " WHERE " + strings.Join(p.filters, " AND ")
	}
	cb := &pivotCube{nr: len(p.rows), nc: len(p.cols), vals: cfg.Values, bases: p.bases, cells: map[string][]any{}}
	plain := make([]string, len(cfg.Values))
	for vi := range cfg.Values {
		v := "1"
		if c := p.vals[vi]; c != nil {
			v = colSQL(c.ID)
		}
		plain[vi] = pivotAgg(cfg.Values[vi].Agg, v)
	}
	rowNames := make([]string, cb.nr)
	for i := range rowNames {
		rowNames[i] = "k" + strconv.Itoa(i)
	}
	colNames := make([]string, cb.nc)
	for i := range colNames {
		colNames[i] = "kc" + strconv.Itoa(i)
	}
	// pass lit le niveau (dr, dc) ; ordered : la passe du dernier niveau d'un
	// axe, dans l'ordre, qui rend aussi les étiquettes de détail.
	pass := func(dr, dc int, order []string, limit int, out *[][]string) error {
		var sel, group []string
		for i := range dr {
			sel = append(sel, p.text(p.rows[i])+" AS "+rowNames[i])
			group = append(group, rowNames[i])
		}
		for i := range dc {
			sel = append(sel, p.text(p.cols[i])+" AS "+colNames[i])
			group = append(group, colNames[i])
		}
		nk := len(sel)
		if order != nil {
			sel = append(sel, "min(_pos) AS o")
		}
		sel = append(sel, plain...)
		if len(sel) == 0 {
			return nil // ni étiquette ni valeur
		}
		q := `SELECT ` + strings.Join(sel, ", ") + ` FROM ` + p.from() + where
		args := append([]any{}, p.args...)
		if len(group) > 0 {
			q += ` GROUP BY ` + strings.Join(group, ", ")
		}
		if order != nil {
			q += ` ORDER BY ` + strings.Join(order, ", ") + ` LIMIT ?`
			args = append(args, limit)
		}
		rs, err := t.query(q, args...)
		if err != nil {
			return err
		}
		defer rs.Close()
		skip := 0
		if order != nil {
			skip = 1
		}
		for rs.Next() {
			row := make([]any, len(sel))
			ptrs := make([]any, len(sel))
			for i := range row {
				ptrs[i] = &row[i]
			}
			if err := rs.Scan(ptrs...); err != nil {
				return err
			}
			r := make([]string, dr)
			for i := range dr {
				r[i] = pivotKeyLabel(row[i])
			}
			c := make([]string, dc)
			for i := range dc {
				c[i] = pivotKeyLabel(row[dr+i])
			}
			cb.cells[cubeKey(r, c)] = row[nk+skip:]
			if out != nil {
				if dr > 0 {
					*out = append(*out, r)
				} else {
					*out = append(*out, c)
				}
			}
		}
		return rs.Err()
	}
	// Une seule passe quand c'est possible (pivotone.go).
	one := pivotOneFits(p, cb)
	if one && !p.sorted() {
		if ok, err := t.pivotOne(p, cb, where, rowNames, colNames, true); err != nil {
			return nil, err
		} else if ok {
			cb.index()
			return cb, nil
		}
	}
	// Colonnes de détail d'abord, puis lignes de détail : bornées avant les
	// autres passes.
	if cb.nc > 0 {
		if err := pass(0, cb.nc, p.axisOrder(p.cols, colNames), MaxPivotCols+1, &cb.cols); err != nil {
			return nil, err
		}
		if len(cb.cols) > MaxPivotCols {
			names := make([]string, cb.nc)
			for i, c := range p.cols {
				names[i] = c.Name
			}
			return nil, msg.Wrap(ErrPivot, "pivot.tooManyColumns", msg.Args{"max": MaxPivotCols, "column": strings.Join(names, ", ")})
		}
	}
	if cb.nr > 0 {
		if err := pass(cb.nr, 0, p.axisOrder(p.rows, rowNames), MaxPivotRows+1, &cb.rows); err != nil {
			return nil, err
		}
		if len(cb.rows) > MaxPivotRows {
			return nil, msg.Wrap(ErrPivot, "pivot.tooManyRows", msg.Args{"max": MaxPivotRows})
		}
	}
	if one && p.sorted() {
		if ok, err := t.pivotOne(p, cb, where, rowNames, colNames, false); err != nil {
			return nil, err
		} else if ok {
			cb.index()
			return cb, nil
		}
	}
	for dr := 0; dr <= cb.nr; dr++ {
		for dc := 0; dc <= cb.nc; dc++ {
			if (dr == cb.nr && dc == 0 && cb.nr > 0) || (dr == 0 && dc == cb.nc && cb.nc > 0) {
				continue // déjà lu
			}
			if len(cb.cols) == 0 && dc > 0 || len(cb.rows) == 0 && dr > 0 {
				continue // aucune étiquette : aucun groupe
			}
			if err := pass(dr, dc, nil, 0, nil); err != nil {
				return nil, err
			}
		}
	}
	cb.index()
	return cb, nil
}

// index range les enfants de chaque groupe (kids), d'après les lignes et
// les colonnes de détail.
func (cb *pivotCube) index() {
	for axis, list := range [2][][]string{cb.rows, cb.cols} {
		kids := map[string][]string{}
		seen := map[string]bool{}
		for _, path := range list {
			for d := range path {
				k := pathKey(path[:d+1])
				if seen[k] {
					continue
				}
				seen[k] = true
				pk := pathKey(path[:d])
				kids[pk] = append(kids[pk], path[d])
			}
		}
		cb.kids[axis] = kids
	}
}

// pivotColEntry : une colonne affichée du TCD (avant ses valeurs) : un
// élément de détail, un groupe plié, un sous-total ou le total général.
type pivotColEntry struct {
	path []string
	info PivotRowInfo
}

// colEntries : les colonnes affichées. flat : le calcul à plat (vue SQL,
// graphiques) : les colonnes de détail et le total, sans sous-totaux ni
// groupes pliés.
func (cb *pivotCube) colEntries(cfg PivotConfig, flat bool) []pivotColEntry {
	var out []pivotColEntry
	var emit func(prefix []string, d int)
	emit = func(prefix []string, d int) {
		for _, k := range cb.children(1, prefix) {
			path := append(append([]string{}, prefix...), k)
			if d == cb.nc-1 {
				out = append(out, pivotColEntry{path, PivotRowInfo{Depth: d, Path: path}})
				continue
			}
			if !flat && cfg.colFolded(path) {
				out = append(out, pivotColEntry{path, PivotRowInfo{Depth: d, Group: true, Collapsed: true, Path: path}})
				continue
			}
			emit(path, d+1)
			if !flat && cfg.Subtotals != SubtotalsNone {
				out = append(out, pivotColEntry{path, PivotRowInfo{Depth: d, Subtotal: true, Path: path}})
			}
		}
	}
	if cb.nc > 0 {
		emit(nil, 0)
	}
	if cb.nc == 0 || !cfg.NoGrandTotal {
		out = append(out, pivotColEntry{nil, PivotRowInfo{Total: true}})
	}
	return out
}

// header : l'en-tête des colonnes de mesure ; flat : chemin complet
// (« 2024 · T1 ») ; sinon l'étiquette du dernier niveau, les niveaux
// au-dessus allant dans colHead.
func (cb *pivotCube) header(cfg PivotConfig, entries []pivotColEntry, flat bool) []string {
	var out []string
	for _, e := range entries {
		for vi := range cfg.Values {
			name := ""
			switch {
			case e.info.Total && cb.nc == 0:
				name = pivotLabel(cfg.Values[vi], cfg.Lang)
			case e.info.Total:
				name = "Total"
			case flat:
				name = strings.Join(e.path, " · ")
			case e.info.Subtotal:
				name = "Total " + e.path[len(e.path)-1]
			default:
				name = e.path[len(e.path)-1]
			}
			if len(cfg.Values) > 1 && !(e.info.Total && cb.nc == 0) {
				name += " · " + pivotLabel(cfg.Values[vi], cfg.Lang)
			}
			out = append(out, name)
		}
	}
	return out
}

// line : les valeurs affichées de la ligne r pour les colonnes entries.
func (cb *pivotCube) line(r []string, entries []pivotColEntry) []string {
	out := make([]string, 0, len(entries)*len(cb.vals))
	for _, e := range entries {
		for vi := range cb.vals {
			out = append(out, cb.text(vi, r, e.path))
		}
	}
	return out
}

// pivotCellsRaw : le TCD à plat (une colonne par champ des lignes, une ligne
// par combinaison, puis le total général) : vue SQL, graphiques, et
// affichage d'un TCD à un seul champ en lignes et en colonnes.
func (t *tx) pivotCellsRaw(raw PivotConfig) ([][]string, error) {
	cfg := raw.normalized()
	cb, err := t.pivotCube(cfg)
	if err != nil {
		return nil, err
	}
	entries := cb.colEntries(cfg, true)
	header := append([]string{}, cfg.RowFields...)
	if cb.nr == 0 {
		header = append(header, "")
	}
	out := [][]string{append(header, cb.header(cfg, entries, true)...)}
	for _, r := range cb.rows {
		out = append(out, append(append([]string{}, r...), cb.line(r, entries)...))
	}
	if len(cfg.Values) > 0 && (cb.nr == 0 || !cfg.NoGrandTotal) {
		keys := []string{"Total"}
		for len(keys) < cb.nr {
			keys = append(keys, "")
		}
		out = append(out, append(keys, cb.line([]string{}, entries)...))
	}
	return out, nil
}

// colFolded dit si le groupe de colonnes path est plié.
func (c PivotConfig) colFolded(path []string) bool {
	return PivotConfig{CollapseAll: c.ColCollapseAll, Collapsed: c.ColCollapsed, Expanded: c.ColExpanded}.folded(path)
}

// pivotColHead : les lignes d'en-tête au-dessus de la dernière (champs en
// colonnes sauf le dernier) : l'étiquette d'un groupe au-dessus de sa
// première colonne ; labels colonnes d'étiquettes à gauche.
func pivotColHead(cb *pivotCube, entries []pivotColEntry, labels int) [][]string {
	if cb.nc < 2 {
		return nil
	}
	nv := len(cb.vals)
	out := make([][]string, cb.nc-1)
	for d := range out {
		row := make([]string, labels, labels+len(entries)*nv)
		var prev []string
		for _, e := range entries {
			var cur []string
			if d < len(e.path)-1 {
				cur = e.path[:d+1]
			}
			for k := range nv {
				s := ""
				if k == 0 && cur != nil && !samePath(cur, prev) {
					s = cur[d]
				}
				row = append(row, s)
			}
			prev = cur
		}
		out[d] = row
	}
	return out
}

func samePath(a, b []string) bool {
	if a == nil || b == nil || len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// pivotColsInfo : la description de chaque colonne affichée (rien pour
// les colonnes d'étiquettes).
func pivotColsInfo(cb *pivotCube, entries []pivotColEntry, labels int) []PivotRowInfo {
	if cb.nc < 2 {
		return nil
	}
	out := make([]PivotRowInfo, labels, labels+len(entries)*len(cb.vals))
	for _, e := range entries {
		for range cb.vals {
			out = append(out, e.info)
		}
	}
	return out
}
