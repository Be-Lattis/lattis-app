// Rechercher / remplacer dans la sélection, comme Excel : une sélection de
// plusieurs cellules borne la recherche (suivant, précédent, remplacer,
// remplacer tout) ; une seule cellule (ou sa fusion) : toute la feuille.

export interface Zone {
	r0: number;
	c0: number;
	r1: number;
	c1: number;
}

export interface Hit {
	row: number;
	col: number;
}

/** Zone de recherche : la sélection si elle compte plusieurs cellules, sinon null (toute la feuille). */
export function findZone(sel: Zone, single: boolean): Zone | null {
	return single ? null : { r0: sel.r0, c0: sel.c0, r1: sel.r1, c1: sel.c1 };
}

/** La cellule est-elle dans la zone (null : toute la feuille) ? */
export function inZone(h: Hit, z: Zone | null): boolean {
	return !z || (h.row >= z.r0 && h.row <= z.r1 && h.col >= z.c0 && h.col <= z.c1);
}

/** Clé de cache d'une recherche : feuille, texte et zone. */
export function findKey(text: string, z: Zone | null, sheet = 0): string {
	return `${sheet}\u0000${text}` + (z ? `\u0000${z.r0},${z.c0},${z.r1},${z.c1}` : '');
}

/**
 * Indice de l'occurrence suivante (ou précédente, back) après la cellule
 * from, dans l'ordre de lecture (ligne puis colonne), en repartant du début
 * (ou de la fin) ; from absent : la première (ou la dernière). -1 si aucune.
 */
export function nextHit(hits: readonly Hit[], from: Hit | null, back: boolean): number {
	if (!hits.length) return -1;
	if (!from) return back ? hits.length - 1 : 0;
	const { row, col } = from;
	const i = back
		? hits.findLastIndex((m) => m.row < row || (m.row === row && m.col < col))
		: hits.findIndex((m) => m.row > row || (m.row === row && m.col > col));
	return i >= 0 ? i : back ? hits.length - 1 : 0;
}
