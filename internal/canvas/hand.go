package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
)

// Colonnes saisies à côté d'un résultat chargé (docs/decisions/0008).
//
// Un tableau chargé par une requête (Power Query, ou Référencer de l'onglet
// SQL) est remplacé à chaque actualisation. Un nom tapé à droite de son
// en-tête y ajoute une colonne saisie : ses cellules se modifient, et ses
// valeurs suivent les lignes du résultat par une clé de jointure (une
// colonne chargée qui identifie les lignes, sinon leur rang). Une ligne qui
// disparaît à l'actualisation laisse ses valeurs de côté
// (_lattis_hand_kept) ; elles reviennent si sa clé revient.
//
// Le propriétaire (owner) désigne le résultat : « query:<id> ». Les tableaux
// croisés (« pivot:<id> ») ont leur propre code : pivothand.go.

// handRow : clé de jointure « rang de la ligne ».
const handRow = "#"

func queryOwner(id int64) string { return "query:" + strconv.FormatInt(id, 10) }

// handInfo est l'état des colonnes saisies d'un résultat.
type handInfo struct {
	owner string
	key   string // nom d'une colonne chargée, ou handRow
	keyed bool   // clé choisie
	cols  map[int64]int64
}

func (t *tx) hand(owner string) (handInfo, error) {
	hi := handInfo{owner: owner, cols: map[int64]int64{}}
	switch err := t.row(`SELECT key FROM _lattis_hand WHERE owner = ?`, owner).Scan(&hi.key); {
	case err == nil:
		hi.keyed = true
	case !errors.Is(err, sql.ErrNoRows):
		return hi, err
	}
	rs, err := t.query(`SELECT h, col FROM _lattis_hand_cols WHERE owner = ?`, owner)
	if err != nil {
		return hi, err
	}
	defer rs.Close()
	for rs.Next() {
		var h, col int64
		if err := rs.Scan(&h, &col); err != nil {
			return hi, err
		}
		hi.cols[col] = h
	}
	return hi, rs.Err()
}

// tableQuery donne la requête qui charge le tableau id (0 : aucune).
func (t *tx) tableQuery(id int64) (int64, error) {
	var q int64
	err := t.row(`SELECT id FROM _lattis_queries WHERE target = ? ORDER BY id LIMIT 1`, id).Scan(&q)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return q, err
}

// isHand dit si la colonne c du tableau est saisie : enregistrée comme
// telle, ou colonne calculée ajoutée avant les colonnes saisies (une
// colonne chargée n'a jamais de formule).
func (hi handInfo) isHand(c Column) bool {
	_, ok := hi.cols[c.ID]
	return ok || c.Formula != ""
}

// loadedCols donne les rangs des colonnes chargées par la requête.
func (hi handInfo) loadedCols(tb Table) []int {
	var out []int
	for i, c := range tb.Cols {
		if !hi.isHand(c) {
			out = append(out, i)
		}
	}
	return out
}

// keySep sépare les colonnes d'une clé de jointure composée (« id + Année » :
// les mêmes numéros dans deux tableaux réunis par UNION).
const keySep = " + "

// keyCols résout la clé de jointure key (une colonne, ou plusieurs séparées
// par keySep) en rangs de colonnes chargées ; nil si l'une n'y est plus (ou
// si la clé est le rang).
func (hi handInfo) keyCols(tb Table, key string) []int {
	if key == handRow || key == "" {
		return nil
	}
	loaded := hi.loadedCols(tb)
	find := func(name string) int {
		for _, i := range loaded {
			if strings.EqualFold(tb.Cols[i].Name, strings.TrimSpace(name)) {
				return i
			}
		}
		return -1
	}
	if i := find(key); i >= 0 {
		return []int{i}
	}
	var out []int
	for _, part := range strings.Split(key, keySep) {
		i := find(part)
		if i < 0 {
			return nil
		}
		out = append(out, i)
	}
	return out
}

// keyName écrit la clé formée des colonnes idx.
func keyName(tb Table, idx []int) string {
	names := make([]string, len(idx))
	for k, i := range idx {
		names[k] = tb.Cols[i].Name
	}
	return strings.Join(names, keySep)
}

// rowKey écrit la valeur de clé d'une ligne (valeurs des colonnes clés).
func rowKey(vals []any) string {
	if len(vals) == 1 {
		return handKeyText(vals[0])
	}
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = handKeyText(v)
	}
	return strings.Join(parts, "\x1f")
}

// ErrHandKey demande la clé de jointure des colonnes saisies d'un tableau
// chargé : Columns, les colonnes chargées (ou paires « a + b ») qui peuvent
// la donner (valeurs uniques, toutes remplies) ; aucune : le rang des
// lignes. La réponse revient par WithKeyChoice : « hand:<clé> » ou
// « hand:# ».
type ErrHandKey struct {
	Table   string
	Columns []string
}

func (e ErrHandKey) Error() string {
	s := fmt.Sprintf("jointure à choisir : « %s » est chargé par une requête ; les valeurs saisies à côté suivront ses lignes, d'une actualisation à l'autre, par une colonne qui les identifie.", e.Table)
	if len(e.Columns) == 0 {
		return s + " Aucune colonne ne convient (valeurs en double ou cases vides) : elles suivront le rang des lignes."
	}
	q := make([]string, len(e.Columns))
	for i, c := range e.Columns {
		q[i] = "« " + c + " »"
	}
	return s + " Colonnes possibles : " + strings.Join(q, ", ") + "."
}

// keyWhy dit pourquoi les colonnes idx ne peuvent pas servir de clé de
// jointure ("" : elles le peuvent). Une colonne : la règle de la clé du
// tableau (keyCheck) ; plusieurs : toutes remplies, combinaisons uniques.
func (t *tx) keyWhy(tb Table, idx []int) (string, error) {
	if len(idx) == 1 {
		return t.keyCheck(tb, idx[0])
	}
	var cols, nulls []string
	for _, i := range idx {
		c := tb.Cols[i]
		if c.Formula != "" {
			return fmt.Sprintf("« %s » est une colonne calculée", c.Name), nil
		}
		cols = append(cols, colSQL(c.ID))
		nulls = append(nulls, colSQL(c.ID)+" IS NULL")
	}
	var total, distinct, empty int
	q := `SELECT count(*), count(DISTINCT json_array(` + strings.Join(cols, ", ") + `)), coalesce(sum(` + strings.Join(nulls, " OR ") + `), 0) FROM ` + tb.sql()
	if err := t.row(q).Scan(&total, &distinct, &empty); err != nil {
		return "", err
	}
	if empty > 0 {
		return fmt.Sprintf("%d ligne(s) avec une case vide", empty), nil
	}
	if distinct < total {
		return fmt.Sprintf("%d combinaison(s) en double", total-distinct), nil
	}
	return "", nil
}

// handCandidates : colonnes chargées qui peuvent servir de clé de jointure ;
// à défaut, paires de colonnes (« id + Année »), les premières trouvées.
func (t *tx) handCandidates(tb Table, hi handInfo) ([]string, error) {
	var out []string
	loaded := hi.loadedCols(tb)
	for _, i := range loaded {
		why, err := t.keyWhy(tb, []int{i})
		if err != nil {
			return nil, err
		}
		if why == "" {
			out = append(out, tb.Cols[i].Name)
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	// Paires : d'abord avec « id », sur les premières colonnes seulement.
	if len(loaded) > 12 {
		loaded = loaded[:12]
	}
	slices.SortStableFunc(loaded, func(a, b int) int {
		ia, ib := strings.EqualFold(tb.Cols[a].Name, "id"), strings.EqualFold(tb.Cols[b].Name, "id")
		switch {
		case ia && !ib:
			return -1
		case ib && !ia:
			return 1
		}
		return 0
	})
	for x := 0; x < len(loaded) && len(out) < 4; x++ {
		for y := x + 1; y < len(loaded) && len(out) < 4; y++ {
			pair := []int{loaded[x], loaded[y]}
			why, err := t.keyWhy(tb, pair)
			if err != nil {
				return nil, err
			}
			if why == "" {
				out = append(out, keyName(tb, pair))
			}
		}
	}
	return out, nil
}

// autoHandKey : la clé du tableau si c'est une colonne chargée, sinon une
// colonne « id » sans doublon ni case vide ; "" s'il faut demander.
func (t *tx) autoHandKey(tb Table, hi handInfo) (string, error) {
	loaded := hi.loadedCols(tb)
	for _, i := range loaded {
		if tb.Key != 0 && tb.Cols[i].ID == tb.Key {
			return tb.Cols[i].Name, nil
		}
	}
	for _, i := range loaded {
		if strings.EqualFold(tb.Cols[i].Name, "id") {
			why, err := t.keyCheck(tb, i)
			if err != nil || why == "" {
				return tb.Cols[i].Name, err
			}
		}
	}
	return "", nil
}

// ensureHandKey choisit la clé de jointure si elle ne l'est pas encore (ou
// si sa colonne a disparu du résultat) : automatique, sinon la réponse
// portée par le contexte, sinon ErrHandKey.
func (t *tx) ensureHandKey(tb Table, hi *handInfo) error {
	if hi.keyed && (hi.key == handRow || hi.keyCols(tb, hi.key) != nil) {
		return nil
	}
	key, err := t.autoHandKey(tb, *hi)
	if err != nil {
		return err
	}
	if key == "" {
		c := string(KeyChoiceFrom(t.Ctx))
		if !strings.HasPrefix(c, "hand:") {
			cands, err := t.handCandidates(tb, *hi)
			if err != nil {
				return err
			}
			return ErrHandKey{Table: tb.Name, Columns: cands}
		}
		if key, err = t.checkHandKey(tb, *hi, strings.TrimPrefix(c, "hand:")); err != nil {
			return err
		}
	}
	return t.setHandKey(hi, key)
}

// checkHandKey vérifie une clé de jointure demandée (une colonne chargée,
// plusieurs séparées par « + », ou « # ») et rend son écriture exacte.
func (t *tx) checkHandKey(tb Table, hi handInfo, key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" || key == handRow {
		return handRow, nil
	}
	idx := hi.keyCols(tb, key)
	if idx == nil {
		return "", msg.New("hand.notLoadedColumn", msg.Args{"column": key, "table": tb.Name})
	}
	why, err := t.keyWhy(tb, idx)
	if err != nil {
		return "", err
	}
	name := keyName(tb, idx)
	if why != "" {
		return "", ErrKey{tb.Name, name, why}
	}
	return name, nil
}

// setHandKey enregistre la clé de jointure.
func (t *tx) setHandKey(hi *handInfo, key string) error {
	var err error
	if hi.keyed {
		if hi.key == key {
			return nil
		}
		u := `UPDATE _lattis_hand SET key = ? WHERE owner = ?`
		err = t.apply([]stmt{st(u, key, hi.owner)}, []stmt{st(u, hi.key, hi.owner)})
	} else {
		err = t.apply(
			[]stmt{st(`INSERT INTO _lattis_hand (owner, key) VALUES (?, ?)`, hi.owner, key)},
			[]stmt{st(`DELETE FROM _lattis_hand WHERE owner = ?`, hi.owner)},
		)
	}
	if err == nil {
		hi.key, hi.keyed = key, true
	}
	return err
}

// addHand enregistre la colonne col (identifiant dans le tableau chargé)
// comme colonne saisie.
func (t *tx) addHand(hi *handInfo, col int64) error {
	if _, ok := hi.cols[col]; ok {
		return nil
	}
	var h int64
	if err := t.row(`SELECT coalesce(max(h), 0) + 1 FROM _lattis_hand_cols WHERE owner = ?`, hi.owner).Scan(&h); err != nil {
		return err
	}
	if err := t.apply(
		[]stmt{st(`INSERT INTO _lattis_hand_cols (owner, h, col) VALUES (?, ?, ?)`, hi.owner, h, col)},
		[]stmt{st(`DELETE FROM _lattis_hand_cols WHERE owner = ? AND h = ?`, hi.owner, h)},
	); err != nil {
		return err
	}
	hi.cols[col] = h
	return nil
}

// handGrown : une colonne vient d'être ajoutée à droite du tableau tb
// (nom tapé à droite de l'en-tête). Sur un tableau chargé, elle devient
// une colonne saisie ; si elle a reçu des valeurs (cellules libres
// reprises), la clé de jointure doit être choisie.
func (t *tx) handGrown(tb Table) error {
	q, err := t.tableQuery(tb.ID)
	if err != nil || q == 0 {
		return err
	}
	if tb, err = t.table(tb.ID); err != nil {
		return err
	}
	hi, err := t.hand(queryOwner(q))
	if err != nil {
		return err
	}
	c := tb.Cols[len(tb.Cols)-1]
	// Identifiant repris d'une colonne saisie supprimée depuis : ses valeurs
	// mises de côté ne sont pas celles de la nouvelle colonne.
	if h, ok := hi.cols[c.ID]; ok {
		if err := t.forgetKept(hi.owner, `h = ?`, h); err != nil {
			return err
		}
	}
	if err := t.addHand(&hi, c.ID); err != nil {
		return err
	}
	var n int
	if err := t.row(`SELECT count(` + colSQL(c.ID) + `) FROM ` + tb.sql()).Scan(&n); err != nil || n == 0 {
		return err
	}
	return t.ensureHandKey(tb, &hi)
}

// handInput : saisie dans la cellule de la colonne i d'un tableau. Un
// tableau chargé ne se modifie que dans ses colonnes saisies ; la première
// valeur (pas une formule, qui vaut pour toute la colonne) demande la clé
// de jointure.
func (t *tx) handInput(tb Table, i int, raw string) error {
	q, err := t.tableQuery(tb.ID)
	if err != nil || q == 0 {
		return err
	}
	hi, err := t.hand(queryOwner(q))
	if err != nil {
		return err
	}
	c := tb.Cols[i]
	if !hi.isHand(c) {
		return ErrQueryTable
	}
	if err := t.addHand(&hi, c.ID); err != nil { // colonne calculée d'avant
		return err
	}
	if strings.TrimSpace(raw) == "" || isFormula(raw) {
		return nil
	}
	return t.ensureHandKey(tb, &hi)
}

// handCell est une valeur saisie : valeur typée, ou texte hors type.
type handCell struct {
	v       any
	invalid bool
}

// handCol est une colonne saisie emportée d'une actualisation à l'autre.
type handCol struct {
	h   int64
	c   Column
	val int // rang parmi les colonnes de valeurs (-1 : colonne calculée)
}

// handCarry emporte les colonnes saisies du tableau remplacé par une
// actualisation (handBefore) jusqu'au nouveau tableau (handAfter).
type handCarry struct {
	hi    handInfo
	cols  []handCol
	nvals int
	key   string // clé avec laquelle vals a été lu (handRow : rang)
	vals  map[string][]handCell
	order []string
	// lostKey : la colonne clé n'était plus dans le tableau remplacé ; les
	// valeurs ont été lues par rang et restent de côté.
	lostKey bool
}

// handBefore lit les colonnes saisies du tableau old, avant son
// remplacement par le résultat de la requête qid (headers : ses colonnes).
// Les colonnes saisies supprimées depuis la dernière actualisation sont
// oubliées, avec leurs valeurs mises de côté.
func (t *tx) handBefore(qid int64, old Table, headers []string, warnings *[]string) (*handCarry, error) {
	hi, err := t.hand(queryOwner(qid))
	if err != nil {
		return nil, err
	}
	inHeaders := func(name string) bool {
		return slices.ContainsFunc(headers, func(h string) bool { return strings.EqualFold(strings.TrimSpace(h), name) })
	}
	present := map[int64]bool{}
	hc := &handCarry{hi: hi, vals: map[string][]handCell{}}
	next := int64(0)
	for _, h := range hi.cols {
		next = max(next, h)
	}
	for _, c := range old.Cols {
		present[c.ID] = true
		h, ok := hi.cols[c.ID]
		if !ok {
			// Colonne calculée ajoutée à la main avant les colonnes saisies.
			if c.Formula == "" || inHeaders(c.Name) {
				continue
			}
			next++
			h = next
		}
		col := handCol{h: h, c: c, val: -1}
		if c.Formula == "" {
			col.val = hc.nvals
			hc.nvals++
		}
		hc.cols = append(hc.cols, col)
	}
	// Colonnes saisies supprimées : oubliées, avec leurs valeurs de côté.
	for col, h := range hi.cols {
		if present[col] {
			continue
		}
		delete(hc.hi.cols, col)
		if err := t.apply(
			[]stmt{st(`DELETE FROM _lattis_hand_cols WHERE owner = ? AND h = ?`, hi.owner, h)},
			[]stmt{st(`INSERT INTO _lattis_hand_cols (owner, h, col) VALUES (?, ?, ?)`, hi.owner, h, col)},
		); err != nil {
			return nil, err
		}
		if err := t.forgetKept(hi.owner, `h = ?`, h); err != nil {
			return nil, err
		}
	}
	if hc.nvals == 0 {
		return hc, nil
	}
	// Valeurs, ligne par ligne, par clé.
	hc.key = hi.key
	var kcs []int
	if !hi.keyed {
		auto, err := t.autoHandKey(old, hi)
		if err != nil {
			return nil, err
		}
		hc.key = auto
		if auto == "" {
			hc.key = handRow
		}
	}
	if hc.key != handRow {
		if kcs = hi.keyCols(old, hc.key); kcs == nil {
			hc.lostKey = true
		}
	}
	sel := []string{"id"}
	for _, i := range kcs {
		sel = append(sel, colSQL(old.Cols[i].ID))
	}
	at := len(sel) // première valeur saisie
	var ids []string
	for _, c := range hc.cols {
		if c.val >= 0 {
			sel = append(sel, colSQL(c.c.ID))
			ids = append(ids, strconv.FormatInt(c.c.ID, 10))
		}
	}
	bad := map[[2]int64]string{}
	rs, err := t.query(`SELECT rid, col, raw FROM _lattis_invalid WHERE tbl = ? AND col IN (`+strings.Join(ids, ", ")+`)`, old.ID)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var rid, col int64
		var raw string
		if err := rs.Scan(&rid, &col, &raw); err != nil {
			rs.Close()
			return nil, err
		}
		bad[[2]int64{rid, col}] = raw
	}
	rs.Close()
	rs, err = t.query(`SELECT ` + strings.Join(sel, ", ") + ` FROM ` + old.sql() + ` ORDER BY _pos`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	vals := make([]any, len(sel))
	ptrs := make([]any, len(sel))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	valCols := make([]Column, 0, hc.nvals)
	for _, c := range hc.cols {
		if c.val >= 0 {
			valCols = append(valCols, c.c)
		}
	}
	for n := 1; rs.Next(); n++ {
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		rid, _ := vals[0].(int64)
		cells := make([]handCell, hc.nvals)
		filled := false
		for j, c := range valCols {
			if raw, ok := bad[[2]int64{rid, c.ID}]; ok {
				cells[j] = handCell{raw, true}
			} else {
				cells[j] = handCell{v: vals[at+j]}
			}
			filled = filled || cells[j].v != nil
		}
		if !filled {
			continue
		}
		k := strconv.Itoa(n)
		if kcs != nil {
			k = rowKey(vals[1:at])
		} else if hc.lostKey {
			k = handRow + strconv.Itoa(n) // ne revient pas tout seul
		}
		if _, dup := hc.vals[k]; !dup {
			hc.order = append(hc.order, k)
		}
		hc.vals[k] = cells
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	if !hi.keyed && len(hc.order) > 0 {
		if hc.key == handRow {
			*warnings = append(*warnings, fmt.Sprintf("%s : pas de colonne clé choisie, les valeurs saisies suivent le rang des lignes", old.Name))
		}
		if err := t.setHandKey(&hc.hi, hc.key); err != nil {
			return nil, err
		}
	}
	return hc, nil
}

// handAfter remet les valeurs saisies dans le nouveau tableau tb, dont les
// colonnes saisies viennent d'être ajoutées (ids : h → identifiant). Les
// valeurs sans ligne (clé disparue ou en double) sont mises de côté ; celles
// mises de côté dont la clé revient sont rattachées.
func (t *tx) handAfter(hc *handCarry, tb Table, loaded int, ids map[int64]int64, warnings *[]string) error {
	owner := hc.hi.owner
	for _, c := range hc.cols {
		col := ids[c.h]
		if old, ok := hc.hi.cols[c.c.ID]; ok && old == c.h {
			u := `UPDATE _lattis_hand_cols SET col = ? WHERE owner = ? AND h = ?`
			if err := t.apply([]stmt{st(u, col, owner, c.h)}, []stmt{st(u, c.c.ID, owner, c.h)}); err != nil {
				return err
			}
			continue
		}
		if err := t.apply(
			[]stmt{st(`INSERT INTO _lattis_hand_cols (owner, h, col) VALUES (?, ?, ?)`, owner, c.h, col)},
			[]stmt{st(`DELETE FROM _lattis_hand_cols WHERE owner = ? AND h = ?`, owner, c.h)},
		); err != nil {
			return err
		}
	}
	if hc.nvals == 0 {
		return nil
	}
	valCols := make([]handCol, 0, hc.nvals)
	for _, c := range hc.cols {
		if c.val >= 0 {
			valCols = append(valCols, c)
		}
	}
	// Lignes du nouveau résultat, par clé.
	lt := tb
	lt.Cols = tb.Cols[:loaded]
	kcs := handInfo{cols: map[int64]int64{}}.keyCols(lt, hc.key)
	rows := map[string][]int64{}
	if hc.key == handRow || kcs != nil {
		sel := []string{"id"}
		for _, i := range kcs {
			sel = append(sel, colSQL(tb.Cols[i].ID))
		}
		rs, err := t.query(`SELECT ` + strings.Join(sel, ", ") + ` FROM ` + tb.sql() + ` ORDER BY _pos`)
		if err != nil {
			return err
		}
		vals := make([]any, len(sel))
		ptrs := make([]any, len(sel))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		for n := 1; rs.Next(); n++ {
			if err := rs.Scan(ptrs...); err != nil {
				rs.Close()
				return err
			}
			rid, _ := vals[0].(int64)
			key := strconv.Itoa(n)
			if kcs != nil {
				key = rowKey(vals[1:])
			}
			rows[key] = append(rows[key], rid)
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return err
		}
	} else if len(hc.order) > 0 {
		*warnings = append(*warnings, fmt.Sprintf("%s : la colonne clé « %s » n'est plus dans le résultat ; les valeurs saisies sont mises de côté (choisissez une autre clé ou rétablissez la colonne)", tb.Name, hc.key))
	}
	// Valeurs mises de côté auparavant.
	kept := map[string]map[int64]handCell{}
	rs, err := t.query(`SELECT k, h, v, invalid FROM _lattis_hand_kept WHERE owner = ?`, owner)
	if err != nil {
		return err
	}
	for rs.Next() {
		var k string
		var h int64
		var c handCell
		if err := rs.Scan(&k, &h, &c.v, &c.invalid); err != nil {
			rs.Close()
			return err
		}
		if kept[k] == nil {
			kept[k] = map[int64]handCell{}
		}
		kept[k][h] = c
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	type put struct {
		rid   int64
		cells []handCell
	}
	var puts []put
	var do, undo []stmt
	unkeep := func(k string) {
		for h, c := range kept[k] {
			do = append(do, st(`DELETE FROM _lattis_hand_kept WHERE owner = ? AND k = ? AND h = ?`, owner, k, h))
			undo = append(undo, st(`INSERT INTO _lattis_hand_kept (owner, k, h, v, invalid) VALUES (?, ?, ?, ?, ?)`, owner, k, h, c.v, c.invalid))
		}
	}
	// Valeurs du tableau remplacé.
	aside, dups := 0, 0
	for _, k := range hc.order {
		cells := hc.vals[k]
		if rid := rows[k]; len(rid) == 1 {
			puts = append(puts, put{rid[0], cells})
			unkeep(k) // la valeur du tableau l'emporte
			delete(kept, k)
			continue
		} else if len(rid) > 1 {
			dups++
		} else {
			aside++
		}
		unkeep(k)
		for j, c := range valCols {
			if cells[j].v == nil {
				continue
			}
			do = append(do, st(`INSERT INTO _lattis_hand_kept (owner, k, h, v, invalid) VALUES (?, ?, ?, ?, ?)`, owner, k, c.h, cells[j].v, cells[j].invalid))
			undo = append(undo, st(`DELETE FROM _lattis_hand_kept WHERE owner = ? AND k = ? AND h = ?`, owner, k, c.h))
		}
		delete(kept, k)
	}
	// Valeurs mises de côté dont la clé revient.
	back := 0
	for k, byH := range kept {
		rid := rows[k]
		if len(rid) != 1 {
			continue
		}
		cells := make([]handCell, len(valCols))
		found := false
		for j, c := range valCols {
			if v, ok := byH[c.h]; ok {
				cells[j], found = v, true
			}
		}
		if !found {
			continue
		}
		puts = append(puts, put{rid[0], cells})
		unkeep(k)
		back++
	}
	if aside > 0 {
		*warnings = append(*warnings, fmt.Sprintf("%s : %s, mises de côté (elles reviendront si la ligne revient)", tb.Name, plural(aside, "valeurs saisies d'une ligne disparue", fmt.Sprintf("valeurs saisies de %d lignes disparues", aside))))
	}
	if dups > 0 {
		*warnings = append(*warnings, fmt.Sprintf("%s : %s en double dans « %s », valeurs saisies mises de côté", tb.Name, plural(dups, "clé", fmt.Sprintf("%d clés", dups)), hc.key))
	}
	if back > 0 {
		*warnings = append(*warnings, fmt.Sprintf("%s : %s, rattachées à leur ligne revenue", tb.Name, plural(back, "valeurs mises de côté", fmt.Sprintf("valeurs mises de côté de %d lignes", back))))
	}
	if len(do) > 0 {
		// Chaque instruction a son inverse au même rang : l'annulation les
		// défait de la dernière à la première.
		slices.Reverse(undo)
		if err := t.apply(do, undo); err != nil {
			return err
		}
	}
	if len(puts) == 0 {
		return nil
	}
	// Valeurs posées par une table de chargement : quelques instructions
	// dans le journal quelle que soit la taille du tableau (le rétablissement
	// les rejoue ; purgeImports la supprime quand plus rien ne la cite).
	scratch := fmt.Sprintf("%shand_%d_%d", importPrefix, tb.ID, uniq())
	defs := []string{"rid INTEGER PRIMARY KEY"}
	for j := range valCols {
		defs = append(defs, fmt.Sprintf("v%d, b%d", j, j))
	}
	if _, err := t.ExecContext(t.Ctx, `CREATE TABLE `+scratch+` (`+strings.Join(defs, ", ")+`)`); err != nil {
		return err
	}
	ins, err := t.PrepareContext(t.Ctx, `INSERT INTO `+scratch+` VALUES (`+marks(1+2*len(valCols))+`)`)
	if err != nil {
		return err
	}
	defer ins.Close()
	args := make([]any, 1+2*len(valCols))
	for _, p := range puts {
		args[0] = p.rid
		for j, c := range valCols {
			typ := tb.Cols[handColIndex(tb, ids[c.h])].Type
			args[1+2*j], args[2+2*j] = nil, nil
			switch v, ok := fitType(p.cells[j].v, typ); {
			case p.cells[j].v == nil:
			case p.cells[j].invalid || !ok:
				args[2+2*j] = handKeyText(p.cells[j].v)
			default:
				args[1+2*j] = v
			}
		}
		if _, err := ins.ExecContext(t.Ctx, args...); err != nil {
			return err
		}
	}
	var fill []stmt
	for j, c := range valCols {
		col := colSQL(ids[c.h])
		fill = append(fill,
			st(fmt.Sprintf(`UPDATE %s SET %s = s.v%d FROM %s AS s WHERE s.rid = %s.id AND s.v%d IS NOT NULL`, tb.sql(), col, j, scratch, tb.sql(), j)),
			st(fmt.Sprintf(`INSERT INTO _lattis_invalid (tbl, rid, col, raw) SELECT ?, rid, ?, b%d FROM %s WHERE b%d IS NOT NULL`, j, scratch, j), tb.ID, ids[c.h]),
		)
	}
	// Annulation : le tableau tout neuf disparaît avec ses colonnes.
	return t.apply(fill, nil)
}

func handColIndex(tb Table, id int64) int {
	for i, c := range tb.Cols {
		if c.ID == id {
			return i
		}
	}
	return 0
}

// forgetKept oublie les valeurs mises de côté de owner qui répondent à
// where (annulable).
func (t *tx) forgetKept(owner, where string, args ...any) (err error) {
	_, err = t.forgetKeptN(owner, where, args...)
	return err
}

func (t *tx) forgetKeptN(owner, where string, args ...any) (int, error) {
	a := append([]any{owner}, args...)
	rs, err := t.query(`SELECT k, h, v, invalid FROM _lattis_hand_kept WHERE owner = ? AND `+where, a...)
	if err != nil {
		return 0, err
	}
	var undo []stmt
	keys := map[string]bool{}
	for rs.Next() {
		var k string
		var h int64
		var v any
		var inv bool
		if err := rs.Scan(&k, &h, &v, &inv); err != nil {
			rs.Close()
			return 0, err
		}
		keys[k] = true
		undo = append(undo, st(`INSERT INTO _lattis_hand_kept (owner, k, h, v, invalid) VALUES (?, ?, ?, ?, ?)`, owner, k, h, v, inv))
	}
	rs.Close()
	if err := rs.Err(); err != nil || len(undo) == 0 {
		return 0, err
	}
	return len(keys), t.apply([]stmt{st(`DELETE FROM _lattis_hand_kept WHERE owner = ? AND `+where, a...)}, undo)
}

// clearHand oublie tout l'état des colonnes saisies de owner : requête
// supprimée, ou tableau chargé disparu (les colonnes qu'il avait restent,
// en colonnes ordinaires, avec un tableau qui n'est plus chargé).
func (t *tx) clearHand(owner string) error {
	if err := t.forgetKept(owner, `1`); err != nil {
		return err
	}
	hi, err := t.hand(owner)
	if err != nil {
		return err
	}
	for col, h := range hi.cols {
		if err := t.apply(
			[]stmt{st(`DELETE FROM _lattis_hand_cols WHERE owner = ? AND h = ?`, owner, h)},
			[]stmt{st(`INSERT INTO _lattis_hand_cols (owner, h, col) VALUES (?, ?, ?)`, owner, h, col)},
		); err != nil {
			return err
		}
	}
	if hi.keyed {
		return t.apply(
			[]stmt{st(`DELETE FROM _lattis_hand WHERE owner = ?`, owner)},
			[]stmt{st(`INSERT INTO _lattis_hand (owner, key) VALUES (?, ?)`, owner, hi.key)},
		)
	}
	return nil
}

// handKeyText écrit une valeur de clé (ou une valeur gardée hors type) en
// texte : un entier rendu en décimal (2021.0) reste « 2021 ».
func handKeyText(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		if v == math.Trunc(v) && math.Abs(v) < 1e15 {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'g', -1, 64)
	case string:
		return v
	case []byte:
		return string(v)
	case bool:
		if v {
			return "1"
		}
		return "0"
	}
	return fmt.Sprint(v)
}

// fitType convertit une valeur gardée au stockage d'une colonne de type typ ;
// false si elle n'y entre pas (elle redevient alors une saisie hors type).
func fitType(v any, typ grid.Type) (any, bool) {
	switch typ {
	case grid.Integer, grid.Boolean:
		switch x := v.(type) {
		case int64:
			return x, true
		case float64:
			if x == math.Trunc(x) {
				return int64(x), true
			}
		}
		return nil, false
	case grid.Decimal:
		switch x := v.(type) {
		case int64:
			return float64(x), true
		case float64:
			return x, true
		}
		return nil, false
	}
	switch x := v.(type) {
	case string:
		return x, true
	case []byte:
		return string(x), true
	}
	return nil, false
}

// handLayout complète la description d'un tableau chargé (Layout).
func (t *tx) handLayout(tb Table, qid int64, ti *TableInfo) error {
	hi, err := t.hand(queryOwner(qid))
	if err != nil {
		return err
	}
	for i, c := range tb.Cols {
		if hi.isHand(c) && i < len(ti.Columns) {
			ti.Columns[i].Hand = true
		}
	}
	if hi.keyed {
		ti.HandKey = hi.key
	}
	return t.row(`SELECT count(DISTINCT k) FROM _lattis_hand_kept WHERE owner = ?`, hi.owner).Scan(&ti.HandKept)
}

// handTable lit un tableau chargé par une requête et l'état de ses colonnes
// saisies.
func (t *tx) handTable(table string) (Table, handInfo, error) {
	tb, err := t.editTable(table)
	if err != nil {
		return tb, handInfo{}, err
	}
	q, err := t.tableQuery(tb.ID)
	if err != nil {
		return tb, handInfo{}, err
	}
	if q == 0 {
		return tb, handInfo{}, msg.New("hand.notLoadedByQuery", msg.Args{"table": tb.Name})
	}
	hi, err := t.hand(queryOwner(q))
	return tb, hi, err
}

// SetHandKey change la clé de jointure des colonnes saisies d'un tableau
// chargé : column, une colonne chargée sans doublon ni case vide ; vide ou
// « # » : le rang des lignes. Les valeurs déjà dans le tableau restent sur
// leurs lignes ; celles mises de côté, rangées par l'ancienne clé, sont
// oubliées (annulable). Rend le nombre de lignes oubliées.
func (m *Model) SetHandKey(ctx context.Context, table, column string) (int, error) {
	forgot := 0
	err := m.Do(ctx, "Clé des colonnes saisies", func(t *tx) error {
		tb, hi, err := t.handTable(table)
		if err != nil {
			return err
		}
		key, err := t.checkHandKey(tb, hi, column)
		if err != nil {
			return err
		}
		if hi.keyed && hi.key == key {
			return nil
		}
		if forgot, err = t.forgetKeptN(hi.owner, `1`); err != nil {
			return err
		}
		return t.setHandKey(&hi, key)
	})
	return forgot, err
}

// ForgetHandKept oublie les valeurs saisies mises de côté (lignes
// disparues) d'un tableau chargé ; rend le nombre de lignes oubliées.
func (m *Model) ForgetHandKept(ctx context.Context, table string) (int, error) {
	n := 0
	err := m.Do(ctx, "Oublier les valeurs mises de côté", func(t *tx) error {
		// Tableau croisé (pivothand.go) : les étiquettes absentes du TCD.
		if _, err := t.tableByName(table); err != nil {
			p, ok, perr := t.pivotHandByName(table)
			if perr != nil {
				return perr
			}
			if ok {
				if err := t.guardObject(`_lattis_pivots`, p.ID); err != nil {
					return err
				}
				n, err = t.forgetPivotKept(p.ID, p.Cfg)
				return err
			}
		}
		_, hi, err := t.handTable(table)
		if err != nil {
			return err
		}
		n, err = t.forgetKeptN(hi.owner, `1`)
		return err
	})
	return n, err
}

// HandKept liste les valeurs mises de côté d'un tableau chargé : clé, puis
// une valeur (texte) par colonne saisie (cols : leurs noms).
func (m *Model) HandKept(ctx context.Context, table string) (cols []string, rows [][]string, err error) {
	err = m.read(ctx, func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		q, err := t.tableQuery(tb.ID)
		if err != nil || q == 0 {
			return err
		}
		hi, err := t.hand(queryOwner(q))
		if err != nil {
			return err
		}
		var hs []int64
		for _, c := range tb.Cols {
			if h, ok := hi.cols[c.ID]; ok && c.Formula == "" {
				hs = append(hs, h)
				cols = append(cols, c.Name)
			}
		}
		rs, err := t.query(`SELECT k, h, v FROM _lattis_hand_kept WHERE owner = ? ORDER BY k`, hi.owner)
		if err != nil {
			return err
		}
		defer rs.Close()
		at := map[string]int{}
		for rs.Next() {
			var k string
			var h int64
			var v any
			if err := rs.Scan(&k, &h, &v); err != nil {
				return err
			}
			i, ok := at[k]
			if !ok {
				i = len(rows)
				at[k] = i
				rows = append(rows, append([]string{k}, make([]string, len(hs))...))
			}
			if j := slices.Index(hs, h); j >= 0 {
				rows[i][1+j] = handKeyText(v)
			}
		}
		return rs.Err()
	})
	return cols, rows, err
}
