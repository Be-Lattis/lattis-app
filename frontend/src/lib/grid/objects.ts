// Images, formes et zones de texte posées sur une feuille
// (internal/canvas/objects.go) : ancrées à une cellule (coin haut gauche),
// avec un décalage et une taille en pixels, comme les graphiques. Ce module
// garde la géométrie (boîte, ancrage, poignées, proportions, extrémités
// des lignes), sans Svelte, pour les essais.

/** Genre d'objet. */
export type ObjectKind = 'image' | 'rect' | 'roundRect' | 'ellipse' | 'line' | 'arrow' | 'triangle' | 'text';

/** Formes du menu Insertion › Formes, dans l'ordre. */
export const SHAPES: ObjectKind[] = ['rect', 'roundRect', 'ellipse', 'triangle', 'line', 'arrow'];

export interface ObjectConfig {
	kind: ObjectKind;
	name?: string;
	/** Empreinte de l'image (_lattis_images) ; alt : texte de remplacement. */
	image?: string;
	alt?: string;
	offX?: number;
	offY?: number;
	width: number;
	height: number;
	/** Rang de premier plan (le plus grand devant). */
	z?: number;
	/** Couleurs « #rrggbb » du fond et du contour ; absentes : aucun. */
	fill?: string;
	line?: string;
	lineWidth?: number;
	/** Transparence du fond, de 0 à 100 %. */
	transparency?: number;
	/** Sens d'une ligne ou d'une flèche (par défaut du coin haut gauche au coin bas droit). */
	flipH?: boolean;
	flipV?: boolean;
	text?: string;
	bold?: boolean;
	italic?: boolean;
	/** Taille du texte (points ; absente : 11). */
	size?: number;
	color?: string;
	align?: 'left' | 'center' | 'right';
	valign?: 'top' | 'middle' | 'bottom';
}

export interface SheetObject {
	id: number;
	row: number;
	col: number;
	config: ObjectConfig;
}

export interface Box {
	x: number;
	y: number;
	w: number;
	h: number;
}

/** Géométrie de la feuille (pixels de la feuille, sans zoom). */
export interface Geometry {
	colOffset(col: number): number;
	rowTop(row: number): number;
	colAt(x: number): number;
	rowAt(y: number): number;
}

export const isLine = (k: ObjectKind) => k === 'line' || k === 'arrow';
/** Un texte se tape dans les formes pleines et les zones de texte. */
export const hasText = (k: ObjectKind) => k !== 'image' && !isLine(k);

/** Plus petit côté d'une forme (px). */
export const MIN_SIDE = 8;

/** Boîte d'un objet, en pixels de la feuille. */
export function objectBox(o: SheetObject, geo: Geometry): Box {
	return {
		x: geo.colOffset(o.col) + (o.config.offX ?? 0),
		y: geo.rowTop(o.row) + (o.config.offY ?? 0),
		w: o.config.width,
		h: o.config.height
	};
}

/** Ancrage d'une boîte : la cellule sous son coin haut gauche, et le décalage dans la cellule. */
export function anchorOf(b: Box, geo: Geometry): { row: number; col: number; offX: number; offY: number } {
	const x = Math.max(0, b.x);
	const y = Math.max(0, b.y);
	const col = Math.max(0, geo.colAt(x));
	const row = Math.max(0, geo.rowAt(y));
	return { row, col, offX: Math.round(x - geo.colOffset(col)), offY: Math.round(y - geo.rowTop(row)) };
}

/** Une poignée : bord ou coin (n, s, e, w, ne, nw, se, sw). */
export type Handle = 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw';
export const HANDLES: Handle[] = ['n', 's', 'e', 'w', 'ne', 'nw', 'se', 'sw'];

/**
 * Boîte après avoir tiré la poignée dir de (dx, dy) depuis b0. ratio : les
 * proportions de b0 sont gardées (coins seulement, comme un tableur : les
 * bords étirent). Le bord opposé reste en place ; la boîte ne sort pas de
 * la feuille et garde min pixels de côté.
 */
export function resizeBox(b0: Box, dir: Handle, dx: number, dy: number, ratio = false, min = MIN_SIDE): Box {
	let x0 = b0.x;
	let y0 = b0.y;
	let x1 = b0.x + b0.w;
	let y1 = b0.y + b0.h;
	if (dir.includes('e')) x1 = Math.max(x0 + min, x1 + dx);
	if (dir.includes('w')) x0 = Math.max(0, Math.min(x1 - min, x0 + dx));
	if (dir.includes('s')) y1 = Math.max(y0 + min, y1 + dy);
	if (dir.includes('n')) y0 = Math.max(0, Math.min(y1 - min, y0 + dy));
	let w = x1 - x0;
	let h = y1 - y0;
	if (ratio && dir.length === 2 && b0.w > 0 && b0.h > 0) {
		// Le côté le plus tiré mène, l'autre suit les proportions.
		const k = Math.max(w / b0.w, h / b0.h);
		w = Math.max(min, b0.w * k);
		h = Math.max(min, b0.h * k);
		if (dir.includes('w')) x0 = Math.max(0, x1 - w);
		if (dir.includes('n')) y0 = Math.max(0, y1 - h);
		w = dir.includes('w') ? x1 - x0 : w;
		h = dir.includes('n') ? y1 - y0 : h;
	}
	return { x: Math.round(x0), y: Math.round(y0), w: Math.round(w), h: Math.round(h) };
}

/** Boîte déplacée de (dx, dy), sans sortir de la feuille. */
export function moveBox(b0: Box, dx: number, dy: number): Box {
	return { ...b0, x: Math.max(0, Math.round(b0.x + dx)), y: Math.max(0, Math.round(b0.y + dy)) };
}

/** Extrémités d'une ligne ou d'une flèche dans sa boîte (la pointe au second point). */
export function lineEnds(c: Pick<ObjectConfig, 'width' | 'height' | 'flipH' | 'flipV'>): { x1: number; y1: number; x2: number; y2: number } {
	return {
		x1: c.flipH ? c.width : 0,
		y1: c.flipV ? c.height : 0,
		x2: c.flipH ? 0 : c.width,
		y2: c.flipV ? 0 : c.height
	};
}

/**
 * Ligne tirée entre deux points de la feuille : sa boîte et son sens
 * (glisser une extrémité de l'autre côté retourne la ligne).
 */
export function lineBox(p1: { x: number; y: number }, p2: { x: number; y: number }): Box & { flipH: boolean; flipV: boolean } {
	const x1 = Math.max(0, Math.round(p1.x));
	const y1 = Math.max(0, Math.round(p1.y));
	const x2 = Math.max(0, Math.round(p2.x));
	const y2 = Math.max(0, Math.round(p2.y));
	return { x: Math.min(x1, x2), y: Math.min(y1, y2), w: Math.abs(x2 - x1), h: Math.abs(y2 - y1), flipH: x1 > x2, flipV: y1 > y2 };
}

/** Pointe d'une flèche (x1, y1) → (x2, y2) : triangle (points SVG), comme ShapeSVG côté Go. */
export function arrowHead(x1: number, y1: number, x2: number, y2: number, lw: number): string {
	const l = Math.hypot(x2 - x1, y2 - y1);
	if (!l) return '';
	const size = Math.max(8, lw * 4);
	const ux = (x2 - x1) / l;
	const uy = (y2 - y1) / l;
	const bx = x2 - ux * size;
	const by = y2 - uy * size;
	const px = (-uy * size) / 2;
	const py = (ux * size) / 2;
	const r = (v: number) => Math.round(v * 100) / 100;
	return `${r(x2)},${r(y2)} ${r(bx + px)},${r(by + py)} ${r(bx - px)},${r(by - py)}`;
}

/** Taille posée d'une image w × h : la sienne, réduite à maxW de large (proportions gardées). */
export function fitImage(w: number, h: number, maxW = 480): { width: number; height: number } {
	if (w <= 0 || h <= 0) return { width: 160, height: 120 };
	if (w > maxW) return { width: maxW, height: Math.max(4, Math.round((h * maxW) / w)) };
	return { width: Math.max(4, Math.round(w)), height: Math.max(4, Math.round(h)) };
}

/** Plus grand côté d'une image enregistrée : au-delà, elle est réduite à l'insertion. */
export const MAX_IMAGE_SIDE = 2000;

/** Taille d'une image réduite pour tenir dans maxSide de côté ; null : rien à réduire. */
export function downscaled(w: number, h: number, maxSide = MAX_IMAGE_SIDE): { width: number; height: number } | null {
	const m = Math.max(w, h);
	if (m <= maxSide) return null;
	const k = maxSide / m;
	return { width: Math.max(1, Math.round(w * k)), height: Math.max(1, Math.round(h * k)) };
}

/** Configuration d'une copie (Ctrl+D, coller) : décalée de step pixels, sans nom ni rang. */
export function copyOf(o: SheetObject, step = 12): { row: number; col: number; config: ObjectConfig } {
	const { z: _z, name: _n, ...config } = o.config;
	return { row: o.row, col: o.col, config: { ...config, offX: (config.offX ?? 0) + step, offY: (config.offY ?? 0) + step } };
}

/** Configuration neuve d'une forme ou d'une zone de texte (couleurs et taille : par défaut, côté Go). */
export function newShape(kind: ObjectKind): ObjectConfig {
	if (isLine(kind)) return { kind, width: 160, height: 0 };
	if (kind === 'text') return { kind, width: 160, height: 60 };
	return { kind, width: 120, height: 80 };
}

/** Opacité du fond (transparence en %). */
export const fillOpacity = (c: ObjectConfig) => 1 - Math.min(100, Math.max(0, c.transparency ?? 0)) / 100;
