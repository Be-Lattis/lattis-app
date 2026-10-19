// Ctrl + flèche, comme Excel, sur une ligne ou une colonne de la feuille :
// dans un bloc de cellules remplies, jusqu'à son bord ; au bord d'un bloc ou
// depuis une cellule vide, jusqu'à la prochaine cellule remplie. Les blocs
// sont des intervalles [début, fin] (un tableau, une cellule libre…).

//
// Un tableau compte comme un bloc plein, de sa bande des groupes (en-têtes
// sur deux lignes) à sa dernière ligne : Ctrl+↓ dans une colonne à trous va
// à son bas (choix voulu : ses lignes ne sont pas toutes chargées, et un
// tableau se lit comme un tout ; Excel s'arrêterait au trou).

export type Span = [number, number];

/** Rectangle occupé sur la feuille (bornes comprises). */
export interface Area {
	r0: number;
	c0: number;
	r1: number;
	c1: number;
}

/** Blocs de la ligne (dc) ou de la colonne (dr) de la cellule (row, col) que traversent les zones. */
export function lineSpans(areas: Iterable<Area>, row: number, col: number, vertical: boolean): Span[] {
	const out: Span[] = [];
	for (const a of areas) {
		if (vertical && col >= a.c0 && col <= a.c1) out.push([a.r0, a.r1]);
		if (!vertical && row >= a.r0 && row <= a.r1) out.push([a.c0, a.c1]);
	}
	return out;
}

/** Fusionne des intervalles en blocs disjoints, triés (deux intervalles qui se touchent n'en font qu'un). */
export function runs(spans: Span[]): Span[] {
	const s = spans.filter(([a, b]) => b >= a).sort((x, y) => x[0] - y[0]);
	const out: Span[] = [];
	for (const [a, b] of s) {
		const last = out[out.length - 1];
		if (last && a <= last[1] + 1) last[1] = Math.max(last[1], b);
		else out.push([a, b]);
	}
	return out;
}

/**
 * Position atteinte depuis pos dans le sens dir (+1 ou -1) ; sans cellule
 * remplie au-delà : pos vers l'avant (la feuille est sans fin), 0 vers
 * l'arrière.
 */
export function jumpLine(blocks: Span[], pos: number, dir: number): number {
	const rs = runs(blocks);
	const inside = rs.find(([a, b]) => pos >= a && pos <= b);
	if (dir > 0) {
		if (inside && pos < inside[1]) return inside[1];
		const next = rs.find(([a]) => a > pos);
		return next ? next[0] : pos;
	}
	if (inside && pos > inside[0]) return inside[0];
	const prev = rs.filter(([, b]) => b < pos).pop();
	return prev ? prev[1] : 0;
}
