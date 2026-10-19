package canvas

import (
	"fmt"
	"strings"
)

// keptSheets dit, pour chaque feuille source qui ne partira pas avec ses
// tableaux regroupés (« Supprimer les onglets »), ce qu'elle contient encore
// (autre tableau, TCD, graphique, bouton) : elle est gardée, et le volet
// comme la barre d'état le disent (« Janvier gardée : elle contient aussi
// le tableau Remises »), au lieu d'un onglet laissé là sans explication.
func (t *tx) keptSheets(tables []Table, gone map[int64]bool) ([]string, error) {
	src := map[int64]bool{}
	for _, tb := range tables {
		src[tb.ID] = true
	}
	var out []string
	seen := map[int64]bool{}
	for _, tb := range tables {
		if gone[tb.Sheet] || seen[tb.Sheet] {
			continue
		}
		seen[tb.Sheet] = true
		var sheetName string
		if err := t.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, tb.Sheet).Scan(&sheetName); err != nil {
			return nil, err
		}
		var what []string
		rs, err := t.query(`SELECT id, name FROM _lattis_tables WHERE sheet = ? ORDER BY id`, tb.Sheet)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var id int64
			var name string
			if err := rs.Scan(&id, &name); err != nil {
				rs.Close()
				return nil, err
			}
			if !src[id] {
				what = append(what, "le tableau "+name)
			}
		}
		rs.Close()
		for _, o := range []struct{ table, one, many string }{
			{`_lattis_pivots`, "un tableau croisé", "%d tableaux croisés"},
			{`_lattis_charts`, "un graphique", "%d graphiques"},
			{`_lattis_buttons`, "un bouton", "%d boutons"},
		} {
			var n int
			if err := t.row(`SELECT count(*) FROM `+o.table+` WHERE sheet = ?`, tb.Sheet).Scan(&n); err != nil {
				return nil, err
			}
			if n > 0 {
				what = append(what, plural(n, o.one, fmt.Sprintf(o.many, n)))
			}
		}
		if len(what) == 0 {
			what = []string{"une page"}
		}
		out = append(out, fmt.Sprintf("« %s » gardée : elle contient aussi %s (à supprimer à la main si besoin)", sheetName, strings.Join(what, ", ")))
	}
	return out, nil
}
