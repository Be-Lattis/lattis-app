// Pastille « suit l'année » (§6.5) : où la poser sur le cadre d'un tableau
// qui suit l'onglet à choix. Au-dessus du cadre, à droite, quand il y a la
// place (ligne du dessus libre, ou bandeau du haut vide quand l'en-tête est
// collé en haut de l'écran : tableau en ligne 1) ; sinon dans le coin haut
// droit du cadre, à cheval sur son trait (un onglet qui ne couvre ni le nom
// de la dernière colonne ni l'essentiel de ses boutons ▾). left : même place, à
// gauche (pastille du nom du tableau).

export interface FollowBadgeBox {
	x: number;
	y: number;
	/** above : au-dessus du cadre ; corner : à cheval sur son trait du haut. */
	place: 'above' | 'corner';
	/** Coins arrondis pour roundRect (haut gauche, haut droit, bas droit, bas gauche). */
	radii: number[];
}

/**
 * x, y : coin haut gauche du cadre ; w : sa largeur ; bw, bh : taille de la
 * pastille. paneTop : haut de la zone des cellules du volet ; strip : le
 * bandeau du haut est juste au-dessus de ce volet (vide au-dessus d'un
 * en-tête visible) et peut la recevoir ; aboveFree : les cellules que la
 * pastille couvrirait au-dessus du cadre sont vides (ni valeur, ni tableau,
 * ni TCD, ni fusion).
 */
export function placeFollowBadge(o: { x: number; y: number; w: number; bw: number; bh: number; paneTop: number; strip: boolean; aboveFree: boolean; left?: boolean }): FollowBadgeBox {
	const x = o.left ? o.x : o.x + o.w - o.bw;
	const above = o.y - o.bh;
	if (o.aboveFree && above >= o.paneTop) return { x, y: above, place: 'above', radii: [4, 4, 0, 0] };
	// En-tête en haut de l'écran : la pastille va dans le bandeau du haut
	// (même si la ligne du dessus, sortie de l'écran, est occupée).
	if (o.strip && above >= 0 && (o.aboveFree || o.y <= o.paneTop + 0.5)) return { x, y: above, place: 'above', radii: [4, 4, 0, 0] };
	return { x: o.left ? x + 1 : x - 1, y: o.y - Math.floor(o.bh / 2), place: 'corner', radii: [4, 4, 4, 4] };
}

/** Nom de tableau tiré d'un nom d'onglet (comme identifier côté moteur) : lettres, _ et chiffres hors tête. */
export function tableIdentifier(s: string): string {
	let out = '';
	for (const ch of s) {
		if (/[\p{L}_]/u.test(ch) || (out && /\p{N}/u.test(ch))) out += ch;
	}
	return out;
}

/**
 * Pastille du nom (à gauche) : pas sur un tableau qui porte le nom de son
 * onglet (« Ventes 2026 » ↔ Ventes2026) ; l'onglet le montre déjà, et le
 * renommer renomme ce tableau (pas l'inverse).
 */
export const showNameBadge = (table: string, sheet: string): boolean => tableIdentifier(sheet).toLowerCase() !== table.toLowerCase();
