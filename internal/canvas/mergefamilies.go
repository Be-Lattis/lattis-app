package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/order"
	"github.com/lalamefine/lattis/internal/series"
)

// Regrouper des feuilles entières (docs/conception/regrouper-onglets-
// semblables.md §6.5, lot 4) : la famille principale (MergePlan.Tables) et
// les autres familles des mêmes onglets (MergePlan.Families) deviennent
// chacune un tableau commun sur la feuille commune, en une seule étape
// d'annulation :
//
//   - merge : tableau commun avec la colonne de catégorie, qui suit le choix ;
//   - once : un seul exemplaire (celui de l'onglet le plus récent), sans
//     colonne de catégorie, le même pour toutes les valeurs ;
//   - leave : le tableau reste sur son onglet (qui est alors gardé).
//
// La disposition reprend celle de l'onglet le plus récent : chaque tableau
// commun prend la place qu'avait le sien, ce qui est en dessous descend
// d'autant que le tableau a grandi ; les tableaux absents de cet onglet
// viennent ensuite, sous les autres. La colonne de catégorie, masquée tant
// qu'une valeur est choisie, prend la place de la première colonne : tout le
// reste se décale d'une colonne, si bien que la feuille a l'air de l'onglet
// d'avant.
//
// Les cellules libres des onglets (titres, totaux, objectifs) sont reprises
// à la même place (sous leur tableau si elles étaient sous un tableau),
// leurs formules visant les tableaux communs : une cellule identique partout
// n'est gardée qu'une fois, une cellule qui change d'un onglet à l'autre
// devient une cellule par valeur (choicecells.go).

// MergeFamily est une autre famille de tableaux des onglets regroupés.
type MergeFamily struct {
	// Tables : un tableau par onglet (au plus), dans l'ordre des onglets.
	Tables  []string      `json:"tables"`
	Columns []MergeColumn `json:"columns,omitempty"`
	Name    string        `json:"name,omitempty"`
	// Mode : merge (défaut), once ou leave.
	Mode string `json:"mode"`
}

// FamilyOutcome dit ce que devient une famille (aperçu et résultat).
type FamilyOutcome struct {
	// Name : le tableau commun ("" : les tableaux restent sur leur onglet).
	Name    string         `json:"name"`
	Mode    string         `json:"mode"`
	Tables  []SimilarTable `json:"tables"`
	Rows    int            `json:"rows"`
	Follows bool           `json:"follows"`
	KeyNote string         `json:"keyNote,omitempty"`
}

// LayoutItem est un élément de la feuille commune (aperçu de la
// disposition) : un tableau commun ou une cellule libre reprise.
type LayoutItem struct {
	Kind     string `json:"kind"` // table, cell
	Name     string `json:"name"` // nom du tableau, ou texte montré de la cellule
	Row      int    `json:"row"`
	Col      int    `json:"col"`
	Rows     int    `json:"rows"`
	Cols     int    `json:"cols"`
	Follows  bool   `json:"follows"`
	PerValue bool   `json:"perValue"`
}

// famJob est un tableau commun à créer.
type famJob struct {
	j    *mergeJob
	mode string
	drop []Table         // once : exemplaires non gardés
	at   map[int64]Table // son tableau sur chaque onglet (exemplaires compris)
	// Position sur la feuille commune.
	row, col int
	// Exécution.
	id      int64
	staging string
	metas   []colMeta
	keyNote string
}

func (fj *famJob) width() int {
	n := len(fj.j.cols)
	if fj.j.hasCat() {
		n++
	}
	return n
}

// follows : le tableau commun suit le choix de l'onglet.
func (fj *famJob) follows(main *mergeJob) bool {
	return fj.j.hasCat() && main.hasCat() && strings.EqualFold(fj.j.plan.Category.Name, main.plan.Category.Name)
}

// carried est une cellule libre reprise sur la feuille commune.
type carried struct {
	row, col int
	shown    string            // texte montré (celui de l'onglet de référence)
	values   map[string]string // par valeur de catégorie ; nil : le même partout
	style    string
}

// sheetsMerge est un regroupement de feuilles préparé.
type sheetsMerge struct {
	jobs     []*famJob // jobs[0] : la famille principale
	left     []SimilarTable
	sheets   []int64          // onglets regroupés, dans l'ordre des sources
	keys     map[int64]string // onglet → valeur de catégorie (texte affiché)
	ref      int64            // onglet de référence (le plus récent)
	refKey   string
	plain    map[string]string // anciens tableaux gardés une fois → tableau commun
	renames  formula.Renames   // anciens tableaux → tableaux communs (cellules reprises)
	stacked  bool
	shiftCol int
	cells    []carried
	lost     int // cellules libres qui tombaient sur un tableau commun
	notes    []string
}

func (sm *sheetsMerge) main() *mergeJob { return sm.jobs[0].j }

// allSources : tous les anciens tableaux (sources et exemplaires non gardés).
func (sm *sheetsMerge) allSources() []Table {
	var out []Table
	for _, fj := range sm.jobs {
		for _, s := range fj.j.srcs {
			out = append(out, s.tb)
		}
		out = append(out, fj.drop...)
	}
	return out
}

func (sm *sheetsMerge) mergeJobs() []*mergeJob {
	out := make([]*mergeJob, len(sm.jobs))
	for i, fj := range sm.jobs {
		out[i] = fj.j
	}
	return out
}

func (sm *sheetsMerge) dropped() []Table {
	var out []Table
	for _, fj := range sm.jobs {
		out = append(out, fj.drop...)
	}
	return out
}

// categoryLabels : le texte d'où se tire la catégorie de chaque source :
// le nom de son onglet (deux tableaux d'un même onglet : leurs noms), le
// titre au-dessus du tableau (cas E) ou une cellule libre de son onglet.
func (t *tx) categoryLabels(from string, srcs []*mergeSource) ([]string, error) {
	labels := make([]string, len(srcs))
	switch {
	case from == "" || from == FromSheet:
		for i, s := range srcs {
			labels[i] = s.sheet
		}
		if len(dedupe(labels)) < len(labels) {
			for i, s := range srcs {
				labels[i] = s.tb.Name // deux tableaux d'une même feuille
			}
		}
	case from == FromTitle:
		for i, s := range srcs {
			cells, err := t.cellMap(s.tb.Sheet)
			if err != nil {
				return nil, err
			}
			if labels[i] = titleAbove(s.tb, cells); labels[i] == "" {
				return nil, fmt.Errorf("%w : « %s » n'a pas de titre au-dessus de lui (cellule de texte juste au-dessus de sa première colonne)", ErrMergeSheets, s.tb.Name)
			}
		}
	default:
		r, c, ok := parseCellSource(from)
		if !ok {
			return nil, fmt.Errorf("%w : source de catégorie « %s » : sheet, title ou cell:A1 attendu", ErrMergeSheets, from)
		}
		a1 := formula.ColumnLetters(c) + fmt.Sprint(r+1)
		for i, s := range srcs {
			var raw string
			err := t.row(`SELECT raw FROM _lattis_cells WHERE sheet = ? AND row = ? AND col = ?`, s.tb.Sheet, r, c).Scan(&raw)
			if errors.Is(err, sql.ErrNoRows) || err == nil && strings.TrimSpace(raw) == "" {
				return nil, fmt.Errorf("%w : la cellule %s de « %s » est vide", ErrMergeSheets, a1, s.sheet)
			}
			if err != nil {
				return nil, err
			}
			labels[i] = strings.TrimSpace(raw)
		}
	}
	return labels, nil
}

// prepareSheetsMerge prépare un regroupement de feuilles : la famille
// principale, puis les autres familles du plan.
func (t *tx) prepareSheetsMerge(plan MergePlan) (*sheetsMerge, error) {
	// Les noms de tous les anciens tableaux se libèrent (supprimés ou
	// renommés), et deux tableaux communs ne prennent pas le même nom.
	exclude := map[int64]bool{}
	resolve := func(names []string) error {
		for _, n := range names {
			tb, err := t.tableByName(n)
			if err != nil {
				return err
			}
			exclude[tb.ID] = true
		}
		return nil
	}
	if err := resolve(plan.Tables); err != nil {
		return nil, err
	}
	for _, f := range plan.Families {
		if f.Mode != FamilyLeave {
			if err := resolve(f.Tables); err != nil {
				return nil, err
			}
		}
	}
	taken := map[string]bool{}
	plan.exclude, plan.taken = exclude, taken
	main, err := t.prepareMerge(plan)
	if err != nil {
		return nil, err
	}
	taken[strings.ToLower(main.plan.Name)] = true
	sm := &sheetsMerge{keys: map[int64]string{}, plain: map[string]string{},
		renames: formula.Renames{Tables: map[string]string{}, Columns: map[string]map[string]string{}}}
	sm.jobs = []*famJob{{j: main, mode: FamilyMerge, at: map[int64]Table{}}}
	sheetAt := map[int64]int{}
	for i, s := range main.srcs {
		if _, dup := sheetAt[s.tb.Sheet]; dup {
			sm.stacked = true
			continue
		}
		sheetAt[s.tb.Sheet] = i
		sm.sheets = append(sm.sheets, s.tb.Sheet)
		sm.jobs[0].at[s.tb.Sheet] = s.tb
	}
	if sm.stacked && len(plan.Families) > 0 {
		return nil, fmt.Errorf("%w : des tableaux d'une même feuille se regroupent seuls (sans autres familles)", ErrMergeSheets)
	}
	cat := main.plan.Category
	for _, s := range main.srcs {
		key := choiceAllKey
		if cat != nil {
			key = text(s.cat, cat.Type)
		}
		if _, ok := sm.keys[s.tb.Sheet]; !ok {
			sm.keys[s.tb.Sheet] = key
		}
	}
	latest := len(main.srcs) - 1
	if cat != nil {
		vals := make([]string, len(main.srcs))
		for i, s := range main.srcs {
			vals[i] = fmt.Sprint(s.cat)
		}
		latest = series.Latest(series.Series{Type: cat.Type, Values: vals})
	}
	sm.ref = main.srcs[latest].tb.Sheet
	sm.refKey = sm.keys[sm.ref]
	if main.hasCat() && !sm.stacked {
		sm.shiftCol = 1
	}
	// rank : ordre des onglets du plus ancien au plus récent (exemplaire
	// gardé d'une famille « once »).
	rank := map[int64]int{}
	{
		idx := make([]int, len(main.srcs))
		for i := range idx {
			idx[i] = i
		}
		if cat != nil {
			sort.SliceStable(idx, func(a, b int) bool {
				va, vb := main.srcs[idx[a]].cat, main.srcs[idx[b]].cat
				switch x := va.(type) {
				case int64:
					y, _ := vb.(int64)
					return x < y
				case string:
					if cat.Type == grid.Date {
						y, _ := vb.(string)
						return x < y
					}
				}
				return idx[a] < idx[b]
			})
		}
		for r, i := range idx {
			rank[main.srcs[i].tb.Sheet] = r
		}
		rank[sm.ref] = len(idx) + 1
	}
	used := map[int64]bool{}
	for _, s := range main.srcs {
		used[s.tb.ID] = true
	}
	for fi, f := range plan.Families {
		mode := f.Mode
		if mode == "" {
			mode = FamilyMerge
		}
		var tables []Table
		for _, n := range f.Tables {
			tb, err := t.tableByName(n)
			if err != nil {
				return nil, err
			}
			if mode != FamilyLeave {
				if _, ok := sheetAt[tb.Sheet]; !ok {
					return nil, fmt.Errorf("%w : « %s » n'est pas sur un des onglets regroupés", ErrMergeSheets, tb.Name)
				}
				if used[tb.ID] {
					return nil, fmt.Errorf("%w : le tableau « %s » est cité deux fois", ErrMergeSheets, tb.Name)
				}
				used[tb.ID] = true
			}
			tables = append(tables, tb)
		}
		switch mode {
		case FamilyLeave:
			for _, tb := range tables {
				var sheet string
				if err := t.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, tb.Sheet).Scan(&sheet); err != nil {
					return nil, err
				}
				sm.left = append(sm.left, SimilarTable{Table: tb.Name, Sheet: tb.Sheet, SheetName: sheet, Rows: tb.Rows})
			}
			continue
		case FamilyMerge, FamilyOnce:
		default:
			return nil, fmt.Errorf("%w : famille %d : mode « %s » : merge, once ou leave attendu", ErrMergeSheets, fi+1, mode)
		}
		if len(tables) == 0 {
			continue
		}
		fj := &famJob{mode: mode, at: map[int64]Table{}}
		var sub MergePlan
		if mode == FamilyOnce {
			keep := tables[0]
			for _, tb := range tables[1:] {
				if rank[tb.Sheet] > rank[keep.Sheet] {
					keep = tb
				}
			}
			names := func(tb Table) []string {
				var out []string
				for _, c := range tb.Cols {
					if !strings.HasPrefix(c.Name, "_") {
						out = append(out, strings.ToLower(c.Name))
					}
				}
				slices.Sort(out)
				return out
			}
			for _, tb := range tables {
				if tb.ID != keep.ID && !slices.Equal(names(tb), names(keep)) {
					return nil, fmt.Errorf("%w : « %s » et « %s » n'ont pas les mêmes colonnes : ils ne se gardent pas en un seul exemplaire (choisissez « regrouper »)", ErrMergeSheets, tb.Name, keep.Name)
				}
				if tb.ID != keep.ID {
					fj.drop = append(fj.drop, tb)
				}
				fj.at[tb.Sheet] = tb
			}
			sub = MergePlan{Tables: []string{keep.Name}, NoCategory: true, Name: f.Name, Old: plan.Old, single: true, exclude: exclude, taken: taken}
		} else {
			sub = MergePlan{Tables: f.Tables, Columns: f.Columns, Name: f.Name, Old: plan.Old, single: true, exclude: exclude, taken: taken}
			if cat != nil {
				vals := make([]string, len(tables))
				for i, tb := range tables {
					vals[i] = cat.Values[sheetAt[tb.Sheet]]
				}
				sub.Category = &MergeCategory{Name: cat.Name, Type: cat.Type, Format: cat.Format, Values: vals}
			} else {
				sub.NoCategory = true
			}
			for _, tb := range tables {
				fj.at[tb.Sheet] = tb
			}
		}
		if strings.TrimSpace(sub.Name) == "" {
			names := make([]string, len(tables))
			for i, tb := range tables {
				names[i] = tb.Name
			}
			sub.Name = familyName(names)
		}
		j, err := t.prepareMerge(sub)
		if err != nil {
			return nil, err
		}
		taken[strings.ToLower(j.plan.Name)] = true
		if j.refused != "" && main.refused == "" {
			main.refused = j.refused
		}
		fj.j = j
		if cat != nil && mode == FamilyMerge && !fj.follows(main) {
			sm.notes = append(sm.notes, fmt.Sprintf("« %s » a déjà une colonne « %s » : sa colonne de catégorie s'appelle « %s » et il ne suivra pas l'onglet à choix", j.plan.Name, cat.Name, j.plan.Category.Name))
		}
		if mode == FamilyOnce && len(fj.drop) > 0 {
			same, err := t.sameData(append([]Table{j.srcs[0].tb}, fj.drop...))
			if err != nil {
				return nil, err
			}
			if !same {
				sm.notes = append(sm.notes, fmt.Sprintf("« %s » : seul l'exemplaire de « %s » est gardé (les autres onglets en avaient une autre version)", j.plan.Name, j.srcs[0].sheet))
			}
		}
		sm.notes = append(sm.notes, j.notes...)
		sm.jobs = append(sm.jobs, fj)
	}
	// Renommages des cellules reprises (et des formules des familles
	// gardées une fois).
	for _, fj := range sm.jobs {
		olds := fj.drop
		for _, s := range fj.j.srcs {
			olds = append(olds, s.tb)
		}
		for _, tb := range olds {
			low := strings.ToLower(tb.Name)
			sm.renames.Tables[low] = fj.j.plan.Name
			if fj.mode == FamilyOnce {
				sm.plain[low] = fj.j.plan.Name
			}
			cols := map[string]string{}
			for i, c := range fj.j.cols {
				for ti, s := range fj.j.srcs {
					if s.tb.ID == tb.ID && c.From[ti] != "" && !strings.EqualFold(c.From[ti], c.Name) {
						cols[strings.ToLower(c.From[ti])] = c.Name
					}
				}
				_ = i
			}
			if len(cols) > 0 {
				sm.renames.Columns[low] = cols
			}
		}
	}
	return sm, nil
}

// rect est la place d'un tableau commun.
type rect struct{ r0, c0, r1, c1 int }

func (r rect) has(row, col int) bool { return row >= r.r0 && row <= r.r1 && col >= r.c0 && col <= r.c1 }

// planLayout place les tableaux communs et les cellules reprises (§6.5,
// « La disposition reprend celle de l'onglet le plus récent »).
func (t *tx) planLayout(sm *sheetsMerge, noCells bool) error {
	if sm.stacked {
		sm.jobs[0].row, sm.jobs[0].col = 0, 0
		return nil
	}
	// Croissance des tableaux de l'onglet de référence : ce qui est dessous
	// descend d'autant.
	type grow struct{ bottom, g int }
	var grows []grow
	for _, fj := range sm.jobs {
		if T, ok := fj.at[sm.ref]; ok {
			g := fj.j.total - T.Rows
			if fj.mode == FamilyOnce {
				g = fj.j.total - T.Rows
			}
			grows = append(grows, grow{T.Bottom(), max(0, g)})
		}
	}
	shift := func(r int) int {
		n := 0
		for _, x := range grows {
			if x.bottom < r {
				n += x.g
			}
		}
		return n
	}
	var rects []rect
	bottom := -1
	place := func(fj *famJob, row, col int) {
		fj.row, fj.col = row, col
		r := rect{row, col, row + fj.j.total, col + fj.width() - 1}
		rects = append(rects, r)
		bottom = max(bottom, r.r1)
	}
	dataShift := func(fj *famJob) int {
		if fj.j.hasCat() {
			return 0 // la colonne de catégorie prend la place du décalage
		}
		return sm.shiftCol
	}
	var later []*famJob
	for _, fj := range sm.jobs {
		T, ok := fj.at[sm.ref]
		if !ok {
			later = append(later, fj)
			continue
		}
		place(fj, T.Row+shift(T.Top()), T.Col+dataShift(fj))
	}
	// Cellules reprises.
	if !noCells {
		if err := t.carryCells(sm, shift); err != nil {
			return err
		}
		for _, c := range sm.cells {
			bottom = max(bottom, c.row)
		}
	}
	// Les tableaux absents de l'onglet de référence viennent ensuite, sous
	// le reste, à la colonne de leur onglet le plus récent.
	for _, fj := range later {
		var T Table
		for _, s := range sm.sheets {
			if x, ok := fj.at[s]; ok {
				T = x
			}
		}
		place(fj, bottom+2, T.Col+dataShift(fj))
	}
	// Une cellule reprise qui tombe sur un tableau commun est perdue.
	kept := sm.cells[:0]
	for _, c := range sm.cells {
		hit := false
		for _, r := range rects {
			hit = hit || r.has(c.row, c.col)
		}
		if hit {
			sm.lost++
			continue
		}
		kept = append(kept, c)
	}
	sm.cells = kept
	return nil
}

// carryCells reprend les cellules libres des onglets regroupés.
func (t *tx) carryCells(sm *sheetsMerge, shift func(int) int) error {
	type entry struct {
		raws   map[int64]string
		styles map[int64]string
	}
	at := map[[2]int]*entry{}
	var order [][2]int
	for _, s := range sm.sheets {
		cells, err := t.cells(s)
		if err != nil {
			return err
		}
		styles := map[[2]int]string{}
		rs, err := t.query(`SELECT row, col, style FROM _lattis_cell_styles WHERE sheet = ?`, s)
		if err != nil {
			return err
		}
		for rs.Next() {
			var r, c int
			var st string
			if err := rs.Scan(&r, &c, &st); err != nil {
				rs.Close()
				return err
			}
			styles[[2]int{r, c}] = st
		}
		rs.Close()
		for _, c := range cells {
			row, col := c.Row+shift(c.Row), c.Col+sm.shiftCol
			// Sous un tableau regroupé : sous son tableau commun.
			best := -1
			for ji, fj := range sm.jobs {
				T, ok := fj.at[s]
				if !ok || T.Bottom() >= c.Row || c.Col < T.Col || c.Col > T.Right() {
					continue
				}
				if best < 0 || T.Bottom() > sm.jobs[best].at[s].Bottom() {
					best = ji
				}
			}
			if best >= 0 {
				fj := sm.jobs[best]
				T := fj.at[s]
				first := fj.col
				if fj.j.hasCat() {
					first++
				}
				row, col = fj.row+fj.j.total+(c.Row-T.Bottom()), first+(c.Col-T.Col)
				if _, placed := fj.at[sm.ref]; !placed {
					continue // tableau placé plus tard : ses légendes restent sur l'onglet
				}
			}
			raw := c.Raw
			if strings.HasPrefix(strings.TrimSpace(raw), "=") {
				if x, ok := formula.RenameRefs(raw, formula.FR, sm.renames, ""); ok {
					raw = x
				}
			}
			p := [2]int{row, col}
			e, ok := at[p]
			if !ok {
				e = &entry{raws: map[int64]string{}, styles: map[int64]string{}}
				at[p] = e
				order = append(order, p)
			}
			e.raws[s] = raw
			if st, ok := styles[[2]int{c.Row, c.Col}]; ok {
				e.styles[s] = st
			}
		}
	}
	for _, p := range order {
		e := at[p]
		c := carried{row: p[0], col: p[1]}
		same := len(e.raws) == len(sm.sheets)
		var first string
		for i, s := range sm.sheets {
			v, ok := e.raws[s]
			if i == 0 {
				first = v
			}
			same = same && ok && v == first
		}
		switch {
		case same:
			c.shown = first
		case !sm.main().hasCat():
			// Sans catégorie, pas de cellule par valeur : celle de l'onglet
			// de référence.
			c.shown = e.raws[sm.ref]
			if c.shown == "" {
				c.shown = first
			}
		default:
			c.values = map[string]string{}
			for s, v := range e.raws {
				c.values[sm.keys[s]] = v
			}
			c.shown = c.values[sm.refKey]
		}
		if st, ok := e.styles[sm.ref]; ok {
			c.style = st
		} else {
			for _, s := range sm.sheets {
				if st, ok := e.styles[s]; ok {
					c.style = st
					break
				}
			}
		}
		sm.cells = append(sm.cells, c)
	}
	return nil
}

// layoutItems : la disposition prévue, pour l'aperçu.
func (sm *sheetsMerge) layoutItems() []LayoutItem {
	out := []LayoutItem{}
	main := sm.main()
	for _, fj := range sm.jobs {
		out = append(out, LayoutItem{Kind: "table", Name: fj.j.plan.Name, Row: fj.row, Col: fj.col, Rows: fj.j.total, Cols: fj.width(), Follows: fj.follows(main)})
	}
	for _, c := range sm.cells {
		out = append(out, LayoutItem{Kind: "cell", Name: c.shown, Row: c.row, Col: c.col, Rows: 0, Cols: 1, PerValue: c.values != nil})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Row != out[j].Row {
			return out[i].Row < out[j].Row
		}
		return out[i].Col < out[j].Col
	})
	return out
}

func (sm *sheetsMerge) perValue() int {
	n := 0
	for _, c := range sm.cells {
		if c.values != nil {
			n++
		}
	}
	return n
}

// outcomes : ce que devient chaque famille (la principale d'abord).
func (t *tx) outcomes(sm *sheetsMerge) ([]FamilyOutcome, error) {
	out := []FamilyOutcome{}
	main := sm.main()
	for _, fj := range sm.jobs {
		o := FamilyOutcome{Name: fj.j.plan.Name, Mode: fj.mode, Rows: fj.j.total, Follows: fj.follows(main), KeyNote: fj.keyNote}
		tabs := []Table{}
		for _, s := range fj.j.srcs {
			tabs = append(tabs, s.tb)
		}
		tabs = append(tabs, fj.drop...)
		for _, tb := range tabs {
			var sheet string
			if err := t.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, tb.Sheet).Scan(&sheet); err != nil {
				return nil, err
			}
			o.Tables = append(o.Tables, SimilarTable{Table: tb.Name, Sheet: tb.Sheet, SheetName: sheet, Rows: tb.Rows})
		}
		out = append(out, o)
	}
	for _, l := range sm.left {
		out = append(out, FamilyOutcome{Mode: FamilyLeave, Tables: []SimilarTable{l}, Rows: l.Rows})
	}
	return out, nil
}

// MergePreview prépare l'aperçu d'un regroupement (lecture pure) : plan
// complété, 20 premières lignes, clé, formules à reprendre, familles et
// disposition de la feuille commune.
func (m *Model) MergePreview(ctx context.Context, plan MergePlan) (MergePreview, error) {
	var p MergePreview
	err := m.read(ctx, func(t *tx) error {
		sm, err := t.prepareSheetsMerge(plan)
		if err != nil {
			return err
		}
		j := sm.main()
		p = MergePreview{Plan: j.plan, Rows: [][]any{}, Total: j.total, Notes: append(append([]string{}, j.notes...), sm.notes...), Refused: j.refused}
		p.Plan.Families, p.Plan.CategoryFrom, p.Plan.NoCells = plan.Families, plan.CategoryFrom, plan.NoCells
		for _, s := range j.srcs {
			st := SimilarTable{Table: s.tb.Name, Sheet: s.tb.Sheet, SheetName: s.sheet, Rows: s.tb.Rows}
			for _, c := range s.tb.Cols {
				if c.ID == s.tb.Key {
					st.Key = c.Name
				}
			}
			p.Tables = append(p.Tables, st)
		}
		if j.hasCat() {
			p.Header = append(p.Header, j.plan.Category.Name)
		}
		for _, c := range j.cols {
			p.Header = append(p.Header, c.Name)
		}
		for i, fj := range sm.jobs {
			if fj.j.key < 0 {
				continue
			}
			note, err := t.mergeKeyCheck(fj.j)
			if err != nil {
				return err
			}
			fj.keyNote = note
			if i == 0 {
				p.Key, p.KeyNote, p.KeyKept = j.cols[j.key].Name, note, note == ""
			} else if note != "" {
				p.Notes = append(p.Notes, fmt.Sprintf("« %s » : %s", fj.j.plan.Name, note))
			}
		}
		gone, err := t.deletableSheets(sm.allSources())
		if err != nil {
			return err
		}
		refs, err := t.planMergeRefsAll(sm.mergeJobs(), sm.plain, sm.dropped(), gone)
		if err != nil {
			return err
		}
		p.Rewritten, p.Pending = refs.rewritten, refs.pending
		p.Old = j.plan.Old
		if p.Old == "" {
			p.Old = MergeOldDelete
			if len(p.Pending) > 0 {
				p.Old = MergeOldHide
			}
		}
		p.Kept = []string{}
		if p.Old == MergeOldDelete {
			if p.Kept, err = t.keptSheets(sm.allSources(), gone); err != nil {
				return err
			}
		}
		if err := t.planLayout(sm, plan.NoCells); err != nil {
			return err
		}
		if p.Families, err = t.outcomes(sm); err != nil {
			return err
		}
		p.Layout = sm.layoutItems()
		p.Cells, p.PerValue, p.Lost = len(sm.cells), sm.perValue(), sm.lost
		// Les premières lignes, source après source.
		for _, s := range j.srcs {
			left := 20 - len(p.Rows)
			if left <= 0 {
				break
			}
			sel, args, err := t.sourceSelect(s, false)
			if err != nil {
				return err
			}
			var outs []string
			for i := range j.cols {
				outs = append(outs, j.valueExpr(s, i))
			}
			q := `SELECT ` + strings.Join(outs, ", ") + ` FROM (` + sel + `) AS q ORDER BY q.p LIMIT ` + fmt.Sprint(left)
			rs, err := t.query(q, args...)
			if err != nil {
				return err
			}
			for rs.Next() {
				vals := make([]any, len(outs))
				ptrs := make([]any, len(outs))
				for k := range vals {
					ptrs[k] = &vals[k]
				}
				if err := rs.Scan(ptrs...); err != nil {
					rs.Close()
					return err
				}
				row := normalize(vals)
				if j.hasCat() {
					row = append([]any{s.cat}, row...)
				}
				p.Rows = append(p.Rows, row)
			}
			rs.Close()
			if err := rs.Err(); err != nil {
				return err
			}
		}
		return nil
	})
	return p, err
}

// MergeSheets regroupe les tableaux du plan (et les autres familles de leurs
// onglets) sur une nouvelle feuille, en une seule étape d'annulation (§7).
func (m *Model) MergeSheets(ctx context.Context, plan MergePlan) (MergeResult, error) {
	var res MergeResult
	err := m.Do(ctx, "Regrouper des onglets", func(t *tx) error {
		var err error
		res, err = t.mergeSheets(plan)
		return err
	})
	return res, err
}

// colMeta : mise en forme d'une colonne du tableau commun, reprise du
// premier tableau qui a la colonne.
type colMeta struct {
	fixed bool
	rules sql.NullString
	width sql.NullInt64
}

// stageJob charge le tableau commun d'une famille dans une table hors
// journal, et prépare ses saisies invalides, sa mise en forme et sa liste
// de validation (journal).
func (t *tx) stageJob(fj *famJob) error {
	j, id := fj.j, fj.id
	colID := func(i int) int64 { return int64(i + 2) }
	defs := []string{"id INTEGER PRIMARY KEY", "_pos TEXT NOT NULL"}
	names := []string{"id", "_pos"}
	if j.hasCat() {
		defs = append(defs, colSQL(1)+" "+sqlType(j.plan.Category.Type))
		names = append(names, colSQL(1))
	}
	for i, c := range j.cols {
		defs = append(defs, colSQL(colID(i))+" "+sqlType(c.Type))
		names = append(names, colSQL(colID(i)))
	}
	// Table de chargement, hors journal (comme Import) : une INSERT … SELECT
	// par source, dans l'ordre des onglets puis des lignes.
	fj.staging = fmt.Sprintf("%smerge%d_%d", importPrefix, id, time.Now().UnixNano())
	if _, err := t.ExecContext(t.Ctx, `CREATE TABLE `+fj.staging+` (`+strings.Join(defs, ", ")+`) STRICT`); err != nil {
		return err
	}
	for _, s := range j.srcs {
		if err := t.Ctx.Err(); err != nil {
			return err
		}
		sel, selArgs, err := t.sourceSelect(s, true)
		if err != nil {
			return err
		}
		outs := []string{fmt.Sprintf("q.id + %d", s.off), order.SpreadSQL(max(1, j.total), fmt.Sprintf("(q.rn + %d)", s.base))}
		// La catégorie en littéral : les colonnes calculées numérotent leurs
		// paramètres (?1, ?2…).
		if j.hasCat() {
			outs = append(outs, catLiteral(s.cat))
		}
		for i := range j.cols {
			outs = append(outs, j.valueExpr(s, i))
		}
		q := `INSERT INTO ` + fj.staging + ` (` + strings.Join(names, ", ") + `) SELECT ` + strings.Join(outs, ", ") + ` FROM (` + sel + `) AS q`
		if _, err := t.ExecContext(t.Ctx, q, selArgs...); err != nil {
			return fmt.Errorf("regroupement de « %s » : %w", s.tb.Name, err)
		}
	}
	// Saisies invalides d'une colonne passée en texte : la saisie devient la
	// valeur (le texte l'accepte).
	var do []stmt
	for i, c := range j.cols {
		for _, s := range j.srcs {
			k := s.from[i]
			if k < 0 || s.tb.Cols[k].Formula != "" {
				continue
			}
			sc := s.tb.Cols[k]
			if c.Type == grid.Text && sc.Type != grid.Text {
				if _, err := t.ExecContext(t.Ctx, `UPDATE `+fj.staging+` SET `+colSQL(colID(i))+` = i.raw FROM _lattis_invalid AS i
					WHERE i.tbl = ? AND i.col = ? AND `+fj.staging+`.id = i.rid + ?`, s.tb.ID, sc.ID, s.off); err != nil {
					return err
				}
				continue
			}
			do = append(do, st(`INSERT INTO _lattis_invalid (tbl, rid, col, raw) SELECT ?, rid + ?, ?, raw FROM _lattis_invalid WHERE tbl = ? AND col = ?`, id, s.off, colID(i), s.tb.ID, sc.ID))
		}
	}
	// Mise en forme : celle des cellules suit ses lignes (rid décalé) ;
	// celle des colonnes et des en-têtes vient du premier tableau qui a la
	// colonne, comme ses règles de mise en forme conditionnelle.
	fj.metas = make([]colMeta, len(j.cols))
	for i := range j.cols {
		first := true
		for _, s := range j.srcs {
			k := s.from[i]
			if k < 0 {
				continue
			}
			sc := s.tb.Cols[k]
			do = append(do, st(`INSERT INTO _lattis_styles (tbl, rid, col, style) SELECT ?, rid + ?, ?, style FROM _lattis_styles WHERE tbl = ? AND col = ? AND rid > 0`, id, s.off, colID(i), s.tb.ID, sc.ID))
			if !first {
				continue
			}
			first = false
			do = append(do, st(`INSERT INTO _lattis_styles (tbl, rid, col, style) SELECT ?, rid, ?, style FROM _lattis_styles WHERE tbl = ? AND col = ? AND rid <= 0`, id, colID(i), s.tb.ID, sc.ID))
			if err := t.row(`SELECT fixed <> 0, rules FROM _lattis_columns WHERE tbl = ? AND id = ?`, s.tb.ID, sc.ID).Scan(&fj.metas[i].fixed, &fj.metas[i].rules); err != nil {
				return err
			}
			err := t.row(`SELECT width FROM _lattis_col_widths WHERE sheet = ? AND col = ?`, s.tb.Sheet, s.tb.Col+k).Scan(&fj.metas[i].width)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
	}
	// Liste de validation de la catégorie (style dv de la colonne), et son
	// format de date.
	if j.hasCat() {
		style := Style{"dv": dedupe(j.catLabel)}
		if f := j.plan.Category.Format; f != "" {
			style["df"] = f
		}
		enc, err := encodeStyle(style)
		if err != nil {
			return err
		}
		do = append(do, st(`INSERT INTO _lattis_styles (tbl, rid, col, style) VALUES (?, 0, 1, ?)`, id, enc))
	}
	if len(do) > 0 {
		return t.apply(do, []stmt{st(`DELETE FROM _lattis_styles WHERE tbl = ?`, id), st(`DELETE FROM _lattis_invalid WHERE tbl = ?`, id)})
	}
	return nil
}

// placeJob met en place le tableau commun chargé, sur la feuille commune.
func (t *tx) placeJob(fj *famJob, sheet int64) error {
	j, id := fj.j, fj.id
	tb := Table{ID: id, Name: j.plan.Name, Sheet: sheet}
	colID := func(i int) int64 { return int64(i + 2) }
	do := []stmt{
		st(`ALTER TABLE ` + fj.staging + ` RENAME TO ` + tb.sql()),
		st(`CREATE INDEX ` + tb.sql() + `_pos ON ` + tb.sql() + `(_pos)`),
		st(`INSERT INTO _lattis_tables (id, sheet, name, row, col) VALUES (?, ?, ?, ?, ?)`, id, sheet, tb.Name, fj.row, fj.col),
	}
	pos := 0
	if j.hasCat() {
		do = append(do, st(`INSERT INTO _lattis_columns (tbl, id, name, type, fixed, pos) VALUES (?, 1, ?, ?, 1, 0)`, id, j.plan.Category.Name, string(j.plan.Category.Type)))
		pos++
	}
	for i, c := range j.cols {
		f := any(nil)
		if j.formulas[i] != "" {
			f = j.formulas[i]
		}
		do = append(do, st(`INSERT INTO _lattis_columns (tbl, id, name, type, fixed, pos, formula, rules) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, colID(i), c.Name, string(c.Type), fj.metas[i].fixed, pos, f, nullable(fj.metas[i].rules)))
		pos++
	}
	if err := t.apply(do, []stmt{
		st(`DELETE FROM _lattis_columns WHERE tbl = ?`, id),
		st(`DELETE FROM _lattis_tables WHERE id = ?`, id),
		st(`DROP INDEX ` + tb.sql() + `_pos`),
		st(`ALTER TABLE ` + tb.sql() + ` RENAME TO ` + fj.staging),
	}); err != nil {
		return err
	}
	// Clé : gardée sans collision (index unique recréé) ; sinon seule la
	// notion de clé part, sans rien demander (règle de Louis).
	if j.key >= 0 && fj.keyNote == "" {
		if err := t.ensureKey(tb.Name, j.cols[j.key].Name); err != nil {
			var ke ErrKey
			if !errors.As(err, &ke) {
				return err
			}
			fj.keyNote = "Pas de clé : " + ke.Why
		}
	}
	return nil
}

func (t *tx) mergeSheets(plan MergePlan) (MergeResult, error) {
	res := MergeResult{Rewritten: []PendingRef{}, Pending: []PendingRef{}, Notes: []string{}, Families: []FamilyOutcome{}}
	// 1. Garde-fous.
	sm, err := t.prepareSheetsMerge(plan)
	if err != nil {
		return res, err
	}
	j := sm.main()
	if j.refused != "" {
		return res, fmt.Errorf("%w : %s", ErrMergeSheets, j.refused)
	}
	for _, fj := range sm.jobs {
		if fj.keyNote, err = t.mergeKeyCheck(fj.j); err != nil {
			return res, err
		}
	}
	all := sm.allSources()
	gone, err := t.deletableSheets(all)
	if err != nil {
		return res, err
	}
	refs, err := t.planMergeRefsAll(sm.mergeJobs(), sm.plain, sm.dropped(), gone)
	if err != nil {
		return res, err
	}
	pending := refs.pending
	res.Rewritten, res.Pending = refs.rewritten, pending
	res.Old = j.plan.Old
	if res.Old == "" {
		res.Old = MergeOldDelete
		if len(pending) > 0 {
			res.Old = MergeOldHide
		}
	}
	if err := t.planLayout(sm, plan.NoCells); err != nil {
		return res, err
	}
	// Ce que devient chaque famille (lu avant que les anciens onglets partent).
	if res.Families, err = t.outcomes(sm); err != nil {
		return res, err
	}
	tabs, err := t.sheetOrder()
	if err != nil {
		return res, err
	}
	at := len(tabs)
	for i, id := range tabs {
		for _, tb := range all {
			if tb.Sheet == id && i < at {
				at = i
			}
		}
	}
	if err := t.purgeImports(); err != nil {
		return res, err
	}
	var id0 int64
	if err := t.row(nextTableID).Scan(&id0); err != nil {
		return res, err
	}
	// 2-3. Tables de chargement, mise en forme.
	for k, fj := range sm.jobs {
		fj.id = id0 + int64(k)
		if err := t.stageJob(fj); err != nil {
			return res, err
		}
	}
	// Cas E : les titres des tableaux empilés regroupés partent avec eux.
	var titles []cell
	if sm.stacked && plan.CategoryFrom == FromTitle && res.Old == MergeOldDelete {
		for _, s := range j.srcs {
			cells, err := t.cellMap(s.tb.Sheet)
			if err != nil {
				return res, err
			}
			for d := 1; d <= 2; d++ {
				if raw, ok := cells[[2]int{s.tb.Top() - d, s.tb.Col}]; ok {
					titles = append(titles, cell{Row: s.tb.Top() - d, Col: s.tb.Col, Raw: raw})
					break
				}
			}
		}
	}
	// 7. Anciens onglets : supprimés (corbeille de l'annulation), ou masqués
	// avec leurs tableaux renommés Janvier_ancien (ce qui les cite suit).
	var kept []string // anciens onglets gardés malgré « Supprimer » (keptSheets)
	switch res.Old {
	case MergeOldHide:
		hidden := map[int64]bool{}
		for _, tb0 := range all {
			old, err := t.mergeTableName(tb0.Name+"_ancien", nil, nil)
			if err != nil {
				return res, err
			}
			cur, err := t.table(tb0.ID)
			if err != nil {
				return res, err
			}
			if err := t.renameTable(cur, old); err != nil {
				return res, err
			}
			if hidden[tb0.Sheet] {
				continue
			}
			hidden[tb0.Sheet] = true
			var was bool
			if err := t.row(`SELECT hidden <> 0 FROM _lattis_sheets WHERE id = ?`, tb0.Sheet).Scan(&was); err != nil {
				return res, err
			}
			if !was {
				q := `UPDATE _lattis_sheets SET hidden = ? WHERE id = ?`
				if err := t.apply([]stmt{st(q, 1, tb0.Sheet)}, []stmt{st(q, 0, tb0.Sheet)}); err != nil {
					return res, err
				}
			}
		}
	default:
		if kept, err = t.keptSheets(all, gone); err != nil {
			return res, err
		}
		for _, tb0 := range all {
			cur, err := t.table(tb0.ID)
			if err != nil {
				return res, err
			}
			if err := t.deleteTable(cur); err != nil {
				return res, err
			}
		}
		for _, c := range titles {
			if !gone[j.srcs[0].tb.Sheet] {
				if err := t.setFreeCell(j.srcs[0].tb.Sheet, c.Row, c.Col, ""); err != nil {
					return res, err
				}
			}
		}
		done := map[int64]bool{}
		for _, tb0 := range all {
			if done[tb0.Sheet] {
				continue
			}
			done[tb0.Sheet] = true
			if gone[tb0.Sheet] {
				if err := t.deleteSheet(tb0.Sheet); err != nil {
					return res, err
				}
			}
		}
		res.Notes = append(res.Notes, kept...)
		if len(pending) > 0 {
			res.Notes = append(res.Notes, fmt.Sprintf("%s citai%s les anciens tableaux : à reprendre", plural(len(pending), "Une formule ou un objet", fmt.Sprintf("%d formules ou objets", len(pending))), plural(len(pending), "t", "ent")))
		}
	}
	// 6. Références (§8) : reprises sur les tableaux communs, une fois les
	// anciens tableaux supprimés ou renommés (un ancien tableau peut porter
	// le nom d'un tableau commun).
	if err := t.applyMergeRefs(refs.ups); err != nil {
		return res, err
	}
	// Nouvelle feuille, placée où était le premier onglet regroupé.
	sheet, err := t.newSheet(j.plan.Sheet)
	if err != nil {
		return res, err
	}
	// Largeurs : celles de l'onglet de référence (décalées avec le reste),
	// sinon celles des colonnes de chaque tableau.
	widths := map[int]int64{}
	if !sm.stacked {
		rs, err := t.query(`SELECT col, width FROM _lattis_col_widths WHERE sheet = ?`, sm.ref)
		if err == nil {
			for rs.Next() {
				var c int
				var w int64
				if err := rs.Scan(&c, &w); err == nil {
					widths[c+sm.shiftCol] = w
				}
			}
			rs.Close()
		}
	}
	if len(widths) == 0 {
		for _, fj := range sm.jobs {
			first := fj.col
			if fj.j.hasCat() {
				first++
			}
			for i, mt := range fj.metas {
				if mt.width.Valid {
					widths[first+i] = mt.width.Int64
				}
			}
		}
	}
	if len(widths) > 0 {
		var wdo []stmt
		cols := make([]int, 0, len(widths))
		for c := range widths {
			cols = append(cols, c)
		}
		slices.Sort(cols)
		for _, c := range cols {
			wdo = append(wdo, st(`INSERT INTO _lattis_col_widths (sheet, col, width) VALUES (?, ?, ?)`, sheet, c, widths[c]))
		}
		if err := t.apply(wdo, []stmt{st(`DELETE FROM _lattis_col_widths WHERE sheet = ?`, sheet)}); err != nil {
			return res, err
		}
	}
	// 3 (fin), 5. Les tableaux : chaque table chargée prend son nom, ses
	// colonnes et sa place ; la clé est recréée s'il n'y a pas de collision.
	for _, fj := range sm.jobs {
		if err := t.placeJob(fj, sheet); err != nil {
			return res, err
		}
	}
	// Cellules reprises : montrées (valeur de l'onglet le plus récent), et
	// par valeur quand elles changent d'un onglet à l'autre.
	var cdo []stmt
	for _, c := range sm.cells {
		if c.shown != "" {
			if err := t.setFreeCell(sheet, c.row, c.col, c.shown); err != nil {
				return res, err
			}
		}
		if c.style != "" {
			cdo = append(cdo, st(`INSERT OR REPLACE INTO _lattis_cell_styles (sheet, row, col, style) VALUES (?, ?, ?, ?)`, sheet, c.row, c.col, c.style))
		}
		for k, v := range c.values {
			cdo = append(cdo, st(`INSERT INTO _lattis_choice_cells (sheet, row, col, value, raw) VALUES (?, ?, ?, ?, ?)`, sheet, c.row, c.col, k, v))
		}
	}
	if len(cdo) > 0 {
		if err := t.apply(cdo, []stmt{st(`DELETE FROM _lattis_choice_cells WHERE sheet = ?`, sheet), st(`DELETE FROM _lattis_cell_styles WHERE sheet = ?`, sheet)}); err != nil {
			return res, err
		}
	}
	keyNote := sm.jobs[0].keyNote
	if j.key >= 0 {
		res.KeyKept, res.KeyNote = keyNote == "", keyNote
		if keyNote != "" {
			res.Notes = append(res.Notes, keyNote)
		}
	}
	for _, fj := range sm.jobs[1:] {
		if fj.keyNote != "" && fj.j.key >= 0 {
			res.Notes = append(res.Notes, fmt.Sprintf("« %s » : %s", fj.j.plan.Name, fj.keyNote))
		}
	}
	if err := t.moveSheet(sheet, at); err != nil {
		return res, err
	}
	// 8. Onglet à choix réglé sur la valeur la plus récente (lot 3).
	if j.hasCat() {
		if err := t.setSheetChoice(sheet, j.plan.Name, j.plan.Category.Name, sm.refKey); err != nil {
			return res, err
		}
	}
	if err := t.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&res.SheetName); err != nil {
		return res, err
	}
	res.Sheet, res.Table, res.Rows = sheet, j.plan.Name, j.total
	res.Cells, res.PerValue = len(sm.cells), sm.perValue()
	res.Notes = append(append(append([]string{}, j.notes...), sm.notes...), res.Notes...)
	if sm.lost > 0 {
		res.Notes = append(res.Notes, plural(sm.lost, "Une cellule libre tombait sur un tableau commun : elle n'est pas reprise", fmt.Sprintf("%d cellules libres tombaient sur un tableau commun : elles ne sont pas reprises", sm.lost)))
	}
	what := "supprimés"
	if res.Old == MergeOldHide {
		what = "masqués"
	}
	n := len(sm.sheets)
	if sm.stacked {
		n = len(j.srcs)
	}
	unit := "onglets regroupés"
	if sm.stacked {
		unit = "tableaux regroupés"
	}
	res.Notice = fmt.Sprintf("%d %s dans « %s » (%s), anciens %s %s", n, unit, j.plan.Name, plural(j.total, fmt.Sprintf("%d ligne", j.total), fmt.Sprintf("%d lignes", j.total)), map[bool]string{true: "tableaux", false: "onglets"}[sm.stacked], what)
	if len(sm.jobs) > 1 {
		var others []string
		for _, fj := range sm.jobs[1:] {
			if fj.mode == FamilyOnce {
				others = append(others, fj.j.plan.Name+" (gardé une fois)")
			} else {
				others = append(others, fj.j.plan.Name)
			}
		}
		res.Notice += " ; aussi " + strings.Join(others, ", ")
	}
	if pv := sm.perValue(); pv > 0 {
		res.Notice += " ; " + plural(pv, "une cellule par "+strings.ToLower(catName(j)), fmt.Sprintf("%d cellules par %s", pv, strings.ToLower(catName(j))))
	}
	if keyNote != "" {
		res.Notice += " ; " + keyNote
	}
	if nr := len(res.Rewritten); nr > 0 || len(pending) > 0 {
		res.Notice += " ; " + refCounts(nr, len(pending))
	}
	if h := summaryHint(j, res.Rewritten); h != "" {
		res.Notes = append(res.Notes, h)
	}
	for _, k := range kept {
		res.Notice += " ; " + k
	}
	t.notes = append(t.notes, res.Notice)
	return res, nil
}
