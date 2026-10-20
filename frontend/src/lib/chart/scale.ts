// Échelles des graphiques : graduations « rondes » comme Excel.
import { numberFormat } from '#lib/i18n/index.js';

export const PALETTE = ['#4472c4', '#ed7d31', '#a5a5a5', '#ffc000', '#5b9bd5', '#70ad47', '#264478', '#9e480e'];

/**
 * Couleur d'un texte posé sur le fond bg (« #rrggbb ») : blanc sur un fond
 * foncé, presque noir sur un fond clair (jaune, gris), d'après la
 * luminance relative (WCAG) ; le contraste choisi est le meilleur des deux.
 */
export function textOn(bg: string): string {
	const m = /^#?([0-9a-f]{6})$/i.exec(bg.trim());
	if (!m) return '#fff';
	const lin = (c: number) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
	const n = parseInt(m[1], 16);
	const L = 0.2126 * lin(((n >> 16) & 255) / 255) + 0.7152 * lin(((n >> 8) & 255) / 255) + 0.0722 * lin((n & 255) / 255);
	// Contraste avec le blanc (1,05 / (L + 0,05)) contre le noir ((L + 0,05) / 0,05).
	return 1.05 / (L + 0.05) >= (L + 0.05) / 0.05 ? '#fff' : '#1f2328';
}

/** Graduations rondes couvrant [lo, hi] (0 inclus, sauf zero faux), environ n intervalles. */
export function ticks(lo: number, hi: number, n = 5, zero = true): number[] {
	if (zero) {
		lo = Math.min(0, lo);
		hi = Math.max(0, hi);
	}
	if (lo === hi) hi = lo + 1;
	const raw = (hi - lo) / n;
	const mag = 10 ** Math.floor(Math.log10(raw));
	const step = [1, 2, 2.5, 5, 10].map((m) => m * mag).find((s) => s >= raw) ?? 10 * mag;
	const out: number[] = [];
	const at = (k: number) => Number((k * step).toPrecision(12));
	let k = Math.floor(lo / step);
	do out.push(at(k++));
	while (out[out.length - 1] < hi);
	return out;
}

/**
 * Règle d'Excel pour l'origine d'un axe de valeurs automatique : 0 est
 * inclus, sauf si toutes les valeurs ont le même signe et restent
 * resserrées (la plus petite dépasse les 5/6 de la plus grande, en valeur
 * absolue) : 1 000 à 1 050 ne donne pas une courbe plate.
 */
export function autoZero(lo: number, hi: number): boolean {
	if (lo > 0) return lo < (5 / 6) * hi;
	if (hi < 0) return hi > (5 / 6) * lo;
	return true;
}

const DAY_MS = 86400000;
const EPOCH = Date.UTC(1899, 11, 30);
const serialOf = (y: number, m: number) => (Date.UTC(y, m, 1) - EPOCH) / DAY_MS;

/**
 * Graduations d'un axe de dates (numéros de série) : tous les 1, 2, 7 ou 14
 * jours, sinon au premier du mois tous les 1, 2, 3, 6, 12… mois ; environ
 * n intervalles, couvrant [lo, hi].
 */
export function dateTicks(lo: number, hi: number, n = 5): number[] {
	if (!(hi > lo)) return [Math.floor(lo), Math.floor(lo) + 1];
	const span = hi - lo;
	const day = [1, 2, 7, 14].find((d) => span / d <= n * 1.6);
	const out: number[] = [];
	if (day !== undefined) {
		for (let v = Math.floor(lo / day) * day; ; v += day) {
			out.push(v);
			if (v >= hi) return out;
		}
	}
	const k = [1, 2, 3, 6, 12, 24, 60, 120, 240, 600].find((m) => span / (m * 30.44) <= n * 1.6) ?? 1200;
	const d = new Date(EPOCH + Math.floor(lo) * DAY_MS);
	let i = Math.floor((d.getUTCFullYear() * 12 + d.getUTCMonth()) / k) * k;
	for (;;) {
		const v = serialOf(Math.floor(i / 12), i % 12);
		out.push(v);
		if (v >= hi) return out;
		i += k;
	}
}

const fmt = () => numberFormat({ maximumFractionDigits: 2 });
const compact = () => numberFormat({ notation: 'compact', maximumFractionDigits: 1 });

/** Libellé d'une valeur isolée (étiquette de données). */
export function tickLabel(v: number): string {
	return Math.abs(v) >= 10000 ? compact().format(v) : fmt().format(v);
}

/**
 * Format commun à toutes les graduations d'un axe : notation compacte
 * (« 10 k ») pour toutes si la plus grande atteint 10 000, sinon nombres
 * entiers ou décimaux pour toutes (jamais « 8 000 » à côté de « 10 k »).
 */
export function axisFormat(values: number[]): (v: number) => string {
	const top = Math.max(0, ...values.map(Math.abs));
	// Même unité pour toutes les graduations (500 s'écrit 0,5 k sur un axe
	// qui monte à 10 k), et juste assez de décimales.
	const [unit, suffix] = top >= 1e10 ? [1e9, ' Md'] : top >= 1e7 ? [1e6, ' M'] : top >= 1e4 ? [1e3, ' k'] : [1, ''];
	const scaled = values.map((v) => v / unit);
	const decimals = Math.min(unit === 1 ? 6 : 2, Math.max(0, ...scaled.map(decimalsOf)));
	const f = numberFormat({ minimumFractionDigits: decimals, maximumFractionDigits: decimals });
	return (v) => (unit === 1 ? f.format(v) : v === 0 ? '0' : f.format(v / unit) + suffix);
}

function decimalsOf(v: number): number {
	const s = String(Number(v.toPrecision(12)));
	const k = s.indexOf('.');
	return k < 0 || s.includes('e') ? 0 : s.length - k - 1;
}

/** Libellé tronqué à k caractères. */
export function shortLabel(s: string, k = 12): string {
	return s.length > k ? s.slice(0, k - 1) + '…' : s;
}

export interface CategoryLayout {
	/** Une étiquette sur `every` (1 : toutes). */
	every: number;
	/** Inclinaison des étiquettes en degrés (0 : horizontales, -45 : inclinées). */
	angle: number;
	/** Hauteur occupée sous l'axe par les étiquettes, en pixels. */
	depth: number;
	/** Longueur maximale d'une étiquette, en caractères. */
	chars: number;
}

/**
 * Disposition des étiquettes de l'axe des catégories (horizontal) : droites
 * si elles tiennent dans leur bande, sinon inclinées à 45° comme Excel ; une
 * sur deux (ou plus) seulement si même inclinées elles se chevauchent.
 */
export function categoryLayout(labels: string[], band: number, charW = 6.2, lineH = 13): CategoryLayout {
	const longest = Math.max(0, ...labels.map((l) => shortLabel(l).length));
	if (longest * charW <= band - 4) return { every: 1, angle: 0, depth: 14, chars: 12 };
	const chars = 16;
	const w = Math.max(0, ...labels.map((l) => shortLabel(l, chars).length)) * charW;
	// Inclinées à 45°, deux étiquettes voisines doivent être écartées d'au
	// moins une hauteur de ligne / sin 45°.
	const every = Math.max(1, Math.ceil((lineH * Math.SQRT2) / Math.max(1, band)));
	return { every, angle: -45, depth: Math.ceil(w * Math.SQRT1_2 + lineH), chars };
}

/** Nombre écrit à la française, ou null. */
export function parseNumber(s: string): number | null {
	const t = s.replace(/[\s  ]/g, '').replace(',', '.');
	if (!/^[-+]?(\d+\.?\d*|\.\d+)(e[-+]?\d+)?$/i.test(t)) return null;
	return Number(t);
}

export interface LegendPlace {
	x: number;
	row: number;
	/** Libellé affiché (coupé seulement s'il dépasse toute la largeur). */
	text: string;
}

/**
 * Légende en haut ou en bas : les entrées s'enchaînent sur toute la largeur
 * et passent à la rangée suivante au besoin, libellés complets ; au-delà de
 * maxRows rangées, les dernières entrées sont omises. Renvoie la place de
 * chaque entrée affichée et le nombre de rangées.
 */
export function legendFlow(names: string[], width: number, maxRows: number, charW = 6.2): { items: LegendPlace[]; rows: number } {
	const items: LegendPlace[] = [];
	const room = Math.max(40, width - 20);
	const maxChars = Math.max(4, Math.floor((room - 30) / charW));
	let x = 0;
	let row = 0;
	for (const n of names) {
		const text = shortLabel(n, maxChars);
		const w = 14 + Math.ceil(text.length * charW) + 14;
		if (x > 0 && x + w > room) {
			row++;
			x = 0;
		}
		if (row >= maxRows) break;
		items.push({ x: 10 + x, row, text });
		x += w;
	}
	return { items, rows: items.length ? row + 1 : 0 };
}
