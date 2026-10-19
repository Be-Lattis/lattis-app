// Rechercher / remplacer dans la sélection, comme Excel : une sélection de
// plusieurs cellules borne la recherche (suivant, précédent, remplacer,
// remplacer tout) ; une seule cellule (ou sa fusion) : toute la feuille.

export interface Zone {
	r0: number;
	c0: number;
	r1: number;
	c1: number;
}

export interface Hit {
	row: number;
	col: number;
}

/** Zone de recherche : la sélection si elle compte plusieurs cellules, sinon null (toute la feuille). */
export function findZone(sel: Zone, single: boolean): Zone | null {
	return single ? null : { r0: sel.r0, c0: sel.c0, r1: sel.r1, c1: sel.c1 };
}

/** La cellule est-elle dans la zone (null : toute la feuille) ? */
export function inZone(h: Hit, z: Zone | null): boolean {
	return !z || (h.row >= z.r0 && h.row <= z.r1 && h.col >= z.c0 && h.col <= z.c1);
}

/** Clé de cache d'une recherche : feuille, texte et zone. */
export function findKey(text: string, z: Zone | null, sheet = 0): string {
	return `${sheet}\u0000${text}` + (z ? `\u0000${z.r0},${z.c0},${z.r1},${z.c1}` : '');
}

/**
 * Cellule trouvée à repérer (ligne et colonne entières teintées) : dans la
 * sélection, la dernière trouvée si elle y est encore ; sur la feuille,
 * seulement tant que la cellule active est restée dessus. Rien pendant une
 * saisie.
 */
export function foundCell(at: Hit | null, zone: Zone | null, active: Hit, editing = false): Hit | null {
	if (!at || editing) return null;
	if (zone) return inZone(at, zone) ? at : null;
	return at.row === active.row && at.col === active.col ? at : null;
}

/**
 * Indice de l'occurrence suivante (ou précédente, back) après la cellule
 * from, dans l'ordre de lecture (ligne puis colonne), en repartant du début
 * (ou de la fin) ; from absent : la première (ou la dernière). -1 si aucune.
 */
export function nextHit(hits: readonly Hit[], from: Hit | null, back: boolean): number {
	if (!hits.length) return -1;
	if (!from) return back ? hits.length - 1 : 0;
	const { row, col } = from;
	const i = back
		? hits.findLastIndex((m) => m.row < row || (m.row === row && m.col < col))
		: hits.findIndex((m) => m.row > row || (m.row === row && m.col > col));
	return i >= 0 ? i : back ? hits.length - 1 : 0;
}

// --- Barre Rechercher / remplacer ------------------------------------------

/** Compteur affiché dans le champ : « 3 sur 12 » (« 5000+ » au-delà de la limite lue). */
export function hitCount(k: number, total: number, limit = 5000): string {
	return `${k + 1} sur ${total}${total >= limit ? '+' : ''}`;
}

/** Adresse A1 d'une zone (0-based) : B2:D10, ou B2 pour une seule cellule. */
export function zoneLabel(z: Zone): string {
	const col = (c: number) => {
		let s = '';
		for (let n = c + 1; n > 0; n = Math.floor((n - 1) / 26)) s = String.fromCharCode(65 + ((n - 1) % 26)) + s;
		return s;
	};
	const a = col(z.c0) + (z.r0 + 1);
	return z.r0 === z.r1 && z.c0 === z.c1 ? a : `${a}:${col(z.c1)}${z.r1 + 1}`;
}

/** Bilan de Remplacer tout. */
export function replacedLabel(n: number, inSelection: boolean): string {
	const where = inSelection ? ' dans la sélection' : '';
	return n ? `${n} cellule${n > 1 ? 's' : ''} remplacée${n > 1 ? 's' : ''}${where} (Ctrl+Z pour annuler)` : `Aucune cellule trouvée${where}`;
}

export type FinderAction = 'next' | 'prev' | 'close' | 'focusFind' | 'openReplace' | null;

/**
 * Touche dans la barre : Entrée ou F3 suivant, Maj+Entrée ou Maj+F3
 * précédent, Échap ferme, Ctrl+F revient au champ Rechercher, Ctrl+H ouvre
 * Remplacer par (jamais la recherche du navigateur).
 */
export function finderAction(e: { key: string; shiftKey?: boolean; ctrlKey?: boolean; metaKey?: boolean; altKey?: boolean }): FinderAction {
	const ctrl = !!(e.ctrlKey || e.metaKey);
	if (e.key === 'Escape') return 'close';
	if ((e.key === 'Enter' && !ctrl && !e.altKey) || (e.key === 'F3' && !ctrl)) return e.shiftKey ? 'prev' : 'next';
	if (ctrl && !e.altKey && e.key.toLowerCase() === 'f') return 'focusFind';
	if (ctrl && !e.altKey && e.key.toLowerCase() === 'h') return 'openReplace';
	return null;
}
