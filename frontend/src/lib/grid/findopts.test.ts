import { describe, expect, it } from 'vitest';
import { defaultFindOpts, listLabel, nextFound, optsKey, replaceLook, replaceSummary, searchAll, searchQuery, type FoundCell, type SearchQuery, type SearchResult } from './findopts';

const cell = (k: number, row: number, col: number): FoundCell => ({ sheet: k + 1, k, name: `F${k + 1}`, row, col, text: '' });

describe('searchQuery', () => {
	it('feuille : la sélection borne la recherche', () => {
		const q = searchQuery('a', 3, { r0: 1, c0: 0, r1: 4, c1: 2 }, defaultFindOpts());
		expect(q).toEqual({ text: 'a', sheet: 3, book: false, byCols: false, look: 'formulas', case: false, whole: false, in: { r0: 1, c0: 0, r1: 4, c1: 2 } });
	});
	it('classeur : depuis la première feuille, sans la sélection', () => {
		const q = searchQuery('a', 3, { r0: 1, c0: 0, r1: 4, c1: 2 }, { ...defaultFindOpts(), book: true, matchCase: true, whole: true, look: 'notes' });
		expect(q.sheet).toBe(0);
		expect(q.in).toBeUndefined();
		expect([q.book, q.case, q.whole, q.look]).toEqual([true, true, true, 'notes']);
	});
	it('les options changent la clé', () => {
		expect(optsKey(defaultFindOpts())).not.toBe(optsKey({ ...defaultFindOpts(), whole: true }));
	});
	it('remplacer dans les valeurs : les formules', () => {
		expect(replaceLook({ ...defaultFindOpts(), look: 'values' }).look).toBe('formulas');
		expect(replaceLook({ ...defaultFindOpts(), look: 'notes' }).look).toBe('notes');
	});
});

describe('searchAll', () => {
	const sheets: Record<number, SearchResult> = {
		1: { sheet: 1, name: 'A', found: [{ sheet: 1, row: 1, col: 0, text: 'x' }], total: 1, next: 2 },
		2: { sheet: 2, name: 'B', found: null, total: 0, next: 5 },
		5: { sheet: 5, name: 'C', found: [{ sheet: 5, row: 0, col: 0, text: 'y' }, { sheet: 5, row: 3, col: 1, text: 'z' }], total: 9, next: 0 }
	};
	const run = async (q: SearchQuery) => sheets[q.sheet || 1];
	it('classeur : feuille après feuille, dans la limite, total de tout', async () => {
		const parts: number[] = [];
		const r = await searchAll(run, searchQuery('x', 1, null, { ...defaultFindOpts(), book: true }), 2, (h) => parts.push(h.length));
		expect(r?.order).toEqual([1, 2, 5]);
		expect(r?.hits.map((h) => [h.name, h.k, h.row])).toEqual([
			['A', 0, 1],
			['C', 2, 0]
		]);
		expect(r?.total).toBe(10);
		expect(parts).toEqual([1, 1, 2]);
	});
	it('feuille : un seul appel', async () => {
		const r = await searchAll(run, searchQuery('x', 1, null, defaultFindOpts()), 10);
		expect(r?.order).toEqual([1]);
	});
	it('recherche dépassée : rien', async () => {
		expect(await searchAll(run, searchQuery('x', 1, null, defaultFindOpts()), 10, undefined, () => true)).toBeNull();
	});
});

describe('nextFound', () => {
	const hits = [cell(0, 1, 2), cell(0, 3, 0), cell(1, 0, 5), cell(1, 2, 1)];
	it('suivante dans la feuille, puis sur la feuille suivante, puis au début', () => {
		expect(nextFound(hits, { k: 0, row: 1, col: 2 }, false, false)).toBe(1);
		expect(nextFound(hits, { k: 0, row: 3, col: 0 }, false, false)).toBe(2);
		expect(nextFound(hits, { k: 1, row: 9, col: 9 }, false, false)).toBe(0);
	});
	it('précédente, en repartant de la fin', () => {
		expect(nextFound(hits, { k: 1, row: 0, col: 5 }, true, false)).toBe(1);
		expect(nextFound(hits, { k: 0, row: 0, col: 0 }, true, false)).toBe(3);
	});
	it('par colonnes', () => {
		const cols = [cell(0, 5, 0), cell(0, 1, 2)];
		expect(nextFound(cols, { k: 0, row: 9, col: 0 }, false, true)).toBe(1);
		expect(nextFound(cols, { k: 0, row: 9, col: 0 }, false, false)).toBe(0);
	});
	it('sans position ni occurrence', () => {
		expect(nextFound(hits, null, false, false)).toBe(0);
		expect(nextFound(hits, null, true, false)).toBe(3);
		expect(nextFound([], null, false, false)).toBe(-1);
	});
});

describe('bilans', () => {
	it('liste coupée', () => {
		expect(listLabel(3, 3)).toBe('3 cellules trouvées');
		expect(listLabel(1000, 1234567)).toMatch(/^1\s?234\s?567 cellules trouvées · 1\s?000 affichées$/);
	});
	it('remplacer tout : laissées et pourquoi', () => {
		expect(replaceSummary({ replaced: 2, skipped: 0 }, 'book')).toBe('2 cellules remplacées dans le classeur (Ctrl+Z pour annuler)');
		expect(replaceSummary({ replaced: 1, skipped: 3, reason: 'feuille protégée' }, 'sheet')).toBe('1 cellule remplacée (Ctrl+Z pour annuler) · 3 cellules laissées : feuille protégée');
		expect(replaceSummary({ replaced: 0, skipped: 1, reason: 'lecture seule' }, 'selection')).toBe('1 cellule laissée : lecture seule');
		expect(replaceSummary({ replaced: 0, skipped: 0 }, 'selection')).toBe('Aucune cellule trouvée dans la sélection');
	});
});
