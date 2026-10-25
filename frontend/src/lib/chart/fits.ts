// Courbes de tendance, barres d'erreur, bulles et bougies : ce que le
// dessin (Chart.svelte) tire des calculs du classeur (canvas/chartfit.go).

/** Types qui acceptent courbes de tendance et barres d'erreur (non empilés). */
export const FITTABLE = new Set<string>(['bar', 'line', 'area', 'scatter', 'combo', 'bubble']);

export type TrendType = 'linear' | 'exp' | 'log' | 'poly' | 'power' | 'movavg';
export const TREND_TYPES: TrendType[] = ['linear', 'exp', 'log', 'poly', 'power', 'movavg'];
export type ErrorType = 'fixed' | 'percent' | 'stddev' | 'stderr' | 'custom';
export const ERROR_TYPES: ErrorType[] = ['fixed', 'percent', 'stddev', 'stderr', 'custom'];

/** Courbe de tendance d'une série (canvas.Trendline). */
export interface Trendline {
	type: string;
	order?: number;
	period?: number;
	forward?: number;
	backward?: number;
	equation?: boolean;
	r2?: boolean;
}

/** Barres d'erreur d'une série (canvas.ErrorBars). */
export interface ErrorBars {
	type: string;
	value?: number;
	plus?: string;
	minus?: string;
	dir?: string;
	noCap?: boolean;
}

/** Courbe calculée (canvas.TrendFit). */
export interface TrendFit {
	type: string;
	coef: number[] | null;
	r2: number;
	points: number[][] | null;
	/** X est le rang de la catégorie (1, 2…). */
	index?: boolean;
}

/** Barres calculées (canvas.ErrorValues). */
export interface ErrorValues {
	plus: (number | null)[] | null;
	minus: (number | null)[] | null;
	center?: number | null;
}

/** Morceau d'une équation : texte, en exposant ou non. */
export interface EqPart {
	t: string;
	sup?: boolean;
}

const SUP: Record<string, string> = { '2': '²', '3': '³', '4': '⁴', '5': '⁵', '6': '⁶' };

/**
 * Équation d'une courbe de tendance, comme un tableur : « y = 2x + 1 »,
 * « y = 0,1x² − 2x + 3 », « y = 2e^(0,5x) » (exposant), « y = 3ln(x) + 1 »,
 * « y = 4x^1,5 ». num : écriture d'un nombre (langue de l'interface).
 */
export function trendEquation(fit: TrendFit, num: (v: number) => string): EqPart[] {
	const c = fit.coef ?? [];
	const term = (v: number, first: boolean) => (first ? (v < 0 ? '−' : '') : v < 0 ? ' − ' : ' + ') + num(Math.abs(v));
	switch (fit.type) {
		case 'linear':
		case 'poly': {
			let s = 'y = ';
			let first = true;
			for (let k = c.length - 1; k >= 0; k--) {
				const v = c[k];
				if (v === 0 && !(k === 0 && first)) continue;
				const coef = Math.abs(v) === 1 && k > 0 ? (first ? (v < 0 ? '−' : '') : v < 0 ? ' − ' : ' + ') : term(v, first);
				s += coef + (k === 0 ? '' : k === 1 ? 'x' : 'x' + (SUP[k] ?? `^${k}`));
				first = false;
			}
			return [{ t: s }];
		}
		case 'exp':
			return [{ t: `y = ${term(c[0], true)}e` }, { t: `${term(c[1], true)}x`, sup: true }];
		case 'log':
			return [{ t: `y = ${term(c[0], true)}ln(x)${c[1] ? term(c[1], false) : ''}` }];
		case 'power':
			return [{ t: `y = ${term(c[0], true)}x` }, { t: term(c[1], true), sup: true }];
	}
	return [];
}

/** Équation en texte simple (exposants entre ^( )) : info-bulles, essais. */
export const equationText = (parts: EqPart[]) => parts.map((p) => (p.sup ? `^(${p.t})` : p.t)).join('');

/** Coefficient de détermination, 4 décimales comme un tableur. */
export const r2Text = (r2: number, num: (v: number, digits: number) => string) => `R² = ${num(r2, 4)}`;

/**
 * Barre d'erreur d'un point : de lo à hi (valeurs), ou null. center : barres
 * centrées sur la moyenne (écart type), sinon sur la valeur du point.
 */
export function errorSpan(e: ErrorValues | null | undefined, i: number, v: number | null | undefined): { lo: number; hi: number; plus: boolean; minus: boolean } | null {
	if (!e || v === null || v === undefined) return null;
	const p = e.plus?.[i] ?? null;
	const m = e.minus?.[i] ?? null;
	if (p === null && m === null) return null;
	const c = e.center ?? v;
	return { lo: c - (m ?? 0), hi: c + (p ?? 0), plus: p !== null, minus: m !== null };
}

/**
 * Rayon des bulles : la plus grande taille (en valeur absolue) a le rayon
 * maxR, les autres une aire proportionnelle (comme un tableur) ; taille
 * nulle, négative ou vide : pas de bulle (0).
 */
export function bubbleRadii(sizes: (number | null)[], maxR: number): number[] {
	const top = Math.max(0, ...sizes.map((s) => (s !== null && s > 0 ? s : 0)));
	return sizes.map((s) => (s !== null && s > 0 && top > 0 ? maxR * Math.sqrt(s / top) : 0));
}

/** Une bougie (ouverture, haut, bas, clôture) ; open absent : haut-bas-clôture. */
export interface Candle {
	open: number | null;
	high: number;
	low: number;
	close: number;
	/** Clôture au-dessus de l'ouverture (bougie creuse, couleur de hausse). */
	up: boolean;
}

/**
 * Bougies d'un graphique boursier : séries {haut, bas, clôture} ou
 * {ouverture, haut, bas, clôture[, volume]} ; un jour sans haut ni bas : null.
 */
export function candles(series: { values: (number | null)[] }[]): (Candle | null)[] {
	const ohlc = series.length >= 4;
	const at = (k: number, i: number) => series[k]?.values[i] ?? null;
	const n = series[0]?.values.length ?? 0;
	const out: (Candle | null)[] = [];
	for (let i = 0; i < n; i++) {
		const open = ohlc ? at(0, i) : null;
		const o = ohlc ? 1 : 0;
		let high = at(o, i);
		let low = at(o + 1, i);
		const close = at(o + 2, i);
		if (close === null || (high === null && low === null)) {
			out.push(null);
			continue;
		}
		const vals = [high, low, close, open].filter((v): v is number => v !== null);
		high = Math.max(...vals);
		low = Math.min(...vals);
		out.push({ open, high, low, close, up: open === null ? true : close >= open });
	}
	return out;
}

/** Séries de prix d'un graphique boursier (sans le volume, la 5e). */
export const stockPrices = (n: number) => Math.min(n, 4);
