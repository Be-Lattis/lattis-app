package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
	"github.com/lalamefine/lattis/internal/msg"
	"github.com/ncruces/go-sqlite3"
)

// Rechercher / remplacer avec options, comme le dialogue d'un tableur
// (Ctrl+F, Ctrl+H, « Options >> ») : dans la feuille (ou la sélection) ou
// dans tout le classeur, par lignes ou par colonnes, dans les valeurs, les
// formules ou les notes, avec ou sans la casse, la cellule entière ou une
// partie, avec les jokers ? (un caractère) et * (une suite), que ~ protège
// (~?, ~*, ~~).
//
// Les cellules des tableaux se cherchent en SQL, sans rien lire en Go de ce
// qui ne correspond pas : LIKE sans la casse (les lettres accentuées, que
// LIKE ne replie pas, y valent un caractère quelconque et slt_find vérifie),
// GLOB avec. Les cellules libres, les en-têtes, les formules et les notes,
// peu nombreux, se comparent en Go. Une recherche rend au plus MaxMatches
// cellules dans l'ordre demandé, et le nombre total de cellules trouvées
// (un comptage SQL quand la limite est atteinte).
//
// Regarder dans :
//
//	formulas  la saisie : formules des cellules libres et des colonnes
//	          calculées (une occurrence par colonne, sur sa première ligne),
//	          valeurs saisies ailleurs ; c'est ce que Remplacer réécrit
//	values    ce qu'affiche la grille : résultats des formules et des
//	          colonnes calculées
//	notes     le texte des notes
//
// Les en-têtes de tableau se cherchent dans les formules et les valeurs. Les
// tableaux croisés, calculés, ne sont pas parcourus. Remplacer tout est une
// seule étape d'annulation, toutes feuilles comprises ; les cellules d'une
// feuille protégée, d'un tableau en lecture seule (classeur partagé) ou
// refusées (formule illisible, nom de colonne pris) sont laissées et
// comptées.

// Où chercher (SearchQuery.Look).
const (
	LookFormulas = "formulas"
	LookValues   = "values"
	LookNotes    = "notes"
)

// SearchQuery décrit une recherche.
type SearchQuery struct {
	Text string `json:"text"`
	// Sheet : la feuille cherchée (0 : la première du classeur, avec Book).
	Sheet int64 `json:"sheet"`
	// Book : tout le classeur (feuilles affichées, dans l'ordre des
	// onglets) ; Search ne cherche qu'une feuille à la fois et donne la
	// suivante (Next), ReplaceAll les fait toutes.
	Book bool `json:"book"`
	// In : la sélection (feuille seulement) ; nil : toute la feuille.
	In     *Rect  `json:"in,omitempty"`
	ByCols bool   `json:"byCols"`
	Look   string `json:"look"`
	Case   bool   `json:"case"`
	Whole  bool   `json:"whole"`
}

// Found est une cellule trouvée.
type Found struct {
	Sheet int64  `json:"sheet"`
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	Table string `json:"table,omitempty"`
	// Text : ce qui a été trouvé (valeur, formule ou note), coupé.
	Text string `json:"text"`
}

// SearchResult est le résultat d'une recherche dans une feuille.
type SearchResult struct {
	Sheet int64   `json:"sheet"`
	Name  string  `json:"name"`
	Found []Found `json:"found"`
	// Total : toutes les cellules trouvées dans la feuille (Found s'arrête à
	// MaxMatches).
	Total int `json:"total"`
	// Next : la feuille à chercher ensuite (Book), 0 après la dernière.
	Next int64 `json:"next"`
}

// ReplaceResult est le bilan de ReplaceAll.
type ReplaceResult struct {
	Replaced int `json:"replaced"`
	// Skipped : cellules trouvées laissées (protection, lecture seule,
	// refus) ; Reason : le premier motif.
	Skipped int    `json:"skipped"`
	Reason  string `json:"reason,omitempty"`
}

// --- Motif -------------------------------------------------------------------

// findPattern est un texte cherché, compilé en expression régulière (la
// vérification exacte, Go) et en motif SQL (le tri des lignes).
type findPattern struct {
	re    *regexp.Regexp
	like  string // sans la casse : LIKE … ESCAPE '\'
	glob  string // avec la casse : GLOB
	cased bool
	exact bool   // LIKE suffit (sinon slt_find vérifie)
	spec  string // argument de slt_find
	lits  string // caractères littéraux (canHold)
}

// compileFind compile un texte cherché avec ses jokers.
func compileFind(text string, cased, whole bool) *findPattern {
	var re, like, glob, lits strings.Builder
	re.WriteString("(?s)")
	if !cased {
		re.WriteString("(?i)")
	}
	if whole {
		re.WriteString(`\A`)
	} else {
		like.WriteByte('%')
		glob.WriteByte('*')
	}
	exact := true
	rs := []rune(text)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '~' && i+1 < len(rs) && (rs[i+1] == '*' || rs[i+1] == '?' || rs[i+1] == '~'):
			i++
			r = rs[i]
		case r == '*':
			re.WriteString(".*")
			like.WriteByte('%')
			glob.WriteByte('*')
			continue
		case r == '?':
			re.WriteByte('.')
			like.WriteByte('_')
			glob.WriteByte('?')
			continue
		}
		re.WriteString(regexp.QuoteMeta(string(r)))
		lits.WriteRune(r)
		switch {
		case r == '\\' || r == '%' || r == '_':
			like.WriteByte('\\')
			like.WriteRune(r)
		case r > unicode.MaxASCII && unicode.SimpleFold(r) != r:
			// LIKE ne replie que l'ASCII : É vaut ici un caractère quelconque.
			like.WriteByte('_')
			exact = false
		default:
			like.WriteRune(r)
		}
		if r == '*' || r == '?' || r == '[' {
			glob.WriteString("[" + string(r) + "]")
		} else {
			glob.WriteRune(r)
		}
	}
	if whole {
		re.WriteString(`\z`)
	} else {
		like.WriteByte('%')
		glob.WriteByte('*')
	}
	flags := []byte("iw")
	if cased {
		flags[0] = 'c'
	}
	if !whole {
		flags[1] = 'p'
	}
	return &findPattern{re: regexp.MustCompile(re.String()), like: like.String(), glob: glob.String(), cased: cased,
		exact: exact, spec: string(flags) + "\x00" + text, lits: lits.String()}
}

// cond : la condition SQL « expr correspond », avec ses arguments.
func (p *findPattern) cond(expr string) (string, []any) {
	if p.cased {
		return expr + ` GLOB ?`, []any{p.glob}
	}
	c := expr + ` LIKE ? ESCAPE '\'`
	if p.exact {
		return c, []any{p.like}
	}
	// CASE et non AND : slt_find seulement sur les lignes que LIKE retient.
	return "(CASE WHEN " + c + " THEN slt_find(" + expr + ", ?) ELSE 0 END)", []any{p.like, p.spec}
}

// match : le texte correspond-il ?
func (p *findPattern) match(s string) bool { return s != "" && p.re.MatchString(s) }

// replace : le texte avec ses correspondances remplacées par by.
func (p *findPattern) replace(s, by string) string { return p.re.ReplaceAllLiteralString(s, by) }

var (
	findMu    sync.Mutex
	findCache = map[string]*regexp.Regexp{}
)

// findRegexp : l'expression d'un argument de slt_find (gardée).
func findRegexp(spec string) *regexp.Regexp {
	findMu.Lock()
	defer findMu.Unlock()
	if re, ok := findCache[spec]; ok {
		return re
	}
	flags, text, _ := strings.Cut(spec, "\x00")
	if len(flags) != 2 {
		flags = "iw"
	}
	re := compileFind(text, flags[0] == 'c', flags[1] == 'w').re
	if len(findCache) > 64 {
		findCache = map[string]*regexp.Regexp{}
	}
	findCache[spec] = re
	return re
}

// registerFind déclare slt_find(texte, motif) : vérification exacte d'une
// ligne retenue par LIKE (lettres accentuées sans la casse).
func registerFind(c *sqlite3.Conn) error {
	return c.CreateFunction("slt_find", 2, sqlite3.DETERMINISTIC|sqlite3.INNOCUOUS, func(ctx sqlite3.Context, arg ...sqlite3.Value) {
		if arg[0].Type() == sqlite3.NULL {
			ctx.ResultBool(false)
			return
		}
		ctx.ResultBool(findRegexp(arg[1].Text()).MatchString(arg[0].Text()))
	})
}

// --- Parcours ------------------------------------------------------------------

// hit est une cellule trouvée, avec de quoi la réécrire.
type hit struct {
	Found
	kind    byte // 'h' en-tête, 'd' donnée, 'f' formule de colonne, 'c' cellule libre, 'n' note
	tb      int  // indice du tableau (searcher.tables)
	i       int  // colonne du tableau
	rid     int64
	text    string // texte entier (forme de la session pour une formule)
	formula bool   // cellule libre qui porte une formule
}

// searcher parcourt une feuille.
type searcher struct {
	t      *tx
	p      *findPattern
	q      SearchQuery
	u      ui
	zone   Rect
	tables []Table
	hits   []hit
	total  int
	values map[string]formula.Value // résultats des formules libres (lus à la demande)
}

// findSheets : les feuilles affichées, dans l'ordre des onglets.
func (t *tx) findSheets() ([]int64, []string, error) {
	rs, err := t.query(`SELECT id, name FROM _lattis_sheets WHERE hidden = 0 ORDER BY pos`)
	if err != nil {
		return nil, nil, err
	}
	defer rs.Close()
	var ids []int64
	var names []string
	for rs.Next() {
		var id int64
		var n string
		if err := rs.Scan(&id, &n); err != nil {
			return nil, nil, err
		}
		ids, names = append(ids, id), append(names, n)
	}
	return ids, names, rs.Err()
}

func (s *searcher) add(h hit) {
	h.Found.Text = clipFound(h.text)
	s.hits = append(s.hits, h)
}

// clipFound coupe un texte trouvé pour la liste des résultats.
func clipFound(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	if r := []rune(s); len(r) > 120 {
		return string(r[:119]) + "…"
	}
	return s
}

// run cherche dans la feuille sheet ; lim > 0 : au plus lim cellules par
// source et le compte total, 0 : toutes, sans compte (remplacement).
func (s *searcher) run(sheet int64, lim int) error {
	s.zone = Rect{R0: 0, C0: 0, R1: math.MaxInt32, C1: math.MaxInt32}
	if s.q.In != nil && !s.q.Book {
		s.zone = *s.q.In
	}
	tables, err := s.t.tables(sheet)
	if err != nil {
		return err
	}
	s.tables = tables
	for ti := range s.tables {
		tb := &s.tables[ti]
		if !s.zone.meets(*tb) {
			continue
		}
		if s.q.Look == LookNotes {
			if err := s.tableNotes(sheet, ti, tb); err != nil {
				return err
			}
			continue
		}
		for i, c := range tb.Cols {
			col := tb.Col + i
			if col < s.zone.C0 || col > s.zone.C1 {
				continue
			}
			if s.zone.has(tb.Row, col) && s.p.match(c.Name) {
				s.add(hit{Found: Found{Sheet: sheet, Row: tb.Row, Col: col, Table: tb.Name}, kind: 'h', tb: ti, i: i, text: c.Name})
				s.total++
			}
			if c.Formula != "" && s.q.Look != LookValues {
				row := max(tb.Row+1, s.zone.R0)
				if row > s.zone.R1 || (tb.Rows > 0 && row > tb.Bottom()) || (tb.Rows == 0 && row != tb.Row+1) {
					continue
				}
				if f := s.u.columnOut(c.Formula); s.p.match(f) {
					s.add(hit{Found: Found{Sheet: sheet, Row: row, Col: col, Table: tb.Name}, kind: 'f', tb: ti, i: i, text: f})
					s.total++
				}
			}
		}
		if err := s.tableData(sheet, ti, tb, lim); err != nil {
			return err
		}
	}
	if s.q.Look == LookNotes {
		return s.freeNotes(sheet)
	}
	return s.freeCells(sheet, lim)
}

// dataCol est une colonne de tableau cherchée en SQL.
type dataCol struct {
	i    int
	expr string
	typ  grid.Type
}

// tableData cherche dans les lignes de données d'un tableau (valeurs
// saisies ; dans les valeurs, aussi celles des colonnes calculées).
func (s *searcher) tableData(sheet int64, ti int, tb *Table, lim int) error {
	var cols []dataCol
	computed := false
	for i, c := range tb.Cols {
		if col := tb.Col + i; col < s.zone.C0 || col > s.zone.C1 {
			continue
		}
		id := colSQL(c.ID)
		if c.Formula != "" {
			if s.q.Look != LookValues {
				continue
			}
			typ := s.t.computedResult(*tb, c)
			if typ == "" {
				typ = grid.Text
			}
			cols = append(cols, dataCol{i, `slt_text(` + id + `, '` + strings.ReplaceAll(string(typ), "'", "''") + `')`, typ})
			computed = true
			continue
		}
		if !canHold(c.Type, s.p.lits) {
			continue
		}
		switch c.Type {
		case grid.Text:
			cols = append(cols, dataCol{i, id, c.Type})
		case grid.Integer:
			cols = append(cols, dataCol{i, `CAST(` + id + ` AS TEXT)`, c.Type})
		default:
			cols = append(cols, dataCol{i, `slt_text(` + id + `, '` + strings.ReplaceAll(string(c.Type), "'", "''") + `')`, c.Type})
		}
	}
	first, last := max(s.zone.R0-tb.Row, 1), int64(s.zone.R1)-int64(tb.Row)
	if len(cols) == 0 || int64(first) > last {
		return nil
	}
	base := tb.view()
	if computed {
		src, ok, err := s.t.computedSource(*tb)
		if err != nil || !ok {
			// Valeurs calculées illisibles : seulement les colonnes saisies.
			kept := cols[:0]
			for _, c := range cols {
				if tb.Cols[c.i].Formula == "" {
					kept = append(kept, c)
				}
			}
			cols = kept
			if len(cols) == 0 {
				return nil
			}
		} else {
			w := ""
			if tb.where != "" {
				w = " WHERE " + tb.where
			}
			base = "(SELECT * FROM (" + src + ") AS " + tb.sql() + w + ") AS " + tb.sql()
		}
	}
	// Lignes de la zone : bornées par leur _pos (index), sans numéroter
	// tout le tableau (row_number coûte cinq fois le parcours).
	view := tb.view()
	var posW []string
	var posA []any
	if first > 1 {
		var lo string
		if err := s.t.row(`SELECT _pos FROM `+view+` ORDER BY _pos LIMIT 1 OFFSET ?`, first-1).Scan(&lo); errors.Is(err, sql.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
		posW, posA = append(posW, `_pos >= ?`), append(posA, lo)
	}
	if last < int64(tb.Rows) {
		var hi string
		err := s.t.row(`SELECT _pos FROM `+view+` ORDER BY _pos LIMIT 1 OFFSET ?`, last-1).Scan(&hi)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil {
			posW, posA = append(posW, `_pos <= ?`), append(posA, hi)
		}
	}
	// inner : les lignes et les textes cherchés.
	inner := func(cs []dataCol) string {
		sel := make([]string, len(cs))
		for k, c := range cs {
			sel[k] = fmt.Sprintf("%s AS v%d", c.expr, k)
		}
		return `(SELECT id, _pos, ` + strings.Join(sel, ", ") + ` FROM ` + base + `)`
	}
	conds := func(cs []dataCol) (string, []any) {
		ws := append([]string{}, posW...)
		args := append([]any{}, posA...)
		var or []string
		for k := range cs {
			w, a := s.p.cond(fmt.Sprintf("v%d", k))
			or, args = append(or, w), append(args, a...)
		}
		return strings.Join(append(ws, "("+strings.Join(or, " OR ")+")"), " AND "), args
	}
	// count : le nombre de cellules qui correspondent.
	count := func(cs []dataCol) (int, error) {
		var parts []string
		var args []any
		for k := range cs {
			w, a := s.p.cond(fmt.Sprintf("v%d", k))
			parts, args = append(parts, "total("+w+")"), append(args, a...)
		}
		where := ""
		if len(posW) > 0 {
			where = ` WHERE ` + strings.Join(posW, " AND ")
		}
		var n float64
		err := s.t.row(`SELECT `+strings.Join(parts, " + ")+` FROM `+inner(cs)+where, append(args, posA...)...).Scan(&n)
		return int(n), err
	}
	type dataRow struct {
		rid  int64
		pos  string
		vals []sql.NullString
	}
	// fetch lit les lignes qui correspondent : toutes (rlim 0), ou les rlim
	// premières dans l'ordre des lignes.
	fetch := func(cs []dataCol, rlim int) ([]dataRow, error) {
		w, args := conds(cs)
		vals := make([]string, len(cs))
		for k := range cs {
			vals[k] = fmt.Sprintf("v%d", k)
		}
		q := `SELECT id, _pos, ` + strings.Join(vals, ", ") + ` FROM ` + inner(cs) + ` WHERE ` + w
		if rlim > 0 {
			q += ` ORDER BY _pos LIMIT ?`
			args = append(args, rlim)
		}
		rs, err := s.t.query(q, args...)
		if err != nil {
			return nil, err
		}
		defer rs.Close()
		var out []dataRow
		for rs.Next() {
			r := dataRow{vals: make([]sql.NullString, len(cs))}
			ptrs := []any{&r.rid, &r.pos}
			for k := range r.vals {
				ptrs = append(ptrs, &r.vals[k])
			}
			if err := rs.Scan(ptrs...); err != nil {
				return nil, err
			}
			out = append(out, r)
		}
		return out, rs.Err()
	}
	// rank : le rang affiché des lignes (triées par _pos), en comptant sur
	// l'index les lignes qui séparent chacune de la précédente.
	rank := func(rows []dataRow) ([]int, error) {
		out := make([]int, len(rows))
		acc := first - 1 // lignes avant la zone
		lower := posW[:0:0]
		largs := []any{}
		if first > 1 {
			lower, largs = []string{`_pos >= ?`}, []any{posA[0]}
		}
		for k, r := range rows {
			if k > 0 && r.pos == rows[k-1].pos {
				out[k] = out[k-1]
				continue
			}
			var n int
			w := strings.Join(append(append([]string{}, lower...), `_pos < ?`), " AND ")
			if err := s.t.row(`SELECT count(*) FROM `+view+` WHERE `+w, append(append([]any{}, largs...), r.pos)...).Scan(&n); err != nil {
				return nil, err
			}
			acc += n
			out[k] = acc + 1
			lower, largs = []string{`_pos >= ?`}, []any{r.pos}
		}
		return out, nil
	}
	// scan : cherche dans les colonnes cs, au plus lim lignes (0 : toutes,
	// sans rang ni compte).
	scan := func(cs []dataCol) error {
		total := 0
		rlim := lim
		if lim > 0 {
			n, err := count(cs)
			if err != nil || n == 0 {
				return err
			}
			total = n
			if n <= lim {
				rlim = 0 // toutes : un parcours de la table, sans l'index
			}
		}
		rows, err := fetch(cs, rlim)
		if err != nil {
			return err
		}
		var ranks []int
		if lim > 0 {
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].pos < rows[j].pos })
			if len(rows) > lim {
				rows = rows[:lim]
			}
			if ranks, err = rank(rows); err != nil {
				return err
			}
		}
		found := 0
		for k, r := range rows {
			for j, v := range r.vals {
				if !v.Valid || !s.p.match(v.String) {
					continue
				}
				c := cs[j]
				text := v.String
				if c.typ != grid.Text {
					text = s.u.valueOut(text)
				}
				h := hit{Found: Found{Sheet: sheet, Col: tb.Col + c.i, Table: tb.Name}, kind: 'd', tb: ti, i: c.i, rid: r.rid, text: text}
				if ranks != nil {
					h.Row = tb.Row + ranks[k]
				}
				s.add(h)
				found++
			}
		}
		s.total += max(total, found)
		return nil
	}
	if !s.q.ByCols || lim == 0 {
		return scan(cols)
	}
	for _, c := range cols {
		if err := scan([]dataCol{c}); err != nil {
			return err
		}
	}
	return nil
}

// notFormulaSQL : la saisie d'une cellule libre n'est pas une formule.
const notFormulaSQL = `NOT (ltrim(raw, ' ' || char(9, 10, 13)) LIKE '=_%')`

// freeCells cherche dans les cellules libres.
func (s *searcher) freeCells(sheet int64, lim int) error {
	z := s.zone
	where := `sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`
	zargs := []any{sheet, z.R0, z.R1, z.C0, z.C1}
	// Valeurs saisies : en SQL. Une apostrophe de tête (texte forcé) ne
	// s'affiche pas.
	expr := `raw`
	if s.q.Look == LookValues {
		expr = `CASE WHEN substr(raw, 1, 1) = '''' THEN substr(raw, 2) ELSE raw END`
	}
	w, args := s.p.cond(expr)
	order := ` ORDER BY row, col`
	if s.q.ByCols {
		order = ` ORDER BY col, row`
	}
	limit := lim
	if limit == 0 {
		limit = -1 // toutes
	}
	all := append(append(append([]any{}, zargs...), args...), limit)
	rs, err := s.t.query(`SELECT row, col, `+expr+` FROM _lattis_cells WHERE `+where+` AND `+notFormulaSQL+` AND `+w+order+` LIMIT ?`, all...)
	if err != nil {
		return err
	}
	rows, found := 0, 0
	for rs.Next() {
		var h hit
		if err := rs.Scan(&h.Row, &h.Col, &h.text); err != nil {
			rs.Close()
			return err
		}
		rows++
		if s.p.match(h.text) {
			h.Sheet, h.kind = sheet, 'c'
			s.add(h)
			found++
		}
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	s.total += found
	if lim > 0 && rows >= lim {
		var n int
		if err := s.t.row(`SELECT count(*) FROM _lattis_cells WHERE `+where+` AND `+notFormulaSQL+` AND `+w, append(append([]any{}, zargs...), args...)...).Scan(&n); err != nil {
			return err
		}
		s.total += max(n-found, 0)
	}
	// Formules : la formule (dans l'écriture de la session) ou son résultat.
	rs, err = s.t.query(`SELECT row, col, raw FROM _lattis_cells WHERE `+where+` AND NOT `+notFormulaSQL, zargs...)
	if err != nil {
		return err
	}
	var fs []hit
	for rs.Next() {
		var h hit
		if err := rs.Scan(&h.Row, &h.Col, &h.text); err != nil {
			rs.Close()
			return err
		}
		fs = append(fs, h)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	if len(fs) > 0 && s.q.Look == LookValues && s.values == nil {
		schema, err := s.t.schema()
		if err != nil {
			return err
		}
		if s.values, err = s.t.nameValuesWith(schema, true); err != nil {
			return err
		}
	}
	for _, h := range fs {
		if s.q.Look == LookValues {
			h.text = s.u.valueOut(s.values[formula.CellKey(sheet, h.Row, h.Col)].String())
		} else {
			h.text = s.u.formulaOut(h.text)
		}
		if s.p.match(h.text) {
			h.Sheet, h.kind, h.formula = sheet, 'c', true
			s.add(h)
			s.total++
		}
	}
	return nil
}

// freeNotes cherche dans les notes des cellules libres.
func (s *searcher) freeNotes(sheet int64) error {
	z := s.zone
	rs, err := s.t.query(`SELECT row, col, style FROM _lattis_cell_styles WHERE sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?
		AND style LIKE '%"nt"%'`, sheet, z.R0, z.R1, z.C0, z.C1)
	if err != nil {
		return err
	}
	defer rs.Close()
	for rs.Next() {
		var h hit
		var st string
		if err := rs.Scan(&h.Row, &h.Col, &st); err != nil {
			return err
		}
		if h.text, _ = parseStyle(st)["nt"].(string); s.p.match(h.text) {
			h.Sheet, h.kind = sheet, 'n'
			s.add(h)
			s.total++
		}
	}
	return rs.Err()
}

// tableNotes cherche dans les notes d'un tableau (en-têtes et cellules,
// qui suivent leur ligne).
func (s *searcher) tableNotes(sheet int64, ti int, tb *Table) error {
	rs, err := s.t.query(`SELECT rid, col, style FROM _lattis_styles WHERE tbl = ? AND rid <> 0 AND style LIKE '%"nt"%'`, tb.ID)
	if err != nil {
		return err
	}
	at := map[int64]int{}
	for i, c := range tb.Cols {
		at[c.ID] = i
	}
	var cells []hit
	for rs.Next() {
		var h hit
		var colID int64
		var st string
		if err := rs.Scan(&h.rid, &colID, &st); err != nil {
			rs.Close()
			return err
		}
		i, ok := at[colID]
		if h.text, _ = parseStyle(st)["nt"].(string); !ok || !s.p.match(h.text) {
			continue
		}
		h.Sheet, h.Table, h.kind, h.tb, h.i, h.Col = sheet, tb.Name, 'n', ti, i, tb.Col+i
		cells = append(cells, h)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	// Rang affiché des lignes (absentes : masquées par un filtre).
	var rids []any
	for _, h := range cells {
		if h.rid > 0 {
			rids = append(rids, h.rid)
		}
	}
	rank := map[int64]int{}
	for k := 0; k < len(rids); k += 500 {
		part := rids[k:min(k+500, len(rids))]
		rs, err := s.t.query(`SELECT id, n FROM (SELECT id, row_number() OVER (ORDER BY _pos) AS n FROM `+tb.view()+`)
			WHERE id IN (`+strings.TrimSuffix(strings.Repeat("?, ", len(part)), ", ")+`)`, part...)
		if err != nil {
			return err
		}
		for rs.Next() {
			var id int64
			var n int
			if err := rs.Scan(&id, &n); err != nil {
				rs.Close()
				return err
			}
			rank[id] = n
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return err
		}
	}
	for _, h := range cells {
		if h.rid < 0 {
			h.Row = tb.Row
		} else if n, ok := rank[h.rid]; ok {
			h.Row = tb.Row + n
		} else {
			continue
		}
		if s.zone.has(h.Row, h.Col) {
			s.add(h)
			s.total++
		}
	}
	return nil
}

// sortHits range les cellules trouvées par lignes ou par colonnes.
func sortHits(hs []hit, byCols bool) {
	sort.SliceStable(hs, func(i, j int) bool {
		a, b := hs[i].Found, hs[j].Found
		if byCols {
			a.Row, a.Col, b.Row, b.Col = a.Col, a.Row, b.Col, b.Row
		}
		if a.Row != b.Row {
			return a.Row < b.Row
		}
		return a.Col < b.Col
	})
}

func normLook(look string) string {
	switch look {
	case LookValues, LookNotes:
		return look
	}
	return LookFormulas
}

// Search cherche dans une feuille (Rechercher tout, Suivant, Précédent) ; avec
// Book, Sheet 0 désigne la première feuille et Next la suivante.
func (m *Model) Search(ctx context.Context, q SearchQuery) (SearchResult, error) {
	res := SearchResult{Sheet: q.Sheet, Found: []Found{}}
	q.Look = normLook(q.Look)
	err := m.read(ctx, func(t *tx) error {
		ids, names, err := t.findSheets()
		if err != nil {
			return err
		}
		if q.Book {
			k := slicesIndex(ids, q.Sheet)
			if q.Sheet == 0 {
				k = 0
			}
			if k < 0 || k >= len(ids) {
				return nil
			}
			res.Sheet, res.Name = ids[k], names[k]
			if k+1 < len(ids) {
				res.Next = ids[k+1]
			}
		} else {
			if k := slicesIndex(ids, q.Sheet); k >= 0 {
				res.Name = names[k]
			}
		}
		if q.Text == "" {
			return nil
		}
		s := &searcher{t: t, p: compileFind(q.Text, q.Case, q.Whole), q: q, u: m.ui(ctx)}
		if err := s.run(res.Sheet, MaxMatches); err != nil {
			return err
		}
		sortHits(s.hits, q.ByCols)
		if len(s.hits) > MaxMatches {
			s.hits = s.hits[:MaxMatches]
		}
		for _, h := range s.hits {
			res.Found = append(res.Found, h.Found)
		}
		res.Total = max(s.total, len(res.Found))
		return nil
	})
	return res, err
}

func slicesIndex(ids []int64, id int64) int {
	for k, v := range ids {
		if v == id {
			return k
		}
	}
	return -1
}

// ReplaceAll remplace le texte cherché par by dans toutes les cellules
// trouvées (la feuille, la sélection ou tout le classeur), en une étape
// d'annulation. Dans les valeurs, comme un tableur, ce sont les formules
// qui sont réécrites. Remplacer une seule cellule : In sur cette cellule.
func (m *Model) ReplaceAll(ctx context.Context, q SearchQuery, by string) (ReplaceResult, error) {
	var res ReplaceResult
	if q.Text == "" {
		return res, nil
	}
	q.Look = normLook(q.Look)
	if q.Look == LookValues {
		q.Look = LookFormulas
	}
	p := compileFind(q.Text, q.Case, q.Whole)
	u := m.ui(ctx)
	lang := m.Language()
	err := m.Do(ctx, "Remplacer", func(t *tx) error {
		res = ReplaceResult{}
		sheets := []int64{q.Sheet}
		if q.Book {
			var err error
			if sheets, _, err = t.findSheets(); err != nil {
				return err
			}
		}
		skip := func(err error) {
			res.Skipped++
			if res.Reason == "" {
				res.Reason = msg.Text(lang, err)
			}
		}
		for _, sheet := range sheets {
			s := &searcher{t: t, p: p, q: q, u: u}
			if err := s.run(sheet, 0); err != nil {
				return err
			}
			if err := s.rewrite(sheet, by, skip, &res); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return ReplaceResult{}, err
	}
	return res, nil
}

// rewrite réécrit les cellules trouvées d'une feuille.
func (s *searcher) rewrite(sheet int64, by string, skip func(error), res *ReplaceResult) error {
	t := s.t
	// Gardes, une fois : la feuille (cellules libres, en-têtes, formules,
	// notes), les données de chaque tableau, ses colonnes chargées par une
	// requête.
	var sheetErr error
	sheetDone := false
	guardSheet := func() error {
		if !sheetDone {
			sheetErr, sheetDone = t.guard(sheet), true
		}
		return sheetErr
	}
	dataErr := map[int]error{}
	handErr := map[[2]int]error{}
	tableLock := func(ti int) error { return t.lockTable(s.tables[ti].ID) }
	var ev Event
	for _, h := range s.hits {
		neu := s.p.replace(h.text, by)
		if neu == h.text {
			continue
		}
		switch h.kind {
		case 'd':
			tb := &s.tables[h.tb]
			err, ok := dataErr[h.tb]
			if !ok {
				err = t.guardCells(sheet, Rect{tb.Row + 1, tb.Col, tb.Row + 1, tb.Col}, false)
				if err == nil {
					err = tableLock(h.tb)
				}
				dataErr[h.tb] = err
			}
			if err != nil {
				skip(err)
				continue
			}
			k := [2]int{h.tb, h.i}
			if err, ok = handErr[k]; !ok {
				err = t.handInput(*tb, h.i, neu)
				handErr[k] = err
			}
			if err != nil {
				skip(err)
				continue
			}
			// Un texte qui commence par = reste un texte (il ferait de la
			// colonne une colonne calculée).
			if isFormula(neu) {
				neu = "'" + neu
			}
			nt, err := t.setCellAt(*tb, h.rid, h.i, neu, &ev)
			if err != nil {
				return err
			}
			*tb = nt
		case 'h':
			tb := &s.tables[h.tb]
			err := guardSheet()
			if err == nil {
				err = tableLock(h.tb)
			}
			if err == nil {
				if name := strings.TrimSpace(neu); name != "" && columnNameTaken(*tb, tb.Cols[h.i].ID, name) {
					err = errColumnTaken(*tb, name)
				}
			}
			if err != nil {
				skip(err)
				continue
			}
			if err := t.input(sheet, h.Row, h.Col, neu, &ev); err != nil {
				return err
			}
			nt, err := t.table(tb.ID)
			if err != nil {
				return err
			}
			*tb = nt
		case 'f':
			tb := &s.tables[h.tb]
			err := guardSheet()
			if err == nil {
				err = tableLock(h.tb)
			}
			if err == nil && !isFormula(neu) {
				err = ErrComputed{tb.Cols[h.i].Name}
			}
			fr := neu
			if err == nil {
				fr, err = s.u.formulaIn(neu)
			}
			if err == nil {
				err = t.setFormula(*tb, h.i, strings.TrimSpace(fr))
				var fe ErrFormula
				if err != nil && !errors.As(err, &fe) {
					return err
				}
			}
			if err != nil {
				skip(err)
				continue
			}
			nt, err := t.table(tb.ID)
			if err != nil {
				return err
			}
			*tb = nt
		case 'c':
			if err := guardSheet(); err != nil {
				skip(err)
				continue
			}
			raw := freeRaw(neu)
			if isFormula(neu) {
				fr, err := s.u.formulaIn(strings.TrimSpace(neu))
				var n formula.Node
				if err == nil {
					n, err = t.prepareFormula(sheet, strings.TrimSpace(fr))
				}
				if err != nil {
					var fe ErrFormula
					if !errors.As(err, &fe) && !msgError(err) {
						return err
					}
					skip(err)
					continue
				}
				raw = formula.Format(n, formula.FR)
			}
			if err := t.setFreeCell(sheet, h.Row, h.Col, raw); err != nil {
				return err
			}
		case 'n':
			if err := guardSheet(); err != nil {
				skip(err)
				continue
			}
			text := strings.TrimRight(neu, " \t\r\n")
			patch := Style{"nt": text}
			if text == "" {
				patch["np"] = nil
			}
			var err error
			if h.Table == "" {
				err = t.styleFree(sheet, h.Row, h.Col, patch, nil)
			} else {
				tb := s.tables[h.tb]
				err = t.styleTable(tb.ID, h.rid, tb.Cols[h.i].ID, patch, nil)
			}
			if err != nil {
				return err
			}
		}
		res.Replaced++
	}
	return nil
}

// msgError : une erreur expliquée à l'utilisateur (refus, pas une panne).
func msgError(err error) bool {
	var m msg.Messager
	return errors.As(err, &m)
}
