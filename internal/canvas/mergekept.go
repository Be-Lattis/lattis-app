package canvas

import (
	"github.com/lalamefine/lattis/internal/msg"
	"strings"
)

// keptSheets dit, pour chaque feuille source qui ne partira pas avec ses
// tableaux regroupés (« Supprimer les onglets »), ce qu'elle contient encore
// (autre tableau, TCD, graphique, bouton) : elle est gardée, et le volet
// comme la barre d'état le disent (« Janvier gardée : elle contient aussi
// le tableau Remises »), au lieu d'un onglet laissé là sans explication.
// Son onglet passe en orange (KeptSheetColor, flagKeptSheets) pour qu'on le
// retrouve.
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
				what = append(what, t.say("mergeSheets.keptTable", msg.Args{"table": name}))
			}
		}
		rs.Close()
		for _, o := range []struct {
			table string
			id    msg.ID
		}{
			{`_lattis_pivots`, "mergeSheets.keptPivots"},
			{`_lattis_charts`, "mergeSheets.keptCharts"},
			{`_lattis_buttons`, "mergeSheets.keptButtons"},
			{`_lattis_objects`, "mergeSheets.keptObjects"},
		} {
			var n int
			if err := t.row(`SELECT count(*) FROM `+o.table+` WHERE sheet = ?`, tb.Sheet).Scan(&n); err != nil {
				return nil, err
			}
			if n > 0 {
				what = append(what, t.say(o.id, msg.Args{"n": n}))
			}
		}
		if len(what) == 0 {
			what = []string{t.say("mergeSheets.keptPage", nil)}
		}
		out = append(out, t.say("note.sheetKept", msg.Args{"sheet": sheetName, "what": strings.Join(what, ", ")}))
	}
	return out, nil
}

// KeptSheetColor : la couleur des anciens onglets gardés malgré
// « Supprimer » (l'orange de TAB_COLORS, frontend/src/lib/shell/sheets.ts).
const KeptSheetColor = "#ffc000"

// flagKeptSheets colore en orange, dans l'étape d'annulation du
// regroupement, les anciens onglets qui restent (même s'ils avaient déjà
// une couleur : l'annulation la leur rend). Ils restent visibles.
func (t *tx) flagKeptSheets(tables []Table, gone map[int64]bool) error {
	seen := map[int64]bool{}
	q := `UPDATE _lattis_sheets SET color = ? WHERE id = ?`
	for _, tb := range tables {
		if gone[tb.Sheet] || seen[tb.Sheet] {
			continue
		}
		seen[tb.Sheet] = true
		var old string
		if err := t.row(`SELECT color FROM _lattis_sheets WHERE id = ?`, tb.Sheet).Scan(&old); err != nil {
			return err
		}
		if old == KeptSheetColor {
			continue
		}
		if err := t.apply([]stmt{st(q, KeptSheetColor, tb.Sheet)}, []stmt{st(q, old, tb.Sheet)}); err != nil {
			return err
		}
	}
	return nil
}
