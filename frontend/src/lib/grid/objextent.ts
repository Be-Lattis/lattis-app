// Zone utilisée par les objets posés sur la feuille (graphiques, cartes,
// boutons et contrôles, tableaux croisés, tableaux déversés) : la barre de
// défilement va jusqu'à leur bord bas et droit, qu'ils soient sélectionnés
// ou non, pour que la zone qui défile ne change pas en les quittant.

export interface Box {
	x: number;
	y: number;
	w: number;
	h: number;
}

/**
 * Dernière ligne et dernière colonne atteintes par des objets flottants
 * (pixels de la feuille) : celles qui contiennent leur bord bas et droit.
 * -1 sans objet.
 */
export function boxesReach(boxes: Iterable<Box>, rowAt: (y: number) => number, colAt: (x: number) => number): { row: number; col: number } {
	let row = -1;
	let col = -1;
	for (const b of boxes) {
		row = Math.max(row, rowAt(b.y + Math.max(0, b.h)));
		col = Math.max(col, colAt(b.x + Math.max(0, b.w)));
	}
	return { row, col };
}

/** Dernière ligne et colonne de blocs de cellules (row, col : coin haut gauche ; rows × cols). -1 sans bloc. */
export function blocksReach(blocks: Iterable<{ row: number; col: number; rows: number; cols: number }>): { row: number; col: number } {
	let row = -1;
	let col = -1;
	for (const b of blocks) {
		if (b.rows <= 0 || b.cols <= 0) continue;
		row = Math.max(row, b.row + b.rows - 1);
		col = Math.max(col, b.col + b.cols - 1);
	}
	return { row, col };
}
