// Grille dans une autre langue que le français (docs/i18n.md, § 5) : les
// formules et les valeurs se tapent et se lisent comme le moteur les rend.
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { setLang } from '#lib/i18n/index.js';
import { literal, parseCondition } from '../remote/rights';
import { callAt, toNextArg } from './autocomplete';
import { explainError } from './errors';
import { formatValue, isErrorValue } from './format';
import { checkFormula } from './formulaEdit';
import { freeKind } from './freecell';
import { formatDateTime, parseDateTime } from './numfmt';
import { numOf } from './rules';
import { todayText } from './shortcuts';
import { styledNumber } from './style';

describe('grille en anglais', () => {
	beforeEach(async () => {
		await setLang('en');
	});
	afterEach(async () => {
		await setLang('fr');
	});

	it('affiche dates, booléens et erreurs dans la langue', () => {
		expect(formatValue('date', '2026-10-15')).toBe('10/15/2026');
		expect(formatValue('boolean', 1)).toBe('TRUE');
		expect(formatValue('formula', false)).toBe('FALSE');
		expect(formatValue('formula', '#VALEUR!')).toBe('#VALUE!');
		expect(isErrorValue('#NAME?')).toBe(true);
		expect(explainError('#VALUE!')).toBe(explainError('#VALEUR!'));
		expect(explainError('#NAME?')).not.toBe('');
	});

	it('formules : « , » sépare les arguments, « . » est la décimale', () => {
		expect(checkFormula('=SUM(1.5, 2)')).toBeNull();
		expect(checkFormula('=SUM(1;2)')).not.toBeNull();
		expect(callAt('=IF(A1>1, ', 9)).toEqual({ name: 'IF', index: 1 });
		expect(toNextArg('=IF(A1', 6)).toEqual({ text: '=IF(A1, ', caret: 8 });
	});

	it('relit les valeurs affichées', () => {
		expect(freeKind('10/19/2026')).toBe('date');
		expect(freeKind('TRUE')).toBe('bool');
		expect(freeKind('1,234.5')).toBe('number');
		expect(parseDateTime('10/19/2026')).toMatchObject({ y: 2026, m: 10, d: 19 });
		expect(formatDateTime({ y: 2026, m: 10, d: 19, h: 0, mi: 0, s: 0 }, 'jj/mm/aaaa')).toBe('10/19/2026');
		expect(numOf('1,234.5')).toBe(1234.5);
		expect(numOf('10/19/2026')).toBe(numOf('2026-10-19'));
		expect(styledNumber('1,234.5', { nf: '0,00' })).toBe('1,234.50');
		expect(todayText(new Date(2026, 9, 19))).toBe('10/19/2026');
	});

	it('droits : littéraux tapés dans la langue', () => {
		expect(literal('10/19/2026', 'date')).toBe("'2026-10-19'");
		expect(literal('yes', 'boolean')).toBe('true');
		expect(literal('vrai', 'boolean')).toBe('true');
		expect(literal('1,234.5', 'decimal')).toBe('1234.5');
		expect(parseCondition(`"Date" = '2026-10-19'`, { Date: 'date' })?.value).toEqual({ kind: 'value', value: '10/19/2026' });
		expect(parseCondition(`"Ok" = true`, { Ok: 'boolean' })?.value).toEqual({ kind: 'value', value: 'true' });
	});
});

describe('grille en allemand', () => {
	beforeEach(async () => {
		await setLang('de');
	});
	afterEach(async () => {
		await setLang('fr');
	});

	it('dates et booléens', () => {
		expect(formatValue('date', '2026-10-15')).toBe('15.10.2026');
		expect(checkFormula('=SUM(1,5; 2)')).toBeNull();
		expect(literal('wahr', 'boolean')).toBe('true');
		expect(literal('19.10.2026', 'date')).toBe("'2026-10-19'");
		expect(literal('1.234,5', 'decimal')).toBe('1234.5');
	});

	it('booléens et erreurs comme Excel en allemand', () => {
		expect(checkFormula('=SUMME(1,5; WAHR)&#WERT!&#NV&#BEZUG!')).toBeNull();
		expect(formatValue('boolean', 1)).toBe('WAHR');
		expect(formatValue('formula', '#VALEUR!')).toBe('#WERT!');
		expect(formatValue('formula', '#N/A')).toBe('#NV');
		expect(isErrorValue('#ZAHL!')).toBe(true);
		expect(explainError('#WERT!')).toBe(explainError('#VALEUR!'));
		expect(freeKind('FALSCH')).toBe('bool');
	});
});

describe('grille en espagnol', () => {
	beforeEach(async () => {
		await setLang('es');
	});
	afterEach(async () => {
		await setLang('fr');
	});

	it('booléens et erreurs comme Excel en espagnol', () => {
		expect(checkFormula('=SUMA(1,5; VERDADERO)&#¡VALOR!&#N/D&#¿NOMBRE?')).toBeNull();
		expect(formatValue('formula', true)).toBe('VERDADERO');
		expect(formatValue('formula', '#NOMBRE!')).toBe('#¡NUM!');
		expect(isErrorValue('#¡REF!')).toBe(true);
		expect(explainError('#¿NOMBRE?')).toBe(explainError('#NOM?'));
		expect(freeKind('falso')).toBe('bool');
	});
});
