// Un axe de la feuille (lignes ou colonnes) : un nombre de cases, une taille
// par défaut, et quelques tailles particulières (les colonnes d'un tableau).
// Les tailles particulières sont peu nombreuses : les calculs les parcourent.

export class Axis {
	private keys: number[];

	constructor(
		public readonly count: number,
		public readonly def: number,
		private sizes: Map<number, number> = new Map()
	) {
		this.keys = [...sizes.keys()].sort((a, b) => a - b);
	}

	size(i: number): number {
		return this.sizes.get(i) ?? this.def;
	}

	/** Position du début de la case i. */
	offset(i: number): number {
		let px = i * this.def;
		for (const k of this.keys) {
			if (k >= i) break;
			px += this.size(k) - this.def;
		}
		return px;
	}

	/** Longueur totale de l'axe. */
	get total(): number {
		return this.offset(this.count);
	}

	/** Case qui contient la position px (bornée à l'axe). */
	indexAt(px: number): number {
		if (px <= 0) return 0;
		let base = 0;
		let idx = 0;
		for (const k of this.keys) {
			const span = (k - idx) * this.def;
			if (px < base + span) return idx + Math.floor((px - base) / this.def);
			base += span;
			const w = this.size(k);
			if (px < base + w) return k;
			base += w;
			idx = k + 1;
		}
		return Math.min(this.count - 1, idx + Math.floor((px - base) / this.def));
	}
}
