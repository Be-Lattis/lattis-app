// Fusions, lignes et colonnes masquées, validation par liste : lues dans
// les couches de style (voir internal/canvas/cellformat.go).

import type { StyleLayer } from './style';

/** Cellules fusionnées : cellule en haut à gauche, lignes et colonnes couvertes. */
export interface MergeRange {
	row: number;
	col: number;
	rows: number;
	cols: number;
}

/** Fusions d'après les styles des cellules libres (clé « ligne,colonne »). */
export function mergeList(styles: Iterable<[string, StyleLayer]>): MergeRange[] {
	const out: MergeRange[] = [];
	for (const [k, s] of styles) {
		const rows = s.mr ?? 0;
		const cols = s.mc ?? 0;
		if (rows < 1 || cols < 1 || rows * cols < 2) continue;
		const [row, col] = k.split(',').map(Number);
		out.push({ row, col, rows, cols });
	}
	return out;
}

/** Fusion qui couvre la cellule (r, c), s'il y en a une. */
export function mergeAt(list: MergeRange[], r: number, c: number): MergeRange | undefined {
	for (const g of list) if (r >= g.row && r < g.row + g.rows && c >= g.col && c < g.col + g.cols) return g;
	return undefined;
}

/** Lignes (ou colonnes) masquées d'après les styles de lignes (ou de colonnes). */
export function hiddenLines(styles: Map<number, StyleLayer>): Set<number> {
	const out = new Set<number>();
	for (const [i, s] of styles) if (s.hd) out.add(i);
	return out;
}

/**
 * Première ligne visible à partir de i dans la direction d (+1 ou -1 ; 0 :
 * vers le bas puis vers le haut), bornée à [0, count[.
 */
export function skipHidden(i: number, d: number, hidden: Set<number>, count: number): number {
	if (!hidden.has(i)) return i;
	for (const dir of d ? [d, -d] : [1, -1]) {
		let k = i;
		while (k >= 0 && k < count && hidden.has(k)) k += dir;
		if (k >= 0 && k < count) return k;
	}
	return i;
}

/**
 * Validation par liste (liste déroulante) : la saisie est-elle permise ?
 * Une cellule vide l'est toujours. La comparaison ignore la casse et les
 * espaces autour ; la valeur retenue est celle de la liste.
 */
export function validate(list: string[] | undefined, text: string): { ok: true; value: string } | { ok: false; message: string } {
	if (!list?.length || text.trim() === '') return { ok: true, value: text };
	const t = text.trim().toLocaleLowerCase('fr');
	const hit = list.find((v) => v.trim().toLocaleLowerCase('fr') === t);
	if (hit !== undefined) return { ok: true, value: hit };
	const shown = list.length > 8 ? list.slice(0, 8).join(', ') + '…' : list.join(', ');
	return { ok: false, message: `« ${text.trim()} » n'est pas permis ici : choisissez dans la liste (${shown}), Alt+↓ pour l'ouvrir` };
}

/** Liste de valeurs saisie dans le dialogue (« a; b; c » ou une valeur par ligne). */
export function parseList(text: string): string[] {
	const seen = new Set<string>();
	const out: string[] = [];
	for (const v of text.split(/[;\n]/)) {
		const t = v.trim();
		if (t && !seen.has(t.toLocaleLowerCase('fr'))) {
			seen.add(t.toLocaleLowerCase('fr'));
			out.push(t);
		}
	}
	return out;
}
