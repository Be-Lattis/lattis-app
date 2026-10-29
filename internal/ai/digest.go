package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/lalamefine/lattis/internal/api"
	"github.com/lalamefine/lattis/internal/doc"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/sharing"
)

// Digest du classeur, joint au premier message (Structure) : ce que le
// modèle doit savoir pour répondre sans explorer. Pour chaque tableau, sa
// clé, ses colonnes avec leur type et leur nombre de valeurs distinctes, les
// formules des colonnes calculées avec le type de leur résultat et leurs
// erreurs, les colonnes de référence ; les tableaux croisés (source,
// champs), les graphiques (type, source), les cellules nommées et les
// feuilles sans tableau. Aucune valeur de cellule, quel que soit le
// partage : les cardinalités et les erreurs sont celles que rendent déjà
// tables_profile et formulas_errors, la définition des tableaux croisés est
// donnée sans les valeurs de leurs filtres.
//
// Taille bornée (DigestBudget) : au-delà, les petits tableaux repassent à
// une ligne (nom, feuille, colonnes et types), puis la liste s'arrête sur
// « … N autres : tables_list ». En cas d'échec, l'ancienne ligne par
// tableau (thinStructure).

// DigestBudget borne le digest, en caractères (≈ 1 500 jetons).
const DigestBudget = 6000

// Coût des comptes de valeurs distinctes : au-delà de ProfileRows lignes,
// ils se font sur les ProfileSample premières lignes (rendus « ≥ N »).
var (
	ProfileRows   = 20000
	ProfileSample = 5000
)

// StructureIn décrit le classeur pour le premier message d'une
// conversation (digest), formules écrites pour la langue lang de
// l'interface ; vide s'il n'y a ni tableau ni tableau croisé.
func StructureIn(ctx context.Context, host api.Host, mode, lang string) string {
	ctx = api.WithoutPrompt(ctx)
	if s, ok := digest(ctx, host, mode, formula.ForLanguage(lang)); ok {
		return s
	}
	return thinStructure(ctx, host, mode)
}

type digestSheet struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type digestTable struct {
	sheet string
	order int // rang de la feuille, puis du tableau
	t     api.Table
	refs  map[string]string // colonne → « Tableau[Clé] »
	prof  tableProfile
}

type pivotDef struct {
	Source    string   `json:"source"`
	RowFields []string `json:"rowFields"`
	ColFields []string `json:"colFields"`
	Cols      string   `json:"cols"`
	Values    []struct {
		Field string `json:"field"`
		Agg   string `json:"agg"`
		Show  string `json:"show"`
	} `json:"values"`
	Filters []struct {
		Field string `json:"field"`
	} `json:"filters"`
}

type chartDef struct {
	Source string   `json:"source"`
	Kind   string   `json:"kind"`
	X      string   `json:"x"`
	Series []string `json:"series"`
}

// digest construit le digest ; faux si la lecture échoue ou si le classeur
// n'a ni tableau ni tableau croisé.
func digest(ctx context.Context, host api.Host, mode string, loc formula.Locale) (string, bool) {
	res, err := api.Invoke(ctx, host, "sheets.list", nil)
	if err != nil {
		return "", false
	}
	var sheets []digestSheet
	if b, err := json.Marshal(res); err != nil || json.Unmarshal(b, &sheets) != nil {
		return "", false
	}
	// Colonnes des tableaux croisés, telles que les lit une requête ou un
	// graphique (sans celles tirées des valeurs d'un champ hors partage).
	outCols := map[string][]string{}
	if res, err := api.Invoke(ctx, host, "tables.list", nil); err == nil {
		if l, ok := sharing.Filter("tables.list", res, sharing.Mode(mode)).([]api.TableEntry); ok {
			for _, t := range l {
				if t.Kind == "pivot" {
					outCols[t.Name] = t.Columns
				}
			}
		}
	}
	var tables []*digestTable
	var pivots, charts, names, bare []string
	nPivots, nCharts := 0, 0
	for si, sh := range sheets {
		if sh.Kind != "" {
			kind := "document"
			if sh.Kind == "html" {
				kind = "page web"
			}
			bare = append(bare, fmt.Sprintf("%s (%s)", oneLine(sh.Name), kind))
			continue
		}
		v, err := api.Invoke(ctx, host, "sheets.get", map[string]any{"sheet": float64(sh.ID)})
		if err != nil {
			return "", false
		}
		view, ok := v.(api.SheetView)
		if !ok {
			return "", false
		}
		for ti, t := range view.Tables {
			dt := &digestTable{sheet: sh.Name, order: si*1000 + ti, t: t, refs: map[string]string{}}
			if info, err := api.Invoke(ctx, host, "tables.get", map[string]any{"table": t.Name}); err == nil {
				if full, ok := info.(api.Table); ok {
					dt.t = full
					for _, c := range full.Columns {
						if c.Ref != "" {
							dt.refs[c.Name] = c.Ref
						}
					}
				}
			}
			tables = append(tables, dt)
		}
		for _, p := range view.Pivots {
			nPivots++
			pivots = append(pivots, pivotLine(p, sh.Name, outCols[p.Name]))
		}
		for _, c := range view.Charts {
			nCharts++
			charts = append(charts, chartLine(c, sh.Name))
		}
		for _, n := range view.Names {
			names = append(names, fmt.Sprintf("%s (%s!%s%d)", oneLine(n.Name), oneLine(sh.Name), formula.ColumnLetters(n.Col-1), n.Row))
		}
		if len(view.Tables) == 0 && len(view.Pivots) == 0 {
			what := "vide"
			switch {
			case len(view.Cells) > 0:
				what = fmt.Sprintf("%d cellule(s) libre(s)", len(view.Cells))
			case len(view.Charts) > 0:
				what = "graphiques"
			}
			bare = append(bare, fmt.Sprintf("%s (%s)", oneLine(sh.Name), what))
		}
	}
	if len(tables) == 0 && nPivots == 0 {
		return "", false
	}
	d, _ := host.Doc()
	for _, t := range tables {
		t.prof = profileOf(ctx, host, d, t.t)
	}

	var head strings.Builder
	fmt.Fprintf(&head, "%s — %d feuille(s), %d tableau(x), %d tableau(x) croisé(s), %d graphique(s) :\n", structureHead, len(sheets), len(tables), nPivots, nCharts)

	// Blocs des tableaux, détaillés ou en une ligne selon la place.
	full := make([]string, len(tables))
	short := make([]string, len(tables))
	for i, t := range tables {
		full[i] = tableBlock(t, loc)
		short[i] = tableLine(t)
	}
	var tail strings.Builder
	list := func(title string, items []string, max int) {
		if len(items) == 0 {
			return
		}
		if len(items) > max {
			items = append(slices.Clone(items[:max]), fmt.Sprintf("… %d autre(s)", len(items)-max))
		}
		if title == "" {
			for _, it := range items {
				tail.WriteString(it + "\n")
			}
			return
		}
		tail.WriteString(title + " : " + strings.Join(items, " ; ") + "\n")
	}
	list("", pivots, 20)
	list("", charts, 20)
	list("Cellules nommées", names, 30)
	list("Feuilles sans tableau", bare, 20)
	tail.WriteString("Repères : type puis nombre de valeurs distinctes (unique : clé possible ; peu de valeurs : bon champ de tableau croisé) ; « = » formule → type du résultat ; → T[C] : valeurs tirées de la clé de T.\n]\n")

	size := head.Len() + tail.Len()
	for _, b := range full {
		size += len(b)
	}
	chosen := slices.Clone(full)
	if size > DigestBudget {
		// Les plus petits tableaux d'abord en une ligne.
		idx := make([]int, len(tables))
		for i := range idx {
			idx[i] = i
		}
		sort.SliceStable(idx, func(a, b int) bool { return tables[idx[a]].t.Rows < tables[idx[b]].t.Rows })
		for _, i := range idx {
			if size <= DigestBudget {
				break
			}
			size += len(short[i]) - len(chosen[i])
			chosen[i] = short[i]
		}
	}
	var s strings.Builder
	s.WriteString(head.String())
	used := head.Len() + tail.Len()
	for i, b := range chosen {
		if used+len(b) > DigestBudget && i > 0 {
			fmt.Fprintf(&s, "… et %d autre(s) tableau(x) : tables_list\n", len(chosen)-i)
			break
		}
		used += len(b)
		s.WriteString(b)
	}
	s.WriteString(tail.String())
	return s.String(), true
}

var typeShort = map[string]string{"text": "txt", "integer": "ent.", "decimal": "déc.", "date": "date", "boolean": "bool", "time": "heure", "datetime": "date-heure"}

func shortType(t string) string {
	if s, ok := typeShort[t]; ok {
		return s
	}
	return t
}

// tableBlock : le tableau en détail.
func tableBlock(t *digestTable, loc formula.Locale) string {
	var b strings.Builder
	rows := t.t.Rows + t.t.Hidden
	fmt.Fprintf(&b, "%s · feuille %s · %d ligne(s)", oneLine(t.t.Name), oneLine(t.sheet), rows)
	key := ""
	for _, c := range t.t.Columns {
		if c.Key {
			key = c.Name
		}
	}
	if key != "" {
		b.WriteString(" · clé " + oneLine(key))
	} else {
		b.WriteString(" · sans clé")
		for _, c := range t.t.Columns {
			if d, ok := t.prof.distinct[c.Name]; ok && c.Formula == "" && !t.prof.sampled && rows > 1 && d == int64(rows) && (c.Type == "integer" || c.Type == "text") {
				b.WriteString(" (clé possible : " + oneLine(c.Name) + ")")
				break
			}
		}
	}
	if t.prof.sampled {
		fmt.Fprintf(&b, " · valeurs distinctes sur les %d premières lignes", ProfileSample)
	}
	b.WriteString("\n")
	var plain []string
	var calc []string
	for _, c := range t.t.Columns {
		if c.Formula != "" {
			f := c.Formula
			if loc != formula.FR {
				if s, err := formula.Translate(f, formula.FR, loc); err == nil {
					f = s
				}
			}
			line := "  " + oneLine(c.Name) + " " + oneLine(f)
			if st, ok := t.prof.results[c.Name]; ok {
				if st.result != "" {
					line += " → " + st.result
				}
				if e := errorSummary(st.errors, loc); e != "" {
					line += ", erreurs : " + e
				}
			}
			calc = append(calc, line)
			continue
		}
		item := oneLine(c.Name) + " " + shortType(c.Type)
		if c.Key {
			item += " clé"
		} else if d, ok := t.prof.distinct[c.Name]; ok {
			switch {
			case t.prof.sampled:
				item += fmt.Sprintf(" ≥%d", d)
			case d == int64(rows) && rows > 1:
				item += " unique"
			default:
				item += fmt.Sprintf(" %d", d)
			}
		}
		if r := t.refs[c.Name]; r != "" {
			item += " → " + oneLine(r)
		}
		if c.Errors > 0 {
			item += fmt.Sprintf(" (%d hors type)", c.Errors)
		}
		plain = append(plain, item)
	}
	for i := 0; i < len(plain); i += 6 {
		b.WriteString("  " + strings.Join(plain[i:min(i+6, len(plain))], " | ") + "\n")
	}
	for _, l := range calc {
		b.WriteString(l + "\n")
	}
	return b.String()
}

// tableLine : le tableau en une ligne (noms et types des colonnes).
func tableLine(t *digestTable) string {
	cols := make([]string, 0, len(t.t.Columns))
	for j, c := range t.t.Columns {
		if j == 40 {
			cols = append(cols, "…")
			break
		}
		typ := shortType(c.Type)
		if c.Formula != "" {
			typ = "formule"
		}
		cols = append(cols, oneLine(c.Name)+" "+typ)
	}
	return fmt.Sprintf("%s · feuille %s · %d ligne(s) : %s\n", oneLine(t.t.Name), oneLine(t.sheet), t.t.Rows+t.t.Hidden, strings.Join(cols, ", "))
}

// errorSummary : « 12 #N/A, 3 #REF! », par nombre décroissant, codes
// écrits pour loc.
func errorSummary(errs map[string]int64, loc formula.Locale) string {
	codes := make([]string, 0, len(errs))
	for c, n := range errs {
		if n > 0 {
			codes = append(codes, c)
		}
	}
	sort.Slice(codes, func(i, j int) bool {
		if errs[codes[i]] != errs[codes[j]] {
			return errs[codes[i]] > errs[codes[j]]
		}
		return codes[i] < codes[j]
	})
	parts := make([]string, len(codes))
	for i, c := range codes {
		parts[i] = fmt.Sprintf("%d %s", errs[c], formula.ErrorText(c, loc))
	}
	return strings.Join(parts, ", ")
}

var aggNames = map[string]string{"sum": "somme", "count": "nombre", "countDistinct": "nombre distinct", "avg": "moyenne", "min": "min", "max": "max"}

// pivotLine : un tableau croisé, sans les valeurs de ses filtres.
func pivotLine(o api.ObjectView, sheet string, out []string) string {
	var p pivotDef
	if b, err := json.Marshal(o.Config); err == nil {
		_ = json.Unmarshal(b, &p)
	}
	parts := []string{"Tableau croisé " + oneLine(o.Name) + " · feuille " + oneLine(sheet) + " ← " + oneLine(p.Source)}
	if len(p.RowFields) > 0 {
		parts = append(parts, "lignes "+joinNames(p.RowFields))
	}
	cols := p.ColFields
	if len(cols) == 0 && p.Cols != "" {
		cols = []string{p.Cols}
	}
	if len(cols) > 0 {
		parts = append(parts, "colonnes "+joinNames(cols))
	}
	if len(p.Values) > 0 {
		vals := make([]string, len(p.Values))
		for i, v := range p.Values {
			agg := aggNames[v.Agg]
			if agg == "" {
				agg = v.Agg
			}
			vals[i] = agg + " " + oneLine(v.Field)
			if v.Show != "" {
				vals[i] += " (" + v.Show + ")"
			}
		}
		parts = append(parts, "valeurs "+strings.Join(vals, ", "))
	}
	if len(p.Filters) > 0 {
		f := make([]string, len(p.Filters))
		for i, x := range p.Filters {
			f[i] = x.Field
		}
		parts = append(parts, "filtres "+joinNames(f))
	}
	if len(out) > 0 {
		parts = append(parts, "colonnes lues "+joinNames(out))
	}
	if o.Error != "" {
		parts = append(parts, "EN ERREUR")
	}
	return strings.Join(parts, " · ")
}

// chartLine : un graphique (titre, type, source, axes).
func chartLine(o api.ObjectView, sheet string) string {
	var c chartDef
	if b, err := json.Marshal(o.Config); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	title := "Graphique"
	if o.Name != "" {
		title += " « " + oneLine(o.Name) + " »"
	}
	parts := []string{title + " · feuille " + oneLine(sheet) + " ← " + oneLine(c.Source), c.Kind}
	if c.X != "" {
		parts = append(parts, "x "+oneLine(c.X))
	}
	if len(c.Series) > 0 {
		parts = append(parts, "séries "+joinNames(c.Series))
	}
	if o.Error != "" {
		parts = append(parts, "EN ERREUR")
	}
	return strings.Join(parts, " · ")
}

func joinNames(list []string) string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = oneLine(s)
	}
	return strings.Join(out, ", ")
}

// tableProfile : valeurs distinctes des colonnes saisies, résultat et
// erreurs des colonnes calculées.
type tableProfile struct {
	distinct map[string]int64
	results  map[string]colStats
	sampled  bool
}

// profileCache garde les valeurs distinctes par classeur et par version de
// la structure d'un tableau (nom, nombre de lignes, colonnes et types) :
// le digest se relève à chaque message sans recompter. Les résultats des
// colonnes calculées se recomptent toujours (ils dépendent d'autres
// tableaux).
var profileCache = struct {
	sync.Mutex
	m map[string]map[string]int64
}{m: map[string]map[string]int64{}}

func profileOf(ctx context.Context, host api.Host, d *doc.Document, t api.Table) tableProfile {
	rows := t.Rows + t.Hidden
	p := tableProfile{distinct: map[string]int64{}, results: map[string]colStats{}, sampled: rows > ProfileRows}
	from := quoteIdent(t.Name)
	if p.sampled {
		from = fmt.Sprintf("(SELECT * FROM %s LIMIT %d)", quoteIdent(t.Name), ProfileSample)
	}
	var plain []string
	sig := fmt.Sprintf("%p|%s|%d|%t", d, t.Name, rows, p.sampled)
	for _, c := range t.Columns {
		if c.Formula == "" {
			plain = append(plain, c.Name)
			sig += "|" + c.Name + ":" + c.Type
		} else if st, err := resultStatsFrom(ctx, host, from, c.Name); err == nil {
			p.results[c.Name] = st
		}
	}
	if len(plain) == 0 {
		return p
	}
	profileCache.Lock()
	cached, ok := profileCache.m[sig]
	profileCache.Unlock()
	if ok {
		p.distinct = cached
		return p
	}
	sel := make([]string, len(plain))
	for i, c := range plain {
		sel[i] = "COUNT(DISTINCT " + quoteIdent(c) + ")"
	}
	res, err := api.Invoke(ctx, host, "db.query", map[string]any{"sql": "SELECT " + strings.Join(sel, ", ") + " FROM " + from})
	if err != nil {
		return p
	}
	b, _ := json.Marshal(res)
	var r struct {
		Rows [][]any `json:"rows"`
	}
	if json.Unmarshal(b, &r) != nil || len(r.Rows) != 1 || len(r.Rows[0]) != len(plain) {
		return p
	}
	for i, c := range plain {
		if n, ok := r.Rows[0][i].(float64); ok {
			p.distinct[c] = int64(n)
		}
	}
	profileCache.Lock()
	if len(profileCache.m) > 256 {
		profileCache.m = map[string]map[string]int64{}
	}
	profileCache.m[sig] = p.distinct
	profileCache.Unlock()
	return p
}
