// Aperçu du cadre d'un tableau pendant la saisie : une ligne ou une colonne
// de plus quand la cellule active est juste dessous, à droite ou à gauche (la
// saisie l'étendra), jusqu'au tableau voisin qu'elle rejoindrait (fusion).
// Mêmes règles que le moteur (input, internal/canvas/actions.go), pour que le
// cadre ne promette pas ce que la saisie ne fera pas (audit 2 M1, M2).

export interface GrowTable {
	row: number; // ligne d'en-tête
	col: number;
	/** Tableau chargé par une requête : il ne s'agrandit pas par la saisie. */
	query?: unknown;
	columns: readonly { filter?: unknown }[];
}

export interface GrowEntry {
	t: GrowTable;
	last: number; // dernière ligne affichée
	lastCol: number;
}

export interface GrowContext<E extends GrowEntry> {
	row: number; // cellule active
	col: number;
	entries: readonly E[];
	/** Feuille protégée : rien à saisir. */
	locked?: boolean;
	/** Structure protégée : seule une ligne de plus (saisie dessous), jamais une colonne. */
	structure?: boolean;
	/** Cellule gardée hors des tableaux (note, total). */
	keptOut?: (row: number, col: number) => boolean;
	/** Cellule d'une plage (tableau converti) : la saisie y reste libre. */
	inRange?: (row: number, col: number) => boolean;
}

export interface Grown {
	last: number;
	lastCol: number;
	firstCol: number;
}

/** Le tableau a un filtre : il ne prend ni ligne ni colonne sur ses lignes de données. */
export const filtered = (t: GrowTable) => t.columns.some((c) => !!c.filter);

const inside = (e: GrowEntry, r: number, c: number) => r >= e.t.row && r <= e.last && c >= e.t.col && c <= e.lastCol;

/** La cellule est juste sous le tableau, sur ses colonnes : la saisie y ajoute une ligne. */
const below = (e: GrowEntry, r: number, c: number) => r === e.last + 1 && c >= e.t.col && c <= e.lastCol && !e.t.query && !filtered(e.t);

/** Étendue affichée du tableau e pour la cellule active. */
export function grown<E extends GrowEntry>(e: E, g: GrowContext<E>): Grown {
	const { row, col, entries } = g;
	const same = { last: e.last, lastCol: e.lastCol, firstCol: e.t.col };
	if (g.locked || e.t.query || g.keptOut?.(row, col) || g.inRange?.(row, col)) return same;
	if (below(e, row, col)) return { ...same, last: e.last + 1 };
	if (g.structure) return same;
	// Case au coin de deux tableaux : la ligne dessous l'emporte (comme le
	// moteur), même sous un tableau de requête, qui la refuse.
	if (entries.some((o) => o !== e && row === o.last + 1 && col >= o.t.col && col <= o.lastCol && !filtered(o.t))) return same;
	// Sur une ligne de données d'un tableau filtré, pas de colonne de plus.
	if (filtered(e.t) && row !== e.t.row) return same;
	const onRows = row >= e.t.row && row <= e.last && !entries.some((o) => inside(o, row, col));
	// Juste à gauche : nouvelle première colonne ; pas si la case est aussi
	// juste à droite d'un autre tableau (c'est lui qui prend la saisie).
	if (onRows && col === e.t.col - 1) {
		const takenByLeft = entries.some((o) => o !== e && o.lastCol === col - 1 && row >= o.t.row && row <= o.last);
		return takenByLeft ? same : { ...same, firstCol: e.t.col - 1 };
	}
	if (onRows && col === e.lastCol + 1) {
		const next = entries.find((o) => o.t.row === e.t.row && o.t.col === col + 1);
		if (next) return { ...same, lastCol: next.lastCol, last: Math.max(e.last, next.last) };
		return { ...same, lastCol: e.lastCol + 1 };
	}
	return same;
}
