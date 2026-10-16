// Volets figés (Affichage › Figer les volets), comme Excel : les premières
// lignes et colonnes restent en place, le reste défile dessous. La grille
// se dessine en quatre volets au plus, chacun avec son décalage et sa
// découpe ; les positions à l'écran et les clics passent par ces fonctions.

/** Un volet : décalage de défilement (left, top) et zone à l'écran [x0, x1[ × [y0, y1[. */
export interface Pane {
	left: number;
	top: number;
	x0: number;
	x1: number;
	y0: number;
	y1: number;
}

/**
 * Volets de la grille. frozenW et frozenH : largeur des colonnes figées et
 * hauteur des lignes figées (0 : rien de figé sur cet axe) ; head et margin
 * : bandeau du haut et marge de gauche ; w et h : taille de la vue.
 */
export function panes(frozenW: number, frozenH: number, margin: number, head: number, w: number, h: number, scrollX: number, scrollY: number): Pane[] {
	const cols = frozenW > 0 ? [{ left: 0, x0: margin, x1: Math.min(w, margin + frozenW) }, { left: scrollX, x0: Math.min(w, margin + frozenW), x1: w }] : [{ left: scrollX, x0: margin, x1: w }];
	const rows = frozenH > 0 ? [{ top: 0, y0: head, y1: Math.min(h, head + frozenH) }, { top: scrollY, y0: Math.min(h, head + frozenH), y1: h }] : [{ top: scrollY, y0: head, y1: h }];
	return rows.flatMap((r) => cols.map((c) => ({ ...c, ...r }))).filter((p) => p.x1 > p.x0 && p.y1 > p.y0);
}

/** Position à l'écran du début d'une case (offset : sa position sur l'axe). */
export function toScreen(offset: number, frozen: boolean, scroll: number, head: number): number {
	return head + offset - (frozen ? 0 : scroll);
}

/** Position sur l'axe d'un point de l'écran (frozenSize : taille de la partie figée). */
export function toGrid(px: number, head: number, frozenSize: number, scroll: number): number {
	const g = px - head;
	return g < frozenSize ? g : g + scroll;
}

/**
 * Défilement qui montre la case [start, end[ sous la partie figée (de
 * taille frozen) d'une vue de taille view ; une case figée ne fait pas
 * défiler.
 */
export function reveal(start: number, end: number, scroll: number, view: number, frozen: number): number {
	if (start < frozen) return scroll;
	if (start - scroll < frozen) return Math.max(0, start - frozen);
	if (end - scroll > view) return Math.max(0, Math.min(start - frozen, end - view));
	return scroll;
}
