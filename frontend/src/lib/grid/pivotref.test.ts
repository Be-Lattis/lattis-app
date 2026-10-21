import { describe, expect, it } from 'vitest';
import { pivotDetailRows, pivotRangeRef, pivotRefText, type RefPivot } from './pivotref';

// TCD en B2 : NID, Nombre, puis deux colonnes saisies (note, double).
const flat: RefPivot = {
	row: 1,
	col: 1,
	config: { name: 'TCD1', rowFields: ['NID'] },
	cells: [
		['NID', 'Nombre', 'note', 'double'],
		['A', '2', 'x', '4'],
		['B', '1', '', '2'],
		['Total', '3', '', '6']
	],
	hand: [{ name: 'note' }, { name: 'double', formula: '=[Nombre]*2' }]
};

// Arborescence : Service › Poste, sous-totaux en haut, total général.
const tree: RefPivot = {
	row: 0,
	col: 0,
	config: { name: 'TCD2', rowFields: ['Service', 'Poste'] },
	cells: [
		['Étiquettes de lignes', 'Somme de Salaire'],
		['RH', '90'],
		['Chargé', '30'],
		['Chef', '60'],
		['Total', '90']
	],
	rows: [
		{ depth: 0, group: true, path: ['RH'] },
		{ depth: 1, path: ['RH', 'Chargé'] },
		{ depth: 1, path: ['RH', 'Chef'] },
		{ depth: 0, total: true }
	]
};

describe('références vers un TCD', () => {
	it('lignes de détail : ni total, ni groupe', () => {
		expect(pivotDetailRows(flat)).toEqual([1, 2]);
		expect(pivotDetailRows({ ...flat, config: { ...flat.config, noGrandTotal: true } })).toEqual([1, 2, 3]);
		expect(pivotDetailRows(tree)).toEqual([2, 3]);
		// Sans champ en lignes : la seule ligne compte.
		expect(pivotDetailRows({ ...flat, config: { name: 'TCD1' }, cells: [['Nombre'], ['3']] })).toEqual([1]);
	});

	it('depuis une cellule libre : TCD1[Col][n], la colonne sur un en-tête ou un total, jamais Cell[…]', () => {
		const free = { row: 10, col: 0 };
		expect(pivotRefText(flat, 2, 2, free)).toBe('TCD1[Nombre][1]');
		expect(pivotRefText(flat, 3, 1, free)).toBe('TCD1[NID][2]');
		expect(pivotRefText(flat, 1, 2, free)).toBe('TCD1[Nombre]');
		expect(pivotRefText(flat, 4, 2, free)).toBe('TCD1[Nombre]');
		expect(pivotRefText(flat, 2, 4, free)).toBe('TCD1[double][1]');
		expect(pivotRefText(flat, 2, 2, null)).toBe('TCD1[Nombre][1]'); // autre feuille
	});

	it('dans une colonne saisie du TCD : [Col] sur sa ligne', () => {
		expect(pivotRefText(flat, 2, 2, { row: 2, col: 4 })).toBe('[Nombre]');
		expect(pivotRefText(flat, 2, 3, { row: 2, col: 4 })).toBe('[note]');
		expect(pivotRefText(flat, 4, 2, { row: 4, col: 4 })).toBe('[Nombre]'); // total : sa propre ligne
		expect(pivotRefText(flat, 3, 2, { row: 2, col: 4 })).toBe('TCD1[Nombre][2]');
		// Juste à droite du TCD (pas une de ses colonnes) : une cellule libre.
		expect(pivotRefText(flat, 2, 2, { row: 2, col: 5 })).toBe('TCD1[Nombre][1]');
	});

	it('arborescence : le champ du niveau dans la colonne des étiquettes', () => {
		expect(pivotRefText(tree, 2, 0, null)).toBe('TCD2[Poste][1]');
		expect(pivotRefText(tree, 1, 0, null)).toBe('TCD2[Service]'); // groupe : la colonne de son champ
		expect(pivotRefText(tree, 3, 1, null)).toBe('TCD2[Somme de Salaire][2]');
		expect(pivotRefText(tree, 0, 1, null)).toBe('TCD2[Somme de Salaire]');
	});

	it('argument qui attend un tableau ou une colonne', () => {
		expect(pivotRefText(flat, 2, 2, null, 'table')).toBe('TCD1');
		expect(pivotRefText(flat, 2, 2, null, 'column')).toBe('TCD1[Nombre]');
	});

	it('hors du TCD, sans nom ou en erreur : rien', () => {
		expect(pivotRefText(flat, 0, 1, null)).toBeNull();
		expect(pivotRefText(flat, 2, 5, null)).toBeNull();
		expect(pivotRefText({ ...flat, config: { rowFields: ['NID'] } }, 2, 2, null)).toBeNull();
		expect(pivotRefText({ ...flat, error: 'x' }, 2, 2, null)).toBeNull();
	});

	it('plage glissée : une colonne donne TCD1[Col]', () => {
		expect(pivotRangeRef(flat, 2, 2, 3, 2)).toBe('TCD1[Nombre]');
		expect(pivotRangeRef(flat, 2, 1, 3, 2)).toBeNull();
		expect(pivotRangeRef(flat, 2, 2, 6, 2)).toBeNull();
	});
});
