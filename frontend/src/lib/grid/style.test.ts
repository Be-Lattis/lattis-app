import { describe, expect, it } from 'vitest';
import { boxStyle, clearPatch, formatNumber, formatStat, isNumericText, mergeStyles, painterPatch, parseStyle, resolveStyle, styledNumber, STYLE_KEYS } from './style';

describe('mise en forme', () => {
	it('lit et fusionne les styles', () => {
		expect(parseStyle('{"b":true}')).toEqual({ b: true });
		expect(parseStyle('')).toBeUndefined();
		expect(mergeStyles({ nf: '0,00', al: 'center' }, { b: true, al: 'left' })).toEqual({ nf: '0,00', al: 'left', b: true });
	});

	it('formate les nombres', () => {
		expect(formatNumber('0,00', 1234.5)).toBe('1\u202f234,50');
		expect(formatNumber('%', 0.125)).toBe('12,5\u00a0%');
		expect(formatNumber('€', 3)).toBe('3,00\u00a0€');
		expect(formatNumber(undefined, 3)).toBeNull();
	});

	it('superpose les couches : cellule > ligne > colonne, null retire', () => {
		const col = { bg: '#eeeeee', b: true };
		const row = { fg: '#ff0000', al: 'center' as const };
		expect(resolveStyle(col, row, undefined)).toEqual({ bg: '#eeeeee', b: true, fg: '#ff0000', al: 'center' });
		// La cellule retire le gras de sa colonne et change l'alignement de sa ligne.
		expect(resolveStyle(col, row, { b: null, al: 'left' })).toEqual({ bg: '#eeeeee', fg: '#ff0000', al: 'left' });
		expect(resolveStyle(undefined, null, undefined)).toBeUndefined();
		expect(resolveStyle({ b: null }, { b: false })).toBeUndefined();
		// Les couches ne sont pas modifiées.
		expect(col).toEqual({ bg: '#eeeeee', b: true });
	});

	it('pinceau et effacement couvrent toutes les propriétés', () => {
		expect(Object.keys(clearPatch())).toEqual([...STYLE_KEYS]);
		const p = painterPatch({ b: true, nf: '%', dp: 1 });
		expect(p.b).toBe(true);
		expect(p.dp).toBe(1);
		expect(p.bg).toBeNull();
		expect(Object.keys(p)).toHaveLength(STYLE_KEYS.length);
	});

	it('met au format les nombres affichés', () => {
		expect(isNumericText('1 234,5')).toBe(true);
		expect(isNumericText('abc')).toBe(false);
		expect(styledNumber('1234,5', { nf: '0,00' })).toBe('1 234,50');
		expect(styledNumber('12,5 %', { dp: 0, nf: '%' })).toBe('13 %');
		expect(styledNumber('Total', { nf: '0,00' })).toBe('Total');
		expect(styledNumber('3', undefined)).toBe('3');
	});
});

describe('barre d’état', () => {
	const nb = (s: string) => s.replace(/[  ]/g, ' ');
	it('arrondit la moyenne, avec le séparateur de milliers', () => {
		expect(formatStat(30.238333333333)).toBe('30,2383');
		expect(nb(formatStat(1200))).toBe('1 200');
		expect(formatStat(0.0000123456)).toBe('0,00001235');
	});
	it('suit le format de la cellule active', () => {
		expect(formatStat(30.238333, { dp: 2 })).toBe('30,24');
		expect(nb(formatStat(1234.5, { nf: '€' }))).toBe('1 234,50 €');
		expect(formatStat(0.256, { nf: '%' })).toMatch(/^25,6\s?%$/);
	});
});

describe('cellule faite de plusieurs cellules (bande des groupes)', () => {
	it('sans mise en forme : rien', () => {
		expect(boxStyle([[undefined, undefined, undefined]])).toBeUndefined();
	});

	it('Contour sur la bande et ses sous-en-têtes : le tour de la cellule de la bande', () => {
		// Contour posé sur la bande (une ligne, trois colonnes) au-dessus des en-têtes :
		// haut sur toute la ligne, gauche sur la première colonne, droite sur la dernière.
		const band = [{ bt: true, bl: true }, { bt: true }, { bt: true, br: true }];
		expect(boxStyle([band])).toEqual({ bt: true, bl: true, br: true });
	});

	it('colonne sur un niveau : haut de la bande, bas de l’en-tête', () => {
		expect(boxStyle([[{ bt: true, bl: true, br: true }], [{ bb: true, bl: true, br: true }]])).toEqual({ bt: true, bb: true, bl: true, br: true });
		// Bords intérieurs ignorés : le bas de la bande est à l'intérieur de la cellule.
		expect(boxStyle([[{ bb: true }], [undefined]])).toBeUndefined();
	});

	it('la première cellule qui définit une propriété l’emporte', () => {
		expect(boxStyle([[{ bg: '#ff0000' }, { bg: '#00ff00', i: true }]])).toEqual({ bg: '#ff0000', i: true });
		// Ancienne mise en forme posée sur l'en-tête seul : reprise par la cellule entière.
		expect(boxStyle([[undefined], [{ bg: '#00ff00', wr: true }]])).toEqual({ bg: '#00ff00', wr: true });
	});

	it('couleur des bordures', () => {
		expect(boxStyle([[{ fg: '#111111' }, { br: true, bc: '#ff0000' }]])).toEqual({ fg: '#111111', br: true, bc: '#ff0000' });
	});
});
