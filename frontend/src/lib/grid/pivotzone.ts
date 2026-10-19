// Zone d'un tableau croisé sur la feuille, et ce que Suppr en fait (comme
// Excel : un TCD pris en entier est supprimé, une partie seulement est
// refusée ; un TCD en erreur, réduit à la cellule de son message, est
// supprimé dès que cette cellule est effacée).

/** Ce qu'il faut d'un TCD affiché (SheetPivot). */
export interface PivotPlace {
	row: number;
	col: number;
	anchor?: number;
	cells: string[][] | null;
	error: string;
	/** Colonnes saisies à droite (les dernières de cells, voir pivothand.ts). */
	hand?: unknown[] | null;
}

export interface Range {
	r0: number;
	c0: number;
	r1: number;
	c1: number;
}

/** Zone du TCD, filtres de rapport compris ; en erreur, la cellule du message. */
export function pivotZone(p: PivotPlace): Range {
	const top = p.anchor ?? p.row;
	if (p.error || !p.cells?.length) return { r0: top, c0: p.col, r1: top, c1: p.col };
	return { r0: top, c0: p.col, r1: p.row + p.cells.length - 1, c1: p.col + (p.cells[0]?.length ?? 1) - 1 };
}

/** TCD en erreur dont la cellule du message est (r, c). */
export const brokenPivotAt = <P extends PivotPlace>(pivots: P[], r: number, c: number): P | undefined =>
	pivots.find((p) => !!p.error && (p.anchor ?? p.row) === r && p.col === c);

/**
 * Effacer la zone z : les TCD supprimés (pris en entier, ou en erreur) et
 * le premier TCD sain touché en partie seulement (l'effacement est alors
 * refusé, comme Excel).
 */
export function pivotsCleared<P extends PivotPlace>(pivots: P[], z: Range): { gone: P[]; partial?: P } {
	const gone: P[] = [];
	let partial: P | undefined;
	for (const p of pivots) {
		const q = pivotZone(p);
		if (q.r0 > z.r1 || q.r1 < z.r0 || q.c0 > z.c1 || q.c1 < z.c0) continue;
		if (q.r0 >= z.r0 && q.r1 <= z.r1 && q.c0 >= z.c0 && q.c1 <= z.c1) gone.push(p);
		// Seulement ses colonnes saisies : leurs valeurs s'effacent (le moteur s'en charge).
		else if (!p.error && p.hand?.length && z.c0 > q.c1 - p.hand.length) continue;
		else partial ??= p;
	}
	return { gone, partial };
}
