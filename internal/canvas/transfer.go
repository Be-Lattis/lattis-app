package canvas

import (
	"context"
	"fmt"
	"slices"
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
	for _, pc := range p.partial {
		tb, ok := cur[pc.tb.ID]
		if !ok || pc.header {
			var err error
			if tb, err = t.table(pc.tb.ID); err != nil {
				return err
			}
			cur[tb.ID] = tb
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
	return nil
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
		cells := 0
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
					continue
				}
				ids, err := t.ridsAt(&tb, first, last)
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
			}
		}
		if all {
			return t.clearFree(sheet, r0, c0, r1, c1)
		}
		return t.deleteWhere(`_lattis_cells`, `sheet = ? AND row BETWEEN ? AND ? AND col BETWEEN ? AND ?`, sheet, r0, r1, c0, c1)
	})
}
