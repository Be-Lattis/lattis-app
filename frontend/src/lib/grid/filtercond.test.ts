import { describe, expect, it } from 'vitest';
import { condArgs, condOps, describeConds, filterKind, kindTitle, searchValues, valuesFilter } from './filtercond';

const vals = (on: boolean[]) => ['Lyon', 'Lille', 'Paris', ''].map((text, i) => ({ text, count: 1, on: on[i] }));

describe('filtre par valeurs', () => {
	it('tout coché sans recherche : pas de filtre', () => {
		expect(valuesFilter(vals([true, true, true, true]), '', false)).toEqual({ filter: null });
	});
	it('cases décochées : les valeurs cochées', () => {
		expect(valuesFilter(vals([true, false, true, false]), '', false).filter).toEqual({ values: ['Lyon', 'Paris'] });
	});
	it('recherche puis OK : seules les valeurs trouvées et cochées (point 5)', () => {
		expect(valuesFilter(vals([true, true, true, true]), 'ly', false).filter).toEqual({ values: ['Lyon'] });
		expect(valuesFilter(vals([true, false, true, true]), 'l', false).filter).toEqual({ values: ['Lyon'] });
	});
	it('rien de trouvé ou rien de coché : erreur', () => {
		expect(valuesFilter(vals([true, true, true, true]), 'zz', false).error).toBe('Aucune valeur ne correspond à la recherche');
		expect(valuesFilter(vals([false, false, false, false]), '', false).error).toBe('Cochez au moins une valeur');
	});
	it('erreurs seulement', () => {
		expect(valuesFilter(vals([true, true, true, true]), '', true).filter).toEqual({ errors: true });
	});
	it('recherche sans casse', () => {
		expect(searchValues(vals([true, true, true, true]), ' PAR ').map((v) => v.text)).toEqual(['Paris']);
	});
});

describe('filtres par condition', () => {
	it('famille selon le type', () => {
		expect(filterKind('integer')).toBe('number');
		expect(filterKind('decimal')).toBe('number');
		expect(filterKind('date')).toBe('date');
		expect(filterKind('text')).toBe('text');
		expect(kindTitle('number')).toBe('Filtres numériques');
	});
	it('conditions proposées', () => {
		expect(condOps('text')).toContain('contains');
		expect(condOps('number')).toEqual(expect.arrayContaining(['between', 'top', 'above']));
		expect(condOps('date')).not.toContain('above');
		expect(condArgs('between')).toBe(2);
		expect(condArgs('above')).toBe(0);
	});
	it('résumé', () => {
		expect(describeConds({ conds: [{ op: 'contains', value: 'ly' }] })).toBe('Contient « ly »');
		expect(describeConds({ conds: [{ op: 'lt', value: '10' }, { op: 'gt', value: '20' }], or: true }, 'number')).toBe(
			'Est inférieur à « 10 » ou Est supérieur à « 20 »'
		);
		expect(describeConds({ conds: [{ op: 'between', value: '01/01/2026', to: '31/01/2026' }] }, 'date')).toBe('Entre 01/01/2026 et 31/01/2026');
		expect(describeConds({ conds: [{ op: 'top', value: '5' }] }, 'number')).toBe('5 premiers');
		expect(describeConds({ conds: [{ op: 'lt', value: '01/01/2026' }] }, 'date')).toBe('Avant « 01/01/2026 »');
	});
});
