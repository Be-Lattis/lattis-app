package canvas

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/lalamefine/lattis/internal/command"
	"github.com/lalamefine/lattis/internal/formula"
	"github.com/lalamefine/lattis/internal/grid"
)

// Tableaux jointifs.
//
// Décision de Louis : deux tableaux, ou un tableau et une cellule libre, ne
// restent jamais jointifs. Un tableau garde toujours au moins une cellule
// vide à sa gauche, à sa droite et sous lui. Quand une action (saisie,
// collage, déplacement, suppression ou insertion de lignes et de colonnes,
// conversion…) colle quelque chose contre lui, cela le rejoint à la fin de
// l'action, dans la même étape d'annulation, sur les feuilles que l'action a
// touchées seulement :
//
//   - un tableau juste à sa droite, en-têtes sur la même ligne : les deux
//     fusionnent (mergeRight), chaque ligne de données avec celle qui est
//     sur la même ligne de la feuille ; le plus gros garde son nom (à
//     égalité, celui de gauche). En-têtes sur des lignes différentes, la
//     fusion mélangerait un en-tête et des données : le tableau de droite
//     s'écarte d'une colonne ;
//   - un tableau juste dessous (son en-tête sur la ligne qui suit la
//     dernière) : il descend d'une ligne, pour que son en-tête ne se lise pas
//     comme une ligne du premier. Pas de fusion verticale, même à colonnes
//     identiques : l'en-tête du second disparaîtrait et ce qui le citait
//     (SOMME(B[Age])) lirait aussi les lignes du premier ;
//   - une cellule libre juste à droite ou à gauche (lignes du tableau) :
//     elle devient une nouvelle colonne (sur la ligne d'en-tête : son nom) ;
//   - une cellule libre juste dessous (colonnes du tableau) : une nouvelle
//     ligne.
//
// La cellule libre rejoint le tableau comme si elle avait été tapée là
// (input), avec sa mise en forme ; une formule y entre par sa valeur (elle
// ne devient pas une colonne calculée de tout le tableau) ; les formules qui
// la citaient (Cell[G1], ou son nom si elle en avait un) citent la cellule du
// tableau (Tableau[Col][n]).
//
// Exception : au-dessus du tableau, une cellule libre reste libre (c'est un
// titre), même collée à l'en-tête. Mais une ligne d'en-têtes posée sur plusieurs
// tableaux, ou plus large que le tableau, en fait un seul tableau (headerOver).
//
// Un tableau filtré, ou chargé par une requête d'import (que l'actualisation
// remplace), ne grandit ni ne fusionne de lui-même : un tableau qui le touche
// s'écarte, une cellule qui le touche reste libre ; un message le dit.

// layoutRe reconnaît une instruction qui peut changer la disposition d'une
// feuille : ancres et colonnes des tableaux, cellules libres, lignes ajoutées
// ou retirées d'un tableau.
var layoutRe = regexp.MustCompile(`(?is)^\s*(INSERT\s+INTO|DELETE\s+FROM|ALTER\s+TABLE)\s+t_\d+\b|_lattis_(tables|cells|columns|range_cells)\b`)

// staticRe : instructions sur ces tables qui ne changent pas la disposition
// (texte d'une cellule, nom, formule, type ou règles d'une colonne, nom,
// filtre ou clé d'un tableau), par exemple les formules réécrites sur
// d'autres feuilles après un renommage.
var staticRe = regexp.MustCompile(`(?is)^\s*UPDATE\s+_lattis_(cells\s+SET\s+raw\s*=\s*\?\s+WHERE|columns\s+SET\s+(name|formula|rules|type|fixed)\s*=|tables\s+SET\s+(name|filter|key_col)\s*=)`)

var (
	tableStmtRe  = regexp.MustCompile(`(?is)^\s*(?:INSERT\s+INTO|DELETE\s+FROM|ALTER\s+TABLE)\s+t_(\d+)\b`)
	sheetArgRe   = regexp.MustCompile(`(?i)\bsheet\s*=\s*\?`)
	tblArgRe     = regexp.MustCompile(`(?i)\btbl\s*=\s*\?`)
	idArgRe      = regexp.MustCompile(`(?i)\bid\s*=\s*\?`)
	insertColsRe = regexp.MustCompile(`(?is)^\s*INSERT\s+(?:OR\s+\w+\s+)?INTO\s+_lattis_\w+\s*\(([^)]*)\)\s*VALUES`)
)

func (t *tx) markLayout(do []stmt) {
	for _, s := range do {
		if !layoutRe.MatchString(s.Q) || staticRe.MatchString(s.Q) {
			continue
		}
		t.layout = true
		if !t.layoutAll && !t.layoutTarget(s) {
			t.layoutAll = true
		}
	}
}

// layoutTarget relève la feuille (ou le tableau) qu'une instruction de
// disposition touche ; faux si elle ne se lit pas (toutes les feuilles
// seront examinées).
func (t *tx) layoutTarget(s stmt) bool {
	if t.layoutSheets == nil {
		t.layoutSheets, t.layoutTables = map[int64]bool{}, map[int64]bool{}
	}
	if m := tableStmtRe.FindStringSubmatch(s.Q); m != nil {
		var id int64
		fmt.Sscan(m[1], &id)
		t.layoutTables[id] = true
		return true
	}
	if m := insertColsRe.FindStringSubmatch(s.Q); m != nil {
		cols := strings.Split(m[1], ",")
		for i, c := range cols {
			c = strings.TrimSpace(c)
			if c != "sheet" && c != "tbl" && !(c == "id" && strings.Contains(s.Q, "_lattis_tables")) {
				continue
			}
			found := false
			for k := i; k < len(s.Args); k += len(cols) {
				if v, ok := asInt(s.Args[k]); ok {
					found = true
					if c == "sheet" {
						t.layoutSheets[v] = true
					} else {
						t.layoutTables[v] = true
					}
				}
			}
			if found {
				return true
			}
		}
		return false
	}
	if strings.Contains(strings.ToUpper(s.Q), "SELECT") && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(s.Q)), "INSERT") {
		return false
	}
	for _, x := range []struct {
		re    *regexp.Regexp
		sheet bool
	}{{sheetArgRe, true}, {tblArgRe, false}, {idArgRe, false}} {
		loc := x.re.FindStringIndex(s.Q)
		if loc == nil {
			continue
		}
		if x.re == idArgRe && !strings.Contains(s.Q, "_lattis_tables") {
			continue
		}
		k := strings.Count(s.Q[:loc[1]], "?") - 1
		if k >= len(s.Args) {
			return false
		}
		v, ok := asInt(s.Args[k])
		if !ok {
			return false
		}
		if x.sheet {
			t.layoutSheets[v] = true
		} else {
			t.layoutTables[v] = true
		}
		return true
	}
	return false
}

func asInt(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int64:
		return x, true
	case int32:
		return int64(x), true
	}
	return 0, false
}

// Apply et ApplyStep relèvent au passage les changements de disposition.
func (t *tx) Apply(do, undo []stmt) error {
	t.markLayout(do)
	return t.Tx.Apply(do, undo)
}

func (t *tx) ApplyStep(s command.Step) error {
	t.markLayout(s.Do)
	return t.Tx.ApplyStep(s)
}

// run exécute une action puis rattache ce qu'elle a rendu jointif.
func (m *Model) run(c *command.Tx, fn func(*tx) error) (*tx, error) {
	t := &tx{Tx: c, m: m}
	if err := fn(t); err != nil {
		return t, err
	}
	return t, t.joinAdjacent()
}

// note retient un message pour la barre d'état (voir TakeNotice).
func (m *Model) note(notes []string) {
	if len(notes) == 0 {
		return
	}
	m.noticeMu.Lock()
	m.notice = strings.Join(notes, " ; ")
	m.noticeMu.Unlock()
}

// TakeNotice rend (et oublie) le message de la dernière action, s'il y en a
// un : tableaux fusionnés, cellules rattachées à un tableau.
func (m *Model) TakeNotice() string {
	m.noticeMu.Lock()
	defer m.noticeMu.Unlock()
	s := m.notice
	m.notice = ""
	return s
}

// joinAdjacent rattache, sur chaque feuille non protégée que l'action a
// touchée, ce qui touche un tableau ; seulement si l'action a changé la
// disposition.
func (t *tx) joinAdjacent() error {
	if !t.layout || t.joining || t.ro {
		return nil
	}
	t.joining = true
	defer func() { t.joining = false }()
	rs, err := t.query(`SELECT id FROM _lattis_sheets ORDER BY id`)
	if err != nil {
		return err
	}
	var sheets []int64
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			rs.Close()
			return err
		}
		sheets = append(sheets, id)
	}
	rs.Close()
	touched := map[int64]bool{}
	for id := range t.layoutSheets {
		touched[id] = true
	}
	for id := range t.layoutTables {
		var sh int64
		if err := t.row(`SELECT sheet FROM _lattis_tables WHERE id = ?`, id).Scan(&sh); err == nil {
			touched[sh] = true
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	for _, sh := range sheets {
		if !t.layoutAll && !touched[sh] || t.guard(sh) != nil {
			continue
		}
		if err := t.joinSheet(sh); err != nil {
			return err
		}
	}
	return nil
}

func (t *tx) joinSheet(sheet int64) error {
	type joined struct {
		first string // première cellule rattachée
		n     int
	}
	absorbed := map[string]*joined{} // tableau → cellules rattachées
	var order []string
	defer func() {
		var notes []string
		for _, name := range order {
			j := absorbed[name]
			what := "« " + clip(j.first) + " » rattaché"
			if j.n > 1 {
				what = fmt.Sprintf("%d cellules (« %s »…) rattachées", j.n, clip(j.first))
			}
			notes = append(notes, fmt.Sprintf("%s au tableau « %s » (Ctrl+Z pour annuler)", what, name))
		}
		t.notes = append(t.notes, notes...)
	}()
	for range 10000 {
		done, err := t.joinStep(sheet, func(tb Table, raw string, n int) {
			j := absorbed[tb.Name]
			if j == nil {
				j = &joined{first: raw}
				absorbed[tb.Name] = j
				order = append(order, tb.Name)
			}
			j.n += n
		})
		if err != nil || done {
			return err
		}
	}
	return fmt.Errorf("%w : impossible de rattacher les cellules voisines des tableaux", ErrTable)
}

// joinStep rattache (ou écarte) une seule chose collée à un tableau de la
// feuille ; done quand il n'y a plus rien.
func (t *tx) joinStep(sheet int64, absorbed func(tb Table, first string, n int)) (done bool, err error) {
	tables, err := t.tables(sheet)
	if err != nil || len(tables) == 0 {
		return true, err
	}
	fixed, err := t.fixedTables(tables)
	if err != nil {
		return false, err
	}
	// Deux tableaux côte à côte.
	for _, a := range tables {
		for _, b := range tables {
			if a.ID == b.ID || a.Right()+1 != b.Col || a.Top() > b.Bottom() || b.Top() > a.Bottom() {
				continue
			}
			switch {
			case fixed[a.ID] != "" || fixed[b.ID] != "":
				return false, t.separate(a, b, false, fixedWhy(a, b, fixed))
			case a.Row != b.Row:
				return false, t.separate(a, b, false, "en-têtes sur des lignes différentes")
			case a.TwoLevel != b.TwoLevel:
				return false, t.separate(a, b, false, "en-têtes sur deux lignes d'un seul côté")
			}
			return false, t.joinTables(a, b)
		}
	}
	// Deux tableaux l'un sur l'autre : l'en-tête du second se lirait comme
	// une ligne du premier.
	for _, a := range tables {
		for _, b := range tables {
			if a.ID == b.ID || b.Top() != a.Bottom()+1 || a.Col > b.Right() || b.Col > a.Right() {
				continue
			}
			why := "deux tableaux empilés ne fusionnent pas"
			if fixed[a.ID] != "" || fixed[b.ID] != "" {
				why = fixedWhy(a, b, fixed)
			}
			return false, t.separate(a, b, true, why)
		}
	}
	cells, err := t.cells(sheet)
	if err != nil {
		return false, err
	}
	// Les cellules d'une plage (tableau converti) restent libres.
	marks, err := t.rangeMarks(sheet)
	if err != nil {
		return false, err
	}
	if len(marks) > 0 {
		cells = slices.DeleteFunc(cells, func(c cell) bool { return marks[[2]int{c.Row, c.Col}] })
	}
	// Les cellules gardées hors des tableaux (notes, totaux) aussi.
	outs, err := t.outCells(sheet)
	if err != nil {
		return false, err
	}
	if len(outs) > 0 {
		cells = slices.DeleteFunc(cells, func(c cell) bool { return outs[[2]int{c.Row, c.Col}] })
	}
	// Rien de libre dans la bande des groupes d'un tableau : une cellule qui
	// y arrive (collage, déplacement) devient le groupe de sa colonne.
	if ok, err := t.absorbBand(sheet, tables); ok || err != nil {
		return false, err
	}
	// Une ligne d'en-têtes posée sur des tableaux : un seul tableau.
	if ok, err := t.headerOver(sheet, tables, cells, fixed); ok || err != nil {
		return false, err
	}
	for _, tb := range tables {
		if fixed[tb.ID] != "" {
			continue
		}
		c, ok := touching(tb, cells)
		if !ok {
			continue
		}
		n, err := t.absorbCell(sheet, tb, c, len(cells))
		if err != nil {
			return false, err
		}
		if cur, err := t.table(tb.ID); err == nil {
			tb = cur
		}
		absorbed(tb, c.Raw, n)
		return false, nil
	}
	// Une cellule contre un tableau qui ne grandit pas : elle reste libre,
	// et on le dit.
	for _, tb := range tables {
		if fixed[tb.ID] == "" {
			continue
		}
		if c, ok := touching(tb, cells); ok {
			t.notes = append(t.notes, fmt.Sprintf("« %s » reste à côté du tableau « %s » sans le rejoindre : %s", clip(c.Raw), tb.Name, fixed[tb.ID]))
		}
	}
	return true, nil
}

func fixedWhy(a, b Table, fixed map[int64]string) string {
	if w := fixed[a.ID]; w != "" {
		return fmt.Sprintf("« %s » %s", a.Name, strings.TrimPrefix(w, "il "))
	}
	return fmt.Sprintf("« %s » %s", b.Name, strings.TrimPrefix(fixed[b.ID], "il "))
}

// separate écarte b de a, contre lequel il est collé : d'une colonne vers la
// droite (côte à côte) ou d'une ligne vers le bas (dessous), avec ce qui le
// suit.
func (t *tx) separate(a, b Table, below bool, why string) error {
	if below {
		if err := t.ensureGapBelow(b.Sheet, b.ID, b.Bottom(), b.Bottom()+1, b.Col, b.Right()); err != nil {
			return err
		}
		if err := t.moveTable(b, 1, 0); err != nil {
			return err
		}
		t.notes = append(t.notes, fmt.Sprintf("Tableau « %s » descendu d'une ligne, sous « %s » sans le rejoindre : %s", b.Name, a.Name, why))
		return nil
	}
	if err := t.ensureGapRight(b.Sheet, b.ID, b.Right(), b.Right()+1, b.Top(), b.Bottom()); err != nil {
		return err
	}
	if err := t.moveTable(b, 0, 1); err != nil {
		return err
	}
	t.notes = append(t.notes, fmt.Sprintf("Tableau « %s » décalé d'une colonne, à côté de « %s » sans le rejoindre : %s", b.Name, a.Name, why))
	return nil
}

// absorbCell fait entrer la cellule libre c dans le tableau tb qu'elle
// touche ; rend le nombre de cellules libres parties avec elle.
func (t *tx) absorbCell(sheet int64, tb Table, c cell, before int) (int, error) {
	raw := c.Raw
	if isFormula(raw) {
		// Une formule entre par sa valeur : dans le tableau, elle ferait une
		// colonne calculée, la même pour toutes les lignes.
		schema, err := t.schema()
		if err != nil {
			return 0, err
		}
		values, err := t.nameValuesWith(schema, true)
		if err != nil {
			return 0, err
		}
		raw = values[formula.CellKey(sheet, c.Row, c.Col)].String()
		if raw != "" && isFormula(raw) {
			raw = "'" + raw
		}
		t.notes = append(t.notes, fmt.Sprintf("La formule « %s » est entrée dans le tableau « %s » par sa valeur", clip(c.Raw), tb.Name))
	}
	if err := t.setFreeCell(sheet, c.Row, c.Col, ""); err != nil {
		return 0, err
	}
	if strings.TrimSpace(raw) != "" {
		var ev Event
		if err := t.input(sheet, c.Row, c.Col, raw, &ev); err != nil {
			return 0, err
		}
	}
	cur, err := t.table(tb.ID)
	if err != nil {
		return 0, err
	}
	if err := t.adoptFree(cur); err != nil {
		return 0, err
	}
	// La saisie a pu emmener d'autres cellules de la même colonne ou ligne.
	after, err := t.cells(sheet)
	if err != nil {
		return 0, err
	}
	return max(1, before-len(after)), nil
}

// adoptFree fait suivre au tableau ce qui accompagnait les cellules libres
// qu'il vient de recouvrir : leur mise en forme, les formules qui les
// citaient par leur adresse ou par leur nom (Tableau[Col][n]) ; un contrôle
// lié à l'une d'elles est signalé.
func (t *tx) adoptFree(tb Table) error {
	in := `sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`
	area := []any{tb.Sheet, tb.Row, tb.Bottom(), tb.Col, tb.Right()}
	// Mise en forme.
	rs, err := t.query(`SELECT row, col, style FROM _lattis_cell_styles WHERE `+in, area...)
	if err != nil {
		return err
	}
	type styled struct {
		row, col int
		style    string
	}
	var ss []styled
	for rs.Next() {
		var s styled
		if err := rs.Scan(&s.row, &s.col, &s.style); err != nil {
			rs.Close()
			return err
		}
		ss = append(ss, s)
	}
	rs.Close()
	for _, s := range ss {
		rid := int64(-1) // en-tête
		if s.row > tb.Row {
			ids, err := t.ridsAt(&tb, s.row, s.row)
			if err != nil || len(ids) == 0 {
				continue
			}
			rid = ids[0]
		}
		if err := t.putTableStyle(tb.ID, rid, tb.Cols[s.col-tb.Col].ID, s.style); err != nil {
			return err
		}
	}
	if len(ss) > 0 {
		if err := t.deleteWhere(`_lattis_cell_styles`, in, area...); err != nil {
			return err
		}
	}
	// Adresses Cell[G1] des données : Tableau[Col][n].
	var sheetName string
	if err := t.row(`SELECT name FROM _lattis_sheets WHERE id = ?`, tb.Sheet).Scan(&sheetName); err != nil {
		return err
	}
	headers := make([]string, len(tb.Cols))
	for i, c := range tb.Cols {
		headers[i] = c.Name
	}
	zone := Rect{R0: tb.Row, C0: tb.Col, R1: tb.Bottom(), C1: tb.Right()}
	if err := t.cellsToTable(tb.Sheet, sheetName, zone, tb.Name, headers); err != nil {
		return err
	}
	// Cellules nommées : le nom laisse la place à Tableau[Col][n].
	rs, err = t.query(`SELECT name, row, col FROM _lattis_names WHERE `+in, area...)
	if err != nil {
		return err
	}
	type named struct {
		name     string
		row, col int
	}
	var ns []named
	for rs.Next() {
		var n named
		if err := rs.Scan(&n.name, &n.row, &n.col); err != nil {
			rs.Close()
			return err
		}
		ns = append(ns, n)
	}
	rs.Close()
	for _, n := range ns {
		ref, ok := tableRef(zone, tb.Name, headers, formula.ColumnLetters(n.col)+fmt.Sprint(n.row+1))
		if !ok {
			t.notes = append(t.notes, fmt.Sprintf("La cellule nommée « %s » est l'en-tête du tableau « %s » : son nom ne désigne plus rien", n.name, tb.Name))
			continue
		}
		if err := t.rewriteRefs(refChange{Renames: formula.Renames{Names: lowerMap(n.name, ref)}}); err != nil {
			return err
		}
		if err := t.deleteWhere(`_lattis_names`, `name = ?`, n.name); err != nil {
			return err
		}
		t.notes = append(t.notes, fmt.Sprintf("La cellule nommée « %s » a rejoint le tableau « %s » : les formules lisent désormais %s", n.name, tb.Name, ref))
	}
	// Contrôles liés : signalés.
	cfgs, ids, err := t.configs(`_lattis_buttons`)
	if err != nil {
		return err
	}
	for _, id := range ids {
		var bc ButtonConfig
		if json.Unmarshal([]byte(cfgs[id]), &bc) != nil || bc.Link == "" {
			continue
		}
		var bsheet int64
		if err := t.row(`SELECT sheet FROM _lattis_buttons WHERE id = ?`, id).Scan(&bsheet); err != nil {
			return err
		}
		sh, addr := SplitLink(bc.Link)
		if sh != "" && !strings.EqualFold(sh, sheetName) || sh == "" && bsheet != tb.Sheet {
			continue
		}
		r, c, ok := formula.CellRef("Cell_" + addr)
		if ok && r >= tb.Row && r <= tb.Bottom() && c >= tb.Col && c <= tb.Right() {
			t.notes = append(t.notes, fmt.Sprintf("Le contrôle « %s » est lié à %s, qui a rejoint le tableau « %s »", bc.Name, addr, tb.Name))
		}
	}
	return nil
}

func clip(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 20 {
		return string(r[:19]) + "…"
	}
	return s
}

// touching donne une cellule libre collée au tableau (à droite ou à gauche
// sur ses lignes, dessous sur ses colonnes) : celle de la ligne d'en-tête
// d'abord, pour qu'elle nomme la nouvelle colonne. Au-dessus du tableau, une
// cellule reste libre (un titre) : c'est l'exception à la règle.
func touching(tb Table, cells []cell) (cell, bool) {
	var best cell
	found := false
	for _, c := range cells {
		side := (c.Col == tb.Right()+1 || c.Col == tb.Col-1) && c.Row >= tb.Row && c.Row <= tb.Bottom()
		below := c.Row == tb.Bottom()+1 && c.Col >= tb.Col && c.Col <= tb.Right()
		if !side && !below || strings.TrimSpace(c.Raw) == "" {
			continue
		}
		if !found || c.Row < best.Row || c.Row == best.Row && c.Col < best.Col {
			best, found = c, true
		}
	}
	return best, found
}

// fixedTables : tableaux qui ne grandissent ni ne fusionnent d'eux-mêmes
// (filtrés, ou résultats d'une requête d'import, que l'actualisation
// remplace), avec la raison.
func (t *tx) fixedTables(tables []Table) (map[int64]string, error) {
	out := map[int64]string{}
	for _, tb := range tables {
		if tb.Filtered() {
			out[tb.ID] = "il est filtré (effacez le filtre pour l'agrandir)"
		}
	}
	rs, err := t.query(`SELECT target FROM _lattis_queries WHERE target IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = "il est chargé par une requête d'import"
	}
	return out, rs.Err()
}

// joinTables fusionne b, collé à droite de a : le plus gros des deux garde
// son nom (à égalité, a).
func (t *tx) joinTables(a, b Table) error {
	bigger := b.Rows*len(b.Cols) > a.Rows*len(a.Cols)
	merged, err := t.mergeRight(a, b)
	if err != nil {
		return err
	}
	if bigger {
		if err := t.rewriteRefs(tableChange(merged.Name, b.Name)); err != nil {
			return err
		}
		q := `UPDATE _lattis_tables SET name = ? WHERE id = ?`
		if err := t.apply([]stmt{st(q, b.Name, merged.ID)}, []stmt{st(q, merged.Name, merged.ID)}); err != nil {
			return err
		}
	}
	t.notes = append(t.notes, fmt.Sprintf("Tableaux « %s » et « %s » fusionnés (Ctrl+Z pour annuler)", a.Name, b.Name))
	return nil
}

// growSide : une saisie juste à gauche (left) ou juste à droite d'un
// tableau, sur ses lignes, y ajoute une colonne (sur la ligne d'en-tête :
// son nom, sinon « Colonne n », n sa place). À gauche, le tableau
// s'ancre une colonne plus à gauche et ses autres colonnes ne bougent pas ;
// à droite, ce qui suit s'écarte. Les autres cellules libres de cette
// colonne, sur les lignes du tableau, le rejoignent aussi.
func (t *tx) growSide(tb Table, row, col int, raw string, ev *Event, left bool) error {
	if tb.Filtered() && row != tb.Row {
		return ErrFiltered{Table: tb.Name, Column: true}
	}
	taken, err := t.takeFree(tb.Sheet, func(c cell) bool {
		return c.Col == col && c.Row >= tb.Row && c.Row <= tb.Bottom() && c.Row != row
	})
	if err != nil {
		return err
	}
	name := ""
	if row == tb.Row {
		name = strings.TrimSpace(raw)
	}
	for _, c := range taken {
		if c.Row == tb.Row && strings.TrimSpace(c.Raw) != "" {
			name = strings.TrimSpace(c.Raw)
		}
	}
	if name == "" {
		// Le nom dit sa place : « Colonne 1 » à gauche, « Colonne n+1 » à
		// droite (numéroté s'il est pris).
		at := len(tb.Cols) + 1
		if left {
			at = 1
		}
		name = fmt.Sprintf("Colonne %d", at)
	}
	name = freeColumnName(tb, name)
	i := len(tb.Cols)
	if left {
		var id int64
		if err := t.row(`SELECT coalesce(max(id), 0) + 1 FROM _lattis_columns WHERE tbl = ?`, tb.ID).Scan(&id); err != nil {
			return err
		}
		if err := t.addColumn(tb, id, name, grid.Text, 0); err != nil {
			return err
		}
		if err := t.moveTable(tb, 0, -1); err != nil {
			return err
		}
		i = 0
	} else if _, err := t.newColumn(tb, i, name); err != nil {
		return err
	}
	if tb, err = t.table(tb.ID); err != nil {
		return err
	}
	for _, c := range taken {
		if c.Row == tb.Row {
			continue
		}
		if err := t.setCell(tb, c.Row-tb.Row-1, i, c.Raw, ev); err != nil {
			return err
		}
		if tb, err = t.table(tb.ID); err != nil {
			return err
		}
	}
	if row == tb.Row {
		return nil
	}
	return t.setCell(tb, row-tb.Row-1, i, raw, ev)
}

// takeFree retire de la feuille les cellules libres retenues par keep et
// les rend (pour les saisir dans un tableau qui s'agrandit).
func (t *tx) takeFree(sheet int64, keep func(cell) bool) ([]cell, error) {
	cells, err := t.cells(sheet)
	if err != nil {
		return nil, err
	}
	var out []cell
	for _, c := range cells {
		if !keep(c) || strings.TrimSpace(c.Raw) == "" {
			continue
		}
		if err := t.setFreeCell(sheet, c.Row, c.Col, ""); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// inAny indique si la cellule (row, col) est dans un des tableaux.
func inAny(tables []Table, row, col int) bool {
	for _, tb := range tables {
		if row >= tb.Row && row <= tb.Bottom() && col >= tb.Col && col <= tb.Right() {
			return true
		}
	}
	return false
}

// freeColumnName rend name, suivi d'un numéro s'il est déjà pris dans tb.
func freeColumnName(tb Table, name string) string {
	taken := map[string]bool{}
	for _, c := range tb.Cols {
		taken[strings.ToLower(c.Name)] = true
	}
	out := name
	for n := 2; taken[strings.ToLower(out)]; n++ {
		out = fmt.Sprintf("%s %d", name, n)
	}
	return out
}

// headerOver : une ligne de cellules libres posée juste au-dessus de
// tableaux (une ligne d'en-têtes descendue sur eux). Quand elle les couvre
// en entier et dépasse un seul tableau (plusieurs tableaux, ou des colonnes
// de plus, vides dessous), elle devient l'en-tête d'un seul tableau : les
// en-têtes actuels redeviennent des lignes de données, les colonnes vides
// entre les tableaux en font partie. Les formules qui citaient les tableaux
// citent le nouveau (passage par une plage, comme Convertir en plage puis
// Mettre sous forme de tableau). Une seule cellule, ou une ligne pas plus
// large qu'un tableau, reste un titre.
func (t *tx) headerOver(sheet int64, tables []Table, cells []cell, fixed map[int64]string) (bool, error) {
	at := map[[2]int]cell{}
	for _, c := range cells {
		if strings.TrimSpace(c.Raw) != "" {
			at[[2]int{c.Row, c.Col}] = c
		}
	}
	for _, c := range cells {
		// Début d'une suite de cellules remplies sur la ligne.
		if _, ok := at[[2]int{c.Row, c.Col}]; !ok {
			continue
		}
		if _, left := at[[2]int{c.Row, c.Col - 1}]; left {
			continue
		}
		r, c0, c1 := c.Row, c.Col, c.Col
		formulas := false
		for {
			x, ok := at[[2]int{r, c1 + 1}]
			if !ok {
				break
			}
			c1++
			formulas = formulas || isFormula(x.Raw)
		}
		if c1 == c0 || formulas || isFormula(c.Raw) || inAny(tables, r, c0) {
			continue
		}
		var under []Table
		width, bottom, ok := 0, r, true
		for _, tb := range tables {
			if tb.Row != r+1 || tb.Right() < c0 || tb.Col > c1 {
				continue
			}
			if tb.Col < c0 || tb.Right() > c1 || fixed[tb.ID] != "" || tb.TwoLevel {
				ok = false
				break
			}
			under = append(under, tb)
			width += len(tb.Cols)
			bottom = max(bottom, tb.Bottom())
		}
		if !ok || len(under) == 0 || len(under) == 1 && width == c1-c0+1 {
			continue
		}
		// Sous la ligne, hors des tableaux : rien (ni cellule, ni autre tableau).
		for _, x := range cells {
			if x.Row > r && x.Row <= bottom && x.Col >= c0 && x.Col <= c1 && strings.TrimSpace(x.Raw) != "" {
				ok = false
			}
		}
		for _, tb := range tables {
			if tb.Row > r+1 && tb.Row <= bottom && tb.Col <= c1 && tb.Right() >= c0 {
				ok = false
			}
		}
		if !ok {
			continue
		}
		names := make([]string, len(under))
		for i, tb := range under {
			names[i] = "« " + tb.Name + " »"
			if err := t.toRange(tb); err != nil {
				return false, err
			}
		}
		name, err := t.formatAsTable(sheet, r+1, under[0].Col)
		if err != nil {
			return false, err
		}
		// La note de Mettre sous forme de tableau est remplacée par celle-ci.
		if n := len(t.notes); n > 0 && strings.HasPrefix(t.notes[n-1], "Plage mise sous forme") {
			t.notes = t.notes[:n-1]
		}
		t.notes = append(t.notes, fmt.Sprintf("Ligne d'en-têtes posée sur %s : un seul tableau « %s », leurs en-têtes redeviennent des données (Ctrl+Z pour annuler)", strings.Join(names, ", "), name))
		return true, nil
	}
	return false, nil
}

// rightOfAny : la cellule (row, col) est juste à droite d'un autre tableau
// que tb, sur ses lignes ; une saisie là l'agrandit, lui, pas tb.
func rightOfAny(tables []Table, tb Table, row, col int) bool {
	for _, o := range tables {
		if o.ID != tb.ID && o.Right()+1 == col && row >= o.Row && row <= o.Bottom() {
			return true
		}
	}
	return false
}
