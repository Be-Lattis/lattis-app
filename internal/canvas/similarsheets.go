package canvas

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/series"
)

// Onglets semblables (docs/conception/regrouper-onglets-semblables.md, §2) :
// des tableaux de feuilles différentes aux mêmes colonnes (un onglet par
// mois, par année, par ville…), que MergeSheets réunit en un seul tableau
// avec une colonne de catégorie. La détection ne lit que les métadonnées
// (et le nombre de lignes, déjà en cache) : elle tourne à chaque ouverture
// et après chaque import sans se voir.

// SimilarTable est un tableau d'un groupe d'onglets semblables.
type SimilarTable struct {
	Table     string `json:"table"`
	Sheet     int64  `json:"sheet"`
	SheetName string `json:"sheetName"`
	Rows      int    `json:"rows"`
	Key       string `json:"key"` // colonne clé ("" : aucune)
	Hidden    bool   `json:"hidden"`
}

// MergeColumn est une colonne du tableau commun.
type MergeColumn struct {
	Name string    `json:"name"`
	Type grid.Type `json:"type"`
	// From : la colonne de chaque tableau source (dans l'ordre des
	// tableaux) qui la remplit ; "" : absente (cases vides).
	From []string `json:"from"`
	// Paired : appariée par un nom proche (« Montant € » → « Montant »),
	// montré dans le dialogue, jamais fait en silence.
	Paired []bool `json:"paired"`
	// Text : passe en texte parce que les types des onglets diffèrent.
	Text bool `json:"text"`
}

// SheetGroup est un groupe de tableaux semblables (§2.2).
type SheetGroup struct {
	// ID : signature du groupe (colonnes majoritaires), gardée par « Ne plus
	// proposer ».
	ID      string         `json:"id"`
	Tables  []SimilarTable `json:"tables"`
	Columns []MergeColumn  `json:"columns"`
	// Category : la catégorie tirée des noms d'onglets (§3).
	Category series.Series `json:"category"`
	Key      string        `json:"key"`
	// Name : nom proposé pour le tableau commun ("" : aucun d'évident).
	Name string `json:"name"`
	// Score : indice de confiance (§2.3) ; Propose : assez haut pour le
	// proposer de soi-même.
	Score     int  `json:"score"`
	Propose   bool `json:"propose"`
	Identical bool `json:"identical"`
	Dismissed bool `json:"dismissed"`
	// Summary : feuilles qui citent tous les tableaux du groupe (synthèse).
	Summary []string `json:"summary"`
	Notes   []string `json:"notes"`

	// Lot 4 (§6.5, similarfamilies.go) : on regroupe des feuilles entières.
	// Sheets : les onglets du groupe ; Families : leurs autres tableaux,
	// appariés famille par famille, avec ce qu'on propose d'en faire.
	Sheets   []SimilarSheet `json:"sheets"`
	Families []TableFamily  `json:"families"`
	// Sources : d'où peut venir la catégorie (noms d'onglets, titres des
	// tableaux empilés, cellule libre) ; CategoryFrom : celle proposée, que
	// décrit Category.
	Sources      []CategorySource `json:"sources"`
	CategoryFrom string           `json:"categoryFrom"`
	// Stacked : cas E, tableaux empilés sous des titres sur une feuille.
	Stacked bool `json:"stacked"`
	// DataColumn : cas F, la colonne (date) qui porte déjà la catégorie.
	DataColumn string `json:"dataColumn"`
	// Copies : onglets du groupe copiés les uns des autres.
	Copies int `json:"copies"`
}

// ProposeScore : indice à partir duquel le groupe est proposé de soi-même.
const ProposeScore = 60

// dismissedKey : groupes refusés (« Ne plus proposer »), dans _lattis_info.
const dismissedKey = "suggest.mergesheets.dismissed"

// foldColumn met un nom de colonne à plat (§2.1) : minuscules, sans
// accents, espaces resserrés, ponctuation de fin retirée, « n° » = « no » =
// « numéro ».
func foldColumn(name string) string {
	s := series.Fold(strings.ReplaceAll(strings.ReplaceAll(name, "°", "o "), "º", "o "))
	s = strings.TrimRightFunc(s, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSpace(r) })
	f := strings.Fields(s)
	for i, w := range f {
		switch w {
		case "no", "num", "numero", "n":
			f[i] = "no"
		}
	}
	return strings.Join(f, " ")
}

// near : deux noms proches (distance d'édition au plus 2, ou l'un contient
// l'autre : « Montant » / « Montant € »).
func near(a, b string) bool {
	la, lb := len([]rune(a)), len([]rune(b))
	if min(la, lb) < 3 {
		return false // « A » et « B » ne sont pas proches
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return true
	}
	return min(la, lb) >= 5 && editDistance(a, b) <= 2
}

func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// candidate est un tableau examiné par la détection.
type candidate struct {
	tb     Table
	sheet  string
	hidden bool
	rank   int // rang de la feuille parmi les onglets
	cols   []Column
	folded []string
	key    string // nom mis à plat de la colonne clé
}

// sameType : types qui vont ensemble sans passer en texte.
func sameType(a, b grid.Type) bool { return Join(a, b) != grid.Text || a == b }

// overlap compte les colonnes communes de deux tableaux (noms mis à plat,
// puis appariements par nom proche et même type).
func overlap(a, b *candidate) int {
	n := 0
	inB := map[string]bool{}
	for _, f := range b.folded {
		inB[f] = true
	}
	inA := map[string]bool{}
	for _, f := range a.folded {
		inA[f] = true
		if inB[f] {
			n++
		}
	}
	// Restes : une colonne qui ne correspond à rien s'apparie à une
	// colonne restante proche, de même type.
	used := map[int]bool{}
	for i, f := range a.folded {
		if inB[f] {
			continue
		}
		for j, g := range b.folded {
			if inA[g] || used[j] {
				continue
			}
			if near(f, g) && sameType(a.cols[i].Type, b.cols[j].Type) {
				used[j] = true
				n++
				break
			}
		}
	}
	return n
}

// together : deux tableaux vont ensemble (§2.2, règles 1 et 2).
func together(a, b *candidate) bool {
	if a.tb.Sheet == b.tb.Sheet {
		return false
	}
	n := overlap(a, b)
	big := max(len(a.folded), len(b.folded))
	if n < 2 || 3*n < 2*big {
		return false
	}
	// La clé est la même : des clés de noms différents ne vont pas
	// ensemble ; un tableau sans clé doit avoir la colonne clé de l'autre.
	switch {
	case a.key != "" && b.key != "":
		return a.key == b.key
	case a.key != "":
		return slices.Contains(b.folded, a.key)
	case b.key != "":
		return slices.Contains(a.folded, b.key)
	}
	return true
}

// excludedTables : tableaux qu'on ne regroupe pas (§1) : chargés par une
// requête, liés à un serveur ou au Data Lake, poussés comme source.
func (t *tx) excludedTables() (map[int64]string, error) {
	out := map[int64]string{}
	for q, why := range map[string]string{
		`SELECT target FROM _lattis_queries WHERE target IS NOT NULL`: "query",
		`SELECT tbl FROM _lattis_remote_tables`:                       "shared",
		`SELECT tbl FROM _lattis_data_sync`:                           "linked",
	} {
		rs, err := t.query(q)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var id int64
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return nil, err
			}
			out[id] = why
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return nil, err
		}
	}
	// Tableaux envoyés au Data Lake (bridge/table_source.go).
	var raw string
	err := t.row(`SELECT value FROM _lattis_info WHERE key = 'table_sources'`).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if raw != "" {
		var v struct {
			Links []struct {
				Tbl int64 `json:"tbl"`
			} `json:"links"`
		}
		if json.Unmarshal([]byte(raw), &v) == nil {
			for _, l := range v.Links {
				out[l.Tbl] = "source"
			}
		}
	}
	return out, nil
}

// protectedSheets : feuilles protégées (toutes si le contenu du classeur
// l'est).
func (t *tx) protectedSheets() (map[int64]bool, bool, error) {
	out := map[int64]bool{}
	all := false
	rs, err := t.query(`SELECT sheet FROM _lattis_protection`)
	if err != nil {
		return nil, false, err
	}
	defer rs.Close()
	for rs.Next() {
		var s int64
		if err := rs.Scan(&s); err != nil {
			return nil, false, err
		}
		out[s] = true
		all = all || s == sheetsScope
	}
	return out, all, rs.Err()
}

// shared : classeur partagé sur un Serveur Lattis (la fusion y est refusée
// au premier lot).
func (t *tx) shared() bool {
	return t.m != nil && t.m.sharing != nil && t.m.sharing.TableShare(0) != ""
}

// candidates lit les tableaux ordinaires de toutes les feuilles (masquées
// comprises), dans l'ordre des onglets.
func (t *tx) candidates() ([]*candidate, error) {
	order, err := t.sheetOrder()
	if err != nil {
		return nil, err
	}
	excluded, err := t.excludedTables()
	if err != nil {
		return nil, err
	}
	prot, all, err := t.protectedSheets()
	if err != nil {
		return nil, err
	}
	var out []*candidate
	for rank, sheet := range order {
		if all || prot[sheet] {
			continue
		}
		var name string
		var hidden bool
		if err := t.row(`SELECT name, hidden <> 0 FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&name, &hidden); err != nil {
			return nil, err
		}
		tables, err := t.tables(sheet)
		if err != nil {
			return nil, err
		}
		for _, tb := range tables {
			if _, ok := excluded[tb.ID]; ok {
				continue
			}
			c := &candidate{tb: tb, sheet: name, hidden: hidden, rank: rank}
			for _, col := range tb.Cols {
				// Colonnes techniques (_…) : hors de la comparaison (décision 0007).
				if strings.HasPrefix(col.Name, "_") {
					continue
				}
				c.cols = append(c.cols, col)
				c.folded = append(c.folded, foldColumn(col.Name))
				if col.ID == tb.Key {
					c.key = foldColumn(col.Name)
				}
			}
			if len(c.cols) >= 2 {
				out = append(out, c)
			}
		}
	}
	return out, nil
}

// SimilarSheets cherche les groupes d'onglets semblables (lecture pure).
func (m *Model) SimilarSheets(ctx context.Context) ([]SheetGroup, error) {
	out := []SheetGroup{}
	err := m.read(ctx, func(t *tx) error {
		var err error
		out, err = t.similarSheets()
		return err
	})
	return out, err
}

func (t *tx) similarSheets() ([]SheetGroup, error) {
	out := []SheetGroup{}
	if t.shared() {
		return out, nil
	}
	cands, err := t.candidates()
	if err != nil || len(cands) < 2 {
		return out, err
	}
	// Union-find sur les paires qui vont ensemble.
	parent := make([]int, len(cands))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for i := range cands {
		for j := i + 1; j < len(cands); j++ {
			if together(cands[i], cands[j]) {
				parent[find(i)] = find(j)
			}
		}
	}
	byRoot := map[int][]*candidate{}
	var roots []int
	for i, c := range cands {
		r := find(i)
		if _, ok := byRoot[r]; !ok {
			roots = append(roots, r)
		}
		byRoot[r] = append(byRoot[r], c)
	}
	dismissed, err := t.dismissedGroups()
	if err != nil {
		return nil, err
	}
	cites, err := t.sheetCitations()
	if err != nil {
		return nil, err
	}
	copies, err := t.copiedSheets()
	if err != nil {
		return nil, err
	}
	// Familles (lot 4) : les groupes vérifiés, puis les groupes de feuilles.
	var fams []*famCand
	for _, r := range roots {
		members := checkGroup(byRoot[r])
		if len(members) < 2 {
			continue
		}
		f := &famCand{members: members}
		if f.same, err = t.sameData(candTables(members)); err != nil {
			return nil, err
		}
		fams = append(fams, f)
	}
	groups, mains, err := t.sheetFamilies(fams, cands)
	if err != nil {
		return nil, err
	}
	taken := map[int64]bool{}
	for _, g := range groups {
		for _, tb := range g.Tables {
			for _, c := range cands {
				if c.tb.Name == tb.Table {
					taken[c.tb.ID] = true
				}
			}
		}
		for _, f := range g.Families {
			for _, tb := range f.Tables {
				for _, c := range cands {
					if c.tb.Name == tb.Table {
						taken[c.tb.ID] = true
					}
				}
			}
		}
	}
	stacked, err := t.stackedGroups(cands, taken)
	if err != nil {
		return nil, err
	}
	byName := map[string]*candidate{}
	for _, c := range cands {
		byName[c.tb.Name] = c
	}
	for _, g := range stacked {
		var ms []*candidate
		for _, tb := range g.Tables {
			ms = append(ms, byName[tb.Table])
		}
		groups, mains = append(groups, g), append(mains, ms)
	}
	for gi := range groups {
		g := groups[gi]
		members := mains[gi]
		if err := t.completeGroup(&g, members, copies); err != nil {
			return nil, err
		}
		g.Dismissed = dismissed[g.ID]
		// Une feuille qui cite tous les tableaux du groupe : une synthèse.
		inGroup := map[int64]bool{}
		for _, m := range members {
			inGroup[m.tb.Sheet] = true
		}
		for _, sc := range cites {
			if inGroup[sc.sheet] {
				continue
			}
			all := true
			for _, m := range members {
				if !sc.tables[strings.ToLower(m.tb.Name)] {
					all = false
					break
				}
			}
			if all {
				g.Summary = append(g.Summary, sc.name)
			}
		}
		g.Score = g.Category.Score
		switch {
		case g.Identical:
			g.Score += 40
		case allMajority(g):
			g.Score += 20
		}
		if len(g.Tables) >= 3 {
			g.Score += 10
		}
		if len(g.Summary) > 0 {
			g.Score += 20
		}
		if g.Copies > 0 {
			g.Score += series.Medium // onglets copiés les uns des autres (§2.3)
		}
		g.Propose = g.Score >= ProposeScore && !g.Dismissed
		out = append(out, g)
	}
	// Le plus grand groupe d'abord (la bulle ne parle que de lui).
	sort.SliceStable(out, func(i, j int) bool { return len(out[i].Tables) > len(out[j].Tables) })
	return out, nil
}

// allMajority : chaque tableau a toutes les colonnes du résultat (dans un
// ordre ou des noms un peu différents).
func allMajority(g SheetGroup) bool {
	for _, c := range g.Columns {
		if slices.Contains(c.From, "") {
			return false
		}
	}
	return true
}

// checkGroup vérifie un groupe formé de proche en proche (§2.2) : un seul
// tableau par feuille, une même clé, et chaque tableau garde au moins les
// deux tiers des colonnes majoritaires (le plus éloigné sort d'abord).
func checkGroup(members []*candidate) []*candidate {
	if len(members) < 2 {
		return members
	}
	// La clé la plus fréquente l'emporte ; les autres clés sortent.
	keys := map[string]int{}
	for _, m := range members {
		if m.key != "" {
			keys[m.key]++
		}
	}
	key, best := "", 0
	for k, n := range keys {
		if n > best || n == best && k < key {
			key, best = k, n
		}
	}
	var kept []*candidate
	for _, m := range members {
		if key != "" && (m.key != "" && m.key != key || m.key == "" && !slices.Contains(m.folded, key)) {
			continue
		}
		kept = append(kept, m)
	}
	members = kept
	for len(members) >= 2 {
		major := majority(members)
		// Un tableau par feuille : celui qui a le plus de colonnes majoritaires.
		bySheet := map[int64]*candidate{}
		for _, m := range members {
			if o, ok := bySheet[m.tb.Sheet]; !ok || shares(m, major) > shares(o, major) {
				bySheet[m.tb.Sheet] = m
			}
		}
		worst, worstShare := -1, 2.0
		var next []*candidate
		for _, m := range members {
			if bySheet[m.tb.Sheet] == m {
				next = append(next, m)
			}
		}
		members = next
		for i, m := range members {
			s := float64(shares(m, major)) / float64(max(1, len(major)))
			if s < worstShare {
				worst, worstShare = i, s
			}
		}
		if worstShare*3 >= 2-1e-9 || worst < 0 {
			break
		}
		members = slices.Delete(members, worst, worst+1)
	}
	sort.SliceStable(members, func(i, j int) bool {
		if members[i].rank != members[j].rank {
			return members[i].rank < members[j].rank
		}
		return members[i].tb.ID < members[j].tb.ID
	})
	return members
}

// majority : colonnes (mises à plat) présentes dans plus de la moitié des
// tableaux, dans l'ordre de leur première apparition.
func majority(members []*candidate) []string {
	count := map[string]int{}
	var order []string
	for _, m := range members {
		seen := map[string]bool{}
		for _, f := range m.folded {
			if seen[f] {
				continue
			}
			seen[f] = true
			if count[f] == 0 {
				order = append(order, f)
			}
			count[f]++
		}
	}
	var out []string
	for _, f := range order {
		if 2*count[f] > len(members) {
			out = append(out, f)
		}
	}
	return out
}

// shares compte les colonnes majoritaires d'un tableau (noms exacts ou
// proches).
func shares(m *candidate, major []string) int {
	n := 0
	for _, f := range major {
		for _, g := range m.folded {
			if f == g || near(f, g) {
				n++
				break
			}
		}
	}
	return n
}

// matchColumns fait correspondre les colonnes de tableaux à réunir : celles
// du premier dans leur ordre, puis celles ajoutées en cours de route, à la
// fin. Une colonne qui ne correspond à rien s'apparie à une colonne restante
// de nom proche et de même type.
func matchColumns(tables []Table) []MergeColumn {
	var out []MergeColumn
	var folded []string
	idx := map[string]int{}
	for ti, tb := range tables {
		has := map[int]bool{}
		var rest []Column
		for _, c := range tb.Cols {
			if strings.HasPrefix(c.Name, "_") {
				continue
			}
			f := foldColumn(c.Name)
			if k, ok := idx[f]; ok && !has[k] {
				out[k].From[ti] = c.Name
				out[k].Type = Join(out[k].Type, c.Type)
				has[k] = true
				continue
			}
			rest = append(rest, c)
		}
		for _, c := range rest {
			f := foldColumn(c.Name)
			k := -1
			for j := range out {
				if !has[j] && near(f, folded[j]) && sameType(out[j].Type, c.Type) {
					k = j
					break
				}
			}
			if k < 0 {
				col := MergeColumn{Name: c.Name, Type: c.Type, From: make([]string, len(tables)), Paired: make([]bool, len(tables))}
				out = append(out, col)
				folded = append(folded, f)
				k = len(out) - 1
				if _, ok := idx[f]; !ok {
					idx[f] = k
				}
			} else {
				out[k].Paired[ti] = true
				out[k].Type = Join(out[k].Type, c.Type)
			}
			out[k].From[ti] = c.Name
			has[k] = true
		}
	}
	for i := range out {
		for ti, tb := range tables {
			if n := out[i].From[ti]; n != "" {
				if c := colIndex(tb, n); c >= 0 && tb.Cols[c].Type != out[i].Type && out[i].Type == grid.Text {
					out[i].Text = true
				}
			}
		}
	}
	return out
}

// buildGroup décrit un groupe vérifié.
func buildGroup(members []*candidate) SheetGroup {
	g := SheetGroup{Summary: []string{}, Notes: []string{}}
	tables := make([]Table, len(members))
	sheets := make([]string, len(members))
	names := make([]string, len(members))
	for i, m := range members {
		tables[i] = m.tb
		sheets[i] = m.sheet
		names[i] = m.tb.Name
		st := SimilarTable{Table: m.tb.Name, Sheet: m.tb.Sheet, SheetName: m.sheet, Rows: m.tb.Rows, Hidden: m.hidden}
		for _, c := range m.tb.Cols {
			if c.ID == m.tb.Key {
				st.Key = c.Name
			}
		}
		if st.Key != "" {
			g.Key = st.Key
		}
		g.Tables = append(g.Tables, st)
	}
	g.Columns = matchColumns(tables)
	g.Identical = true
	for _, m := range members[1:] {
		if !slices.Equal(m.folded, members[0].folded) {
			g.Identical = false
		}
	}
	g.Category = series.Recognize(sheets)
	g.Name = groupName(g.Category, names)
	g.Notes = columnNotes(g.Columns, g.Tables)
	// Signature : les colonnes majoritaires, dans l'ordre.
	h := sha1.Sum([]byte(strings.Join(majority(members), "\x1f")))
	g.ID = hex.EncodeToString(h[:6])
	return g
}

// columnNotes : appariements et passages en texte, pour le dialogue.
func columnNotes(cols []MergeColumn, tables []SimilarTable) []string {
	notes := []string{}
	for _, c := range cols {
		for ti, p := range c.Paired {
			if p {
				notes = append(notes, "« "+c.From[ti]+" » ("+tables[ti].SheetName+") → « "+c.Name+" »")
			}
		}
		if c.Text {
			notes = append(notes, "« "+c.Name+" » passe en texte (types différents d'un onglet à l'autre)")
		}
	}
	return notes
}

// groupName propose le nom du tableau commun : la partie commune des noms
// d'onglets (« Ventes 2024 », « Ventes 2025 » → Ventes), sinon celle des
// noms de tableaux ; "" : aucun d'évident.
func groupName(cat series.Series, tables []string) string {
	switch cat.Kind {
	case series.MonthYear, series.Month, series.Year, series.Quarter, series.Week, series.Date:
		if n := identifier(cat.Common); len([]rune(n)) >= 2 {
			return n
		}
	}
	c := series.Recognize(tables)
	if n := identifier(c.Common); len([]rune(n)) >= 2 && !strings.EqualFold(n, identifier(cat.Column)) {
		return n
	}
	return ""
}

// sheetCite : tableaux cités par les formules d'une feuille.
type sheetCite struct {
	sheet  int64
	name   string
	tables map[string]bool
}

// sheetCitations relève, feuille par feuille, les tableaux cités par les
// formules des cellules libres.
func (t *tx) sheetCitations() ([]sheetCite, error) {
	rs, err := t.query(`SELECT c.sheet, s.name, c.raw FROM _lattis_cells c JOIN _lattis_sheets s ON s.id = c.sheet WHERE c.raw LIKE '=%' ORDER BY s.pos`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []sheetCite
	at := map[int64]int{}
	for rs.Next() {
		var sheet int64
		var name, raw string
		if err := rs.Scan(&sheet, &name, &raw); err != nil {
			return nil, err
		}
		n, err := formula.Parse(raw, formula.FR)
		if err != nil {
			continue
		}
		k, ok := at[sheet]
		if !ok {
			k = len(out)
			at[sheet] = k
			out = append(out, sheetCite{sheet: sheet, name: name, tables: map[string]bool{}})
		}
		for _, r := range refsOf(n, "") {
			if r.table != "" {
				out[k].tables[strings.ToLower(r.table)] = true
			}
		}
	}
	return out, rs.Err()
}

// dismissedGroups lit les groupes refusés.
func (t *tx) dismissedGroups() (map[string]bool, error) {
	out := map[string]bool{}
	var raw string
	err := t.row(`SELECT value FROM _lattis_info WHERE key = ?`, dismissedKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	if json.Unmarshal([]byte(raw), &ids) == nil {
		for _, id := range ids {
			out[id] = true
		}
	}
	return out, nil
}

// DismissSimilar range « Ne plus proposer » pour un groupe (sa signature)
// dans le fichier : un collègue qui reçoit le classeur n'est pas relancé
// non plus. Comme le zoom, hors du journal d'annulation, mais le classeur
// devient « modifié ».
func (m *Model) DismissSimilar(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return msg.New("sheet.groupUnspecified", nil)
	}
	changed := false
	err := m.db.Write(ctx, func(sqlTx *sql.Tx) error {
		var raw string
		err := sqlTx.QueryRowContext(ctx, `SELECT value FROM _lattis_info WHERE key = ?`, dismissedKey).Scan(&raw)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var ids []string
		if raw != "" {
			_ = json.Unmarshal([]byte(raw), &ids)
		}
		if slices.Contains(ids, id) {
			return nil
		}
		b, err := json.Marshal(append(ids, id))
		if err != nil {
			return err
		}
		changed = true
		_, err = sqlTx.ExecContext(ctx, `INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, dismissedKey, string(b))
		return err
	})
	if err == nil && changed {
		m.protMu.Lock()
		m.protGen++
		m.protMu.Unlock()
		m.cacheMu.Lock()
		m.cache = nil
		m.cacheMu.Unlock()
	}
	return err
}
