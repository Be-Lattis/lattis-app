package canvas

import (
	"context"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/msg"
)

// En-têtes sur deux lignes (demande de Louis) : beaucoup de classeurs ont une
// ligne de groupes fusionnés au-dessus des en-têtes (BOISEMENT sur Nom |
// Statut | Parcelle). Le nom d'une colonne reste son seul nom, « Groupe.Sous »
// (le premier point sépare), ce qui donne T[BOISEMENT.Nom] dans les formules
// et en SQL sans rien de plus. Le drapeau du tableau (two_level) ne change
// que l'affichage : le groupe dans une bande juste au-dessus de l'en-tête
// (Row-1), le reste dans l'en-tête. La bande fait partie du tableau : aucune
// cellule libre n'y vit, un titre se pose au-dessus d'elle.

// splitGroup sépare le groupe (avant le premier point) du sous-en-tête ;
// groupe vide sans point, ou quand une des deux parties serait vide.
func splitGroup(name string) (group, sub string) {
	if i := strings.Index(name, "."); i > 0 && i < len(name)-1 {
		return name[:i], name[i+1:]
	}
	return "", name
}

// cleanGroup : un groupe tient sur une ligne et sans point (le point sépare
// le groupe du sous-en-tête).
func cleanGroup(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(strings.ReplaceAll(s, ".", ""))
}

// SetTwoLevel affiche (ou non) les en-têtes du tableau sur deux lignes. À
// l'activation, les cellules libres de la ligne au-dessus de l'en-tête
// (fusionnées ou non) deviennent les groupes des colonnes qu'elles
// couvrent ; un tableau en première ligne descend d'une ligne pour faire
// place à la bande.
func (m *Model) SetTwoLevel(ctx context.Context, table string, on bool) error {
	label := "Groupes d'en-têtes"
	if !on {
		label = "Retirer les groupes d'en-têtes"
	}
	return m.Do(ctx, label, func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		return t.setTwoLevel(tb, on)
	})
}

func (t *tx) setTwoLevel(tb Table, on bool) error {
	if tb.TwoLevel == on {
		return nil
	}
	q := `UPDATE _lattis_tables SET two_level = ? WHERE id = ?`
	flag := func() error {
		return t.apply([]stmt{st(q, on, tb.ID)}, []stmt{st(q, tb.TwoLevel, tb.ID)})
	}
	if !on {
		return flag()
	}
	if tb.Row == 0 {
		// La bande doit rester sur la feuille : le tableau descend.
		if err := t.ensureGapBelow(tb.Sheet, tb.ID, tb.Bottom(), tb.Bottom()+1, tb.Col, tb.Right()); err != nil {
			return err
		}
		if err := t.moveTable(tb, 1, 0); err != nil {
			return err
		}
		t.notes = append(t.notes, t.say("note.tableMovedForGroups", msg.Args{"table": tb.Name}))
		return flag()
	}
	groups, err := t.groupsAbove(tb)
	if err != nil {
		return err
	}
	if err := flag(); err != nil {
		return err
	}
	n, single := 0, 0
	for i, g := range groups {
		if g == "" {
			continue
		}
		c := tb.Cols[i]
		if cur, _ := splitGroup(c.Name); strings.EqualFold(cur, g) {
			continue
		}
		alone := (i == 0 || groups[i-1] != g) && (i == len(groups)-1 || groups[i+1] != g)
		name := groupedName(g, c.Name, alone)
		if name == c.Name {
			single++
			continue
		}
		name = freeColumnNameExcept(tb, c.ID, name)
		if err := t.rename(tb, c, name); err != nil {
			return err
		}
		tb.Cols[i].Name = name
		if g2, _ := splitGroup(name); g2 == "" {
			single++
		} else {
			n++
		}
	}
	if n > 0 || single > 0 {
		var parts []string
		if n > 0 {
			parts = append(parts, t.say("twoLevel.renamed", msg.Args{"n": n}))
		}
		if single > 0 {
			parts = append(parts, t.say("twoLevel.single", msg.Args{"n": single}))
		}
		t.notes = append(t.notes, t.say("note.groupsFromAbove", msg.Args{"table": tb.Name, "details": strings.Join(parts, ", ")}))
	}
	return nil
}

// Colonnes sur un niveau (demande de Louis) : dans un tableau à deux lignes,
// une colonne au nom simple (sans « Groupe. ») n'a pas de groupe ; sa
// cellule de la bande et son en-tête ne font qu'une cellule, fusionnée sur
// les deux lignes. Un groupe seul sur sa colonne dont l'en-tête est
// automatique (« Topographie » sur « Colonne 17 ») devient le nom de la
// colonne ; un groupe qui répète l'en-tête (« DECOUVREUR » sur
// « DECOUVREUR ») aussi : la colonne reste sur un niveau.

// autoSub : en-tête donné par Lattis (« Colonne n ») ou vide.
func autoSub(s string) bool {
	s = strings.TrimSpace(s)
	return s == "" || autoColRe.MatchString(s)
}

var labelAccents = strings.NewReplacer(
	"à", "a", "â", "a", "ä", "a", "á", "a", "ã", "a", "å", "a",
	"ç", "c", "é", "e", "è", "e", "ê", "e", "ë", "e",
	"î", "i", "ï", "i", "í", "i", "ì", "i", "ñ", "n",
	"ô", "o", "ö", "o", "ó", "o", "ò", "o", "õ", "o",
	"ù", "u", "û", "u", "ü", "u", "ú", "u", "ÿ", "y", "œ", "oe", "æ", "ae",
)

// sameLabel : deux libellés égaux aux majuscules, accents et espaces près.
func sameLabel(a, b string) bool {
	f := func(s string) string {
		return labelAccents.Replace(strings.ToLower(strings.Join(strings.Fields(s), " ")))
	}
	return f(a) == f(b)
}

// groupedName : nom d'une colonne de sous-en-tête sub sous le groupe g ;
// alone : le groupe ne couvre que cette colonne. Sur un niveau (nom simple)
// quand le groupe répète l'en-tête, ou qu'il est seul au-dessus d'un
// en-tête automatique.
func groupedName(g, sub string, alone bool) string {
	switch {
	case g == "", sameLabel(g, sub):
		return sub
	case alone && autoSub(sub):
		return g
	}
	return g + "." + sub
}

// groupAlone : le groupe g posé sur la colonne i n'en couvrirait pas
// d'autre (ses voisines ont un autre groupe), hors des colonnes lo à hi
// qui le reçoivent aussi.
func groupAlone(tb Table, i, lo, hi int, g string) bool {
	if lo != hi {
		return false
	}
	other := func(k int) bool {
		if k < 0 || k >= len(tb.Cols) {
			return true
		}
		h, _ := splitGroup(tb.Cols[k].Name)
		return h != g
	}
	return other(i-1) && other(i+1)
}

// groupsAbove lit les groupes posés au-dessus de l'en-tête (ligne Row-1),
// comme Excel : une cellule fusionnée (même sur plusieurs lignes) donne son
// texte à toutes les colonnes qu'elle couvre ; une valeur non fusionnée
// couvre les cellules vides qui la suivent, jusqu'à la valeur ou la fusion
// suivante. Les cellules et leurs fusions sont retirées de la feuille (elles
// deviennent la bande).
func (t *tx) groupsAbove(tb Table) ([]string, error) {
	r := tb.Row - 1
	cells, err := t.cells(tb.Sheet)
	if err != nil {
		return nil, err
	}
	merges, err := t.merges(tb.Sheet)
	if err != nil {
		return nil, err
	}
	var over []Merge
	for _, g := range merges {
		if g.overlaps(r, tb.Col, r, tb.Right()) {
			over = append(over, g)
		}
	}
	inMerge := func(c cell) (Merge, bool) {
		for _, g := range over {
			if g.overlaps(c.Row, c.Col, c.Row, c.Col) {
				return g, true
			}
		}
		return Merge{}, false
	}
	// Textes de la ligne, et de celles des fusions qui la traversent (une
	// fusion sur deux lignes, un texte resté sous la fusion).
	text := map[[2]int]string{}
	var spill []cell
	for _, c := range cells {
		g, ok := inMerge(c)
		if c.Row != r && !ok {
			continue
		}
		s, err := t.freeText(tb.Sheet, c)
		if err != nil {
			return nil, err
		}
		if s = cleanGroup(s); s != "" {
			text[[2]int{c.Row, c.Col}] = s
		}
		if ok && (c.Row != r || c.Col < tb.Col || c.Col > tb.Right()) && !(c.Row == g.Row && c.Col == g.Col) {
			spill = append(spill, c)
		}
	}
	merged := map[int]string{}
	isMerged := map[int]bool{}
	for _, g := range over {
		// Le texte de la fusion est celui de sa première cellule, sinon le
		// premier qu'elle couvre.
		v := text[[2]int{g.Row, g.Col}]
		for rr := g.Row; v == "" && rr <= g.bottom(); rr++ {
			for c := g.Col; v == "" && c <= g.right(); c++ {
				v = text[[2]int{rr, c}]
			}
		}
		for c := max(g.Col, tb.Col); c <= min(g.right(), tb.Right()); c++ {
			isMerged[c], merged[c] = true, v
		}
	}
	groups := make([]string, len(tb.Cols))
	cur := ""
	for c := tb.Col; c <= tb.Right(); c++ {
		switch v, ok := text[[2]int{r, c}]; {
		case isMerged[c]:
			groups[c-tb.Col], cur = merged[c], ""
		case ok:
			groups[c-tb.Col], cur = v, v
		default:
			groups[c-tb.Col] = cur
		}
	}
	// La ligne devient la bande : fusions, cellules et mise en forme partent.
	// Une fusion qui déborde de la bande y part entière : son texte aussi.
	for _, g := range over {
		if err := t.styleFree(tb.Sheet, g.Row, g.Col, Style{"mr": nil, "mc": nil}, nil); err != nil {
			return nil, err
		}
		if g.Row != r || g.Col < tb.Col || g.Col > tb.Right() {
			if err := t.setFreeCell(tb.Sheet, g.Row, g.Col, ""); err != nil {
				return nil, err
			}
			if err := t.deleteWhere(`_lattis_cell_styles`, `sheet = ? AND row = ? AND col = ?`, tb.Sheet, g.Row, g.Col); err != nil {
				return nil, err
			}
		}
	}
	for _, c := range spill {
		if err := t.setFreeCell(tb.Sheet, c.Row, c.Col, ""); err != nil {
			return nil, err
		}
	}
	if err := t.clearBand(tb); err != nil {
		return nil, err
	}
	return groups, nil
}

// clearBand retire de la bande du tableau les cellules libres et leur mise
// en forme.
func (t *tx) clearBand(tb Table) error {
	r := tb.Row - 1
	cells, err := t.cells(tb.Sheet)
	if err != nil {
		return err
	}
	for _, c := range cells {
		if c.Row == r && c.Col >= tb.Col && c.Col <= tb.Right() {
			if err := t.setFreeCell(tb.Sheet, c.Row, c.Col, ""); err != nil {
				return err
			}
		}
	}
	return t.deleteWhere(`_lattis_cell_styles`, `sheet = ? AND row = ? AND col BETWEEN ? AND ?`, tb.Sheet, r, tb.Col, tb.Right())
}

// freeText : texte d'une cellule libre ; une formule entre par sa valeur.
func (t *tx) freeText(sheet int64, c cell) (string, error) {
	if !isFormula(c.Raw) {
		return strings.TrimPrefix(c.Raw, "'"), nil
	}
	schema, err := t.schema()
	if err != nil {
		return "", err
	}
	values, err := t.nameValuesWith(schema, true)
	if err != nil {
		return "", err
	}
	return values[formula.CellKey(sheet, c.Row, c.Col)].String(), nil
}

// bandAt donne le tableau dont la bande des groupes contient (row, col),
// avec le rang de la colonne.
func bandAt(tables []Table, row, col int) (Table, int, bool) {
	for _, tb := range tables {
		if tb.TwoLevel && row == tb.Row-1 && col >= tb.Col && col <= tb.Right() {
			return tb, col - tb.Col, true
		}
	}
	return Table{}, 0, false
}

// groupSpan : colonnes consécutives qui partagent le groupe de la colonne i
// (la cellule fusionnée de la bande) ; i seule si elle n'a pas de groupe.
func groupSpan(tb Table, i int) (lo, hi int) {
	g, _ := splitGroup(tb.Cols[i].Name)
	lo, hi = i, i
	if g == "" {
		return
	}
	same := func(k int) bool { h, _ := splitGroup(tb.Cols[k].Name); return h == g }
	for lo > 0 && same(lo-1) {
		lo--
	}
	for hi < len(tb.Cols)-1 && same(hi+1) {
		hi++
	}
	return
}

// inputBand : une saisie dans la bande des groupes renomme le groupe de
// toutes les colonnes qu'il couvre (ou le pose sur une colonne sans
// groupe) ; vide, elle retire le groupe.
func (t *tx) inputBand(tb Table, i int, raw string) error {
	if err := t.lockTable(tb.ID); err != nil {
		return err
	}
	g := strings.TrimSpace(raw)
	if isFormula(g) {
		return msg.Wrap(ErrTable, "table.groupNotFormula", nil)
	}
	if strings.Contains(g, ".") {
		return msg.Wrap(ErrTable, "table.groupNoDot", nil)
	}
	lo, hi := groupSpan(tb, i)
	return t.setGroups(tb, lo, hi, cleanGroup(g))
}

// setGroups pose le groupe g (vide : aucun) sur les colonnes lo à hi. Un
// nom qui redeviendrait celui d'une autre colonne reste complet, et la
// barre d'état le dit.
func (t *tx) setGroups(tb Table, lo, hi int, g string) error {
	for k := lo; k <= hi; k++ {
		c := tb.Cols[k]
		_, sub := splitGroup(c.Name)
		name := groupedName(g, sub, groupAlone(tb, k, lo, hi, g))
		if name == c.Name {
			continue
		}
		if columnNameTaken(tb, c.ID, name) {
			if g == "" {
				t.notes = append(t.notes, t.say("note.columnKeepsFullName", msg.Args{"column": c.Name, "name": sub, "table": tb.Name}))
				continue
			}
			name = freeColumnNameExcept(tb, c.ID, name)
		}
		if err := t.rename(tb, c, name); err != nil {
			return err
		}
		tb.Cols[k].Name = name
	}
	return nil
}

// absorbBand : une cellule libre arrivée dans la bande des groupes d'un
// tableau (collage, déplacement de cellules) devient le groupe de sa
// colonne ; une seule à la fois (joinStep recommence).
func (t *tx) absorbBand(sheet int64, tables []Table) (bool, error) {
	var cells []cell
	for _, tb := range tables {
		if !tb.TwoLevel {
			continue
		}
		if cells == nil {
			var err error
			if cells, err = t.cells(sheet); err != nil {
				return false, err
			}
		}
		for _, c := range cells {
			if c.Row != tb.Row-1 || c.Col < tb.Col || c.Col > tb.Right() {
				continue
			}
			s, err := t.freeText(sheet, c)
			if err != nil {
				return false, err
			}
			if err := t.setFreeCell(sheet, c.Row, c.Col, ""); err != nil {
				return false, err
			}
			if err := t.deleteWhere(`_lattis_cell_styles`, `sheet = ? AND row = ? AND col = ?`, sheet, c.Row, c.Col); err != nil {
				return false, err
			}
			if g := cleanGroup(s); g != "" {
				if err := t.lockTable(tb.ID); err != nil {
					return false, err
				}
				if err := t.setGroups(tb, c.Col-tb.Col, c.Col-tb.Col, g); err != nil {
					return false, err
				}
				t.notes = append(t.notes, t.say("note.groupFromAbove", msg.Args{"group": clip(g), "column": tb.Cols[c.Col-tb.Col].Name}))
			}
			return true, nil
		}
	}
	return false, nil
}

// mergeBand : Fusionner dans la bande et l'en-tête d'un tableau à deux
// lignes. La bande et l'en-tête d'une colonne mettent la colonne sur un
// niveau ; des cellules voisines de la bande (avec ou sans leurs en-têtes)
// forment un seul groupe. Faux si la plage ne touche aucune bande.
func (t *tx) mergeBand(tables []Table, r0, c0, r1, c1 int) (bool, error) {
	var hit []Table
	band := false
	for _, tb := range tables {
		if tb.Top() <= r1 && tb.Bottom() >= r0 && tb.Col <= c1 && tb.Right() >= c0 {
			hit = append(hit, tb)
			band = band || tb.TwoLevel && r0 <= tb.Row-1 && r1 >= tb.Row-1
		}
	}
	if !band {
		return false, nil
	}
	if len(hit) > 1 {
		return true, msg.Wrap(ErrMerge, "merge.severalTables", nil)
	}
	tb := hit[0]
	if c0 < tb.Col || c1 > tb.Right() || r0 < tb.Row-1 || r1 > tb.Row {
		return true, msg.Wrap(ErrMerge, "merge.twoLevelOnly", msg.Args{"table": tb.Name})
	}
	if err := t.lockTable(tb.ID); err != nil {
		return true, err
	}
	if c0 == c1 {
		return true, t.singleLevel(tb, c0-tb.Col)
	}
	return true, t.joinGroup(tb, c0-tb.Col, c1-tb.Col)
}

// singleLevel met la colonne i sur un niveau : son nom devient le
// sous-en-tête, ou le groupe si le sous-en-tête était automatique.
func (t *tx) singleLevel(tb Table, i int) error {
	c := tb.Cols[i]
	g, sub := splitGroup(c.Name)
	if g == "" {
		t.notes = append(t.notes, t.say("note.columnAlreadySingle", msg.Args{"column": c.Name}))
		return nil
	}
	lo, hi := groupSpan(tb, i)
	name := sub
	if autoSub(sub) {
		name = g
	}
	if columnNameTaken(tb, c.ID, name) {
		name = freeColumnNameExcept(tb, c.ID, name)
	}
	if err := t.rename(tb, c, name); err != nil {
		return err
	}
	why := t.say("twoLevel.whyLeavesGroup", msg.Args{"group": g})
	switch {
	case name == g:
		why = t.say("twoLevel.whyGroupBecomesName", nil)
	case lo == hi:
		why = t.say("twoLevel.whyGroupRemoved", msg.Args{"group": g})
	}
	t.notes = append(t.notes, t.say("note.columnSingle", msg.Args{"column": c.Name, "name": name, "why": why}))
	return nil
}

// joinGroup réunit les colonnes lo à hi sous un seul groupe : le premier
// groupe qu'elles portent (de gauche à droite), sinon « Groupe », à
// renommer en tapant dans la bande.
func (t *tx) joinGroup(tb Table, lo, hi int) error {
	g := ""
	var lost []string
	for k := lo; k <= hi; k++ {
		h, _ := splitGroup(tb.Cols[k].Name)
		switch {
		case h == "":
		case g == "":
			g = h
		case h != g && !slices.Contains(lost, h):
			lost = append(lost, h)
		}
	}
	placeholder := g == ""
	if placeholder {
		g = "Groupe"
	}
	for k := lo; k <= hi; k++ {
		c := tb.Cols[k]
		_, sub := splitGroup(c.Name)
		name := g + "." + sub
		if name == c.Name {
			continue
		}
		name = freeColumnNameExcept(tb, c.ID, name)
		if err := t.rename(tb, c, name); err != nil {
			return err
		}
		tb.Cols[k].Name = name
	}
	note := t.say("note.groupMerged", msg.Args{"group": g, "n": hi - lo + 1})
	if placeholder {
		note += t.say("twoLevel.typeName", nil)
	}
	if len(lost) > 0 {
		note += t.say("twoLevel.replaces", msg.Args{"names": strings.Join(lost, " », « ")})
	}
	t.notes = append(t.notes, note)
	return nil
}

// unmergeBand : Défusionner dans la bande des groupes. Un groupe sur
// plusieurs colonnes reste sur la première, les autres passent sur un
// niveau (comme Excel garde le texte dans la première cellule). Une
// colonne sur un niveau ne change pas ici : son groupe se tape dans la
// bande (l'interface la sépare d'abord), ou elle se renomme « Groupe.Nom ».
func (t *tx) unmergeBand(tables []Table, r0, c0, r1, c1 int) error {
	said := len(t.notes)
	single := 0
	for _, tb := range tables {
		if !tb.TwoLevel || r0 > tb.Row || r1 < tb.Row-1 || c1 < tb.Col || c0 > tb.Right() {
			continue
		}
		for i := max(c0, tb.Col) - tb.Col; i <= min(c1, tb.Right())-tb.Col; {
			lo, hi := groupSpan(tb, i)
			i = hi + 1
			g, _ := splitGroup(tb.Cols[lo].Name)
			if g == "" {
				single++
				continue
			}
			if lo == hi {
				continue
			}
			if err := t.lockTable(tb.ID); err != nil {
				return err
			}
			if err := t.setGroups(tb, lo+1, hi, ""); err != nil {
				return err
			}
			t.notes = append(t.notes, t.say("note.groupUnmerged", msg.Args{"group": g, "column": tb.Cols[lo].Name, "n": hi - lo}))
		}
	}
	if single > 0 && len(t.notes) == said {
		t.notes = append(t.notes, t.say("note.singleLevelHint", nil))
	}
	return nil
}
