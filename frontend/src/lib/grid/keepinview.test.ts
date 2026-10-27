import { describe, expect, it } from 'vitest';
import { afterWidthChange, keepInView, seenIn, sideRoom, wholeIn } from './keepinview';

// Colonnes de 100 px.
const axis = { offset: (i: number) => i * 100, indexAt: (x: number) => Math.floor(x / 100) };

describe('objet gardé à l’écran quand un volet s’ouvre', () => {
	it('déjà visible en entier : pas de défilement', () => {
		expect(keepInView({ x0: 200, x1: 400 }, 0, 800, 0, axis)).toBe(0);
	});

	it('TCD au bord droit, la grille passe de 1000 à 700 px : il revient à gauche', () => {
		const pv = { x0: 700, x1: 950 };
		expect(seenIn(pv, 0, 1000, 0)).toBe(true);
		expect(seenIn(pv, 0, 700, 0)).toBe(false);
		// 950 - 700 = 250, calé sur la colonne suivante pour ne pas la couper.
		const s = keepInView(pv, 0, 700, 0, axis);
		expect(s).toBe(300);
		expect(wholeIn(pv, s, 700, 0)).toBe(true);
	});

	it('sans axe : défilement minimal', () => {
		expect(keepInView({ x0: 700, x1: 950 }, 0, 700, 0)).toBe(250);
	});

	it('le calage sur une colonne ne dépasse pas le début de l’objet', () => {
		expect(keepInView({ x0: 650, x1: 950 }, 0, 700, 0, axis)).toBe(300);
		// Graphique posé au milieu d'une colonne, presque aussi large que la vue.
		expect(keepInView({ x0: 260, x1: 950 }, 0, 700, 0, axis)).toBe(260);
	});

	it('objet trop large : son début, ou la cellule active', () => {
		const wide = { x0: 600, x1: 1600 };
		expect(keepInView(wide, 0, 700, 0, axis)).toBe(600);
		// La cellule active se voit depuis le début du TCD.
		expect(keepInView(wide, 0, 700, 0, axis, { x0: 800, x1: 900 })).toBe(600);
		// Plus loin : on la montre elle.
		expect(keepInView(wide, 0, 700, 0, axis, { x0: 1400, x1: 1500 })).toBe(800);
		// Début du TCD à l'écran, coupé à droite : on le cale à gauche pour en montrer le plus.
		expect(keepInView(wide, 300, 700, 0, axis, { x0: 700, x1: 800 })).toBe(600);
		// Graphique plus large que la vue : son début (must : ses 100 premiers px).
		expect(keepInView({ x0: 1000, x1: 1500 }, 531, 480, 0, axis, { x0: 1000, x1: 1100 })).toBe(1000);
	});

	it('à gauche, sous les colonnes figées', () => {
		expect(keepInView({ x0: 300, x1: 400 }, 250, 700, 100, axis)).toBe(200);
	});

	it('colonnes figées : un objet figé ne fait pas défiler', () => {
		expect(keepInView({ x0: 0, x1: 900 }, 0, 700, 100, axis)).toBe(0);
		expect(seenIn({ x0: 0, x1: 50 }, 500, 700, 100)).toBe(true);
	});

	it('défilement jamais négatif', () => {
		expect(keepInView({ x0: 0, x1: 50 }, 30, 700, 0)).toBe(0);
	});
});

describe('volet fermé : la feuille ne bouge pas', () => {
	it('gardé à l’écran à l’ouverture seulement', () => {
		expect(afterWidthChange(1000, 700)).toBe('keep');
		expect(afterWidthChange(700, 1000)).toBe('stay');
		expect(afterWidthChange(700, 700)).toBe('stay');
		// Premier affichage (ou zoom changé) : rien à garder.
		expect(afterWidthChange(0, 700)).toBe('stay');
	});

	it('place des volets réservée à droite, en px de la feuille', () => {
		// Volet de 300 px à droite de la grille (fenêtre de 1300 px).
		expect(sideRoom(1300, 1000, 1)).toBe(300);
		expect(sideRoom(1300, 1000, 1.5)).toBe(200);
		// Aucun volet : rien à réserver, jamais négatif.
		expect(sideRoom(1300, 1300, 1)).toBe(0);
		expect(sideRoom(1300, 1310, 1)).toBe(0);
		expect(sideRoom(1300, 1000, 0)).toBe(300);
	});
});
