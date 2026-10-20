// Palettes de couleur (police, remplissage, bordure), comme Excel
// (décision 0017) : les dix couleurs du thème et cinq variantes de chacune
// (plus claires, plus foncées), puis dix couleurs standard fixes.
//
// Une couleur prise dans le thème est gardée comme référence au thème
// (« theme:accent1:+40 » : Accent 1, plus clair 40 % ; « theme:dk2:-25 » :
// Texte 2, plus foncé 25 %) : changer le thème du classeur recolore ces
// cellules. Les couleurs standard et « Autres couleurs… » restent #rrggbb.
// paint() rend la couleur à dessiner pour le thème appliqué.
import { BUILTIN_THEMES, fromHsl, isHex, THEME_SLOTS, themeColors, toHsl, type ThemeColors, type ThemeSlot } from './theme';
import { t } from '#lib/i18n/index.js';

/** Couleur variée d'un pourcentage, à la manière d'Excel (sur la luminosité TSL) : t > 0 plus claire (L + (1 − L)·t), t < 0 plus foncée (L·(1 + t)). */
export function tintShade(hex: string, pct: number): string {
	if (!pct) return hex.toLowerCase();
	const [h, s, l] = toHsl(hex);
	const t = Math.max(-100, Math.min(100, pct)) / 100;
	return fromHsl(h, s, t > 0 ? l + (1 - l) * t : l * (1 + t));
}

/** Variantes proposées sous une couleur du thème (pourcentages), selon sa luminosité, comme Excel. */
export function variants(hex: string): number[] {
	const l = toHsl(hex)[2];
	if (l >= 1) return [-5, -15, -25, -35, -50];
	if (l <= 0) return [50, 35, 25, 15, 5];
	if (l > 0.8) return [-10, -25, -50, -75, -90];
	if (l < 0.2) return [90, 75, 50, 25, 10];
	return [80, 60, 40, -25, -50];
}

export interface ThemeRef {
	slot: ThemeSlot;
	/** Pourcentage (−100 à 100), 0 : la couleur elle-même. */
	pct: number;
}

const REF = /^theme:(lt1|dk1|lt2|dk2|accent[1-6])(?::([+-]\d{1,3}))?$/;

export function isThemeRef(v: unknown): v is string {
	return typeof v === 'string' && REF.test(v);
}

export function parseThemeRef(v: unknown): ThemeRef | null {
	const m = typeof v === 'string' ? REF.exec(v) : null;
	if (!m) return null;
	const pct = m[2] ? Number(m[2]) : 0;
	return Math.abs(pct) <= 100 ? { slot: m[1] as ThemeSlot, pct } : null;
}

export function themeRef(slot: ThemeSlot, pct = 0): string {
	return pct ? `theme:${slot}:${pct > 0 ? '+' : ''}${pct}` : `theme:${slot}`;
}

/** Couleur #rrggbb d'une couleur de style (référence au thème résolue ; autre valeur rendue telle quelle). */
export function resolveColor<T extends string | null | undefined>(v: T, colors: ThemeColors): T | string {
	const r = parseThemeRef(v);
	return r ? tintShade(colors[r.slot], r.pct) : v;
}

// Thème appliqué (applyTheme, theme.svelte.ts) : celui de paint().
let current: ThemeColors = themeColors(BUILTIN_THEMES[0]);

export function setPaintColors(c: ThemeColors) {
	current = c;
}

export function paintColors(): ThemeColors {
	return current;
}

/** Couleur à dessiner (canevas, CSS) d'une couleur de style, dans le thème appliqué. */
export function paint<T extends string | null | undefined>(v: T): T | string {
	return resolveColor(v, current);
}

// --- Noms ----------------------------------------------------------------------

/** Couleurs standard (fixes, quel que soit le thème) : celles d'Excel ; noms lus à chaque usage (langue). */
export const STANDARD_COLORS: { hex: string; name: string }[] = [
	{ hex: '#c00000', get name() { return t('grid.color.standard.darkRed'); } },
	{ hex: '#ff0000', get name() { return t('grid.color.standard.red'); } },
	{ hex: '#ffc000', get name() { return t('grid.color.standard.orange'); } },
	{ hex: '#ffff00', get name() { return t('grid.color.standard.yellow'); } },
	{ hex: '#92d050', get name() { return t('grid.color.standard.lightGreen'); } },
	{ hex: '#00b050', get name() { return t('grid.color.standard.green'); } },
	{ hex: '#00b0f0', get name() { return t('grid.color.standard.lightBlue'); } },
	{ hex: '#0070c0', get name() { return t('grid.color.standard.blue'); } },
	{ hex: '#002060', get name() { return t('grid.color.standard.darkBlue'); } },
	{ hex: '#7030a0', get name() { return t('grid.color.standard.purple'); } }
];

export const SLOT_NAMES: Record<ThemeSlot, string> = {
	get lt1() {
		return t('grid.color.slot.lt1');
	},
	get dk1() {
		return t('grid.color.slot.dk1');
	},
	get lt2() {
		return t('grid.color.slot.lt2');
	},
	get dk2() {
		return t('grid.color.slot.dk2');
	},
	get accent1() {
		return t('grid.color.slot.accent1');
	},
	get accent2() {
		return t('grid.color.slot.accent2');
	},
	get accent3() {
		return t('grid.color.slot.accent3');
	},
	get accent4() {
		return t('grid.color.slot.accent4');
	},
	get accent5() {
		return t('grid.color.slot.accent5');
	},
	get accent6() {
		return t('grid.color.slot.accent6');
	}
};

/** Nom courant d'une couleur (Blanc, Noir, Gris, Rouge, Orange, Jaune, Vert, Sarcelle, Bleu, Violet, Rose). */
export function hueName(hex: string): string {
	const [h, s, l] = toHsl(hex);
	if (l >= 0.97) return t('grid.color.hue.white');
	if (l <= 0.06) return t('grid.color.hue.black');
	if (s < 0.15) return t('grid.color.hue.gray');
	if (h < 15 || h >= 345) return t('grid.color.hue.red');
	if (h < 45) return t('grid.color.hue.orange');
	if (h < 70) return t('grid.color.hue.yellow');
	if (h < 160) return t('grid.color.hue.green');
	if (h < 195) return t('grid.color.hue.teal');
	if (h < 255) return t('grid.color.hue.blue');
	if (h < 300) return t('grid.color.hue.purple');
	return t('grid.color.hue.pink');
}

/** Libellé d'une variation (« plus clair 40 % »). */
const pctName = (pct: number) => (pct > 0 ? t('grid.color.lighter', { pct }) : t('grid.color.darker', { pct: -pct }));

/** Nom d'une couleur de style (infobulles, lecteurs d'écran) : « Vert, Accent 1, plus clair 40 % », « Rouge », « #12AB34 ». */
export function colorName(v: string | null | undefined, colors: ThemeColors = current): string {
	if (!v) return '';
	const r = parseThemeRef(v);
	if (r) return [hueName(colors[r.slot]), SLOT_NAMES[r.slot], r.pct ? pctName(r.pct) : ''].filter(Boolean).join(', ');
	const std = STANDARD_COLORS.find((c) => c.hex === v.toLowerCase());
	if (std) return std.name;
	return isHex(v) ? v.toUpperCase() : v;
}

// --- Palette -----------------------------------------------------------------------

export interface Swatch {
	/** Valeur gardée dans le style (référence au thème ou #rrggbb). */
	value: string;
	/** Couleur affichée. */
	hex: string;
	name: string;
}

/** Couleurs du thème de la palette : une ligne des dix couleurs, puis cinq lignes de variantes (colonne par colonne). */
export function themeRows(colors: ThemeColors = current): Swatch[][] {
	const head = THEME_SLOTS.map((slot) => ({ value: themeRef(slot), hex: colors[slot], name: colorName(themeRef(slot), colors) }));
	const cols = THEME_SLOTS.map((slot) => variants(colors[slot]));
	const rows: Swatch[][] = [head];
	for (let k = 0; k < 5; k++)
		rows.push(
			THEME_SLOTS.map((slot, j) => {
				const value = themeRef(slot, cols[j][k]);
				return { value, hex: tintShade(colors[slot], cols[j][k]), name: colorName(value, colors) };
			})
		);
	return rows;
}

/** Couleurs standard de la palette. */
export function standardRow(): Swatch[] {
	return STANDARD_COLORS.map((c) => ({ value: c.hex, hex: c.hex, name: c.name }));
}

// --- Couleurs récentes (propres au poste) ------------------------------------------

const RECENT_KEY = 'lattis.recentColors';
const RECENT_MAX = 10;

/** Couleurs choisies récemment par « Autres couleurs… » (la plus récente d'abord). */
export function recentColors(): string[] {
	try {
		const v = JSON.parse(localStorage.getItem(RECENT_KEY) ?? '[]');
		return Array.isArray(v) ? v.filter((c) => isHex(c) || isThemeRef(c)).slice(0, RECENT_MAX) : [];
	} catch {
		return [];
	}
}

/** Ajoute une couleur en tête des couleurs récentes. */
export function pushRecent(c: string, list = recentColors()): string[] {
	const out = [c, ...list.filter((x) => x !== c)].slice(0, RECENT_MAX);
	try {
		localStorage.setItem(RECENT_KEY, JSON.stringify(out));
	} catch {
		// stockage indisponible : la liste vaut pour la session
	}
	return out;
}
