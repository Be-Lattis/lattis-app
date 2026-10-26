import { describe, expect, it } from 'vitest';
import type { PivotConfig } from './Sheet.svelte';
import { derivedFields, derivedGroup, groupOf, setGroup, suggestStep } from './pivotgroup';
import { newSlicer, slicerOn, slicerPick, slicerSize, timelinePick, withSlicer } from './pivotslicer';
import { needsBaseField, needsBaseItem, shows } from './pivotfields';
import { setColFolded } from './pivottree';

const base: PivotConfig = { source: 'Ventes', rowFields: ['Région', 'Date'], colFields: ['Produit'], values: [{ field: 'Montant', agg: 'sum' }] };

describe('grouper un champ', () => {
	it('une date par années, trimestres, mois : champs ajoutés devant elle', () => {
		const c = setGroup(base, 'Date', { by: ['years', 'quarters', 'months'] }, ['Région', 'Date', 'Produit', 'Montant']);
		expect(c.rowFields).toHaveLength(4);
		expect(c.rowFields![3]).toBe('Date');
		const names = derivedFields(c).map((d) => d.name);
		expect(names).toHaveLength(2);
		expect(c.rowFields!.slice(1, 3)).toEqual(names);
		expect(groupOf(c, 'Date')?.by).toEqual(['years', 'quarters', 'months']);
		expect(derivedGroup(c, names[0])?.field).toBe('Date');
		// Regrouper autrement : les anciens niveaux quittent les zones.
		const d = setGroup(c, 'Date', { by: ['months'] });
		expect(d.rowFields).toEqual(['Région', 'Date']);
		expect(derivedFields(d)).toEqual([]);
		// Dissocier.
		const e = setGroup(c, 'Date', null);
		expect(e.groups).toBeUndefined();
		expect(e.rowFields).toEqual(['Région', 'Date']);
	});
	it('un nom pris par une colonne : le champ ajouté en prend un autre', () => {
		const c = setGroup(base, 'Date', { by: ['years', 'months'] }, ['Région', 'Date'].concat(derivedFields(setGroup(base, 'Date', { by: ['years', 'months'] })).map((d) => d.name)));
		const n = derivedFields(c)[0].name;
		expect(n).toContain('(Date)');
	});
	it('les valeurs gardées par le ▾ du champ sont oubliées', () => {
		const c = setGroup({ ...base, items: [{ field: 'Date', values: ['x'] }] }, 'Date', { by: ['years'] });
		expect(c.items).toBeUndefined();
	});
	it('intervalles proposés', () => {
		expect(suggestStep(3, 97)).toEqual({ start: 0, end: 97, step: 10 });
		expect(suggestStep(0, 1)).toMatchObject({ step: 0.1 });
	});
});

describe('segments', () => {
	const all = ['A', 'B', 'C'];
	it('clic : un seul élément ; Ctrl : ajouter ou retirer', () => {
		expect(slicerPick([], all, 'B', false)).toEqual(['B']);
		expect(slicerPick(['B'], all, 'B', false)).toEqual([]);
		expect(slicerPick([], all, 'B', true)).toEqual(['A', 'C']);
		expect(slicerPick(['A'], all, 'C', true)).toEqual(['A', 'C']);
		expect(slicerPick(['A', 'C'], all, 'B', true)).toEqual([]);
		expect(slicerPick(['A'], all, 'A', true)).toEqual([]);
	});
	it('chronologie : une période, ou une plage (Maj)', () => {
		const p = ['2024-01', '2024-02', '2024-03', '2024-04'];
		expect(timelinePick([], p, '2024-02', false)).toEqual(['2024-02']);
		expect(timelinePick(['2024-02'], p, '2024-04', true)).toEqual(['2024-02', '2024-03', '2024-04']);
		expect(timelinePick(['2024-03'], p, '2024-01', true)).toEqual(['2024-01', '2024-02', '2024-03']);
	});
	it('place, taille, ajout et retrait', () => {
		const s = newSlicer(base, 'Région', 1, 5);
		expect(s).toMatchObject({ field: 'Région', row: 1, col: 7 });
		expect(slicerSize(s)).toEqual({ w: 170, h: 200 });
		expect(slicerSize(newSlicer(base, 'Date', 1, 5, true)).w).toBe(360);
		const c = withSlicer(base, 0, s);
		expect(c.slicers).toHaveLength(1);
		expect(slicerOn(s, 'x')).toBe(true);
		expect(slicerOn({ ...s, values: ['y'] }, 'x')).toBe(false);
		expect(withSlicer(c, 0, null).slicers).toBeUndefined();
	});
});

describe('colonnes imbriquées', () => {
	it('plier et déplier un groupe de colonnes', () => {
		const c = setColFolded(base, ['Nord'], true);
		expect(c.colCollapsed).toEqual([['Nord']]);
		expect(c.collapsed).toBeUndefined();
		expect(setColFolded(c, ['Nord'], false).colCollapsed).toBeUndefined();
		expect(setColFolded({ ...base, colCollapseAll: true }, ['Sud'], false).colExpanded).toEqual([['Sud']]);
	});
});

describe('afficher les valeurs', () => {
	it('tous les calculs, et ceux qui demandent une base', () => {
		expect(shows()).toHaveLength(15);
		expect(needsBaseField('runTotal')).toBe(true);
		expect(needsBaseItem('runTotal')).toBe(false);
		expect(needsBaseItem('diff')).toBe(true);
		expect(needsBaseField('pctTotal')).toBe(false);
	});
});
