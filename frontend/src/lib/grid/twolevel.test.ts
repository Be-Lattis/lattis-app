import { describe, expect, it } from 'vitest';
import { bandMerges, groupSpans, headerLabel, spanAt, splitGroup, tableTop } from './twolevel';

describe('en-têtes sur deux lignes', () => {
	it('sépare au premier point', () => {
		expect(splitGroup('BOISEMENT.Nom')).toEqual({ group: 'BOISEMENT', sub: 'Nom' });
		expect(splitGroup('Latitude.Lambert 93.X')).toEqual({ group: 'Latitude', sub: 'Lambert 93.X' });
		expect(splitGroup('Nom')).toEqual({ group: '', sub: 'Nom' });
		expect(splitGroup('.x')).toEqual({ group: '', sub: '.x' });
		expect(splitGroup('x.')).toEqual({ group: '', sub: 'x.' });
	});

	it('ne coupe pas un nom quand les en-têtes sont sur une ligne', () => {
		expect(headerLabel('Prix H.T.', false)).toBe('Prix H.T.');
		expect(headerLabel('BOISEMENT.Nom', true)).toBe('Nom');
	});

	it('réunit les colonnes consécutives du même groupe', () => {
		const names = ['BOISEMENT.Nom', 'BOISEMENT.Statut', 'Note', 'Latitude.DD', 'Latitude.L93', 'BOISEMENT.Autre'];
		expect(groupSpans(names)).toEqual([
			{ group: 'BOISEMENT', i0: 0, i1: 1 },
			{ group: '', i0: 2, i1: 2 },
			{ group: 'Latitude', i0: 3, i1: 4 },
			{ group: 'BOISEMENT', i0: 5, i1: 5 }
		]);
		expect(spanAt(names, 4)).toEqual({ group: 'Latitude', i0: 3, i1: 4 });
		// Deux colonnes sans groupe gardent chacune leur cellule.
		expect(groupSpans(['A', 'B'])).toHaveLength(2);
	});

	it('garde fusionnés les groupes repris de cellules fusionnées', () => {
		const names = ['N° DE NID.Nid', 'N° DE NID.Ref', 'BOISEMENT.Nom', 'BOISEMENT.Statut', 'BOISEMENT.Parcelle', 'Note'];
		expect(groupSpans(names).map((s) => [s.i0, s.i1])).toEqual([
			[0, 1],
			[2, 4],
			[5, 5]
		]);
	});

	it('commence le tableau à sa bande', () => {
		expect(tableTop({ row: 3 })).toBe(3);
		expect(tableTop({ row: 3, twoLevel: true })).toBe(2);
	});

	it('donne les groupes de plusieurs colonnes comme des fusions', () => {
		const columns = ['N° DE NID.Nid', 'N° DE NID.Ref', 'Note', 'BOISEMENT.Nom', 'BOISEMENT.Statut'].map((name) => ({ name }));
		expect(bandMerges([{ row: 3, col: 1, twoLevel: true, columns }])).toEqual([
			{ row: 2, col: 1, rows: 1, cols: 2 },
			{ row: 2, col: 4, rows: 1, cols: 2 }
		]);
		expect(bandMerges([{ row: 3, col: 1, columns }])).toEqual([]);
	});
});
