import { describe, expect, it } from 'vitest';
import { headerOfferText, tableUnderTitle, titleAboveFilled } from './titlehead';

const t = { name: 'T', row: 3, col: 1, columns: [{}, {}, {}] };

describe('ligne au-dessus d’un tableau', () => {
	it('trouve le tableau juste dessous, sur ses colonnes', () => {
		expect(tableUnderTitle([t], 2, 1)).toBe(t);
		expect(tableUnderTitle([t], 2, 3)).toBe(t);
		expect(tableUnderTitle([t], 2, 0)).toBeUndefined();
		expect(tableUnderTitle([t], 2, 4)).toBeUndefined();
		expect(tableUnderTitle([t], 1, 1)).toBeUndefined();
		expect(tableUnderTitle([t], 3, 1)).toBeUndefined();
	});

	it('pas sur des en-têtes sur deux lignes', () => {
		expect(tableUnderTitle([{ ...t, twoLevel: true }], 2, 1)).toBeUndefined();
		expect(titleAboveFilled({ ...t, twoLevel: true }, () => 'x')).toBe(false);
	});

	it('demande une cellule remplie au-dessus des colonnes', () => {
		const cells = new Map([['2,0', 'Liste'], ['2,4', 'Note']]);
		const raw = (r: number, c: number) => cells.get(`${r},${c}`);
		expect(titleAboveFilled(t, raw)).toBe(false);
		cells.set('2,2', '  ');
		expect(titleAboveFilled(t, raw)).toBe(false);
		cells.set('2,3', 'Ville');
		expect(titleAboveFilled(t, raw)).toBe(true);
		expect(titleAboveFilled({ ...t, row: 0 }, () => 'x')).toBe(false);
	});

	it('nomme le tableau dans la proposition', () => {
		expect(headerOfferText('Ventes')).toBe('Cette ligne ressemble aux en-têtes de « Ventes »');
	});
});
