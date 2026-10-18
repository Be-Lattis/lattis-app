// Zone de nom, comme Excel : elle sert d'abord à aller quelque part. Le
// texte tapé est lu comme une adresse (F10, $F$10, Cell[F10], A1:C3), un
// tableau (Ventes), une colonne de tableau (Ventes[Prix]) ou une de ses
// cellules (Ventes[Prix][3], ou Prix[3] dans le tableau actif).

/** Un tableau de la feuille : en-tête en (row, col), dernière ligne et colonne. */
export interface GotoTable {
	name: string;
	row: number;
	col: number;
	last: number;
	lastCol: number;
	columns: string[];
}

/** Plage à sélectionner (à partir de 0) ; une cellule : r0 = r1, c0 = c1. */
export interface GotoRange {
	r0: number;
	c0: number;
	r1: number;
	c1: number;
}

const MAX_COL = 475_254; // ZZZZ
const MAX_ROW = 999_999_999;

/** Une adresse A1 (F10, $F$10) : [ligne, colonne] à partir de 0. */
export function parseA1(s: string): [number, number] | null {
	const m = /^\$?([a-z]{1,4})\$?([1-9][0-9]{0,8})$/i.exec(s.trim());
	if (!m) return null;
	let c = 0;
	for (const ch of m[1].toUpperCase()) c = c * 26 + ch.charCodeAt(0) - 64;
	const r = Number(m[2]);
	if (c > MAX_COL || r > MAX_ROW) return null;
	return [r - 1, c - 1];
}

/** Le nom ressemble à une adresse de cellule (F10, AB12, R1C1) : refusé pour un tableau. */
export function a1Like(name: string): boolean {
	return /^(?:[a-z]{1,4}[1-9][0-9]{0,8}|r[1-9][0-9]*c[1-9][0-9]*)$/i.test(name.trim());
}

const eq = (a: string, b: string) => a.toLowerCase() === b.toLowerCase();

/** Une référence d'un côté de « : » (une cellule ou une plage). */
function part(s: string, tables: GotoTable[], current?: GotoTable): GotoRange | null {
	s = s.trim();
	if (!s) return null;
	// Cell[F10], Cell[A1:C3], cell_F10 (ancienne forme).
	const cell = /^cell(?:\[(.+)\]|_(.+))$/i.exec(s);
	if (cell) return cell[1] ? range(cell[1], [], undefined) : range(cell[2], [], undefined);
	// Tableau, Tableau[Colonne], Tableau[Colonne][n] ; un tableau existant
	// passe avant une adresse (un ancien nom comme « Q1 »).
	const m = /^([^[\]]+?)\s*(?:\[([^\]]+)\])?\s*(?:\[\s*([0-9]+)\s*\])?$/.exec(s);
	const t = m && tables.find((x) => eq(x.name, m[1]));
	if (m && t) {
		const [, , colName, n] = m;
		if (colName === undefined) return n === undefined ? { r0: t.row, c0: t.col, r1: t.last, c1: t.lastCol } : null;
		const i = t.columns.findIndex((c) => eq(c, colName.trim()));
		if (i < 0) return null;
		return cellOf(t, i, n);
	}
	const a1 = parseA1(s);
	if (a1) return { r0: a1[0], c0: a1[1], r1: a1[0], c1: a1[1] };
	if (!m) return null;
	const [, head, colName, n] = m;
	// Colonne[n] (la forme de la zone de nom d'un tableau) dans le tableau actif.
	if (current && colName !== undefined && n === undefined && /^[0-9]+$/.test(colName.trim())) {
		const i = current.columns.findIndex((c) => eq(c, head));
		if (i >= 0) return cellOf(current, i, colName.trim());
	}
	return null;
}

function cellOf(t: GotoTable, i: number, n: string | undefined): GotoRange | null {
	const c = t.col + i;
	if (n === undefined) return { r0: Math.min(t.row + 1, t.last), c0: c, r1: t.last, c1: c };
	const r = t.row + Number(n);
	if (r > t.last) return null;
	return { r0: r, c0: c, r1: r, c1: c };
}

function range(s: string, tables: GotoTable[], current?: GotoTable): GotoRange | null {
	// « : » hors des crochets (Cell[A1:C3] est une seule référence).
	const sides: string[] = [''];
	let depth = 0;
	for (const ch of s) {
		if (ch === '[') depth++;
		else if (ch === ']') depth--;
		if (ch === ':' && depth === 0) sides.push('');
		else sides[sides.length - 1] += ch;
	}
	if (sides.length > 2) return null;
	const a = part(sides[0], tables, current);
	if (!a) return null;
	if (sides.length === 1) return a;
	const b = part(sides[1], tables, current);
	if (!b) return null;
	return { r0: Math.min(a.r0, b.r0), c0: Math.min(a.c0, b.c0), r1: Math.max(a.r1, b.r1), c1: Math.max(a.c1, b.c1) };
}

/**
 * Où va le texte tapé dans la zone de nom (null : ce n'est ni une adresse ni
 * un tableau) ; current : le tableau de la cellule active, pour Colonne[n].
 */
export function parseGoto(text: string, tables: GotoTable[], current?: GotoTable): GotoRange | null {
	const s = text.trim();
	return s ? range(s, tables, current) : null;
}
