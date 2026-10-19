// Lignes masquées par le filtre d'un tableau. Elles gardent leur place sur
// la feuille (comme dans Excel, une cellule libre a la même adresse avec ou
// sans filtre) : une bande grisée « N lignes masquées » les représente sous
// les lignes affichées et refuse la saisie ; les cellules libres commencent
// sous la dernière ligne réelle du tableau.

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

/** Libellé de la bande. */
export function bandLabel(n: number): string {
	const s = n > 1 ? 's' : '';
	return `⧩ ${n.toLocaleString('fr-FR')} ligne${s} masquée${s} par le filtre`;
}

/** Message affiché quand une saisie vise la bande. */
export function bandRefusal(table: string): string {
	return `Lignes masquées par le filtre du tableau « ${table} » : effacez le filtre pour les modifier`;
}
