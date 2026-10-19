package canvas

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// MinRows est le nombre de lignes (en-tête compris) à partir duquel une zone
// de cellules libres devient un tableau.
const MinRows = 2

// dense dit si une zone (lignes de cellules) remplit au moins trois cases
// d'un même carré de 2 × 2 : la condition pour qu'elle devienne d'elle-même
// un tableau. Une colonne ou une ligne seule, ou des cellules en escalier,
// restent des cellules libres.
func dense(g [][]string) bool {
	filled := func(r, c int) int {
		if c < len(g[r]) && strings.TrimSpace(g[r][c]) != "" {
			return 1
		}
		return 0
	}
	for r := 0; r+1 < len(g); r++ {
		for c := 0; c+1 < max(len(g[r]), len(g[r+1])); c++ {
			if filled(r, c)+filled(r, c+1)+filled(r+1, c)+filled(r+1, c+1) >= 3 {
				return true
			}
		}
	}
	return false
}

// Event décrit ce qu'une saisie a provoqué, pour l'afficher à l'utilisateur.
type Event struct {
	Created string    `json:"created"` // tableau créé
	Widened string    `json:"widened"` // colonne élargie ("Tableau.Colonne")
	From    grid.Type `json:"from"`    // élargissement
	To      grid.Type `json:"to"`
	Invalid bool      `json:"invalid"` // saisie hors type dans une colonne au type fixé
	// Headless : tableau collé sans ligne d'en-tête reconnue (« Colonne n »).
	Headless bool `json:"headless"`
	// Keyed : colonne devenue clé pour une formule (« Tableau.Colonne »).
	Keyed string `json:"keyed"`
	// Refused : valeurs collées écartées car hors du type figé d'un tableau
	// partagé (« Colonne : « valeur » », au plus quelques-unes) ;
	// RefusedCount : leur nombre (§9).
	Refused      []string `json:"refused,omitempty"`
	RefusedCount int      `json:"refusedCount,omitempty"`
}

// SetCell traite une saisie validée dans la cellule (row, col) d'une feuille,
// comme le ferait la grille : dans un tableau, dans sa ligne ou sa colonne
// grisée (extension), ou dans une cellule libre (avec détection d'îlot).
func (m *Model) SetCell(ctx context.Context, sheet int64, row, col int, raw string) (Event, error) {
	var ev Event
	err := m.Do(ctx, "Saisie", func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		err := t.input(sheet, row, col, raw, &ev)
		ev.Keyed = t.keyed
		return err
	})
	return ev, err
}

// input applique une saisie dans la transaction en cours.
func (t *tx) input(sheet int64, row, col int, raw string, ev *Event) error {
	{
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		empty := strings.TrimSpace(raw) == ""
		// Dans une plage (tableau converti), la saisie reste libre : elle
		// n'agrandit pas un tableau voisin.
		plain, err := t.inRange(sheet, row, col)
		if err != nil {
			return err
		}
		for _, tb := range tables {
			inCols := col >= tb.Col && col <= tb.Right()
			inRows := row >= tb.Row && row <= tb.Bottom()
			if inCols && inRows {
				// Tableau publié en lecture seule (classeur partagé) : refusé
				// tout de suite, l'en-tête compris (un renommage de colonne
				// n'écrit aucune ligne).
				if err := t.lockTable(tb.ID); err != nil {
					return err
				}
			}
			switch {
			case inCols && row == tb.Row:
				c := tb.Cols[col-tb.Col]
				name := strings.TrimSpace(raw)
				if name == "" {
					name = freeColumnNameExcept(tb, c.ID, fmt.Sprintf("Colonne %d", col-tb.Col+1))
				}
				if columnNameTaken(tb, c.ID, name) {
					if !t.pasting {
						return errColumnTaken(tb, name)
					}
					name = freeColumnNameExcept(tb, c.ID, name)
				}
				return t.rename(tb, c, name)
			case inCols && inRows:
				return t.setCell(tb, row-tb.Row-1, col-tb.Col, raw, ev)
			case plain && !(inCols && inRows) && !(inCols && row == tb.Row):
				continue
			case inCols && row == tb.Bottom()+1 && !empty:
				if tb.Filtered() {
					return ErrFiltered{Table: tb.Name}
				}
				// Une note ou un total sous le tableau reste à part (legend.go).
				if !t.noLegend {
					kind, why, err := t.footKind(tb, row, col, raw)
					if err != nil {
						return err
					}
					if kind != "" {
						return t.keepOut(tb, row, col, raw, kind, why)
					}
				}
				outs, err := t.outCells(sheet)
				if err != nil {
					return err
				}
				// Les autres cellules libres de la nouvelle ligne la rejoignent
				// (pas celles gardées à part, qui descendent avec le bas du
				// tableau).
				taken, err := t.takeFree(sheet, func(c cell) bool {
					return c.Row == row && c.Col >= tb.Col && c.Col <= tb.Right() && c.Col != col && !outs[[2]int{c.Row, c.Col}]
				})
				if err != nil {
					return err
				}
				if err := t.appendRows(tb, 1); err != nil {
					return err
				}
				tb.Rows++
				for _, c := range taken {
					if err := t.setCell(tb, tb.Rows-1, c.Col-tb.Col, c.Raw, ev); err != nil {
						return err
					}
				}
				return t.setCell(tb, tb.Rows-1, col-tb.Col, raw, ev)
			case (inRows && (col == tb.Col-1 || col == tb.Right()+1)) && row != tb.Row && !empty && !t.noLegend &&
				noteRe.MatchString(raw) && !inAny(tables, row, col):
				// « ← à vérifier » à côté d'une ligne : une remarque, pas une
				// colonne.
				return t.keepOut(tb, row, col, raw, outNote, "remarque à côté du tableau")
			case inRows && col == tb.Col-1 && !empty && !inAny(tables, row, col) && !rightOfAny(tables, tb, row, col):
				// Juste à gauche : nouvelle première colonne. Juste à droite d'un autre
				// tableau, c'est lui qui la prend (et les deux fusionnent).
				if err := t.growSide(tb, row, col, raw, ev, true); err != nil || row != tb.Bottom() {
					return err
				}
				return t.reshapeYoung(tb.ID)
			case inRows && col == tb.Right()+1 && !empty && !inAny(tables, row, col):
				// Juste à droite : nouvelle dernière colonne. Un tableau juste
				// après (lignes qui se recoupent) : la saisie comble l'écart
				// et les deux tableaux fusionnent.
				var next *Table
				for i := range tables {
					if o := tables[i]; o.Col == col+1 && o.Row <= tb.Bottom() && tb.Row <= o.Bottom() {
						next = &tables[i]
					}
				}
				if err := t.growSide(tb, row, col, raw, ev, false); err != nil {
					return err
				}
				if next == nil {
					if row != tb.Bottom() {
						return nil
					}
					return t.reshapeYoung(tb.ID)
				}
				o, err := t.table(next.ID)
				if err != nil {
					return err
				}
				if tb, err = t.table(tb.ID); err != nil {
					return err
				}
				// En-têtes sur des lignes différentes, tableau filtré ou chargé
				// par une requête : pas de fusion, le rattachement de fin
				// d'action les écarte et le dit.
				fixed, err := t.fixedTables([]Table{tb, o})
				if err != nil || tb.Row != o.Row || fixed[tb.ID] != "" || fixed[o.ID] != "" {
					return err
				}
				return t.joinTables(tb, o)
			}
		}
		if isFormula(raw) {
			// Formule dans une cellule libre : enregistrée réécrite (RECHERCHEV
			// devient REFERENCE), calculée à l'affichage. Elle ne fait pas
			// naître de tableau.
			n, err := t.prepareFormula(sheet, strings.TrimSpace(raw))
			if err != nil {
				return err
			}
			return t.setFreeCell(sheet, row, col, formula.Format(n, formula.FR))
		}
		if empty {
			if err := t.clearOut(sheet, row, col); err != nil {
				return err
			}
		}
		if err := t.setFreeCell(sheet, row, col, freeRaw(raw)); err != nil || empty {
			return err
		}
		// Sous une note gardée hors du tableau, une autre note.
		if !t.noLegend {
			if out, err := t.noteAbove(sheet, row, col); err != nil || out {
				if err != nil {
					return err
				}
				return t.markOut(sheet, row, col, outNote)
			}
		}
		// Collage cellule par cellule : comme avant, deux lignes suffisent.
		name, err := t.birthOf(sheet, row, col, !t.pasting)
		if name != "" {
			ev.Created = name
		}
		return err
	}
}

// setCell écrit une saisie dans la ligne de données n, colonne i. Une saisie
// hors type élargit une colonne au type inféré, ou est marquée invalide dans
// une colonne au type fixé. Une colonne encore vide prend le type de sa
// première valeur.
func (t *tx) setCell(tb Table, n, i int, raw string, ev *Event) error {
	rid, _, err := t.rowID(tb, n)
	if err != nil {
		return err
	}
	_, err = t.setCellAt(tb, rid, i, raw, ev)
	return err
}

// setCellAt saisit raw dans la colonne i de la ligne rid ; renvoie le
// tableau, changé quand la colonne a pris un autre type.
func (t *tx) setCellAt(tb Table, rid int64, i int, raw string, ev *Event) (Table, error) {
	var err error
	c := tb.Cols[i]
	if src := strings.TrimSpace(raw); strings.HasPrefix(src, "=") && len(src) > 1 {
		return tb, t.setFormula(tb, i, src)
	}
	if c.Formula != "" {
		if strings.TrimSpace(raw) == "" {
			return tb, t.setFormula(tb, i, "") // effacer une cellule calculée retire la formule
		}
		return tb, ErrComputed{c.Name}
	}
	if strings.TrimSpace(raw) == "" {
		return tb, t.setValue(tb, rid, c, nil, "")
	}
	// Apostrophe de tête ('123, '007) : du texte, comme dans Excel ; elle force
	// le type et n'est pas gardée dans la valeur (choix de Louis).
	typed := raw
	forced := strings.HasPrefix(raw, "'") && len(raw) > 1
	if forced {
		raw = raw[1:]
	}
	infer := func() grid.Type {
		if forced {
			return grid.Text
		}
		return Infer(raw)
	}
	v, perr := grid.Parse(c.Type, raw)
	if forced && c.Type != grid.Text {
		v, perr = nil, errForcedText
	}
	if perr != nil && t.frozenTypes(tb.ID) {
		// Tableau publié d'un classeur partagé : type figé, la valeur hors
		// type n'est pas gardée en erreur (rien ne partirait au serveur).
		return tb, t.refuse(tb, c, raw, ev)
	}
	if c.Fixed {
		if perr != nil {
			ev.Invalid = true
			return tb, t.setValue(tb, rid, c, nil, typed)
		}
		return tb, t.setValue(tb, rid, c, v, "")
	}
	// La colonne a-t-elle une autre valeur ? EXISTS s'arrête à la première
	// (un count parcourait toute la colonne : 0,25 s par saisie sur 1 M de lignes).
	var others int
	if err := t.row(`SELECT EXISTS (SELECT 1 FROM `+tb.sql()+` WHERE `+colSQL(c.ID)+` IS NOT NULL AND id <> ?)`, rid).Scan(&others); err != nil {
		return tb, err
	}
	to := c.Type
	switch {
	case others == 0:
		to = infer() // colonne vide : elle prend le type de sa première valeur
	case perr != nil:
		to = Join(c.Type, infer())
		ev.Widened, ev.From, ev.To = tb.Name+"."+c.Name, c.Type, to
	}
	if to != c.Type {
		if tb, err = t.retype(tb, i, to); err != nil {
			return tb, err
		}
	}
	if v, err = grid.Parse(to, raw); err != nil {
		return tb, err
	}
	return tb, t.setValue(tb, rid, tb.Cols[i], v, "")
}

// errForcedText : saisie forcée en texte par une apostrophe, dans une colonne
// d'un autre type.
var errForcedText = errors.New("texte forcé par une apostrophe")

// appendRows ajoute n lignes vides en fin de tableau et décale ce qui est dessous.
func (t *tx) appendRows(tb Table, n int) error {
	var last string
	var maxID int64
	if err := t.row(`SELECT coalesce(max(_pos), ''), coalesce(max(id), 0) FROM `+tb.sql()).Scan(&last, &maxID); err != nil {
		return err
	}
	for k := range n {
		pos, err := t.rowKey(tb, last, "")
		if err != nil {
			return err
		}
		if err := t.insertRow(tb, maxID+int64(k)+1, pos); err != nil {
			return err
		}
		last = pos
	}
	return t.ensureGapBelow(tb.Sheet, tb.ID, tb.Bottom(), tb.Bottom()+n, tb.Col, tb.Right())
}

// newColumn ajoute une colonne au rang pos et décale ce qui est à droite.
func (t *tx) newColumn(tb Table, pos int, name string) (Table, error) {
	var id int64
	if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_columns WHERE tbl = ?`, tb.ID).Scan(&id); err != nil {
		return tb, err
	}
	if err := t.addColumn(tb, id, name, grid.Text, pos); err != nil {
		return tb, err
	}
	if err := t.ensureGapRight(tb.Sheet, tb.ID, tb.Right(), tb.Right()+1, tb.Row, tb.Bottom()); err != nil {
		return tb, err
	}
	return t.table(tb.ID)
}

// birth crée un tableau si la cellule saisie appartient à une zone de
// cellules libres d'au moins MinRows lignes. La première ligne de la zone
// donne les en-têtes.
func (t *tx) birth(sheet int64, row, col int) (string, error) {
	return t.birthOf(sheet, row, col, false)
}

// birthOf est birth ; typed (saisie) : la zone doit aussi remplir un carré
// de 2 × 2 (dense), une liste d'une seule colonne reste libre. Un tableau
// déplacé ou relu (notes, titres) renaît sans cette condition.
func (t *tx) birthOf(sheet int64, row, col int, typed bool) (string, error) {
	cells, err := t.cells(sheet)
	if err != nil {
		return "", err
	}
	// Les cellules gardées hors des tableaux (notes, totaux) ne relient pas
	// un îlot.
	outs, err := t.outCells(sheet)
	if err != nil {
		return "", err
	}
	if outs[[2]int{row, col}] {
		return "", nil
	}
	isl := island(slices.DeleteFunc(slices.Clone(cells), func(c cell) bool { return outs[[2]int{c.Row, c.Col}] }), row, col)
	// Près d'une plage (tableau converti), l'îlot reste une plage.
	if keep, err := t.keepRange(sheet, isl); keep || err != nil {
		return "", err
	}
	bounds := func(isl []cell) (r0, r1, c0, c1 int) {
		r0, r1, c0, c1 = isl[0].Row, isl[0].Row, isl[0].Col, isl[0].Col
		for _, c := range isl {
			r0, r1 = min(r0, c.Row), max(r1, c.Row)
			c0, c1 = min(c0, c.Col), max(c1, c.Col)
		}
		return
	}
	read := func(r0, r1, c0, c1 int) [][]string {
		g := make([][]string, r1-r0+1)
		for i := range g {
			g[i] = make([]string, c1-c0+1)
		}
		for _, c := range cells {
			if c.Row >= r0 && c.Row <= r1 && c.Col >= c0 && c.Col <= c1 {
				g[c.Row-r0][c.Col-c0] = c.Raw
			}
		}
		return g
	}
	r0, r1, c0, c1 := bounds(isl)
	// Un titre (« Ventes 2026 ») au-dessus de la ligne d'en-têtes reste une
	// cellule libre : le tableau naît dessous.
	var titles []string
	if n := titleRows(read(r0, r1, c0, c1)); n > 0 {
		for _, c := range isl {
			if c.Row < r0+n {
				titles = append(titles, c.Raw)
			}
		}
		isl = slices.DeleteFunc(isl, func(c cell) bool { return c.Row < r0+n })
		r0, r1, c0, c1 = bounds(isl)
	}
	if r1-r0+1 < MinRows {
		return "", nil
	}
	grid2 := read(r0, r1, c0, c1)
	if typed && !dense(grid2) {
		return "", nil
	}
	headers, data, top := grid2[0], grid2[1:], r0
	if !typedHeader(grid2) {
		// Une liste de valeurs (10, 20, 30) : la première ligne reste une
		// donnée, les colonnes s'appellent « Colonne n » et l'en-tête se
		// place au-dessus. Sans place libre au-dessus, les cellules restent
		// libres (comme dans Excel, où une liste n'est pas un tableau).
		if r0 == 0 {
			return "", nil
		}
		free, err := t.areaFree(sheet, r0-1, c0-1, r0-1, c1+1)
		if err != nil || !free {
			return "", err
		}
		headers = make([]string, c1-c0+1)
		for i := range headers {
			headers[i] = "Colonne " + itoa(i+1)
		}
		data, top = grid2, r0-1
	}
	for _, c := range cells {
		if c.Row >= r0 && c.Row <= r1 && c.Col >= c0 && c.Col <= c1 {
			if err := t.setFreeCell(sheet, c.Row, c.Col, ""); err != nil {
				return "", err
			}
			if err := t.clearOut(sheet, c.Row, c.Col); err != nil {
				return "", err
			}
		}
	}
	if len(titles) > 0 {
		t.notes = append(t.notes, fmt.Sprintf("« %s » gardé comme titre au-dessus du tableau (clic droit sur l'en-tête pour changer)", clip(strings.Join(titles, " "))))
	}
	name, err := t.tableName(sheet, top, c0)
	if err != nil {
		return "", err
	}
	_, err = t.createTable(sheet, name, top, c0, headers, data)
	return name, err
}

// typedHeader : la première ligne d'un îlot saisi donne les en-têtes si
// elle a au moins un texte. Une première ligne sans texte (nombre, date,
// heure, booléen, pourcentage) est une donnée, pas un titre, même au-dessus
// de valeurs d'autres types : 10 au-dessus de 20 et 30, ou une date en tête
// d'une liste mixte.
func typedHeader(rows [][]string) bool {
	if len(rows) < 2 {
		return true
	}
	seen := false
	for _, first := range rows[0] {
		f := Infer(first)
		if f == "" {
			continue
		}
		if f == grid.Text && !isTimeOrPercent(first) {
			return true
		}
		seen = true
	}
	return !seen
}

// isTimeOrPercent : une heure (10:30) ou un pourcentage (15 %), valeurs
// qu'une cellule libre reconnaît mais que Infer laisse en texte.
func isTimeOrPercent(s string) bool {
	if _, ok := freeTime(s); ok {
		return true
	}
	_, ok := freePercent(s)
	return ok
}

// tableName : le seul tableau d'une feuille prend le nom de la feuille, les
// autres « TableauN ». Comme dans Excel, un nom de tableau
// est un identifiant (sans espace), pour s'écrire tel quel dans une formule.

// identifier rend un nom utilisable dans une formule (« Feuille 1 » →
// « Feuille1 ») ; vide s'il ne reste rien.
func identifier(s string) string {
	var b strings.Builder
	for i, r := range s {
		switch {
		case unicode.IsLetter(r) || r == '_':
			b.WriteRune(r)
		case unicode.IsDigit(r) && i > 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (t *tx) tableName(sheet int64, row, col int) (string, error) {
	var sheetName string
	if err := t.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, sheet).Scan(&sheetName); err != nil {
		return "", err
	}
	free := func(n string) (bool, error) {
		var k int
		err := t.row(`SELECT count(*) FROM _lattis_tables WHERE name = ?`, n).Scan(&k)
		return k == 0, err
	}
	// Le nom de l'onglet va au premier tableau de la feuille, seul tableau.
	var others int
	if err := t.row(`SELECT count(*) FROM _lattis_tables WHERE sheet = ?`, sheet).Scan(&others); err != nil {
		return "", err
	}
	if others == 0 {
		// Une feuille nommée comme une adresse (« Q1 ») ne donne pas son nom.
		if name := identifier(sheetName); name != "" && !A1Like(name) {
			if ok, err := free(name); ok || err != nil {
				return name, err
			}
		}
	}
	for n := 1; ; n++ {
		name := fmt.Sprintf("Tableau%d", n)
		if ok, err := free(name); ok || err != nil {
			return name, err
		}
	}
}

// --- Actions sur un tableau désigné par son nom ------------------------------

// InsertRow insère une ligne vide avant la ligne de données n (n = nombre de
// lignes pour ajouter à la fin).
func (m *Model) InsertRow(ctx context.Context, table string, n int) error {
	return m.Do(ctx, "Insérer une ligne", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		if tb.Filtered() {
			return ErrFiltered{Table: tb.Name}
		}
		if n < 0 || n > tb.Rows {
			return errors.New("canvas: ligne hors du tableau")
		}
		var before, after string
		if n > 0 {
			if _, before, err = t.rowID(tb, n-1); err != nil {
				return err
			}
		}
		if n < tb.Rows {
			if _, after, err = t.rowID(tb, n); err != nil {
				return err
			}
		}
		pos, err := t.rowKey(tb, before, after)
		if err != nil {
			return err
		}
		var id int64
		if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM ` + tb.sql()).Scan(&id); err != nil {
			return err
		}
		if err := t.insertRow(tb, id, pos); err != nil {
			return err
		}
		return t.ensureGapBelow(tb.Sheet, tb.ID, tb.Bottom(), tb.Bottom()+1, tb.Col, tb.Right())
	})
}

// DeleteRow supprime la ligne de données n. Le tableau rétrécit ; ce qui est
// dessous ne remonte pas.
func (m *Model) DeleteRow(ctx context.Context, table string, n int) error {
	return m.Do(ctx, "Supprimer une ligne", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		rid, _, err := t.rowID(tb, n)
		if err != nil {
			return err
		}
		return t.deleteRow(tb, rid)
	})
}

// InsertColumn insère une colonne avant la colonne i.
func (m *Model) InsertColumn(ctx context.Context, table string, i int, name string) error {
	return m.Do(ctx, "Insérer une colonne", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		if i < 0 || i > len(tb.Cols) {
			return errors.New("canvas: colonne hors du tableau")
		}
		if name = strings.TrimSpace(name); name != "" && columnNameTaken(tb, 0, name) {
			return errColumnTaken(tb, name)
		}
		_, err = t.newColumn(tb, i, name)
		return err
	})
}

// DeleteColumn supprime la colonne i (un tableau garde au moins une colonne).
func (m *Model) DeleteColumn(ctx context.Context, table string, i int) error {
	return m.Do(ctx, "Supprimer une colonne", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		if i < 0 || i >= len(tb.Cols) || len(tb.Cols) == 1 {
			return errors.New("canvas: colonne impossible à supprimer")
		}
		if err := t.dropColumn(tb, tb.Cols[i], i); err != nil {
			return err
		}
		return t.trimAfterDrop(tb.ID)
	})
}

// FixType fixe le type de la colonne i : il ne sera plus élargi, les saisies
// hors type seront marquées invalides.
func (m *Model) FixType(ctx context.Context, table string, i int) error {
	return m.Do(ctx, "Fixer le type", func(t *tx) error {
		tb, err := t.editTable(table)
		if err != nil {
			return err
		}
		q := `UPDATE _lattis_columns SET fixed = ? WHERE tbl = ? AND id = ?`
		return t.apply([]stmt{st(q, 1, tb.ID, tb.Cols[i].ID)}, []stmt{st(q, tb.Cols[i].Fixed, tb.ID, tb.Cols[i].ID)})
	})
}
