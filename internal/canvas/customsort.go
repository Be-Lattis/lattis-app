package canvas

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Tri personnalisé, comme les listes personnalisées d'Excel : les valeurs
// d'une colonne (ou d'un champ de TCD) dans un ordre choisi, les autres
// ensuite dans l'ordre naturel, les vides toujours à la fin.
//
// Les valeurs se comparent au texte affiché (comme le filtre), sans tenir
// compte de la casse (FoldFunc). L'ordre d'une colonne de tableau est gardé
// dans le classeur (_lattis_info, clé « sortOrders », par nom de tableau et
// de colonne : il survit à l'actualisation d'un tableau chargé, qui
// recrée le tableau) ; celui d'un champ de TCD dans la configuration du TCD
// (PivotConfig.Orders). Les listes nommées de l'utilisateur sont dans
// _lattis_info, clé « customLists » ; les listes prédéfinies (jours, mois)
// ne s'enregistrent pas.

// CustomOrder est un ordre personnalisé : Values dans l'ordre voulu (List :
// la liste nommée d'où il vient, qui le fournit quand Values est vide) ;
// Desc renverse l'ordre de la liste (les autres valeurs restent après).
type CustomOrder struct {
	List   string   `json:"list,omitempty"`
	Values []string `json:"values"`
	Desc   bool     `json:"desc,omitempty"`
}

// CustomList est une liste personnalisée nommée (Builtin : prédéfinie).
type CustomList struct {
	Name    string   `json:"name"`
	Values  []string `json:"values"`
	Builtin bool     `json:"builtin,omitempty"`
}

// BuiltinLists sont les listes prédéfinies, comme celles d'Excel.
var BuiltinLists = []CustomList{
	{Name: "Jours", Values: []string{"lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi", "dimanche"}, Builtin: true},
	{Name: "Jours abrégés", Values: []string{"lun.", "mar.", "mer.", "jeu.", "ven.", "sam.", "dim."}, Builtin: true},
	{Name: "Mois", Values: []string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"}, Builtin: true},
	{Name: "Mois abrégés", Values: []string{"janv.", "févr.", "mars", "avr.", "mai", "juin", "juil.", "août", "sept.", "oct.", "nov.", "déc."}, Builtin: true},
}

// FoldFunc est la fonction SQL qui replie un texte pour comparer les
// valeurs d'un ordre personnalisé (minuscules Unicode, comme Fold).
const FoldFunc = "slt_fold"

// Fold replie un texte comme FoldFunc.
func Fold(s string) string { return strings.ToLower(s) }

// OrderCase est le rang (0, 1…) de la valeur expr dans values, NULL pour
// une valeur hors de la liste ; expr est comparée repliée.
func OrderCase(expr string, values []string) string {
	var b strings.Builder
	b.WriteString("CASE " + FoldFunc + "(" + expr + ")")
	seen := map[string]bool{}
	n := 0
	for _, v := range values {
		f := Fold(v)
		if seen[f] {
			continue
		}
		seen[f] = true
		fmt.Fprintf(&b, " WHEN %s THEN %d", sqlLiteral(f), n)
		n++
	}
	if n == 0 {
		return "NULL"
	}
	b.WriteString(" END")
	return b.String()
}

// OrderBy est le tri SQL d'une expression selon un ordre personnalisé :
// valeurs de la liste d'abord (dans son ordre, ou à rebours), puis les
// autres selon natural (croissant).
func OrderBy(expr, natural string, o CustomOrder) string {
	cs := OrderCase(expr, o.Values)
	dir := "ASC"
	if o.Desc {
		dir = "DESC"
	}
	return "(" + cs + ") IS NULL, (" + cs + ") " + dir + ", " + natural + " ASC"
}

const (
	customListsKey = "customLists"
	sortOrdersKey  = "sortOrders"
)

// cleanValues retire les vides (toujours en dernier) et les doublons
// (après repli).
func cleanValues(vals []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, v := range vals {
		if v == "" || seen[Fold(v)] {
			continue
		}
		seen[Fold(v)] = true
		out = append(out, v)
	}
	return out
}

// infoValue lit une clé de _lattis_info (had : la clé existe).
func (t *tx) infoValue(key string) (string, bool, error) {
	var raw string
	err := t.row(`SELECT value FROM _lattis_info WHERE key = ?`, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return raw, err == nil, err
}

// setInfo remplace une clé de _lattis_info (vide : retirée), annulable.
func (t *tx) setInfo(key, now string) error {
	old, had, err := t.infoValue(key)
	if err != nil {
		return err
	}
	if old == now && had == (now != "") {
		return nil
	}
	set := func(v string, exists bool) stmt {
		if !exists {
			return st(`DELETE FROM _lattis_info WHERE key = ?`, key)
		}
		return st(`INSERT INTO _lattis_info (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, v)
	}
	return t.apply([]stmt{set(now, now != "")}, []stmt{set(old, had)})
}

func (t *tx) userLists() ([]CustomList, error) {
	raw, _, err := t.infoValue(customListsKey)
	if err != nil || raw == "" {
		return []CustomList{}, err
	}
	out := []CustomList{}
	if json.Unmarshal([]byte(raw), &out) != nil {
		return []CustomList{}, nil // illisible : ignoré
	}
	return out, nil
}

// customLists : listes prédéfinies puis celles du classeur.
func (t *tx) customLists() ([]CustomList, error) {
	user, err := t.userLists()
	if err != nil {
		return nil, err
	}
	return append(slices.Clone(BuiltinLists), user...), nil
}

// CustomLists rend les listes personnalisées : prédéfinies (jours, mois)
// puis celles du classeur.
func (m *Model) CustomLists(ctx context.Context) ([]CustomList, error) {
	var out []CustomList
	err := m.read(ctx, func(t *tx) error {
		var err error
		out, err = t.customLists()
		return err
	})
	return out, err
}

// SetCustomLists remplace les listes personnalisées du classeur (les
// listes prédéfinies passées sont ignorées), en une étape d'annulation.
func (m *Model) SetCustomLists(ctx context.Context, lists []CustomList) error {
	clean := []CustomList{}
	seen := map[string]bool{}
	for _, b := range BuiltinLists {
		seen[strings.ToLower(b.Name)] = true
	}
	for _, l := range lists {
		if l.Builtin {
			continue
		}
		l.Name = strings.TrimSpace(l.Name)
		if l.Name == "" {
			return fmt.Errorf("liste personnalisée sans nom")
		}
		if seen[strings.ToLower(l.Name)] {
			return fmt.Errorf("la liste « %s » existe déjà", l.Name)
		}
		seen[strings.ToLower(l.Name)] = true
		l.Values = cleanValues(l.Values)
		if len(l.Values) == 0 {
			return fmt.Errorf("la liste « %s » est vide", l.Name)
		}
		clean = append(clean, l)
	}
	now := ""
	if len(clean) > 0 {
		b, err := json.Marshal(clean)
		if err != nil {
			return err
		}
		now = string(b)
	}
	return m.Do(ctx, "Listes personnalisées", func(t *tx) error { return t.setInfo(customListsKey, now) })
}

// resolveOrder complète un ordre donné par le nom d'une liste.
func (t *tx) resolveOrder(o CustomOrder) (CustomOrder, error) {
	o.List = strings.TrimSpace(o.List)
	if len(o.Values) == 0 && o.List != "" {
		lists, err := t.customLists()
		if err != nil {
			return o, err
		}
		i := slices.IndexFunc(lists, func(l CustomList) bool { return strings.EqualFold(l.Name, o.List) })
		if i < 0 {
			return o, fmt.Errorf("liste personnalisée « %s » introuvable", o.List)
		}
		o.List, o.Values = lists[i].Name, lists[i].Values
	}
	o.Values = cleanValues(o.Values)
	if len(o.Values) == 0 {
		return o, fmt.Errorf("ordre personnalisé vide : donnez des valeurs ou une liste")
	}
	return o, nil
}

// tableOrders : ordres personnalisés des colonnes d'un tableau (par nom de
// colonne) et la colonne du dernier tri personnalisé (Applied), ré-appliqué
// quand le tableau chargé par une requête est actualisé.
type tableOrders struct {
	Orders  map[string]CustomOrder `json:"orders,omitempty"`
	Applied string                 `json:"applied,omitempty"`
}

func (t *tx) sortOrders() (map[string]tableOrders, error) {
	raw, _, err := t.infoValue(sortOrdersKey)
	out := map[string]tableOrders{}
	if err != nil || raw == "" {
		return out, err
	}
	if json.Unmarshal([]byte(raw), &out) != nil {
		return map[string]tableOrders{}, nil
	}
	return out, nil
}

func (t *tx) setSortOrders(all map[string]tableOrders) error {
	for k, v := range all {
		if len(v.Orders) == 0 && v.Applied == "" {
			delete(all, k)
		}
	}
	now := ""
	if len(all) > 0 {
		b, err := json.Marshal(all)
		if err != nil {
			return err
		}
		now = string(b)
	}
	return t.setInfo(sortOrdersKey, now)
}

// foldKey trouve la clé de m égale à name sans tenir compte de la casse.
func foldKey[V any](m map[string]V, name string) (string, bool) {
	if _, ok := m[name]; ok {
		return name, true
	}
	for k := range m {
		if strings.EqualFold(k, name) {
			return k, true
		}
	}
	return name, false
}

// setAppliedOrder note la colonne du dernier tri personnalisé du tableau
// ("" : un tri ordinaire l'a remplacé).
func (t *tx) setAppliedOrder(table, col string) error {
	all, err := t.sortOrders()
	if err != nil {
		return err
	}
	k, ok := foldKey(all, table)
	if !ok && col == "" {
		return nil
	}
	to := all[k]
	if to.Applied == col {
		return nil
	}
	to.Applied = col
	all[k] = to
	return t.setSortOrders(all)
}

// columnOrder : ordre personnalisé enregistré d'une colonne.
func (t *tx) columnOrder(table, col string) (CustomOrder, bool, error) {
	all, err := t.sortOrders()
	if err != nil {
		return CustomOrder{}, false, err
	}
	k, ok := foldKey(all, table)
	if !ok {
		return CustomOrder{}, false, nil
	}
	ck, ok := foldKey(all[k].Orders, col)
	if !ok {
		return CustomOrder{}, false, nil
	}
	return all[k].Orders[ck], true, nil
}

// ColumnOrder rend l'ordre personnalisé enregistré de la colonne i d'un
// tableau (ok faux : aucun) et si c'est le dernier tri du tableau.
func (m *Model) ColumnOrder(ctx context.Context, table string, i int) (o CustomOrder, ok, applied bool, err error) {
	err = m.read(ctx, func(t *tx) error {
		tb, err := t.tableByName(table)
		if err != nil {
			return err
		}
		if i < 0 || i >= len(tb.Cols) {
			return fmt.Errorf("colonne %d hors du tableau", i)
		}
		if o, ok, err = t.columnOrder(tb.Name, tb.Cols[i].Name); err != nil || !ok {
			return err
		}
		all, err := t.sortOrders()
		if err != nil {
			return err
		}
		k, _ := foldKey(all, tb.Name)
		applied = strings.EqualFold(all[k].Applied, tb.Cols[i].Name)
		return nil
	})
	return o, ok, applied, err
}

// SortTableCustom trie un tableau selon la colonne i dans un ordre
// personnalisé, et l'enregistre pour la colonne (proposé au prochain tri,
// ré-appliqué à l'actualisation d'un tableau chargé), en une étape
// d'annulation.
func (m *Model) SortTableCustom(ctx context.Context, table string, i int, o CustomOrder) error {
	return m.Do(ctx, "Tri personnalisé", func(t *tx) error {
		tb, err := t.sortTable(table)
		if err != nil {
			return err
		}
		if i < 0 || i >= len(tb.Cols) {
			return fmt.Errorf("colonne %d hors du tableau", i)
		}
		if o, err = t.resolveOrder(o); err != nil {
			return err
		}
		if err := t.sortRows(tb, i, o.Desc, o.Values); err != nil {
			return err
		}
		all, err := t.sortOrders()
		if err != nil {
			return err
		}
		k, _ := foldKey(all, tb.Name)
		to := all[k]
		if to.Orders == nil {
			to.Orders = map[string]CustomOrder{}
		}
		ck, _ := foldKey(to.Orders, tb.Cols[i].Name)
		delete(to.Orders, ck)
		to.Orders[tb.Cols[i].Name] = o
		to.Applied = tb.Cols[i].Name
		delete(all, k)
		all[tb.Name] = to
		return t.setSortOrders(all)
	})
}

// reapplyOrder trie de nouveau un tableau selon son dernier tri
// personnalisé (actualisation d'un tableau chargé : ses lignes arrivent
// dans l'ordre de la requête).
func (t *tx) reapplyOrder(id int64) error {
	tb, err := t.table(id)
	if err != nil {
		return err
	}
	all, err := t.sortOrders()
	if err != nil {
		return err
	}
	k, ok := foldKey(all, tb.Name)
	if !ok || all[k].Applied == "" {
		return nil
	}
	i := slices.IndexFunc(tb.Cols, func(c Column) bool { return strings.EqualFold(c.Name, all[k].Applied) })
	if i < 0 {
		return nil // colonne disparue du résultat
	}
	o, ok, err := t.columnOrder(tb.Name, tb.Cols[i].Name)
	if err != nil || !ok || len(o.Values) == 0 {
		return err
	}
	return t.sortRows(tb, i, o.Desc, o.Values)
}

// rewriteSortOrders suit les tableaux et colonnes renommés.
func (t *tx) rewriteSortOrders(rc refChange) error {
	all, err := t.sortOrders()
	if err != nil || len(all) == 0 {
		return err
	}
	changed := false
	neu := map[string]tableOrders{}
	for name, to := range all {
		n := rc.Table(name)
		changed = changed || n != name
		cols := map[string]CustomOrder{}
		for c, o := range to.Orders {
			nc := rc.Column(name, c)
			changed = changed || nc != c
			cols[nc] = o
		}
		if to.Orders != nil {
			to.Orders = cols
		}
		if to.Applied != "" {
			if a := rc.Column(name, to.Applied); a != to.Applied {
				to.Applied, changed = a, true
			}
		}
		neu[n] = to
	}
	if !changed {
		return nil
	}
	return t.setSortOrders(neu)
}
