import { describe, expect, it } from 'vitest';
import { brokenPivotAt, pivotsCleared, pivotZone } from './pivotzone';
import { changeSource } from './pivotfields';
import type { PivotConfig } from './Sheet.svelte';

// TCD sain : filtres de rapport en lignes 1-2 (anchor 1), tableau en 3-5, colonnes 1-2.
const ok = { id: 1, row: 3, anchor: 1, col: 1, cells: [['Pays', 'Somme'], ['FR', '17'], ['Total', '17']], error: '' };
// TCD en erreur : la seule cellule de son message.
const broken = { id: 2, row: 8, anchor: 8, col: 0, cells: null, error: 'tableau « Requête1 » introuvable' };

describe('zone d’un tableau croisé', () => {
	it('filtres compris, ou la cellule du message en erreur', () => {
		expect(pivotZone(ok)).toEqual({ r0: 1, c0: 1, r1: 5, c1: 2 });
		expect(pivotZone(broken)).toEqual({ r0: 8, c0: 0, r1: 8, c1: 0 });
		expect(brokenPivotAt([ok, broken], 8, 0)).toBe(broken);
		expect(brokenPivotAt([ok, broken], 3, 1)).toBeUndefined();
	});
});

describe('Suppr sur un tableau croisé', () => {
	it('pris en entier : supprimé', () => {
		expect(pivotsCleared([ok, broken], { r0: 0, c0: 0, r1: 6, c1: 4 })).toEqual({ gone: [ok], partial: undefined });
	});
	it('en partie : refusé', () => {
		expect(pivotsCleared([ok], { r0: 3, c0: 1, r1: 3, c1: 1 }).partial).toBe(ok);
		// Sans ses filtres de rapport : encore une partie.
		expect(pivotsCleared([ok], { r0: 3, c0: 1, r1: 5, c1: 2 }).partial).toBe(ok);
	});
	it('en erreur : sa cellule suffit', () => {
		expect(pivotsCleared([ok, broken], { r0: 8, c0: 0, r1: 8, c1: 0 })).toEqual({ gone: [broken], partial: undefined });
	});
	it('hors de la zone : rien', () => {
		expect(pivotsCleared([ok, broken], { r0: 10, c0: 0, r1: 12, c1: 3 })).toEqual({ gone: [], partial: undefined });
	});
});

describe('changer la source', () => {
	const cfg: PivotConfig = {
		name: 'TCD1',
		source: 'Requête1',
		rowFields: ['Pays', 'Ville'],
		cols: 'Année',
		values: [{ field: 'Ventes', agg: 'sum' }, { agg: 'count' }],
		filters: [{ field: 'Canal', values: ['Web'] }],
		items: [{ field: 'Ville', values: ['Lyon'] }],
		sort: { Pays: 'desc', Ville: 'asc' },
		collapsed: [['FR']]
	};
	it('garde les champs présents, sous leur nom exact', () => {
		const c = changeSource(cfg, 'Ventes2', [
			{ name: 'pays', type: 'text' },
			{ name: 'Ventes', type: 'decimal' },
			{ name: 'Canal', type: 'text' }
		]);
		expect(c.source).toBe('Ventes2');
		expect(c.name).toBe('TCD1');
		expect(c.rowFields).toEqual(['pays']);
		expect(c.cols).toBeUndefined();
		expect(c.values).toEqual([{ field: 'Ventes', agg: 'sum' }, { agg: 'count' }]);
		expect(c.filters).toEqual([{ field: 'Canal', values: ['Web'] }]);
		expect(c.items).toBeUndefined();
		expect(c.sort).toEqual({ pays: 'desc' });
		expect(c.collapsed).toBeUndefined();
		expect(cfg.source).toBe('Requête1');
	});
	it('rien en commun : compte les lignes', () => {
		const c = changeSource({ source: 'A', rowFields: ['X'] }, 'B', [{ name: 'Calc', type: 'formula' }]);
		expect(c.rowFields).toBeUndefined();
		expect(c.values).toEqual([{ agg: 'count' }]);
	});
});
