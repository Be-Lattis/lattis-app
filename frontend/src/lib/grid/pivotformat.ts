// Format de nombre par défaut des valeurs d'un tableau croisé (calculé côté
// Go, voir internal/canvas/pivotformat.go) : format de la colonne source, ou
// deux décimales pour une moyenne. C'est la couche la moins précise : la
// mise en forme posée sur les cellules (Ajouter / Réduire une décimale)
// l'emporte.

import { parseStyle, type StyleLayer } from './style';
import { numberFormat } from '#lib/i18n/index.js';

export interface FormattedPivot {
	row: number;
	col: number;
	cells: string[][]; // en-tête, groupes, total
	formats?: string[] | null; // style JSON par colonne
	/** Mise en forme de chaque colonne (style JSON) : ses lignes, et son en-tête (internal/canvas/pivotstyle.go). */
	styles?: string[] | null;
	headStyles?: string[] | null;
	error?: string;
}

/** Couche de format par défaut de la cellule (r, c) si elle est une valeur de TCD. */
export function pivotLayer(pivots: FormattedPivot[], r: number, c: number): StyleLayer | undefined {
	for (const p of pivots) {
		if (p.error || !p.formats?.length || !p.cells.length) continue;
		// La ligne d'en-tête n'a pas de format de nombre.
		if (r <= p.row || r >= p.row + p.cells.length || c < p.col || c >= p.col + p.cells[0].length) continue;
		return parseStyle(p.formats[c - p.col]);
	}
	return undefined;
}

/**
 * Mise en forme de la colonne du TCD en (r, c) : celle de son en-tête sur
 * la ligne d'en-tête, celle de ses lignes ailleurs (détail, groupes,
 * sous-totaux, total). Elle suit la colonne (tri, actualisation), au-dessus
 * des lignes et colonnes de la feuille, sous les styles de cellule.
 */
export function pivotColLayer(pivots: FormattedPivot[], r: number, c: number): StyleLayer | undefined {
	for (const p of pivots) {
		if (p.error || !p.cells.length) continue;
		if (r < p.row || r >= p.row + p.cells.length || c < p.col || c >= p.col + p.cells[0].length) continue;
		return parseStyle((r === p.row ? p.headStyles : p.styles)?.[c - p.col]);
	}
	return undefined;
}

const grouped = () => numberFormat({ maximumFractionDigits: 0 });

/**
 * Texte d'une valeur de TCD (ligne i > 0, colonne j > 0) déjà passée par
 * son format : un entier sans format (un nombre, un total) prend le
 * séparateur de milliers (« 1 200 »), comme les sommes décimales. Le
 * séparateur est une espace insécable ordinaire : l'espace fine
 * d'Intl ne se voit plus en gras (ligne du total).
 */
export function pivotValueText(text: string, i: number, j: number): string {
	if (!(i > 0 && j > 0)) return text;
	const t = /^-?\d{4,15}$/.test(text) ? grouped().format(Number(text)) : text;
	return t.replace(/ /g, ' ');
}
