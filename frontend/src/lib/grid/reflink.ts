// Colonnes de référence (décision 0016) : une validation par liste tirée de
// la colonne clé entière d'un tableau (Espèces[Nom]) fait de la cellule une
// référence vers ce tableau. « Basculer sur » va à la ligne citée, Alt+←
// revient (pile de retours) ; sur la clé, « Lignes qui citent » fait le
// chemin inverse.

/** Tableau du classeur tel que la feuille le connaît (key : rang de la colonne clé, -1 ou absent : aucune). */
export interface RefTable {
	name: string;
	columns: string[];
	key?: number;
}

/** Ligne visée par une référence (DocumentService.ReferenceRow). */
export interface KeyRow {
	table: string;
	key: string;
	found: boolean;
	id?: number;
}

/** Lignes d'un tableau qui citent une valeur de clé (DocumentService.Citations). */
export interface Citation {
	table: string;
	column: string;
	count: number;
	first: number;
}

/** Endroit d'où l'on a basculé : feuille, cellule, et son adresse affichée (Suivi!C5). */
export interface RefSpot {
	sheet: number;
	row: number;
	col: number;
	label: string;
}

/**
 * Tableau et colonne clé visés par la source d'une validation, si elle est
 * exactement la colonne clé entière d'un tableau (sans casse) ; sinon null.
 */
export function refTarget(src: string | undefined, tables: readonly RefTable[]): { table: string; key: string } | null {
	if (!src) return null;
	const m = /^\s*=?\s*([^[\]!]+?)\s*\[([^[\]]+)\]\s*$/.exec(src);
	if (!m) return null;
	const name = m[1].replace(/^'(.*)'$/, '$1').trim().toLowerCase();
	if (name === 'cell') return null;
	const t = tables.find((x) => x.name.toLowerCase() === name);
	if (!t || t.key === undefined || t.key < 0) return null;
	const k = t.columns[t.key];
	return k !== undefined && k.toLowerCase() === m[2].trim().toLowerCase() ? { table: t.name, key: k } : null;
}

/** Profondeur de la pile des retours. */
export const MAX_BACK = 20;

/** Pile des retours après un saut : l'origine s'empile (sans doublon au sommet), bornée. */
export function pushBack(stack: readonly RefSpot[], spot: RefSpot): RefSpot[] {
	const top = stack[stack.length - 1];
	const same = top && top.sheet === spot.sheet && top.row === spot.row && top.col === spot.col;
	const out = same ? [...stack] : [...stack, spot];
	return out.slice(-MAX_BACK);
}

/** Retour : l'endroit au sommet, et la pile sans lui. */
export function popBack(stack: readonly RefSpot[]): { spot: RefSpot | undefined; rest: RefSpot[] } {
	return { spot: stack[stack.length - 1], rest: stack.slice(0, -1) };
}

/** Message de la barre d'état quand une validation fait d'une colonne une référence. */
export function refMessage(column: string, table: string): string {
	return `« ${column} » devient une référence vers « ${table} » : Ctrl+clic ou clic droit › Basculer sur…`;
}

/** Libellé de l'entrée « Lignes qui citent » d'une valeur de clé. */
export function citeLabel(value: string, cites: readonly Citation[]): string {
	const n = cites.reduce((s, c) => s + c.count, 0);
	return n ? `Lignes qui citent « ${value} » (${n})` : `Aucune ligne ne cite « ${value} »`;
}
