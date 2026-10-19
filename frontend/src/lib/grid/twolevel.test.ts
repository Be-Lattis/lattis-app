import { describe, expect, it } from 'vitest';
import { groupSpans, headerLabel, spanAt, splitGroup } from './twolevel';

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
});
