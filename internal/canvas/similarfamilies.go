package canvas

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/series"
)

// Feuilles à plusieurs tableaux (docs/conception/regrouper-onglets-semblables.md
// §6.5, lot 4) : on regroupe des feuilles entières. Les tableaux s'apparient
// d'un onglet à l'autre famille par famille (mêmes colonnes, §2.2) ; la plus
// grande famille dont les données changent est la famille principale (les
// champs Tables, Columns, Category du groupe), les autres sont listées dans
// Families avec ce qu'on propose d'en faire :
//
//	merge : un tableau commun avec la colonne de catégorie, qui suit le choix
//	once  : mêmes données dans tous les onglets (Tarifs recopié) : un seul
//	        exemplaire, sans colonne de catégorie
//	leave : tableau présent dans un seul onglet : il reste sur son onglet
//
// Cas E : des tableaux semblables empilés sur une même feuille sous des
// titres (« Paris », « Lyon ») ; la catégorie vient des titres. La catégorie
// peut aussi venir d'une cellule libre qui change d'un onglet à l'autre (un
// titre « Janvier 2025 » en A1). Cas F : une colonne Date dont les valeurs
// tombent dans le mois de chaque onglet ; la colonne de catégorie est
// facultative.

// Ce que devient une famille (TableFamily.Mode, MergeFamily.Mode).
const (
	FamilyMerge = "merge"
	FamilyOnce  = "once"
	FamilyLeave = "leave"
)

// Sources de la catégorie (MergePlan.CategoryFrom, CategorySource.Source).
const (
	FromSheet = "sheet" // noms d'onglets
	FromTitle = "title" // titre au-dessus de chaque tableau (cas E)
	fromCell  = "cell:" // cellule libre : « cell:A1 »
)

// TableFamily est une famille de tableaux appariés d'un onglet à l'autre.
type TableFamily struct {
	Tables  []SimilarTable `json:"tables"`
	Columns []MergeColumn  `json:"columns"`
	Key     string         `json:"key"`
	Name    string         `json:"name"`
	// Same : mêmes colonnes et mêmes données dans tous les onglets.
	Same bool   `json:"same"`
	Mode string `json:"mode"` // merge, once ou leave (proposé)
	Note string `json:"note,omitempty"`
}

// SimilarSheet est un onglet d'un groupe.
type SimilarSheet struct {
	Sheet  int64  `json:"sheet"`
	Name   string `json:"name"`
	Hidden bool   `json:"hidden"`
	// Copied : copié d'un autre onglet du groupe (« Copier la feuille »).
	Copied bool `json:"copied"`
}

// CategorySource est une source possible de la catégorie.
type CategorySource struct {
	Source   string        `json:"source"` // sheet, title ou cell:A1
	Label    string        `json:"label"`  // « nom de l'onglet », « cellule A1 »
	Category series.Series `json:"category"`
}

// famCand : un groupe vérifié de tableaux semblables (une famille).
type famCand struct {
	members []*candidate
	same    bool
}

// sameDataLimit : au-delà, des tableaux ne sont pas comparés ligne à ligne
// pendant la détection (ils sont tenus pour différents).
const sameDataLimit = 5000

// tableRows lit les lignes d'un tableau (colonnes saisies non techniques,
// dans leur ordre), dans l'ordre des lignes.
func (t *tx) tableRows(tb Table, limit int) ([][]any, error) {
	var cols []string
	for _, c := range tb.Cols {
		if c.Formula == "" && !strings.HasPrefix(c.Name, "_") {
			cols = append(cols, colSQL(c.ID))
		}
	}
	if len(cols) == 0 {
		return nil, nil
	}
	// La détection tourne à chaque action : la lecture reste valable tant
	// que le tableau ne change pas.
	return cachedOn(t, fmt.Sprintf("datarows:%s:%d:%s", tb.sql(), limit, strings.Join(cols, ",")), []string{tb.sql()}, func() ([][]any, error) {
		rs, err := t.query(`SELECT `+strings.Join(cols, ", ")+` FROM `+tb.sql()+` ORDER BY _pos LIMIT ?`, limit)
		if err != nil {
			return nil, err
		}
		defer rs.Close()
		var out [][]any
		for rs.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rs.Scan(ptrs...); err != nil {
				return nil, err
			}
			out = append(out, normalize(vals))
		}
		return out, rs.Err()
	})
}

// sameData : les tableaux ont les mêmes colonnes (noms mis à plat, dans
// l'ordre) et les mêmes lignes (empreinte des valeurs, dans l'ordre).
func (t *tx) sameData(tables []Table) (bool, error) {
	if len(tables) < 2 {
		return false, nil
	}
	fold := func(tb Table) []string {
		var out []string
		for _, c := range tb.Cols {
			if c.Formula == "" && !strings.HasPrefix(c.Name, "_") {
				out = append(out, foldColumn(c.Name))
			}
		}
		return out
	}
	first := fold(tables[0])
	for _, tb := range tables {
		if tb.Rows != tables[0].Rows || tb.Rows > sameDataLimit || !slices.Equal(fold(tb), first) {
			return false, nil
		}
	}
	ref, err := t.tableRows(tables[0], sameDataLimit)
	if err != nil {
		return false, err
	}
	for _, tb := range tables[1:] {
		rows, err := t.tableRows(tb, sameDataLimit)
		if err != nil {
			return false, err
		}
		if !reflect.DeepEqual(rows, ref) {
			return false, nil
		}
	}
	return true, nil
}

func candTables(ms []*candidate) []Table {
	out := make([]Table, len(ms))
	for i, m := range ms {
		out[i] = m.tb
	}
	return out
}

// similarTable décrit un tableau candidat.
func similarTable(m *candidate) SimilarTable {
	st := SimilarTable{Table: m.tb.Name, Sheet: m.tb.Sheet, SheetName: m.sheet, Rows: m.tb.Rows, Hidden: m.hidden}
	for _, c := range m.tb.Cols {
		if c.ID == m.tb.Key {
			st.Key = c.Name
		}
	}
	return st
}

// familyName : nom proposé pour le tableau commun d'une famille (partie
// commune des noms de tableaux, sinon le nom du premier).
func familyName(names []string) string {
	if len(names) >= 2 {
		if n := identifier(series.Recognize(names).Common); len([]rune(n)) >= 2 {
			return n
		}
	}
	base := strings.TrimRightFunc(names[0], func(r rune) bool { return r >= '0' && r <= '9' || r == '_' })
	if n := identifier(base); len([]rune(n)) >= 2 {
		return n
	}
	return names[0]
}

// family décrit une famille attachée à un groupe.
func (t *tx) family(ms []*candidate) (TableFamily, error) {
	f := TableFamily{}
	names := make([]string, len(ms))
	for i, m := range ms {
		st := similarTable(m)
		if st.Key != "" {
			f.Key = st.Key
		}
		f.Tables = append(f.Tables, st)
		names[i] = m.tb.Name
	}
	f.Name = familyName(names)
	if len(ms) == 1 {
		f.Mode, f.Columns = FamilyLeave, matchColumns(candTables(ms))
		return f, nil
	}
	f.Columns = matchColumns(candTables(ms))
	same, err := t.sameData(candTables(ms))
	if err != nil {
		return f, err
	}
	f.Same = same
	f.Mode = FamilyMerge
	if same {
		f.Mode = FamilyOnce
		return f, nil
	}
	// Données presque identiques (Tarifs mis à jour une année) : comme des
	// données différentes, et le volet le dit.
	for i := 1; i < len(ms); i++ {
		eq, err := t.sameData([]Table{ms[i-1].tb, ms[i].tb})
		if err != nil {
			return f, err
		}
		if eq {
			for k := 1; k < len(ms); k++ {
				if e, err := t.sameData([]Table{ms[k-1].tb, ms[k].tb}); err == nil && !e {
					f.Note = t.say("mergeSheets.followsChoice", msg.Args{"table": f.Name, "sheet": ms[k].sheet})
					break
				}
			}
			break
		}
	}
	return f, nil
}

// sheetFamilies forme les groupes de feuilles : la plus grande famille (dont
// les données changent) d'abord, puis les familles dont les tableaux sont
// sur ses onglets, et les tableaux seuls de ces onglets.
func (t *tx) sheetFamilies(fams []*famCand, cands []*candidate) ([]SheetGroup, [][]*candidate, error) {
	sort.SliceStable(fams, func(i, j int) bool {
		if fams[i].same != fams[j].same {
			return !fams[i].same
		}
		return len(fams[i].members) > len(fams[j].members)
	})
	used := make([]bool, len(fams))
	var out []SheetGroup
	var mains [][]*candidate
	taken := map[int64]bool{}
	for i, f := range fams {
		if used[i] {
			continue
		}
		g := buildGroup(f.members)
		if len(g.Tables) < 3 && g.Category.Score < series.Weak {
			continue // deux tableaux seulement : il faut des noms en série
		}
		used[i] = true
		g.Identical = g.Identical && !f.same
		sheets := map[int64]bool{}
		for _, m := range f.members {
			sheets[m.tb.Sheet] = true
			taken[m.tb.ID] = true
		}
		g.Families = []TableFamily{}
		for k, o := range fams {
			if used[k] {
				continue
			}
			var in []*candidate
			for _, m := range o.members {
				if sheets[m.tb.Sheet] && !taken[m.tb.ID] {
					in = append(in, m)
				}
			}
			if len(in) < 2 || 2*len(in) < len(o.members) {
				continue
			}
			used[k] = true
			for _, m := range in {
				taken[m.tb.ID] = true
			}
			fam, err := t.family(in)
			if err != nil {
				return nil, nil, err
			}
			g.Families = append(g.Families, fam)
		}
		// Tableaux seuls des onglets du groupe : ils restent sur leur onglet.
		for _, c := range cands {
			if sheets[c.tb.Sheet] && !taken[c.tb.ID] {
				taken[c.tb.ID] = true
				fam, err := t.family([]*candidate{c})
				if err != nil {
					return nil, nil, err
				}
				g.Families = append(g.Families, fam)
			}
		}
		out = append(out, g)
		mains = append(mains, f.members)
	}
	return out, mains, nil
}

// titleAbove : le titre d'un tableau, cellule libre de texte juste au-dessus
// (une ou deux lignes) de sa première colonne ; "" s'il n'y en a pas.
func titleAbove(tb Table, cells map[[2]int]string) string {
	for d := 1; d <= 2; d++ {
		raw, ok := cells[[2]int{tb.Top() - d, tb.Col}]
		if !ok {
			continue
		}
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.HasPrefix(raw, "=") {
			return ""
		}
		return raw
	}
	return ""
}

func (t *tx) cellMap(sheet int64) (map[[2]int]string, error) {
	cells, err := t.cells(sheet)
	if err != nil {
		return nil, err
	}
	out := make(map[[2]int]string, len(cells))
	for _, c := range cells {
		out[[2]int{c.Row, c.Col}] = c.Raw
	}
	return out, nil
}

// stackedGroups : cas E, des tableaux semblables empilés sur une même
// feuille, chacun sous un titre différent.
func (t *tx) stackedGroups(cands []*candidate, taken map[int64]bool) ([]SheetGroup, error) {
	bySheet := map[int64][]*candidate{}
	var order []int64
	for _, c := range cands {
		if taken[c.tb.ID] {
			continue
		}
		if _, ok := bySheet[c.tb.Sheet]; !ok {
			order = append(order, c.tb.Sheet)
		}
		bySheet[c.tb.Sheet] = append(bySheet[c.tb.Sheet], c)
	}
	var out []SheetGroup
	for _, s := range order {
		cs := bySheet[s]
		if len(cs) < 2 {
			continue
		}
		cells, err := t.cellMap(s)
		if err != nil {
			return nil, err
		}
		// Les tableaux titrés, rangés par position ; ceux qui vont avec le
		// premier (colonnes, deux tiers).
		var titled []*candidate
		var titles []string
		for _, c := range cs {
			if title := titleAbove(c.tb, cells); title != "" {
				titled = append(titled, c)
				titles = append(titles, title)
			}
		}
		if len(titled) < 2 {
			continue
		}
		idx := make([]int, len(titled))
		for i := range idx {
			idx[i] = i
		}
		sort.SliceStable(idx, func(a, b int) bool {
			ta, tb := titled[idx[a]].tb, titled[idx[b]].tb
			if ta.Row != tb.Row {
				return ta.Row < tb.Row
			}
			return ta.Col < tb.Col
		})
		var members []*candidate
		var labels []string
		for _, i := range idx {
			c := titled[i]
			if len(members) > 0 {
				x := *c
				x.tb.Sheet = -1 // together compare des feuilles différentes
				if !together(members[0], &x) {
					continue
				}
			}
			if slices.Contains(labels, titles[i]) {
				continue
			}
			members = append(members, c)
			labels = append(labels, titles[i])
		}
		if len(members) < 2 {
			continue
		}
		g := buildGroup(members)
		g.Stacked = true
		g.Category = series.Recognize(labels)
		g.Name = groupName(g.Category, nil)
		if g.Name == "" {
			g.Name = familyName([]string{members[0].tb.Name})
		}
		if len(members) < 3 && g.Category.Score < series.Weak {
			continue
		}
		g.CategoryFrom = FromTitle
		g.Sources = []CategorySource{{Source: FromTitle, Label: "titre au-dessus de chaque tableau", Category: g.Category}}
		g.Families = []TableFamily{}
		g.Sheets = []SimilarSheet{{Sheet: s, Name: members[0].sheet, Hidden: members[0].hidden}}
		out = append(out, g)
	}
	return out, nil
}

// cellSources : cellules libres présentes à la même place sur tous les
// onglets du groupe, toutes différentes et qui forment une série (un titre
// « Janvier 2025 », « Février 2025 »…).
func (t *tx) cellSources(sheets []int64) ([]CategorySource, error) {
	if len(sheets) < 2 {
		return nil, nil
	}
	var maps []map[[2]int]string
	for _, s := range sheets {
		m, err := t.cellMap(s)
		if err != nil {
			return nil, err
		}
		maps = append(maps, m)
	}
	var pos [][2]int
	for p := range maps[0] {
		pos = append(pos, p)
	}
	sort.Slice(pos, func(i, j int) bool {
		if pos[i][0] != pos[j][0] {
			return pos[i][0] < pos[j][0]
		}
		return pos[i][1] < pos[j][1]
	})
	var out []CategorySource
	for _, p := range pos {
		vals := make([]string, 0, len(maps))
		for _, m := range maps {
			v, ok := m[p]
			if !ok || strings.HasPrefix(strings.TrimSpace(v), "=") || strings.TrimSpace(v) == "" {
				break
			}
			vals = append(vals, strings.TrimSpace(v))
		}
		if len(vals) < len(maps) || len(dedupe(vals)) < len(vals) {
			continue
		}
		s := series.Recognize(vals)
		if s.Kind == series.Other || s.Kind == series.Shape || s.Score < series.Medium {
			continue
		}
		a1 := formula.ColumnLetters(p[1]) + strconv.Itoa(p[0]+1)
		out = append(out, CategorySource{Source: fromCell + a1, Label: "cellule " + a1, Category: s})
		if len(out) >= 3 {
			break
		}
	}
	return out, nil
}

// parseCellSource : « cell:A1 » → ligne et colonne.
func parseCellSource(src string) (int, int, bool) {
	a1, ok := strings.CutPrefix(src, fromCell)
	if !ok {
		return 0, 0, false
	}
	a1 = strings.ToUpper(strings.TrimSpace(a1))
	col, i := 0, 0
	for i < len(a1) && a1[i] >= 'A' && a1[i] <= 'Z' {
		col = col*26 + int(a1[i]-'A'+1)
		i++
	}
	row, err := strconv.Atoi(a1[i:])
	if i == 0 || err != nil || row < 1 {
		return 0, 0, false
	}
	return row - 1, col - 1, true
}

// dataCategory : cas F, une colonne date dont les valeurs de chaque tableau
// tombent dans la période de son onglet (le mois, l'année, le jour) ; ""
// sinon.
func (t *tx) dataCategory(g SheetGroup, members []*candidate) (string, error) {
	cat := g.Category
	var width int // préfixe ISO commun : aaaa-mm (7), aaaa (4), aaaa-mm-jj (10)
	switch cat.Kind {
	case series.MonthYear:
		width = 7
	case series.Year:
		width = 4
	case series.Date:
		width = 10
	default:
		return "", nil
	}
	if len(cat.Values) != len(members) {
		return "", nil
	}
	for _, mc := range g.Columns {
		ok := true
		for i, m := range members {
			name := mc.From[i]
			k := colIndex(m.tb, name)
			if name == "" || k < 0 || m.tb.Cols[k].Type != grid.Date || m.tb.Cols[k].Formula != "" {
				ok = false
				break
			}
			c := colSQL(m.tb.Cols[k].ID)
			span, err := cachedOn(t, "minmax:"+m.tb.sql()+":"+c, []string{m.tb.sql()}, func() ([2]sql.NullString, error) {
				var v [2]sql.NullString
				err := t.row(`SELECT min(`+c+`), max(`+c+`) FROM `+m.tb.sql()).Scan(&v[0], &v[1])
				return v, err
			})
			if err != nil {
				return "", err
			}
			lo, hi := span[0], span[1]
			want := cat.Values[i]
			if len(want) < width || !lo.Valid || !hi.Valid || len(lo.String) < width || len(hi.String) < width ||
				lo.String[:width] != want[:width] || hi.String[:width] != want[:width] {
				ok = false
				break
			}
		}
		if ok {
			return mc.Name, nil
		}
	}
	return "", nil
}

// copiedSheets lit les copies d'onglets notées par « Copier la feuille »
// (nouvelle feuille → feuille copiée).
func (t *tx) copiedSheets() (map[int64]int64, error) {
	out := map[int64]int64{}
	var raw string
	err := t.row(`SELECT value FROM _lattis_info WHERE key = ?`, copiesKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]int64
	if json.Unmarshal([]byte(raw), &m) == nil {
		for k, v := range m {
			if id, err := strconv.ParseInt(k, 10, 64); err == nil {
				out[id] = v
			}
		}
	}
	return out, nil
}

// copiesKey : copies d'onglets (« Copier la feuille »), signal moyen de la
// détection (§2.3) et moment de la proposition (§4, 3).
const copiesKey = "sheets.copied"

// noteCopy retient, dans le journal, que la feuille id est une copie de from.
func (t *tx) noteCopy(id, from int64) error {
	var raw sql.NullString
	err := t.row(`SELECT value FROM _lattis_info WHERE key = ?`, copiesKey).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	m := map[string]int64{}
	if raw.Valid {
		_ = json.Unmarshal([]byte(raw.String), &m)
	}
	// La copie d'une copie remonte à l'original : les onglets d'une même
	// famille se reconnaissent entre eux.
	if src, ok := m[strconv.FormatInt(from, 10)]; ok {
		from = src
	}
	m[strconv.FormatInt(id, 10)] = from
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	set := `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`
	undo := []stmt{st(`DELETE FROM _lattis_info WHERE key = ?`, copiesKey)}
	if raw.Valid {
		undo = []stmt{st(set, copiesKey, raw.String)}
	}
	return t.apply([]stmt{st(set, copiesKey, string(b))}, undo)
}

// completeGroup ajoute à un groupe de feuilles ses onglets, les sources de
// catégorie et le cas F.
func (t *tx) completeGroup(g *SheetGroup, members []*candidate, copies map[int64]int64) error {
	if g.Stacked {
		return nil
	}
	inGroup := map[int64]bool{}
	var ids []int64
	for _, m := range members {
		inGroup[m.tb.Sheet] = true
		ids = append(ids, m.tb.Sheet)
	}
	for _, m := range members {
		ss := SimilarSheet{Sheet: m.tb.Sheet, Name: m.sheet, Hidden: m.hidden}
		if from, ok := copies[m.tb.Sheet]; ok && (inGroup[from] || copies[from] != 0 && inGroup[copies[from]]) {
			ss.Copied = true
			g.Copies++
		}
		// L'original d'une copie du groupe compte aussi.
		for c, from := range copies {
			if from == m.tb.Sheet && inGroup[c] && !ss.Copied {
				ss.Copied = true
				g.Copies++
				break
			}
		}
		g.Sheets = append(g.Sheets, ss)
	}
	g.CategoryFrom = FromSheet
	g.Sources = []CategorySource{{Source: FromSheet, Label: "nom de l'onglet", Category: g.Category}}
	cells, err := t.cellSources(ids)
	if err != nil {
		return err
	}
	g.Sources = append(g.Sources, cells...)
	// Une cellule en série plus sûre que les noms d'onglets (Feuille1,
	// Feuille2 ; « Mars (2) », une copie pas encore renommée) devient la
	// source proposée.
	for _, c := range cells {
		if c.Category.Score > g.Category.Score {
			g.Category, g.CategoryFrom = c.Category, c.Source
			break
		}
	}
	if len(g.Families) == 0 {
		dc, err := t.dataCategory(*g, members)
		if err != nil {
			return err
		}
		g.DataColumn = dc
	}
	return nil
}
