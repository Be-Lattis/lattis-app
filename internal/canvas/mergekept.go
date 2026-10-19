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
func (t *tx) keptSheets(j *mergeJob, gone map[int64]bool) ([]string, error) {
	src := map[int64]bool{}
	for _, s := range j.srcs {
		src[s.tb.ID] = true
	}
	var out []string
	seen := map[int64]bool{}
	for _, s := range j.srcs {
		if gone[s.tb.Sheet] || seen[s.tb.Sheet] {
			continue
		}
		seen[s.tb.Sheet] = true
		var what []string
		rs, err := t.query(`SELECT id, name FROM _lattis_tables WHERE sheet = ? ORDER BY id`, s.tb.Sheet)
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
			if err := t.row(`SELECT count(*) FROM `+o.table+` WHERE sheet = ?`, s.tb.Sheet).Scan(&n); err != nil {
				return nil, err
			}
			if n > 0 {
				what = append(what, plural(n, o.one, fmt.Sprintf(o.many, n)))
			}
		}
		if len(what) == 0 {
			what = []string{"une page"}
		}
		out = append(out, fmt.Sprintf("« %s » gardée : elle contient aussi %s (à supprimer à la main si besoin)", s.sheet, strings.Join(what, ", ")))
	}
	return out, nil
}
