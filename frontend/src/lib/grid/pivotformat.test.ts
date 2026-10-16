import { describe, expect, it } from 'vitest';
import { pivotLayer } from './pivotformat';
import { resolveStyle, styledNumber } from './style';

describe('format des valeurs de TCD', () => {
	const p = {
		row: 1,
		col: 1,
		cells: [
			['Service', 'Moyenne de Salaire', 'Somme de Prime'],
			['RH', '3979,08045977011', '1200'],
			['Total', '4089,41176470588', '2400']
		],
		formats: ['', '{"dp":2}', '{"nf":"€"}']
	};
	it('deux décimales pour une moyenne, format source sinon', () => {
		expect(pivotLayer([p], 2, 2)).toEqual({ dp: 2 });
		expect(styledNumber(p.cells[1][1], resolveStyle(pivotLayer([p], 2, 2)))).toBe('3 979,08');
		expect(pivotLayer([p], 3, 3)).toEqual({ nf: '€' });
		expect(pivotLayer([p], 2, 1)).toBeUndefined(); // libellé
		expect(pivotLayer([p], 1, 2)).toBeUndefined(); // en-tête
		expect(pivotLayer([p], 4, 2)).toBeUndefined(); // sous le TCD
	});
	it('la décimale ajoutée sur la cellule l’emporte', () => {
		expect(styledNumber(p.cells[1][1], resolveStyle(pivotLayer([p], 2, 2), { dp: 3 }))).toBe('3 979,080');
	});
});
