package canvas

import (
	"encoding/json"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
)

// Report des renommages dans tout le classeur.
//
// Renommer un tableau, une colonne, une feuille ou une cellule nommée, ou
// fusionner deux tableaux (le tableau absorbé disparaît, ses colonnes
// peuvent changer de nom), réécrit tout ce qui les cite, dans la même étape
// d'annulation : formules des cellules libres, colonnes calculées de tous
// les tableaux, règles de mise en forme conditionnelle par formule
// (colonnes, lignes et colonnes de la feuille, cellules), tableaux croisés,
// graphiques, contrôles (liste tirée d'une colonne, cellule liée),
// requêtes d'import et sources des validations (listes tirées d'une plage). rewriteRefs s'appelle avant de changer les noms : les
// formules sont lues avec les noms d'avant.
//
// Restent hors de portée : le texte SQL (onglet SQL, favoris, requêtes SQL
// du classeur, REQUETE dans une formule) et les scripts Lua.

// refChange : les renommages, et pour une fusion les colonnes que le
// tableau absorbé apporte au tableau qui le reçoit (sous leurs nouveaux
// noms, dans leur ordre) : une requête qui lisait le tableau absorbé n'en
// garde que celles-là, sous leurs anciens noms.
type refChange struct {
	formula.Renames
	kept map[string][]string // ancien nom du tableau absorbé (minuscules) → colonnes
	// pairs : colonnes renommées de chaque tableau (ancien nom, minuscules),
	// ancien et nouveau nom tels qu'écrits, dans l'ordre.
	pairs map[string][][2]string
}

// tableChange : le tableau old prend le nom neu.
func tableChange(old, neu string) refChange {
	return refChange{Renames: formula.Renames{Tables: lowerMap(old, neu)}}
}

// column ajoute une colonne renommée du tableau table (ancien nom).
func (rc *refChange) column(table, old, neu string) {
	if old == neu {
		return
	}
	k := strings.ToLower(table)
	if rc.Columns == nil {
		rc.Columns = map[string]map[string]string{}
	}
	if rc.Columns[k] == nil {
		rc.Columns[k] = map[string]string{}
	}
	rc.Columns[k][strings.ToLower(old)] = neu
	if rc.pairs == nil {
		rc.pairs = map[string][][2]string{}
	}
	rc.pairs[k] = append(rc.pairs[k], [2]string{old, neu})
}

func lowerMap(old, neu string) map[string]string {
	return map[string]string{strings.ToLower(old): neu}
}

func (t *tx) rewriteRefs(rc refChange) error {
	if rc.Empty() {
		return nil
	}
	steps := []func(refChange) error{t.rewriteCellFormulas, t.rewriteColumnFormulas, t.rewriteLineRules,
		t.rewriteCellRules, t.rewritePivots, t.rewriteCharts, t.rewriteButtons, t.rewriteQueries, t.rewriteDocObjects, t.rewriteSortOrders, t.rewriteValidations}
	for _, f := range steps {
		if err := f(rc); err != nil {
			return err
		}
	}
	return nil
}

// update applique des mises à jour d'une colonne texte : rows donne, pour
// chaque ligne, la clé (args du WHERE) et l'ancienne et la nouvelle valeur.
type textUpdate struct {
	key      []any
	old, neu any
}

func (t *tx) applyUpdates(q string, ups []textUpdate) error {
	if len(ups) == 0 {
		return nil
	}
	var do, undo []stmt
	for _, u := range ups {
		do = append(do, st(q, append([]any{u.neu}, u.key...)...))
		undo = append(undo, st(q, append([]any{u.old}, u.key...)...))
	}
	return t.apply(do, undo)
}

func (t *tx) rewriteCellFormulas(rc refChange) error {
	rs, err := t.query(`SELECT sheet, row, col, raw FROM _lattis_cells WHERE raw LIKE '=%'`)
	if err != nil {
		return err
	}
	var ups []textUpdate
	for rs.Next() {
		var sheet int64
		var row, col int
		var raw string
		if err := rs.Scan(&sheet, &row, &col, &raw); err != nil {
			rs.Close()
			return err
		}
		if neu, ok := formula.RenameRefs(raw, formula.FR, rc.Renames, ""); ok {
			ups = append(ups, textUpdate{[]any{sheet, row, col}, raw, neu})
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	return t.applyUpdates(`UPDATE _lattis_cells SET raw = ? WHERE sheet = ? AND row = ? AND col = ?`, ups)
}

// rewriteColumnFormulas : formules des colonnes calculées et règles des
// colonnes ; leurs [Colonne] sont celles de leur tableau.
func (t *tx) rewriteColumnFormulas(rc refChange) error {
	rs, err := t.query(`SELECT c.tbl, c.id, t.name, coalesce(c.formula, ''), coalesce(c.rules, '') FROM _lattis_columns c
		JOIN _lattis_tables t ON t.id = c.tbl WHERE c.formula IS NOT NULL OR c.rules LIKE '%"formula"%'`)
	if err != nil {
		return err
	}
	var fs, rules []textUpdate
	for rs.Next() {
		var tbl, id int64
		var name, f, r string
		if err := rs.Scan(&tbl, &id, &name, &f, &r); err != nil {
			rs.Close()
			return err
		}
		if neu, ok := formula.RenameRefs(f, formula.EN, rc.Renames, name); ok && f != "" {
			fs = append(fs, textUpdate{[]any{tbl, id}, f, neu})
		}
		if neu, ok := renameRuleList(r, rc.Renames, name); ok {
			rules = append(rules, textUpdate{[]any{tbl, id}, r, neu})
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	if err := t.applyUpdates(`UPDATE _lattis_columns SET formula = ? WHERE tbl = ? AND id = ?`, fs); err != nil {
		return err
	}
	return t.applyUpdates(`UPDATE _lattis_columns SET rules = ? WHERE tbl = ? AND id = ?`, rules)
}

// renameRuleList réécrit les formules d'une liste de règles JSON (value en
// français, f en anglais) ; les autres champs sont gardés tels quels.
func renameRuleList(s string, r formula.Renames, self string) (string, bool) {
	if !strings.Contains(s, `"formula"`) {
		return s, false
	}
	var list []map[string]any
	if json.Unmarshal([]byte(s), &list) != nil {
		return s, false
	}
	if !renameRules(list, r, self) {
		return s, false
	}
	out, err := encodeJSON(list)
	if err != nil {
		return s, false
	}
	return out, true
}

func renameRules(list []map[string]any, r formula.Renames, self string) bool {
	changed := false
	for _, rule := range list {
		if op, _ := rule["op"].(string); op != "formula" {
			continue
		}
		for key, loc := range map[string]formula.Locale{"value": formula.FR, "f": formula.EN} {
			if v, ok := rule[key].(string); ok {
				if neu, ok := formula.RenameRefs(v, loc, r, self); ok {
					rule[key] = neu
					changed = true
				}
			}
		}
	}
	return changed
}

func encodeJSON(v any) (string, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSpace(b.String()), nil
}

func (t *tx) rewriteLineRules(rc refChange) error {
	for _, q := range []struct{ table, key string }{{"_lattis_row_rules", "row"}, {"_lattis_col_rules", "col"}} {
		rs, err := t.query(`SELECT sheet, ` + q.key + `, rules FROM ` + q.table + ` WHERE rules LIKE '%"formula"%'`)
		if err != nil {
			return err
		}
		var ups []textUpdate
		for rs.Next() {
			var sheet int64
			var k int
			var r string
			if err := rs.Scan(&sheet, &k, &r); err != nil {
				rs.Close()
				return err
			}
			if neu, ok := renameRuleList(r, rc.Renames, ""); ok {
				ups = append(ups, textUpdate{[]any{sheet, k}, r, neu})
			}
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return err
		}
		if err := t.applyUpdates(`UPDATE `+q.table+` SET rules = ? WHERE sheet = ? AND `+q.key+` = ?`, ups); err != nil {
			return err
		}
	}
	return nil
}

// rewriteCellRules : règles posées sur des cellules libres (propriété « cf »
// de leur style).
func (t *tx) rewriteCellRules(rc refChange) error {
	rs, err := t.query(`SELECT sheet, row, col, style FROM _lattis_cell_styles WHERE style LIKE '%"formula"%'`)
	if err != nil {
		return err
	}
	var ups []textUpdate
	for rs.Next() {
		var sheet int64
		var row, col int
		var s string
		if err := rs.Scan(&sheet, &row, &col, &s); err != nil {
			rs.Close()
			return err
		}
		var style map[string]any
		if json.Unmarshal([]byte(s), &style) != nil {
			continue
		}
		raw, _ := json.Marshal(style["cf"])
		var list []map[string]any
		if json.Unmarshal(raw, &list) != nil || !renameRules(list, rc.Renames, "") {
			continue
		}
		style["cf"] = list
		if neu, err := encodeJSON(style); err == nil {
			ups = append(ups, textUpdate{[]any{sheet, row, col}, s, neu})
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	return t.applyUpdates(`UPDATE _lattis_cell_styles SET style = ? WHERE sheet = ? AND row = ? AND col = ?`, ups)
}

// configs lit les configurations JSON d'une table d'objets (TCD, graphiques,
// contrôles).
func (t *tx) configs(table string) (map[int64]string, []int64, error) {
	rs, err := t.query(`SELECT id, config FROM ` + table + ` ORDER BY id`)
	if err != nil {
		return nil, nil, err
	}
	defer rs.Close()
	out := map[int64]string{}
	var ids []int64
	for rs.Next() {
		var id int64
		var cfg string
		if err := rs.Scan(&id, &cfg); err != nil {
			return nil, nil, err
		}
		out[id] = cfg
		ids = append(ids, id)
	}
	return out, ids, rs.Err()
}

// renameField renomme un champ (colonne du tableau source, ancien nom).
func renameField(m map[string]any, key string, r formula.Renames, table string) bool {
	if v, ok := m[key].(string); ok && v != "" {
		if n := r.Column(table, v); n != v {
			m[key] = n
			return true
		}
	}
	return false
}

func renameFieldList(m map[string]any, key string, r formula.Renames, table string) bool {
	list, ok := m[key].([]any)
	if !ok {
		return false
	}
	changed := false
	for i, v := range list {
		if s, ok := v.(string); ok {
			if n := r.Column(table, s); n != s {
				list[i] = n
				changed = true
			}
		}
	}
	return changed
}

// rewritePivots : tableau source et champs des TCD.
func (t *tx) rewritePivots(rc refChange) error {
	cfgs, ids, err := t.configs(`_lattis_pivots`)
	if err != nil {
		return err
	}
	var ups []textUpdate
	for _, id := range ids {
		var m map[string]any
		if json.Unmarshal([]byte(cfgs[id]), &m) != nil {
			continue
		}
		if !renamePivotConfig(m, rc) {
			continue
		}
		if neu, err := encodeJSON(m); err == nil {
			ups = append(ups, textUpdate{[]any{id}, cfgs[id], neu})
		}
	}
	return t.applyUpdates(`UPDATE _lattis_pivots SET config = ? WHERE id = ?`, ups)
}

// renamePivotConfig : tableau source et champs d'une configuration de TCD
// (posé sur une feuille ou inséré dans un Document) ; vrai si elle change.
func renamePivotConfig(m map[string]any, rc refChange) bool {
	src, _ := m["source"].(string)
	changed := false
	if n := rc.Table(src); n != src {
		m["source"], changed = n, true
	}
	for _, k := range []string{"cols", "rows", "value"} {
		changed = renameField(m, k, rc.Renames, src) || changed
	}
	changed = renameFieldList(m, "rowFields", rc.Renames, src) || changed
	for _, k := range []string{"values", "filters", "items"} {
		list, _ := m[k].([]any)
		for _, v := range list {
			if f, ok := v.(map[string]any); ok {
				changed = renameField(f, "field", rc.Renames, src) || changed
			}
		}
	}
	if sort, ok := m["sort"].(map[string]any); ok {
		neu := map[string]any{}
		for k, v := range sort {
			n := rc.Column(src, k)
			changed = changed || n != k
			neu[n] = v
		}
		m["sort"] = neu
	}
	if orders, ok := m["orders"].(map[string]any); ok {
		neu := map[string]any{}
		for k, v := range orders {
			n := rc.Column(src, k)
			changed = changed || n != k
			neu[n] = v
		}
		m["orders"] = neu
	}
	return changed
}

// pivotHeaders : pour chaque TCD (nom, minuscules) dont le tableau source
// perd ou change des colonnes, l'ancienne et la nouvelle étiquette de ses
// colonnes (champs des lignes et des colonnes, valeurs), que les graphiques
// tirés du TCD désignent.
func (t *tx) pivotHeaders(rc refChange) (map[string]map[string]string, error) {
	cfgs, ids, err := t.configs(`_lattis_pivots`)
	if err != nil {
		return nil, err
	}
	out := map[string]map[string]string{}
	for _, id := range ids {
		var pc PivotConfig
		if json.Unmarshal([]byte(cfgs[id]), &pc) != nil || pc.Name == "" {
			continue
		}
		pc = pc.normalized()
		labels := map[string]string{}
		for _, f := range append(append([]string{}, pc.RowFields...), pc.Cols) {
			if n := rc.Column(pc.Source, f); f != "" && n != f {
				labels[strings.ToLower(f)] = n
			}
		}
		for _, v := range pc.Values {
			w := v
			w.Field = rc.Column(pc.Source, v.Field)
			if a, b := pivotLabel(v), pivotLabel(w); a != b {
				labels[strings.ToLower(a)] = b
			}
		}
		if len(labels) > 0 {
			out[strings.ToLower(pc.Name)] = labels
		}
	}
	return out, nil
}

// rewriteCharts : source et séries des graphiques (tirés d'un tableau ou
// d'un TCD).
func (t *tx) rewriteCharts(rc refChange) error {
	headers, err := t.pivotHeaders(rc)
	if err != nil {
		return err
	}
	cfgs, ids, err := t.configs(`_lattis_charts`)
	if err != nil {
		return err
	}
	var ups []textUpdate
	for _, id := range ids {
		var m map[string]any
		if json.Unmarshal([]byte(cfgs[id]), &m) != nil {
			continue
		}
		if !renameChartConfig(m, rc, headers) {
			continue
		}
		if neu, err := encodeJSON(m); err == nil {
			ups = append(ups, textUpdate{[]any{id}, cfgs[id], neu})
		}
	}
	return t.applyUpdates(`UPDATE _lattis_charts SET config = ? WHERE id = ?`, ups)
}

// renameChartConfig : source, axe X, séries et colonne de couleur d'une
// configuration de graphique (posé sur une feuille ou inséré dans un
// Document) ; headers donne les en-têtes renommés des TCD. Vrai si elle change.
func renameChartConfig(m map[string]any, rc refChange, headers map[string]map[string]string) bool {
	src, _ := m["source"].(string)
	changed := false
	cols := rc.Renames
	if h, ok := headers[strings.ToLower(src)]; ok {
		cols = formula.Renames{Columns: map[string]map[string]string{strings.ToLower(src): h}}
	}
	changed = renameField(m, "x", cols, src) || changed
	changed = renameFieldList(m, "series", cols, src) || changed
	changed = renameField(m, "colorBy", cols, src) || changed
	filters, _ := m["filters"].([]any)
	for _, v := range filters {
		if f, ok := v.(map[string]any); ok {
			changed = renameField(f, "field", cols, src) || changed
		}
	}
	if n := rc.Table(src); n != src {
		m["source"], changed = n, true
	}
	return changed
}

// rewriteDocObjects : graphiques et TCD insérés dans les onglets Document
// (embeds du Delta Quill, {"lattisChart": {…}} et {"lattisPivot": {…}}).
func (t *tx) rewriteDocObjects(rc refChange) error {
	headers, err := t.pivotHeaders(rc)
	if err != nil {
		return err
	}
	rs, err := t.query(`SELECT sheet, source FROM _lattis_pages WHERE kind = ? AND (source LIKE '%"lattisChart"%' OR source LIKE '%"lattisPivot"%')`, KindDoc)
	if err != nil {
		return err
	}
	var ups []textUpdate
	for rs.Next() {
		var sheet int64
		var src string
		if err := rs.Scan(&sheet, &src); err != nil {
			rs.Close()
			return err
		}
		var delta map[string]any
		if json.Unmarshal([]byte(src), &delta) != nil {
			continue
		}
		ops, _ := delta["ops"].([]any)
		changed := false
		for _, op := range ops {
			o, _ := op.(map[string]any)
			ins, _ := o["insert"].(map[string]any)
			if c, ok := ins["lattisChart"].(map[string]any); ok {
				changed = renameChartConfig(c, rc, headers) || changed
			}
			if p, ok := ins["lattisPivot"].(map[string]any); ok {
				changed = renamePivotConfig(p, rc) || changed
			}
		}
		if !changed {
			continue
		}
		if neu, err := encodeJSON(delta); err == nil {
			ups = append(ups, textUpdate{[]any{sheet}, src, neu})
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	return t.applyUpdates(`UPDATE _lattis_pages SET source = ? WHERE sheet = ?`, ups)
}

// rewriteButtons : colonne qui donne les choix d'une liste, feuille de la
// cellule liée.
func (t *tx) rewriteButtons(rc refChange) error {
	cfgs, ids, err := t.configs(`_lattis_buttons`)
	if err != nil {
		return err
	}
	var ups []textUpdate
	for _, id := range ids {
		var m map[string]any
		if json.Unmarshal([]byte(cfgs[id]), &m) != nil {
			continue
		}
		changed := false
		if from, ok := m["optionsFrom"].(string); ok && from != "" {
			if neu, ok := formula.RenameRefs(from, formula.FR, rc.Renames, ""); ok {
				m["optionsFrom"], changed = neu, true
			}
		}
		if link, ok := m["link"].(string); ok {
			if sh, cell := SplitLink(link); sh != "" {
				if n := rc.Sheet(sh); n != sh {
					m["link"], changed = n+"!"+cell, true
				}
			}
		}
		if !changed {
			continue
		}
		if neu, err := encodeJSON(m); err == nil {
			ups = append(ups, textUpdate{[]any{id}, cfgs[id], neu})
		}
	}
	return t.applyUpdates(`UPDATE _lattis_buttons SET config = ? WHERE id = ?`, ups)
}

// rewriteQueries : requêtes d'import qui lisent un tableau du classeur. Une
// colonne renommée du tableau source ne change pas le reste de la requête :
// une étape « renommer » (et, après une fusion, « garder les colonnes ») est
// ajoutée en tête, qui redonne à la requête les colonnes d'avant.
func (t *tx) rewriteQueries(rc refChange) error {
	rs, err := t.query(`SELECT id, source, steps FROM _lattis_queries ORDER BY id`)
	if err != nil {
		return err
	}
	type q struct {
		id            int64
		source, steps string
	}
	var qs []q
	for rs.Next() {
		var x q
		if err := rs.Scan(&x.id, &x.source, &x.steps); err != nil {
			rs.Close()
			return err
		}
		qs = append(qs, x)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	var srcUps, stepUps []textUpdate
	renameSource := func(s map[string]any) (string, bool) {
		kind, _ := s["kind"].(string)
		switch kind {
		case "table":
			tb, _ := s["table"].(string)
			if n := rc.Table(tb); n != tb {
				s["table"] = n
				return tb, true
			}
			return tb, false
		case SourceFormula:
			// Tableau lié à une formule : la formule suit les renommages
			// comme celle d'une cellule.
			if f, _ := s["formula"].(string); f != "" {
				if neu, ok := formula.RenameRefs(f, formula.FR, rc.Renames, ""); ok {
					s["formula"] = neu
					return "", true
				}
			}
		case "sql":
			if sh, _ := s["sheet"].(string); sh != "" {
				if n := rc.Sheet(sh); n != sh {
					s["sheet"] = n
					return "", true
				}
			}
		}
		return "", false
	}
	for _, x := range qs {
		var src map[string]any
		if json.Unmarshal([]byte(x.source), &src) != nil {
			continue
		}
		var steps []map[string]any
		if strings.TrimSpace(x.steps) != "" && json.Unmarshal([]byte(x.steps), &steps) != nil {
			continue
		}
		srcChanged := false
		table, ok := renameSource(src)
		srcChanged = ok
		if kind, _ := src["kind"].(string); kind == "table" {
			table, _ = src["table"].(string)
			if old := oldName(rc, table); old != "" {
				table = old
			}
		}
		stepsChanged := false
		for _, s := range steps {
			if o, ok := s["other"].(map[string]any); ok {
				if _, ok := renameSource(o); ok {
					stepsChanged = true
				}
			}
		}
		// Colonnes du tableau source renommées : étapes en tête.
		if kind, _ := src["kind"].(string); kind == "table" && table != "" {
			var head []map[string]any
			if kept, ok := rc.kept[strings.ToLower(table)]; ok {
				cols := make([]any, len(kept))
				for i, c := range kept {
					cols[i] = c
				}
				head = append(head, map[string]any{"op": "select", "columns": cols})
			}
			for _, p := range rc.pairs[strings.ToLower(table)] {
				head = append(head, map[string]any{"op": "rename", "column": p[1], "name": p[0]})
			}
			if len(head) > 0 {
				steps = append(head, steps...)
				stepsChanged = true
			}
		}
		if srcChanged {
			if neu, err := encodeJSON(src); err == nil {
				srcUps = append(srcUps, textUpdate{[]any{x.id}, x.source, neu})
			}
		}
		if stepsChanged {
			if neu, err := encodeJSON(steps); err == nil {
				stepUps = append(stepUps, textUpdate{[]any{x.id}, x.steps, neu})
			}
		}
	}
	if err := t.applyUpdates(`UPDATE _lattis_queries SET source = ? WHERE id = ?`, srcUps); err != nil {
		return err
	}
	return t.applyUpdates(`UPDATE _lattis_queries SET steps = ? WHERE id = ?`, stepUps)
}

// oldName donne l'ancien nom d'un tableau renommé en neu ("" sinon).
func oldName(rc refChange, neu string) string {
	for old, n := range rc.Tables {
		if strings.EqualFold(n, neu) {
			return old
		}
	}
	return ""
}
