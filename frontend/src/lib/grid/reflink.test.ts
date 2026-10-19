import { describe, expect, it } from 'vitest';
import { citeLabel, MAX_BACK, popBack, pushBack, refTarget, type RefSpot } from './reflink';

const tables = [
	{ name: 'Espèces', columns: ['Nom', 'Famille'], key: 0 },
	{ name: 'Nids', columns: ['Nid', 'Espèce'], key: -1 },
	{ name: 'Sites', columns: ['Code'] }
];

describe('colonne de référence', () => {
	it('colonne clé entière : référence (sans casse, avec ou sans =)', () => {
		expect(refTarget('Espèces[Nom]', tables)).toEqual({ table: 'Espèces', key: 'Nom' });
		expect(refTarget('=espèces[ nom ]', tables)).toEqual({ table: 'Espèces', key: 'Nom' });
	});
	it('autre colonne, tableau sans clé, lignes, plage : pas une référence', () => {
		expect(refTarget('Espèces[Famille]', tables)).toBeNull();
		expect(refTarget('Nids[Nid]', tables)).toBeNull();
		expect(refTarget('Sites[Code]', tables)).toBeNull();
		expect(refTarget('Espèces[Nom][1:3]', tables)).toBeNull();
		expect(refTarget('Cell[A1:A5]', tables)).toBeNull();
		expect(refTarget('Feuille2!Cell[A1:A5]', tables)).toBeNull();
		expect(refTarget('Inconnu[Nom]', tables)).toBeNull();
		expect(refTarget(undefined, tables)).toBeNull();
	});
});

describe('pile des retours', () => {
	const at = (row: number): RefSpot => ({ sheet: 1, row, col: 2, label: `F!C${row + 1}` });
	it('empile, sans doublon au sommet, et dépile', () => {
		let s = pushBack([], at(4));
		s = pushBack(s, at(4));
		s = pushBack(s, at(9));
		expect(s.map((x) => x.row)).toEqual([4, 9]);
		const { spot, rest } = popBack(s);
		expect(spot?.row).toBe(9);
		expect(rest.map((x) => x.row)).toEqual([4]);
		expect(popBack([]).spot).toBeUndefined();
	});
	it('bornée', () => {
		let s: RefSpot[] = [];
		for (let i = 0; i < MAX_BACK + 5; i++) s = pushBack(s, at(i));
		expect(s.length).toBe(MAX_BACK);
		expect(s[0].row).toBe(5);
	});
});

describe('lignes qui citent', () => {
	it('total des tableaux, ou aucune', () => {
		expect(citeLabel('Pie', [{ table: 'Nids', column: 'Espèce', count: 3, first: 1 }, { table: 'Vus', column: 'Espèce', count: 1, first: 4 }])).toBe('Lignes qui citent « Pie » (4)');
		expect(citeLabel('Pie', [{ table: 'Nids', column: 'Espèce', count: 0, first: 0 }])).toBe('Aucune ligne ne cite « Pie »');
	});
});
