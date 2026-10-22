import { describe, expect, it, vi } from 'vitest';
import { sweepAt, watchSource, WORK_DELAY, WORK_FADE, workAlpha, workZones, type WorkLookup } from './working';

// Tableau Ventes : en-tête ligne 2, colonnes 1 à 4, données jusqu'à la ligne 1 000 001.
const ventes = { id: 7, name: 'Ventes' };
const look: WorkLookup = {
	table: (t) => (t === ventes || t === 'Ventes' ? { row: 2, col: 1, last: 1_000_001, lastCol: 4 } : undefined)
};

describe('workZones', () => {
	it('colonne d’un tableau : de l’en-tête à la dernière ligne', () => {
		expect(workZones('deleteColumn', [ventes, 2], look)).toEqual([{ r0: 2, c0: 3, r1: 1_000_001, c1: 3 }]);
		expect(workZones('setType', [ventes, 0], look)).toEqual([{ r0: 2, c0: 1, r1: 1_000_001, c1: 1 }]);
		expect(workZones('suppressColumns', [ventes, 1, 2], look)).toEqual([{ r0: 2, c0: 2, r1: 1_000_001, c1: 3 }]);
		expect(workZones('deleteColumnSpans', [[{ table: 'Ventes', from: 3, to: 3 }]], look)).toEqual([{ r0: 2, c0: 4, r1: 1_000_001, c1: 4 }]);
	});

	it('tri : tout le tableau ; ligne : la ligne de données', () => {
		expect(workZones('sortTable', [ventes, 1, true], look)).toEqual([{ r0: 2, c0: 1, r1: 1_000_001, c1: 4 }]);
		expect(workZones('deleteRow', [ventes, 0], look)).toEqual([{ r0: 3, c0: 1, r1: 3, c1: 4 }]);
	});

	it('collage, effacement, mise en forme : le bloc ou le rectangle', () => {
		expect(workZones('paste', [10, 2, [['a', 'b', 'c'], ['d']]], look)).toEqual([{ r0: 10, c0: 2, r1: 11, c1: 4 }]);
		expect(workZones('clearRange', [5, 3, 1, 0, false], look)).toEqual([{ r0: 1, c0: 0, r1: 5, c1: 3 }]);
		expect(workZones('applyStyle', [1, 1, 1, 1, { bold: true }], look)).toEqual([{ r0: 1, c0: 1, r1: 1, c1: 1 }]);
	});

	it('lignes et colonnes entières de la feuille', () => {
		expect(workZones('sheetLines', ['insertRows', 4, 2], look)?.[0]).toMatchObject({ r0: 4, r1: 5, c0: 0 });
		expect(workZones('sheetLines', ['deleteCols', 3, 1], look)?.[0]).toMatchObject({ c0: 1, c1: 3, r0: 0 });
	});

	it('sans zone : méthode inconnue, tableau absent, arguments incomplets', () => {
		expect(workZones('undo', [], look)).toBeNull();
		expect(workZones('deleteColumn', [{ id: 99 }, 0], look)).toBeNull();
		expect(workZones('paste', [1, 1, []], look)).toBeNull();
	});
});

describe('watchSource', () => {
	it('marque la zone le temps de l’action, même quand elle échoue', async () => {
		let fail = false;
		const src = {
			deleteColumn: vi.fn(async (..._args: unknown[]) => {
				if (fail) throw new Error('non');
				return 'ok';
			}),
			window: vi.fn(async () => 'rows')
		};
		const started: number[] = [];
		const ended: number[] = [];
		let n = 0;
		const w = watchSource(src, look, () => (started.push(++n), n), (id) => ended.push(id));
		await expect(w.deleteColumn(ventes, 1)).resolves.toBe('ok');
		expect(src.deleteColumn).toHaveBeenCalledWith(ventes, 1);
		expect([started, ended]).toEqual([[1], [1]]);
		fail = true;
		await expect(w.deleteColumn(ventes, 1)).rejects.toThrow('non');
		expect(ended).toEqual([1, 2]);
		// Lecture : passée telle quelle, sans zone.
		expect(w.window).toBe(src.window);
		await w.window();
		expect(started).toHaveLength(2);
		// Membre absent : reste absent (la feuille teste `source.paste`).
		expect((w as { paste?: unknown }).paste).toBeUndefined();
	});
});

describe('reflet', () => {
	it('paraît après le délai, puis passe en boucle', () => {
		expect(workAlpha(100, 0)).toBe(0);
		expect(workAlpha(WORK_DELAY + WORK_FADE, 0)).toBe(1);
		expect(sweepAt(0, 0)).toBe(0);
		expect(sweepAt(2100, 0)).toBeCloseTo(0.5);
	});
});
