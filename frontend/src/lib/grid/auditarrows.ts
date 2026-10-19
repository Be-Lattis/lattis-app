// Flèches d'audit (Formules › Afficher les liens) : où pointe chaque flèche.
//
// Le moteur (cells.dependencies, internal/canvas/deps.go) donne pour chaque
// antécédent ou dépendant une cible : une colonne de tableau (ligne = son
// en-tête), des lignes précises d'une colonne (kind « row » : Tableau[Col][1],
// [Col][-1], Tableau[Col][2:4], la ligne trouvée par REFERENCE ; rows > 1 :
// plage encadrée), un tableau ou une cellule.

/** Cible d'une flèche (sous-ensemble de DepTarget). */
export interface ArrowTarget {
	kind: string;
	table?: string;
	row: number;
	col: number;
	rows?: number;
	note?: string;
}

/** Lignes de la feuille visées : row, et rows lignes encadrées (1 : la cellule). */
export interface ArrowSpan {
	row: number;
	rows: number;
	col: number;
}

/**
 * arrowSpan situe une cible vue depuis la cellule (row, col) du lien. Une
 * colonne du même tableau, depuis une ligne de données, est lue sur cette
 * ligne ([Quantité]*[Prix unitaire]) : la flèche part de la cellule de la
 * ligne, pas de l'en-tête — sauf si le moteur a laissé une note (ligne
 * voisine absente ou masquée : la flèche vise la colonne entière).
 */
export function arrowSpan(d: ArrowTarget, at: { row: number }, own: { name: string; row: number } | null | undefined): ArrowSpan {
	if (d.kind === 'column' && !d.note && own && d.table === own.name && at.row > own.row) return { row: at.row, rows: 1, col: d.col };
	return { row: d.row, rows: Math.max(1, d.rows ?? 1), col: d.col };
}

/** Géométrie de la grille : abscisse et largeur d'une colonne, ordonnée et hauteur d'un groupe de lignes. */
export interface ArrowGeometry {
	x(col: number): number;
	w(col: number): number;
	y(row: number): number;
	h(r0: number, r1: number): number;
}

/** Cadre (en pixels) des cellules visées. */
export function spanBox(s: ArrowSpan, g: ArrowGeometry): { x: number; y: number; w: number; h: number } {
	return { x: g.x(s.col), y: g.y(s.row), w: g.w(s.col), h: g.h(s.row, s.row + s.rows - 1) };
}

/** Point d'attache de la flèche : le centre de la cellule, ou du cadre de la plage. */
export function spanPoint(s: ArrowSpan, g: ArrowGeometry): { x: number; y: number } {
	const b = spanBox(s, g);
	return { x: b.x + b.w / 2, y: b.y + b.h / 2 };
}

/** Notes des cibles (sans doublon), pour la barre d'état. */
export function arrowNotes(list: ArrowTarget[]): string[] {
	return [...new Set(list.map((d) => d.note ?? '').filter((n) => n !== ''))];
}
