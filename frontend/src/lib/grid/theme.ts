// Thèmes de couleur des tableaux (Affichage › Thème, décision 0017) : une
// couleur pour les tableaux ordinaires, une pour les tableaux chargés par
// une source externe (requête). Chacune est une seule couleur de base dont
// family() tire les nuances (en-tête, texte, bordure, icônes, pastille).
//
// Le thème choisi vit dans le classeur (DocState.theme) ; les réglages
// donnent le thème par défaut (ui.theme) et la liste proposée : les thèmes
// prédéfinis (sauf ui.builtinThemes = false), ceux de l'organisation et
// ceux de l'utilisateur (ui.themes, un objet par identifiant).
import type { SettingState } from '#lib/shell/settings.js';

// Comme findSetting (settings.ts, qui importe ce fichier).
const findSetting = (list: readonly SettingState[], path: string) => list.find((s) => s.path.toLowerCase() === path.toLowerCase());

/** Nuances d'une famille de couleur. */
export interface Family {
	/** Couleur de base. */
	base: string;
	/** Fond de l'en-tête. */
	head: string;
	/** Texte de l'en-tête (contraste ≥ 4,5:1 sur head). */
	text: string;
	/** Cadre et filets du tableau. */
	border: string;
	/** Icônes de l'en-tête (type, ▾), libellés discrets. */
	icon: string;
	/** Pastille du nom (texte blanc, contraste ≥ 4,5:1). */
	badge: string;
	/** Pastille « suit l'année » : plus foncée que badge (texte blanc). */
	strong: string;
}

/**
 * Couleurs du thème au-delà des deux bases, comme Excel (facultatives :
 * absentes, themeColors les déduit des deux bases). accents : Accent 3 à
 * Accent 6 (Accent 1 et 2 sont table et source) ; dark et light : Texte 2
 * et Arrière-plan 2 (Texte 1 et Arrière-plan 1 sont le noir et le blanc).
 */
export interface ThemeExtras {
	accents?: string[] | null;
	dark?: string;
	light?: string;
}

/** Thème proposé (prédéfini, de l'organisation ou de l'utilisateur). */
export interface ThemeDef extends ThemeExtras {
	id: string;
	name: string;
	/** Couleur de base des tableaux ordinaires (#rrggbb). */
	table: string;
	/** Couleur de base des tableaux d'une source externe (#rrggbb). */
	source: string;
	origin: 'builtin' | 'org' | 'user';
}

/** Thème gardé dans un classeur (canvas.Theme). */
export interface WorkbookTheme extends ThemeExtras {
	id?: string;
	name: string;
	table: string;
	source: string;
}

/** Emplacements des couleurs du thème (ordre des colonnes de la palette, comme Excel). */
export const THEME_SLOTS = ['lt1', 'dk1', 'lt2', 'dk2', 'accent1', 'accent2', 'accent3', 'accent4', 'accent5', 'accent6'] as const;
export type ThemeSlot = (typeof THEME_SLOTS)[number];
/** Couleurs du thème (#rrggbb), par emplacement. */
export type ThemeColors = Record<ThemeSlot, string>;

/** Thème appliqué : ses deux familles et ses dix couleurs. */
export interface ResolvedTheme {
	/** Identifiant du thème de la liste qu'il est (coché dans le menu), null : couleurs personnalisées. */
	id: string | null;
	name: string;
	table: Family;
	source: Family;
	colors: ThemeColors;
}

export const DEFAULT_THEME = 'vert-bleu';

// Une ligne par thème (lue aussi par internal/canvas/themecolor_test.go).
export const BUILTIN_THEMES: ThemeDef[] = [
	{ id: 'vert-bleu', name: 'Vert et bleu', table: '#2f7d55', source: '#3b6fb6', accents: ['#e07b24', '#7b4fb0', '#c99a1e', '#b83b5e'], dark: '#34495e', light: '#e8eeea', origin: 'builtin' },
	{ id: 'bleu-orange', name: 'Bleu et orange', table: '#2563eb', source: '#ea580c', accents: ['#16a34a', '#9333ea', '#ca8a04', '#db2777'], dark: '#1e293b', light: '#e2e8f0', origin: 'builtin' },
	{ id: 'violet-sarcelle', name: 'Violet et sarcelle', table: '#7c3aed', source: '#0d9488', accents: ['#f59e0b', '#e11d48', '#2563eb', '#65a30d'], dark: '#3b3355', light: '#ede9f6', origin: 'builtin' },
	{ id: 'sarcelle-violet', name: 'Sarcelle et violet', table: '#0d9488', source: '#7c3aed', accents: ['#f59e0b', '#e11d48', '#2563eb', '#65a30d'], dark: '#134e4a', light: '#e6f2f1', origin: 'builtin' },
	{ id: 'gris-bleu', name: 'Gris et bleu', table: '#64748b', source: '#2563eb', accents: ['#0d9488', '#d97706', '#dc2626', '#7c3aed'], dark: '#1e293b', light: '#e2e8f0', origin: 'builtin' },
	{ id: 'orange-bleu', name: 'Orange et bleu', table: '#c2410c', source: '#2563eb', accents: ['#15803d', '#a16207', '#7c3aed', '#be123c'], dark: '#3b2f2a', light: '#f3ebe5', origin: 'builtin' },
	{ id: 'bordeaux-gris', name: 'Bordeaux et gris', table: '#9f1239', source: '#64748b', accents: ['#b45309', '#0f766e', '#1d4ed8', '#6d28d9'], dark: '#3f1d2b', light: '#efe7ea', origin: 'builtin' }
];

// Les couleurs d'avant les thèmes, gardées à l'identique pour « Vert et bleu ».
const EXACT: Record<string, Family> = {
	'#2f7d55': { base: '#2f7d55', head: '#e3efe7', text: '#14532d', border: '#8fbc9f', icon: '#6b8f78', badge: '#2f7d55', strong: '#217346' },
	'#3b6fb6': { base: '#3b6fb6', head: '#e3ecf7', text: '#14325a', border: '#8fa9cc', icon: '#6b7f99', badge: '#3b6fb6', strong: '#2b5797' }
};

// --- Couleurs ---------------------------------------------------------------

export function isHex(s: unknown): s is string {
	return typeof s === 'string' && /^#[0-9a-f]{6}$/i.test(s);
}

function rgb(hex: string): [number, number, number] {
	const n = parseInt(hex.slice(1), 16);
	return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

function hex(r: number, g: number, b: number): string {
	const c = (v: number) => Math.round(Math.min(255, Math.max(0, v))).toString(16).padStart(2, '0');
	return `#${c(r)}${c(g)}${c(b)}`;
}

/** Teinte (0–360), saturation et luminosité (0–1). */
export function toHsl(h: string): [number, number, number] {
	const [r, g, b] = rgb(h).map((v) => v / 255);
	const mx = Math.max(r, g, b);
	const mn = Math.min(r, g, b);
	const l = (mx + mn) / 2;
	if (mx === mn) return [0, 0, l];
	const d = mx - mn;
	const s = l > 0.5 ? d / (2 - mx - mn) : d / (mx + mn);
	const hue = mx === r ? (g - b) / d + (g < b ? 6 : 0) : mx === g ? (b - r) / d + 2 : (r - g) / d + 4;
	return [hue * 60, s, l];
}

export function fromHsl(h: number, s: number, l: number): string {
	const k = (n: number) => (n + h / 30) % 12;
	const a = s * Math.min(l, 1 - l);
	const f = (n: number) => l - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)));
	return hex(f(0) * 255, f(8) * 255, f(4) * 255);
}

/** Luminance relative (WCAG). */
function luminance(h: string): number {
	const [r, g, b] = rgb(h).map((v) => {
		const c = v / 255;
		return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
	});
	return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/** Rapport de contraste WCAG de deux couleurs (1 à 21). */
export function contrast(a: string, b: string): number {
	const [x, y] = [luminance(a), luminance(b)].sort((p, q) => q - p);
	return (x + 0.05) / (y + 0.05);
}

const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v));

/** Nuances d'une couleur de base : fond d'en-tête pâle, texte foncé lisible dessus, bordure, icônes, pastille. */
export function family(base: string): Family {
	base = isHex(base) ? base.toLowerCase() : '#2f7d55';
	const exact = EXACT[base];
	if (exact) return { ...exact };
	const [h, s, l] = toHsl(base);
	// Gris (peu saturé) : reste gris ; sinon saturation ramenée à des bornes lisibles.
	const sat = (k: number, lo: number, hi: number) => (s < 0.15 ? s : clamp(s * k, lo, hi));
	const head = fromHsl(h, sat(0.6, 0.25, 0.55), 0.92);
	const darkEnough = (sv: number, lv: number, on: string) => {
		let c = fromHsl(h, sv, lv);
		while (contrast(c, on) < 4.5 && lv > 0.05) c = fromHsl(h, sv, (lv -= 0.02));
		return c;
	};
	return {
		base,
		head,
		text: darkEnough(sat(1, 0.45, 0.7), 0.22, head),
		border: fromHsl(h, sat(0.5, 0.2, 0.45), 0.66),
		icon: fromHsl(h, sat(0.3, 0.1, 0.25), 0.48),
		badge: darkEnough(sat(1, 0.35, 0.6), Math.min(l, 0.38), '#ffffff'),
		strong: darkEnough(sat(1, 0.4, 0.7), Math.min(l, 0.3), '#ffffff')
	};
}

// --- Couleurs du thème (palettes de couleur) ----------------------------------

/** Accents, Texte 2 et Arrière-plan 2 valides d'un thème (les autres sont écartés). */
export function themeExtras(t: unknown): ThemeExtras {
	const o = (t ?? {}) as Record<string, unknown>;
	const out: ThemeExtras = {};
	if (Array.isArray(o.accents) && o.accents.length && o.accents.length <= 4 && o.accents.every(isHex)) out.accents = o.accents.map((c: string) => c.toLowerCase());
	if (isHex(o.dark)) out.dark = o.dark.toLowerCase();
	if (isHex(o.light)) out.light = o.light.toLowerCase();
	return out;
}

/**
 * Les dix couleurs d'un thème. Ce qu'il ne donne pas se déduit de ses deux
 * bases (même calcul que canvas.ThemeColors) : accents 3 à 6 à 120° de
 * l'une et de l'autre, Texte 2 très foncé dans la teinte de la source,
 * Arrière-plan 2 très pâle dans celle des tableaux.
 */
export function themeColors(t: { table: string; source: string } & ThemeExtras): ThemeColors {
	const table = isHex(t.table) ? t.table.toLowerCase() : '#2f7d55';
	const source = isHex(t.source) ? t.source.toLowerCase() : '#3b6fb6';
	const [h1, s1] = toHsl(table);
	const [h2, s2] = toHsl(source);
	const s = clamp(Math.max(s1, s2), 0.45, 0.75);
	const turn = (h: number, d: number) => fromHsl((((h + d) % 360) + 360) % 360, s, 0.45);
	const derived = [turn(h1, 120), turn(h2, 120), turn(h1, 240), turn(h2, 240)];
	const own = t.accents?.filter(isHex) ?? [];
	const acc = derived.map((d, i) => own[i]?.toLowerCase() ?? d);
	return {
		lt1: '#ffffff',
		dk1: '#000000',
		lt2: isHex(t.light) ? t.light.toLowerCase() : fromHsl(h1, Math.min(s1, 0.25), 0.93),
		dk2: isHex(t.dark) ? t.dark.toLowerCase() : fromHsl(h2, Math.min(s2, 0.3), 0.25),
		accent1: table,
		accent2: source,
		accent3: acc[0],
		accent4: acc[1],
		accent5: acc[2],
		accent6: acc[3]
	};
}

// --- Liste et résolution ------------------------------------------------------

/** Thèmes proposés : prédéfinis (sauf ui.builtinThemes = false), puis ceux de l'organisation, puis ceux de l'utilisateur. */
export function themeCatalog(list: readonly SettingState[]): ThemeDef[] {
	const builtin = findSetting(list, 'ui.builtinThemes')?.value !== false;
	const st = findSetting(list, 'ui.themes');
	const managed = new Set(st?.managed ?? []);
	const raw = st?.value && typeof st.value === 'object' ? (st.value as Record<string, unknown>) : {};
	const added: ThemeDef[] = [];
	for (const [id, v] of Object.entries(raw)) {
		const t = v as Partial<WorkbookTheme> | null;
		if (!t || !isHex(t.table) || !isHex(t.source) || typeof t.name !== 'string' || !t.name.trim()) continue;
		added.push({ id, name: t.name.trim(), table: t.table.toLowerCase(), source: t.source.toLowerCase(), ...themeExtras(t), origin: managed.has(id) ? 'org' : 'user' });
	}
	const rank = (t: ThemeDef) => (t.origin === 'org' ? 0 : 1);
	added.sort((a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name, 'fr'));
	const out = [...(builtin ? BUILTIN_THEMES : []), ...added.filter((t) => !BUILTIN_THEMES.some((b) => b.id === t.id) || !builtin)];
	return out.length ? out : BUILTIN_THEMES;
}

/** Thème par défaut des classeurs sans thème : ui.theme s'il est dans la liste, sinon le premier proposé. */
export function defaultTheme(list: readonly SettingState[], catalog = themeCatalog(list)): ThemeDef {
	const id = findSetting(list, 'ui.theme')?.value;
	return catalog.find((t) => t.id === id) ?? catalog.find((t) => t.id === DEFAULT_THEME) ?? catalog[0];
}

const same = (a: string, b: string) => a.toLowerCase() === b.toLowerCase();

/** Thème appliqué au classeur : le sien s'il en a un, sinon le défaut des réglages. */
export function resolveTheme(wb: WorkbookTheme | null | undefined, list: readonly SettingState[]): ResolvedTheme {
	const catalog = themeCatalog(list);
	if (wb && isHex(wb.table) && isHex(wb.source)) {
		// Coché dans le menu : le thème de la liste aux mêmes couleurs (de même identifiant d'abord).
		const match = catalog.find((t) => t.id === wb.id && same(t.table, wb.table) && same(t.source, wb.source)) ?? catalog.find((t) => same(t.table, wb.table) && same(t.source, wb.source));
		// Accents absents (classeur d'avant les palettes) : ceux du thème de la liste aux mêmes couleurs.
		const own = themeExtras(wb);
		const extras = own.accents || own.dark || own.light ? own : themeExtras(match);
		return { id: match?.id ?? null, name: wb.name || match?.name || 'Personnalisé', table: family(wb.table), source: family(wb.source), colors: themeColors({ ...extras, table: wb.table, source: wb.source }) };
	}
	const d = defaultTheme(list, catalog);
	return { id: d.id, name: d.name, table: family(d.table), source: family(d.source), colors: themeColors(d) };
}

/** Thème à garder dans le classeur pour un thème de la liste (avec ses accents). */
export function workbookTheme(t: ThemeDef): WorkbookTheme {
	return { id: t.id, name: t.name, table: t.table, source: t.source, ...themeExtras(t) };
}

/** Identifiant libre pour un thème de l'utilisateur tiré de son nom (« Mon thème » : mon-theme, mon-theme-2…). */
export function themeId(name: string, taken: readonly string[]): string {
	const slug =
		name
			.normalize('NFD')
			.replace(/[̀-ͯ]/g, '')
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, '-')
			.replace(/^-+|-+$/g, '')
			.slice(0, 40) || 'theme';
	let id = slug;
	for (let n = 2; taken.includes(id); n++) id = `${slug}-${n}`;
	return id;
}

// --- Variables CSS et palette de la grille --------------------------------------

/** Variables CSS du thème, posées sur la racine du document (--lattis-table…, --lattis-source…). */
export function themeVars(t: ResolvedTheme): Record<string, string> {
	const out: Record<string, string> = {};
	for (const [k, f] of [
		['table', t.table],
		['source', t.source]
	] as const) {
		out[`--lattis-${k}`] = f.base;
		out[`--lattis-${k}-head`] = f.head;
		out[`--lattis-${k}-text`] = f.text;
		out[`--lattis-${k}-border`] = f.border;
		out[`--lattis-${k}-icon`] = f.icon;
		out[`--lattis-${k}-badge`] = f.badge;
		out[`--lattis-${k}-strong`] = f.strong;
	}
	return out;
}

/** Couleurs de la grille (palette C de Sheet.svelte) tirées du thème. */
export function gridPalette(t: ResolvedTheme) {
	return {
		head: t.table.head,
		headText: t.table.text,
		border: t.table.border,
		icon: t.table.icon,
		nameBadge: t.table.badge,
		followBadge: t.table.strong,
		linkHead: t.source.head,
		linkHeadText: t.source.text,
		linkBorder: t.source.border,
		linkIcon: t.source.icon
	};
}
