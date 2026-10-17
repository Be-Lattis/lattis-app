import { describe, expect, it } from 'vitest';
import { addBreak, delimString, formatLabel, guessDelims, guessFixed, maxChars, suggestBreaks } from './textcols';

describe('Convertir : séparateurs', () => {
	it('écrit les séparateurs cochés', () => {
		expect(delimString({ tab: true, semi: true, comma: false, space: true, other: true, otherChar: '|x' })).toBe('\t; |');
		expect(delimString({ tab: false, semi: false, comma: false, space: false, other: true, otherChar: '' })).toBe('');
	});
	it('propose la tabulation, ou le point-virgule, ou la virgule', () => {
		expect(guessDelims(['a\tb'])).toMatchObject({ tab: true, semi: false });
		expect(guessDelims(['a;b', 'c,d'])).toMatchObject({ tab: false, semi: true, comma: false });
		expect(guessDelims(['a,b'])).toMatchObject({ tab: false, comma: true });
		expect(guessDelims(['ab'])).toMatchObject({ tab: true });
	});
});

describe('Convertir : largeur fixe', () => {
	const lines = ['Dupont  12  Lyon', 'Li      7   Rome'];
	it('propose les coupures des colonnes alignées', () => {
		expect(suggestBreaks(lines)).toEqual([8, 12]);
		expect(guessFixed(lines)).toBe(true);
		expect(guessFixed(['a;b', 'c;d'])).toBe(false);
	});
	it('ajoute des coupures, sans doublon ni hors des bornes', () => {
		expect(addBreak([4], 2, 10)).toEqual([2, 4]);
		expect(addBreak([4], 4, 10)).toEqual([4]);
		expect(addBreak([4], 10, 10)).toEqual([4]);
	});
	it('compte les caractères, pas les unités UTF-16', () => {
		expect(maxChars(['é', '😀😀'])).toBe(2);
	});
});

describe('Convertir : libellés des formats', () => {
	it('suit Excel', () => {
		expect(formatLabel(undefined)).toBe('Standard');
		expect(formatLabel({ format: 'text', order: '' })).toBe('Texte');
		expect(formatLabel({ format: 'date', order: 'MDY' })).toBe('MJA');
		expect(formatLabel({ format: 'skip', order: '' })).toBe('Ignorer la colonne');
	});
});
