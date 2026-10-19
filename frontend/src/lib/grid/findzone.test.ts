import { describe, expect, it } from 'vitest';
import { findKey, findZone, foundCell, inZone, nextHit } from './findzone';

describe('foundCell', () => {
	const at = { row: 3, col: 2 };
	it('sur la feuille : tant que la cellule active est dessus', () => {
		expect(foundCell(at, null, { row: 3, col: 2 })).toEqual(at);
		expect(foundCell(at, null, { row: 4, col: 2 })).toBeNull();
	});
	it('dans la sélection : tant que la cellule y est', () => {
		expect(foundCell(at, { r0: 0, c0: 0, r1: 5, c1: 5 }, { row: 0, col: 0 })).toEqual(at);
		expect(foundCell(at, { r0: 4, c0: 0, r1: 5, c1: 5 }, { row: 4, col: 0 })).toBeNull();
	});
	it('rien sans cellule trouvée ni pendant une saisie', () => {
		expect(foundCell(null, null, at)).toBeNull();
		expect(foundCell(at, null, at, true)).toBeNull();
	});
});

describe('findZone', () => {
	it('une seule cellule : toute la feuille', () => {
		expect(findZone({ r0: 3, c0: 2, r1: 3, c1: 2 }, true)).toBeNull();
	});
	it('plusieurs cellules : la sélection', () => {
		expect(findZone({ r0: 1, c0: 0, r1: 9, c1: 2 }, false)).toEqual({ r0: 1, c0: 0, r1: 9, c1: 2 });
	});
});

describe('inZone', () => {
	const z = { r0: 1, c0: 1, r1: 3, c1: 2 };
	it('bornes comprises', () => {
		expect(inZone({ row: 1, col: 1 }, z)).toBe(true);
		expect(inZone({ row: 3, col: 2 }, z)).toBe(true);
		expect(inZone({ row: 4, col: 2 }, z)).toBe(false);
		expect(inZone({ row: 2, col: 0 }, z)).toBe(false);
	});
	it('sans zone : partout', () => {
		expect(inZone({ row: 999, col: 999 }, null)).toBe(true);
	});
});

describe('findKey', () => {
	it('distingue la zone', () => {
		expect(findKey('a', null)).not.toBe(findKey('a', { r0: 0, c0: 0, r1: 1, c1: 1 }));
		expect(findKey('a', { r0: 0, c0: 0, r1: 1, c1: 1 })).not.toBe(findKey('a', { r0: 0, c0: 0, r1: 2, c1: 1 }));
	});
	it('distingue la feuille (occurrences d’une autre feuille oubliées)', () => {
		expect(findKey('a', null, 1)).not.toBe(findKey('a', null, 2));
		expect(findKey('a', null, 1)).toBe(findKey('a', null, 1));
	});
});

describe('nextHit', () => {
	const hits = [
		{ row: 1, col: 1 },
		{ row: 1, col: 3 },
		{ row: 4, col: 0 }
	];
	it('suivant, en repartant du début', () => {
		expect(nextHit(hits, { row: 1, col: 1 }, false)).toBe(1);
		expect(nextHit(hits, { row: 2, col: 0 }, false)).toBe(2);
		expect(nextHit(hits, { row: 4, col: 0 }, false)).toBe(0);
	});
	it('précédent, en repartant de la fin', () => {
		expect(nextHit(hits, { row: 4, col: 0 }, true)).toBe(1);
		expect(nextHit(hits, { row: 1, col: 1 }, true)).toBe(2);
	});
	it('sans départ : la première ou la dernière', () => {
		expect(nextHit(hits, null, false)).toBe(0);
		expect(nextHit(hits, null, true)).toBe(2);
		expect(nextHit([], null, false)).toBe(-1);
	});
});
