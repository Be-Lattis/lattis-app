// Sélection et déplacement autour des cellules fusionnées, comme Excel :
// une plage qui touche une fusion l'englobe entière ; les flèches, Tab et
// Entrée passent une fusion comme une seule cellule.

import { mergeAt, type MergeRange } from './sheetfmt';

export type CellRange = { r0: number; r1: number; c0: number; c1: number };
export type Cell = { row: number; col: number };

/** Plage couverte par une fusion. */
export function mergeRect(g: MergeRange): CellRange {
	return { r0: g.row, r1: g.row + g.rows - 1, c0: g.col, c1: g.col + g.cols - 1 };
}

/** Plage entre deux cellules (dans n'importe quel ordre). */
export function rangeOf(a: Cell, b: Cell): CellRange {
	return { r0: Math.min(a.row, b.row), r1: Math.max(a.row, b.row), c0: Math.min(a.col, b.col), c1: Math.max(a.col, b.col) };
}

/**
 * Plage agrandie jusqu'à englober entièrement chaque fusion qu'elle touche,
 * et ainsi de suite (une fusion englobée peut en toucher une autre), comme Excel.
 */
export function expandRange(r: CellRange, merges: readonly MergeRange[]): CellRange {
	if (!merges.length) return r;
	let { r0, r1, c0, c1 } = r;
	for (let changed = true; changed; ) {
		changed = false;
		for (const g of merges) {
			const m = mergeRect(g);
			if (m.r1 < r0 || m.r0 > r1 || m.c1 < c0 || m.c0 > c1) continue;
			if (m.r0 < r0 || m.r1 > r1 || m.c0 < c0 || m.c1 > c1) {
				r0 = Math.min(r0, m.r0);
				r1 = Math.max(r1, m.r1);
				c0 = Math.min(c0, m.c0);
				c1 = Math.max(c1, m.c1);
				changed = true;
			}
		}
	}
	return { r0, r1, c0, c1 };
}

const same = (a: CellRange, b: CellRange) => a.r0 === b.r0 && a.r1 === b.r1 && a.c0 === b.c0 && a.c1 === b.c1;

/** La plage n'est-elle que la cellule (ou la fusion) de la cellule active ? */
export function isSingleCell(sel: CellRange, active: Cell, merges: readonly MergeRange[]): boolean {
	const g = mergeAt(merges as MergeRange[], active.row, active.col);
	return same(sel, g ? mergeRect(g) : { r0: active.row, r1: active.row, c0: active.col, c1: active.col });
}

/**
 * Cellule d'arrivée d'une flèche (ou de Tab, Entrée) depuis la cellule
 * (row, col) : on sort d'une fusion par son bord dans le sens du déplacement,
 * sur la ligne (ou colonne) en cours ; une fusion atteinte donne sa cellule
 * en haut à gauche.
 */
export function stepCell(row: number, col: number, dr: number, dc: number, merges: readonly MergeRange[], maxRow: number, maxCol: number): Cell {
	const from = mergeAt(merges as MergeRange[], row, col);
	if (from) {
		const m = mergeRect(from);
		if (dr > 0) row = m.r1;
		else if (dr < 0) row = m.r0;
		if (dc > 0) col = m.c1;
		else if (dc < 0) col = m.c0;
	}
	const next = { row: Math.min(maxRow, Math.max(0, row + dr)), col: Math.min(maxCol, Math.max(0, col + dc)) };
	const to = mergeAt(merges as MergeRange[], next.row, next.col);
	if (to && to !== from) return { row: to.row, col: to.col };
	// Bord de la feuille : on reste dans la fusion de départ.
	if (to && to === from) return { row: from.row, col: from.col };
	return next;
}

/**
 * Maj + flèche : nouveau coin mobile de la sélection (la cellule active
 * reste fixe). Le bord de la plage agrandie aux fusions avance (ou recule)
 * d'une cellule ; tant que la plage obtenue ne change pas (une fusion la
 * ramène), on continue, comme Excel.
 */
export function extendCorner(active: Cell, corner: Cell, dr: number, dc: number, merges: readonly MergeRange[], maxRow: number, maxCol: number): Cell {
	const cur = expandRange(rangeOf(active, corner), merges);
	const axis = (a: number, k: number, d: number, lo: number, hi: number, max: number) => {
		// Côté qui bouge : celui du coin mobile (même ligne : celui du sens).
		const high = k > a || (k === a && d > 0);
		return { start: (high ? hi : lo) + d, max };
	};
	let { row, col } = corner;
	if (dr) {
		const { start, max } = axis(active.row, corner.row, dr, cur.r0, cur.r1, maxRow);
		for (let r = start; r >= 0 && r <= max; r += dr) {
			const next = expandRange(rangeOf(active, { row: r, col }), merges);
			if (!same(next, cur)) return { row: r, col };
		}
		return corner;
	}
	if (dc) {
		const { start, max } = axis(active.col, corner.col, dc, cur.c0, cur.c1, maxCol);
		for (let c = start; c >= 0 && c <= max; c += dc) {
			const next = expandRange(rangeOf(active, { row, col: c }), merges);
			if (!same(next, cur)) return { row, col: c };
		}
		return corner;
	}
	return { row, col };
}

/**
 * Lignes (ou colonnes) entières de from à to, choisies par la gouttière ou
 * le bandeau (ou Maj+Espace) : exactement celles-là, comme Excel. Une fusion
 * qui les traverse n'agrandit pas la sélection (sinon un clic sur la ligne
 * d'en-tête d'un tableau à deux lignes prenait aussi la bande, et l'on ne
 * pouvait ni masquer ni régler une seule de ces lignes). from est la ligne
 * (colonne) où le glisser a commencé : la cellule active est sa première
 * cellule, ou le haut à gauche de sa fusion.
 */
export function wholeLines(kind: 'rows' | 'cols', from: number, to: number, last: number, merges: readonly MergeRange[]): { active: Cell; range: CellRange } {
	const i0 = Math.min(from, to);
	const i1 = Math.max(from, to);
	const range = kind === 'rows' ? { r0: i0, r1: i1, c0: 0, c1: last } : { r0: 0, r1: last, c0: i0, c1: i1 };
	const start = kind === 'rows' ? { row: from, col: 0 } : { row: 0, col: from };
	const g = mergeAt(merges as MergeRange[], start.row, start.col);
	return { active: g ? { row: g.row, col: g.col } : start, range };
}

/** Partie commune de deux plages (null : aucune). */
export function intersect(a: CellRange, b: CellRange): CellRange | null {
	const r = { r0: Math.max(a.r0, b.r0), r1: Math.min(a.r1, b.r1), c0: Math.max(a.c0, b.c0), c1: Math.min(a.c1, b.c1) };
	return r.r0 <= r.r1 && r.c0 <= r.c1 ? r : null;
}
