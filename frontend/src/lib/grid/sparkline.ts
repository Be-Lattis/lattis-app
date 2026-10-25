// Graphiques sparkline : géométrie d'un petit graphique dans une cellule
// (courbe, histogramme, positif/négatif), dessin sur le canevas de la grille
// et en SVG (impression). Les groupes viennent du classeur
// (canvas/sparkline.go, Layout.sparklines) ; un groupe tiré d'un tableau est
// lu dans les lignes affichées (cache de la grille), rien d'autre.

export type SparkKind = 'line' | 'column' | 'winloss';

/** Configuration d'un groupe (canvas.SparklineConfig). */
export interface SparklineConfig {
	kind: string;
	table?: string;
	cols?: string[] | null;
	at?: string;
	source?: string;
	location?: string;
	high?: boolean;
	low?: boolean;
	first?: boolean;
	last?: boolean;
	negative?: boolean;
	markers?: boolean;
	color?: string;
	negColor?: string;
	markerColor?: string;
	highColor?: string;
	lowColor?: string;
	firstColor?: string;
	lastColor?: string;
	axis?: boolean;
	same?: boolean;
	min?: number | null;
	max?: number | null;
	weight?: number;
}

/** Un groupe posé sur la feuille (canvas.SparklineInfo). */
export interface SparklineGroup {
	id: number;
	config: SparklineConfig;
	cells?: { row: number; col: number; values: (number | null)[] | null }[] | null;
	src?: number[] | null;
	atCol: number;
	lo?: number | null;
	hi?: number | null;
	error?: string;
}

/** Couleurs par défaut : série, négatifs, marqueurs, haut, bas, premier, dernier. */
export const SPARK_COLORS = {
	color: '#376092',
	negColor: '#d00000',
	markerColor: '#376092',
	highColor: '#1e8e3e',
	lowColor: '#d00000',
	firstColor: '#e07000',
	lastColor: '#e07000'
} as const;

export const colorOf = (c: SparklineConfig, k: keyof typeof SPARK_COLORS) => c[k] || SPARK_COLORS[k];

export interface SparkPoint {
	x: number;
	y: number;
	color: string;
}
export interface SparkBar {
	x: number;
	y: number;
	w: number;
	h: number;
	color: string;
}
/** Ce qu'il faut dessiner dans une cellule de w × h (coordonnées dans la cellule). */
export interface SparkShape {
	/** Courbe : morceaux (coupés aux cellules vides). */
	lines: { x: number; y: number }[][];
	bars: SparkBar[];
	points: SparkPoint[];
	/** Axe horizontal (y), quand zéro est dans l'échelle. */
	axis: number | null;
	color: string;
	weight: number;
}

const PAD = 3;

/** Valeur numérique d'une cellule de tableau (nombre, ou texte d'un nombre). */
export function sparkValue(v: unknown): number | null {
	if (typeof v === 'number') return isFinite(v) ? v : null;
	if (typeof v === 'string' && v.trim() !== '') {
		const n = Number(v);
		return isFinite(n) ? n : null;
	}
	return null;
}

/**
 * Géométrie d'un graphique sparkline. scale : échelle commune du groupe
 * (sinon celle des valeurs) ; min et max de la configuration l'emportent.
 * Histogramme : barres depuis zéro ; positif/négatif : demi-hauteur au-dessus
 * ou au-dessous du milieu.
 */
export function sparkGeometry(values: (number | null)[], cfg: SparklineConfig, w: number, h: number, scale?: { lo?: number | null; hi?: number | null }): SparkShape {
	const kind = (cfg.kind === 'column' || cfg.kind === 'winloss' ? cfg.kind : 'line') as SparkKind;
	const out: SparkShape = { lines: [], bars: [], points: [], axis: null, color: colorOf(cfg, 'color'), weight: cfg.weight || 1.5 };
	const n = values.length;
	const idx = values.map((v, i) => (v === null ? -1 : i)).filter((i) => i >= 0);
	if (!n || !idx.length || w <= 2 * PAD || h <= 2 * PAD) return out;
	const vals = idx.map((i) => values[i]!);
	let lo = scale?.lo ?? Math.min(...vals);
	let hi = scale?.hi ?? Math.max(...vals);
	if (cfg.min !== null && cfg.min !== undefined) lo = cfg.min;
	if (cfg.max !== null && cfg.max !== undefined) hi = cfg.max;
	if (kind === 'column') {
		lo = Math.min(lo, 0);
		hi = Math.max(hi, 0);
	}
	const top = PAD;
	const bottom = h - PAD;
	const y = (v: number) => (hi === lo ? (top + bottom) / 2 : bottom - ((Math.min(Math.max(v, lo), hi) - lo) / (hi - lo)) * (bottom - top));
	// Points mis en valeur : haut et bas (tous ceux qui l'atteignent), premier, dernier.
	const max = Math.max(...vals);
	const min = Math.min(...vals);
	const first = idx[0];
	const last = idx[idx.length - 1];
	const mark = (i: number, v: number): string | null => {
		if (cfg.high && v === max) return colorOf(cfg, 'highColor');
		if (cfg.low && v === min) return colorOf(cfg, 'lowColor');
		if (cfg.first && i === first) return colorOf(cfg, 'firstColor');
		if (cfg.last && i === last) return colorOf(cfg, 'lastColor');
		if (cfg.negative && v < 0) return colorOf(cfg, 'negColor');
		return null;
	};
	if (kind === 'line') {
		const x = (i: number) => (n === 1 ? w / 2 : PAD + (i * (w - 2 * PAD)) / (n - 1));
		let run: { x: number; y: number }[] = [];
		values.forEach((v, i) => {
			if (v === null) {
				if (run.length) out.lines.push(run);
				run = [];
				return;
			}
			run.push({ x: x(i), y: y(v) });
		});
		if (run.length) out.lines.push(run);
		for (const i of idx) {
			const v = values[i]!;
			const c = mark(i, v) ?? (cfg.markers ? colorOf(cfg, 'markerColor') : null);
			if (c) out.points.push({ x: x(i), y: y(v), color: c });
		}
		if (cfg.axis && lo < 0 && hi > 0) out.axis = y(0);
		return out;
	}
	const band = (w - 2 * PAD) / n;
	const bw = Math.max(1, band * 0.75);
	if (kind === 'column') {
		const base = y(0);
		for (const i of idx) {
			const v = values[i]!;
			const yv = y(v);
			const c = mark(i, v) ?? out.color;
			out.bars.push({ x: PAD + i * band + (band - bw) / 2, y: Math.min(yv, base), w: bw, h: Math.max(1, Math.abs(base - yv)), color: c });
		}
		if (cfg.axis && lo < 0) out.axis = base;
		return out;
	}
	// Positif/négatif : hauteur fixe, au-dessus ou au-dessous du milieu.
	const mid = h / 2;
	const half = Math.max(1, mid - PAD - 1);
	for (const i of idx) {
		const v = values[i]!;
		if (v === 0) continue;
		const c = mark(i, v) ?? (v < 0 ? colorOf(cfg, 'negColor') : out.color);
		out.bars.push({ x: PAD + i * band + (band - bw) / 2, y: v > 0 ? mid - half : mid, w: bw, h: half, color: c });
	}
	if (cfg.axis) out.axis = mid;
	return out;
}

/** Dessine un graphique sparkline sur le canevas, cellule de largeur w en (x, y). */
export function drawSparkline(ctx: CanvasRenderingContext2D, s: SparkShape, x: number, y: number, w: number) {
	if (s.axis !== null) {
		ctx.fillStyle = '#9aa0a6';
		ctx.fillRect(x + PAD, Math.round(y + s.axis), w - 2 * PAD, 1);
	}
	for (const b of s.bars) {
		ctx.fillStyle = b.color;
		ctx.fillRect(x + b.x, y + b.y, b.w, b.h);
	}
	if (s.lines.length) {
		ctx.strokeStyle = s.color;
		ctx.lineWidth = s.weight;
		ctx.lineJoin = 'round';
		ctx.lineCap = 'round';
		ctx.beginPath();
		for (const run of s.lines) {
			run.forEach((p, i) => (i ? ctx.lineTo(x + p.x, y + p.y) : ctx.moveTo(x + p.x, y + p.y)));
			if (run.length === 1) ctx.lineTo(x + run[0].x + 0.1, y + run[0].y);
		}
		ctx.stroke();
	}
	for (const p of s.points) {
		ctx.fillStyle = p.color;
		ctx.beginPath();
		ctx.arc(x + p.x, y + p.y, Math.max(1.8, s.weight + 0.6), 0, Math.PI * 2);
		ctx.fill();
	}
}

const r1 = (v: number) => Math.round(v * 10) / 10;

/** Le même graphique en SVG (impression, export), w × h pixels. */
export function sparkSvg(s: SparkShape, w: number, h: number): string {
	let b = `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">`;
	if (s.axis !== null) b += `<line x1="${PAD}" x2="${w - PAD}" y1="${r1(s.axis)}" y2="${r1(s.axis)}" stroke="#9aa0a6" stroke-width="1"/>`;
	for (const r of s.bars) b += `<rect x="${r1(r.x)}" y="${r1(r.y)}" width="${r1(r.w)}" height="${r1(r.h)}" fill="${r.color}"/>`;
	for (const run of s.lines) b += `<polyline points="${run.map((p) => `${r1(p.x)},${r1(p.y)}`).join(' ')}" fill="none" stroke="${s.color}" stroke-width="${s.weight}" stroke-linejoin="round" stroke-linecap="round"/>`;
	for (const p of s.points) b += `<circle cx="${r1(p.x)}" cy="${r1(p.y)}" r="${r1(Math.max(1.8, s.weight + 0.6))}" fill="${p.color}"/>`;
	return b + '</svg>';
}

/** Lettres d'une colonne (0 : A). */
function letters(c: number): string {
	let s = '';
	for (c += 1; c > 0; c = Math.floor((c - 1) / 26)) s = String.fromCharCode(65 + ((c - 1) % 26)) + s;
	return s;
}

/** Plage en A1 (une cellule seule sans « : »), lignes et colonnes de 0. */
export function a1(r0: number, c0: number, r1: number, c1: number): string {
	const a = `${letters(c0)}${r0 + 1}`;
	return r0 === r1 && c0 === c1 ? a : `${a}:${letters(c1)}${r1 + 1}`;
}

/**
 * Emplacement proposé pour une source de cellules libres (comme un
 * tableur le suggère) : la colonne juste à droite, une cellule par ligne ;
 * une seule colonne de source : la cellule dessous.
 */
export function defaultLocation(r0: number, c0: number, r1: number, c1: number): string {
	if (c0 === c1 && r1 > r0) return a1(r1 + 1, c0, r1 + 1, c0);
	return a1(r0, c1 + 1, r1, c1 + 1);
}

/** Cellule (r, c) d'un graphique d'un groupe de cellules libres, avec ses valeurs. */
export function freeSparkAt(groups: SparklineGroup[], r: number, c: number): { g: SparklineGroup; values: (number | null)[] } | null {
	for (const g of groups) for (const s of g.cells ?? []) if (s.row === r && s.col === c) return { g, values: s.values ?? [] };
	return null;
}
