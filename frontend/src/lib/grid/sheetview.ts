// Position de chaque feuille (cellule active, sélection, défilement), comme
// Excel : retrouvée en revenant sur l'onglet ; A1 pour une feuille pas encore
// visitée.

export interface SheetView {
	active: { row: number; col: number };
	anchor: { row: number; col: number };
	top: number; // défilement vertical (px virtuels)
	left: number; // défilement horizontal (px)
}

export const HOME: SheetView = { active: { row: 0, col: 0 }, anchor: { row: 0, col: 0 }, top: 0, left: 0 };

export class SheetViews {
	private views = new Map<number, SheetView>();
	private current: number | undefined;

	/**
	 * Passage à la feuille sheet : garde la vue de la feuille quittée (now),
	 * renvoie celle de la nouvelle, ou null si la feuille ne change pas.
	 */
	switchTo(sheet: number, now: SheetView): SheetView | null {
		if (this.current === sheet) return null;
		const first = this.current === undefined;
		if (!first) this.views.set(this.current!, structuredClone(now));
		this.current = sheet;
		// À l'ouverture : la vue telle qu'elle est (A1 au départ).
		if (first) return null;
		return structuredClone(this.views.get(sheet) ?? HOME);
	}

	/** Oublie une feuille (supprimée). */
	forget(sheet: number) {
		this.views.delete(sheet);
	}
}
