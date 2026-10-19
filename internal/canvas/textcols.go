package canvas

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// Convertir le texte en colonnes, comme l'assistant d'Excel (Données ›
// Outils de données › Convertir) : chaque cellule d'une colonne est coupée en
// parties (séparateurs ou largeur fixe), chaque partie convertie selon le
// format de sa colonne (Standard, Texte, Date ou non importée).
//
// Dans un tableau, la première partie importée reste dans la colonne
// d'origine et les suivantes deviennent de nouvelles colonnes insérées à
// droite, typées selon leurs valeurs. Dans des cellules libres, les parties
// s'écrivent à partir de la cellule de destination, vers la droite.

// TextColumnsOptions décrit une conversion.
type TextColumnsOptions struct {
	// Fixed : largeur fixe (coupures Breaks) ; sinon délimité.
	Fixed bool `json:"fixed"`
	// Delims : caractères séparateurs (tabulation, point-virgule, virgule,
	// espace, autre).
	Delims string `json:"delims"`
	// Merge : séparateurs consécutifs considérés comme un seul.
	Merge bool `json:"merge"`
	// Quote : qualificateur de texte (" ou ', vide : aucun).
	Quote string `json:"quote"`
	// Breaks : positions des coupures (en caractères), largeur fixe.
	Breaks []int `json:"breaks"`
	// Columns : format de chaque colonne produite (absente : Standard).
	Columns []TextColumnFormat `json:"columns"`
	// Decimal et Thousands : séparateurs des nombres (défaut « , » et espace).
	Decimal   string `json:"decimal"`
	Thousands string `json:"thousands"`
	// Dest : cellule de destination (« C3 », « $C$3 ») des cellules libres ;
	// vide : la première cellule convertie. Ignorée dans un tableau.
	Dest string `json:"dest"`
}

// TextColumnFormat est le format d'une colonne produite.
type TextColumnFormat struct {
	// Format : "" (Standard), "text", "date" ou "skip" (ne pas importer).
	Format string `json:"format"`
	// Order : ordre des dates (« DMY » par défaut, « MDY », « YMD », « DYM »,
	// « MYD », « YDM »).
	Order string `json:"order"`
}

// TextColumnsPreview est l'aperçu de l'assistant.
type TextColumnsPreview struct {
	Table  string `json:"table"`  // tableau converti ("" : cellules libres)
	Column string `json:"column"` // sa colonne
	Header bool   `json:"header"` // l'en-tête du tableau est converti aussi
	// Lines : premières lignes de texte (l'en-tête en premier s'il est
	// converti) ; Rows : leurs parties.
	Lines []string   `json:"lines"`
	Rows  [][]string `json:"rows"`
	Count int        `json:"count"` // colonnes produites (toutes les lignes)
	Total int        `json:"total"` // lignes converties
	// Overwrite : des cellules non vides de la destination seraient remplacées.
	Overwrite bool   `json:"overwrite"`
	Dest      string `json:"dest"` // destination par défaut
}

// previewLines borne l'aperçu.
const previewLines = 200

// ErrNoText signale une sélection sans texte à convertir.
var ErrNoText = errors.New("aucune donnée à convertir : sélectionnez des cellules d'une seule colonne")

// textSource est la colonne à convertir.
type textSource struct {
	sheet  int64
	tb     *Table // nil : cellules libres
	ci     int    // colonne du tableau
	header bool   // l'en-tête est converti (texte en tête de texts)
	ids    []int64
	vals   []any // valeurs actuelles des lignes ids
	rows   []int // cellules libres : lignes de la feuille
	col    int
	r0     int
	texts  []string
}

// textSource lit les textes de la colonne col, lignes r0 à r1 : une colonne
// de tableau (bornée au tableau) ou des cellules libres.
func (t *tx) textSource(sheet int64, r0, r1, col int) (textSource, error) {
	src := textSource{sheet: sheet, col: col, r0: r0}
	if r1 < r0 {
		r0, r1 = r1, r0
		src.r0 = r0
	}
	tables, err := t.tables(sheet)
	if err != nil {
		return src, err
	}
	for i := range tables {
		tb := tables[i]
		if col < tb.Col || col > tb.Right() || !tb.spansRows(r0, r1) {
			continue
		}
		// Une sélection qui déborde du tableau sur des cellules libres est refusée.
		var k int
		if err := t.row(`SELECT count(*) FROM _lattis_cells WHERE sheet = ? AND col = ? AND row BETWEEN ? AND ? AND (row < ? OR row > ?)`,
			sheet, col, r0, r1, tb.Row, tb.Bottom()).Scan(&k); err != nil {
			return src, err
		}
		if k > 0 {
			return src, fmt.Errorf("la sélection déborde du tableau « %s » : convertissez le tableau et les cellules libres séparément", tb.Name)
		}
		return src, t.tableSource(&src, tb, max(r0, tb.Row), min(r1, tb.Bottom()))
	}
	rs, err := t.query(`SELECT row, raw FROM _lattis_cells WHERE sheet = ? AND col = ? AND row BETWEEN ? AND ? ORDER BY row`, sheet, col, r0, r1)
	if err != nil {
		return src, err
	}
	defer rs.Close()
	for rs.Next() {
		var row int
		var raw string
		if err := rs.Scan(&row, &raw); err != nil {
			return src, err
		}
		// Les formules ne sont pas converties ; l'apostrophe qui force le
		// texte n'en fait pas partie.
		if isFormula(raw) || strings.TrimSpace(raw) == "" {
			continue
		}
		if s := strings.TrimLeft(raw, " \t"); strings.HasPrefix(s, "'") {
			raw = s[1:]
		}
		src.rows = append(src.rows, row)
		src.texts = append(src.texts, raw)
	}
	if err := rs.Err(); err != nil {
		return src, err
	}
	if len(src.rows) == 0 {
		return src, ErrNoText
	}
	return src, nil
}

// tableSource lit les lignes r0 à r1 (lignes de la feuille) d'une colonne de tableau.
func (t *tx) tableSource(src *textSource, tb Table, r0, r1 int) error {
	src.tb = &tb
	src.ci = src.col - tb.Col
	c := tb.Cols[src.ci]
	if c.Formula != "" {
		return ErrComputed{c.Name}
	}
	if tb.Filtered() {
		return tb.errFiltered(false)
	}
	if r0 == tb.Row {
		src.header = true
		src.texts = append(src.texts, c.Name)
		r0++
	}
	if r1 >= r0 {
		invalid := map[int64]string{}
		rs, err := t.query(`SELECT rid, raw FROM _lattis_invalid WHERE tbl = ? AND col = ?`, tb.ID, c.ID)
		if err != nil {
			return err
		}
		for rs.Next() {
			var rid int64
			var raw string
			if err := rs.Scan(&rid, &raw); err != nil {
				rs.Close()
				return err
			}
			invalid[rid] = raw
		}
		rs.Close()
		rs, err = t.query(`SELECT id, `+colSQL(c.ID)+` FROM `+tb.sql()+` ORDER BY _pos LIMIT ? OFFSET ?`, r1-r0+1, r0-tb.Row-1)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var id int64
			var v any
			if err := rs.Scan(&id, &v); err != nil {
				return err
			}
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			s := ""
			if v != nil {
				s = text(v, c.Type)
			} else {
				s = invalid[id]
			}
			src.ids = append(src.ids, id)
			src.vals = append(src.vals, v)
			src.texts = append(src.texts, s)
		}
		if err := rs.Err(); err != nil {
			return err
		}
	}
	if len(src.texts) == 0 {
		return ErrNoText
	}
	return nil
}

// splitText coupe une ligne selon les options.
func splitText(s string, o TextColumnsOptions) []string {
	r := []rune(s)
	if o.Fixed {
		breaks := slices.Clone(o.Breaks)
		slices.Sort(breaks)
		breaks = slices.Compact(breaks)
		var parts []string
		from := 0
		for _, b := range breaks {
			if b <= from {
				continue
			}
			to := min(b, len(r))
			parts = append(parts, strings.TrimSpace(string(r[from:to])))
			from = to
		}
		parts = append(parts, strings.TrimSpace(string(r[min(from, len(r)):])))
		return parts
	}
	delim := map[rune]bool{}
	for _, d := range o.Delims {
		delim[d] = true
	}
	var q rune
	if o.Quote != "" {
		q = []rune(o.Quote)[0]
	}
	var parts []string
	var cur strings.Builder
	start, afterDelim := true, false
	for i := 0; i < len(r); i++ {
		ch := r[i]
		if start && q != 0 && ch == q {
			// Champ entre qualificateurs : les séparateurs y sont du texte, un
			// qualificateur doublé est un qualificateur.
			for i++; i < len(r); i++ {
				if r[i] == q {
					if i+1 < len(r) && r[i+1] == q {
						cur.WriteRune(q)
						i++
						continue
					}
					break
				}
				cur.WriteRune(r[i])
			}
			start, afterDelim = false, false
			continue
		}
		if delim[ch] {
			if o.Merge && afterDelim {
				continue
			}
			parts = append(parts, cur.String())
			cur.Reset()
			start, afterDelim = true, true
			continue
		}
		cur.WriteRune(ch)
		start, afterDelim = false, false
	}
	return append(parts, cur.String())
}

// textPart est une partie convertie : son type ("" si vide), sa saisie
// (locale française, lisible par grid.Parse) et le texte d'origine.
type textPart struct {
	typ    grid.Type
	raw    string
	orig   string
	forced bool // format Texte
}

// convertPart convertit une partie selon le format de sa colonne.
func convertPart(p string, f TextColumnFormat, o TextColumnsOptions) textPart {
	s := strings.TrimSpace(p)
	if s == "" {
		return textPart{orig: p}
	}
	switch f.Format {
	case "text":
		return textPart{typ: grid.Text, raw: p, orig: p, forced: true}
	case "date":
		if d, ok := orderedDate(s, f.Order); ok {
			return textPart{typ: grid.Date, raw: d.Format("02/01/2006"), orig: p}
		}
	}
	if n, ok := localNumber(s, o.Decimal, o.Thousands); ok {
		return textPart{typ: Infer(n), raw: n, orig: p}
	}
	switch Infer(s) {
	case grid.Boolean:
		return textPart{typ: grid.Boolean, raw: strings.ToUpper(s), orig: p}
	case grid.Date:
		if d, ok := grid.ParseDate(s); ok {
			return textPart{typ: grid.Date, raw: d.Format("02/01/2006"), orig: p}
		}
	}
	// Un nombre écrit avec d'autres séparateurs que ceux choisis reste du texte.
	return textPart{typ: grid.Text, raw: p, orig: p}
}

// localNumber lit un nombre écrit avec les séparateurs choisis et le récrit à
// la française (« 1,234.5 » avec le point décimal → « 1234,5 ») ; un signe
// moins à la fin est accepté, comme dans Excel.
func localNumber(s, dec, thou string) (string, bool) {
	if dec == "" {
		dec = ","
	}
	if thou == dec {
		thou = ""
	}
	neg := false
	if len(s) > 1 && strings.HasSuffix(s, "-") {
		neg, s = true, strings.TrimSpace(strings.TrimSuffix(s, "-"))
	}
	switch thou {
	case "":
	case " ":
		s = strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) || r == ' ' || r == ' ' {
				return -1
			}
			return r
		}, s)
	default:
		s = strings.ReplaceAll(s, thou, "")
	}
	for _, sep := range []string{",", "."} {
		if sep != dec && strings.Contains(s, sep) {
			return "", false
		}
	}
	if dec != "," {
		s = strings.Replace(s, dec, ",", 1)
	}
	if neg {
		if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
			return "", false
		}
		s = "-" + s
	}
	switch Infer(s) {
	case grid.Integer, grid.Decimal:
		return s, true
	}
	return "", false
}

// orderedDate lit une date dont l'ordre jour, mois, année est donné (« DMY »
// par défaut) ; une année sur deux chiffres va de 1930 à 2029, comme Excel.
func orderedDate(s, order string) (time.Time, bool) {
	if len(order) != 3 {
		order = "DMY"
	}
	f := strings.FieldsFunc(s, func(r rune) bool { return r == '/' || r == '-' || r == '.' || r == ' ' })
	if len(f) != 3 {
		return time.Time{}, false
	}
	var d, m, y int
	yLen := 0
	for i, k := range strings.ToUpper(order) {
		n, err := strconv.Atoi(f[i])
		if err != nil || n < 0 {
			return time.Time{}, false
		}
		switch k {
		case 'D':
			d = n
		case 'M':
			m = n
		case 'Y':
			y, yLen = n, len(f[i])
		default:
			return time.Time{}, false
		}
	}
	if yLen <= 2 {
		if y < 30 {
			y += 2000
		} else {
			y += 1900
		}
	}
	if m < 1 || m > 12 || d < 1 || y < 1 || y > 9999 {
		return time.Time{}, false
	}
	out := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	if out.Day() != d {
		return time.Time{}, false
	}
	return out, true
}

// freeText est la saisie d'une partie dans une cellule libre : une partie au
// format Texte (ou un nombre aux séparateurs étrangers) que la cellule lirait
// autrement prend l'apostrophe qui force le texte.
func (p textPart) freeText() string {
	switch p.typ {
	case "":
		return ""
	case grid.Text:
		s := strings.TrimSpace(p.orig)
		_, isText := valueLiteral(p.orig).(formula.StrLit)
		inferred := Infer(p.orig)
		if isFormula(p.orig) || strings.HasPrefix(s, "'") ||
			p.forced && !isText || inferred == grid.Integer || inferred == grid.Decimal || inferred == grid.Date || inferred == grid.Boolean {
			return "'" + p.orig
		}
		return p.orig
	}
	return p.raw
}

// format donne le format de la colonne produite j.
func (o TextColumnsOptions) format(j int) TextColumnFormat {
	if j < len(o.Columns) {
		return o.Columns[j]
	}
	return TextColumnFormat{}
}

// split coupe toutes les lignes ; renvoie les parties et le nombre de colonnes.
func (o TextColumnsOptions) split(texts []string) ([][]string, int) {
	out := make([][]string, len(texts))
	n := 0
	for i, s := range texts {
		out[i] = splitText(s, o)
		n = max(n, len(out[i]))
	}
	return out, n
}

// imported donne les colonnes produites qui sont importées.
func (o TextColumnsOptions) imported(n int) []int {
	var out []int
	for j := range n {
		if o.format(j).Format != "skip" {
			out = append(out, j)
		}
	}
	return out
}

// dest donne la cellule de destination des cellules libres.
func (o TextColumnsOptions) dest(src textSource) (int, int, error) {
	s := strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(o.Dest), "$", ""), "=")
	if s == "" {
		return src.r0, src.col, nil
	}
	sheet, row, col, ok := formula.A1Address(s)
	if !ok || sheet != "" {
		return 0, 0, fmt.Errorf("destination « %s » : une cellule de la feuille, comme B2", o.Dest)
	}
	return row, col, nil
}

// TextToColumnsPreview donne l'aperçu de la conversion des cellules de la
// colonne col, lignes r0 à r1.
func (m *Model) TextToColumnsPreview(ctx context.Context, sheet int64, r0, r1, col int, o TextColumnsOptions) (TextColumnsPreview, error) {
	var out TextColumnsPreview
	err := m.read(ctx, func(t *tx) error {
		src, err := t.textSource(sheet, r0, r1, col)
		if err != nil {
			return err
		}
		parts, n := o.split(src.texts)
		k := min(len(src.texts), previewLines)
		out = TextColumnsPreview{Lines: src.texts[:k], Rows: parts[:k], Count: n, Total: len(src.texts),
			Header: src.header, Dest: formula.ColumnLetters(src.col) + strconv.Itoa(min(r0, r1)+1)}
		if src.tb != nil {
			out.Table, out.Column = src.tb.Name, src.tb.Cols[src.ci].Name
			return nil
		}
		out.Overwrite, err = t.textOverwrite(src, parts, o)
		return err
	})
	return out, err
}

// textOverwrite dit si l'écriture remplacerait des cellules libres non vides
// (autres que les cellules converties elles-mêmes) ; refuse une destination
// qui recouvre un tableau.
func (t *tx) textOverwrite(src textSource, parts [][]string, o TextColumnsOptions) (bool, error) {
	n := 0
	for _, p := range parts {
		n = max(n, len(p))
	}
	k := len(o.imported(n))
	if k == 0 {
		return false, nil
	}
	dr, dc, err := o.dest(src)
	if err != nil {
		return false, err
	}
	off := dr - src.r0
	lo, hi := src.rows[0]+off, src.rows[len(src.rows)-1]+off
	if lo < 0 {
		return false, errors.New("la destination sort de la feuille")
	}
	sheet := src.sheet
	tables, err := t.tables(sheet)
	if err != nil {
		return false, err
	}
	for _, tb := range tables {
		if tb.spansCols(dc, dc+k-1) && tb.spansRows(lo, hi) {
			return false, fmt.Errorf("la destination recouvre le tableau « %s »", tb.Name)
		}
	}
	source := map[int]bool{}
	for _, r := range src.rows {
		source[r] = true
	}
	rs, err := t.query(`SELECT row, col FROM _lattis_cells WHERE sheet = ? AND col BETWEEN ? AND ? AND row BETWEEN ? AND ?`, sheet, dc, dc+k-1, lo, hi)
	if err != nil {
		return false, err
	}
	defer rs.Close()
	dest := map[int]bool{}
	for _, r := range src.rows {
		dest[r+off] = true
	}
	for rs.Next() {
		var r, c int
		if err := rs.Scan(&r, &c); err != nil {
			return false, err
		}
		if dest[r] && !(c == src.col && source[r]) {
			return true, nil
		}
	}
	return false, rs.Err()
}

// TextToColumns convertit les cellules de la colonne col, lignes r0 à r1, en
// colonnes ; renvoie le nombre de colonnes importées. Une seule étape
// d'annulation.
func (m *Model) TextToColumns(ctx context.Context, sheet int64, r0, r1, col int, o TextColumnsOptions) (int, error) {
	n := 0
	err := m.Do(ctx, "Convertir", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		src, err := t.textSource(sheet, r0, r1, col)
		if err != nil {
			return err
		}
		parts, count := o.split(src.texts)
		imp := o.imported(count)
		if len(imp) == 0 {
			return errors.New("aucune colonne à importer")
		}
		n = len(imp)
		if src.tb != nil {
			return t.textToTable(src, parts, imp, o)
		}
		return t.textToCells(sheet, src, parts, imp, o)
	})
	return n, err
}

// textToCells écrit les parties dans des cellules libres.
func (t *tx) textToCells(sheet int64, src textSource, parts [][]string, imp []int, o TextColumnsOptions) error {
	if _, err := t.textOverwrite(src, parts, o); err != nil {
		return err
	}
	dr, dc, _ := o.dest(src)
	off := dr - src.r0
	for i, row := range src.rows {
		for k, j := range imp {
			p := ""
			if j < len(parts[i]) {
				p = parts[i][j]
			}
			if err := t.setFreeCell(sheet, row+off, dc+k, convertPart(p, o.format(j), o).freeText()); err != nil {
				return err
			}
		}
	}
	return nil
}

// textColumn est une colonne produite dans un tableau : son type et ses
// valeurs, ligne par ligne.
type textColumn struct {
	typ  grid.Type
	vals []any
}

// tableColumn type et convertit les parties d'une colonne produite ; cur est
// le type à joindre (les autres lignes de la colonne d'origine), "" sinon.
func tableColumn(ps []textPart, cur grid.Type) textColumn {
	typ := cur
	for _, p := range ps {
		if p.forced {
			typ = grid.Text
		}
		typ = Join(typ, p.typ)
	}
	if typ == "" {
		typ = grid.Text
	}
	out := textColumn{typ: typ, vals: make([]any, len(ps))}
	for i, p := range ps {
		if p.typ == "" {
			continue
		}
		if typ == grid.Text {
			s := p.orig
			if p.typ != grid.Text {
				s = strings.TrimSpace(s)
			}
			out.vals[i] = s
			continue
		}
		v, err := grid.Parse(typ, p.raw)
		if err != nil {
			// Booléen et nombres mêlés : la colonne passe en texte.
			if cur == grid.Text || typ == grid.Text {
				return out
			}
			return tableColumn(ps, grid.Text)
		}
		out.vals[i] = v
	}
	return out
}

// textToTable écrit les parties dans la colonne d'origine et de nouvelles
// colonnes insérées à droite.
func (t *tx) textToTable(src textSource, parts [][]string, imp []int, o TextColumnsOptions) error {
	tb := *src.tb
	c := tb.Cols[src.ci]
	body := parts
	var head []string
	if src.header {
		head, body = parts[0], parts[1:]
	}
	partsOf := func(j int) []textPart {
		out := make([]textPart, len(body))
		for i, ps := range body {
			p := ""
			if j < len(ps) {
				p = ps[j]
			}
			out[i] = convertPart(p, o.format(j), o)
		}
		return out
	}

	// Colonne d'origine : le type des autres lignes compte s'il en reste.
	var cur grid.Type
	if len(src.ids) < tb.Rows {
		var total, selected int
		if err := t.row(`SELECT count(` + colSQL(c.ID) + `) FROM ` + tb.sql()).Scan(&total); err != nil {
			return err
		}
		for _, v := range src.vals {
			if v != nil {
				selected++
			}
		}
		if total > selected {
			cur = c.Type
		}
	}
	first := tableColumn(partsOf(imp[0]), cur)
	if len(src.ids) > 0 && first.typ != c.Type {
		var err error
		if tb, err = t.retype(tb, src.ci, first.typ); err != nil {
			return err
		}
	}
	for i, id := range src.ids {
		if err := t.setValue(tb, id, tb.Cols[src.ci], first.vals[i], ""); err != nil {
			return err
		}
	}

	// Noms : l'en-tête converti les donne, sinon « Colonne 2 », « Colonne 3 »…
	used := map[string]bool{}
	for _, x := range tb.Cols {
		used[strings.ToLower(x.Name)] = true
	}
	headName := func(j int) string {
		if j < len(head) {
			return strings.TrimSpace(head[j])
		}
		return ""
	}
	if name := headName(imp[0]); src.header && name != "" && name != c.Name {
		if used[strings.ToLower(name)] {
			return fmt.Errorf("la colonne « %s » existe déjà", name)
		}
		delete(used, strings.ToLower(c.Name))
		used[strings.ToLower(name)] = true
		if err := t.rename(tb, tb.Cols[src.ci], name); err != nil {
			return err
		}
		c.Name = name
	}
	suffix := 2
	unique := func(name string) string {
		if name != "" && !used[strings.ToLower(name)] {
			used[strings.ToLower(name)] = true
			return name
		}
		base := name
		if base == "" {
			base = c.Name
		}
		for ; ; suffix++ {
			if n := fmt.Sprintf("%s %d", base, suffix); !used[strings.ToLower(n)] {
				used[strings.ToLower(n)] = true
				suffix++
				return n
			}
		}
	}

	// Nouvelles colonnes : ajoutées vides et typées, puis remplies (l'annulation
	// les supprime, leurs valeurs n'ont pas d'inverse à garder).
	rest := imp[1:]
	if len(rest) == 0 {
		return nil
	}
	var id int64
	if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_columns WHERE tbl = ?`, tb.ID).Scan(&id); err != nil {
		return err
	}
	right := tb.Right()
	cols := make([]textColumn, len(rest))
	ids := make([]int64, len(rest))
	for k, j := range rest {
		cols[k] = tableColumn(partsOf(j), "")
		ids[k] = id + int64(k)
		if err := t.addColumn(tb, ids[k], unique(headName(j)), cols[k].typ, src.ci+1+k); err != nil {
			return err
		}
	}
	if err := t.ensureGapRight(tb.Sheet, tb.ID, right, right+len(rest), tb.Row, tb.Bottom()); err != nil {
		return err
	}
	set := make([]string, len(rest))
	for k := range rest {
		set[k] = colSQL(ids[k]) + " = ?"
	}
	q := `UPDATE ` + tb.sql() + ` SET ` + strings.Join(set, ", ") + ` WHERE id = ?`
	var do []stmt
	for i, rid := range src.ids {
		args := make([]any, 0, len(rest)+1)
		empty := true
		for k := range rest {
			v := cols[k].vals[i]
			empty = empty && v == nil
			args = append(args, v)
		}
		if empty {
			continue
		}
		do = append(do, st(q, append(args, rid)...))
	}
	if len(do) == 0 {
		return nil
	}
	return t.apply(do, nil)
}
