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

/** Couleurs standard (fixes, quel que soit le thème) : celles d'Excel. */
export const STANDARD_COLORS: { hex: string; name: string }[] = [
	{ hex: '#c00000', name: 'Rouge foncé' },
	{ hex: '#ff0000', name: 'Rouge' },
	{ hex: '#ffc000', name: 'Orange' },
	{ hex: '#ffff00', name: 'Jaune' },
	{ hex: '#92d050', name: 'Vert clair' },
	{ hex: '#00b050', name: 'Vert' },
	{ hex: '#00b0f0', name: 'Bleu clair' },
	{ hex: '#0070c0', name: 'Bleu' },
	{ hex: '#002060', name: 'Bleu foncé' },
	{ hex: '#7030a0', name: 'Violet' }
];

export const SLOT_NAMES: Record<ThemeSlot, string> = {
	lt1: 'Arrière-plan 1',
	dk1: 'Texte 1',
	lt2: 'Arrière-plan 2',
	dk2: 'Texte 2',
	accent1: 'Accent 1',
	accent2: 'Accent 2',
	accent3: 'Accent 3',
	accent4: 'Accent 4',
	accent5: 'Accent 5',
	accent6: 'Accent 6'
};

/** Nom courant d'une couleur (Blanc, Noir, Gris, Rouge, Orange, Jaune, Vert, Sarcelle, Bleu, Violet, Rose). */
export function hueName(hex: string): string {
	const [h, s, l] = toHsl(hex);
	if (l >= 0.97) return 'Blanc';
	if (l <= 0.06) return 'Noir';
	if (s < 0.15) return 'Gris';
	if (h < 15 || h >= 345) return 'Rouge';
	if (h < 45) return 'Orange';
	if (h < 70) return 'Jaune';
	if (h < 160) return 'Vert';
	if (h < 195) return 'Sarcelle';
	if (h < 255) return 'Bleu';
	if (h < 300) return 'Violet';
	return 'Rose';
}

/** Libellé d'une variation (« plus clair 40 % »). */
const pctName = (pct: number) => (pct > 0 ? `plus clair ${pct} %` : `plus foncé ${-pct} %`);

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
