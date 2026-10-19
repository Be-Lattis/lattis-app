package canvas

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
)

// Regrouper des onglets, lot 2 (docs/conception/regrouper-onglets-semblables.md,
// §8) : ce qui citait les anciens tableaux est réécrit sur le tableau commun
// filtré par la catégorie, dans la même étape d'annulation. C'est une autre
// sorte de changement que les renommages de refs.go (Janvier[Montant] devient
// « Ventes[Montant] où Mois = Janvier ») : formula.FilterTables pour les
// formules (cellules libres, colonnes calculées des autres tableaux, règles
// de mise en forme conditionnelle), un filtre de rapport pour les tableaux
// croisés et un filtre pour les graphiques. Ce qui n'a pas d'équivalent exact
// (une ligne précise, REFERENCE, un contrôle, une requête, un script) reste
// « à reprendre », et les anciens onglets sont alors gardés masqués (§12.1).

// mergeRefs : la reprise préparée (lecture seule), appliquée par mergeSheets.
type mergeRefs struct {
	rewritten []PendingRef
	pending   []PendingRef
	ups       []refUpdate
}

// refUpdate : un texte à remplacer (formule ou configuration JSON), désigné
// par sa table, sa colonne et sa clé.
type refUpdate struct {
	table, col, where string
	key               []any
	neu               string
}

// mergeTarget : un ancien tableau et sa place dans le tableau commun.
type mergeTarget struct {
	filtered formula.Filtered
	text     string // la valeur de la catégorie telle que l'affiche un filtre (TCD, graphique)
}

// mergeTargets : les anciens tableaux (nom en minuscules) et leur partie du
// tableau commun ; why dit pourquoi rien ne peut être réécrit (pas de
// colonne de catégorie, deux onglets de même catégorie).
func (t *tx) mergeTargets(j *mergeJob) (map[string]mergeTarget, string, error) {
	if !j.hasCat() {
		return nil, "pas de colonne de catégorie pour distinguer les onglets", nil
	}
	cat := j.plan.Category
	seen := map[string]string{}
	out := map[string]mergeTarget{}
	for _, s := range j.srcs {
		var text sql.NullString
		if err := t.row(`SELECT slt_text(?, ?)`, s.cat, string(cat.Type)).Scan(&text); err != nil {
			return nil, "", err
		}
		low := strings.ToLower(text.String)
		if other, dup := seen[low]; dup {
			return nil, fmt.Sprintf("« %s » et « %s » ont la même valeur de catégorie", other, s.sheet), nil
		}
		seen[low] = s.sheet
		value, crit := formula.CategoryNodes(formula.FromSQL(s.cat, cat.Type))
		if value == nil {
			return nil, "catégorie illisible en formule", nil
		}
		cols := map[string]string{}
		for i, c := range j.cols {
			if k := s.from[i]; k >= 0 {
				cols[strings.ToLower(s.tb.Cols[k].Name)] = c.Name
			}
		}
		out[strings.ToLower(s.tb.Name)] = mergeTarget{
			filtered: formula.Filtered{Table: j.plan.Name, Category: cat.Name, Value: value, Crit: crit, Columns: cols},
			text:     text.String,
		}
	}
	return out, "", nil
}

// refPlanner prépare la reprise d'une formule ou d'un objet.
type refPlanner struct {
	targets map[string]mergeTarget
	// whys : anciens tableaux dont rien n'est réécrit, et pourquoi (voir
	// mergeTargets) ; whyDetect les repère dans une formule.
	whys      map[string]string
	whyDetect map[string]formula.Filtered
	tables    map[string]formula.Filtered
	detect    map[string]formula.Filtered
	// plain : anciens tableaux gardés une seule fois (lot 4, famille
	// « once ») : un simple renommage vers le tableau commun.
	plain map[string]string
	rc    refChange // renommages des TCD et graphiques (tableau, colonnes)
}

func newRefPlanner(jobs []*mergeJob, targets map[string]mergeTarget, whys map[string]string, plain map[string]string) *refPlanner {
	p := &refPlanner{targets: targets, whys: whys, whyDetect: map[string]formula.Filtered{}, tables: map[string]formula.Filtered{}, detect: map[string]formula.Filtered{}, plain: plain}
	p.rc = refChange{Renames: formula.Renames{Tables: map[string]string{}, Columns: map[string]map[string]string{}}}
	for _, j := range jobs {
		for _, s := range j.srcs {
			low := strings.ToLower(s.tb.Name)
			if _, ok := plain[low]; ok {
				continue
			}
			p.detect[low] = formula.Filtered{Table: j.plan.Name}
			if tg, ok := targets[low]; ok {
				p.tables[low] = tg.filtered
				p.rc.Tables[low] = j.plan.Name
				p.rc.Columns[low] = tg.filtered.Columns
			}
			if _, ok := whys[low]; ok {
				p.whyDetect[low] = formula.Filtered{Table: j.plan.Name}
			}
		}
	}
	for low, to := range plain {
		p.detect[low] = formula.Filtered{Table: to}
		p.rc.Tables[low] = to
	}
	return p
}

// whyOf : la raison du premier ancien tableau sans réécriture cité par n.
func (p *refPlanner) whyOf(n formula.Node) (string, bool) {
	if len(p.whyDetect) == 0 {
		return "", false
	}
	if _, cites, _ := formula.FilterTables(n, p.whyDetect); !cites {
		return "", false
	}
	for low := range p.whyDetect {
		if _, c, _ := formula.FilterTables(n, map[string]formula.Filtered{low: p.whyDetect[low]}); c {
			return p.whys[low], true
		}
	}
	return "", true
}

// formula reprend une formule (avec ou sans « = ») écrite dans la langue
// loc ; self : le tableau de ses [Colonne]. cites : elle citait un ancien
// tableau ; why : elle reste à reprendre, et pourquoi.
func (p *refPlanner) formula(src string, loc formula.Locale, self string) (neu string, cites bool, why string) {
	n, err := formula.Parse(src, loc)
	if err != nil {
		return "", false, ""
	}
	if w, ok := p.whyOf(n); ok {
		return "", true, w
	}
	renamed := false
	if len(p.plain) > 0 {
		if x, ok := formula.RetargetTables(n, p.plain); ok {
			n, renamed = x, true
		}
	}
	out, cites, why := formula.FilterTables(n, p.tables)
	if why != "" {
		return "", true, why
	}
	if !cites && !renamed {
		return "", false, ""
	}
	neu = formula.Format(out, loc)
	if !strings.HasPrefix(strings.TrimSpace(src), "=") {
		neu = strings.TrimPrefix(neu, "=")
	}
	return neu, true, ""
}

// display : une formule anglaise (colonne calculée, règle) montrée en français.
func display(src string, loc formula.Locale) string {
	if loc == formula.FR {
		return "=" + strings.TrimPrefix(strings.TrimSpace(src), "=")
	}
	if n, err := formula.Parse(src, loc); err == nil {
		return formula.Format(n, formula.FR)
	}
	return src
}

// rules reprend une liste de règles JSON (value en français, f en anglais).
// before et after : la première formule touchée, en français.
func (p *refPlanner) rules(s, self string) (neu string, cites bool, why, before, after string) {
	if !strings.Contains(s, `"formula"`) {
		return "", false, "", "", ""
	}
	var list []map[string]any
	if json.Unmarshal([]byte(s), &list) != nil {
		return "", false, "", "", ""
	}
	if cites, why, before, after = p.ruleList(list, self); !cites || why != "" {
		return "", cites, why, before, after
	}
	out, err := encodeJSON(list)
	if err != nil {
		return "", true, err.Error(), before, after
	}
	return out, true, "", before, after
}

func (p *refPlanner) ruleList(list []map[string]any, self string) (cites bool, why, before, after string) {
	for _, rule := range list {
		if op, _ := rule["op"].(string); op != "formula" {
			continue
		}
		for _, k := range []struct {
			key string
			loc formula.Locale
		}{{"value", formula.FR}, {"f", formula.EN}} {
			v, ok := rule[k.key].(string)
			if !ok {
				continue
			}
			neu, c, w := p.formula(v, k.loc, self)
			if !c {
				continue
			}
			cites = true
			if before == "" {
				before = display(v, k.loc)
				if w == "" {
					after = display(neu, k.loc)
				}
			}
			if w != "" {
				return true, w, before, ""
			}
			rule[k.key] = neu
		}
	}
	return cites, "", before, after
}

// pivot reprend la configuration d'un TCD (posé sur une feuille ou dans un
// Document) : le tableau commun, ses colonnes, et un filtre de rapport sur
// la catégorie. Vide si le TCD ne lit pas un ancien tableau.
func (p *refPlanner) pivot(m map[string]any) (source, after, why string) {
	source, _ = m["source"].(string)
	if !p.isOld(source) {
		return "", "", ""
	}
	if to, ok := p.plain[strings.ToLower(source)]; ok {
		renamePivotConfig(m, p.rc)
		return source, fmt.Sprintf("sur « %s »", to), ""
	}
	tg, ok := p.targets[strings.ToLower(source)]
	if !ok {
		return source, "", p.whys[strings.ToLower(source)]
	}
	raw, _ := json.Marshal(m)
	var pc PivotConfig
	if json.Unmarshal(raw, &pc) != nil {
		return source, "", "configuration illisible"
	}
	pc = pc.normalized()
	fields := append(append([]string{}, pc.RowFields...), pc.Cols)
	for _, v := range pc.Values {
		fields = append(fields, v.Field)
	}
	for _, f := range append(append([]PivotFilter{}, pc.Filters...), pc.Items...) {
		fields = append(fields, f.Field)
	}
	if w := missingField(tg, source, fields); w != "" {
		return source, "", w
	}
	renamePivotConfig(m, p.rc)
	filters, _ := m["filters"].([]any)
	m["filters"] = append(filters, map[string]any{"field": tg.filtered.Category, "values": []any{tg.text}})
	return source, fmt.Sprintf("sur « %s », filtre %s = %s", tg.filtered.Table, tg.filtered.Category, tg.text), ""
}

// chart reprend la configuration d'un graphique tiré d'un ancien tableau :
// le tableau commun, ses colonnes, et un filtre sur la catégorie.
func (p *refPlanner) chart(m map[string]any) (source, after, why string) {
	source, _ = m["source"].(string)
	if !p.isOld(source) {
		return "", "", ""
	}
	if to, ok := p.plain[strings.ToLower(source)]; ok {
		renameChartConfig(m, p.rc, nil)
		return source, fmt.Sprintf("sur « %s »", to), ""
	}
	tg, ok := p.targets[strings.ToLower(source)]
	if !ok {
		return source, "", p.whys[strings.ToLower(source)]
	}
	var fields []string
	for _, k := range []string{"x", "colorBy"} {
		if v, _ := m[k].(string); v != "" {
			fields = append(fields, v)
		}
	}
	list, _ := m["series"].([]any)
	for _, v := range list {
		if s, ok := v.(string); ok {
			fields = append(fields, s)
		}
	}
	filters, _ := m["filters"].([]any)
	for _, f := range filters {
		if fm, ok := f.(map[string]any); ok {
			if s, _ := fm["field"].(string); s != "" {
				fields = append(fields, s)
			}
		}
	}
	if w := missingField(tg, source, fields); w != "" {
		return source, "", w
	}
	renameChartConfig(m, p.rc, nil)
	m["filters"] = append(filters, map[string]any{"field": tg.filtered.Category, "values": []any{tg.text}})
	return source, fmt.Sprintf("sur « %s », filtre %s = %s", tg.filtered.Table, tg.filtered.Category, tg.text), ""
}

func missingField(tg mergeTarget, source string, fields []string) string {
	for _, f := range fields {
		if f == "" {
			continue
		}
		if _, ok := tg.filtered.Columns[strings.ToLower(strings.TrimSpace(f))]; !ok {
			return fmt.Sprintf("la colonne « %s » de « %s » n'est pas dans « %s »", f, source, tg.filtered.Table)
		}
	}
	return ""
}

func (p *refPlanner) isOld(name string) bool {
	_, ok := p.detect[strings.ToLower(name)]
	return ok && name != ""
}

// planMergeRefs prépare la reprise de tout ce qui cite un tableau source,
// hors des feuilles qui partent avec eux (gone) et des sources elles-mêmes.
func (t *tx) planMergeRefs(j *mergeJob, gone map[int64]bool) (*mergeRefs, error) {
	return t.planMergeRefsAll([]*mergeJob{j}, nil, nil, gone)
}

// planMergeRefsAll : la reprise pour plusieurs tableaux communs à la fois
// (lot 4 : une famille de tableaux, un tableau commun) ; plain : anciens
// tableaux gardés une seule fois (minuscules → tableau commun), dropped :
// leurs exemplaires non gardés.
func (t *tx) planMergeRefsAll(jobs []*mergeJob, plain map[string]string, dropped []Table, gone map[int64]bool) (*mergeRefs, error) {
	j := jobs[0]
	targets := map[string]mergeTarget{}
	whys := map[string]string{}
	for _, job := range jobs {
		tg, why, err := t.mergeTargets(job)
		if err != nil {
			return nil, err
		}
		for _, s := range job.srcs {
			low := strings.ToLower(s.tb.Name)
			if _, ok := plain[low]; ok {
				continue
			}
			if x, ok := tg[low]; ok {
				targets[low] = x
			} else if why != "" {
				whys[low] = why
			}
		}
	}
	p := newRefPlanner(jobs, targets, whys, plain)
	out := &mergeRefs{rewritten: []PendingRef{}, pending: []PendingRef{}}
	srcID := map[int64]bool{}
	for _, job := range jobs {
		for _, s := range job.srcs {
			srcID[s.tb.ID] = true
		}
	}
	for _, tb := range dropped {
		srcID[tb.ID] = true
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
	// add range une formule ou un objet repris (neu) ou à reprendre (why).
	add := func(r PendingRef, why, neu string, up refUpdate) {
		if why != "" {
			r.Why = why
			out.pending = append(out.pending, r)
			return
		}
		out.rewritten = append(out.rewritten, r)
		up.neu = neu
		out.ups = append(out.ups, up)
	}
	cellWhere := func(sheet int64, row, col int) string {
		return sheetNames[sheet] + " " + formula.ColumnLetters(col) + fmt.Sprint(row+1)
	}

	// Formules des cellules libres.
	rs, err = t.query(`SELECT sheet, row, col, raw FROM _lattis_cells WHERE raw LIKE '=%' ORDER BY sheet, row, col`)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var r PendingRef
		if err := rs.Scan(&r.Sheet, &r.Row, &r.Col, &r.Formula); err != nil {
			rs.Close()
			return nil, err
		}
		if gone[r.Sheet] {
			continue
		}
		neu, cites, w := p.formula(r.Formula, formula.FR, "")
		if !cites {
			continue
		}
		r.Kind, r.SheetName, r.Where, r.After = "cell", sheetNames[r.Sheet], cellWhere(r.Sheet, r.Row, r.Col), neu
		add(r, w, neu, refUpdate{table: "_lattis_cells", col: "raw", where: "sheet = ? AND row = ? AND col = ?", key: []any{r.Sheet, r.Row, r.Col}})
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}

	// Colonnes calculées et règles des colonnes des autres tableaux.
	rs, err = t.query(`SELECT t.id, t.sheet, t.name, c.id, c.name, coalesce(c.formula, ''), coalesce(c.rules, '') FROM _lattis_columns c
		JOIN _lattis_tables t ON t.id = c.tbl WHERE (c.formula IS NOT NULL AND c.formula <> '') OR c.rules LIKE '%"formula"%' ORDER BY t.id, c.pos`)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var tbl, cid int64
		var r PendingRef
		var table, col, f, rules string
		if err := rs.Scan(&tbl, &r.Sheet, &table, &cid, &col, &f, &rules); err != nil {
			rs.Close()
			return nil, err
		}
		if srcID[tbl] || gone[r.Sheet] {
			continue
		}
		r.SheetName = sheetNames[r.Sheet]
		if f != "" {
			if neu, cites, w := p.formula(f, formula.EN, table); cites {
				c := r
				c.Kind, c.Where, c.Formula = "column", table+"["+col+"]", display(f, formula.EN)
				if w == "" {
					c.After = display(neu, formula.EN)
				}
				add(c, w, neu, refUpdate{table: "_lattis_columns", col: "formula", where: "tbl = ? AND id = ?", key: []any{tbl, cid}})
			}
		}
		if neu, cites, w, before, after := p.rules(rules, table); cites {
			c := r
			c.Kind, c.Where, c.Formula, c.After = "rule", "Mise en forme de "+table+"["+col+"]", before, after
			add(c, w, neu, refUpdate{table: "_lattis_columns", col: "rules", where: "tbl = ? AND id = ?", key: []any{tbl, cid}})
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}

	// Règles des lignes et des colonnes de la feuille.
	for _, q := range []struct{ table, key, label string }{{"_lattis_row_rules", "row", "ligne"}, {"_lattis_col_rules", "col", "colonne"}} {
		rs, err := t.query(`SELECT sheet, ` + q.key + `, rules FROM ` + q.table + ` WHERE rules LIKE '%"formula"%' ORDER BY sheet, ` + q.key)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var r PendingRef
			var k int
			var rules string
			if err := rs.Scan(&r.Sheet, &k, &rules); err != nil {
				rs.Close()
				return nil, err
			}
			if gone[r.Sheet] {
				continue
			}
			neu, cites, w, before, after := p.rules(rules, "")
			if !cites {
				continue
			}
			label := fmt.Sprint(k + 1)
			if q.key == "col" {
				label = formula.ColumnLetters(k)
				r.Col = k
			} else {
				r.Row = k
			}
			r.Kind, r.SheetName, r.Formula, r.After = "rule", sheetNames[r.Sheet], before, after
			r.Where = fmt.Sprintf("Mise en forme de la %s %s (%s)", q.label, label, r.SheetName)
			add(r, w, neu, refUpdate{table: q.table, col: "rules", where: "sheet = ? AND " + q.key + " = ?", key: []any{r.Sheet, k}})
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return nil, err
		}
	}

	// Règles posées sur des cellules libres (propriété cf de leur style).
	rs, err = t.query(`SELECT sheet, row, col, style FROM _lattis_cell_styles WHERE style LIKE '%"formula"%' ORDER BY sheet, row, col`)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var r PendingRef
		var s string
		if err := rs.Scan(&r.Sheet, &r.Row, &r.Col, &s); err != nil {
			rs.Close()
			return nil, err
		}
		if gone[r.Sheet] {
			continue
		}
		var style map[string]any
		if json.Unmarshal([]byte(s), &style) != nil {
			continue
		}
		raw, _ := json.Marshal(style["cf"])
		var list []map[string]any
		if json.Unmarshal(raw, &list) != nil {
			continue
		}
		cites, w, before, after := p.ruleList(list, "")
		if !cites {
			continue
		}
		style["cf"] = list
		neu, err := encodeJSON(style)
		if err != nil {
			w = err.Error()
		}
		r.Kind, r.SheetName, r.Formula, r.After = "rule", sheetNames[r.Sheet], before, after
		r.Where = "Mise en forme de " + cellWhere(r.Sheet, r.Row, r.Col)
		add(r, w, neu, refUpdate{table: "_lattis_cell_styles", col: "style", where: "sheet = ? AND row = ? AND col = ?", key: []any{r.Sheet, r.Row, r.Col}})
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}

	// Tableaux croisés et graphiques. Les en-têtes d'un TCD dont une colonne
	// change de nom (« Montant € » apparié à « Montant ») changent aussi :
	// les graphiques tirés du TCD suivent.
	headers, err := t.pivotHeaders(p.rc)
	if err != nil {
		return nil, err
	}
	for _, o := range []struct{ table, kind, label string }{{`_lattis_pivots`, "pivot", "Tableau croisé"}, {`_lattis_charts`, "chart", "Graphique"}} {
		rs, err := t.query(`SELECT id, sheet, row, col, config FROM ` + o.table + ` ORDER BY id`)
		if err != nil {
			return nil, err
		}
		type obj struct {
			id  int64
			r   PendingRef
			cfg string
		}
		var objs []obj
		for rs.Next() {
			var x obj
			if err := rs.Scan(&x.id, &x.r.Sheet, &x.r.Row, &x.r.Col, &x.cfg); err != nil {
				rs.Close()
				return nil, err
			}
			objs = append(objs, x)
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return nil, err
		}
		for _, x := range objs {
			var m map[string]any
			if gone[x.r.Sheet] || json.Unmarshal([]byte(x.cfg), &m) != nil {
				continue
			}
			up := refUpdate{table: o.table, col: "config", where: "id = ?", key: []any{x.id}}
			var source, after, w string
			if o.kind == "pivot" {
				source, after, w = p.pivot(m)
			} else {
				source, after, w = p.chart(m)
				if source == "" && renameChartConfig(m, p.rc, headers) {
					// Graphique tiré d'un TCD repris : ses séries suivent.
					if neu, err := encodeJSON(m); err == nil {
						up.neu = neu
						out.ups = append(out.ups, up)
					}
					continue
				}
			}
			if source == "" {
				continue
			}
			r := x.r
			r.Kind, r.SheetName = o.kind, sheetNames[r.Sheet]
			r.Where = cellWhere(r.Sheet, r.Row, r.Col)
			r.Formula = o.label + " sur « " + source + " »"
			if w == "" {
				r.After = o.label + " " + after
			}
			neu, err := encodeJSON(m)
			if err != nil && w == "" {
				w = err.Error()
			}
			add(r, w, neu, up)
		}
	}

	// Graphiques et TCD insérés dans les onglets Document.
	rs, err = t.query(`SELECT sheet, source FROM _lattis_pages WHERE kind = ? AND (source LIKE '%"lattisChart"%' OR source LIKE '%"lattisPivot"%') ORDER BY sheet`, KindDoc)
	if err != nil {
		return nil, err
	}
	type page struct {
		sheet int64
		src   string
	}
	var pages []page
	for rs.Next() {
		var x page
		if err := rs.Scan(&x.sheet, &x.src); err != nil {
			rs.Close()
			return nil, err
		}
		pages = append(pages, x)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}
	for _, pg := range pages {
		if gone[pg.sheet] {
			continue
		}
		var delta map[string]any
		if json.Unmarshal([]byte(pg.src), &delta) != nil {
			continue
		}
		ops, _ := delta["ops"].([]any)
		var cited []string
		var afters []string
		w := ""
		changed := false
		for _, op := range ops {
			o, _ := op.(map[string]any)
			ins, _ := o["insert"].(map[string]any)
			if c, ok := ins["lattisChart"].(map[string]any); ok {
				source, after, why := p.chart(c)
				if source == "" {
					changed = renameChartConfig(c, p.rc, headers) || changed
					continue
				}
				cited, afters = append(cited, "Graphique sur « "+source+" »"), append(afters, "Graphique "+after)
				if w == "" {
					w = why
				}
			}
			if pv, ok := ins["lattisPivot"].(map[string]any); ok {
				source, after, why := p.pivot(pv)
				if source == "" {
					continue
				}
				cited, afters = append(cited, "Tableau croisé sur « "+source+" »"), append(afters, "Tableau croisé "+after)
				if w == "" {
					w = why
				}
			}
		}
		up := refUpdate{table: "_lattis_pages", col: "source", where: "sheet = ?", key: []any{pg.sheet}}
		neu, err := encodeJSON(delta)
		if err != nil {
			return nil, err
		}
		if len(cited) == 0 {
			if changed {
				up.neu = neu
				out.ups = append(out.ups, up)
			}
			continue
		}
		r := PendingRef{Kind: "document", Sheet: pg.sheet, SheetName: sheetNames[pg.sheet], Where: "Document « " + sheetNames[pg.sheet] + " »",
			Formula: strings.Join(cited, ", ")}
		if w == "" {
			r.After = strings.Join(afters, ", ")
		}
		add(r, w, neu, up)
	}

	// Contrôles (liste tirée d'une colonne) : à reprendre.
	rs, err = t.query(`SELECT sheet, row, col, config FROM _lattis_buttons ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var r PendingRef
		var cfg string
		if err := rs.Scan(&r.Sheet, &r.Row, &r.Col, &cfg); err != nil {
			rs.Close()
			return nil, err
		}
		var m map[string]any
		if gone[r.Sheet] || json.Unmarshal([]byte(cfg), &m) != nil {
			continue
		}
		from, _ := m["optionsFrom"].(string)
		if from == "" {
			continue
		}
		n, err := formula.Parse(from, formula.FR)
		if err != nil {
			continue
		}
		if _, cites, _ := formula.FilterTables(n, p.detect); cites {
			r.Kind, r.SheetName, r.Where, r.Formula = "control", sheetNames[r.Sheet], "Contrôle en "+cellWhere(r.Sheet, r.Row, r.Col), "liste tirée de "+from
			r.Why = "un contrôle lit une colonne entière, pas une partie de tableau"
			out.pending = append(out.pending, r)
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, err
	}

	// Requêtes d'import qui lisent un tableau source : hors de portée, comme
	// le texte SQL et les scripts Lua (ni repérés ni réécrits).
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
		if tbl, _ := m["table"].(string); p.isOld(tbl) {
			out.pending = append(out.pending, PendingRef{Kind: "query", Where: "Requête « " + name + " »", Formula: "lit « " + tbl + " »",
				Why: "une requête lit tout le tableau : à reprendre (étape Filtrer sur " + catName(j) + ")"})
		}
	}
	rs.Close()
	return out, rs.Err()
}

func catName(j *mergeJob) string {
	if j.hasCat() {
		return j.plan.Category.Name
	}
	return "la catégorie"
}

// applyMergeRefs écrit la reprise préparée. L'annulation remet le texte
// d'avant l'écriture (un ancien tableau masqué a pu être renommé entre-temps).
func (t *tx) applyMergeRefs(ups []refUpdate) error {
	if len(ups) == 0 {
		return nil
	}
	var do, undo []stmt
	for _, u := range ups {
		var cur sql.NullString
		if err := t.row(`SELECT `+u.col+` FROM `+u.table+` WHERE `+u.where, u.key...).Scan(&cur); err != nil {
			return err
		}
		q := `UPDATE ` + u.table + ` SET ` + u.col + ` = ? WHERE ` + u.where
		do = append(do, st(q, append([]any{u.neu}, u.key...)...))
		undo = append(undo, st(q, append([]any{nullable(cur)}, u.key...)...))
	}
	return t.apply(do, undo)
}

// summaryHint : une feuille qui faisait une synthèse par onglet (au moins
// trois formules reprises) gagnerait à devenir un tableau croisé.
func summaryHint(j *mergeJob, rewritten []PendingRef) string {
	if !j.hasCat() {
		return ""
	}
	count := map[string]int{}
	var order []string
	for _, r := range rewritten {
		if r.Kind != "cell" {
			continue
		}
		if count[r.SheetName] == 0 {
			order = append(order, r.SheetName)
		}
		count[r.SheetName]++
	}
	for _, s := range order {
		if count[s] >= 3 {
			return fmt.Sprintf("La feuille « %s » fait une synthèse par %s : un tableau croisé sur « %s » (%s en lignes) la remplacerait et suivrait les nouvelles lignes", s, strings.ToLower(j.plan.Category.Name), j.plan.Name, j.plan.Category.Name)
		}
	}
	return ""
}
