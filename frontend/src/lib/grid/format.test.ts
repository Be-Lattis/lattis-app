import { describe, expect, it } from 'vitest';
import { editValue, formatValue, isErrorValue } from './format';

describe('formatValue', () => {
	it('affiche selon la locale française', () => {
		expect(formatValue('decimal', 1234.5)).toBe('1 234,50');
		expect(formatValue('date', '2026-10-15')).toBe('15/10/2026');
		expect(formatValue('boolean', 1)).toBe('VRAI');
		expect(formatValue('boolean', 0)).toBe('FAUX');
		expect(formatValue('integer', 100000)).toBe('100000');
		expect(formatValue('text', null)).toBe('');
	});

	it('propose une valeur éditable sans séparateur de milliers', () => {
		expect(editValue('decimal', 1234.5)).toBe('1234,5');
	});
});

describe('valeurs calculées', () => {
	it('s’affichent au format Standard', () => {
		expect(formatValue('formula', 51)).toBe('51');
		expect(formatValue('formula', 0.1 + 0.2)).toBe('0,3');
		expect(formatValue('formula', true)).toBe('VRAI');
		expect(formatValue('formula', '#DIV/0!')).toBe('#DIV/0!');
	});

	it('reconnaissent les erreurs', () => {
		expect(isErrorValue('#N/A')).toBe(true);
		expect(isErrorValue('#NOM?')).toBe(true);
		expect(isErrorValue('#hashtag')).toBe(false);
	});
});
