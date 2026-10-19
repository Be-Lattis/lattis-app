// Géométrie des graphiques « au-delà des classiques » : fréquences, boîtes à
// moustaches, cascade, compartimentage, radar, perspective (3D), carte.
// Fonctions pures, sans DOM : Chart.svelte et ses voisins les dessinent.

import { tickLabel, ticks } from './scale';

// --- Fréquences -----------------------------------------------------------

export interface Bins {
	/** Bornes des classes : edges[i] ≤ v < edges[i + 1] (la dernière inclut sa borne haute). */
	edges: number[];
	labels: string[];
	/** Effectif de chaque classe, pour chaque série. */
	counts: number[][];
}

/** Pas « rond » (1, 2, 2,5, 5 × 10^k) au moins égal à raw. */
export function niceStep(raw: number): number {
	if (!(raw > 0) || !isFinite(raw)) return 1;
	const mag = 10 ** Math.floor(Math.log10(raw));
	return Number(([1, 2, 2.5, 5, 10].map((m) => m * mag).find((s) => s >= raw * (1 - 1e-12)) ?? 10 * mag).toPrecision(12));
}

/**
 * Classes d'un histogramme de fréquences, communes à toutes les séries :
 * n classes (Sturges par défaut) de largeur ronde.
 */
export function histogramBins(series: (number | null)[][], n = 0): Bins {
	const all = series.flat().filter((v): v is number => v !== null && isFinite(v));
	if (!all.length) return { edges: [], labels: [], counts: series.map(() => []) };
	let lo = Math.min(...all);
	let hi = Math.max(...all);
	const k = n > 0 ? n : Math.min(50, Math.max(1, Math.ceil(Math.log2(all.length)) + 1));
	if (lo === hi) {
		lo -= 0.5;
		hi += 0.5;
	}
	const step = niceStep((hi - lo) / k);
	const start = Math.floor(lo / step + 1e-9) * step;
	// hi tombe sur une borne : il rejoint la dernière classe (incluse).
	const m = Math.max(1, Math.ceil((hi - start) / step - 1e-9));
	const edges = Array.from({ length: m + 1 }, (_, i) => Number((start + i * step).toPrecision(12)));
	const counts = series.map((s) => {
		const c = new Array(m).fill(0);
		for (const v of s) {
			if (v === null || !isFinite(v)) continue;
			let i = Math.floor((v - start) / step + 1e-9);
			if (i >= m) i = m - 1;
			if (i < 0) i = 0;
			c[i]++;
		}
		return c;
	});
	const labels = edges.slice(0, -1).map((e, i) => `${tickLabel(e)} – ${tickLabel(edges[i + 1])}`);
	return { edges, labels, counts };
}

// --- Boîtes à moustaches ----------------------------------------------------

export interface BoxStats {
	min: number;
	q1: number;
	median: number;
	q3: number;
	max: number;
	mean: number;
	/** Valeurs au-delà de 1,5 écart interquartile (au plus 100 de chaque côté si calculées par le classeur). */
	outliers: number[];
	n: number;
	/** Nombre de valeurs extrêmes (calcul du classeur : toutes ne sont pas listées). */
	out?: number;
}

/** Boîte telle que la donne le classeur (canvas.BoxStats : liste des extrêmes éventuellement nulle). */
export type BoxStatsIn = Omit<BoxStats, 'outliers'> & { outliers: number[] | null };

/** Quantile p (0 à 1) d'un tableau trié, interpolé comme QUARTILE.INCLURE. */
export function quantile(sorted: number[], p: number): number {
	if (!sorted.length) return NaN;
	const h = (sorted.length - 1) * p;
	const i = Math.floor(h);
	return sorted[i] + (h - i) * ((sorted[i + 1] ?? sorted[i]) - sorted[i]);
}

export function boxStats(values: (number | null)[]): BoxStats | null {
	const s = values.filter((v): v is number => v !== null && isFinite(v)).sort((a, b) => a - b);
	if (!s.length) return null;
	const q1 = quantile(s, 0.25);
	const q3 = quantile(s, 0.75);
	const iqr = q3 - q1;
	const lo = q1 - 1.5 * iqr;
	const hi = q3 + 1.5 * iqr;
	const inside = s.filter((v) => v >= lo && v <= hi);
	return {
		min: inside[0] ?? s[0],
		q1,
		median: quantile(s, 0.5),
		q3,
		max: inside[inside.length - 1] ?? s[s.length - 1],
		mean: s.reduce((a, v) => a + v, 0) / s.length,
		outliers: s.filter((v) => v < lo || v > hi),
		n: s.length
	};
}

/**
 * Boîtes d'un graphique : par catégorie (libellés répétés : une boîte par
 * libellé et par série), sinon une boîte par série.
 */
export function boxGroups(labels: string[], series: { name: string; values: (number | null)[] }[]): { labels: string[]; boxes: (BoxStats | null)[][] } {
	const groups = [...new Set(labels)];
	if (groups.length === labels.length || groups.length > 40) {
		return { labels: series.map((s) => s.name), boxes: series.map((s, k) => series.map((_, j) => (j === k ? boxStats(s.values) : null))) };
	}
	return {
		labels: groups,
		boxes: groups.map((g) => series.map((s) => boxStats(s.values.filter((_, i) => labels[i] === g))))
	};
}

/** Libellés des classes d'un histogramme de fréquences, d'après leurs bornes. */
export function binLabels(edges: number[], f: (v: number) => string = tickLabel): string[] {
	return edges.slice(0, -1).map((e, i) => `${f(e)} – ${f(edges[i + 1])}`);
}

/**
 * Boîtes entre deux états (animation) : chaque statistique va de
 * l'ancienne (même libellé, même série) à la nouvelle ; une boîte nouvelle
 * s'ouvre depuis sa médiane. Les valeurs extrêmes sont celles d'arrivée.
 */
export function lerpBoxes(from: (BoxStats | null)[][] | null, to: (BoxStats | null)[][], e: number, fromLabels?: string[], toLabels?: string[]): (BoxStats | null)[][] {
	if (e >= 1) return to;
	const at = new Map<string, number>();
	fromLabels?.forEach((l, i) => at.has(l) || at.set(l, i));
	const l = (a: number, b: number) => a + (b - a) * e;
	return to.map((row, i) => {
		const src = from?.[toLabels && fromLabels ? (at.get(toLabels[i]) ?? -1) : i];
		return row.map((b, k) => {
			if (!b) return null;
			const a = src?.[k] ?? { ...b, min: b.median, q1: b.median, q3: b.median, max: b.median, mean: b.median };
			return { ...b, min: l(a.min, b.min), q1: l(a.q1, b.q1), median: l(a.median, b.median), q3: l(a.q3, b.q3), max: l(a.max, b.max), mean: l(a.mean, b.mean) };
		});
	});
}

// --- Cascade ------------------------------------------------------------------

export interface Step {
	from: number;
	to: number;
	kind: 'up' | 'down' | 'total';
	value: number;
}

/** Marches d'une cascade : chaque valeur part du cumul précédent ; un total final. */
export function waterfallSteps(values: (number | null)[]): Step[] {
	let acc = 0;
	const out: Step[] = values.map((v) => {
		const d = v ?? 0;
		const s: Step = { from: acc, to: acc + d, kind: d < 0 ? 'down' : 'up', value: d };
		acc += d;
		return s;
	});
	out.push({ from: 0, to: acc, kind: 'total', value: acc });
	return out;
}

// --- Compartimentage (treemap « squarified ») ------------------------------

export interface Tile {
	i: number;
	x: number;
	y: number;
	w: number;
	h: number;
}

/** Rectangles d'aires proportionnelles aux valeurs positives, aussi carrés que possible. */
export function squarify(values: (number | null)[], x: number, y: number, w: number, h: number): Tile[] {
	const items = values
		.map((v, i) => ({ i, v: Math.max(0, v ?? 0) }))
		.filter((it) => it.v > 0)
		.sort((a, b) => b.v - a.v);
	const total = items.reduce((a, it) => a + it.v, 0);
	if (!total || w <= 0 || h <= 0) return [];
	const scale = (w * h) / total;
	const areas = items.map((it) => ({ i: it.i, a: it.v * scale }));
	const out: Tile[] = [];
	let rect = { x, y, w, h };
	const worst = (row: number[], side: number) => {
		const s = row.reduce((a, b) => a + b, 0);
		const mx = Math.max(...row);
		const mn = Math.min(...row);
		return Math.max((side * side * mx) / (s * s), (s * s) / (side * side * mn));
	};
	let row: { i: number; a: number }[] = [];
	const layout = () => {
		const s = row.reduce((a, r) => a + r.a, 0);
		if (rect.w >= rect.h) {
			const cw = s / rect.h;
			let yy = rect.y;
			for (const r of row) {
				const hh = r.a / cw;
				out.push({ i: r.i, x: rect.x, y: yy, w: cw, h: hh });
				yy += hh;
			}
			rect = { x: rect.x + cw, y: rect.y, w: rect.w - cw, h: rect.h };
		} else {
			const rh = s / rect.w;
			let xx = rect.x;
			for (const r of row) {
				const ww = r.a / rh;
				out.push({ i: r.i, x: xx, y: rect.y, w: ww, h: rh });
				xx += ww;
			}
			rect = { x: rect.x, y: rect.y + rh, w: rect.w, h: rect.h - rh };
		}
		row = [];
	};
	for (const it of areas) {
		const side = Math.min(rect.w, rect.h);
		if (!row.length || worst([...row.map((r) => r.a), it.a], side) <= worst(row.map((r) => r.a), side)) row.push(it);
		else {
			layout();
			row.push(it);
		}
	}
	if (row.length) layout();
	return out;
}

// --- Radar, jauge, secteurs -------------------------------------------------

/** Point de la branche i (sur n, la première en haut) à la fraction f du rayon. */
export function radarPoint(cx: number, cy: number, r: number, i: number, n: number, f: number): { x: number; y: number } {
	const a = -Math.PI / 2 + (2 * Math.PI * i) / Math.max(1, n);
	return { x: cx + r * f * Math.cos(a), y: cy + r * f * Math.sin(a) };
}

/** Secteur d'anneau entre les angles a0 et a1 (radians, sens horaire à l'écran). */
export function ringPath(cx: number, cy: number, r0: number, r1: number, a0: number, a1: number): string {
	const p = (r: number, a: number) => `${cx + r * Math.cos(a)} ${cy + r * Math.sin(a)}`;
	const large = Math.abs(a1 - a0) > Math.PI ? 1 : 0;
	if (r0 <= 0) return `M ${cx} ${cy} L ${p(r1, a0)} A ${r1} ${r1} 0 ${large} 1 ${p(r1, a1)} Z`;
	return `M ${p(r1, a0)} A ${r1} ${r1} 0 ${large} 1 ${p(r1, a1)} L ${p(r0, a1)} A ${r0} ${r0} 0 ${large} 0 ${p(r0, a0)} Z`;
}

/** Jauge : valeur affichée (dernière de la série), bornes et part de l'arc. */
export function gaugeValue(values: (number | null)[], min?: number, max?: number): { value: number | null; lo: number; hi: number; frac: number } {
	let value: number | null = null;
	for (let i = values.length - 1; i >= 0; i--)
		if (values[i] !== null) {
			value = values[i];
			break;
		}
	const lo = min ?? 0;
	const t = ticks(lo, Math.max(value ?? 100, lo + 1));
	let hi = max ?? t[t.length - 1];
	if (hi <= lo) hi = lo + 1;
	const frac = value === null ? 0 : Math.min(1, Math.max(0, (value - lo) / (hi - lo)));
	return { value, lo, hi, frac };
}

// --- Perspective (3D) -------------------------------------------------------

/** Couleur #rrggbb éclaircie (f > 0) ou assombrie (f < 0). */
export function shade(hex: string, f: number): string {
	const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
	if (!m) return hex;
	const n = parseInt(m[1], 16);
	const ch = [(n >> 16) & 255, (n >> 8) & 255, n & 255].map((c) => Math.round(f >= 0 ? c + (255 - c) * f : c * (1 + f)));
	return '#' + ch.map((c) => c.toString(16).padStart(2, '0')).join('');
}

export interface PieSlice3D {
	i: number;
	from: number;
	to: number;
	part: number;
	/** Face du dessus (ellipse). */
	top: string;
	/** Parois visibles (devant), éventuellement deux morceaux. */
	walls: string[];
	/** Milieu de la face du dessus (étiquette). */
	mid: { x: number; y: number };
}

/**
 * Secteurs en perspective : disque vu de biais (ellipse rx × ry), épaisseur
 * depth. Seule la paroi de devant (angles 0 à π à l'écran) est visible ; les
 * parois de deux secteurs ne se recouvrent pas : on les dessine toutes, puis
 * les dessus.
 */
export function pie3d(values: (number | null)[], cx: number, cy: number, rx: number, ry: number, depth: number): PieSlice3D[] {
	const total = values.reduce<number>((a, v) => a + Math.max(0, v ?? 0), 0);
	if (!total) return [];
	const pt = (a: number, dy = 0) => `${cx + rx * Math.cos(a)} ${cy + ry * Math.sin(a) + dy}`;
	const out: PieSlice3D[] = [];
	let a = -Math.PI / 2;
	values.forEach((v, i) => {
		const part = Math.max(0, v ?? 0) / total;
		if (!part) return;
		const from = a;
		let to = a + part * 2 * Math.PI;
		a = to;
		if (to - from >= 2 * Math.PI - 1e-9) to = from + 2 * Math.PI - 1e-4;
		const large = to - from > Math.PI ? 1 : 0;
		const top = `M ${cx} ${cy} L ${pt(from)} A ${rx} ${ry} 0 ${large} 1 ${pt(to)} Z`;
		const walls: string[] = [];
		// Intersections avec les parties visibles [0, π] + 2kπ.
		for (const base of [-2 * Math.PI, 0, 2 * Math.PI]) {
			const s = Math.max(from, base);
			const e = Math.min(to, base + Math.PI);
			if (e - s > 1e-6) {
				const lg = e - s > Math.PI ? 1 : 0;
				walls.push(`M ${pt(s)} A ${rx} ${ry} 0 ${lg} 1 ${pt(e)} L ${pt(e, depth)} A ${rx} ${ry} 0 ${lg} 0 ${pt(s, depth)} Z`);
			}
		}
		const m = (from + to) / 2;
		out.push({ i, from, to, part, top, walls, mid: { x: cx + rx * 0.62 * Math.cos(m), y: cy + ry * 0.62 * Math.sin(m) } });
	});
	return out;
}

// --- Carte (Web Mercator, tuiles de 256 px) ---------------------------------

export const TILE = 256;

/** Position d'un point en pixels du monde au zoom z. */
export function project(lat: number, lon: number, z: number): { x: number; y: number } {
	const s = TILE * 2 ** z;
	const la = Math.max(-85.05112878, Math.min(85.05112878, lat));
	const sin = Math.sin((la * Math.PI) / 180);
	return { x: ((lon + 180) / 360) * s, y: (0.5 - Math.log((1 + sin) / (1 - sin)) / (4 * Math.PI)) * s };
}

export interface MapView {
	z: number;
	/** Coin haut gauche de la vue, en pixels du monde. */
	x0: number;
	y0: number;
}

/** Vue qui montre tous les points (zoom entier le plus fort, marge comprise). */
export function fitView(pts: { lat: number; lon: number }[], w: number, h: number, margin = 24, maxZoom = 16): MapView {
	if (!pts.length) {
		const c = project(46.6, 2.4, 5); // France entière
		return { z: 5, x0: c.x - w / 2, y0: c.y - h / 2 };
	}
	let z = maxZoom;
	for (; z > 0; z--) {
		const ps = pts.map((p) => project(p.lat, p.lon, z));
		const xs = ps.map((p) => p.x);
		const ys = ps.map((p) => p.y);
		if (Math.max(...xs) - Math.min(...xs) <= Math.max(1, w - 2 * margin) && Math.max(...ys) - Math.min(...ys) <= Math.max(1, h - 2 * margin)) break;
	}
	if (pts.length === 1) z = Math.min(z, 12);
	const ps = pts.map((p) => project(p.lat, p.lon, z));
	const cx = (Math.min(...ps.map((p) => p.x)) + Math.max(...ps.map((p) => p.x))) / 2;
	const cy = (Math.min(...ps.map((p) => p.y)) + Math.max(...ps.map((p) => p.y))) / 2;
	return { z, x0: cx - w / 2, y0: cy - h / 2 };
}

/** Niveau des tuiles OpenStreetMap le plus fort. */
export const TILE_MAX_ZOOM = 19;

/** Niveau de tuiles pour un zoom fractionnaire : le plus proche (tuiles affichées entre 181 et 362 px). */
export function tileZoom(z: number): number {
	return Math.max(0, Math.min(TILE_MAX_ZOOM, Math.round(z)));
}

export interface ViewTile {
	z: number;
	/** Colonne repliée autour du globe (celle qu'on demande). */
	x: number;
	y: number;
	/** Colonne non repliée (position dans la vue). */
	u: number;
	left: number;
	top: number;
	/** Côté à l'écran (256 px au zoom entier). */
	size: number;
}

/**
 * Tuiles qui couvrent la vue : position à l'écran et coordonnées (x replié
 * autour du globe). Zoom fractionnaire : tuiles du niveau le plus proche,
 * agrandies ou réduites.
 */
export function viewTiles(v: MapView, w: number, h: number): ViewTile[] {
	const tz = tileZoom(v.z);
	const size = TILE * 2 ** (v.z - tz);
	const n = 2 ** tz;
	const out: ViewTile[] = [];
	for (let ty = Math.floor(v.y0 / size); ty * size < v.y0 + h; ty++) {
		if (ty < 0 || ty >= n) continue;
		for (let tx = Math.floor(v.x0 / size); tx * size < v.x0 + w; tx++) {
			out.push({ z: tz, x: ((tx % n) + n) % n, y: ty, u: tx, left: tx * size - v.x0, top: ty * size - v.y0, size });
		}
	}
	return out;
}

/**
 * Densité : grille de cellules de cell px, chaque point pèse w et se répand
 * sur un rayon (noyau quadratique). Renvoie la valeur de chaque cellule et
 * le maximum.
 */
export function heatGrid(pts: { x: number; y: number; w: number }[], width: number, height: number, cell = 6, radius = 24): { cols: number; rows: number; values: Float64Array; max: number } {
	const cols = Math.max(1, Math.ceil(width / cell));
	const rows = Math.max(1, Math.ceil(height / cell));
	const values = new Float64Array(cols * rows);
	const rc = Math.ceil(radius / cell);
	for (const p of pts) {
		const ci = Math.floor(p.x / cell);
		const ri = Math.floor(p.y / cell);
		for (let r = Math.max(0, ri - rc); r <= Math.min(rows - 1, ri + rc); r++) {
			for (let c = Math.max(0, ci - rc); c <= Math.min(cols - 1, ci + rc); c++) {
				const dx = (c + 0.5) * cell - p.x;
				const dy = (r + 0.5) * cell - p.y;
				const d2 = (dx * dx + dy * dy) / (radius * radius);
				if (d2 < 1) values[r * cols + c] += p.w * (1 - d2) * (1 - d2);
			}
		}
	}
	let max = 0;
	for (const v of values) if (v > max) max = v;
	return { cols, rows, values, max };
}

/** Couleur de densité (0 à 1) : bleu, vert, jaune, rouge. */
export function heatColor(t: number): string {
	const stops: [number, number, number][] = [
		[49, 130, 189],
		[49, 163, 84],
		[255, 204, 0],
		[227, 26, 28]
	];
	const x = Math.max(0, Math.min(1, t)) * (stops.length - 1);
	const i = Math.min(stops.length - 2, Math.floor(x));
	const f = x - i;
	const c = stops[i].map((a, k) => Math.round(a + (stops[i + 1][k] - a) * f));
	return `rgb(${c[0]}, ${c[1]}, ${c[2]})`;
}
