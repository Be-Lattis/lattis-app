import { beforeEach, describe, expect, it } from 'vitest';
import { colorName, hueName, isThemeRef, paint, parseThemeRef, pushRecent, recentColors, resolveColor, setPaintColors, standardRow, STANDARD_COLORS, themeRef, themeRows, tintShade, variants } from './colors';
import { BUILTIN_THEMES, resolveTheme, themeCatalog, themeColors, toHsl, workbookTheme } from './theme';
import { quillFormats } from '#lib/doc/format.js';

const vertBleu = themeColors(BUILTIN_THEMES[0]);
const bleuOrange = themeColors(BUILTIN_THEMES[1]);

describe('variantes à la manière d’Excel (luminosité TSL)', () => {
	// Mêmes valeurs que internal/canvas/themecolor_test.go : l'export calcule pareil.
	it.each([
		['#2f7d55', 0, '#2f7d55'],
		['#2f7d55', 80, '#ceeddd'],
		['#2f7d55', 40, '#6cc898'],
		['#2f7d55', -25, '#235e40'],
		['#2f7d55', -50, '#183f2b'],
		['#000000', 50, '#808080'],
		['#ffffff', -5, '#f2f2f2'],
		['#ffffff', -50, '#808080']
	])('%s %i % → %s', (hex, pct, want) => {
		expect(tintShade(hex, pct)).toBe(want);
	});

	it('plus clair : L + (1 − L)·t ; plus foncé : L·(1 − s)', () => {
		const l = toHsl('#2563eb')[2];
		expect(toHsl(tintShade('#2563eb', 60))[2]).toBeCloseTo(l + (1 - l) * 0.6, 2);
		expect(toHsl(tintShade('#2563eb', -25))[2]).toBeCloseTo(l * 0.75, 2);
		// Teinte gardée.
		expect(Math.abs(toHsl(tintShade('#2563eb', 40))[0] - toHsl('#2563eb')[0])).toBeLessThan(2);
	});

	it('variantes proposées selon la luminosité, comme Excel', () => {
		expect(variants('#ffffff')).toEqual([-5, -15, -25, -35, -50]);
		expect(variants('#000000')).toEqual([50, 35, 25, 15, 5]);
		expect(variants('#e8eeea')).toEqual([-10, -25, -50, -75, -90]);
		expect(variants('#1e293b')).toEqual([90, 75, 50, 25, 10]);
		expect(variants('#2f7d55')).toEqual([80, 60, 40, -25, -50]);
	});
});

describe('couleurs du thème', () => {
	it('dix couleurs : blanc, noir, Arrière-plan 2, Texte 2, six accents', () => {
		expect(vertBleu).toEqual({ lt1: '#ffffff', dk1: '#000000', lt2: '#e8eeea', dk2: '#34495e', accent1: '#2f7d55', accent2: '#3b6fb6', accent3: '#e07b24', accent4: '#7b4fb0', accent5: '#c99a1e', accent6: '#b83b5e' });
	});

	it('déduites des deux bases quand le thème ne les donne pas (comme canvas.ThemeColors)', () => {
		expect(themeColors({ table: '#123456', source: '#654321' })).toMatchObject({ accent3: '#be2873', accent4: '#28be73', accent5: '#73be28', accent6: '#7328be', dk2: '#53402d', lt2: '#e9edf2' });
		const part = themeColors({ table: '#123456', source: '#654321', accents: ['#FF0000'], dark: '#111111' });
		expect(part).toMatchObject({ accent3: '#ff0000', accent4: '#28be73', dk2: '#111111', lt2: '#e9edf2' });
	});

	it('les accents des thèmes prédéfinis sont distincts et lisibles en police sur le blanc', () => {
		for (const t of BUILTIN_THEMES) {
			const c = themeColors(t);
			const acc = [c.accent1, c.accent2, c.accent3, c.accent4, c.accent5, c.accent6];
			expect(new Set(acc).size).toBe(6);
			for (const a of acc) expect(toHsl(a)[2]).toBeLessThan(0.62);
			expect(toHsl(c.dk2)[2]).toBeLessThan(0.3);
			expect(toHsl(c.lt2)[2]).toBeGreaterThan(0.85);
		}
	});

	it('thèmes des réglages : accents facultatifs, invalides écartés', () => {
		const cat = themeCatalog([{ path: 'ui.themes', value: { acme: { name: 'ACME', table: '#0b5394', source: '#e69138', accents: ['#FF0000', '#00b050'], dark: '#111111', light: 'clair' } } }]);
		const acme = cat.find((t) => t.id === 'acme')!;
		expect(acme).toMatchObject({ accents: ['#ff0000', '#00b050'], dark: '#111111' });
		expect(acme.light).toBeUndefined();
		expect(resolveTheme(null, [{ path: 'ui.themes', value: { acme } }, { path: 'ui.theme', value: 'acme' }]).colors).toMatchObject({ accent3: '#ff0000', accent4: '#00b050', dk2: '#111111' });
	});

	it('classeur d’avant les palettes : un thème prédéfini reprend ses accents', () => {
		const old = resolveTheme({ id: 'bleu-orange', name: 'Bleu et orange', table: '#2563eb', source: '#ea580c' }, []);
		expect(old.colors).toEqual(bleuOrange);
		// Gardé avec ses accents depuis la liste.
		expect(workbookTheme(BUILTIN_THEMES[1])).toMatchObject({ accents: BUILTIN_THEMES[1].accents, dark: '#1e293b', light: '#e2e8f0' });
		// Thème du classeur avec ses propres accents : ils l'emportent.
		const own = resolveTheme({ name: 'x', table: '#2563eb', source: '#ea580c', accents: ['#000001', '#000002', '#000003', '#000004'] }, []);
		expect(own.colors.accent6).toBe('#000004');
	});
});

describe('références au thème', () => {
	it('lecture et écriture', () => {
		expect(themeRef('accent1')).toBe('theme:accent1');
		expect(themeRef('accent1', 40)).toBe('theme:accent1:+40');
		expect(themeRef('dk2', -25)).toBe('theme:dk2:-25');
		expect(parseThemeRef('theme:accent6:+80')).toEqual({ slot: 'accent6', pct: 80 });
		expect(parseThemeRef('theme:lt1')).toEqual({ slot: 'lt1', pct: 0 });
		for (const bad of ['theme:accent7', 'theme:accent1:40', 'theme:accent1:+101', '#ff0000', '', null, 12]) expect(parseThemeRef(bad)).toBeNull();
		expect(isThemeRef('theme:dk1:+50')).toBe(true);
		expect(isThemeRef('#000000')).toBe(false);
	});

	it('résolues avec le thème : changer de thème recolore', () => {
		expect(resolveColor('theme:accent1', vertBleu)).toBe('#2f7d55');
		expect(resolveColor('theme:accent1', bleuOrange)).toBe('#2563eb');
		expect(resolveColor('theme:accent3:+40', bleuOrange)).toBe(tintShade('#16a34a', 40));
		expect(resolveColor('#C00000', bleuOrange)).toBe('#C00000');
		expect(resolveColor(undefined, bleuOrange)).toBeUndefined();
		setPaintColors(bleuOrange);
		expect(paint('theme:accent2')).toBe('#ea580c');
		setPaintColors(vertBleu);
		expect(paint('theme:accent2')).toBe('#3b6fb6');
	});

	it('une page (Document) reçoit la couleur résolue', () => {
		setPaintColors(vertBleu);
		expect(quillFormats({ fg: 'theme:accent1', bg: null }, {})).toEqual({ color: '#2f7d55', background: false });
	});

	it('noms des couleurs', () => {
		expect(colorName('theme:accent1', vertBleu)).toBe('Vert, Accent 1');
		expect(colorName('theme:accent1:+40', vertBleu)).toBe('Vert, Accent 1, plus clair 40 %');
		expect(colorName('theme:lt1:-5', vertBleu)).toBe('Blanc, Arrière-plan 1, plus foncé 5 %');
		expect(colorName('theme:dk1', vertBleu)).toBe('Noir, Texte 1');
		expect(colorName('#ff0000')).toBe('Rouge');
		expect(colorName('#12ab34')).toBe('#12AB34');
		expect(hueName('#3b6fb6')).toBe('Bleu');
		expect(hueName('#e07b24')).toBe('Orange');
	});
});

describe('palette', () => {
	it('une ligne des dix couleurs du thème, puis cinq lignes de variantes', () => {
		const rows = themeRows(vertBleu);
		expect(rows).toHaveLength(6);
		for (const r of rows) expect(r).toHaveLength(10);
		expect(rows[0].map((s) => s.value)).toEqual(['theme:lt1', 'theme:dk1', 'theme:lt2', 'theme:dk2', 'theme:accent1', 'theme:accent2', 'theme:accent3', 'theme:accent4', 'theme:accent5', 'theme:accent6']);
		// Sous Accent 1 : plus clair 80, 60, 40 %, plus foncé 25, 50 %.
		expect(rows.slice(1).map((r) => r[4].value)).toEqual(['theme:accent1:+80', 'theme:accent1:+60', 'theme:accent1:+40', 'theme:accent1:-25', 'theme:accent1:-50']);
		expect(rows[3][4]).toEqual({ value: 'theme:accent1:+40', hex: '#6cc898', name: 'Vert, Accent 1, plus clair 40 %' });
		// Sous le blanc : plus foncé ; sous le noir : plus clair.
		expect(rows[1][0].value).toBe('theme:lt1:-5');
		expect(rows[1][1].value).toBe('theme:dk1:+50');
		for (const r of rows) for (const s of r) expect(s.hex).toMatch(/^#[0-9a-f]{6}$/);
	});

	it('couleurs standard : celles d’Excel, rouge, orange et vert compris, quel que soit le thème', () => {
		expect(standardRow().map((s) => s.name)).toEqual(['Rouge foncé', 'Rouge', 'Orange', 'Jaune', 'Vert clair', 'Vert', 'Bleu clair', 'Bleu', 'Bleu foncé', 'Violet']);
		expect(standardRow().map((s) => s.value)).toEqual(STANDARD_COLORS.map((c) => c.hex));
		for (const s of standardRow()) {
			expect(s.value).toMatch(/^#[0-9a-f]{6}$/);
			expect(s.value).toBe(s.hex);
		}
		expect(standardRow().find((s) => s.name === 'Rouge')?.value).toBe('#ff0000');
		expect(standardRow().find((s) => s.name === 'Orange')?.value).toBe('#ffc000');
		expect(standardRow().find((s) => s.name === 'Vert')?.value).toBe('#00b050');
	});
});

describe('couleurs récentes', () => {
	beforeEach(() => {
		try {
			localStorage.removeItem('lattis.recentColors');
		} catch {
			// pas de stockage
		}
	});

	it('la plus récente d’abord, sans doublon, dix au plus', () => {
		let list: string[] = [];
		for (let i = 0; i < 12; i++) list = pushRecent(`#0000${String(i).padStart(2, '0')}`, list);
		expect(list).toHaveLength(10);
		expect(list[0]).toBe('#000011');
		list = pushRecent('#000005', list);
		expect(list[0]).toBe('#000005');
		expect(list.filter((c) => c === '#000005')).toHaveLength(1);
		if (typeof localStorage !== 'undefined') expect(recentColors()).toEqual(list);
	});
});
