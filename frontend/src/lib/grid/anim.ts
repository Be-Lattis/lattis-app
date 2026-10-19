// Animations de la grille (choix de Louis) : courtes et discrètes, comme
// l'Excel récent. Tout est calculé au dessin, à partir d'un instant de
// départ : rien n'est relu du classeur, et rien ne tourne au repos.
//
// - lignes ou colonnes insérées / supprimées : un axe décalé (ShiftAxis) où
//   la bande insérée grandit de 0 à sa taille, ou la bande supprimée se
//   referme ; tout ce qui suit glisse ;
// - tableaux déplacés, agrandis ou nés : décalage, cadre et opacité
//   interpolés (tableMotion) ;
// - barres de données : de l'ancienne valeur (ou de zéro) à la nouvelle ;
// - valeurs recalculées : un bref éclat qui s'estompe.

/** Durée des glissements et des cadres, en ms. */
export const ANIM_MS = 220;
/** Durée de l'éclat d'une valeur recalculée, en ms. */
export const FLASH_MS = 450;

/** Courbe ease-out (cubique) : départ vif, arrivée douce. */
export const ease = (k: number) => 1 - (1 - Math.min(1, Math.max(0, k))) ** 3;

/** Avancement (0..1) d'une animation commencée à t0. */
export const progress = (now: number, t0: number, ms = ANIM_MS) => Math.min(1, Math.max(0, (now - t0) / ms));

export const lerp = (a: number, b: number, k: number) => a + (b - a) * k;

/** Ce que le dessin attend d'un axe (voir axis.ts). */
export interface AxisLike {
	readonly count: number;
	offset(i: number): number;
	size(i: number): number;
	indexAt(px: number): number;
	readonly total: number;
}

/**
 * Axe pendant une insertion ou une suppression : les cases [i0, i1) de
 * l'axe d'arrivée sont réduites au facteur k (insertion : k va de 0 à 1),
 * et un vide de gap pixels précède la case i1 (suppression : il va de la
 * taille supprimée à 0). Le reste glisse d'autant.
 */
export class ShiftAxis implements AxisLike {
	private a: number;
	private len: number;
	readonly count: number;

	constructor(
		private base: AxisLike,
		private i0: number,
		private i1: number,
		private k: number,
		private gap: number
	) {
		this.count = base.count;
		this.a = base.offset(i0);
		this.len = base.offset(i1) - this.a;
	}

	/** Décalage des cases à partir de i1. */
	private get shift(): number {
		return (this.k - 1) * this.len + this.gap;
	}

	offset(i: number): number {
		if (i >= this.i1) return this.base.offset(i) + this.shift;
		if (i <= this.i0) return this.base.offset(i);
		return this.a + (this.base.offset(i) - this.a) * this.k;
	}

	size(i: number): number {
		const s = this.base.size(i);
		return i >= this.i0 && i < this.i1 ? s * this.k : s;
	}

	indexAt(px: number): number {
		if (px < this.a) return this.base.indexAt(px);
		const band = this.len * this.k;
		if (px < this.a + band) return Math.min(this.i1 - 1, this.base.indexAt(this.a + (px - this.a) / this.k));
		// Dans le vide d'une suppression : la case qui le suit.
		if (px < this.a + band + this.gap) return Math.min(this.count - 1, this.i1);
		return this.base.indexAt(px - this.shift);
	}

	get total(): number {
		return this.base.total + this.shift;
	}
}

/** Lignes ou colonnes insérées (n > 0) ou supprimées (n < 0) à partir de la case at. */
export interface LinesShift {
	at: number;
	n: number;
	/** Taille supprimée, en pixels (suppression). */
	px: number;
}

/** Axe animé d'une insertion ou d'une suppression, à l'avancement e (0..1, déjà adouci). */
export function shiftAxis(base: AxisLike, s: LinesShift, e: number): AxisLike {
	if (e >= 1) return base;
	if (s.n > 0) return new ShiftAxis(base, s.at, Math.min(base.count, s.at + s.n), e, 0);
	return new ShiftAxis(base, s.at, s.at, 1, s.px * (1 - e));
}

/** Géométrie d'un tableau à l'écran, en pixels de la feuille. */
export interface Geo {
	x: number;
	y: number;
	w: number;
	h: number;
}

/** Mouvement d'un tableau : d'où il part (géométrie dessinée avant la modification). */
export interface Motion {
	t0: number;
	from: Geo;
	/** Tableau qui vient de naître : il s'ouvre en fondu. */
	born?: boolean;
	/** Durée, en ms (ANIM_MS par défaut). */
	ms?: number;
	/** Cadre seul étiré ou rétracté (cellule active juste à côté) : le contenu
	 * du tableau ne bouge pas. */
	grow?: boolean;
}

/**
 * Mouvements des tableaux après une modification : ceux qui ont bougé ou
 * changé de taille partent de leur géométrie dessinée ; ceux qui n'existaient
 * pas naissent (sauf ceux déjà présents avant, mais hors de la vue).
 */
export function tableMotions(drawn: Map<number, Geo>, known: Set<number>, now: Map<number, Geo>, t0: number): Map<number, Motion> {
	const out = new Map<number, Motion>();
	for (const [id, g] of now) {
		const was = drawn.get(id);
		if (was) {
			if (Math.abs(was.x - g.x) > 0.5 || Math.abs(was.y - g.y) > 0.5 || Math.abs(was.w - g.w) > 0.5 || Math.abs(was.h - g.h) > 0.5) out.set(id, { t0, from: was });
		} else if (!known.has(id)) out.set(id, { t0, from: g, born: true });
	}
	return out;
}

/** Géométrie affichée d'un tableau en mouvement, et son opacité, à l'avancement e (adouci). */
export function motionAt(m: Motion, to: Geo, e: number, headH: number): Geo & { alpha: number } {
	if (m.born) return { x: to.x, y: to.y, w: to.w, h: lerp(Math.min(headH, to.h), to.h, e), alpha: e };
	return { x: lerp(m.from.x, to.x, e), y: lerp(m.from.y, to.y, e), w: lerp(m.from.w, to.w, e), h: lerp(m.from.h, to.h, e), alpha: 1 };
}

/** Barre de données affichée : ses bornes (fractions de la cellule). */
export interface BarSpan {
	a: number;
	b: number;
}

/** Barre de longueur nulle, posée sur l'axe de la barre b (là d'où elle pousse). */
export function zeroBar(b: { a: number; b: number; axis?: number; rev?: boolean }): BarSpan {
	const at = b.axis ?? (b.rev ? 1 : 0);
	return { a: at, b: at };
}

export const sameBar = (x: BarSpan, y: BarSpan) => Math.abs(x.a - y.a) < 1e-6 && Math.abs(x.b - y.b) < 1e-6;

export const lerpBar = (x: BarSpan, y: BarSpan, e: number): BarSpan => ({ a: lerp(x.a, y.a, e), b: lerp(x.b, y.b, e) });

/** Opacité de l'éclat d'une valeur recalculée (1 → 0), 0 une fois fini. */
export function flashAlpha(now: number, t0: number): number {
	const k = progress(now, t0, FLASH_MS);
	return k >= 1 ? 0 : 1 - ease(k);
}

/**
 * Séries d'un graphique entre deux états : chaque valeur va de l'ancienne
 * (ou de zéro pour une nouvelle) à la nouvelle ; une valeur vide le reste.
 */
export function lerpSeries<S extends { values: (number | null)[] }>(from: S[] | null, to: S[], e: number): S[] {
	if (e >= 1) return to;
	return to.map((s, k) => ({
		...s,
		values: s.values.map((v, i) => (v === null ? null : lerp(from?.[k]?.values[i] ?? 0, v, e)))
	}));
}

/** Mêmes valeurs (les graphiques ne s'animent que si leurs données changent). */
export function sameSeries(a: { values: (number | null)[] }[], b: { values: (number | null)[] }[]): boolean {
	return a.length === b.length && a.every((s, k) => s.values.length === b[k].values.length && s.values.every((v, i) => v === b[k].values[i]));
}

/** Mouvement réduit demandé par le système (prefers-reduced-motion). */
let reducedQuery: MediaQueryList | null | undefined;
export function reducedMotion(): boolean {
	if (reducedQuery === undefined) reducedQuery = typeof matchMedia === 'function' ? matchMedia('(prefers-reduced-motion: reduce)') : null;
	return !!reducedQuery?.matches;
}
