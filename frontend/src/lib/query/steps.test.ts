import { describe, expect, it } from 'vitest';
import type { Column } from '#bindings/internal/query/models.js';
import { OPS, defaultStep, insertStep, moveStep, otherKey, otherSource, typeName } from './steps';

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
	it('insère après l’étape choisie et déplace', () => {
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
