// Bordures des cellules : style de trait et couleur de chaque bord, comme un
// tableur. Un bord (bt, bb, bl, br du style) vaut true (trait fin, couleur
// bc ou automatique : les classeurs d'avant), ou « style couleur » :
// « double », « thick #c00000 », « dashed theme:accent1:-25 ».
//
// Dessin : les bords sont relevés pendant le dessin des cellules puis tracés
// ensemble, sur les lignes du quadrillage : deux cellules voisines partagent
// le même trait (le plus visible l'emporte), sans doublon.

export type LineStyle = 'thin' | 'medium' | 'thick' | 'double' | 'dotted' | 'dashed';

/** Styles proposés (ruban, Format de cellule), dans l'ordre des menus. */
export const LINE_STYLES: LineStyle[] = ['thin', 'medium', 'thick', 'double', 'dotted', 'dashed'];

export interface Edge {
	style: LineStyle;
	color?: string;
}

export type EdgeKey = 'bt' | 'bb' | 'bl' | 'br';
export const EDGE_KEYS: EdgeKey[] = ['bt', 'bb', 'bl', 'br'];

/** Bord lu dans un style (bc : couleur par défaut) ; null sans bord. */
export function parseEdge(v: unknown, bc?: string): Edge | null {
	if (!v) return null;
	if (typeof v !== 'string') return { style: 'thin', color: bc || undefined };
	const [s, c] = v.trim().split(/\s+/, 2);
	const style = (LINE_STYLES as string[]).includes(s) ? (s as LineStyle) : 'thin';
	return { style, color: c || bc || undefined };
}

/** Valeur enregistrée d'un bord : true pour un trait fin sans couleur (comme avant), sinon « style couleur ». */
export function edgeValue(e: Edge | null): true | string | null {
	if (!e) return null;
	if (e.style === 'thin' && !e.color) return true;
	return e.color ? `${e.style} ${e.color}` : e.style;
}

/** Épaisseur d'un trait en pixels (un double fait trois pixels : deux traits et l'intervalle). */
export function edgeWidth(style: LineStyle): number {
	return style === 'medium' ? 2 : style === 'thick' || style === 'double' ? 3 : 1;
}

/** Poids d'un bord partagé par deux cellules : le plus visible l'emporte. */
export function edgeWeight(e: Edge): number {
	return { thin: 1, dotted: 2, dashed: 3, medium: 4, double: 5, thick: 6 }[e.style];
}

/** Pointillés du trait (canevas). */
export function edgeDash(style: LineStyle): number[] {
	return style === 'dotted' ? [1, 2] : style === 'dashed' ? [4, 2] : [];
}

/** Bordure CSS d'un bord (aperçus du dialogue et du ruban). */
export function edgeCss(e: Edge | null, color: string): string {
	if (!e) return 'none';
	const kind = e.style === 'dotted' ? 'dotted' : e.style === 'dashed' ? 'dashed' : e.style === 'double' ? 'double' : 'solid';
	return `${edgeWidth(e.style)}px ${kind} ${color}`;
}

interface Seg {
	h: boolean; // horizontal
	at: number; // ligne du quadrillage (y ou x, au demi-pixel)
	from: number;
	to: number;
	edge: Edge;
}

/**
 * Relevé des bordures d'un dessin de la grille : add pour chaque cellule,
 * stroke une fois toutes les cellules dessinées.
 */
export class BorderSet {
	private segs = new Map<string, Seg>();

	get empty(): boolean {
		return this.segs.size === 0;
	}

	clear() {
		this.segs.clear();
	}

	/** Bords d'une cellule de style st, en (x, y) de taille w × h. */
	add(st: { bt?: unknown; bb?: unknown; bl?: unknown; br?: unknown; bc?: string }, x: number, y: number, w: number, h: number) {
		if (!st.bt && !st.bb && !st.bl && !st.br) return;
		if (w <= 0 || h <= 0) return; // ligne ou colonne masquée
		const X0 = Math.round(x) - 0.5;
		const X1 = Math.round(x + w) - 0.5;
		const Y0 = Math.round(y) - 0.5;
		const Y1 = Math.round(y + h) - 0.5;
		this.put(true, Y0, X0, X1, parseEdge(st.bt, st.bc));
		this.put(true, Y1, X0, X1, parseEdge(st.bb, st.bc));
		this.put(false, X0, Y0, Y1, parseEdge(st.bl, st.bc));
		this.put(false, X1, Y0, Y1, parseEdge(st.br, st.bc));
	}

	private put(h: boolean, at: number, from: number, to: number, edge: Edge | null) {
		if (!edge) return;
		const k = `${h ? 'h' : 'v'}${at},${from},${to}`;
		const old = this.segs.get(k);
		if (!old || edgeWeight(edge) >= edgeWeight(old.edge)) this.segs.set(k, { h, at, from, to, edge });
	}

	/**
	 * Trace les bords (resolve : couleur à dessiner d'une couleur de style,
	 * automatique pour undefined ; gap : couleur entre les deux traits d'un
	 * double), puis vide le relevé.
	 */
	stroke(ctx: CanvasRenderingContext2D, resolve: (c: string | undefined) => string, gap: string) {
		const groups = new Map<string, { edge: Edge; color: string; segs: Seg[] }>();
		for (const s of this.segs.values()) {
			const color = resolve(s.edge.color);
			const k = s.edge.style + '|' + color;
			let g = groups.get(k);
			if (!g) groups.set(k, (g = { edge: s.edge, color, segs: [] }));
			g.segs.push(s);
		}
		for (const g of groups.values()) {
			const style = g.edge.style;
			const line = (off: number, width: number, color: string, ext: number) => {
				ctx.strokeStyle = color;
				ctx.lineWidth = width;
				ctx.setLineDash(edgeDash(style));
				ctx.beginPath();
				for (const s of g.segs) {
					if (s.h) {
						ctx.moveTo(s.from - ext, s.at + off);
						ctx.lineTo(s.to + ext, s.at + off);
					} else {
						ctx.moveTo(s.at + off, s.from - ext);
						ctx.lineTo(s.at + off, s.to + ext);
					}
				}
				ctx.stroke();
			};
			if (style === 'double') {
				line(0, 1, gap, 1.5);
				line(-1, 1, g.color, 1.5);
				line(1, 1, g.color, 1.5);
			} else if (style === 'medium') line(0.5, 2, g.color, 1);
			else if (style === 'thick') line(0, 3, g.color, 1.5);
			else line(0, 1, g.color, 0.5);
		}
		ctx.setLineDash([]);
		this.segs.clear();
	}
}

/**
 * Correctif de bordure d'une commande du ruban (Bordures ▾) pour une plage
 * de r0..r1 × c0..c1, avec le stylo choisi (style et couleur du trait) :
 * des morceaux de plage, chacun avec son correctif, posés en une étape.
 */
export type BorderCmd = 'bottom' | 'top' | 'left' | 'right' | 'none' | 'all' | 'outline' | 'thickOutline' | 'inside';

export interface StylePart {
	r0: number;
	c0: number;
	r1: number;
	c1: number;
	patch: Record<string, unknown>;
}

export function borderParts(cmd: BorderCmd, pen: Edge, r0: number, c0: number, r1: number, c1: number): StylePart[] {
	const v = edgeValue(pen);
	const part = (a: number, b: number, c: number, d: number, patch: Record<string, unknown>): StylePart => ({ r0: a, c0: b, r1: c, c1: d, patch });
	switch (cmd) {
		case 'bottom':
			return [part(r1, c0, r1, c1, { bb: v })];
		case 'top':
			return [part(r0, c0, r0, c1, { bt: v })];
		case 'left':
			return [part(r0, c0, r1, c0, { bl: v })];
		case 'right':
			return [part(r0, c1, r1, c1, { br: v })];
		case 'none':
			return [part(r0, c0, r1, c1, { bt: null, bb: null, bl: null, br: null })];
		case 'all':
			return [part(r0, c0, r1, c1, { bt: v, bb: v, bl: v, br: v })];
		case 'inside': {
			const out: StylePart[] = [];
			if (r1 > r0) out.push(part(r0, c0, r1 - 1, c1, { bb: v }));
			if (c1 > c0) out.push(part(r0, c0, r1, c1 - 1, { br: v }));
			return out;
		}
		case 'outline':
		case 'thickOutline': {
			const e = cmd === 'thickOutline' ? edgeValue({ style: 'thick', color: pen.color }) : v;
			return [part(r0, c0, r0, c1, { bt: e }), part(r1, c0, r1, c1, { bb: e }), part(r0, c0, r1, c0, { bl: e }), part(r0, c1, r1, c1, { br: e })];
		}
	}
}
