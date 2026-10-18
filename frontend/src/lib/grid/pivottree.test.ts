import { describe, expect, it } from 'vitest';
import { canFoldAll, foldAll, foldTarget, isFolded, pivotBoxHit, pivotBoxLeft, pivotLabelOffset, setFolded, PIVOT_INDENT } from './pivottree';

describe('TCD en arborescence', () => {
	it('indente chaque niveau, sans retrait pour le total général', () => {
		expect(pivotLabelOffset(undefined)).toBe(0);
		expect(pivotLabelOffset({ depth: 0, total: true })).toBe(0);
		const g = pivotLabelOffset({ depth: 0, group: true });
		expect(g).toBeGreaterThan(0);
		expect(pivotLabelOffset({ depth: 1 })).toBe(g + PIVOT_INDENT);
		// Le sous-total en bas s'aligne sur son groupe.
		expect(pivotLabelOffset({ depth: 0, subtotal: true })).toBe(g);
	});

	it('repère la case −/+ des seuls groupes', () => {
		const l = pivotBoxLeft(1);
		expect(pivotBoxHit({ depth: 1, group: true }, l + 2)).toBe(true);
		expect(pivotBoxHit({ depth: 1, group: true }, l + 40)).toBe(false);
		expect(pivotBoxHit({ depth: 1 }, l + 2)).toBe(false);
		expect(pivotBoxHit({ depth: 0, group: true }, l + 2)).toBe(false);
	});

	it('plie et déplie un groupe, sans toucher aux autres', () => {
		let cfg: { source: string; collapseAll?: boolean; collapsed?: string[][]; expanded?: string[][] } = { source: 'T' };
		cfg = setFolded(cfg, ['RH'], true);
		expect(cfg.collapsed).toEqual([['RH']]);
		expect(isFolded(cfg, ['RH'])).toBe(true);
		expect(isFolded(cfg, ['IT'])).toBe(false);
		cfg = setFolded(cfg, ['IT', 'Dev'], true);
		cfg = setFolded(cfg, ['RH'], false);
		expect(cfg.collapsed).toEqual([['IT', 'Dev']]);
		cfg = setFolded(cfg, ['IT', 'Dev'], false);
		expect(cfg).toEqual({ source: 'T' });
	});

	it('réduit tout, puis déplie des exceptions ; développe tout', () => {
		let cfg: { source: string; collapseAll?: boolean; collapsed?: string[][]; expanded?: string[][] } = { source: 'T', collapsed: [['RH']] };
		cfg = foldAll(cfg, true);
		expect(cfg).toEqual({ source: 'T', collapseAll: true });
		expect(isFolded(cfg, ['IT'])).toBe(true);
		cfg = setFolded(cfg, ['IT'], false);
		expect(cfg.expanded).toEqual([['IT']]);
		expect(isFolded(cfg, ['IT'])).toBe(false);
		expect(isFolded(cfg, ['RH'])).toBe(true);
		cfg = setFolded(cfg, ['IT'], true);
		expect(cfg).toEqual({ source: 'T', collapseAll: true });
		expect(foldAll(cfg, false)).toEqual({ source: 'T' });
	});

	it('choisit le groupe visé par Développer et Réduire', () => {
		expect(foldTarget({ depth: 0, group: true, collapsed: true, path: ['RH'] }, true)).toEqual(['RH']);
		expect(foldTarget({ depth: 0, group: true, path: ['RH'] }, true)).toBeNull();
		expect(foldTarget({ depth: 0, group: true, path: ['RH'] }, false)).toEqual(['RH']);
		// Ligne du dernier champ : Réduire plie son groupe.
		expect(foldTarget({ depth: 1, path: ['RH', 'Chef'] }, false)).toEqual(['RH']);
		expect(foldTarget({ depth: 1, path: ['RH', 'Chef'] }, true)).toBeNull();
		expect(foldTarget({ depth: 0, subtotal: true, path: ['RH'] }, false)).toEqual(['RH']);
		expect(foldTarget({ depth: 0, group: true, collapsed: true, path: ['RH'] }, false)).toBeNull();
		expect(foldTarget({ depth: 0, total: true }, false)).toBeNull();
	});

	it('sait s’il reste à plier ou à déplier', () => {
		const rows = [{ depth: 0, group: true, collapsed: true, path: ['RH'] }, { depth: 0, group: true, path: ['IT'] }, { depth: 1, path: ['IT', 'Dev'] }];
		expect(canFoldAll(rows, true)).toBe(true);
		expect(canFoldAll(rows, false)).toBe(true);
		expect(canFoldAll(rows.slice(0, 1), false)).toBe(false);
		expect(canFoldAll(null, true)).toBe(false);
	});
});
