import { describe, expect, it } from 'vitest';
import type { Column } from '#bindings/internal/query/models.js';
import { OPS, clickColumns, defaultStep, insertStep, moveStep, newQuery, otherKey, otherSource, quickSteps, sourceCancel, typeName, withColumns } from './steps';

const cols = [
	{ name: 'Ville', type: 'text' },
	{ name: 'Montant', type: 'decimal' }
] as Column[];

describe('étapes de requête', () => {
	it('chaque opération a une étape par défaut', () => {
		for (const o of OPS) expect(defaultStep(o.op, cols).op).toBe(o.op);
	});
	it('la colonne choisie sert de défaut', () => {
		expect(defaultStep('filter', cols, 'Montant').conditions?.[0].column).toBe('Montant');
		expect(defaultStep('remove', cols, 'Ville').columns).toEqual(['Ville']);
		expect(defaultStep('select', cols).columns).toEqual(['Ville', 'Montant']);
		expect(defaultStep('pivot', cols, 'Ville').valueColumn).toBe('Montant');
	});
	it("insère après l'étape choisie et déplace", () => {
		const a = { op: 'top', count: 1 };
		const b = { op: 'top', count: 2 };
		const c = { op: 'top', count: 3 };
		expect(insertStep([a, b], -1, c)).toEqual([c, a, b]);
		expect(insertStep([a, b], 0, c)).toEqual([a, c, b]);
		expect(insertStep([a, b], 1, c)).toEqual([a, b, c]);
		expect(moveStep([a, b, c], 2, -1)).toEqual([a, c, b]);
		expect(moveStep([a, b, c], 0, -1)).toEqual([a, b, c]);
	});
	it('désigne une autre requête ou un tableau', () => {
		expect(otherKey(otherSource('query:Ventes 2026'))).toBe('query:Ventes 2026');
		expect(otherSource('table:Clients')).toEqual({ kind: 'table', table: 'Clients' });
		expect(otherKey(null)).toBe('');
	});
	it('nomme les types', () => {
		expect(typeName('decimal')).toBe('Décimal');
		expect(typeName('')).toBe('Auto');
	});
});

describe("menu des colonnes de l'aperçu", () => {
	const names = ['A', 'B', 'C', 'D'];
	it('clic, Ctrl et Maj choisissent les colonnes', () => {
		expect(clickColumns(names, [], '', 'B', {})).toEqual({ selected: ['B'], anchor: 'B' });
		expect(clickColumns(names, ['B'], 'B', 'B', {})).toEqual({ selected: [], anchor: '' });
		expect(clickColumns(names, ['B'], 'B', 'D', { ctrl: true })).toEqual({ selected: ['B', 'D'], anchor: 'D' });
		expect(clickColumns(names, ['B', 'D'], 'D', 'B', { ctrl: true })).toEqual({ selected: ['D'], anchor: 'B' });
		expect(clickColumns(names, ['D'], 'D', 'B', { shift: true })).toEqual({ selected: ['B', 'C', 'D'], anchor: 'D' });
		expect(clickColumns(names, ['A'], 'C', 'D', { shift: true, ctrl: true })).toEqual({ selected: ['A', 'C', 'D'], anchor: 'C' });
	});
	it("ajoute les étapes sans boîte, dans l'ordre des colonnes", () => {
		const three = [...cols, { name: 'Date', type: 'date' }] as Column[];
		expect(quickSteps('remove', three, ['Date', 'Ville'])).toEqual([{ op: 'remove', columns: ['Ville', 'Date'] }]);
		expect(quickSteps('keep', three, ['Montant'])).toEqual([{ op: 'select', columns: ['Montant'] }]);
		expect(quickSteps('unpivotOthers', three, ['Ville'])[0].columns).toEqual(['Montant', 'Date']);
		expect(quickSteps('sortDesc', three, ['Montant'])).toEqual([{ op: 'sort', sort: [{ column: 'Montant', desc: true }] }]);
		expect(quickSteps('type', three, ['Ville', 'Montant'], 'decimal')).toEqual([{ op: 'type', column: 'Ville', type: 'decimal' }]);
		expect(quickSteps('transform', three, ['Ville'], 'upper')).toEqual([{ op: 'transform', column: 'Ville', fn: 'upper' }]);
		expect(quickSteps('remove', three, [])).toEqual([]);
	});
	it('pré-remplit les boîtes avec les colonnes choisies', () => {
		expect(withColumns(defaultStep('group', cols), ['Ville', 'Montant']).columns).toEqual(['Ville', 'Montant']);
		expect(withColumns(defaultStep('rename', cols, 'Ville'), ['Ville', 'Montant']).column).toBe('Ville');
	});
});

describe('Annuler dans la boîte de la source', () => {
	const fresh = newQuery('Ventes', { kind: 'lattis', connection: 'Serveur', sql: '' });
	it('à la création, annule toute la création', () => {
		expect(sourceCancel(true, fresh, false)).toBe('close');
		expect(sourceCancel(true, fresh, true)).toBe('close');
	});
	it('source déjà validée ou requête enregistrée : ferme seulement la boîte', () => {
		expect(sourceCancel(false, fresh, true)).toBe('keep');
		expect(sourceCancel(false, fresh, false)).toBe('refresh');
		expect(sourceCancel(true, { ...fresh, id: 4 }, true)).toBe('keep');
		expect(sourceCancel(false, { ...fresh, id: 4 }, false)).toBe('refresh');
	});
});
