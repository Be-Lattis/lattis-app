// Dessin d'une icône du jeu sur un <canvas> (en-têtes de la grille). Seules
// les icônes faites de <path> s'y prêtent (Path2D lit leur attribut d).
import { icons, type IconName } from './icons.js';

export interface IconPath {
	d: string;
	/** « a » : trait d'accent ; « af » : aplat d'accent ; vide : trait normal. */
	kind: '' | 'a' | 'af';
	width?: number;
}

const parsed = new Map<IconName, IconPath[]>();

/** iconPaths lit les tracés d'une icône ; une icône avec d'autres éléments que <path> est refusée. */
export function iconPaths(name: IconName): IconPath[] {
	let out = parsed.get(name);
	if (out) return out;
	const svg: string = icons[name];
	out = [];
	for (const m of svg.matchAll(/<(\w+)([^>]*)\/>/g)) {
		if (m[1] !== 'path') throw new Error(`icône ${name} : <${m[1]}> ne se dessine pas sur le canvas`);
		const attr = (a: string) => m[2].match(new RegExp(`\\b${a}="([^"]*)"`))?.[1];
		const kind = attr('class');
		const width = attr('stroke-width');
		out.push({ d: attr('d') ?? '', kind: kind === 'a' || kind === 'af' ? kind : '', width: width ? Number(width) : undefined });
	}
	parsed.set(name, out);
	return out;
}

const paths2d = new Map<string, Path2D>();

/**
 * drawIcon dessine l'icône dans le carré (x, y, size). weight épaissit le
 * trait (1 : celui du jeu, 1,5 px à 24 px) pour les petites tailles.
 */
export function drawIcon(ctx: CanvasRenderingContext2D, name: IconName, x: number, y: number, size: number, color: string, accent = color, weight = 1) {
	ctx.save();
	ctx.translate(x, y);
	ctx.scale(size / 24, size / 24);
	ctx.lineCap = 'round';
	ctx.lineJoin = 'round';
	for (const p of iconPaths(name)) {
		let path = paths2d.get(p.d);
		if (!path) paths2d.set(p.d, (path = new Path2D(p.d)));
		if (p.kind === 'af') {
			ctx.fillStyle = accent;
			ctx.fill(path);
		} else {
			ctx.strokeStyle = p.kind === 'a' ? accent : color;
			ctx.lineWidth = (p.width ?? 1.5) * weight;
			ctx.stroke(path);
		}
	}
	ctx.restore();
}
