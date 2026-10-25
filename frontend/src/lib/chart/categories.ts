// Couleurs par catégorie (carte « Couleur par ») : une couleur par valeur
// distincte d'une colonne, réparties sur tout le spectre.
//
// - Teintes OKLCH (perceptuellement régulières) également espacées : pour n
//   valeurs, deux teintes voisines sont à 360/n degrés, l'écart le plus grand
//   possible ; au-delà de 8 valeurs, la luminosité alterne d'une teinte à sa
//   voisine (sombre, claire) pour les départager encore.
// - Les rangs (valeurs triées) parcourent les teintes par un pas proche du
//   nombre d'or (≈ 0,382·n, premier avec n) : deux valeurs voisines dans le
//   tri, donc dans la légende, ont des couleurs éloignées.
// - L'attribution ne dépend que des valeurs triées (pas de l'ordre des
//   lignes) : trier ou filtrer le tableau ne mélange pas les couleurs.
// - Au-delà de MAX_COLORS valeurs, les plus fréquentes gardent une couleur
//   (MAX_COLORS − 1), les autres sont grises (« Autres »).

import { t } from '#lib/i18n/index.js';

/** Valeurs colorées au plus (au-delà : « Autres », en gris). */
export const MAX_COLORS = 20;
/** Gris des valeurs regroupées (« Autres »). */
export const OTHER_COLOR = '#8a9099';
/** Gris clair des cellules vides. */
export const EMPTY_COLOR = '#c3c8ce';

/** Teinte OKLCH de la première couleur (rouge). */
const HUE0 = 29;

function gcd(a: number, b: number): number {
	while (b) [a, b] = [b, a % b];
	return a;
}

/** Pas de parcours des teintes : proche de n/φ², premier avec n. */
export function hueStride(n: number): number {
	if (n <= 3) return 1;
	const target = n * 0.381966;
	let best = 1;
	for (let s = 1; s < n; s++) if (gcd(s, n) === 1 && Math.abs(s - target) < Math.abs(best - target)) best = s;
	return best;
}

/** Emplacement de teinte (0 … n−1) du rang k parmi n valeurs. */
export function hueSlot(k: number, n: number): number {
	return (k * hueStride(n)) % n;
}

/** Teinte (degrés, OKLCH) du rang k parmi n valeurs. */
export function categoryHue(k: number, n: number): number {
	return (HUE0 + (hueSlot(k, n) * 360) / n) % 360;
}

const toSrgb = (x: number) => (x <= 0.0031308 ? 12.92 * x : 1.055 * Math.pow(x, 1 / 2.4) - 0.055);

/** OKLCH vers sRGB linéaire (composantes hors [0, 1] si hors gamut). */
function oklchLinear(L: number, C: number, h: number): [number, number, number] {
	const a = C * Math.cos((h * Math.PI) / 180);
	const b = C * Math.sin((h * Math.PI) / 180);
	const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3;
	const m = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3;
	const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3;
	return [4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s, -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s, -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s];
}

const inGamut = (c: number[]) => c.every((x) => x >= -1e-4 && x <= 1 + 1e-4);

/** Couleur #rrggbb d'une teinte OKLCH ; la saturation baisse jusqu'à tenir dans sRGB. */
export function oklchHex(L: number, C: number, h: number): string {
	let lin = oklchLinear(L, C, h);
	if (!inGamut(lin)) {
		let lo = 0;
		let hi = C;
		for (let i = 0; i < 20; i++) {
			const mid = (lo + hi) / 2;
			if (inGamut(oklchLinear(L, mid, h))) lo = mid;
			else hi = mid;
		}
		lin = oklchLinear(L, lo, h);
	}
	return '#' + lin.map((x) => Math.round(Math.max(0, Math.min(1, toSrgb(x))) * 255).toString(16).padStart(2, '0')).join('');
}

/** Couleurs de n valeurs, par rang (valeurs triées). */
export function categoryColors(n: number): string[] {
	const out: string[] = [];
	for (let k = 0; k < n; k++) {
		const slot = hueSlot(k, n);
		const h = (HUE0 + (slot * 360) / n) % 360;
		// Peu de valeurs : une luminosité moyenne, lisible sur le fond de carte.
		// Beaucoup : sombre et claire en alternance d'une teinte à la voisine.
		const [L, C] = n <= 8 ? [0.63, 0.19] : slot % 2 ? [0.79, 0.15] : [0.55, 0.19];
		out.push(oklchHex(L, C, h));
	}
	return out;
}

export interface LegendEntry {
	label: string;
	color: string;
	count: number;
}

export interface CategoryColoring {
	/** Couleur de chaque valeur (rang dans categories). */
	colors: string[];
	/** Légende : valeurs colorées dans l'ordre du tri, puis « Autres » et « (vide) ». */
	legend: LegendEntry[];
}

/**
 * Couleurs d'une carte colorée : categories (valeurs distinctes triées) et
 * groups (rang de la valeur de chaque point, −1 : vide). Au-delà de max
 * valeurs, les max − 1 plus fréquentes (à égalité : les premières du tri)
 * gardent une couleur, les autres prennent OTHER_COLOR.
 */
export function colorCategories(categories: string[], groups: number[], max = MAX_COLORS): CategoryColoring {
	const n = categories.length;
	const counts = new Array<number>(n).fill(0);
	let empty = 0;
	for (const g of groups) {
		if (g >= 0 && g < n) counts[g]++;
		else empty++;
	}
	let kept = categories.map((_, i) => i);
	if (n > max) {
		const top = new Set([...kept].sort((a, b) => counts[b] - counts[a] || a - b).slice(0, max - 1));
		kept = kept.filter((i) => top.has(i));
	}
	const palette = categoryColors(kept.length);
	const colors = new Array<string>(n).fill(OTHER_COLOR);
	kept.forEach((i, k) => (colors[i] = palette[k]));
	const legend: LegendEntry[] = kept.map((i) => ({ label: categories[i], color: colors[i], count: counts[i] }));
	const keptSet = new Set(kept);
	const others = counts.reduce((s, c, i) => (keptSet.has(i) ? s : s + c), 0);
	if (n > kept.length) legend.push({ label: t('chart.legend.others', { n: n - kept.length }), color: OTHER_COLOR, count: others });
	if (empty) legend.push({ label: t('chart.legend.empty'), color: EMPTY_COLOR, count: empty });
	return { colors, legend };
}

/** Couleur d'un point d'après son groupe (−1 ou absent : vide). */
export function groupColor(c: CategoryColoring, g: number | undefined): string {
	return g === undefined || g < 0 || g >= c.colors.length ? EMPTY_COLOR : c.colors[g];
}

/** Distance perceptuelle (OKLab) entre deux couleurs #rrggbb. */
export function colorDistance(a: string, b: string): number {
	const lab = (hex: string) => {
		const [r, g, bl] = [1, 3, 5].map((i) => {
			const x = parseInt(hex.slice(i, i + 2), 16) / 255;
			return x <= 0.04045 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4;
		});
		const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * bl);
		const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * bl);
		const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * bl);
		return [0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s, 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s, 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s];
	};
	const p = lab(a);
	const q = lab(b);
	return Math.hypot(p[0] - q[0], p[1] - q[1], p[2] - q[2]);
}
