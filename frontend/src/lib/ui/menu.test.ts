import { describe, expect, it } from 'vitest';
import { clip, clipped, flyout } from './menu';

describe('noms abrégés dans les menus', () => {
	it('un nom court reste tel quel', () => {
		expect(clip('Ventes')).toBe('Ventes');
		expect(clipped('Ventes')).toBe(false);
	});
	it('un nom long finit par « … » à 30 caractères', () => {
		const long = 'Montant total des ventes réalisées par le magasin au cours du mois';
		const c = clip(long);
		expect(Array.from(c)).toHaveLength(30);
		expect(c.endsWith('…')).toBe(true);
		expect(long.startsWith(c.slice(0, -1))).toBe(true);
		expect(clipped(long)).toBe(true);
	});
	it('pas d’espace avant « … », ni de retour à la ligne', () => {
		expect(clip('abcd efgh', 6)).toBe('abcd…');
		expect(clip('un\nnom', 30)).toBe('un nom');
	});
	it('nom absent : chaîne vide', () => expect(clip(undefined)).toBe(''));
	it('ne coupe pas un caractère hors du plan de base', () => expect(clip('😀😀😀😀', 3)).toBe('😀😀…'));
});

describe('place d’un sous-menu', () => {
	const box = { left: 0, top: 0, right: 1000, bottom: 800 };
	const item = { left: 300, top: 100, right: 600, bottom: 126 };
	it('à droite quand il y a la place', () => expect(flyout(item, 200, 150, box)).toEqual({ left: false, dy: 0 }));
	it('à gauche près du bord droit', () => expect(flyout({ ...item, left: 700, right: 990 }, 200, 150, box).left).toBe(true));
	it('du côté le plus large si aucun ne suffit', () => {
		expect(flyout({ ...item, left: 90, right: 900 }, 200, 150, box).left).toBe(false);
		expect(flyout({ ...item, left: 160, right: 950 }, 200, 150, box).left).toBe(true);
	});
	it('remonté s’il dépasse en bas, sans passer le haut', () => {
		expect(flyout({ ...item, top: 700, bottom: 726 }, 200, 150, box).dy).toBe(-(695 + 150 - 796));
		expect(flyout({ ...item, top: 20, bottom: 46 }, 200, 900, box).dy).toBe(-11);
	});
});
