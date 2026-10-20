// Lignes masquées par le filtre d'un tableau. Elles gardent leur place sur
// la feuille (comme dans Excel, une cellule libre a la même adresse avec ou
// sans filtre) : une bande grisée « N lignes masquées » les représente sous
// les lignes affichées et refuse la saisie ; les cellules libres commencent
// sous la dernière ligne réelle du tableau.
import { fmtNumber, t } from '#lib/i18n/index.js';

export interface FilteredTable {
	name: string;
	row: number; // ligne d'en-tête
	col: number;
	rows: number; // lignes affichées
	hidden?: number; // lignes masquées
	columns: unknown[];
}

/** Lignes de la feuille couvertes par la bande, ou null sans filtre. */
export function hiddenBand(t: FilteredTable): { first: number; last: number } | null {
	const n = t.hidden ?? 0;
	if (n <= 0) return null;
	const first = t.row + t.rows + 1;
	return { first, last: first + n - 1 };
}

/** Tableau dont la bande des lignes masquées couvre la cellule. */
export function bandAt<T extends FilteredTable>(tables: T[], r: number, c: number): T | undefined {
	return tables.find((t) => {
		const b = hiddenBand(t);
		return b !== null && r >= b.first && r <= b.last && c >= t.col && c < t.col + t.columns.length;
	});
}

/**
 * Libellé de la bande ; category : lignes masquées par le seul choix de
 * l'onglet (onglet à choix), qu'on remplit comme un onglet neuf.
 */
export function bandLabel(n: number, category?: string): string {
	if (category) return t('grid.hidden.band.choice', { n, category });
	return t('grid.hidden.band.filter', { n });
}

/**
 * La bande accepte la saisie : le tableau suit l'onglet à choix et aucun
 * filtre de l'utilisateur ne s'y ajoute (la ligne saisie reçoit la valeur
 * choisie, à la suite des lignes affichées).
 */
export function bandOpen(t: { follows?: boolean; columns: readonly { filter?: unknown }[] }): boolean {
	return !!t.follows && !t.columns.some((c) => c.filter);
}

/** Message affiché quand une saisie vise la bande. */
export function bandRefusal(table: string): string {
	return t('grid.hidden.refusal', { table });
}
