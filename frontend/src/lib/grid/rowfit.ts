// Hauteur automatique des lignes, comme Excel : une ligne sans hauteur
// choisie grandit pour son contenu (texte renvoyé à la ligne, sauts de
// ligne Alt+Entrée, grande police), cellules libres comme cellules de
// tableau.
//
// Performances (tableaux d'un million de lignes) : rien n'est mesuré au
// chargement pour les tableaux. Les cellules libres (peu nombreuses) sont
// mesurées à chaque mise en page ; les lignes de tableau seulement quand
// elles sont affichées et que leur bloc est chargé (cache.ts), une fois,
// jusqu'à ce que leurs données changent (forget). Les hauteurs trouvées
// modifient l'axe des lignes par Axis.with (seulement les lignes changées).
// Une ligne mesurée est affichée, donc jamais au-dessus de la vue : la
// grandir ne fait pas sauter ce que l'on regarde.
//
// Seules les hauteurs choisies (glisser, « Hauteur de ligne… ») sont
// enregistrées dans le classeur ; les hauteurs automatiques se recalculent.

export class AutoRows {
	/** Hauteurs dues aux cellules libres (ligne → px), remesurées à chaque mise en page. */
	free = new Map<number, number>();
	/** Hauteurs dues aux lignes de tableau mesurées. */
	private table = new Map<number, number>();
	private measured = new Set<number>();

	/** Hauteur automatique d'une ligne (undefined : hauteur par défaut). */
	height(r: number): number | undefined {
		const a = this.free.get(r);
		const b = this.table.get(r);
		if (a === undefined) return b;
		return b === undefined ? a : Math.max(a, b);
	}

	/** Toutes les hauteurs automatiques connues. */
	all(): Map<number, number> {
		const out = new Map(this.table);
		for (const [r, h] of this.free) out.set(r, Math.max(h, out.get(r) ?? 0));
		return out;
	}

	/** Lignes à remesurer (données changées) ; leur hauteur actuelle reste jusque-là, sans clignoter. */
	forget(first: number, count: number) {
		if (count > this.measured.size) {
			for (const r of this.measured) if (r >= first && r < first + count) this.measured.delete(r);
		} else for (let r = first; r < first + count; r++) this.measured.delete(r);
	}

	/** Tout remesurer (largeurs de colonnes, mise en forme…) ; les lignes hors de keep (plus dans un tableau) perdent tout de suite leur hauteur. */
	forgetAll(keep?: (r: number) => boolean) {
		this.measured.clear();
		if (keep) for (const r of [...this.table.keys()]) if (!keep(r)) this.table.delete(r);
	}

	/** Oublie les hauteurs de tableau (changement de feuille). */
	clear() {
		this.measured.clear();
		this.table.clear();
	}

	/**
	 * Mesure les lignes [first, last] pas encore mesurées. measure(r) : hauteur
	 * nécessaire (0 : la hauteur par défaut suffit), undefined si la ligne
	 * n'est pas encore chargée (elle sera mesurée plus tard). Rend les lignes
	 * dont la hauteur automatique change, avec leur nouvelle hauteur.
	 */
	measure(first: number, last: number, measure: (r: number) => number | undefined, max = 1000): Map<number, number | undefined> {
		const changed = new Map<number, number | undefined>();
		for (let r = first; r <= last && r - first < max; r++) {
			if (this.measured.has(r)) continue;
			const h = measure(r);
			if (h === undefined) continue;
			this.measured.add(r);
			const before = this.height(r);
			if (h > 0) this.table.set(r, h);
			else this.table.delete(r);
			const after = this.height(r);
			if (after !== before) changed.set(r, after);
		}
		return changed;
	}
}
