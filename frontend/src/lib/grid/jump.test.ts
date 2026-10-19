import { describe, expect, it } from 'vitest';
import { jumpLine, runs } from './jump';

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
});
