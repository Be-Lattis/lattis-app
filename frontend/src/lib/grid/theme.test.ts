import { describe, expect, it } from 'vitest';
import { BUILTIN_THEMES, contrast, DEFAULT_THEME, defaultTheme, family, fromHsl, gridPalette, resolveTheme, themeCatalog, themeId, themeVars, toHsl } from './theme';
import { applyTheme, tableTheme } from './theme.svelte.js';
import type { SettingState } from '#lib/shell/settings.js';

const org = (extra: SettingState[] = []): SettingState[] => [
	{ path: 'ui.themes', value: { acme: { name: 'ACME', table: '#0B5394', source: '#e69138' }, perso: { name: 'Perso', table: '#7c3aed', source: '#0d9488' }, casse: { name: 'Cassé', table: 'bleu', source: '#000000' } }, managed: ['acme'] },
	...extra
];

describe('family : nuances d’une couleur de base', () => {
	it('garde à l’identique les couleurs d’avant les thèmes', () => {
		expect(family('#2f7d55')).toEqual({ base: '#2f7d55', head: '#e3efe7', text: '#14532d', border: '#8fbc9f', icon: '#6b8f78', badge: '#2f7d55', strong: '#217346' });
		expect(family('#3B6FB6').head).toBe('#e3ecf7');
		expect(family('#3b6fb6').text).toBe('#14325a');
	});

	it.each([...BUILTIN_THEMES.flatMap((t) => [t.table, t.source]), '#ffff00', '#00ffff', '#000000', '#ffffff', '#808080', '#ff0000', '#e69138'])('%s : texte lisible, en-tête pâle, pastille lisible', (base) => {
		const f = family(base);
		for (const c of Object.values(f)) expect(c).toMatch(/^#[0-9a-f]{6}$/);
		expect(contrast(f.text, f.head)).toBeGreaterThanOrEqual(4.5);
		expect(contrast(f.badge, '#ffffff')).toBeGreaterThanOrEqual(4.5);
		expect(contrast(f.strong, '#ffffff')).toBeGreaterThanOrEqual(4.5);
		expect(toHsl(f.head)[2]).toBeGreaterThan(0.85);
		// La bordure se voit sur le blanc de la grille, sans écraser l'en-tête.
		expect(contrast(f.border, '#ffffff')).toBeGreaterThan(1.5);
	});

	it('même teinte que la base', () => {
		const [h] = toHsl('#7c3aed');
		const f = family('#7c3aed');
		for (const c of [f.head, f.text, f.border, f.badge]) expect(Math.abs(toHsl(c)[0] - h)).toBeLessThan(4);
		// Un gris reste gris.
		expect(toHsl(family('#64748b').head)[1]).toBeLessThan(0.3);
	});

	it('conversions et contraste', () => {
		expect(fromHsl(...toHsl('#2563eb'))).toBe('#2563eb');
		expect(contrast('#000000', '#ffffff')).toBeCloseTo(21, 5);
		expect(contrast('#777777', '#777777')).toBe(1);
		// Couleur illisible : vert par défaut.
		expect(family('vert').base).toBe('#2f7d55');
	});
});

describe('liste des thèmes et résolution', () => {
	it('prédéfinis, puis organisation, puis utilisateur ; invalides écartés', () => {
		const cat = themeCatalog(org());
		expect(cat.slice(0, BUILTIN_THEMES.length).map((t) => t.id)).toEqual(BUILTIN_THEMES.map((t) => t.id));
		expect(cat.slice(BUILTIN_THEMES.length).map((t) => [t.id, t.origin, t.table])).toEqual([
			['acme', 'org', '#0b5394'],
			['perso', 'user', '#7c3aed']
		]);
		expect(themeCatalog([]).map((t) => t.id)).toEqual(BUILTIN_THEMES.map((t) => t.id));
	});

	it('l’organisation peut ne proposer que les siens', () => {
		expect(themeCatalog(org([{ path: 'ui.builtinThemes', value: false }])).map((t) => t.id)).toEqual(['acme', 'perso']);
		// Liste vide : les prédéfinis malgré tout.
		expect(themeCatalog([{ path: 'ui.builtinThemes', value: false }]).length).toBe(BUILTIN_THEMES.length);
	});

	it('thème par défaut : ui.theme s’il est proposé, sinon Vert et bleu, sinon le premier', () => {
		expect(defaultTheme([]).id).toBe(DEFAULT_THEME);
		expect(defaultTheme(org([{ path: 'ui.theme', value: 'acme', locked: true }])).id).toBe('acme');
		expect(defaultTheme(org([{ path: 'ui.theme', value: 'inconnu' }])).id).toBe(DEFAULT_THEME);
		expect(defaultTheme(org([{ path: 'ui.builtinThemes', value: false }, { path: 'ui.theme', value: DEFAULT_THEME }])).id).toBe('acme');
	});

	it('le thème du classeur l’emporte sur le défaut, même inconnu de ce poste', () => {
		const list = org([{ path: 'ui.theme', value: 'acme' }]);
		expect(resolveTheme(null, list)).toMatchObject({ id: 'acme', name: 'ACME' });
		expect(resolveTheme(undefined, []).table.head).toBe('#e3efe7');
		const r = resolveTheme({ id: 'bleu-orange', name: 'Bleu et orange', table: '#2563eb', source: '#ea580c' }, list);
		expect(r.id).toBe('bleu-orange');
		expect(r.table.base).toBe('#2563eb');
		// Couleurs d'un thème d'un autre poste : appliquées, rien de coché.
		const other = resolveTheme({ id: 'chez-paul', name: 'Paul', table: '#123456', source: '#654321' }, list);
		expect(other).toMatchObject({ id: null, name: 'Paul' });
		expect(other.source.base).toBe('#654321');
		// Mêmes couleurs qu'un thème de la liste : coché.
		expect(resolveTheme({ name: 'x', table: '#7C3AED', source: '#0d9488' }, list).id).toBe('violet-sarcelle');
		// Couleurs illisibles : le défaut.
		expect(resolveTheme({ name: 'x', table: 'rouge', source: '#000000' }, list).id).toBe('acme');
	});

	it('identifiant d’un thème de l’utilisateur', () => {
		expect(themeId('Mon thème été', [])).toBe('mon-theme-ete');
		expect(themeId('Mon thème', ['mon-theme', 'mon-theme-2'])).toBe('mon-theme-3');
		expect(themeId('!!!', [])).toBe('theme');
	});
});

describe('variables CSS et grille', () => {
	it('expose les deux familles en variables', () => {
		const v = themeVars(resolveTheme(null, []));
		expect(v['--lattis-table']).toBe('#2f7d55');
		expect(v['--lattis-table-head']).toBe('#e3efe7');
		expect(v['--lattis-table-text']).toBe('#14532d');
		expect(v['--lattis-table-border']).toBe('#8fbc9f');
		expect(v['--lattis-source-head']).toBe('#e3ecf7');
		expect(v['--lattis-source-text']).toBe('#14325a');
		expect(v['--lattis-source-border']).toBe('#8fa9cc');
		expect(Object.keys(v)).toHaveLength(14);
	});

	it('palette de la grille', () => {
		const p = gridPalette(resolveTheme({ name: 'x', table: '#2563eb', source: '#ea580c' }, []));
		expect(p.head).toBe(family('#2563eb').head);
		expect(p.linkHead).toBe(family('#ea580c').head);
		expect(p.nameBadge).toBe(family('#2563eb').badge);
		expect(p.followBadge).toBe(family('#2563eb').strong);
	});

	it('applyTheme pose les variables et le thème courant', () => {
		const style = new Map<string, string>();
		const root = { style: { setProperty: (k: string, v: string) => style.set(k, v) } } as unknown as HTMLElement;
		applyTheme({ name: 'Bleu et orange', table: '#2563eb', source: '#ea580c' }, [], root);
		expect(tableTheme.current.name).toBe('Bleu et orange');
		expect(style.get('--lattis-table')).toBe('#2563eb');
		expect(style.get('--lattis-source-head')).toBe(family('#ea580c').head);
		applyTheme(null, [], root);
		expect(style.get('--lattis-table-head')).toBe('#e3efe7');
	});
});
