// Format de nombre par défaut des valeurs d'un tableau croisé (calculé côté
// Go, voir internal/canvas/pivotformat.go) : format de la colonne source, ou
// deux décimales pour une moyenne. C'est la couche la moins précise : la
// mise en forme posée sur les cellules (Ajouter / Réduire une décimale)
// l'emporte.

import { parseStyle, type StyleLayer } from './style';

export interface FormattedPivot {
	row: number;
	col: number;
	cells: string[][]; // en-tête, groupes, total
	formats?: string[] | null; // style JSON par colonne
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
