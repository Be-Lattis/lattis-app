import { describe, expect, it } from 'vitest';
import { expandRange, extendCorner, intersect, isSingleCell, mergeRect, rangeOf, stepCell, wholeLines } from './mergesel';
import type { MergeRange } from './sheetfmt';
import { bandMerges } from './twolevel';

// B2:C3 (lignes 1-2, colonnes 1-2) et D3:D5 (lignes 2-4, colonne 3).
const merges: MergeRange[] = [
	{ row: 1, col: 1, rows: 2, cols: 2 },
	{ row: 2, col: 3, rows: 3, cols: 1 }
];
const MAX = 1000;

describe('plage agrandie aux fusions', () => {
	it('englobe une fusion touchée, sans plus', () => {
		expect(expandRange({ r0: 0, r1: 1, c0: 0, c1: 1 }, merges)).toEqual({ r0: 0, r1: 2, c0: 0, c1: 2 });
	});
	it('enchaîne les fusions qui se touchent', () => {
		// C2:D2 touche B2:C3, qui mène à la ligne 3, où D3:D5 commence.
		expect(expandRange({ r0: 1, r1: 1, c0: 2, c1: 3 }, merges)).toEqual({ r0: 1, r1: 4, c0: 1, c1: 3 });
	});
	it('ne change pas une plage sans fusion', () => {
		const r = { r0: 6, r1: 8, c0: 0, c1: 5 };
		expect(expandRange(r, merges)).toEqual(r);
		expect(expandRange(r, [])).toBe(r);
	});
	it('reconnaît la fusion seule comme une cellule', () => {
		expect(isSingleCell({ r0: 1, r1: 2, c0: 1, c1: 2 }, { row: 1, col: 1 }, merges)).toBe(true);
		expect(isSingleCell({ r0: 5, r1: 5, c0: 5, c1: 5 }, { row: 5, col: 5 }, merges)).toBe(true);
		expect(isSingleCell({ r0: 1, r1: 3, c0: 1, c1: 2 }, { row: 1, col: 1 }, merges)).toBe(false);
	});
});

describe('flèches autour des fusions', () => {
	it('entre dans une fusion par sa cellule en haut à gauche, de tous côtés', () => {
		expect(stepCell(0, 2, 1, 0, merges, MAX, MAX)).toEqual({ row: 1, col: 1 }); // par le haut
		expect(stepCell(3, 2, -1, 0, merges, MAX, MAX)).toEqual({ row: 1, col: 1 }); // par le bas
		expect(stepCell(2, 0, 0, 1, merges, MAX, MAX)).toEqual({ row: 1, col: 1 }); // par la gauche
		expect(stepCell(4, 4, 0, -1, merges, MAX, MAX)).toEqual({ row: 2, col: 3 }); // par la droite
	});
	it('sort par le bord dans le sens du déplacement', () => {
		expect(stepCell(1, 1, 0, 1, merges, MAX, MAX)).toEqual({ row: 1, col: 3 });
		expect(stepCell(1, 1, 1, 0, merges, MAX, MAX)).toEqual({ row: 3, col: 1 });
		expect(stepCell(1, 1, 0, -1, merges, MAX, MAX)).toEqual({ row: 1, col: 0 });
		expect(stepCell(1, 1, -1, 0, merges, MAX, MAX)).toEqual({ row: 0, col: 1 });
		// D3:D5 vers le bas : D6.
		expect(stepCell(2, 3, 1, 0, merges, MAX, MAX)).toEqual({ row: 5, col: 3 });
	});
	it('passe d’une fusion à la voisine', () => {
		expect(stepCell(2, 1, 0, 1, merges, MAX, MAX)).toEqual({ row: 2, col: 3 });
	});
	it('reste dans la fusion au bord de la feuille', () => {
		const top: MergeRange[] = [{ row: 0, col: 0, rows: 2, cols: 2 }];
		expect(stepCell(0, 0, -1, 0, top, MAX, MAX)).toEqual({ row: 0, col: 0 });
		expect(stepCell(0, 0, 0, -1, top, MAX, MAX)).toEqual({ row: 0, col: 0 });
	});
	it('sans fusion : la cellule voisine', () => {
		expect(stepCell(7, 7, 0, 1, merges, MAX, MAX)).toEqual({ row: 7, col: 8 });
	});
});

describe('Maj + flèche autour des fusions', () => {
	const sel = (a: { row: number; col: number }, k: { row: number; col: number }) => expandRange(rangeOf(a, k), merges);
	it('agrandit depuis une fusion par son bord', () => {
		const a = { row: 1, col: 1 };
		const k = extendCorner(a, a, 0, 1, merges, MAX, MAX);
		expect(k).toEqual({ row: 1, col: 3 });
		// D2 touche D3:D5 : la plage descend jusqu'à la ligne 5.
		expect(sel(a, k)).toEqual({ r0: 1, r1: 4, c0: 1, c1: 3 });
	});
	it('englobe la fusion atteinte puis continue au-delà', () => {
		const a = { row: 1, col: 0 };
		let k = extendCorner(a, a, 0, 1, merges, MAX, MAX);
		expect(sel(a, k)).toEqual({ r0: 1, r1: 2, c0: 0, c1: 2 });
		// Une cellule de plus à droite de la fusion, pas une colonne dans la fusion.
		k = extendCorner(a, k, 0, 1, merges, MAX, MAX);
		expect(sel(a, k).c1).toBe(3);
	});
	it('recule par-dessus la fusion jusqu’à ce que la plage change', () => {
		const a = { row: 1, col: 1 };
		const k = extendCorner(a, a, 0, -1, merges, MAX, MAX);
		expect(sel(a, k)).toEqual({ r0: 1, r1: 2, c0: 0, c1: 2 });
		// Retour vers la droite : la plage redevient la fusion seule.
		const back = extendCorner(a, k, 0, 1, merges, MAX, MAX);
		expect(sel(a, back)).toEqual({ r0: 1, r1: 2, c0: 1, c1: 2 });
	});
	it('vers le haut depuis une fusion : la ligne au-dessus', () => {
		const a = { row: 2, col: 3 };
		const k = extendCorner(a, a, -1, 0, merges, MAX, MAX);
		expect(k.row).toBe(1);
		expect(sel(a, k)).toEqual({ r0: 1, r1: 4, c0: 3, c1: 3 });
	});
	it('ne bouge pas au bord de la feuille', () => {
		const a = { row: 0, col: 0 };
		expect(extendCorner(a, a, -1, 0, merges, MAX, MAX)).toEqual(a);
	});
});

describe('lignes et colonnes entières (gouttière, bandeau) et fusions', () => {
	// Tableau à deux lignes en A2 (bande en ligne 0, en-tête en ligne 1) :
	// ARBRE sur un niveau (A1:A2), groupe Nid sur B1:C1, DECOUVREUR (D1:D2).
	const band = bandMerges([{ row: 1, col: 0, twoLevel: true, columns: [{ name: 'ARBRE' }, { name: 'Nid.Haut' }, { name: 'Nid.Diam' }, { name: 'DECOUVREUR' }] }]);
	// Et une fusion verticale libre, A10:A13 (lignes 9 à 12).
	const all: MergeRange[] = [...band, { row: 9, col: 0, rows: 4, cols: 1 }];
	const LAST = 50;

	it("la ligne d'en-tête seule, sans la bande (le clic n'était pas agrandi à la fusion comme une plage)", () => {
		// Avant : la plage de la ligne 1 agrandie aux fusions prenait la bande.
		expect(expandRange({ r0: 1, r1: 1, c0: 0, c1: LAST }, all)).toEqual({ r0: 0, r1: 1, c0: 0, c1: LAST });
		const w = wholeLines('rows', 1, 1, LAST, all);
		expect(w.range).toEqual({ r0: 1, r1: 1, c0: 0, c1: LAST });
		// La cellule active : la fusion ARBRE, par son haut.
		expect(w.active).toEqual({ row: 0, col: 0 });
	});
	it('une ligne au milieu d’une fusion verticale : cette ligne seule', () => {
		expect(wholeLines('rows', 10, 10, LAST, all)).toEqual({ active: { row: 9, col: 0 }, range: { r0: 10, r1: 10, c0: 0, c1: LAST } });
	});
	it('glisser vers le haut : de la ligne de départ à celle atteinte', () => {
		expect(wholeLines('rows', 11, 8, LAST, all).range).toEqual({ r0: 8, r1: 11, c0: 0, c1: LAST });
	});
	it('une colonne d’un groupe : cette colonne seule, pas tout le groupe', () => {
		expect(expandRange({ r0: 0, r1: LAST, c0: 2, c1: 2 }, all).c0).toBe(1);
		const w = wholeLines('cols', 2, 2, LAST, all);
		expect(w.range).toEqual({ r0: 0, r1: LAST, c0: 2, c1: 2 });
		expect(w.active).toEqual({ row: 0, col: 1 });
	});
	it('surlignage : la partie sélectionnée d’une fusion', () => {
		expect(intersect(mergeRect(all[all.length - 1]), { r0: 10, r1: 11, c0: 0, c1: LAST })).toEqual({ r0: 10, r1: 11, c0: 0, c1: 0 });
		expect(intersect({ r0: 0, r1: 1, c0: 0, c1: 0 }, { r0: 5, r1: 6, c0: 0, c1: LAST })).toBeNull();
	});
});
