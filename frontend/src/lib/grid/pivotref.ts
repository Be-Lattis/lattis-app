// Références vers un tableau croisé pointées au clic pendant la saisie d'une
// formule (internal/canvas/pivotref.go) : un TCD se lit comme un tableau
// nommé (TCD1[Nombre]), jamais par l'adresse de la cellule cliquée.
import type { PivotRowInfo } from './pivottree';

/** Ce qu'il faut d'un TCD affiché (SheetPivot). */
export interface RefPivot {
	row: number;
	col: number;
	cells: string[][];
	rows?: PivotRowInfo[] | null;
	config: { name?: string; rowFields?: string[]; noGrandTotal?: boolean };
	hand?: { name: string; formula?: string }[] | null;
	error?: string;
}

/**
 * Lignes de détail du TCD (indice dans cells, en-tête exclu) : ni total
 * général, ni groupe, ni sous-total. Sans champ en lignes, la seule ligne
 * (le total) en est une. Ce sont les lignes de TCD1[Col], et le rang n de
 * TCD1[Col][n] les compte, comme le moteur (pivotFormulaTable).
 */
export function pivotDetailRows(p: RefPivot): number[] {
	const nk = p.config.rowFields?.length ?? 0;
	const out: number[] = [];
	for (let i = 1; i < p.cells.length; i++) {
		if (nk === 0) out.push(i);
		else if (p.rows) {
			const info = p.rows[i - 1];
			if (info && !info.group && !info.subtotal && !info.total && (info.path?.length ?? 0) === nk) out.push(i);
		} else if (p.config.noGrandTotal || i < p.cells.length - 1) out.push(i);
	}
	return out;
}

/**
 * Nom de colonne d'une cellule du TCD (j : rang de la colonne) : le texte de
 * son en-tête ; en arborescence, la colonne des étiquettes d'une ligne de
 * détail donne le champ de son niveau ([Ville]), qui est aussi une colonne.
 */
function columnName(p: RefPivot, i: number, j: number): string {
	const head = (p.cells[0]?.[j] ?? '').trim();
	if (j === 0 && i > 0 && p.rows) {
		const info = p.rows[i - 1];
		const f = info ? p.config.rowFields?.[info.depth] : undefined;
		if (f) return f;
	}
	return head;
}

/**
 * Référence de la cellule (row, col) du TCD p, pointée depuis une formule
 * écrite en at (null : autre feuille). Dans une colonne saisie du TCD, sa
 * propre ligne se lit [Col] (une formule y vaut pour toute la colonne,
 * comme une colonne calculée) ; ailleurs, une ligne de détail donne
 * TCD1[Col][n] (n : son rang parmi les lignes de détail), l'en-tête, un
 * total, un groupe ou un sous-total la colonne entière TCD1[Col].
 * kind : ce qu'attend l'argument en cours (un tableau, une colonne entière).
 * null si la cellule n'est pas dans le TCD (ou s'il n'a pas de nom).
 */
export function pivotRefText(p: RefPivot, row: number, col: number, at: { row: number; col: number } | null, kind: '' | 'table' | 'column' = ''): string | null {
	const name = p.config.name;
	const i = row - p.row;
	const j = col - p.col;
	if (!name || p.error || i < 0 || i >= p.cells.length || j < 0 || j >= (p.cells[0]?.length ?? 0)) return null;
	if (kind === 'table') return name;
	const c = columnName(p, i, j);
	if (!c) return name;
	if (kind === 'column' || i === 0) return `${name}[${c}]`;
	const w = p.cells[0].length;
	const hs = p.col + w - (p.hand?.length ?? 0);
	if (at && at.row === row && at.col >= hs && at.col < p.col + w) return `[${c}]`;
	const n = pivotDetailRows(p).indexOf(i);
	return n < 0 ? `${name}[${c}]` : `${name}[${c}][${n + 1}]`;
}

/**
 * Plage glissée dans un TCD : une colonne (au moins deux lignes) donne
 * TCD1[Col] ; null si elle déborde d'une colonne ou sort du TCD.
 */
export function pivotRangeRef(p: RefPivot, r0: number, c0: number, r1: number, c1: number): string | null {
	const name = p.config.name;
	if (!name || p.error || c0 !== c1) return null;
	const top = Math.min(r0, r1);
	const bottom = Math.max(r0, r1);
	const j = c0 - p.col;
	if (top < p.row || bottom >= p.row + p.cells.length || j < 0 || j >= (p.cells[0]?.length ?? 0)) return null;
	const c = (p.cells[0][j] ?? '').trim();
	return c ? `${name}[${c}]` : null;
}
