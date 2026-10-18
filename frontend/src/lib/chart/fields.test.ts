import { describe, expect, it } from 'vitest';
import { chartFields } from './kinds';

// Même règle pour une feuille et un document.
describe('axe et séries d’un nouveau graphique', () => {
	const cols = [
		{ name: 'Produit', numeric: false },
		{ name: 'Qté', numeric: true },
		{ name: 'Prix', numeric: true }
	];
	it('X : la colonne de texte ; séries : les nombres', () => expect(chartFields('bar', cols)).toEqual({ x: 'Produit', series: ['Qté', 'Prix'] }));
	it('une série : la dernière', () => expect(chartFields('gauge', cols).series).toEqual(['Prix']));
	it('nuage de points : X numérique', () => expect(chartFields('scatter', cols, 2)).toEqual({ x: 'Prix', series: ['Qté'] }));
	it('fréquences : la colonne active', () => expect(chartFields('histogram', cols, 1).series).toEqual(['Qté']));
});
