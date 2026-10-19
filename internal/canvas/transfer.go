package canvas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Rect est un rectangle de cellules d'une feuille, bornes comprises.
type Rect struct {
	R0 int `json:"r0"`
	C0 int `json:"c0"`
	R1 int `json:"r1"`
	C1 int `json:"c1"`
}

func (r Rect) height() int { return r.R1 - r.R0 + 1 }
func (r Rect) width() int  { return r.C1 - r.C0 + 1 }

func (r Rect) has(row, col int) bool {
	return row >= r.R0 && row <= r.R1 && col >= r.C0 && col <= r.C1
}

// holds : le tableau est entièrement dans le rectangle.
func (r Rect) holds(tb Table) bool {
	return tb.Row >= r.R0 && tb.Bottom() <= r.R1 && tb.Col >= r.C0 && tb.Right() <= r.C1
}

// meets : le rectangle recoupe le tableau.
func (r Rect) meets(tb Table) bool {
	return tb.Row <= r.R1 && tb.Bottom() >= r.R0 && tb.Col <= r.C1 && tb.Right() >= r.C0
}

// tiles donne le nombre de répétitions (lignes, colonnes) d'un bloc h × w
// collé sur la sélection dst, comme Excel : une sélection dont chaque côté
// est un multiple du bloc est remplie par le motif répété (une seule cellule
// copiée remplit toute la sélection) ; sinon le bloc est collé une fois, à
// partir du coin haut gauche.
func tiles(h, w int, dst Rect) (int, int) {
	dh, dw := dst.height(), dst.width()
	if h <= 0 || w <= 0 || dh < h || dw < w || dh%h != 0 || dw%w != 0 {
		return 1, 1
	}
	return dh / h, dw / w
}

// piece est une cellule transportée par un copier-coller ou un déplacement.
type piece struct {
	dr, dc int    // position dans le bloc
	raw    string // saisie d'une cellule libre (formule comprise, Cell[…] inchangées)
	text   string // valeur affichée : ce qui entre dans un tableau, ou valeur de tableau
	style  string // mise en forme (JSON), notes et fusions comprises
	name   string // nom de la cellule (déplacement seulement)
	free   bool   // cellule libre d'origine (sinon : cellule de tableau, transportée en valeur)
}

// subTable : colonnes i0..i1 d'un tableau, en-tête et toutes les lignes
// comprises dans la source (sans être tout le tableau).
type subTable struct {
	tb     Table
	i0, i1 int
}

// payload est le contenu d'une zone source.
type payload struct {
	whole  []Table
	subs   []subTable
	pieces []piece
	// Cellules de tableau à vider (déplacement) : tableau, ligne, colonne.
	partial []partialCell
}

type partialCell struct {
	tb     Table
	rid    int64 // 0 : en-tête
	i      int
	header bool
}

// Transfer copie (move faux) ou déplace le contenu du rectangle src de la
// feuille from vers la feuille to, coin haut gauche en (dst.R0, dst.C0),
// comme le copier-coller et le glisser de la bordure de la sélection
// d'Excel, en une seule étape d'annulation :
//
//   - un tableau entièrement compris est recopié (types, colonnes calculées,
//     clé, mise en forme) ou déplacé ; posé côte à côte d'un autre il
//     fusionne, et ce qu'il chevauche s'écarte vers la droite (settle) ;
//   - des colonnes entières d'un tableau (en-tête et toutes les lignes) font
//     un nouveau tableau ;
//   - les cellules libres emportent leur saisie (formules telles quelles :
//     les adresses Cell[…] sont absolues), leur mise en forme, notes et
//     fusions ; déplacées, leur nom et les adresses qui les visaient suivent ;
//   - les autres cellules de tableau sont transportées en valeur, mise en
//     forme comprise ; déplacées, la source est vidée.
//
// À la destination, une valeur posée dans un tableau le remplace cellule
// par cellule, et juste dessous ou à droite elle l'étend, comme une saisie.
// values est le bloc affiché de la source (valeurs des formules) : il donne
// la valeur des colonnes calculées et des formules qui entrent dans un
// tableau. En copie, une sélection dst multiple du bloc le reçoit répété.
func (m *Model) Transfer(ctx context.Context, from int64, src Rect, to int64, dst Rect, move bool, values [][]string) (Event, error) {
	var ev Event
	if src.R0 > src.R1 || src.C0 > src.C1 || dst.R0 < 0 || dst.C0 < 0 {
		return ev, fmt.Errorf("%w : zone hors de la feuille", ErrTable)
	}
	dr, dc := dst.R0-src.R0, dst.C0-src.C0
	if move && from == to && dr == 0 && dc == 0 {
		return ev, nil
	}
	label := "Coller"
	if move {
		label = "Déplacer les cellules"
	}
	err := m.Do(ctx, label, func(t *tx) error {
		if err := t.guard(to); err != nil {
			return err
		}
		if move {
			if err := t.guard(from); err != nil {
				return err
			}
		}
		t.collect = true
		return t.transfer(from, src, to, dst, move, values, &ev)
	})
	return ev, err
}

func (t *tx) transfer(from int64, src Rect, to int64, dst Rect, move bool, values [][]string, ev *Event) error {
	h, w := src.height(), src.width()
	dr, dc := dst.R0-src.R0, dst.C0-src.C0
	nr, nc := 1, 1
	if !move {
		nr, nc = tiles(h, w, dst)
	}
	srcTables, err := t.tables(from)
	if err != nil {
		return err
	}
	// Déplacement de cellules libres seules, loin des tableaux : MoveCells.
	if move && from == to && !slices.ContainsFunc(srcTables, src.meets) {
		target := Rect{src.R0 + dr, src.C0 + dc, src.R1 + dr, src.C1 + dc}
		if !slices.ContainsFunc(srcTables, func(tb Table) bool { return touches(target.R0, target.R1, target.C0, target.C1, tb) }) {
			return t.moveFree(from, src.R0, src.C0, src.R1, src.C1, dr, dc)
		}
	}
	p, err := t.gather(from, src, srcTables, values, move)
	if err != nil {
		return err
	}
	if len(p.pieces)*nr*nc > MaxPasteCells {
		return ErrPasteTooBig{len(p.pieces) * nr * nc}
	}
	if move {
		if err := t.lift(from, src, p); err != nil {
			return err
		}
	}
	for ti := range nr {
		for tj := range nc {
			oR, oC := dr+ti*h, dc+tj*w // décalage de cette répétition
			block := Rect{src.R0 + oR, src.C0 + oC, src.R1 + oR, src.C1 + oC}
			// La destination est remplacée (cellules libres et leur mise en forme).
			for _, tbl := range []string{"_lattis_cells", "_lattis_cell_styles"} {
				if err := t.deleteWhere(tbl, `sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`,
					to, block.R0, block.R1, block.C0, block.C1); err != nil {
					return err
				}
			}
			if err := t.placeTables(p, to, oR, oC, move); err != nil {
				return err
			}
			if err := t.putPieces(p.pieces, to, block.R0, block.C0, move, ev); err != nil {
				return err
			}
		}
	}
	if move && from == to {
		// Les adresses Cell[…] qui visaient les cellules libres déplacées
		// suivent ; celles qui visaient une cellule remplacée : #REF!.
		freeAt := map[[2]int]bool{}
		for _, pc := range p.pieces {
			if pc.free {
				freeAt[[2]int{src.R0 + pc.dr, src.C0 + pc.dc}] = true
			}
		}
		target := Rect{src.R0 + dr, src.C0 + dc, src.R1 + dr, src.C1 + dc}
		return t.rewriteCellRefs(from, func(r, c int) (int, int, bool) {
			switch {
			case freeAt[[2]int{r, c}]:
				return r + dr, c + dc, true
			case target.has(r, c) && !src.has(r, c):
				return 0, 0, false
			}
			return r, c, true
		})
	}
	return nil
}

// gather lit le contenu de la zone src.
func (t *tx) gather(sheet int64, src Rect, tables []Table, values [][]string, move bool) (payload, error) {
	var p payload
	shown := func(r, c int) (string, bool) {
		i, j := r-src.R0, c-src.C0
		if i < len(values) && j < len(values[i]) {
			return values[i][j], true
		}
		return "", false
	}
	inTables := func(r, c int) bool {
		for _, tb := range tables {
			if r >= tb.Row && r <= tb.Bottom() && c >= tb.Col && c <= tb.Right() {
				return true
			}
		}
		return false
	}
	for _, tb := range tables {
		switch {
		case !src.meets(tb):
		case src.holds(tb):
			p.whole = append(p.whole, tb)
		case src.R0 <= tb.Row && src.R1 >= tb.Bottom():
			p.subs = append(p.subs, subTable{tb, max(src.C0, tb.Col) - tb.Col, min(src.C1, tb.Right()) - tb.Col})
		default:
			if err := t.gatherPartial(&p, tb, src, shown); err != nil {
				return p, err
			}
		}
	}
	// Cellules libres : saisie, mise en forme, nom.
	at := map[[2]int]*piece{}
	get := func(r, c int) *piece {
		k := [2]int{r, c}
		if pc, ok := at[k]; ok {
			return pc
		}
		pc := &piece{dr: r - src.R0, dc: c - src.C0, free: true}
		at[k] = pc
		return pc
	}
	in := `sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`
	args := []any{sheet, src.R0, src.R1, src.C0, src.C1}
	for _, q := range []struct {
		sql string
		set func(*piece, string)
	}{
		{`SELECT row, col, raw FROM _lattis_cells WHERE ` + in, func(pc *piece, s string) { pc.raw = s }},
		{`SELECT row, col, style FROM _lattis_cell_styles WHERE ` + in, func(pc *piece, s string) { pc.style = s }},
		{`SELECT row, col, name FROM _lattis_names WHERE ` + in, func(pc *piece, s string) {
			if move {
				pc.name = s
			}
		}},
	} {
		rs, err := t.query(q.sql, args...)
		if err != nil {
			return p, err
		}
		for rs.Next() {
			var r, c int
			var s string
			if err := rs.Scan(&r, &c, &s); err != nil {
				rs.Close()
				return p, err
			}
			if !inTables(r, c) {
				q.set(get(r, c), s)
			}
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return p, err
		}
	}
	for k, pc := range at {
		pc.text = pc.raw
		if isFormula(pc.raw) {
			if v, ok := shown(k[0], k[1]); ok {
				pc.text = v
			}
		}
		p.pieces = append(p.pieces, *pc)
	}
	slices.SortFunc(p.pieces, func(a, b piece) int {
		if a.dr != b.dr {
			return a.dr - b.dr
		}
		return a.dc - b.dc
	})
	return p, nil
}

// gatherPartial lit les cellules d'un tableau coupé par la zone : en-têtes
// et valeurs (le texte tel qu'il se saisit), mise en forme de la colonne et
// de la cellule.
func (t *tx) gatherPartial(p *payload, tb Table, src Rect, shown func(r, c int) (string, bool)) error {
	r0, r1 := max(src.R0, tb.Row), min(src.R1, tb.Bottom())
	c0, c1 := max(src.C0, tb.Col), min(src.C1, tb.Right())
	colStyles := map[int64]Style{}
	headStyles := map[int64]Style{}
	for i := c0 - tb.Col; i <= c1-tb.Col; i++ {
		c := tb.Cols[i]
		cs, err := t.tableStyle(tb.ID, 0, c.ID)
		if err != nil {
			return err
		}
		hs, err := t.tableStyle(tb.ID, -1, c.ID)
		if err != nil {
			return err
		}
		colStyles[c.ID], headStyles[c.ID] = cs, hs
	}
	enc := func(s Style) string {
		for k, v := range s {
			if v == nil {
				delete(s, k)
			}
		}
		out, _ := encodeStyle(s)
		return out
	}
	if r0 == tb.Row {
		for c := c0; c <= c1; c++ {
			col := tb.Cols[c-tb.Col]
			p.pieces = append(p.pieces, piece{dr: r0 - src.R0, dc: c - src.C0, text: col.Name, style: enc(over(headStyles[col.ID]))})
			p.partial = append(p.partial, partialCell{tb: tb, i: c - tb.Col, header: true})
		}
		r0++
	}
	if r0 > r1 {
		return nil
	}
	if tb.Filtered() {
		r1 = min(r1, tb.Row+tb.Shown)
	}
	if r0 > r1 {
		return nil
	}
	sel := []string{"id"}
	var args []any
	for c := c0; c <= c1; c++ {
		col := tb.Cols[c-tb.Col]
		sel = append(sel, fmt.Sprintf(`coalesce(slt_text(%s, ?), (SELECT raw FROM _lattis_invalid i WHERE i.tbl = ? AND i.rid = %s.id AND i.col = ?), '')`,
			colSQL(col.ID), tb.sql()))
		args = append(args, string(col.Type), tb.ID, col.ID)
	}
	args = append(args, r1-r0+1, r0-tb.Row-1)
	rs, err := t.query(`SELECT `+strings.Join(sel, ", ")+` FROM `+tb.view()+` ORDER BY _pos LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return err
	}
	type line struct {
		rid  int64
		vals []string
	}
	var got []line
	for rs.Next() {
		l := line{vals: make([]string, c1-c0+1)}
		ptrs := []any{&l.rid}
		for i := range l.vals {
			ptrs = append(ptrs, &l.vals[i])
		}
		if err := rs.Scan(ptrs...); err != nil {
			rs.Close()
			return err
		}
		got = append(got, l)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return err
	}
	for k, l := range got {
		r := r0 + k
		for c := c0; c <= c1; c++ {
			col := tb.Cols[c-tb.Col]
			text := l.vals[c-c0]
			if col.Formula != "" {
				text, _ = shown(r, c)
			}
			cell, err := t.tableStyle(tb.ID, l.rid, col.ID)
			if err != nil {
				return err
			}
			p.pieces = append(p.pieces, piece{dr: r - src.R0, dc: c - src.C0, text: text, style: enc(over(colStyles[col.ID], cell))})
			if col.Formula == "" {
				p.partial = append(p.partial, partialCell{tb: tb, rid: l.rid, i: c - tb.Col})
			}
		}
	}
	return nil
}

// lift vide la source d'un déplacement : cellules libres (saisie, mise en
// forme, nom) et valeurs des cellules de tableau coupées (les en-têtes
// redeviennent « Colonne n »). Tableaux et colonnes entières partent avec
// placeTables.
func (t *tx) lift(sheet int64, src Rect, p payload) error {
	for _, tbl := range []string{"_lattis_cells", "_lattis_cell_styles", "_lattis_names"} {
		if err := t.deleteWhere(tbl, `sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`, sheet, src.R0, src.R1, src.C0, src.C1); err != nil {
			return err
		}
	}
	cur := map[int64]Table{}
	// last : dernière ligne affichée de chaque tableau avant la coupe ;
	// reach : la coupe l'a vidée (les lignes vides du bas partent).
	last, reach := map[int64]int64{}, map[int64]bool{}
	for _, pc := range p.partial {
		tb, ok := cur[pc.tb.ID]
		if !ok || pc.header {
			var err error
			if tb, err = t.table(pc.tb.ID); err != nil {
				return err
			}
			cur[tb.ID] = tb
		}
		if _, ok := last[tb.ID]; !ok {
			id, err := t.lastShown(tb)
			if err != nil {
				return err
			}
			last[tb.ID] = id
		}
		if !pc.header && pc.rid == last[tb.ID] {
			reach[tb.ID] = true
		}
		c := tb.Cols[pc.i]
		if pc.header {
			if err := t.rename(tb, c, fmt.Sprintf("Colonne %d", pc.i+1)); err != nil {
				return err
			}
			continue
		}
		if err := t.setValue(tb, pc.rid, c, nil, ""); err != nil {
			return err
		}
		if err := t.putTableStyle(tb.ID, pc.rid, c.ID, ""); err != nil {
			return err
		}
	}
	for id := range cur {
		if err := t.tidyBottom(id, true, reach[id]); err != nil {
			return err
		}
	}
	return nil
}

// defaultColumnName : nom donné d'office à une colonne (jamais renommée).
var defaultColumnName = regexp.MustCompile(`^Colonne \d+$`)

// tidyBottom range un tableau dont des cellules viennent d'être emportées
// ou effacées (choix de Louis). Avec drop (cellules emportées), vidé et
// jamais personnalisé (colonnes « Colonne n », sans formule), il disparaît,
// ce qui libère la place autour. Sinon, avec reach (l'opération a touché sa
// dernière ligne affichée), ses dernières lignes vides partent (trimBottom).
func (t *tx) tidyBottom(id int64, drop, reach bool) error {
	tb, err := t.table(id)
	if err != nil {
		return err
	}
	if drop && !tb.Filtered() && tb.Rows > 0 {
		plain := true
		var conds []string
		for _, c := range tb.Cols {
			if c.Formula != "" || !defaultColumnName.MatchString(c.Name) {
				plain = false
				break
			}
			conds = append(conds, filledSQL(tb, c))
		}
		if plain && len(conds) > 0 {
			var filled int
			if err := t.row(fmt.Sprintf(`SELECT count(*) FROM %s AS d WHERE %s`, tb.sql(), strings.Join(conds, " OR "))).Scan(&filled); err != nil {
				return err
			}
			if filled == 0 {
				return t.deleteTable(tb)
			}
		}
	}
	if !reach {
		return nil
	}
	_, err = t.trimBottom(tb.ID)
	return err
}

// contentCols : colonnes dont une valeur retient une ligne (décision 0009).
// Les colonnes calculées, la colonne de catégorie masquée par l'onglet à
// choix (sa valeur est imposée) et la colonne numérotée d'office (clé
// d'entiers ou suite régulière, décision 0010 : son numéro est donné à toute
// ligne ajoutée) n'en sont pas ; une autre clé, si. Un tableau qui n'a
// d'autre colonne saisie que la colonne numérotée la garde : une ligne y est
// vide quand son numéro l'est.
func (t *tx) contentCols(tb Table) ([]Column, error) {
	auto, numbered, err := t.autoNumber(tb)
	if err != nil {
		return nil, err
	}
	var content []Column
	for _, c := range tb.Cols {
		if c.Formula != "" || numbered && c.ID == auto.col.ID {
			continue
		}
		if ch := tb.choice; ch != nil && !ch.All && c.ID == ch.Col.ID {
			continue
		}
		content = append(content, c)
	}
	if len(content) == 0 && numbered {
		content = []Column{auto.col}
	}
	return content, nil
}

// rowsLocked : les lignes du tableau ne se suppriment pas d'office (tableau
// publié en lecture seule, ou chargé par une requête).
func (t *tx) rowsLocked(tb Table) (bool, error) {
	if t.lockTable(tb.ID) != nil {
		return true, nil
	}
	var queries int
	if err := t.row(`SELECT count(*) FROM _lattis_queries WHERE target = ?`, tb.ID).Scan(&queries); err != nil {
		return true, err
	}
	return queries > 0, nil
}

// lastShown donne la clé technique de la dernière ligne affichée du tableau
// (sous l'onglet à choix : la dernière de la valeur choisie), 0 s'il n'en a
// pas.
func (t *tx) lastShown(tb Table) (int64, error) {
	var id int64
	err := t.row(`SELECT id FROM ` + tb.view() + ` ORDER BY _pos DESC LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// trimBottom supprime les lignes vides au bas d'un tableau (décision 0009,
// règle du bas), après une suppression ou un effacement qui a atteint sa
// dernière ligne : vide au sens de dropEmptiedRows (contentCols, saisies hors
// type comprises, mise en forme ignorée). Sous l'onglet à choix, le bas est
// celui des lignes de la valeur choisie (les lignes affichées), pas la fin
// physique du tableau. Un tableau filtré par l'utilisateur, publié en lecture
// seule ou chargé par une requête ne bouge pas ; il garde au moins une ligne
// affichée. Même étape d'annulation que l'opération ; rend le nombre de
// lignes retirées (dit dans la barre d'état).
func (t *tx) trimBottom(id int64) (int, error) {
	tb, err := t.table(id)
	if err != nil || tb.userFiltered() || tb.Shown < 2 {
		return 0, err
	}
	if locked, err := t.rowsLocked(tb); err != nil || locked {
		return 0, err
	}
	content, err := t.contentCols(tb)
	if err != nil || len(content) == 0 {
		return 0, err // rien qui dise une ligne vide : rien ne part
	}
	conds := make([]string, len(content))
	for i, c := range content {
		conds[i] = filledSQL(tb, c)
	}
	from := tb.sql() + " AS d"
	if tb.where != "" {
		from = "(SELECT * FROM " + tb.sql() + " WHERE " + tb.where + ") AS d"
	}
	rs, err := t.query(fmt.Sprintf(`SELECT d.id, %s FROM %s ORDER BY d._pos DESC`, strings.Join(conds, " OR "), from))
	if err != nil {
		return 0, err
	}
	var empty []int64
	for rs.Next() {
		var rid int64
		var filled bool
		if err := rs.Scan(&rid, &filled); err != nil {
			rs.Close()
			return 0, err
		}
		if filled {
			break
		}
		empty = append(empty, rid)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return 0, err
	}
	if len(empty) >= tb.Shown {
		empty = empty[:tb.Shown-1] // la première ligne affichée reste
	}
	if len(empty) == 0 {
		return 0, nil
	}
	if err := t.trashRows(tb, idList(empty)); err != nil {
		return 0, err
	}
	if len(empty) == 1 {
		t.notes = append(t.notes, "1 ligne vide retirée en bas du tableau (Ctrl+Z pour annuler)")
	} else {
		t.notes = append(t.notes, fmt.Sprintf("%d lignes vides retirées en bas du tableau (Ctrl+Z pour annuler)", len(empty)))
	}
	return len(empty), nil
}

// placeTables recopie ou déplace les tableaux et colonnes entières de la
// source, décalés de (dr, dc), sur la feuille to, puis les pose (fusion
// côte à côte, écartement des tableaux chevauchés).
func (t *tx) placeTables(p payload, to int64, dr, dc int, move bool) error {
	var ids []int64
	for _, tb := range p.whole {
		id := tb.ID
		if move {
			if tb.Sheet != to {
				q := `UPDATE _lattis_tables SET sheet = ? WHERE id = ?`
				if err := t.apply([]stmt{st(q, to, tb.ID)}, []stmt{st(q, tb.Sheet, tb.ID)}); err != nil {
					return err
				}
			}
		} else {
			var err error
			if id, err = t.copyTableID(tb, to); err != nil {
				return err
			}
		}
		if err := t.moveTable(Table{ID: id}, dr, dc); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	for _, s := range p.subs {
		row, col := s.tb.Row+dr, s.tb.Col+s.i0+dc
		if move {
			tb, err := t.table(s.tb.ID)
			if err != nil {
				return err
			}
			if err := t.extractColumns(tb, s.i0, s.i1, to, row, col); err != nil {
				return err
			}
			continue
		}
		id, err := t.copyTableID(s.tb, to)
		if err != nil {
			return err
		}
		for i := len(s.tb.Cols) - 1; i >= 0; i-- {
			if i >= s.i0 && i <= s.i1 {
				continue
			}
			cur, err := t.table(id)
			if err != nil {
				return err
			}
			if err := t.dropColumn(cur, cur.Cols[i], i); err != nil {
				return err
			}
		}
		if err := t.moveTable(Table{ID: id}, row-s.tb.Row, col-s.tb.Col); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	for _, id := range ids {
		var n int
		if err := t.row(`SELECT count(*) FROM _lattis_tables WHERE id = ?`, id).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			continue // fusionné dans un autre
		}
		tb, err := t.table(id)
		if err != nil {
			return err
		}
		if tb.Row < 0 || tb.Col < 0 {
			return fmt.Errorf("%w : hors de la feuille", ErrTable)
		}
		if _, err := t.place(tb); err != nil {
			return err
		}
	}
	return nil
}

// copyTableID recopie un tableau sur la feuille sheet (même position) et
// renvoie l'identifiant de la copie.
func (t *tx) copyTableID(tb Table, sheet int64) (int64, error) {
	n, err := t.copyTable(tb, sheet)
	if err != nil {
		return 0, err
	}
	var id int64
	err = t.row(`SELECT id FROM _lattis_tables WHERE name = ?`, n).Scan(&id)
	return id, err
}

// putPieces pose les cellules transportées, coin haut gauche en (row, col).
func (t *tx) putPieces(pieces []piece, sheet int64, row, col int, move bool, ev *Event) error {
	tables, err := t.tables(sheet)
	if err != nil {
		return err
	}
	var born []piece // valeurs de tableau posées en cellules libres
	for _, pc := range pieces {
		r, c := row+pc.dr, col+pc.dc
		var dest *Table
		ext := false
		for i := range tables {
			tb := &tables[i]
			inCols := c >= tb.Col && c <= tb.Right()
			inRows := r >= tb.Row && r <= tb.Bottom()
			switch {
			case inCols && inRows:
				dest = tb
			case inCols && r == tb.Bottom()+1, inRows && c == tb.Right()+1:
				ext = true
			}
		}
		text := pc.text
		switch {
		case dest != nil:
			// Dans un tableau : la valeur remplace celle de la cellule.
			i := c - dest.Col
			if r == dest.Row {
				if strings.TrimSpace(text) != "" {
					// Un en-tête collé déjà pris dans le tableau devient « Produit 2 ».
					name := freeColumnNameExcept(*dest, dest.Cols[i].ID, strings.TrimSpace(text))
					if err := t.rename(*dest, dest.Cols[i], name); err != nil {
						return err
					}
				}
				if err := t.putTableStyle(dest.ID, -1, dest.Cols[i].ID, pc.style); err != nil {
					return err
				}
				continue
			}
			if dest.Cols[i].Formula != "" {
				continue // une colonne calculée garde sa formule
			}
			if isFormula(text) {
				continue // formule sans sa valeur affichée : rien à poser
			}
			if err := t.setCell(*dest, r-dest.Row-1, i, text, ev); err != nil {
				return err
			}
			rid, _, err := t.rowID(*dest, r-dest.Row-1)
			if err != nil {
				return err
			}
			if err := t.putTableStyle(dest.ID, rid, dest.Cols[i].ID, pc.style); err != nil {
				return err
			}
			if tables, err = t.tables(sheet); err != nil {
				return err
			}
		case ext && strings.TrimSpace(text) != "":
			// Juste dessous ou à droite d'un tableau : il s'étend, comme à la saisie.
			if err := t.input(sheet, r, c, text, ev); err != nil {
				return err
			}
			if tables, err = t.tables(sheet); err != nil {
				return err
			}
			for _, tb := range tables {
				if r > tb.Row && r <= tb.Bottom() && c >= tb.Col && c <= tb.Right() {
					rid, _, err := t.rowID(tb, r-tb.Row-1)
					if err != nil {
						return err
					}
					if err := t.putTableStyle(tb.ID, rid, tb.Cols[c-tb.Col].ID, pc.style); err != nil {
						return err
					}
				}
			}
		default:
			raw := pc.raw
			if !pc.free {
				raw = freeRaw(text)
				if strings.TrimSpace(raw) != "" {
					born = append(born, piece{dr: r, dc: c})
				}
			}
			if err := t.setFreeCell(sheet, r, c, raw); err != nil {
				return err
			}
			if err := t.putFreeStyle(sheet, r, c, pc.style); err != nil {
				return err
			}
			if move && pc.name != "" {
				if err := t.deleteWhere(`_lattis_names`, `sheet = ? AND row = ? AND col = ?`, sheet, r, c); err != nil {
					return err
				}
				q := `INSERT INTO _lattis_names (name, sheet, row, col) VALUES (?, ?, ?, ?)`
				if err := t.apply([]stmt{st(q, pc.name, sheet, r, c)}, []stmt{st(`DELETE FROM _lattis_names WHERE name = ?`, pc.name)}); err != nil {
					return err
				}
			}
		}
	}
	// Des valeurs de tableau posées en zone libre sur au moins deux lignes
	// font un tableau, comme un collage.
	for _, b := range born {
		raw, ok, err := t.rawAt(sheet, b.dr, b.dc)
		if err != nil {
			return err
		}
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		if !ok || raw == "" || inTable(tables, b.dr, b.dc) {
			continue
		}
		name, err := t.birth(sheet, b.dr, b.dc)
		if err != nil {
			return err
		}
		if name != "" {
			ev.Created = name
		}
		break
	}
	return nil
}

// tidyAfterClear range un tableau dont la cellule de la colonne col vient
// d'être vidée par une saisie vide : sa dernière colonne « Colonne n » restée
// vide part (une valeur tapée par erreur à droite), puis ses dernières lignes
// vides si la cellule était sur sa dernière ligne affichée (tidyBottom ; sous
// l'onglet à choix, la dernière de la valeur choisie).
func (t *tx) tidyAfterClear(tb Table, row, col int) error {
	reach := row == tb.Row+tb.Shown
	if tb.Filtered() {
		if !reach || tb.userFiltered() {
			return nil
		}
		_, err := t.trimBottom(tb.ID)
		return err
	}
	if c := tb.Cols[col-tb.Col]; col == tb.Right() && len(tb.Cols) > 1 && c.Formula == "" && autoColRe.MatchString(c.Name) {
		empty, err := t.columnEmpty(tb, c)
		if err != nil {
			return err
		}
		if empty {
			if err := t.dropColumns(tb, []int{col - tb.Col}); err != nil {
				return err
			}
		}
	}
	return t.tidyBottom(tb.ID, false, reach)
}

// putFreeStyle remplace la mise en forme d'une cellule libre (vide : aucune).
func (t *tx) putFreeStyle(sheet int64, row, col int, style string) error {
	return t.setStyle(`_lattis_cell_styles`, `sheet = ? AND row = ? AND col = ?`, `sheet, row, col`, []any{sheet, row, col},
		func(string) (string, error) { return style, nil })
}

// putTableStyle remplace la mise en forme d'une cellule de tableau (rid -1 :
// en-tête ; vide : aucune).
func (t *tx) putTableStyle(tbl, rid, col int64, style string) error {
	return t.setStyle(`_lattis_styles`, `tbl = ? AND rid = ? AND col = ?`, `tbl, rid, col`, []any{tbl, rid, col},
		func(string) (string, error) { return style, nil })
}

// ClearRange efface la zone [r0, r1] × [c0, c1] en une seule étape
// d'annulation : les tableaux entièrement compris sont supprimés ; les
// autres cellules de tableau perdent leur valeur (une colonne calculée
// perd sa formule si toutes ses lignes sont effacées ; les en-têtes
// restent) ; les cellules libres perdent leur contenu (Suppr), ou tout
// avec all (Supprimer : mise en forme, fusions, notes et validations).
func (m *Model) ClearRange(ctx context.Context, sheet int64, r0, c0, r1, c1 int, all bool) error {
	if r0 > r1 || c0 > c1 {
		return nil
	}
	label := "Effacer le contenu"
	if all {
		label = "Supprimer"
	}
	return m.Do(ctx, label, func(t *tx) error {
		if err := t.guard(sheet); err != nil {
			return err
		}
		zone := Rect{r0, c0, r1, c1}
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		// Colonnes vidées (choix de Louis) : une colonne sans donnée dont
		// l'en-tête est dans la zone, ou la dernière à droite devenue vide,
		// quitte le tableau. Un en-tête qui ne peut pas partir dit pourquoi.
		// split : le tableau a été coupé en deux (il n'existe plus).
		dropCleared := func(tb Table) (split bool, err error) {
			if tb.Filtered() || len(tb.Cols) < 2 {
				return false, nil
			}
			var idx []int
			for c := max(c0, tb.Col); c <= min(c1, tb.Right()); c++ {
				i := c - tb.Col
				col := tb.Cols[i]
				header := r0 <= tb.Row
				if !header && c != tb.Right() {
					continue
				}
				empty, err := t.columnEmpty(tb, col)
				if err != nil {
					return false, err
				}
				switch {
				case empty && col.Formula == "" && header && i > 0 && i < len(tb.Cols)-1:
					// Colonnes vides au milieu, en-têtes effacés : le tableau se
					// coupe en deux de part et d'autre (choix de Louis).
					j := i
					for j+1 < len(tb.Cols)-1 && tb.Col+j+1 <= c1 {
						if more, err := t.columnsEmpty(tb, j+1, j+1); err != nil {
							return false, err
						} else if !more {
							break
						}
						j++
					}
					return true, t.splitAt(tb, i, j)
				case empty && col.Formula == "":
					idx = append(idx, i)
				case header && col.Formula != "":
					t.notes = append(t.notes, fmt.Sprintf("« %s » est une colonne calculée : son en-tête ne s'efface pas (clic droit › Supprimer la colonne pour la retirer)", clip(col.Name)))
				case header:
					t.notes = append(t.notes, fmt.Sprintf("L'en-tête « %s » ne s'efface pas : la colonne contient des données (sélectionnez toute la colonne puis Suppr pour la retirer)", clip(col.Name)))
				}
			}
			if len(idx) == 0 {
				return false, nil
			}
			if len(idx) == len(tb.Cols) {
				t.notes = append(t.notes, fmt.Sprintf("Le tableau « %s » garde au moins une colonne", tb.Name))
				idx = idx[1:]
			}
			return false, t.dropColumns(tb, idx)
		}
		cells, dropped := 0, 0
		for _, tb := range tables {
			switch {
			case !zone.meets(tb):
			case zone.holds(tb):
				if err := t.deleteTable(tb); err != nil {
					return err
				}
			default:
				first, last := max(r0, tb.Row+1), min(r1, tb.Bottom())
				if tb.Filtered() {
					last = min(last, tb.Row+tb.Shown)
				}
				if first > last {
					if _, err := dropCleared(tb); err != nil {
						return err
					}
					continue
				}
				ids, err := t.ridsAt(&tb, first, last)
				if err != nil {
					return err
				}
				// L'effacement atteint la dernière ligne affichée : les lignes
				// vides du bas partent ensuite (décision 0009, règle du bas).
				reach := last == tb.Row+tb.Shown
				// Lignes dont l'effacement vide quelque chose : candidates à
				// la suppression si plus rien n'y reste (emptiedRows).
				var cleared []Column
				for c := max(c0, tb.Col); c <= min(c1, tb.Right()); c++ {
					if col := tb.Cols[c-tb.Col]; col.Formula == "" {
						cleared = append(cleared, col)
					}
				}
				touched, err := t.filledRows(tb, ids, cleared)
				if err != nil {
					return err
				}
				for c := max(c0, tb.Col); c <= min(c1, tb.Right()); c++ {
					i := c - tb.Col
					col := tb.Cols[i]
					if col.Formula != "" {
						if first == tb.Row+1 && last == tb.Bottom() {
							if err := t.setFormula(tb, i, ""); err != nil {
								return err
							}
						}
						continue
					}
					for _, rid := range ids {
						if cells++; cells > MaxPasteCells {
							return ErrPasteTooBig{cells}
						}
						if err := t.setValue(tb, rid, col, nil, ""); err != nil {
							return err
						}
						if all {
							if err := t.putTableStyle(tb.ID, rid, col.ID, ""); err != nil {
								return err
							}
						}
					}
				}
				// Lignes devenues vides : elles partent (demande de Louis,
				// décision 0009), comme si elles n'avaient jamais été tapées.
				if n, err := t.dropEmptiedRows(tb, touched); err != nil {
					return err
				} else if n > 0 {
					dropped += n
					if tb, err = t.table(tb.ID); err != nil {
						return err
					}
				}
				// Coupé en deux : l'ancien tableau n'existe plus (audit 2 I3).
				if split, err := dropCleared(tb); err != nil || split {
					if err != nil {
						return err
					}
					continue
				}
				// Les dernières lignes devenues vides partent (choix de Louis),
				// sous l'onglet à choix celles de la valeur choisie.
				if err := t.tidyBottom(tb.ID, false, reach); err != nil {
					return err
				}
			}
		}
		switch {
		case dropped == 1:
			t.notes = append(t.notes, "1 ligne vide supprimée (Ctrl+Z pour annuler)")
		case dropped > 1:
			t.notes = append(t.notes, fmt.Sprintf("%d lignes vides supprimées (Ctrl+Z pour annuler)", dropped))
		}
		if err := t.clearPivotHand(sheet, zone); err != nil {
			return err
		}
		if err := t.clearPivots(sheet, zone); err != nil {
			return err
		}
		if all {
			return t.clearFree(sheet, r0, c0, r1, c1)
		}
		if err := t.clearOutIn(sheet, r0, c0, r1, c1); err != nil {
			return err
		}
		return t.deleteWhere(`_lattis_cells`, `sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`, sheet, r0, r1, c0, c1)
	})
}

// filledSQL : la cellule de la colonne c de la ligne d porte quelque chose
// (une valeur, ou une saisie hors type).
func filledSQL(tb Table, c Column) string {
	return fmt.Sprintf(`(coalesce(CAST(d.%s AS TEXT), '') <> '' OR EXISTS (SELECT 1 FROM _lattis_invalid i WHERE i.tbl = %d AND i.rid = d.id AND i.col = %d))`, colSQL(c.ID), tb.ID, c.ID)
}

// idList écrit des clés techniques en liste SQL (des entiers).
func idList(ids []int64) string {
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(s, ", ")
}

// filledRows donne, parmi ids (dans leur ordre), les lignes qui ont une
// valeur dans l'une des colonnes cols.
func (t *tx) filledRows(tb Table, ids []int64, cols []Column) ([]int64, error) {
	if len(ids) == 0 || len(cols) == 0 {
		return nil, nil
	}
	conds := make([]string, len(cols))
	for i, c := range cols {
		conds[i] = filledSQL(tb, c)
	}
	rs, err := t.query(fmt.Sprintf(`SELECT d.id FROM %s AS d WHERE d.id IN (%s) AND (%s)`, tb.sql(), idList(ids), strings.Join(conds, " OR ")))
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	in := map[int64]bool{}
	for rs.Next() {
		var id int64
		if err := rs.Scan(&id); err != nil {
			return nil, err
		}
		in[id] = true
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	var out []int64
	for _, id := range ids {
		if in[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

// dropEmptiedRows supprime, parmi les lignes touched qu'un effacement vient
// de vider, celles où plus rien n'est saisi (décision 0009) : les colonnes
// calculées, la colonne de catégorie masquée par l'onglet à choix (sa valeur
// est imposée), la colonne numérotée d'office (clé d'entiers ou suite
// régulière, décision 0010 : son numéro est donné à toute ligne ajoutée) et
// la mise en forme ne comptent pas ; une autre clé, si, comme toute colonne
// saisie. Un tableau garde au moins une ligne. Rien ne part
// d'un tableau chargé par une requête ou publié en lecture seule : les
// cellules restent seulement vidées. Rend le nombre de lignes supprimées.
func (t *tx) dropEmptiedRows(tb Table, touched []int64) (int, error) {
	if len(touched) == 0 {
		return 0, nil
	}
	if locked, err := t.rowsLocked(tb); err != nil || locked {
		return 0, err
	}
	content, err := t.contentCols(tb)
	if err != nil {
		return 0, err
	}
	filled, err := t.filledRows(tb, touched, content)
	if err != nil {
		return 0, err
	}
	keep := map[int64]bool{}
	for _, id := range filled {
		keep[id] = true
	}
	var empty []int64
	for _, id := range touched {
		if !keep[id] {
			empty = append(empty, id)
		}
	}
	if len(empty) >= tb.Rows {
		empty = empty[1:] // le tableau garde sa première ligne
	}
	if len(empty) == 0 {
		return 0, nil
	}
	if err := t.trashRows(tb, idList(empty)); err != nil {
		return 0, err
	}
	return len(empty), nil
}

// clearPivots supprime les tableaux croisés effacés avec la zone, comme
// Excel : un TCD pris en entier (filtres de rapport compris), ou un TCD en
// erreur (source introuvable, colonne disparue) dont la cellule s'y trouve.
// Une partie seulement d'un TCD sain reste (l'interface l'a refusée).
func (t *tx) clearPivots(sheet int64, zone Rect) error {
	ps, err := t.pivots(sheet)
	if err != nil {
		return err
	}
	for _, p := range ps {
		if r := pivotRect(p); !(zone.has(r.R0, r.C0) && zone.has(r.R1, r.C1)) {
			continue
		}
		if err := t.deletePivot(p.ID); err != nil {
			return err
		}
	}
	return nil
}
