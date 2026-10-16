import { describe, expect, it } from 'vitest';
import { categoryOf, formatCell, formatDateTime, formatPatch, fromSerial, parseDateTime } from './numfmt';
import { autoHeight, cellFont, fontPx, wrapLines } from './textwrap';

describe('formats de date et d’heure', () => {
	const p = parseDateTime('2026-10-16T08:05:09')!;

	it('lit les dates affichées', () => {
		expect(parseDateTime('16/10/2026')).toEqual({ y: 2026, m: 10, d: 16, h: 0, mi: 0, s: 0 });
		expect(parseDateTime('16/10/26 08:30')).toEqual({ y: 2026, m: 10, d: 16, h: 8, mi: 30, s: 0 });
		expect(parseDateTime('2026-02-30')).toBeNull();
		expect(parseDateTime('08:30')?.h).toBe(8);
		expect(parseDateTime('Lyon')).toBeNull();
	});

	it('met au motif, mm étant les minutes après hh', () => {
		expect(formatDateTime(p, 'jj/mm/aaaa')).toBe('16/10/2026');
		expect(formatDateTime(p, 'jj mmm aaaa')).toBe('16 oct. 2026');
		expect(formatDateTime(p, 'jjjj jj mmmm aaaa')).toBe('vendredi 16 octobre 2026');
		expect(formatDateTime(p, 'aaaa-mm-jj')).toBe('2026-10-16');
		expect(formatDateTime(p, 'hh:mm')).toBe('08:05');
		expect(formatDateTime(p, 'jj/mm/aa hh:mm:ss')).toBe('16/10/26 08:05:09');
		expect(formatDateTime(p, 'mmmm aaaa')).toBe('octobre 2026');
	});

	it('lit les numéros de série d’Excel', () => {
		expect(fromSerial(46311)).toMatchObject({ y: 2026, m: 10, d: 16 });
		expect(fromSerial(46311.5)).toMatchObject({ h: 12, mi: 0 });
		expect(fromSerial(-1)).toBeNull();
	});
});

describe('affichage selon le format', () => {
	it('dates, heures et texte', () => {
		expect(formatCell('2026-10-16', '2026-10-16', { nf: 'date', df: 'jj mmmm aaaa' })).toBe('16 octobre 2026');
		expect(formatCell('16/10/2026', undefined, { nf: 'date', df: 'aaaa-mm-jj' })).toBe('2026-10-16');
		expect(formatCell('46311', 46311, { nf: 'date' })).toBe('16/10/2026');
		expect(formatCell('0,5', undefined, { nf: 'time' })).toBe('12:00');
		expect(formatCell('Lyon', 'Lyon', { nf: 'date' })).toBe('Lyon');
		expect(formatCell('1234,5', 1234.5, { nf: 'text' })).toBe('1234,5');
	});

	it('nombres, avec ou sans séparateur de milliers', () => {
		expect(formatCell('1234,5', 1234.5, { nf: '0,00', dp: 1 })).toBe('1 234,5');
		expect(formatCell('1234,5', 1234.5, { nf: '0,00', dp: 2, ng: true })).toBe('1234,50');
		expect(formatCell('1 234,5', undefined, { nf: '0,00', dp: 0, ng: true })).toBe('1235');
		expect(formatCell('12', 12, { nf: '€', dp: 0 })).toBe('12 €');
		expect(formatCell('Total', undefined, { nf: '0,00' })).toBe('Total');
		expect(formatCell('3', 3, { b: true })).toBe('3');
	});

	it('catégories du dialogue', () => {
		expect(categoryOf(undefined)).toBe('standard');
		expect(categoryOf({ nf: '0' })).toBe('number');
		expect(categoryOf({ dp: 1 })).toBe('number');
		expect(categoryOf({ nf: 'date', df: 'jj/mm' })).toBe('date');
		expect(formatPatch('number', { dp: 3, sep: false })).toEqual({ nf: '0,00', dp: 3, ng: true, df: null });
		expect(formatPatch('date', { df: 'aaaa-mm-jj' })).toEqual({ nf: 'date', dp: null, ng: null, df: 'aaaa-mm-jj' });
		expect(formatPatch('standard')).toEqual({ nf: null, dp: null, ng: null, df: null });
	});
});

describe('renvoyer à la ligne', () => {
	const measure = (s: string) => s.length * 10;

	it('coupe aux espaces', () => {
		expect(wrapLines('un deux trois quatre', 100, measure)).toEqual(['un deux', 'trois', 'quatre']);
		expect(wrapLines('a  b', 100, measure)).toEqual(['a b']);
		expect(wrapLines('ligne\nsuivante', 1000, measure)).toEqual(['ligne', 'suivante']);
	});

	it('coupe un mot trop long', () => {
		expect(wrapLines('abcdefghij', 40, measure)).toEqual(['abcd', 'efgh', 'ij']);
		expect(wrapLines('x abcdefgh', 40, measure)).toEqual(['x', 'abcd', 'efgh']);
	});

	it('hauteur automatique', () => {
		expect(autoHeight('court', 200, { wr: true }, measure, 22)).toBe(0);
		expect(autoHeight('un deux trois quatre', 112, { wr: true }, measure, 22)).toBe(3 * 17 + 4);
		expect(autoHeight('x', 100, { fs: 24 }, measure, 22)).toBeGreaterThan(22);
	});

	it('police', () => {
		expect(fontPx(undefined)).toBe(13);
		expect(fontPx(12)).toBe(16);
		expect(cellFont({ b: true, i: true, ff: 'Arial', fs: 9 })).toBe('italic 600 12px "Arial", "Segoe UI", system-ui, -apple-system, sans-serif');
	});
});
