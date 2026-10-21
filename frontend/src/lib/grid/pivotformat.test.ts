import { describe, expect, it } from 'vitest';
import { pivotColLayer, pivotLayer, pivotValueText } from './pivotformat';

describe('valeurs entières du TCD', () => {
	it('séparateur de milliers sur un nombre ou un total sans format', () => {
		expect(pivotValueText('1200', 9, 2)).toBe('1 200');
		expect(pivotValueText('4 020,07', 9, 1)).toBe('4 020,07');
		expect(pivotValueText('85', 3, 2)).toBe('85');
		// Étiquettes (colonne 0) et en-têtes (ligne 0) : tels quels (années…).
		expect(pivotValueText('2024', 3, 0)).toBe('2024');
		expect(pivotValueText('2024', 0, 1)).toBe('2024');
		// Déjà formaté : inchangé.
		expect(pivotValueText('4 020,07', 9, 1)).toBe('4 020,07');
	});
});
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

describe('mise en forme des colonnes de TCD', () => {
	const p = {
		row: 1,
		col: 1,
		cells: [
			['NID', 'Nombre'],
			['A', '2'],
			['Total', '2']
		],
		styles: ['', '{"b":true}'],
		headStyles: ['{"bg":"#ffff00"}', '']
	};
	it('les lignes (total compris) prennent le style de la colonne, l’en-tête le sien', () => {
		expect(pivotColLayer([p], 2, 2)).toEqual({ b: true });
		expect(pivotColLayer([p], 3, 2)).toEqual({ b: true });
		expect(pivotColLayer([p], 1, 1)).toEqual({ bg: '#ffff00' });
		expect(pivotColLayer([p], 1, 2)).toBeUndefined();
		expect(pivotColLayer([p], 4, 2)).toBeUndefined(); // sous le TCD
	});
	it('un style de cellule l’emporte sur celui de la colonne', () => {
		expect(resolveStyle(pivotColLayer([p], 2, 2), { b: null })).toBeUndefined();
	});
});
