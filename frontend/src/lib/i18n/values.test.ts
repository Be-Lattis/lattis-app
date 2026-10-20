import { afterEach, describe, expect, it } from 'vitest';
import { setLang } from './i18n.svelte.js';
import { boolText, dateStyle, errorCode, fnName, formulaSep, frenchErrorCode, isoToShort, parseBool, parseNumberText, parseShortDate, shortDate } from './values.js';

describe('valeurs dans la langue de la session', () => {
	afterEach(async () => {
		await setLang('fr');
	});

	it('dates courtes de chaque langue', () => {
		expect(shortDate(2026, 1, 5, 'fr')).toBe('05/01/2026');
		expect(shortDate(2026, 1, 5, 'en')).toBe('01/05/2026');
		expect(shortDate(2026, 1, 5, 'de')).toBe('05.01.2026');
		expect(shortDate(2026, 1, 5, 'es')).toBe('05/01/2026');
		expect(shortDate(2026, 1, 5, 'zh')).toBe('2026/01/05');
		expect(isoToShort('2026-10-19', 'en')).toBe('10/19/2026');
		expect(isoToShort('texte', 'en')).toBe('texte');
		expect(dateStyle('de')).toEqual({ order: 'dmy', sep: '.' });
	});

	it('relit les dates tapées dans la langue', () => {
		expect(parseShortDate('19/10/2026', 'fr')).toEqual({ y: 2026, m: 10, d: 19 });
		expect(parseShortDate('10/19/2026', 'en')).toEqual({ y: 2026, m: 10, d: 19 });
		expect(parseShortDate('19.10.26', 'de')).toEqual({ y: 2026, m: 10, d: 19 });
		expect(parseShortDate('2026/10/19', 'zh')).toEqual({ y: 2026, m: 10, d: 19 });
		expect(parseShortDate('2026-10-19', 'en')).toEqual({ y: 2026, m: 10, d: 19 });
		expect(parseShortDate('19/10/2026', 'en')).toBeNull();
		expect(parseShortDate('31/02/2026', 'fr')).toBeNull();
	});

	it('séparateurs, booléens, erreurs et noms de fonctions', () => {
		expect(formulaSep('fr')).toBe(';');
		expect(formulaSep('de')).toBe(';');
		expect(formulaSep('en')).toBe(',');
		expect(formulaSep('zh')).toBe(',');
		expect(boolText(true, 'fr')).toBe('VRAI');
		expect(boolText(false, 'en')).toBe('FALSE');
		expect(parseBool('wahr', 'de')).toBe(true);
		expect(parseBool('Falso', 'es')).toBe(false);
		expect(parseBool('是', 'zh')).toBe(true);
		expect(parseBool('vrai', 'en')).toBe(true);
		expect(parseBool('false', 'fr')).toBe(false);
		expect(parseBool('peut-être', 'fr')).toBeNull();
		expect(errorCode('#VALEUR!', 'en')).toBe('#VALUE!');
		expect(errorCode('#VALEUR!', 'fr')).toBe('#VALEUR!');
		expect(errorCode('#N/A', 'en')).toBe('#N/A');
		expect(frenchErrorCode('#NAME?')).toBe('#NOM?');
		expect(boolText(true, 'de')).toBe('WAHR');
		expect(boolText(false, 'es')).toBe('FALSO');
		expect(boolText(true, 'zh')).toBe('TRUE');
		expect(errorCode('#VALEUR!', 'de')).toBe('#WERT!');
		expect(errorCode('#N/A', 'es')).toBe('#N/D');
		expect(errorCode('#DEVERSEMENT!', 'de')).toBe('#ÜBERLAUF!');
		expect(errorCode('#NOM?', 'zh')).toBe('#NAME?');
		expect(frenchErrorCode('#¿NOMBRE?')).toBe('#NOM?');
		expect(frenchErrorCode('#BEZUG!')).toBe('#REF!');
		expect(frenchErrorCode('#CIRC!')).toBe('#CIRC!');
		const sum = { en: 'SUM', fr: 'SOMME', de: 'SUMME', es: 'SUMA' };
		expect(fnName(sum, 'fr')).toBe('SOMME');
		expect(fnName(sum, 'de')).toBe('SUMME');
		expect(fnName(sum, 'es')).toBe('SUMA');
		expect(fnName(sum, 'zh')).toBe('SUM');
	});

	it('nombres écrits dans la langue', () => {
		expect(parseNumberText('1 234,5', 'fr')).toBe(1234.5);
		expect(parseNumberText('1.234', 'fr')).toBe(1.234);
		expect(parseNumberText('1,234.5', 'en')).toBe(1234.5);
		expect(parseNumberText('1,234', 'en')).toBe(1234);
		expect(parseNumberText('1,5', 'en')).toBe(1.5);
		expect(parseNumberText('1.234,5', 'de')).toBe(1234.5);
		expect(parseNumberText('1.234', 'de')).toBe(1234);
		expect(parseNumberText('12,5 %', 'fr')).toBe(0.125);
		expect(parseNumberText('abc', 'en')).toBeNull();
	});

	it('suit la langue de la session par défaut', async () => {
		expect(formulaSep()).toBe(';');
		await setLang('en');
		expect(formulaSep()).toBe(',');
		expect(boolText(true)).toBe('TRUE');
		expect(shortDate(2026, 10, 19)).toBe('10/19/2026');
	});
});
