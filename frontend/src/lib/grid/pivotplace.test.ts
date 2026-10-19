import { describe, expect, it } from 'vitest';
import { placeAt, placedMessage, placeHint, placeKey, placesOnClick, type Placement } from './pivotplace';

const create: Placement<{ source: string }> = { kind: 'create', config: { source: 'Ventes' }, source: 'Ventes' };
const move: Placement = { kind: 'move', id: 7, name: 'TCD1', sheet: 2, row: 3, col: 1 };

describe('pose d’un tableau croisé', () => {
	it('création : le premier clic pose le coin haut gauche à la cellule, sur la feuille affichée', () => {
		expect(placeAt(create, 1, 10, 4)).toEqual({ op: 'create', row: 10, col: 4, config: { source: 'Ventes' } });
		expect(placeAt(create, 5, 0, 0)).toEqual({ op: 'create', row: 0, col: 0, config: { source: 'Ventes' } });
	});

	it('déplacement : vers une autre cellule ou une autre feuille', () => {
		expect(placeAt(move, 2, 8, 6)).toEqual({ op: 'move', id: 7, row: 8, col: 6 });
		expect(placeAt(move, 4, 3, 1)).toEqual({ op: 'move', id: 7, row: 3, col: 1 });
	});

	it('déplacement sur sa propre place : rien à faire', () => {
		expect(placeAt(move, 2, 3, 1)).toEqual({ op: 'none' });
		expect(placedMessage({ op: 'none' })).toMatch(/déjà/);
	});

	it('seul un clic gauche sur une cellule pose ; le reste navigue', () => {
		expect(placesOnClick(0, true)).toBe(true);
		expect(placesOnClick(2, true)).toBe(false); // clic droit : menu
		expect(placesOnClick(0, false)).toBe(false); // en-tête, marge, barre de défilement
	});

	it('Échap annule ; flèches, Page suivante et raccourcis passent', () => {
		expect(placeKey('Escape')).toBe('cancel');
		for (const k of ['ArrowDown', 'PageDown', 'Tab', 'Enter', 'a']) expect(placeKey(k)).toBe('pass');
	});

	it('indication de la barre d’état', () => {
		expect(placeHint(create)).toBe('Cliquez sur la cellule où placer le tableau croisé (Échap : annuler)');
		expect(placeHint(move)).toContain('« TCD1 »');
		expect(placedMessage({ op: 'create', row: 0, col: 0, config: null })).toMatch(/créé/);
		expect(placedMessage({ op: 'move', id: 1, row: 0, col: 0 })).toMatch(/déplacé/);
	});
});
