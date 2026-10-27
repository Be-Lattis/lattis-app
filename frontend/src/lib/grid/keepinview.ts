// Objet gardé à l'écran quand un volet s'ouvre à droite (champs du TCD,
// format du graphique, assistant, conflits…) : la grille rétrécit et ce
// qui était au bord droit passerait dessous. On fait défiler (le contenu
// part vers la gauche) pour montrer l'objet en entier s'il tient, sinon
// son début, ou la cellule active.
//
// Positions en pixels de la feuille sur l'axe des colonnes ; scroll :
// défilement horizontal ; view : largeur de la zone des cellules (sans la
// marge des numéros de ligne) ; frozen : largeur des colonnes figées.

import type { Offsets } from './panes';

/** Étendue [x0, x1[ d'un objet sur l'axe des colonnes. */
export interface Span {
	x0: number;
	x1: number;
}

/** Une partie de l'objet se voit-elle (figé : toujours, il ne défile pas) ? */
export function seenIn(o: Span, scroll: number, view: number, frozen: number): boolean {
	if (o.x0 < frozen) return true;
	return o.x1 > scroll + frozen && o.x0 < scroll + view;
}

/** L'objet se voit-il en entier ? */
export function wholeIn(o: Span, scroll: number, view: number, frozen: number): boolean {
	return o.x0 >= scroll + frozen && o.x1 <= scroll + view;
}

/**
 * Défilement qui montre l'objet o : en entier s'il tient (défilement
 * minimal, la première colonne montrée n'est pas coupée quand axis est
 * donné) ; sinon depuis son début, ou de quoi voir must (la cellule
 * active) si son début ne la montre pas. Inchangé s'il se voit déjà, ou
 * s'il commence dans les colonnes figées.
 */
export function keepInView(o: Span, scroll: number, view: number, frozen: number, axis?: Offsets, must?: Span): number {
	const avail = view - frozen;
	if (o.x0 < frozen || avail <= 0 || wholeIn(o, scroll, view, frozen)) return scroll;
	if (o.x1 - o.x0 > avail) {
		const s = Math.max(0, o.x0 - frozen);
		if (must && !wholeIn(must, s, view, frozen)) return keepInView(must, scroll, view, frozen, axis);
		return s;
	}
	if (o.x0 < scroll + frozen) return Math.max(0, o.x0 - frozen);
	let s = o.x1 - view;
	if (axis) {
		const c = axis.indexAt(s + frozen);
		if (axis.offset(c) < s + frozen) s = Math.min(axis.offset(c + 1) - frozen, o.x0 - frozen);
	}
	return Math.max(0, s);
}

/**
 * Largeur de la grille passée de oldW à newW (même zoom) : 'keep' si elle
 * a rétréci (un volet s'ouvre, l'objet est gardé à l'écran), 'stay' sinon
 * (volet fermé : la feuille ne bouge pas, elle s'élargit à droite).
 */
export function afterWidthChange(oldW: number, newW: number): 'keep' | 'stay' {
	return oldW > 0 && newW < oldW ? 'keep' : 'stay';
}

/**
 * Place à droite de la grille jusqu'au bord de la fenêtre (volets ouverts,
 * dans la feuille ou autour), en px de la feuille (k : zoom). La zone qui
 * défile la réserve d'avance : à la fermeture d'un volet, la grille
 * s'élargit sans que le navigateur ramène la barre en arrière.
 */
export function sideRoom(windowRight: number, gridRight: number, k: number): number {
	return Math.max(0, windowRight - gridRight) / (k > 0 ? k : 1);
}
