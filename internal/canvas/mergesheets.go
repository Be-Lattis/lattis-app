package canvas

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/lalamefine/lattis/internal/series"
)

// Regrouper des onglets (docs/conception/regrouper-onglets-semblables.md,
// §5 à §7) : les tableaux de plusieurs onglets aux mêmes colonnes deviennent
// un seul tableau, sur une nouvelle feuille, avec une colonne de catégorie
// en tête (Mois, Année, Ville…) tirée des noms d'onglets. Une seule étape
// d'annulation ; les lignes ne passent pas par le journal (comme Import).
//
// Ce qui citait les anciens tableaux (formules, TCD, graphiques) est repris
// sur le tableau commun filtré par la catégorie (lot 2, mergerefs.go) ; ce
// qui n'a pas d'équivalent exact est listé « à reprendre », et les anciens
// onglets sont alors gardés masqués plutôt que supprimés (décision de
// Louis, §12.1).

// ErrMerge signale un regroupement impossible.
var ErrMergeSheets = errors.New("regrouper des onglets")

// Que deviennent les anciens onglets (MergePlan.Old).
const (
	MergeOldDelete = "delete" // supprimés (Ctrl+Z les rend)
	MergeOldHide   = "hide"   // masqués, tableaux renommés Janvier_ancien
)

// MergeCategory est la colonne de catégorie du tableau commun.
type MergeCategory struct {
	Name string    `json:"name"`
	Type grid.Type `json:"type"` // date, integer ou text
	// Values : une valeur par tableau source, dans l'ordre (date jj/mm/aaaa
	// ou aaaa-mm-jj, entier, texte).
	Values []string `json:"values"`
	// Format : motif d'affichage d'une date (« mmmm aaaa »).
	Format string `json:"format,omitempty"`
}

// MergePlan décrit un regroupement. Ce qui manque se déduit des tableaux,
// comme la détection (SimilarSheets) le propose.
type MergePlan struct {
	// Tables : tableaux sources, dans l'ordre des lignes du résultat.
	Tables []string `json:"tables"`
	// Category : colonne de catégorie (nil : déduite des noms d'onglets) ;
	// NoCategory : pas de colonne de catégorie (cas F, la catégorie est déjà
	// dans les données).
	Category   *MergeCategory `json:"category,omitempty"`
	NoCategory bool           `json:"noCategory,omitempty"`
	// Columns : correspondance validée des colonnes (From : une colonne par
	// tableau source, "" si absente) ; vide : déduite.
	Columns []MergeColumn `json:"columns,omitempty"`
	// Sheet : nom de la nouvelle feuille (vide : celui du tableau) ; Name :
	// nom du tableau commun (vide : proposé).
	Sheet string `json:"sheet,omitempty"`
	Name  string `json:"name,omitempty"`
	// Old : delete ou hide ; vide : supprimés, sauf s'il reste des formules
	// à reprendre (gardés masqués).
	Old string `json:"old,omitempty"`
	// Slicer : poser l'onglet à choix sur la nouvelle feuille (lot 3).
	Slicer bool `json:"slicer,omitempty"`

	// Lot 4 (§6.5, mergefamilies.go). CategoryFrom : d'où vient la
	// catégorie : "sheet" ou vide (noms d'onglets), "title" (titre au-dessus
	// de chaque tableau, cas E), "cell:A1" (cellule libre de chaque onglet).
	CategoryFrom string `json:"categoryFrom,omitempty"`
	// Families : les autres tableaux des onglets et ce qu'on en fait
	// (merge, once, leave).
	Families []MergeFamily `json:"families,omitempty"`
	// NoCells : ne pas reprendre les cellules libres des onglets (titres,
	// totaux, objectifs) sur la feuille commune.
	NoCells bool `json:"noCells,omitempty"`

	// Pour les familles : un seul tableau permis ; tableaux dont le nom se
	// libère (sources de tout le regroupement) ; noms déjà pris par les
	// autres tableaux communs.
	single  bool
	exclude map[int64]bool
	taken   map[string]bool
}

// PendingRef est ce qui citait un ancien tableau (formule, colonne
// calculée, règle de mise en forme, TCD, graphique, Document, contrôle,
// requête) : repris (After, la formule ou l'objet d'après) ou à reprendre à
// la main (Why, pourquoi).
type PendingRef struct {
	Kind      string `json:"kind"` // cell, column, rule, pivot, chart, document, control, query
	Sheet     int64  `json:"sheet"`
	SheetName string `json:"sheetName"`
	Row       int    `json:"row"`
	Col       int    `json:"col"`
	Where     string `json:"where"`
	Formula   string `json:"formula"`
	After     string `json:"after,omitempty"`
	Why       string `json:"why,omitempty"`
}

// MergePreview est l'aperçu d'un regroupement, pour le dialogue.
type MergePreview struct {
	Plan   MergePlan      `json:"plan"` // complété
	Tables []SimilarTable `json:"tables"`
	Header []string       `json:"header"`
	Rows   [][]any        `json:"rows"` // 20 premières lignes
	Total  int            `json:"total"`
	// Key : colonne clé ; KeyKept : gardée sur le tableau commun ; KeyNote :
	// pourquoi elle ne l'est pas (« Pas de clé : F-001 est dans Janvier et
	// dans Février »).
	Key     string `json:"key"`
	KeyKept bool   `json:"keyKept"`
	KeyNote string `json:"keyNote"`
	// Rewritten : ce qui sera repris sur le tableau commun filtré ; Pending :
	// ce qui reste à reprendre à la main.
	Rewritten []PendingRef `json:"rewritten"`
	Pending   []PendingRef `json:"pending"`
	// Old : ce que deviennent les anciens onglets si rien n'est choisi.
	Old   string   `json:"old"`
	Notes []string `json:"notes"`
	// Kept : anciens onglets qui resteront même supprimés, et pourquoi
	// (ils contiennent autre chose : voir keptSheets).
	Kept []string `json:"kept"`
	// Refused : le regroupement est refusé (garde-fou), et pourquoi.
	Refused string `json:"refused"`
	// Lot 4 : ce que devient chaque famille de tableaux (la principale
	// d'abord), la disposition de la feuille commune, les cellules libres
	// reprises (dont par valeur de la catégorie) et celles qui tombaient sur
	// un tableau commun.
	Families []FamilyOutcome `json:"families"`
	Layout   []LayoutItem    `json:"layout"`
	Cells    int             `json:"cells"`
	PerValue int             `json:"perValue"`
	Lost     int             `json:"lost"`
}

// MergeResult décrit ce que MergeSheets a fait.
type MergeResult struct {
	Sheet     int64        `json:"sheet"`
	SheetName string       `json:"sheetName"`
	Table     string       `json:"table"`
	Rows      int          `json:"rows"`
	Old       string       `json:"old"`
	KeyKept   bool         `json:"keyKept"`
	KeyNote   string       `json:"keyNote"`
	Rewritten []PendingRef `json:"rewritten"`
	Pending   []PendingRef `json:"pending"`
	Notes     []string     `json:"notes"`
	Notice    string       `json:"notice"`
	// Lot 4 : les tableaux communs (famille principale d'abord) et les
	// cellules libres reprises.
	Families []FamilyOutcome `json:"families"`
	Cells    int             `json:"cells"`
	PerValue int             `json:"perValue"`
}

// mergeSource est un tableau source préparé.
type mergeSource struct {
	tb    Table
	sheet string
	cat   any   // valeur de la catégorie, au type de stockage
	from  []int // par colonne du résultat : rang dans tb.Cols, -1 si absente
	off   int64 // décalage des identifiants de lignes
	base  int   // lignes des sources précédentes
	maxID int64
}

// mergeJob est un regroupement préparé.
type mergeJob struct {
	plan     MergePlan
	srcs     []*mergeSource
	cols     []MergeColumn
	formulas []string // formule gardée (anglais), ou ""
	valued   []bool   // colonne calculée gardée en valeurs
	key      int      // rang de la colonne clé, -1 sans clé
	catLabel []string // valeurs de la catégorie telles qu'on les saisit
	total    int
	notes    []string
	refused  string
}

func (j *mergeJob) hasCat() bool { return j.plan.Category != nil }

// prepareMerge vérifie et complète un plan. Un garde-fou (feuille protégée,
// tableau filtré, chargé par une requête…) n'est pas une erreur : il est
// rendu dans refused (l'aperçu le montre, MergeSheets le renvoie).
func (t *tx) prepareMerge(plan MergePlan) (*mergeJob, error) {
	j := &mergeJob{plan: plan, key: -1}
	if len(plan.Tables) < 2 && !(plan.single && len(plan.Tables) == 1) {
		return nil, msg.Wrap(ErrMergeSheets, "mergeSheets.chooseTwoTables", nil)
	}
	switch plan.Old {
	case "", MergeOldDelete, MergeOldHide:
	default:
		return nil, msg.Wrap(ErrMergeSheets, "mergeSheets.oldSheets", msg.Args{"old": plan.Old})
	}
	excluded, err := t.excludedTables()
	if err != nil {
		return nil, err
	}
	refuse := func(s string) {
		if j.refused == "" {
			j.refused = s
		}
	}
	if t.shared() {
		refuse(t.say("mergeSheets.sharedWorkbook", nil))
	}
	if err := t.guardWorkbook(); err != nil {
		refuse(msg.Text(t.m.Language(), err))
	}
	tables := make([]Table, len(plan.Tables))
	seen := map[int64]bool{}
	for i, name := range plan.Tables {
		tb, err := t.tableByName(name)
		if err != nil {
			return nil, err
		}
		if seen[tb.ID] {
			return nil, msg.Wrap(ErrMergeSheets, "mergeSheets.tableTwice", msg.Args{"table": tb.Name})
		}
		seen[tb.ID] = true
		tables[i] = tb
		var sheet string
		if err := t.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, tb.Sheet).Scan(&sheet); err != nil {
			return nil, err
		}
		j.srcs = append(j.srcs, &mergeSource{tb: tb, sheet: sheet})
		if why, ok := excluded[tb.ID]; ok {
			refuse(t.say("mergeSheets.excluded", msg.Args{"table": tb.Name, "why": t.say("mergeSheets.why."+msg.ID(why), nil)}))
		}
		if err := t.guard(tb.Sheet); err != nil {
			refuse(msg.Text(t.m.Language(), err))
		}
		if err := t.lockTable(tb.ID); err != nil {
			refuse(err.Error())
		}
		if tb.Filtered() {
			refuse(t.say("mergeSheets.filtered", msg.Args{"table": tb.Name}))
		}
	}
	// Colonnes.
	if len(plan.Columns) == 0 {
		j.cols = matchColumns(tables)
	} else {
		for _, c := range plan.Columns {
			c.Name = strings.TrimSpace(c.Name)
			if c.Name == "" {
				return nil, msg.Wrap(ErrMergeSheets, "mergeSheets.unnamedColumn", nil)
			}
			if len(c.From) != len(tables) {
				return nil, msg.Wrap(ErrMergeSheets, "mergeSheets.sourcePerTable", msg.Args{"column": c.Name, "n": len(tables)})
			}
			if len(c.Paired) != len(tables) {
				c.Paired = make([]bool, len(tables))
			}
			typ := grid.Type("")
			c.Text = false
			for ti, n := range c.From {
				if n == "" {
					continue
				}
				k := colIndex(tables[ti], n)
				if k < 0 {
					return nil, msg.Wrap(ErrMergeSheets, "mergeSheets.noColumn", msg.Args{"table": tables[ti].Name, "column": n})
				}
				c.From[ti] = tables[ti].Cols[k].Name
				typ = Join(typ, tables[ti].Cols[k].Type)
			}
			if typ == "" {
				typ = grid.Text
			}
			if c.Type == "" || Join(c.Type, typ) != c.Type {
				c.Type = typ
			}
			for ti, n := range c.From {
				if k := colIndex(tables[ti], n); n != "" && k >= 0 && tables[ti].Cols[k].Type != c.Type && c.Type == grid.Text {
					c.Text = true
				}
			}
			j.cols = append(j.cols, c)
		}
	}
	if len(j.cols) == 0 {
		return nil, msg.Wrap(ErrMergeSheets, "mergeSheets.noColumns", nil)
	}
	names := map[string]bool{}
	for _, c := range j.cols {
		low := strings.ToLower(c.Name)
		if names[low] {
			return nil, msg.Wrap(ErrMergeSheets, "mergeSheets.duplicateColumn", msg.Args{"column": c.Name})
		}
		names[low] = true
	}
	for _, s := range j.srcs {
		s.from = make([]int, len(j.cols))
	}
	for i, c := range j.cols {
		for ti, s := range j.srcs {
			s.from[i] = -1
			if c.From[ti] != "" {
				s.from[i] = colIndex(s.tb, c.From[ti])
			}
		}
	}
	// Catégorie (§3).
	if !plan.NoCategory {
		labels, err := t.categoryLabels(plan.CategoryFrom, j.srcs)
		if err != nil {
			return nil, err
		}
		rec := t.localSeries(series.Recognize(labels))
		cat := plan.Category
		if cat == nil {
			cat = &MergeCategory{Name: rec.Column, Type: rec.Type, Format: rec.Format}
		} else {
			c := *cat
			cat = &c
		}
		cat.Name = strings.TrimSpace(cat.Name)
		if cat.Name == "" {
			cat.Name = rec.Column
		}
		if cat.Type == "" {
			cat.Type, cat.Format = rec.Type, rec.Format
		}
		switch cat.Type {
		case grid.Date, grid.Integer, grid.Text:
		default:
			return nil, msg.Wrap(ErrMergeSheets, "mergeSheets.categoryType", msg.Args{"type": cat.Type})
		}
		if len(cat.Values) == 0 {
			cat.Values = rec.Labels
			if cat.Type != rec.Type {
				cat.Values = labels
			}
		}
		if len(cat.Values) != len(j.srcs) {
			return nil, msg.Wrap(ErrMergeSheets, "mergeSheets.categoryPerTable", msg.Args{"n": len(j.srcs)})
		}
		cat.Values = slices.Clone(cat.Values)
		for i, v := range cat.Values {
			v = strings.TrimSpace(v)
			cat.Values[i] = v
			val, err := grid.Parse(cat.Type, v)
			if err != nil || val == nil {
				return nil, msg.Wrap(ErrMergeSheets, "mergeSheets.categoryValueInvalid", msg.Args{"value": v, "sheet": j.srcs[i].sheet, "type": typeName(cat.Type)})
			}
			j.srcs[i].cat = val
			j.catLabel = append(j.catLabel, dateLabel(val, cat.Type, v))
		}
		// Un nom déjà pris par une colonne : « Mois (onglet) ».
		if names[strings.ToLower(cat.Name)] {
			base := cat.Name
			cat.Name = base + " (onglet)"
			for k := 2; names[strings.ToLower(cat.Name)]; k++ {
				cat.Name = fmt.Sprintf("%s (onglet %d)", base, k)
			}
			j.notes = append(j.notes, t.say("mergeSheets.categoryNameTaken", msg.Args{"column": base, "category": cat.Name}))
		}
		j.plan.Category = cat
	} else {
		j.plan.Category = nil
	}
	// Nom du tableau et de la feuille.
	name := strings.TrimSpace(plan.Name)
	if name == "" {
		srcNames := make([]string, len(j.srcs))
		labels := make([]string, len(j.srcs))
		for i, s := range j.srcs {
			srcNames[i], labels[i] = s.tb.Name, s.sheet
		}
		name = groupName(series.Recognize(labels), srcNames)
	}
	if name == "" {
		name = t.defaultName("name.merged")
	}
	if id := identifier(name); id != "" {
		name = id
	} else {
		name = identifier(t.defaultName("name.merged"))
	}
	if A1Like(name) {
		name += "_"
	}
	exclude := seen
	if plan.exclude != nil {
		exclude = plan.exclude
	}
	free, err := t.mergeTableName(name, exclude, plan.taken)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(free, name) && strings.TrimSpace(plan.Name) != "" {
		j.notes = append(j.notes, t.say("mergeSheets.tableNameTaken", msg.Args{"name": name, "table": free}))
	}
	j.plan.Name = free
	if strings.TrimSpace(plan.Sheet) == "" {
		// La feuille garde l'écriture des onglets (« Suivi des nids_2026 »
		// → « Suivi des nids »), le tableau un nom d'identifiant.
		labels := make([]string, len(j.srcs))
		for i, s := range j.srcs {
			labels[i] = s.sheet
		}
		common := strings.Trim(series.Recognize(labels).Common, " _-.·")
		switch {
		case len([]rune(common)) >= 2 && identifier(common) == j.plan.Name:
			j.plan.Sheet = common
		case strings.TrimSpace(plan.Name) != "":
			j.plan.Sheet = strings.TrimSpace(plan.Name)
		default:
			j.plan.Sheet = j.plan.Name
		}
	} else {
		j.plan.Sheet = strings.TrimSpace(plan.Sheet)
	}
	// Colonnes calculées (§7, étape 4) : gardées en formule si elle est la
	// même partout (noms des tableaux reportés), sinon en valeurs.
	// Une formule qui lit son tableau entier par son nom (SOMME(Janvier[X]),
	// le total du mois) changerait de sens sur le tableau commun (le total de
	// l'année) : la colonne garde ses valeurs.
	retarget := map[string]string{}
	whole := map[string]formula.Filtered{}
	for _, s := range j.srcs {
		retarget[strings.ToLower(s.tb.Name)] = j.plan.Name
		whole[strings.ToLower(s.tb.Name)] = formula.Filtered{}
	}
	j.formulas = make([]string, len(j.cols))
	j.valued = make([]bool, len(j.cols))
	for i, c := range j.cols {
		var fs []string
		some, all, cites := false, true, false
		for _, s := range j.srcs {
			k := s.from[i]
			if k < 0 {
				all = false
				continue
			}
			f := s.tb.Cols[k].Formula
			if f == "" {
				all = false
				continue
			}
			some = true
			if n, err := formula.Parse(f, formula.EN); err == nil {
				if _, c, _ := formula.FilterTables(n, whole); c {
					cites = true
				}
				if x, ok := formula.RetargetTables(n, retarget); ok {
					n = x
				}
				f = formula.Format(n, formula.EN)
			}
			fs = append(fs, f)
		}
		switch {
		case !some:
		case cites:
			j.valued[i] = true
			j.notes = append(j.notes, t.say("mergeSheets.formulaReadsTable", msg.Args{"column": c.Name}))
		case all && len(slices.Compact(slices.Clone(fs))) == 1:
			j.formulas[i] = fs[0]
		default:
			j.valued[i] = true
			j.notes = append(j.notes, t.say("mergeSheets.formulaDiffers", msg.Args{"column": c.Name}))
		}
		// Valeurs gardées : le type est celui des résultats (une part en
		// décimal dans une colonne d'abord saisie en entiers), quand il se
		// devine pour chaque onglet.
		if j.valued[i] {
			typ, known := grid.Type(""), true
			for _, s := range j.srcs {
				k := s.from[i]
				if k < 0 {
					continue
				}
				sc := s.tb.Cols[k]
				r := sc.Type
				if sc.Formula != "" {
					r = t.computedResult(s.tb, sc)
				}
				if r == "" {
					known = false
					break
				}
				typ = Join(typ, r)
			}
			if known && typ != "" {
				j.cols[i].Type = typ
			}
		}
	}
	// Clé : la colonne du résultat qui porte la clé des sources.
	for i := range j.cols {
		for _, s := range j.srcs {
			if k := s.from[i]; k >= 0 && s.tb.Key != 0 && s.tb.Cols[k].ID == s.tb.Key && j.formulas[i] == "" && !j.valued[i] {
				j.key = i
			}
		}
	}
	// Identifiants et ordre des lignes.
	var off int64
	for _, s := range j.srcs {
		if err := t.row(`SELECT coalesce(max(id), 0) FROM ` + s.tb.sql()).Scan(&s.maxID); err != nil {
			return nil, err
		}
		s.off, s.base = off, j.total
		off += s.maxID
		j.total += s.tb.Rows
	}
	for _, c := range j.cols {
		for ti, p := range c.Paired {
			if p {
				j.notes = append(j.notes, t.say("mergeSheets.columnRenamed", msg.Args{"from": c.From[ti], "sheet": j.srcs[ti].sheet, "column": c.Name}))
			}
		}
		if c.Text {
			j.notes = append(j.notes, t.say("mergeSheets.columnToText", msg.Args{"column": c.Name}))
		}
	}
	j.plan.Columns = j.cols
	j.plan.Tables = make([]string, len(j.srcs))
	for i, s := range j.srcs {
		j.plan.Tables[i] = s.tb.Name
	}
	return j, nil
}

func typeName(t grid.Type) string {
	switch t {
	case grid.Date:
		return "date"
	case grid.Integer:
		return "nombre entier"
	}
	return "texte"
}

// dateLabel : la valeur telle qu'on la saisit (une date en jj/mm/aaaa).
func dateLabel(v any, t grid.Type, raw string) string {
	if t == grid.Date {
		if s, ok := v.(string); ok {
			return grid.ShowStored(s)
		}
	}
	if t == grid.Integer {
		return fmt.Sprint(v)
	}
	return raw
}

// mergeTableName rend le nom libre le plus proche (Ventes, Ventes_2…) parmi
// les tableaux, sauf les sources (supprimées ou renommées), et les TCD.
func (t *tx) mergeTableName(name string, sources map[int64]bool, also map[string]bool) (string, error) {
	ps, err := t.allPivots()
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
	for n := range also {
		taken[strings.ToLower(n)] = true
	}
	for _, p := range ps {
		taken[strings.ToLower(p.Cfg.Name)] = true
	}
	rs, err := t.query(`SELECT id, name FROM _lattis_tables`)
	if err != nil {
		return "", err
	}
	for rs.Next() {
		var id int64
		var n string
		if err := rs.Scan(&id, &n); err != nil {
			rs.Close()
			return "", err
		}
		if !sources[id] {
			taken[strings.ToLower(n)] = true
		}
	}
	rs.Close()
	cand := name
	for k := 2; taken[strings.ToLower(cand)]; k++ {
		cand = fmt.Sprintf("%s_%d", name, k)
	}
	return cand, rs.Err()
}

// sourceSelect : les colonnes d'un tableau source (id, rang, puis une
// expression par colonne affichée, e0…), colonnes calculées comprises.
func (t *tx) sourceSelect(s *mergeSource, rn bool) (string, []any, error) {
	needSQL := false
	for _, c := range s.tb.Cols {
		needSQL = needSQL || c.Formula != ""
	}
	var exprs []string
	var args []any
	if needSQL {
		var err error
		if exprs, args, _, err = t.columnSQL(s.tb); err != nil {
			return "", nil, err
		}
	} else {
		for _, c := range s.tb.Cols {
			exprs = append(exprs, s.tb.sql()+"."+colSQL(c.ID))
		}
	}
	sel := []string{s.tb.sql() + ".id AS id"}
	if rn {
		sel = append(sel, "row_number() OVER (ORDER BY "+s.tb.sql()+"._pos) AS rn")
	} else {
		sel = append(sel, s.tb.sql()+"._pos AS p")
	}
	for k, e := range exprs {
		sel = append(sel, fmt.Sprintf("%s AS e%d", e, k))
	}
	return `SELECT ` + strings.Join(sel, ", ") + ` FROM ` + s.tb.sql(), args, nil
}

// valueExpr : la valeur de la colonne i du résultat pour une source (q :
// la sous-requête de sourceSelect).
func (j *mergeJob) valueExpr(s *mergeSource, i int) string {
	k := s.from[i]
	if k < 0 || j.formulas[i] != "" {
		return "NULL"
	}
	c := s.tb.Cols[k]
	x := fmt.Sprintf("q.e%d", k)
	to := j.cols[i].Type
	if c.Formula == "" {
		return convertExpr(x, c.Type, to)
	}
	// Valeurs d'une colonne calculée : gardées si elles vont au type.
	switch to {
	case grid.Integer, grid.Boolean:
		return fmt.Sprintf("CASE WHEN typeof(%[1]s) = 'integer' THEN %[1]s WHEN typeof(%[1]s) = 'real' AND %[1]s = CAST(%[1]s AS INTEGER) THEN CAST(%[1]s AS INTEGER) END", x)
	case grid.Decimal:
		return fmt.Sprintf("CASE WHEN typeof(%[1]s) IN ('integer', 'real') THEN CAST(%[1]s AS REAL) END", x)
	}
	return fmt.Sprintf("CASE WHEN %[1]s IS NOT NULL THEN CAST(%[1]s AS TEXT) END", x)
}

// keyCheck cherche une valeur de clé présente deux fois (sans tenir compte
// de la casse, comme l'index de la clé) : vide si la clé peut être gardée,
// sinon la raison, avec un exemple.
func (t *tx) mergeKeyCheck(j *mergeJob) (string, error) {
	if j.key < 0 {
		return "", nil
	}
	typ := j.cols[j.key].Type
	var parts []string
	for si, s := range j.srcs {
		k := s.from[j.key]
		if k < 0 {
			return t.say("mergeSheets.noKeyColumn", msg.Args{"sheet": s.sheet, "column": j.cols[j.key].Name}), nil
		}
		c := s.tb.Cols[k]
		x := convertExpr(s.tb.sql()+"."+colSQL(c.ID), c.Type, typ)
		parts = append(parts, fmt.Sprintf("SELECT %s AS k, %s AS v, %d AS s FROM %s", formula.KeyExpr(x, typ), x, si, s.tb.sql()))
	}
	var v sql.NullString
	var a, b, n int
	// Une seule requête sur l'union des colonnes clés (chaque table a déjà
	// son index sur la clé).
	err := t.row(`SELECT min(v), min(s), max(s), count(*) FROM (`+strings.Join(parts, " UNION ALL ")+`) GROUP BY k HAVING count(*) > 1 LIMIT 1`).Scan(&v, &a, &b, &n)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	val := v.String
	if !v.Valid || val == "" {
		val = "(vide)"
	}
	if a == b {
		return t.say("mergeSheets.noKeyRepeated", msg.Args{"value": val, "n": n, "sheet": j.srcs[a].sheet}), nil
	}
	return t.say("mergeSheets.noKeyTwoSheets", msg.Args{"value": val, "sheet": j.srcs[a].sheet, "other": j.srcs[b].sheet}), nil
}

// deletableSheets : feuilles sources qui ne contiendront plus rien une fois
// leurs tableaux regroupés (ni autre tableau, ni TCD, graphique, bouton ou
// page) : elles partent avec eux. Leurs cellules libres (titres, totaux :
// des légendes, décision 0005) partent aussi.
func (t *tx) deletableSheets(tables []Table) (map[int64]bool, error) {
	src := map[int64]bool{}
	for _, tb := range tables {
		src[tb.ID] = true
	}
	out := map[int64]bool{}
	for _, tb := range tables {
		if _, done := out[tb.Sheet]; done {
			continue
		}
		ok := true
		rs, err := t.query(`SELECT id FROM _lattis_tables WHERE sheet = ?`, tb.Sheet)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var id int64
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return nil, err
			}
			ok = ok && src[id]
		}
		rs.Close()
		var objs int
		if err := t.row(`SELECT (SELECT count(*) FROM _lattis_pivots WHERE sheet = ?1) + (SELECT count(*) FROM _lattis_charts WHERE sheet = ?1)
			+ (SELECT count(*) FROM _lattis_buttons WHERE sheet = ?1) + (SELECT count(*) FROM _lattis_pages WHERE sheet = ?1)`, tb.Sheet).Scan(&objs); err != nil {
			return nil, err
		}
		out[tb.Sheet] = ok && objs == 0
	}
	return out, nil
}

// catLiteral écrit une valeur de catégorie (entier ou texte) en SQL.
func catLiteral(v any) string {
	switch v := v.(type) {
	case int64:
		return fmt.Sprint(v)
	case float64:
		return fmt.Sprint(v)
	case nil:
		return "NULL"
	}
	return "'" + strings.ReplaceAll(fmt.Sprint(v), "'", "''") + "'"
}

func dedupe(vals []string) []string {
	var out []string
	for _, v := range vals {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// refCounts : « 14 formules reprises, 2 à reprendre à la main ».
func (t *tx) refCounts(done, left int) string {
	var parts []string
	if done > 0 {
		parts = append(parts, t.say("mergeSheets.refsDone", msg.Args{"n": done}))
	}
	if left > 0 {
		id := msg.ID("mergeSheets.refsLeft")
		if done == 0 {
			id = "mergeSheets.refsLeftAlone"
		}
		parts = append(parts, t.say(id, msg.Args{"n": left}))
	}
	return strings.Join(parts, ", ")
}
