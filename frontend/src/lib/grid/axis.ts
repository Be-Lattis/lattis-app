// Un axe de la feuille (lignes ou colonnes) : un nombre de cases, une taille
// par défaut, et des tailles particulières (colonnes d'un tableau, hauteurs
// choisies, hauteurs automatiques des lignes qui renvoient à la ligne…).
//
// Les tailles particulières sont rangées triées, avec la somme cumulée de
// leurs écarts à la taille par défaut : position et case sous une position
// se calculent par dichotomie (O(log K) pour K tailles particulières), ce
// qui reste rapide quand les hauteurs automatiques de milliers de lignes
// chargées s'ajoutent. with() donne un nouvel axe avec quelques tailles
// changées, sans rien remesurer.

export class Axis {
	private keys: number[];
	// cum[j] : somme des écarts (taille - défaut) des j premières cases particulières.
	private cum: Float64Array;

	constructor(
		public readonly count: number,
		public readonly def: number,
		private sizes: Map<number, number> = new Map()
	) {
		this.keys = [...sizes.keys()].sort((a, b) => a - b);
		this.cum = new Float64Array(this.keys.length + 1);
		for (let j = 0; j < this.keys.length; j++) this.cum[j + 1] = this.cum[j] + this.size(this.keys[j]) - def;
	}

	size(i: number): number {
		return this.sizes.get(i) ?? this.def;
	}

	/** Nombre de cases particulières d'indice < i. */
	private below(i: number): number {
		let lo = 0;
		let hi = this.keys.length;
		while (lo < hi) {
			const m = (lo + hi) >> 1;
			if (this.keys[m] < i) lo = m + 1;
			else hi = m;
		}
		return lo;
	}

	/** Position du début de la case i. */
	offset(i: number): number {
		return i * this.def + this.cum[this.below(i)];
	}

	/** Longueur totale de l'axe. */
	get total(): number {
		return this.offset(this.count);
	}

	/** Case qui contient la position px (bornée à l'axe ; une case de taille nulle n'en contient aucune). */
	indexAt(px: number): number {
		if (px < 0) px = 0;
		// Dernière case particulière qui commence avant px (ou à px).
		let lo = 0;
		let hi = this.keys.length;
		while (lo < hi) {
			const m = (lo + hi) >> 1;
			if (this.keys[m] * this.def + this.cum[m] <= px) lo = m + 1;
			else hi = m;
		}
		let base = 0;
		let idx = 0;
		if (lo > 0) {
			const k = this.keys[lo - 1];
			const start = k * this.def + this.cum[lo - 1];
			const w = this.size(k);
			if (px < start + w) return k;
			base = start + w;
			idx = k + 1;
		}
		return Math.min(this.count - 1, idx + Math.floor((px - base) / this.def));
	}

	/** Les tailles particulières (copie). */
	entries(): Map<number, number> {
		return new Map(this.sizes);
	}

	/** Nouvel axe où quelques cases changent de taille (undefined ou la taille par défaut : retour au défaut). */
	with(changes: Map<number, number | undefined>): Axis {
		if (!changes.size) return this;
		const sizes = new Map(this.sizes);
		for (const [i, s] of changes) {
			if (s === undefined || s === this.def) sizes.delete(i);
			else sizes.set(i, s);
		}
		return new Axis(this.count, this.def, sizes);
	}
}
