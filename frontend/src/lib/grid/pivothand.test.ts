import { describe, expect, it } from 'vitest';
import { handAlign, handEditText, handKeptNote, handStart, pivotHandAt, type HandPivot } from './pivothand';

const p: HandPivot = {
	row: 1,
	col: 1,
	cells: [
		['NID', 'Nombre', 'test', 'double'],
		['A', '2', 'x', '4'],
		['Total', '2', '', '4']
	],
	hand: [{ name: 'test' }, { name: 'double', formula: '=[Nombre]*2' }]
};

describe('colonnes saisies d’un TCD', () => {
	it('situe les colonnes saisies à droite des cellules calculées', () => {
		expect(handStart(p)).toBe(3);
		expect(pivotHandAt(p, 1, 3)).toBe(0);
		expect(pivotHandAt(p, 3, 4)).toBe(1);
		expect(pivotHandAt(p, 2, 2)).toBe(-1); // colonne du TCD
		expect(pivotHandAt(p, 4, 3)).toBe(-1); // sous le TCD
		expect(pivotHandAt(p, 1, 5)).toBe(-1); // à droite
		expect(pivotHandAt({ ...p, hand: null }, 1, 3)).toBe(-1);
	});
	it('édite la formule de la colonne, ou la valeur', () => {
		expect(handEditText(p, 2, 4, '4')).toBe('=[Nombre]*2');
		expect(handEditText(p, 1, 4, 'double')).toBe('double'); // en-tête : le nom
		expect(handEditText(p, 2, 3, 'x')).toBe('x');
	});
	it('aligne les nombres à droite', () => {
		expect(handAlign('12,5')).toBe('right');
		expect(handAlign('1 200')).toBe('right');
		expect(handAlign('15 %')).toBe('right');
		expect(handAlign('oui')).toBe('left');
		expect(handAlign('01/02/2026')).toBe('left');
	});
	it('dit les valeurs mises de côté', () => {
		expect(handKeptNote(0, '')).toBe('');
		expect(handKeptNote(2, '')).toContain('2 lignes');
		expect(handKeptNote(1, 'Aucun champ')).toBe('Aucun champ');
	});
});
