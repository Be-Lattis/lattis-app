import { describe, expect, it } from 'vitest';
import { jumpLine, lineSpans, runs } from './jump';

describe('Ctrl + flèche', () => {
	// Colonne : un tableau en 2..6 (en-tête compris), des cellules libres en 9, 10, 11 et 20.
	const col: [number, number][] = [
		[2, 6],
		[9, 9],
		[10, 10],
		[11, 11],
		[20, 20]
	];
	it('assemble les blocs voisins', () => {
		expect(runs(col)).toEqual([
			[2, 6],
			[9, 11],
			[20, 20]
		]);
		expect(runs([[3, 4], [5, 7], [1, 1]])).toEqual([[1, 1], [3, 7]]);
	});
	it('va au bord du bloc, puis au bloc suivant', () => {
		expect(jumpLine(col, 2, 1)).toBe(6); // dans le tableau : sa dernière ligne
		expect(jumpLine(col, 4, 1)).toBe(6);
		expect(jumpLine(col, 6, 1)).toBe(9); // au bord : le bloc suivant
		expect(jumpLine(col, 9, 1)).toBe(11); // cellules libres jointives : un bloc
		expect(jumpLine(col, 11, 1)).toBe(20);
		expect(jumpLine(col, 20, 1)).toBe(20); // plus rien : ne bouge pas
		expect(jumpLine(col, 0, 1)).toBe(2); // cellule vide : la prochaine remplie
		expect(jumpLine(col, 15, 1)).toBe(20);
	});
	it('remonte de même', () => {
		expect(jumpLine(col, 20, -1)).toBe(11);
		expect(jumpLine(col, 11, -1)).toBe(9);
		expect(jumpLine(col, 9, -1)).toBe(6);
		expect(jumpLine(col, 5, -1)).toBe(2);
		expect(jumpLine(col, 2, -1)).toBe(0); // plus rien : en haut
		expect(jumpLine(col, 15, -1)).toBe(11);
		expect(jumpLine([], 7, -1)).toBe(0);
	});
	it('un tableau est un bloc plein, bande des groupes comprise (audit 2 M10)', () => {
		// Tableau aux en-têtes sur deux lignes : bande en 1, en-tête en 2, données 3..8 ;
		// cellule libre en 12, colonne C (2).
		const areas = [
			{ r0: 1, c0: 0, r1: 8, c1: 3 },
			{ r0: 12, c0: 2, r1: 12, c1: 2 }
		];
		const down = lineSpans(areas, 0, 2, true);
		expect(down).toEqual([
			[1, 8],
			[12, 12]
		]);
		expect(jumpLine(down, 0, 1)).toBe(1); // depuis au-dessus : la bande
		expect(jumpLine(down, 4, 1)).toBe(8); // dans le tableau, même à trous : son bas
		expect(jumpLine(down, 8, 1)).toBe(12);
		expect(lineSpans(areas, 12, 0, false)).toEqual([[2, 2]]); // sur la ligne 12 : la cellule libre
		expect(lineSpans(areas, 1, 0, false)).toEqual([[0, 3]]); // sur la bande : le tableau
	});
});
