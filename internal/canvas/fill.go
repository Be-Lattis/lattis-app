package canvas

import (
	"context"
	"encoding/json"
)

// Fill écrit la recopie (poignée de la sélection) : block, la suite calculée
// à partir de la zone src, est collé à partir de (row, col), comme Paste, et
// chaque cellule remplie prend la validation des données (propriété dv) de
// la cellule de src dont elle est la recopie, comme le copier-coller la
// porte : D3 recopiée de D2 refuse ce que D2 refuse. Une seule étape
// d'annulation pour les valeurs et les règles.
func (m *Model) Fill(ctx context.Context, sheet int64, src Rect, row, col int, block [][]string) (Event, error) {
	h, w := len(block), 0
	for _, r := range block {
		w = max(w, len(r))
	}
	if src.R0 > src.R1 || src.C0 > src.C1 || h == 0 || w == 0 || src.height()*src.width() > MaxPasteCells {
		return m.Paste(ctx, sheet, row, col, block)
	}
	// Validation de la source lue avant le collage : une recopie contre des
	// cellules libres fait naître un tableau, qui ne la garde pas.
	srcDV := map[[2]int]any{}
	use := false
	err := m.read(ctx, func(t *tx) error {
		tables, err := t.tables(sheet)
		if err != nil {
			return err
		}
		for r := src.R0; r <= src.R1; r++ {
			for c := src.C0; c <= src.C1; c++ {
				_, v, err := t.dvRawAt(sheet, r, c, tables)
				if err != nil {
					return err
				}
				srcDV[[2]int{r, c}] = v
				use = use || !empty(v)
			}
		}
		if !use {
			// Rien à la source : seules des règles de la destination à retirer.
			use, err = t.sheetHasDV(sheet, tables)
		}
		return err
	})
	if err != nil {
		return Event{}, err
	}
	if !use {
		return m.Paste(ctx, sheet, row, col, block)
	}
	dst := Rect{row, col, row + h - 1, col + w - 1}
	// Source comprise : elle garde sa règle si un tableau est né.
	area := Rect{min(src.R0, dst.R0), min(src.C0, dst.C0), max(src.R1, dst.R1), max(src.C1, dst.C1)}
	return m.paste(ctx, sheet, row, col, block, func(t *tx) error { return t.fillDV(sheet, src, area, srcDV) })
}

// fillDV donne à chaque cellule de area la validation srcDV de la cellule de
// src qu'elle répète (src répété dans les deux sens, comme la recopie) ; une
// source sans validation la retire de la destination.
func (t *tx) fillDV(sheet int64, src, area Rect, srcDV map[[2]int]any) error {
	// Tableaux relus après le collage : la recopie a pu en agrandir un.
	tables, err := t.tables(sheet)
	if err != nil {
		return err
	}
	lines, err := t.sheetLines(sheet)
	if err != nil {
		return err
	}
	mod := func(a, n int) int { return ((a % n) + n) % n }
	h, w := src.height(), src.width()
	for r := area.R0; r <= area.R1; r++ {
		for c := area.C0; c <= area.C1; c++ {
			v := srcDV[[2]int{src.R0 + mod(r-src.R0, h), src.C0 + mod(c-src.C0, w)}]
			cell, cur, err := t.dvRawAt(sheet, r, c, tables)
			if err != nil {
				return err
			}
			if cell == nil {
				continue // en-tête de tableau : pas de validation
			}
			patch := Style{"dv": v}
			if empty(v) {
				patch["dv"] = ""
			}
			if cell.tb != nil {
				patch = dvForTable(patch, cell.tb, c)
			}
			if sameDV(cur, patch["dv"]) {
				continue
			}
			if cell.tb == nil {
				if err := t.styleFree(sheet, r, c, patch, lines.at(r, c)); err != nil {
					return err
				}
				continue
			}
			colID := cell.tb.Cols[c-cell.tb.Col].ID
			colStyle, err := t.tableStyle(cell.tb.ID, 0, colID)
			if err != nil {
				return err
			}
			if err := t.styleTable(cell.tb.ID, cell.rid, colID, patch, over(lines.at(r, c), colStyle)); err != nil {
				return err
			}
		}
	}
	return nil
}

// sheetHasDV dit si une couche de la feuille (cellules, lignes, colonnes,
// tableaux) porte une validation.
func (t *tx) sheetHasDV(sheet int64, tables []Table) (bool, error) {
	var n int
	err := t.row(`SELECT (SELECT count(*) FROM _lattis_cell_styles WHERE sheet = ? AND style LIKE '%"dv"%')
		+ (SELECT count(*) FROM _lattis_row_styles WHERE sheet = ? AND style LIKE '%"dv"%')
		+ (SELECT count(*) FROM _lattis_col_styles WHERE sheet = ? AND style LIKE '%"dv"%')`, sheet, sheet, sheet).Scan(&n)
	if err != nil || n > 0 {
		return n > 0, err
	}
	for _, tb := range tables {
		if err := t.row(`SELECT count(*) FROM _lattis_styles WHERE tbl = ? AND style LIKE '%"dv"%'`, tb.ID).Scan(&n); err != nil || n > 0 {
			return n > 0, err
		}
	}
	return false, nil
}

// sameDV dit si deux propriétés dv sont la même règle (vide : aucune).
func sameDV(a, b any) bool {
	if empty(a) || empty(b) {
		return empty(a) && empty(b)
	}
	x, err1 := json.Marshal(a)
	y, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(x) == string(y)
}
