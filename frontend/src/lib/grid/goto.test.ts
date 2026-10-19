import { describe, expect, it } from 'vitest';
import { a1Like, parseA1, parseGoto, type GotoTable } from './goto';

const ventes: GotoTable = { name: 'Ventes', row: 2, col: 1, last: 7, lastCol: 3, columns: ['Produit', 'Prix', 'Qté'] };
const tables = [ventes];

describe('zone de nom : aller à', () => {
	it('une adresse A1', () => {
		expect(parseGoto('F10', tables)).toEqual({ r0: 9, c0: 5, r1: 9, c1: 5 });
		expect(parseGoto(' $f$10 ', tables)).toEqual({ r0: 9, c0: 5, r1: 9, c1: 5 });
		expect(parseGoto('Cell[F10]', tables)).toEqual({ r0: 9, c0: 5, r1: 9, c1: 5 });
		expect(parseGoto('cell_B3', tables)).toEqual({ r0: 2, c0: 1, r1: 2, c1: 1 });
		expect(parseA1('AA1')).toEqual([0, 26]);
	});
	it('une plage', () => {
		expect(parseGoto('C3:A1', tables)).toEqual({ r0: 0, c0: 0, r1: 2, c1: 2 });
		expect(parseGoto('Cell[A1:C3]', tables)).toEqual({ r0: 0, c0: 0, r1: 2, c1: 2 });
	});
	it('un tableau, une colonne, une cellule de tableau', () => {
		expect(parseGoto('ventes', tables)).toEqual({ r0: 2, c0: 1, r1: 7, c1: 3 });
		expect(parseGoto('Ventes[Prix]', tables)).toEqual({ r0: 3, c0: 2, r1: 7, c1: 2 });
		expect(parseGoto('Ventes[Prix][2]', tables)).toEqual({ r0: 4, c0: 2, r1: 4, c1: 2 });
		expect(parseGoto('Ventes[Prix][1]:Ventes[Qté][3]', tables)).toEqual({ r0: 3, c0: 2, r1: 5, c1: 3 });
		expect(parseGoto('Prix[2]', tables, ventes)).toEqual({ r0: 4, c0: 2, r1: 4, c1: 2 });
	});
	it('rien pour un nom nouveau ou une référence hors du tableau', () => {
		expect(parseGoto('Clients', tables)).toBeNull();
		expect(parseGoto('Ventes[Remise]', tables)).toBeNull();
		expect(parseGoto('Ventes[Prix][9]', tables)).toBeNull();
		expect(parseGoto('Prix[2]', tables)).toBeNull();
		expect(parseGoto('', tables)).toBeNull();
	});
	it('un tableau existant passe avant une adresse', () => {
		const q1: GotoTable = { ...ventes, name: 'Q1' };
		expect(parseGoto('Q1', [q1])).toEqual({ r0: 2, c0: 1, r1: 7, c1: 3 });
	});
	it('les noms qui se lisent comme une adresse', () => {
		for (const n of ['F10', 'ab12', 'R1C1']) expect(a1Like(n)).toBe(true);
		for (const n of ['Tableau1', 'F10_', 'A0', 'ABCDE1']) expect(a1Like(n)).toBe(false);
	});
});
