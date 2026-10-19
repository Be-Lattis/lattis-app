package canvas

import (
	"context"
	"fmt"
	"strings"

	"github.com/lalamefine/lattis/internal/formula"
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
	label := "En-têtes sur deux lignes"
	if !on {
		label = "En-têtes sur une ligne"
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
		t.notes = append(t.notes, fmt.Sprintf("Tableau « %s » descendu d'une ligne pour faire place à ses groupes d'en-têtes", tb.Name))
		return flag()
	}
	groups, err := t.groupsAbove(tb)
	if err != nil {
		return err
	}
	if err := flag(); err != nil {
		return err
	}
	n := 0
	for i, g := range groups {
		if g == "" {
			continue
		}
		c := tb.Cols[i]
		if cur, _ := splitGroup(c.Name); strings.EqualFold(cur, g) {
			continue
		}
		name := freeColumnNameExcept(tb, c.ID, g+"."+c.Name)
		if err := t.rename(tb, c, name); err != nil {
			return err
		}
		tb.Cols[i].Name = name
		n++
	}
	if n > 0 {
		t.notes = append(t.notes, fmt.Sprintf("Ligne au-dessus de « %s » reprise en groupes d'en-têtes (%d colonnes renommées « Groupe.Nom »)", tb.Name, n))
	}
	return nil
}

// groupsAbove lit les groupes posés au-dessus de l'en-tête (ligne Row-1),
// comme Excel : une cellule fusionnée couvre exactement sa fusion ; une
// valeur non fusionnée couvre les cellules vides qui la suivent, jusqu'à la
// valeur ou la fusion suivante. Les cellules et leurs fusions sont retirées
// de la feuille (elles deviennent la bande).
func (t *tx) groupsAbove(tb Table) ([]string, error) {
	r := tb.Row - 1
	cells, err := t.cells(tb.Sheet)
	if err != nil {
		return nil, err
	}
	text := map[int]string{}
	for _, c := range cells {
		if c.Row == r {
			s, err := t.freeText(tb.Sheet, c)
			if err != nil {
				return nil, err
			}
			text[c.Col] = cleanGroup(s)
		}
	}
	merges, err := t.merges(tb.Sheet)
	if err != nil {
		return nil, err
	}
	merged := map[int]string{}
	isMerged := map[int]bool{}
	var anchors [][2]int
	for _, g := range merges {
		if !g.overlaps(r, tb.Col, r, tb.Right()) {
			continue
		}
		v := ""
		if g.Row == r {
			v = text[g.Col]
		}
		for c := max(g.Col, tb.Col); c <= min(g.right(), tb.Right()); c++ {
			isMerged[c], merged[c] = true, v
		}
		anchors = append(anchors, [2]int{g.Row, g.Col})
	}
	groups := make([]string, len(tb.Cols))
	cur := ""
	for c := tb.Col; c <= tb.Right(); c++ {
		switch v, ok := text[c]; {
		case isMerged[c]:
			groups[c-tb.Col], cur = merged[c], ""
		case ok:
			groups[c-tb.Col], cur = v, v
		default:
			groups[c-tb.Col] = cur
		}
	}
	// La ligne devient la bande : fusions, cellules et mise en forme partent.
	for _, a := range anchors {
		if err := t.styleFree(tb.Sheet, a[0], a[1], Style{"mr": nil, "mc": nil}, nil); err != nil {
			return nil, err
		}
		if a[0] == r && (a[1] < tb.Col || a[1] > tb.Right()) {
			if err := t.setFreeCell(tb.Sheet, a[0], a[1], ""); err != nil {
				return nil, err
			}
			if err := t.deleteWhere(`_lattis_cell_styles`, `sheet = ? AND row = ? AND col = ?`, tb.Sheet, a[0], a[1]); err != nil {
				return nil, err
			}
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
		return fmt.Errorf("%w : un groupe d'en-têtes est un texte, pas une formule", ErrTable)
	}
	if strings.Contains(g, ".") {
		return fmt.Errorf("%w : un groupe d'en-têtes ne contient pas de point (le point sépare le groupe du nom dans « Groupe.Nom »)", ErrTable)
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
		name := sub
		if g != "" {
			name = g + "." + sub
		}
		if name == c.Name {
			continue
		}
		if columnNameTaken(tb, c.ID, name) {
			if g == "" {
				t.notes = append(t.notes, fmt.Sprintf("La colonne « %s » garde son nom complet : « %s » est déjà pris dans le tableau « %s »", c.Name, sub, tb.Name))
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
				t.notes = append(t.notes, fmt.Sprintf("« %s » devient le groupe d'en-têtes de la colonne « %s »", clip(g), tb.Cols[c.Col-tb.Col].Name))
			}
			return true, nil
		}
	}
	return false, nil
}
