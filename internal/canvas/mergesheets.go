package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/order"
	"github.com/lalamefine/lattis/internal/series"
)

// Regrouper des onglets (docs/conception/regrouper-onglets-semblables.md,
// §5 à §7) : les tableaux de plusieurs onglets aux mêmes colonnes deviennent
// un seul tableau, sur une nouvelle feuille, avec une colonne de catégorie
// en tête (Mois, Année, Ville…) tirée des noms d'onglets. Une seule étape
// d'annulation ; les lignes ne passent pas par le journal (comme Import).
//
// Premier lot : les formules, TCD et graphiques qui citaient les anciens
// tableaux ne sont pas réécrits (lot 2) ; ils sont listés « à reprendre »,
// et les anciens onglets sont alors gardés masqués plutôt que supprimés
// (décision de Louis, §12.1).

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
}

// PendingRef est ce qui citait un ancien tableau et reste à reprendre à la
// main (formule, colonne calculée, TCD, graphique, requête).
type PendingRef struct {
	Kind      string `json:"kind"` // cell, column, pivot, chart, query
	Sheet     int64  `json:"sheet"`
	SheetName string `json:"sheetName"`
	Row       int    `json:"row"`
	Col       int    `json:"col"`
	Where     string `json:"where"`
	Formula   string `json:"formula"`
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
	Key     string       `json:"key"`
	KeyKept bool         `json:"keyKept"`
	KeyNote string       `json:"keyNote"`
	Pending []PendingRef `json:"pending"`
	// Old : ce que deviennent les anciens onglets si rien n'est choisi.
	Old   string   `json:"old"`
	Notes []string `json:"notes"`
	// Refused : le regroupement est refusé (garde-fou), et pourquoi.
	Refused string `json:"refused"`
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
	Pending   []PendingRef `json:"pending"`
	Notes     []string     `json:"notes"`
	Notice    string       `json:"notice"`
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
	if len(plan.Tables) < 2 {
		return nil, fmt.Errorf("%w : choisissez au moins deux tableaux", ErrMergeSheets)
	}
	switch plan.Old {
	case "", MergeOldDelete, MergeOldHide:
	default:
		return nil, fmt.Errorf("%w : anciens onglets « %s » : delete ou hide attendu", ErrMergeSheets, plan.Old)
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
		refuse("le classeur est partagé sur un Serveur Lattis : le regroupement d'onglets n'y est pas encore possible")
	}
	if err := t.guardWorkbook(); err != nil {
		refuse(err.Error())
	}
	tables := make([]Table, len(plan.Tables))
	seen := map[int64]bool{}
	for i, name := range plan.Tables {
		tb, err := t.tableByName(name)
		if err != nil {
			return nil, err
		}
		if seen[tb.ID] {
			return nil, fmt.Errorf("%w : le tableau « %s » est cité deux fois", ErrMergeSheets, tb.Name)
		}
		seen[tb.ID] = true
		tables[i] = tb
		var sheet string
		if err := t.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, tb.Sheet).Scan(&sheet); err != nil {
			return nil, err
		}
		j.srcs = append(j.srcs, &mergeSource{tb: tb, sheet: sheet})
		if why, ok := excluded[tb.ID]; ok {
			refuse(fmt.Sprintf("« %s » est %s", tb.Name, why))
		}
		if err := t.guard(tb.Sheet); err != nil {
			refuse(err.Error())
		}
		if err := t.lockTable(tb.ID); err != nil {
			refuse(err.Error())
		}
		if tb.Filtered() {
			refuse(fmt.Sprintf("« %s » est filtré : retirez d'abord le filtre (des lignes masquées seraient regroupées sans qu'on les voie)", tb.Name))
		}
	}
	// Colonnes.
	if len(plan.Columns) == 0 {
		j.cols = matchColumns(tables)
	} else {
		for _, c := range plan.Columns {
			c.Name = strings.TrimSpace(c.Name)
			if c.Name == "" {
				return nil, fmt.Errorf("%w : colonne sans nom", ErrMergeSheets)
			}
			if len(c.From) != len(tables) {
				return nil, fmt.Errorf("%w : colonne « %s » : une source par tableau attendue (%d)", ErrMergeSheets, c.Name, len(tables))
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
					return nil, fmt.Errorf("%w : « %s » n'a pas de colonne « %s »", ErrMergeSheets, tables[ti].Name, n)
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
		return nil, fmt.Errorf("%w : aucune colonne à regrouper", ErrMergeSheets)
	}
	names := map[string]bool{}
	for _, c := range j.cols {
		low := strings.ToLower(c.Name)
		if names[low] {
			return nil, fmt.Errorf("%w : deux colonnes « %s »", ErrMergeSheets, c.Name)
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
		labels := make([]string, len(j.srcs))
		for i, s := range j.srcs {
			labels[i] = s.sheet
		}
		if len(dedupe(labels)) < len(labels) {
			for i, s := range j.srcs {
				labels[i] = s.tb.Name // deux tableaux d'une même feuille
			}
		}
		rec := series.Recognize(labels)
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
			return nil, fmt.Errorf("%w : catégorie de type « %s » : date, integer ou text attendu", ErrMergeSheets, cat.Type)
		}
		if len(cat.Values) == 0 {
			cat.Values = rec.Labels
			if cat.Type != rec.Type {
				cat.Values = labels
			}
		}
		if len(cat.Values) != len(j.srcs) {
			return nil, fmt.Errorf("%w : une valeur de catégorie par tableau attendue (%d)", ErrMergeSheets, len(j.srcs))
		}
		cat.Values = slices.Clone(cat.Values)
		for i, v := range cat.Values {
			v = strings.TrimSpace(v)
			cat.Values[i] = v
			val, err := grid.Parse(cat.Type, v)
			if err != nil || val == nil {
				return nil, fmt.Errorf("%w : la valeur « %s » de « %s » ne convient pas à une colonne %s", ErrMergeSheets, v, j.srcs[i].sheet, typeName(cat.Type))
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
			j.notes = append(j.notes, fmt.Sprintf("Une colonne s'appelle déjà « %s » : la catégorie s'appelle « %s »", base, cat.Name))
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
		name = "Regroupement"
	}
	if id := identifier(name); id != "" {
		name = id
	} else {
		name = "Regroupement"
	}
	if A1Like(name) {
		name += "_"
	}
	free, err := t.mergeTableName(name, seen)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(free, name) && strings.TrimSpace(plan.Name) != "" {
		j.notes = append(j.notes, fmt.Sprintf("Le nom « %s » est déjà pris : le tableau s'appelle « %s »", name, free))
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
	retarget := map[string]string{}
	for _, s := range j.srcs {
		retarget[strings.ToLower(s.tb.Name)] = j.plan.Name
	}
	j.formulas = make([]string, len(j.cols))
	j.valued = make([]bool, len(j.cols))
	for i, c := range j.cols {
		var fs []string
		some, all := false, true
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
				if x, ok := formula.RetargetTables(n, retarget); ok {
					n = x
				}
				f = formula.Format(n, formula.EN)
			}
			fs = append(fs, f)
		}
		switch {
		case !some:
		case all && len(slices.Compact(slices.Clone(fs))) == 1:
			j.formulas[i] = fs[0]
		default:
			j.valued[i] = true
			j.notes = append(j.notes, fmt.Sprintf("« %s » : la formule n'est pas la même dans tous les onglets, la colonne garde ses valeurs", c.Name))
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
				j.notes = append(j.notes, fmt.Sprintf("« %s » (%s) → « %s »", c.From[ti], j.srcs[ti].sheet, c.Name))
			}
		}
		if c.Text {
			j.notes = append(j.notes, fmt.Sprintf("« %s » passe en texte (types différents d'un onglet à l'autre)", c.Name))
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
			if d, err := time.Parse("2006-01-02", s); err == nil {
				return d.Format("02/01/2006")
			}
		}
	}
	if t == grid.Integer {
		return fmt.Sprint(v)
	}
	return raw
}

// mergeTableName rend le nom libre le plus proche (Ventes, Ventes_2…) parmi
// les tableaux, sauf les sources (supprimées ou renommées), et les TCD.
func (t *tx) mergeTableName(name string, sources map[int64]bool) (string, error) {
	ps, err := t.allPivots()
	if err != nil {
		return "", err
	}
	taken := map[string]bool{}
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
			return fmt.Sprintf("Pas de clé : « %s » n'a pas de colonne « %s »", s.sheet, j.cols[j.key].Name), nil
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
		return fmt.Sprintf("Pas de clé : %s est %d fois dans %s", val, n, j.srcs[a].sheet), nil
	}
	return fmt.Sprintf("Pas de clé : %s est dans %s et dans %s", val, j.srcs[a].sheet, j.srcs[b].sheet), nil
}

// deletableSheets : feuilles sources qui ne contiendront plus rien une fois
// leurs tableaux regroupés (ni autre tableau, ni TCD, graphique, bouton ou
// page) : elles partent avec eux. Leurs cellules libres (titres, totaux :
// des légendes, décision 0005) partent aussi.
func (t *tx) deletableSheets(j *mergeJob) (map[int64]bool, error) {
	src := map[int64]bool{}
	for _, s := range j.srcs {
		src[s.tb.ID] = true
	}
	out := map[int64]bool{}
	for _, s := range j.srcs {
		if _, done := out[s.tb.Sheet]; done {
			continue
		}
		ok := true
		rs, err := t.query(`SELECT id FROM _lattis_tables WHERE sheet = ?`, s.tb.Sheet)
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
			+ (SELECT count(*) FROM _lattis_buttons WHERE sheet = ?1) + (SELECT count(*) FROM _lattis_pages WHERE sheet = ?1)`, s.tb.Sheet).Scan(&objs); err != nil {
			return nil, err
		}
		out[s.tb.Sheet] = ok && objs == 0
	}
	return out, nil
}

// pendingRefs liste ce qui cite un tableau source hors des feuilles qui
// partent (gone) : à reprendre à la main au premier lot (§8 : lot 2).
func (t *tx) pendingRefs(j *mergeJob, gone map[int64]bool) ([]PendingRef, error) {
	out := []PendingRef{}
	src := map[string]bool{}
	srcID := map[int64]bool{}
	for _, s := range j.srcs {
		src[strings.ToLower(s.tb.Name)] = true
		srcID[s.tb.ID] = true
	}
	sheetNames := map[int64]string{}
	rs, err := t.query(`SELECT id, name FROM _lattis_sheets`)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var id int64
		var n string
		if err := rs.Scan(&id, &n); err != nil {
			rs.Close()
			return nil, err
		}
		sheetNames[id] = n
	}
	rs.Close()
	cites := func(n formula.Node, own string) bool {
		for _, r := range refsOf(n, own) {
			if src[strings.ToLower(r.table)] {
				return true
			}
		}
		return false
	}
	// Formules des cellules libres.
	rs, err = t.query(`SELECT sheet, row, col, raw FROM _lattis_cells WHERE raw LIKE '=%' ORDER BY sheet, row, col`)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var p PendingRef
		if err := rs.Scan(&p.Sheet, &p.Row, &p.Col, &p.Formula); err != nil {
			rs.Close()
			return nil, err
		}
		if gone[p.Sheet] {
			continue
		}
		if n, err := formula.Parse(p.Formula, formula.FR); err == nil && cites(n, "") {
			p.Kind, p.SheetName = "cell", sheetNames[p.Sheet]
			p.Where = p.SheetName + " " + formula.ColumnLetters(p.Col) + fmt.Sprint(p.Row+1)
			out = append(out, p)
		}
	}
	rs.Close()
	// Colonnes calculées des autres tableaux.
	rs, err = t.query(`SELECT t.id, t.sheet, t.name, c.name, c.formula FROM _lattis_columns c JOIN _lattis_tables t ON t.id = c.tbl
		WHERE c.formula IS NOT NULL AND c.formula <> '' ORDER BY t.id, c.pos`)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var id int64
		var p PendingRef
		var table, col, f string
		if err := rs.Scan(&id, &p.Sheet, &table, &col, &f); err != nil {
			rs.Close()
			return nil, err
		}
		if srcID[id] || gone[p.Sheet] {
			continue
		}
		if n, err := formula.Parse(f, formula.EN); err == nil && cites(n, table) {
			p.Kind, p.SheetName = "column", sheetNames[p.Sheet]
			p.Where = table + "[" + col + "]"
			p.Formula = "=" + strings.TrimPrefix(formula.Format(n, formula.FR), "=")
			out = append(out, p)
		}
	}
	rs.Close()
	// TCD et graphiques.
	for _, o := range []struct{ table, kind, label string }{{`_lattis_pivots`, "pivot", "Tableau croisé"}, {`_lattis_charts`, "chart", "Graphique"}} {
		rs, err := t.query(`SELECT sheet, row, col, config FROM ` + o.table + ` ORDER BY id`)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var p PendingRef
			var cfg string
			if err := rs.Scan(&p.Sheet, &p.Row, &p.Col, &cfg); err != nil {
				rs.Close()
				return nil, err
			}
			var m map[string]any
			if gone[p.Sheet] || json.Unmarshal([]byte(cfg), &m) != nil {
				continue
			}
			if source, _ := m["source"].(string); src[strings.ToLower(source)] {
				p.Kind, p.SheetName = o.kind, sheetNames[p.Sheet]
				p.Where = fmt.Sprintf("%s %s%d", p.SheetName, formula.ColumnLetters(p.Col), p.Row+1)
				p.Formula = o.label + " sur « " + source + " »"
				out = append(out, p)
			}
		}
		rs.Close()
	}
	// Requêtes d'import qui lisent un tableau source.
	rs, err = t.query(`SELECT name, source FROM _lattis_queries ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var name, source string
		if err := rs.Scan(&name, &source); err != nil {
			rs.Close()
			return nil, err
		}
		var m map[string]any
		if json.Unmarshal([]byte(source), &m) != nil {
			continue
		}
		if tbl, _ := m["table"].(string); src[strings.ToLower(tbl)] {
			out = append(out, PendingRef{Kind: "query", Where: "Requête « " + name + " »", Formula: "lit « " + tbl + " »"})
		}
	}
	rs.Close()
	return out, rs.Err()
}

// MergePreview prépare l'aperçu d'un regroupement (lecture pure) : plan
// complété, 20 premières lignes, clé, formules à reprendre.
func (m *Model) MergePreview(ctx context.Context, plan MergePlan) (MergePreview, error) {
	var p MergePreview
	err := m.read(ctx, func(t *tx) error {
		j, err := t.prepareMerge(plan)
		if err != nil {
			return err
		}
		p = MergePreview{Plan: j.plan, Rows: [][]any{}, Total: j.total, Notes: append([]string{}, j.notes...), Refused: j.refused}
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
		if j.key >= 0 {
			p.Key = j.cols[j.key].Name
			if p.KeyNote, err = t.mergeKeyCheck(j); err != nil {
				return err
			}
			p.KeyKept = p.KeyNote == ""
		}
		gone, err := t.deletableSheets(j)
		if err != nil {
			return err
		}
		if p.Pending, err = t.pendingRefs(j, gone); err != nil {
			return err
		}
		p.Old = j.plan.Old
		if p.Old == "" {
			p.Old = MergeOldDelete
			if len(p.Pending) > 0 {
				p.Old = MergeOldHide
			}
		}
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

// MergeSheets regroupe les tableaux du plan en un seul, sur une nouvelle
// feuille, en une seule étape d'annulation (§7).
func (m *Model) MergeSheets(ctx context.Context, plan MergePlan) (MergeResult, error) {
	var res MergeResult
	err := m.Do(ctx, "Regrouper des onglets", func(t *tx) error {
		var err error
		res, err = t.mergeSheets(plan)
		return err
	})
	return res, err
}

func (t *tx) mergeSheets(plan MergePlan) (MergeResult, error) {
	res := MergeResult{Pending: []PendingRef{}, Notes: []string{}}
	// 1. Garde-fous.
	j, err := t.prepareMerge(plan)
	if err != nil {
		return res, err
	}
	if j.refused != "" {
		return res, fmt.Errorf("%w : %s", ErrMergeSheets, j.refused)
	}
	keyNote, err := t.mergeKeyCheck(j)
	if err != nil {
		return res, err
	}
	gone, err := t.deletableSheets(j)
	if err != nil {
		return res, err
	}
	pending, err := t.pendingRefs(j, gone)
	if err != nil {
		return res, err
	}
	res.Pending = pending
	res.Old = j.plan.Old
	if res.Old == "" {
		res.Old = MergeOldDelete
		if len(pending) > 0 {
			res.Old = MergeOldHide
		}
	}
	tabs, err := t.sheetOrder()
	if err != nil {
		return res, err
	}
	at := len(tabs)
	for i, id := range tabs {
		for _, s := range j.srcs {
			if s.tb.Sheet == id && i < at {
				at = i
			}
		}
	}
	if err := t.purgeImports(); err != nil {
		return res, err
	}
	var id int64
	if err := t.row(nextTableID).Scan(&id); err != nil {
		return res, err
	}
	tb := Table{ID: id, Name: j.plan.Name}
	// Colonnes du résultat : la catégorie (1) puis les colonnes (2…).
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
	// 2. Table de chargement, hors journal (comme Import) : une INSERT …
	// SELECT par source, dans l'ordre des onglets puis des lignes.
	staging := fmt.Sprintf("%smerge%d_%d", importPrefix, id, time.Now().UnixNano())
	if _, err := t.ExecContext(t.Ctx, `CREATE TABLE `+staging+` (`+strings.Join(defs, ", ")+`) STRICT`); err != nil {
		return res, err
	}
	for _, s := range j.srcs {
		if err := t.Ctx.Err(); err != nil {
			return res, err
		}
		sel, selArgs, err := t.sourceSelect(s, true)
		if err != nil {
			return res, err
		}
		outs := []string{fmt.Sprintf("q.id + %d", s.off), order.SpreadSQL(j.total, fmt.Sprintf("(q.rn + %d)", s.base))}
		// La catégorie en littéral : les colonnes calculées numérotent leurs
		// paramètres (?1, ?2…).
		if j.hasCat() {
			outs = append(outs, catLiteral(s.cat))
		}
		args := selArgs
		for i := range j.cols {
			outs = append(outs, j.valueExpr(s, i))
		}
		q := `INSERT INTO ` + staging + ` (` + strings.Join(names, ", ") + `) SELECT ` + strings.Join(outs, ", ") + ` FROM (` + sel + `) AS q`
		if _, err := t.ExecContext(t.Ctx, q, args...); err != nil {
			return res, fmt.Errorf("regroupement de « %s » : %w", s.tb.Name, err)
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
				if _, err := t.ExecContext(t.Ctx, `UPDATE `+staging+` SET `+colSQL(colID(i))+` = i.raw FROM _lattis_invalid AS i
					WHERE i.tbl = ? AND i.col = ? AND `+staging+`.id = i.rid + ?`, s.tb.ID, sc.ID, s.off); err != nil {
					return res, err
				}
				continue
			}
			do = append(do, st(`INSERT INTO _lattis_invalid (tbl, rid, col, raw) SELECT ?, rid + ?, ?, raw FROM _lattis_invalid WHERE tbl = ? AND col = ?`, id, s.off, colID(i), s.tb.ID, sc.ID))
		}
	}
	// 3. Mise en forme : celle des cellules suit ses lignes (rid décalé) ;
	// celle des colonnes et des en-têtes vient du premier tableau qui a la
	// colonne, comme ses règles de mise en forme conditionnelle.
	type colMeta struct {
		fixed bool
		rules sql.NullString
		width sql.NullInt64
	}
	metas := make([]colMeta, len(j.cols))
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
			if err := t.row(`SELECT fixed <> 0, rules FROM _lattis_columns WHERE tbl = ? AND id = ?`, s.tb.ID, sc.ID).Scan(&metas[i].fixed, &metas[i].rules); err != nil {
				return res, err
			}
			err := t.row(`SELECT width FROM _lattis_col_widths WHERE sheet = ? AND col = ?`, s.tb.Sheet, s.tb.Col+k).Scan(&metas[i].width)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return res, err
			}
		}
	}
	// 8 (début). Liste de validation de la catégorie (style dv de la
	// colonne), et son format de date.
	if j.hasCat() {
		style := Style{"dv": dedupe(j.catLabel)}
		if f := j.plan.Category.Format; f != "" {
			style["df"] = f
		}
		enc, err := encodeStyle(style)
		if err != nil {
			return res, err
		}
		do = append(do, st(`INSERT INTO _lattis_styles (tbl, rid, col, style) VALUES (?, 0, 1, ?)`, id, enc))
	}
	if len(do) > 0 {
		if err := t.apply(do, []stmt{st(`DELETE FROM _lattis_styles WHERE tbl = ?`, id), st(`DELETE FROM _lattis_invalid WHERE tbl = ?`, id)}); err != nil {
			return res, err
		}
	}
	// 7. Anciens onglets : supprimés (corbeille de l'annulation), ou masqués
	// avec leurs tableaux renommés Janvier_ancien (ce qui les cite suit).
	switch res.Old {
	case MergeOldHide:
		hidden := map[int64]bool{}
		for _, s := range j.srcs {
			old, err := t.mergeTableName(s.tb.Name+"_ancien", nil)
			if err != nil {
				return res, err
			}
			cur, err := t.table(s.tb.ID)
			if err != nil {
				return res, err
			}
			if err := t.renameTable(cur, old); err != nil {
				return res, err
			}
			if hidden[s.tb.Sheet] {
				continue
			}
			hidden[s.tb.Sheet] = true
			var was bool
			if err := t.row(`SELECT hidden <> 0 FROM _lattis_sheets WHERE id = ?`, s.tb.Sheet).Scan(&was); err != nil {
				return res, err
			}
			if !was {
				q := `UPDATE _lattis_sheets SET hidden = ? WHERE id = ?`
				if err := t.apply([]stmt{st(q, 1, s.tb.Sheet)}, []stmt{st(q, 0, s.tb.Sheet)}); err != nil {
					return res, err
				}
			}
		}
	default:
		for _, s := range j.srcs {
			if err := t.deleteTable(s.tb); err != nil {
				return res, err
			}
		}
		done := map[int64]bool{}
		for _, s := range j.srcs {
			if done[s.tb.Sheet] {
				continue
			}
			done[s.tb.Sheet] = true
			if gone[s.tb.Sheet] {
				if err := t.deleteSheet(s.tb.Sheet); err != nil {
					return res, err
				}
			} else {
				res.Notes = append(res.Notes, fmt.Sprintf("La feuille « %s » contient autre chose : gardée", s.sheet))
			}
		}
		if len(pending) > 0 {
			res.Notes = append(res.Notes, fmt.Sprintf("%s citai%s les anciens tableaux : à reprendre", plural(len(pending), "Une formule ou un objet", fmt.Sprintf("%d formules ou objets", len(pending))), plural(len(pending), "t", "ent")))
		}
	}
	// Nouvelle feuille, placée où était le premier onglet regroupé.
	sheet, err := t.newSheet(j.plan.Sheet)
	if err != nil {
		return res, err
	}
	tb.Sheet = sheet
	var wdo []stmt
	shift := 0
	if j.hasCat() {
		shift = 1
	}
	for i, mt := range metas {
		if mt.width.Valid {
			wdo = append(wdo, st(`INSERT INTO _lattis_col_widths (sheet, col, width) VALUES (?, ?, ?)`, sheet, i+shift, mt.width.Int64))
		}
	}
	if len(wdo) > 0 {
		if err := t.apply(wdo, []stmt{st(`DELETE FROM _lattis_col_widths WHERE sheet = ?`, sheet)}); err != nil {
			return res, err
		}
	}
	// 3 (fin). Le tableau : la table chargée prend son nom, ses colonnes et
	// sa place en A1.
	do = []stmt{
		st(`ALTER TABLE ` + staging + ` RENAME TO ` + tb.sql()),
		st(`CREATE INDEX ` + tb.sql() + `_pos ON ` + tb.sql() + `(_pos)`),
		st(`INSERT INTO _lattis_tables (id, sheet, name, row, col) VALUES (?, ?, ?, 0, 0)`, id, sheet, tb.Name),
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
			id, colID(i), c.Name, string(c.Type), metas[i].fixed, pos, f, nullable(metas[i].rules)))
		pos++
	}
	if err := t.apply(do, []stmt{
		st(`DELETE FROM _lattis_columns WHERE tbl = ?`, id),
		st(`DELETE FROM _lattis_tables WHERE id = ?`, id),
		st(`DROP INDEX ` + tb.sql() + `_pos`),
		st(`ALTER TABLE ` + tb.sql() + ` RENAME TO ` + staging),
	}); err != nil {
		return res, err
	}
	// 5. Clé : gardée sans collision (index unique recréé) ; sinon seule la
	// notion de clé part, sans rien demander (règle de Louis).
	if j.key >= 0 {
		if keyNote == "" {
			if err := t.ensureKey(tb.Name, j.cols[j.key].Name); err != nil {
				var ke ErrKey
				if !errors.As(err, &ke) {
					return res, err
				}
				keyNote = "Pas de clé : " + ke.Why
			}
		}
		res.KeyKept, res.KeyNote = keyNote == "", keyNote
		if keyNote != "" {
			res.Notes = append(res.Notes, keyNote)
		}
	}
	if err := t.moveSheet(sheet, at); err != nil {
		return res, err
	}
	// 8. Onglet à choix réglé sur la valeur la plus récente (lot 3).
	if j.hasCat() {
		vals := make([]string, len(j.srcs))
		for i, s := range j.srcs {
			vals[i] = fmt.Sprint(s.cat)
		}
		latest := series.Latest(series.Series{Type: j.plan.Category.Type, Values: vals})
		if err := t.setSheetChoice(sheet, tb.Name, j.plan.Category.Name, fmt.Sprint(j.srcs[latest].cat)); err != nil {
			return res, err
		}
	}
	if err := t.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&res.SheetName); err != nil {
		return res, err
	}
	res.Sheet, res.Table, res.Rows = sheet, tb.Name, j.total
	res.Notes = append(j.notes, res.Notes...)
	what := "supprimés"
	if res.Old == MergeOldHide {
		what = "masqués"
	}
	res.Notice = fmt.Sprintf("%d onglets regroupés dans « %s » (%s), anciens onglets %s", len(j.srcs), tb.Name, plural(j.total, fmt.Sprintf("%d ligne", j.total), fmt.Sprintf("%d lignes", j.total)), what)
	if keyNote != "" {
		res.Notice += " ; " + keyNote
	}
	if len(pending) > 0 {
		res.Notice += fmt.Sprintf(" ; %s à reprendre", plural(len(pending), "une formule", fmt.Sprintf("%d formules", len(pending))))
	}
	t.notes = append(t.notes, res.Notice)
	return res, nil
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
