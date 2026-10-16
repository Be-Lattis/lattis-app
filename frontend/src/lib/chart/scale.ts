// Échelles des graphiques : graduations « rondes » comme Excel.

export const PALETTE = ['#4472c4', '#ed7d31', '#a5a5a5', '#ffc000', '#5b9bd5', '#70ad47', '#264478', '#9e480e'];

/** Graduations rondes couvrant [lo, hi] (0 inclus), environ n intervalles. */
export function ticks(lo: number, hi: number, n = 5): number[] {
	lo = Math.min(0, lo);
	hi = Math.max(0, hi);
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

const fmt = new Intl.NumberFormat('fr-FR', { maximumFractionDigits: 2 });
const compact = new Intl.NumberFormat('fr-FR', { notation: 'compact', maximumFractionDigits: 1 });

/** Libellé d'une valeur isolée (étiquette de données). */
export function tickLabel(v: number): string {
	return Math.abs(v) >= 10000 ? compact.format(v) : fmt.format(v);
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
	const f = new Intl.NumberFormat('fr-FR', { minimumFractionDigits: decimals, maximumFractionDigits: decimals });
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
