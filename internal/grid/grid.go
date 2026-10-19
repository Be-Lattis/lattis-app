// Package grid sert les lignes d'un tableau à la grille, par fenêtres.
//
// La table n'a pas d'ordre propre : chaque ligne porte une clé d'ordre _pos
// (voir internal/order), indexée. Lire la ligne n par LIMIT/OFFSET obligerait
// SQLite à parcourir n lignes ; on relève donc une clé « ancre » toutes les
// AnchorEvery lignes et on part de l'ancre la plus proche, ce qui borne
// l'OFFSET à AnchorEvery-1 quelle que soit la position dans la table.
//
// Prototype de la phase 1.1 : un seul tableau de démonstration, sans
// insertion ni suppression de lignes (les ancres restent donc valables).
package grid

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lalamefine/lattis/internal/store"
)

// AnchorEvery est l'écart, en lignes, entre deux ancres.
const AnchorEvery = 1000

// MaxWindow borne le nombre de lignes d'une fenêtre.
const MaxWindow = 2000

// Column décrit une colonne affichée.
type Column struct {
	Name string `json:"name"`
	Type Type   `json:"type"`
}

// Info décrit le tableau.
type Info struct {
	Name       string   `json:"name"`
	SQLName    string   `json:"sqlName"` // nom de la table SQL (onglet SQL)
	SQLColumns []string `json:"sqlColumns"`
	Rows       int      `json:"rows"`
	Columns    []Column `json:"columns"`
}

// Window est une fenêtre de lignes consécutives, à partir de la ligne First
// (0 = première ligne dans l'ordre d'affichage). Cells est indexé
// [ligne][colonne] ; IDs donne la clé technique de chaque ligne.
type Window struct {
	First   int           `json:"first"`
	IDs     []int64       `json:"ids"`
	Cells   [][]any       `json:"cells"`
	Invalid []InvalidCell `json:"invalid"` // saisies hors type de la fenêtre
	Styles  []InvalidCell `json:"styles"`  // mise en forme des cellules (Raw : style JSON)
	// Règles « formule » de mise en forme conditionnelle vérifiées par les
	// cellules de la fenêtre.
	Marks []Mark `json:"marks"`
	// Sync : cellules d'un tableau partagé dont la modification attend le
	// serveur, ou en conflit (classeur collaboratif ; rempli par le pont).
	Sync []SyncMark `json:"sync,omitempty"`
	// Locked : lignes de la fenêtre en lecture seule pour un participant
	// (lignes d'autres membres, §8.1 ; rempli par le pont).
	Locked []int `json:"locked,omitempty"`
	Micros int64 `json:"micros"` // temps passé côté Go, pour les mesures
}

// SyncMark marque une cellule d'un classeur collaboratif : Kind « pending »
// (en attente de confirmation du serveur) ou « conflict ». Col -1 : toute
// la ligne.
type SyncMark struct {
	Row  int    `json:"row"`
	Col  int    `json:"col"`
	Kind string `json:"kind"`
}

// Mark : la règle Rule (rang dans la liste de la colonne) s'applique à la
// cellule (ligne Row de la fenêtre, colonne affichée Col).
type Mark struct {
	Row  int `json:"row"`
	Col  int `json:"col"`
	Rule int `json:"rule"`
}

// InvalidCell est une saisie hors type : la cellule est vide, la valeur
// brute est gardée pour l'affichage et la correction.
type InvalidCell struct {
	Row int    `json:"row"` // indice dans la fenêtre
	Col int    `json:"col"` // colonne affichée
	Raw string `json:"raw"`
}

// Table donne accès à un tableau stocké dans une table SQLite.
type Table struct {
	db      *store.Store
	sqlName string
	name    string
	cols    []Column
	sqlCols []string // noms SQL des colonnes, dans l'ordre d'affichage

	mu      sync.RWMutex
	anchors []string
	rows    int

	// deps : tables t_N que lit la source (la table, et celles d'un filtre
	// qui en lit d'autres) ; filtered : la source est une requête filtrée.
	// Versions relevées au chargement (voir Fresh).
	deps                        []string
	filtered                    bool
	rowsVer, valuesVer, dataVer int64

	selectWindow string
	exprs        []string           // expression SQL de chaque colonne affichée
	stats        map[statsKey]Stats // résumés déjà calculés (Stats)
	// computed : le tableau a des colonnes calculées, qui peuvent lire
	// d'autres tableaux ou des cellules : toute modification l'invalide.
	computed bool
	args     []any                    // paramètres des expressions de colonnes (?1…)
	decode   func(col int, v any) any // conversion des valeurs calculées

	// Saisies hors type (tableaux du classeur) : identifiant du tableau et
	// identifiant de chaque colonne affichée.
	tableID int64
	colIDs  map[int64]int

	// Règles « formule » : expression SQL de chacune (après les colonnes
	// dans la lecture des fenêtres), sa colonne affichée et son rang.
	marks     []string
	markCols  []int
	markRules []int
	truthy    func(v any) bool
	cf        map[string]RuleStats // statistiques des règles déjà calculées
}

// WithMarks ajoute à la lecture des fenêtres les règles « formule » de mise
// en forme conditionnelle (expressions SQL et leurs paramètres, numérotés à
// la suite de ceux des colonnes) ; truthy dit si un résultat est vrai.
func (t *Table) WithMarks(exprs []string, args []any, cols, rules []int, truthy func(v any) bool) *Table {
	if len(exprs) == 0 {
		return t
	}
	t.marks, t.markCols, t.markRules, t.truthy = exprs, cols, rules, truthy
	t.args = append(t.args, args...)
	t.selectWindow = fmt.Sprintf(`SELECT id, %s, %s FROM %s WHERE _pos >= ? ORDER BY _pos LIMIT ? OFFSET ?`,
		strings.Join(t.exprs, ", "), strings.Join(exprs, ", "), t.sqlName)
	return t
}

// WithExprs remplace les colonnes lues par des expressions SQL (colonnes
// calculées), avec leurs paramètres numérotés ?1…?n, et une conversion des
// valeurs lues pour l'interface.
func (t *Table) WithExprs(exprs []string, args []any, decode func(col int, v any) any) *Table {
	t.selectWindow = fmt.Sprintf(`SELECT id, %s FROM %s WHERE _pos >= ? ORDER BY _pos LIMIT ? OFFSET ?`,
		strings.Join(exprs, ", "), t.sqlName)
	t.args, t.decode, t.exprs = args, decode, exprs
	return t
}

// WithComputed marque un tableau à colonnes calculées (voir Reusable).
func (t *Table) WithComputed(on bool) *Table {
	t.computed = on
	return t
}

// Fresh dit si le tableau chargé lit encore ce que contient la base (voir
// store.GridVersion) : faux s'il faut le recharger, parce que ses lignes,
// leur ordre ou sa définition (colonnes, filtre, choix de l'onglet) ont pu
// changer, ou, pour un tableau filtré, ses valeurs (une saisie fait entrer
// ou sortir une ligne du filtre), ou, pour un tableau à colonnes calculées
// qui lisent ailleurs, une donnée du classeur (agrégats calculés au
// chargement). Sinon ses totaux en cache sont oubliés quand ses valeurs ont
// pu changer, et le reste sert encore.
//
// Les versions viennent du Store, relevées avant le chargement : rien ne
// dépend de l'appel d'une fonction après chaque écriture, ni du texte de la
// source (une source filtrée est une sous-requête, pas le nom t_N).
func (t *Table) Fresh() bool {
	rows, values, data := t.db.GridVersion(t.deps)
	t.mu.Lock()
	defer t.mu.Unlock()
	if rows != t.rowsVer || (t.filtered && values != t.valuesVer) || (t.computed && data != t.dataVer) {
		return false
	}
	if values != t.valuesVer || data != t.dataVer {
		t.stats, t.cf = nil, nil
		t.valuesVer, t.dataVer = values, data
	}
	return true
}

// WithInvalid fait lire les saisies hors type de _lattis_invalid avec les
// fenêtres.
func (t *Table) WithInvalid(tableID int64, colIDs []int64) *Table {
	t.tableID = tableID
	t.colIDs = map[int64]int{}
	for i, id := range colIDs {
		t.colIDs[id] = i
	}
	return t
}

// ColIndex donne la colonne affichée d'une colonne du classeur (voir
// WithInvalid).
func (t *Table) ColIndex(id int64) (int, bool) {
	i, ok := t.colIDs[id]
	return i, ok
}

// ErrRange signale une fenêtre ou une colonne hors du tableau.
var ErrRange = errors.New("grid: hors du tableau")

// dataTable : nom SQL d'un tableau du classeur (t_N), dans une source.
var dataTable = regexp.MustCompile(`\bt_\d+\b`)

// NewTable relie un tableau existant ; sqlCols donne le nom SQL de chaque
// colonne (nil : c_0, c_1…). Load doit être appelé avant les lectures.
func NewTable(db *store.Store, sqlName, name string, cols []Column, sqlCols []string) *Table {
	names := sqlCols
	if names == nil {
		names = make([]string, len(cols))
		for i := range cols {
			names[i] = colName(i)
		}
	}
	var deps []string
	for _, n := range dataTable.FindAllString(sqlName, -1) {
		if !slices.Contains(deps, n) {
			deps = append(deps, n)
		}
	}
	if len(deps) == 0 {
		deps = []string{sqlName}
	}
	return &Table{
		db:       db,
		sqlName:  sqlName,
		name:     name,
		cols:     cols,
		sqlCols:  names,
		exprs:    names,
		deps:     deps,
		filtered: strings.ContainsAny(sqlName, " ("),
		selectWindow: fmt.Sprintf(`SELECT id, %s FROM %s WHERE _pos >= ? ORDER BY _pos LIMIT ? OFFSET ?`,
			strings.Join(names, ", "), sqlName),
	}
}

// colName donne l'identifiant SQL interne de la colonne i. Le nom affiché est
// dans les métadonnées : un renommage ne touche jamais au schéma.
func colName(i int) string { return fmt.Sprintf("c_%d", i) }

// Load relève les ancres et le nombre de lignes.
func (t *Table) Load(ctx context.Context) error {
	// Versions relevées avant la lecture : une écriture validée pendant
	// qu'on lit les fait avancer, le tableau sera relu (Fresh).
	rowsVer, valuesVer, dataVer := t.db.GridVersion(t.deps)
	rs, err := t.db.Read().QueryContext(ctx, `SELECT _pos FROM `+t.sqlName+` ORDER BY _pos`)
	if err != nil {
		return err
	}
	defer rs.Close()
	var anchors []string
	n := 0
	for rs.Next() {
		if n%AnchorEvery == 0 {
			var p string
			if err := rs.Scan(&p); err != nil {
				return err
			}
			anchors = append(anchors, p)
		}
		n++
	}
	if err := rs.Err(); err != nil {
		return err
	}
	t.mu.Lock()
	t.anchors, t.rows = anchors, n
	t.rowsVer, t.valuesVer, t.dataVer = rowsVer, valuesVer, dataVer
	t.stats, t.cf = nil, nil
	t.mu.Unlock()
	return nil
}

// Info décrit le tableau.
func (t *Table) Info() Info {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return Info{Name: t.name, SQLName: t.sqlName, SQLColumns: t.sqlCols, Rows: t.rows, Columns: t.cols}
}

// Window lit count lignes à partir de la ligne first.
func (t *Table) Window(ctx context.Context, first, count int) (Window, error) {
	start := time.Now()
	t.mu.RLock()
	rows := t.rows
	var anchor string
	if first >= 0 && first < rows {
		anchor = t.anchors[first/AnchorEvery]
	}
	t.mu.RUnlock()
	if first < 0 || first >= rows || count <= 0 {
		return Window{}, ErrRange
	}
	count = min(count, MaxWindow, rows-first)

	args := append(append([]any{}, t.args...), anchor, count, first%AnchorEvery)
	rs, err := t.db.Read().QueryContext(ctx, t.selectWindow, args...)
	if err != nil {
		return Window{}, err
	}
	defer rs.Close()

	w := Window{First: first, IDs: make([]int64, 0, count), Cells: make([][]any, 0, count)}
	dest := make([]any, len(t.cols)+1+len(t.marks))
	ptrs := make([]any, len(dest))
	for i := range dest {
		ptrs[i] = &dest[i]
	}
	for rs.Next() {
		if err := rs.Scan(ptrs...); err != nil {
			return Window{}, err
		}
		id, _ := dest[0].(int64)
		for k := range t.marks {
			if t.truthy(dest[len(t.cols)+1+k]) {
				w.Marks = append(w.Marks, Mark{Row: len(w.IDs), Col: t.markCols[k], Rule: t.markRules[k]})
			}
		}
		row := make([]any, len(t.cols))
		for c := range row {
			if t.decode != nil {
				row[c] = t.decode(c, dest[c+1])
				continue
			}
			row[c] = jsonValue(dest[c+1])
		}
		w.IDs = append(w.IDs, id)
		w.Cells = append(w.Cells, row)
	}
	if err := rs.Err(); err != nil {
		return Window{}, err
	}
	if t.colIDs != nil && len(w.IDs) > 0 {
		if err := t.invalid(ctx, &w); err != nil {
			return Window{}, err
		}
	}
	w.Micros = time.Since(start).Microseconds()
	return w, nil
}

// jsonValue évite qu'un texte lu en []byte soit encodé en base64.
func jsonValue(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

// SetCell remplace la valeur d'une cellule à partir d'une saisie et renvoie
// la valeur stockée.
func (t *Table) SetCell(ctx context.Context, id int64, col int, raw string) (any, error) {
	if col < 0 || col >= len(t.cols) {
		return nil, ErrRange
	}
	v, err := Parse(t.cols[col].Type, raw)
	if err != nil {
		return nil, err
	}
	err = t.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE `+t.sqlName+` SET `+t.sqlCols[col]+` = ? WHERE id = ?`, v, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return ErrRange
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (t *Table) invalid(ctx context.Context, w *Window) error {
	index := make(map[int64]int, len(w.IDs))
	lo, hi := w.IDs[0], w.IDs[0]
	for i, id := range w.IDs {
		index[id] = i
		lo, hi = min(lo, id), max(hi, id)
	}
	if err := t.cellsFrom(ctx, `SELECT rid, col, style FROM _lattis_styles WHERE tbl = ? AND rid BETWEEN ? AND ?`, lo, hi, index, &w.Styles); err != nil {
		return err
	}
	return t.cellsFrom(ctx, `SELECT rid, col, raw FROM _lattis_invalid WHERE tbl = ? AND rid BETWEEN ? AND ?`, lo, hi, index, &w.Invalid)
}

// cellsFrom lit des valeurs par cellule (rid, col, texte) pour la fenêtre.
func (t *Table) cellsFrom(ctx context.Context, q string, lo, hi int64, index map[int64]int, out *[]InvalidCell) error {
	rs, err := t.db.Read().QueryContext(ctx, q, t.tableID, max(lo, 1), hi)
	if err != nil {
		return err
	}
	defer rs.Close()
	for rs.Next() {
		var rid, col int64
		var raw string
		if err := rs.Scan(&rid, &col, &raw); err != nil {
			return err
		}
		r, ok1 := index[rid]
		c, ok2 := t.colIDs[col]
		if ok1 && ok2 {
			*out = append(*out, InvalidCell{r, c, raw})
		}
	}
	return rs.Err()
}

// Stats résume des cellules d'un tableau pour la barre d'état : valeurs non
// vides, nombres et leur somme. Lignes first à first+count-1 (ordre
// d'affichage), colonnes cols (indices d'affichage). Le calcul se fait en
// SQL, sans lire les lignes : une colonne d'un million de lignes se résume
// en une requête.
type Stats struct {
	Count int     `json:"count"`
	Nums  int     `json:"nums"`
	Sum   float64 `json:"sum"`
}

// Stats résume les lignes first à first+count-1 et les colonnes cols.

func (t *Table) Stats(ctx context.Context, first, count int, cols []int) (Stats, error) {
	t.mu.RLock()
	rows := t.rows
	var anchor string
	if first >= 0 && first < rows {
		anchor = t.anchors[first/AnchorEvery]
	}
	t.mu.RUnlock()
	if first < 0 || first >= rows || count <= 0 || len(cols) == 0 {
		return Stats{}, nil
	}
	count = min(count, rows-first)
	// Cache par colonne : la Table est jetée à chaque modification du
	// classeur, ses résultats restent donc justes ; resélectionner une
	// colonne, ou en ajouter une à la sélection, ne recalcule que le neuf.
	var out Stats
	var todo []int
	t.mu.RLock()
	for _, c := range cols {
		if c < 0 || c >= len(t.exprs) {
			t.mu.RUnlock()
			return Stats{}, ErrRange
		}
		if st, ok := t.stats[statsKey{first, count, c}]; ok {
			out.Count, out.Nums, out.Sum = out.Count+st.Count, out.Nums+st.Nums, out.Sum+st.Sum
		} else {
			todo = append(todo, c)
		}
	}
	t.mu.RUnlock()
	if len(todo) == 0 {
		return out, nil
	}
	// Une seule requête pour les colonnes à calculer : trois agrégats chacune.
	var inner, aggs []string
	for k, c := range todo {
		x := fmt.Sprintf("x%d", k)
		inner = append(inner, t.exprs[c]+" AS "+x)
		if t.cols[c].Type == Boolean { // VRAI / FAUX : comptés, pas additionnés (comme Excel)
			aggs = append(aggs, "count("+x+"), 0, 0")
			continue
		}
		isNum := "typeof(" + x + ") IN ('integer', 'real')"
		aggs = append(aggs, "count("+x+"), coalesce(sum("+isNum+"), 0), total(CASE WHEN "+isNum+" THEN "+x+" END)")
	}
	// Paramètres numérotés : ceux des colonnes calculées (?1…?n, même si
	// les colonnes résumées n'en utilisent aucun), puis la fenêtre.
	n := len(t.args)
	q := "SELECT " + strings.Join(aggs, ", ") +
		fmt.Sprintf(" FROM (SELECT %s FROM %s WHERE _pos >= ?%d ORDER BY _pos LIMIT ?%d OFFSET ?%d)", strings.Join(inner, ", "), t.sqlName, n+1, n+2, n+3)
	args := append(append([]any{}, t.args...), anchor, count, first%AnchorEvery)
	if first == 0 && count == rows {
		// Toutes les lignes (colonne entière, Ctrl+A) : pas besoin de les
		// ordonner pour les additionner.
		// ?n cité une fois : le pilote attend autant de paramètres que le
		// plus grand numéro, même quand les colonnes résumées n'en lisent pas.
		where := ""
		if n > 0 {
			where = fmt.Sprintf(" WHERE ?%d IS NULL OR 1", n)
		}
		q = "SELECT " + strings.Join(aggs, ", ") + fmt.Sprintf(" FROM (SELECT %s FROM %s%s)", strings.Join(inner, ", "), t.sqlName, where)
		args = append([]any{}, t.args...)
	}
	got := make([]Stats, len(todo))
	dest := make([]any, 0, 3*len(todo))
	for k := range got {
		dest = append(dest, &got[k].Count, &got[k].Nums, &got[k].Sum)
	}
	if err := t.db.Read().QueryRowContext(ctx, q, args...).Scan(dest...); err != nil {
		return Stats{}, err
	}
	t.mu.Lock()
	if t.stats == nil {
		t.stats = map[statsKey]Stats{}
	}
	for k, c := range todo {
		t.stats[statsKey{first, count, c}] = got[k]
		out.Count, out.Nums, out.Sum = out.Count+got[k].Count, out.Nums+got[k].Nums, out.Sum+got[k].Sum
	}
	t.mu.Unlock()
	return out, nil
}

// statsKey : plage de lignes et colonne d'un résumé gardé en cache.
type statsKey struct{ first, count, col int }
