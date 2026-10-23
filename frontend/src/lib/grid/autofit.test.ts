import { describe, expect, it } from 'vitest';
import { fitColumns, fitRows, fitsSelection } from './autofit.js';

describe('fitsSelection', () => {
	it('vaut pour la sélection quand la poignée en fait partie et qu’elle compte plusieurs lignes ou colonnes', () => {
		expect(fitsSelection(2, 0, 5)).toBe(true);
		expect(fitsSelection(0, 0, 5)).toBe(true);
		expect(fitsSelection(5, 0, 5)).toBe(true);
	});
	it('hors de la sélection, ou une seule colonne sélectionnée : la poignée seule', () => {
		expect(fitsSelection(6, 0, 5)).toBe(false);
		expect(fitsSelection(3, 3, 3)).toBe(false);
	});
});

describe('fitColumns', () => {
	it('toute la feuille : chaque colonne sélectionnée, sauf les masquées', () => {
		expect(fitColumns(1, 0, 4, new Set([2]))).toEqual([0, 1, 3, 4]);
	});
	it('poignée hors de la sélection : sa colonne seule', () => {
		expect(fitColumns(7, 0, 4, new Set())).toEqual([7]);
	});
});

describe('fitRows', () => {
	it('lignes sélectionnées : seulement celles qui ont une hauteur choisie, dans l’ordre', () => {
		expect(fitRows(2, 0, 999_999, [40, 3, 2_000_000, 0])).toEqual([0, 3, 40]);
	});
	it('poignée hors de la sélection : sa ligne seule', () => {
		expect(fitRows(10, 0, 5, [1, 2])).toEqual([10]);
	});
});
