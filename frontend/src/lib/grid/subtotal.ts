// Données › Sous-total : fonctions proposées (codes de SOUS.TOTAL) et choix
// de départ de la boîte (testé par vitest). Voir canvas/subtotal.go.

/** Options envoyées au classeur (canvas.SubtotalOptions). */
export interface SubtotalOptions {
	table: string;
	by: string;
	func: number;
	columns: string[];
	replace: boolean;
	pageBreaks: boolean;
	below: boolean;
}

/** Un tableau de la feuille : ses colonnes, et lesquelles sont des nombres. */
export interface SubtotalTable {
	name: string;
	columns: string[];
	numeric: boolean[];
}

/** Fonctions de la boîte, dans l'ordre d'un tableur : code de SOUS.TOTAL, libellé (clé i18n). */
export const SUBTOTAL_FUNCS: readonly { code: number; label: string }[] = [
	{ code: 9, label: 'grid.subtotal.fn.sum' },
	{ code: 3, label: 'grid.subtotal.fn.count' },
	{ code: 1, label: 'grid.subtotal.fn.average' },
	{ code: 4, label: 'grid.subtotal.fn.max' },
	{ code: 5, label: 'grid.subtotal.fn.min' },
	{ code: 6, label: 'grid.subtotal.fn.product' },
	{ code: 2, label: 'grid.subtotal.fn.countNums' },
	{ code: 7, label: 'grid.subtotal.fn.stdev' },
	{ code: 8, label: 'grid.subtotal.fn.stdevp' },
	{ code: 10, label: 'grid.subtotal.fn.var' },
	{ code: 11, label: 'grid.subtotal.fn.varp' }
];

/**
 * Choix de départ : le tableau de la cellule active, sa colonne pour « À
 * chaque changement de » (sinon la première), Somme, et la dernière colonne
 * de nombres à totaliser.
 */
export function subtotalDefaults(tables: readonly SubtotalTable[], table?: string, column?: number): { table: string; by: string; func: number; columns: string[] } {
	const t = tables.find((x) => x.name === table) ?? tables[0];
	if (!t) return { table: '', by: '', func: 9, columns: [] };
	const byAt = column !== undefined && column >= 0 && column < t.columns.length && !t.numeric[column] ? column : t.numeric.findIndex((n) => !n);
	const by = t.columns[Math.max(0, byAt)] ?? '';
	let last = -1;
	t.numeric.forEach((n, i) => {
		if (n && t.columns[i] !== by) last = i;
	});
	return { table: t.name, by, func: 9, columns: last >= 0 ? [t.columns[last]] : [] };
}
