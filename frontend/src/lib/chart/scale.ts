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

/** Libellé d'une graduation. */
export function tickLabel(v: number): string {
	return Math.abs(v) >= 10000 ? compact.format(v) : fmt.format(v);
}

/** Nombre écrit à la française, ou null. */
export function parseNumber(s: string): number | null {
	const t = s.replace(/[\s  ]/g, '').replace(',', '.');
	if (!/^[-+]?(\d+\.?\d*|\.\d+)(e[-+]?\d+)?$/i.test(t)) return null;
	return Number(t);
}
