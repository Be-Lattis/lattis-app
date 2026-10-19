import { describe, expect, it } from 'vitest';
import { blocksReach, boxesReach } from './objextent';

// Grille régulière : lignes de 20 px, colonnes de 100 px.
const rowAt = (y: number) => Math.floor(y / 20);
const colAt = (x: number) => Math.floor(x / 100);

describe('boxesReach', () => {
	it('sans objet', () => {
		expect(boxesReach([], rowAt, colAt)).toEqual({ row: -1, col: -1 });
	});
	it('le bord bas et droit du plus loin', () => {
		const r = boxesReach(
			[
				{ x: 0, y: 0, w: 480, h: 290 },
				{ x: 900, y: 10, w: 120, h: 32 }
			],
			rowAt,
			colAt
		);
		expect(r).toEqual({ row: 14, col: 10 });
	});
	it('un objet plus bas qu’un autre plus à droite', () => {
		const r = boxesReach(
			[
				{ x: 50, y: 2000, w: 100, h: 100 },
				{ x: 3000, y: 0, w: 10, h: 10 }
			],
			rowAt,
			colAt
		);
		expect(r.row).toBe(105);
		expect(r.col).toBe(30);
	});
});

describe('blocksReach', () => {
	it('dernière cellule des blocs', () => {
		expect(
			blocksReach([
				{ row: 2, col: 3, rows: 5, cols: 2 },
				{ row: 0, col: 10, rows: 1, cols: 1 }
			])
		).toEqual({ row: 6, col: 10 });
	});
	it('bloc vide ignoré', () => {
		expect(blocksReach([{ row: 50, col: 50, rows: 0, cols: 3 }])).toEqual({ row: -1, col: -1 });
	});
});
