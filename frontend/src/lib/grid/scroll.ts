// Défilement virtuel vertical.
//
// 1 M de lignes de 24 px font 24 M px, au-delà de ce que certains moteurs
// acceptent comme hauteur d'élément. La zone de défilement native est donc
// plafonnée à MAX_SCROLL px et sa position est mise à l'échelle vers la
// position virtuelle. La molette, elle, est gérée en pixels virtuels pour
// garder un défilement fin quelle que soit la taille du tableau.

export const MAX_SCROLL = 8_000_000;

export class VScroll {
	/** Position virtuelle, en px, du haut de la zone visible. */
	top = 0;

	constructor(
		public content: number,
		public view: number
	) {}

	/** Hauteur de l'élément espaceur de la zone de défilement native. */
	get spacer(): number {
		return Math.min(this.content, MAX_SCROLL);
	}

	get maxTop(): number {
		return Math.max(0, this.content - this.view);
	}

	private get ratio(): number {
		const native = this.spacer - this.view;
		return native > 0 ? this.maxTop / native : 1;
	}

	/** Position virtuelle depuis la position native (barre de défilement). */
	fromNative(scrollTop: number) {
		this.top = clamp(scrollTop * this.ratio, 0, this.maxTop);
	}

	/** Position native correspondant à la position virtuelle. */
	toNative(): number {
		return this.top / this.ratio;
	}

	/** Vrai si la position native correspond déjà à la position virtuelle (à un pixel natif près). */
	inSync(scrollTop: number): boolean {
		return Math.abs(scrollTop * this.ratio - this.top) <= this.ratio + 0.5;
	}

	by(delta: number) {
		this.top = clamp(this.top + delta, 0, this.maxTop);
	}

	to(top: number) {
		this.top = clamp(top, 0, this.maxTop);
	}
}

export function clamp(v: number, lo: number, hi: number): number {
	return Math.min(hi, Math.max(lo, v));
}
