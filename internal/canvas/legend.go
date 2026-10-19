package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// Légende d'un tableau : ce qui l'accompagne sans en faire partie.
//
// Un titre au-dessus de l'en-tête reste libre (exception de touching). Une
// note sous le tableau (« Source : INSEE », « * hors taxes », une phrase
// sous une colonne de nombres), une ligne de total (« Total », une formule
// tapée sous le tableau) ou une remarque à côté d'une ligne (« ← à
// vérifier ») restent aussi des cellules libres : elles sont marquées « hors
// tableau » (propriété out de leur style) et ne comptent pas pour la règle
// « jamais jointifs ». Elles suivent le tableau quand il grandit, restent
// collées à lui, et s'affichent en gris italique (note) ou en gras (total).
// Clic droit « Ajouter au tableau » retire la marque : la cellule rejoint
// alors le tableau comme avant.

// outKey marque une cellule libre gardée hors du tableau qu'elle touche.
const outKey = "out"

// Allure d'une cellule gardée hors du tableau.
const (
	outNote  = "note"
	outTotal = "total"
)

var (
	// noteRe : début d'une note ou d'une remarque.
	noteRe = regexp.MustCompile(`(?i)^\s*(?:\*|\(\w{1,3}\)|[←↑→↓⚠]|(?:sources?|notes?|nb|n\.b\.|remarques?|attention)(?:\s|:|$))`)
	// totalRe : libellé d'une ligne de total.
	totalRe = regexp.MustCompile(`(?i)^\s*(?:sous[- ]?)?(?:total|totaux|somme|moyenne)(?:\s|:|$)`)
	// autoColRe : nom de colonne donné par Lattis, pas par l'utilisateur.
	autoColRe = regexp.MustCompile(`^Colonne \d+$`)
)

// totalLabel : libellé d'une ligne de total. « Total », « Sous-total : »,
// « Total général », « Total HT » le sont ; un nom propre après le mot
// (« Total Energies », « Moyenne Garonne ») est une donnée (audit 2 M5).
func totalLabel(s string) bool {
	loc := totalRe.FindStringIndex(s)
	if loc == nil {
		return false
	}
	rest := strings.TrimSpace(s[loc[1]:])
	if rest == "" || strings.HasSuffix(s[:loc[1]], ":") || strings.HasPrefix(rest, ":") {
		return true
	}
	word, _, _ := strings.Cut(rest, " ")
	runes := []rune(word)
	if !unicode.IsUpper(runes[0]) || strings.ToUpper(word) == word {
		return true // « total des ventes », « Total HT »
	}
	switch strings.ToLower(word) {
	case "général", "générale", "global", "globale", "annuel", "annuelle", "mensuel", "mensuelle", "cumulé", "cumulée":
		return true
	}
	return false
}

func outStyle(kind string) Style {
	s := Style{outKey: true}
	switch kind {
	case outNote:
		s["i"], s["fg"] = true, "#6b7280"
	case outTotal:
		s["b"] = true
	}
	return s
}

// markOut garde la cellule libre (row, col) hors des tableaux.
func (t *tx) markOut(sheet int64, row, col int, kind string) error {
	return t.styleFree(sheet, row, col, outStyle(kind), nil)
}

// clearOut retire la marque « hors tableau » (et l'allure de note ou de
// total qu'elle avait donnée).
func (t *tx) clearOut(sheet int64, row, col int) error {
	return t.setStyle(`_lattis_cell_styles`, `sheet = ? AND row = ? AND col = ?`, `sheet, row, col`, []any{sheet, row, col},
		func(old string) (string, error) {
			s := parseStyle(old)
			if s[outKey] != true {
				return old, nil
			}
			delete(s, outKey)
			switch {
			case s["fg"] == "#6b7280" && s["i"] == true:
				delete(s, "fg")
				delete(s, "i")
			case s["b"] == true:
				delete(s, "b")
			}
			return encodeStyle(s)
		})
}

// clearOutIn retire la marque « hors tableau » des cellules libres d'une
// zone (Suppr : effacer la cellule retire la marque, audit 2 I4).
func (t *tx) clearOutIn(sheet int64, r0, c0, r1, c1 int) error {
	rs, err := t.query(`SELECT row, col FROM _lattis_cell_styles WHERE sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ? AND style LIKE '%"out":true%'`, sheet, r0, r1, c0, c1)
	if err != nil {
		return err
	}
	var ps [][2]int
	for rs.Next() {
		var p [2]int
		if err := rs.Scan(&p[0], &p[1]); err != nil {
			rs.Close()
			return err
		}
		ps = append(ps, p)
	}
	rs.Close()
	for _, p := range ps {
		if err := t.clearOut(sheet, p[0], p[1]); err != nil {
			return err
		}
	}
	return rs.Err()
}

// noteAbove : la cellule libre au-dessus de (row, col) est une note gardée
// hors d'un tableau.
func (t *tx) noteAbove(sheet int64, row, col int) (bool, error) {
	if row == 0 {
		return false, nil
	}
	var style string
	err := t.row(`SELECT style FROM _lattis_cell_styles WHERE sheet = ? AND row = ? AND col = ?`, sheet, row-1, col).Scan(&style)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	s := parseStyle(style)
	return s[outKey] == true && s["i"] == true, err
}

// outCells donne les cellules libres marquées « hors tableau » d'une feuille.
func (t *tx) outCells(sheet int64) (map[[2]int]bool, error) {
	rs, err := t.query(`SELECT row, col, style FROM _lattis_cell_styles WHERE sheet = ? AND style LIKE '%"out":true%'`, sheet)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := map[[2]int]bool{}
	for rs.Next() {
		var r, c int
		var s string
		if err := rs.Scan(&r, &c, &s); err != nil {
			return nil, err
		}
		if parseStyle(s)[outKey] == true {
			out[[2]int{r, c}] = true
		}
	}
	return out, rs.Err()
}

// ErrQueryTable : un tableau chargé par une requête ne s'agrandit pas à la
// main (ni ligne ni colonne) : l'actualisation le remplacerait. Même texte
// que le refus de l'interface (QUERY_READONLY, Sheet.svelte).
var ErrQueryTable = errors.New("Tableau chargé par une requête : ses cellules ne se modifient pas ; changez la requête (Données › Requêtes) puis actualisez (F5)")

// queryLoaded : le tableau id est chargé par une requête d'import.
func (t *tx) queryLoaded(id int64) (bool, error) {
	var n int
	err := t.row(`SELECT count(*) FROM _lattis_queries WHERE target = ?`, id).Scan(&n)
	return n > 0, err
}

// refuseQuery refuse l'agrandissement d'un tableau chargé par une requête.
func (t *tx) refuseQuery(tb Table) error {
	q, err := t.queryLoaded(tb.ID)
	if err == nil && q {
		err = ErrQueryTable
	}
	return err
}

// outKindInRow donne l'allure (note, total) d'une cellule déjà gardée hors
// du tableau sur la ligne row, dans ses colonnes ; "" s'il n'y en a pas.
func (t *tx) outKindInRow(tb Table, row int) (string, error) {
	rs, err := t.query(`SELECT style FROM _lattis_cell_styles WHERE sheet = ? AND row = ? AND col BETWEEN ? AND ? ORDER BY col`, tb.Sheet, row, tb.Col, tb.Right())
	if err != nil {
		return "", err
	}
	defer rs.Close()
	for rs.Next() {
		var style string
		if err := rs.Scan(&style); err != nil {
			return "", err
		}
		if s := parseStyle(style); s[outKey] == true {
			if s["b"] == true {
				return outTotal, nil
			}
			return outNote, nil
		}
	}
	return "", rs.Err()
}

// totalLeft : sur la ligne row, une cellule à gauche de col, dans les
// colonnes du tableau, est gardée à part comme total.
func (t *tx) totalLeft(tb Table, row, col int) (bool, error) {
	rs, err := t.query(`SELECT style FROM _lattis_cell_styles WHERE sheet = ? AND row = ? AND col BETWEEN ? AND ?`, tb.Sheet, row, tb.Col, col-1)
	if err != nil {
		return false, err
	}
	defer rs.Close()
	for rs.Next() {
		var style string
		if err := rs.Scan(&style); err != nil {
			return false, err
		}
		if s := parseStyle(style); s[outKey] == true && s["b"] == true {
			return true, nil
		}
	}
	return false, rs.Err()
}

// footKind dit si une saisie dans la ligne juste sous le tableau est une
// note ou un total plutôt qu'une nouvelle ligne (vide : une ligne) ; why
// l'explique.
func (t *tx) footKind(tb Table, row, col int, raw string) (kind, why string, err error) {
	cells, err := t.cells(tb.Sheet)
	if err != nil {
		return "", "", err
	}
	outs, err := t.outCells(tb.Sheet)
	if err != nil {
		return "", "", err
	}
	for _, c := range cells {
		if c.Row != row || c.Col == col || c.Col < tb.Col || c.Col > tb.Right() || strings.TrimSpace(c.Raw) == "" {
			continue
		}
		if !outs[[2]int{c.Row, c.Col}] {
			return "", "", nil // une ligne de données en cours de saisie
		}
	}
	// À droite d'un libellé de total gardé à part (« Total » puis 245) : la
	// valeur de cette ligne de total, pas une ligne de données.
	if total, err := t.totalLeft(tb, row, col); err != nil || total {
		return outTotal, "valeur de la ligne de total", err
	}
	src := strings.TrimSpace(raw)
	switch {
	case isFormula(src):
		return outTotal, "une formule sous le tableau reste à part (ligne de total)", nil
	case totalLabel(src):
		// Une colonne qui a déjà des valeurs de cette forme (« Total
		// Access ») : une donnée de plus.
		if c := tb.Cols[col-tb.Col]; c.Formula == "" && (c.Type == grid.Text || c.Type == "") {
			word, _, _ := strings.Cut(strings.TrimSpace(src), " ")
			var same int
			if err := t.row(`SELECT EXISTS (SELECT 1 FROM `+tb.sql()+` WHERE lower(`+colSQL(c.ID)+`) LIKE lower(?) || '%')`, word).Scan(&same); err != nil {
				return "", "", err
			}
			if same == 1 {
				return "", "", nil
			}
		}
		return outTotal, "ligne de total", nil
	case strings.HasPrefix(src, "'"):
		return "", "", nil
	case noteRe.MatchString(src):
		return outNote, "note sous le tableau", nil
	}
	c := tb.Cols[col-tb.Col]
	typed := c.Formula == "" && c.Type != grid.Text && c.Type != ""
	text := Infer(src) == grid.Text && !isTimeOrPercent(src)
	// Une ligne réduite à un texte sous une colonne typée d'un tableau de
	// plusieurs colonnes : une note (dans une liste d'une colonne, la
	// colonne s'élargit, comme avant).
	if typed && text && len(tb.Cols) > 1 {
		var any int
		if err := t.row(`SELECT EXISTS (SELECT 1 FROM ` + tb.sql() + ` WHERE ` + colSQL(c.ID) + ` IS NOT NULL)`).Scan(&any); err != nil {
			return "", "", err
		}
		if any == 1 {
			return outNote, fmt.Sprintf("un texte sous la colonne « %s » ne la change pas en texte", c.Name), nil
		}
	}
	if n := utf8.RuneCountInString(src); text && n >= 25 && strings.Contains(src, " ") && c.Type == grid.Text {
		var longest int
		if err := t.row(`SELECT coalesce(max(length(` + colSQL(c.ID) + `)), 0) FROM ` + tb.sql()).Scan(&longest); err != nil {
			return "", "", err
		}
		if n > 2*longest {
			return outNote, "phrase sous le tableau", nil
		}
	}
	return "", "", nil
}

// keepOut pose une saisie hors du tableau tb, marquée kind, et le dit.
func (t *tx) keepOut(tb Table, row, col int, raw, kind, why string) error {
	if err := t.putFreeIn(tb.Sheet, row, col, raw, tb.Name); err != nil {
		return err
	}
	if err := t.markOut(tb.Sheet, row, col, kind); err != nil {
		return err
	}
	what := "note"
	if kind == outTotal {
		what = "total"
	}
	t.notes = append(t.notes, fmt.Sprintf("« %s » gardé comme %s hors du tableau « %s » : %s (clic droit : Ajouter au tableau)", clip(raw), what, tb.Name, why))
	return nil
}

// putFree pose une saisie dans une cellule libre, comme la saisie au
// clavier mais sans détection d'îlot.
func (t *tx) putFree(sheet int64, row, col int, raw string) error {
	return t.putFreeIn(sheet, row, col, raw, "")
}

// putFreeIn est putFree pour une formule tapée contre le tableau home : ses
// [Colonne] citent ce tableau (SOMME([Montant]) → SOMME(Ventes[Montant])).
func (t *tx) putFreeIn(sheet int64, row, col int, raw, home string) error {
	if isFormula(raw) {
		n, err := formula.Parse(strings.TrimSpace(raw), formula.FR)
		if err != nil {
			return ErrFormula{err.Error()}
		}
		if home != "" {
			n = qualifyRefs(n, home)
		}
		if n, err = t.prepareFormula(sheet, formula.Format(n, formula.FR)); err != nil {
			return err
		}
		return t.setFreeCell(sheet, row, col, formula.Format(n, formula.FR))
	}
	return t.setFreeCell(sheet, row, col, freeRaw(raw))
}

// qualifyRefs donne le tableau home aux références [Colonne] sans tableau.
func qualifyRefs(n formula.Node, home string) formula.Node {
	switch v := n.(type) {
	case formula.Ref:
		if v.Table == "" {
			v.Table, v.Row = home, false // hors du tableau : toute la colonne
		}
		return v
	case formula.RowRange:
		v.Ref = qualifyRefs(v.Ref, home).(formula.Ref)
		return v
	case formula.Unary:
		v.X = qualifyRefs(v.X, home)
		return v
	case formula.Binary:
		v.L = qualifyRefs(v.L, home)
		v.R = qualifyRefs(v.R, home)
		return v
	case formula.Call:
		args := make([]formula.Node, len(v.Args))
		for i, a := range v.Args {
			args[i] = qualifyRefs(a, home)
		}
		v.Args = args
		return v
	}
	return n
}

// titleRows compte les premières lignes d'un îlot qui sont des titres et
// non l'en-tête : des lignes d'une seule cellule (« Ventes 2026 », « Source :
// interne ») ou clairsemées (2025, 2026 au-dessus de T1 T2 T1 T2), au-dessus
// d'une ligne d'en-têtes (au moins deux textes, rien d'autre, plus remplie
// que chacune d'elles).
func titleRows(g [][]string) int {
	most := 0
	for k, r := range g {
		n := filledCount(r)
		if k > 0 && n >= 2 && n > most && allText(r) {
			return k
		}
		if n != 1 && n*2 >= len(r) {
			return 0
		}
		most = max(most, n)
	}
	return 0
}

func filledCount(r []string) int {
	n := 0
	for _, v := range r {
		if strings.TrimSpace(v) != "" {
			n++
		}
	}
	return n
}

// allText : toutes les cellules remplies de la ligne sont des textes.
func allText(r []string) bool {
	for _, v := range r {
		if v = strings.TrimSpace(v); v != "" && (Infer(v) != grid.Text || isTimeOrPercent(v) || isFormula(v)) {
			return false
		}
	}
	return true
}

// footRows compte les dernières lignes d'un bloc collé qui accompagnent le
// tableau sans en faire partie : total (« Total » en tête de ligne) ou note
// (une seule cellule, qui commence comme une note).
func footRows(g [][]string) (n int, kinds []string) {
	for i := len(g) - 1; i > 0; i-- {
		first := ""
		for _, v := range g[i] {
			if v = strings.TrimSpace(v); v != "" {
				first = v
				break
			}
		}
		switch {
		case totalLabel(first):
			kinds = append(kinds, outTotal)
		case filledCount(g[i]) == 1 && noteRe.MatchString(first):
			kinds = append(kinds, outNote)
		default:
			return n, kinds
		}
		n++
	}
	return n, kinds
}

// reshapeYoung : un tableau tout juste né d'un titre (« Ventes 2026 » tapé
// au-dessus, puis « Produit » dessous) a pris le titre pour en-tête. Dès que
// sa dernière ligne de données se lit comme une ligne d'en-têtes (plus de
// textes que l'en-tête n'a de vrais noms, les lignes d'avant n'ayant qu'une
// cellule), il redevient des cellules libres et l'îlot est relu : les titres
// restent au-dessus.
func (t *tx) reshapeYoung(id int64) error {
	tb, err := t.table(id)
	if err != nil || tb.Rows < 1 || tb.Rows > 4 || tb.Filtered() {
		return err
	}
	var names []string
	real := 0
	for _, c := range tb.Cols {
		name := c.Name
		if c.Formula != "" {
			return nil
		}
		if autoColRe.MatchString(name) {
			name = ""
		} else {
			real++
		}
		names = append(names, name)
	}
	if real != 1 {
		return nil
	}
	rows := make([][]string, tb.Rows)
	for n := range rows {
		rows[n] = make([]string, len(tb.Cols))
		for i := range tb.Cols {
			if rows[n][i], _, err = t.rawAt(tb.Sheet, tb.Row+1+n, tb.Col+i); err != nil {
				return err
			}
		}
		if n < tb.Rows-1 && filledCount(rows[n]) != 1 {
			return nil
		}
	}
	if last := rows[tb.Rows-1]; filledCount(last) <= real || !allText(last) {
		return nil
	}
	if err := t.deleteTable(tb); err != nil {
		return err
	}
	for i := range tb.Cols {
		if err := t.setFreeCell(tb.Sheet, tb.Row, tb.Col+i, names[i]); err != nil {
			return err
		}
		for n := range rows {
			if err := t.setFreeCell(tb.Sheet, tb.Row+1+n, tb.Col+i, rows[n][i]); err != nil {
				return err
			}
		}
	}
	_, err = t.birth(tb.Sheet, tb.Bottom(), tb.Col)
	return err
}

// --- Commandes ---------------------------------------------------------------

// HeaderToTitle : « Sortir l'en-tête du tableau (c'est un titre) », l'inverse
// d'« Utiliser la 1re ligne comme en-têtes » : les noms de l'en-tête (sauf
// « Colonne n ») deviennent des cellules libres au même endroit, la 1re ligne
// de données devient l'en-tête.
func (m *Model) HeaderToTitle(ctx context.Context, table string) error {
	return m.Do(ctx, "Sortir l'en-tête du tableau", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		if tb.Rows < 1 || tb.Filtered() {
			return nil
		}
		var titles []string
		named := false
		for _, c := range tb.Cols {
			name := c.Name
			if autoColRe.MatchString(name) {
				name = ""
			}
			named = named || name != ""
			titles = append(titles, name)
		}
		if err := t.promote(tb); err != nil {
			return err
		}
		// Rien que des « Colonne n » : pas de titre, le tableau reste en place.
		if !named {
			return nil
		}
		if tb, err = t.table(tb.ID); err != nil {
			return err
		}
		if err := t.moveTable(tb, 1, 0); err != nil {
			return err
		}
		for i, s := range titles {
			if err := t.setFreeCell(tb.Sheet, tb.Row, tb.Col+i, textRaw(s)); err != nil {
				return err
			}
		}
		return nil
	})
}

// ErrNotLastRow : seule la dernière ligne d'un tableau peut en sortir.
var ErrNotLastRow = errors.New("seule la dernière ligne du tableau peut en sortir (déplacez-la en bas d'abord)")

// RowToNote : « Sortir cette ligne du tableau (total ou note) » ; la
// dernière ligne de données devient des cellules libres juste dessous,
// gardées hors du tableau (une note, ou un total si elle commence par
// « Total »). Sur un tableau chargé par une requête aussi : la barre d'état
// prévient que l'actualisation la remettra dans le tableau (la ligne gardée
// à part descend alors d'une ligne, rien n'est écrasé : LoadQuery).
func (m *Model) RowToNote(ctx context.Context, table string, n int) error {
	return m.Do(ctx, "Sortir la ligne du tableau", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		if tb.Filtered() {
			return ErrFiltered{Table: tb.Name}
		}
		if n != tb.Rows-1 || n < 0 {
			return ErrNotLastRow
		}
		row := tb.Row + 1 + n
		vals := make([]string, len(tb.Cols))
		for i, c := range tb.Cols {
			if c.Formula != "" {
				continue
			}
			if vals[i], _, err = t.rawAt(tb.Sheet, row, tb.Col+i); err != nil {
				return err
			}
		}
		rid, _, err := t.rowID(tb, n)
		if err != nil {
			return err
		}
		if err := t.deleteRow(tb, rid); err != nil {
			return err
		}
		kind := outNote
		for _, v := range vals {
			if v = strings.TrimSpace(v); v != "" {
				if totalLabel(v) {
					kind = outTotal
				}
				break
			}
		}
		for i, v := range vals {
			if strings.TrimSpace(v) == "" {
				continue
			}
			// Un nombre (une date…) reste un nombre : un total sorti du
			// tableau se calcule encore. Un texte reste un texte.
			if tb.Cols[i].Type == grid.Text || tb.Cols[i].Type == "" {
				v = textRaw(v)
			}
			if err := t.setFreeCell(tb.Sheet, row, tb.Col+i, v); err != nil {
				return err
			}
			if err := t.markOut(tb.Sheet, row, tb.Col+i, kind); err != nil {
				return err
			}
		}
		var qname string
		switch err := t.row(`SELECT name FROM _lattis_queries WHERE target = ?`, tb.ID).Scan(&qname); {
		case err == nil:
			t.notes = append(t.notes, fmt.Sprintf("Ligne sortie du tableau « %s » ; l'actualisation de la requête « %s » l'y remettra (la ligne gardée à part descendra d'une ligne) : filtrez-la dans la requête pour l'en retirer pour de bon", tb.Name, qname))
		case !errors.Is(err, sql.ErrNoRows):
			return err
		}
		return nil
	})
}

// JoinTable : « Ajouter au tableau » ; la cellule libre (row, col) perd sa
// marque « hors tableau » et rejoint le tableau qu'elle touche.
func (m *Model) JoinTable(ctx context.Context, sheet int64, row, col int) error {
	return m.Do(ctx, "Ajouter au tableau", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		outs, err := t.outCells(sheet)
		if err != nil {
			return err
		}
		// Les cellules gardées à part contiguës sur la ligne (« Total »,
		// 245) la rejoignent avec elle.
		lo, hi := col, col
		for outs[[2]int{row, lo - 1}] {
			lo--
		}
		for outs[[2]int{row, hi + 1}] {
			hi++
		}
		for c := lo; c <= hi; c++ {
			if err := t.clearOut(sheet, row, c); err != nil {
				return err
			}
		}
		t.noLegend = true
		if t.layoutSheets == nil {
			t.layoutSheets, t.layoutTables = map[int64]bool{}, map[int64]bool{}
		}
		t.layout, t.layoutSheets[sheet] = true, true
		return nil
	})
}
