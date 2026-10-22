import { describe, expect, it } from 'vitest';
import type { Layout } from '#bindings/internal/canvas/models.js';
import { keepShown } from './layoutfirst.js';

const cfg = { source: 'Ventes', rowFields: ['Pays'], values: [{ field: 'Montant', agg: 'sum' }] };

function layout(over: Partial<Layout>): Layout {
	return { sheet: 1, sheets: [], tables: [], cells: [], widths: [], heights: [], names: [], styles: [], pivots: [], charts: [], buttons: [], ranges: [], ...over } as unknown as Layout;
}

describe('la vue d’abord : ce qui reste à calculer garde ce qui était affiché', () => {
	const shown = layout({
		tables: [{ id: 3, keyHint: true, autoNumber: 0 }] as never,
		pivots: [{ id: 7, row: 2, col: 4, config: cfg, cells: [['Pays', 'Somme'], ['FR', '10']], error: '' }] as never,
		charts: [{ id: 9, row: 1, col: 8, config: { source: 'Ventes', kind: 'bar' }, labels: ['FR'], series: [], error: '' }] as never
	});
	const quick = layout({
		pending: true,
		tables: [{ id: 3, keyHint: false, autoNumber: -1, pending: true }] as never,
		pivots: [{ id: 7, row: 2, col: 4, config: cfg, cells: null, error: '', pending: true }] as never,
		charts: [{ id: 9, row: 1, col: 8, config: { source: 'Ventes', kind: 'bar' }, labels: null, series: null, error: '', pending: true }] as never
	});

	it('anciennes valeurs du TCD, du graphique et de la suggestion de clé', () => {
		const l = keepShown(shown, quick);
		expect(l.pending).toBe(true);
		expect(l.tables?.[0]).toMatchObject({ keyHint: true, autoNumber: 0, pending: true });
		expect(l.pivots?.[0]).toMatchObject({ cells: [['Pays', 'Somme'], ['FR', '10']], pending: false });
		expect(l.charts?.[0]).toMatchObject({ labels: ['FR'], pending: false });
	});

	it('rien de gardé pour une autre feuille, une autre configuration, ou sans mise en page avant', () => {
		expect(keepShown(null, quick)).toBe(quick);
		expect(keepShown({ ...shown, sheet: 2 }, quick)).toBe(quick);
		const moved = layout({ ...quick, pivots: [{ ...quick.pivots![0], config: { ...cfg, rowFields: ['Ville'] } }] as never });
		expect(keepShown(shown, moved).pivots?.[0].pending).toBe(true);
	});

	it('mise en page déjà complète : rendue telle quelle', () => {
		const full = layout({ pivots: shown.pivots });
		expect(keepShown(shown, full)).toBe(full);
	});
});
