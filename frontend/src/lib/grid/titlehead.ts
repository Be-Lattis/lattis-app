// « Utiliser comme en-têtes » (TitleToHeader, internal/canvas/titlehead.go) :
// des en-têtes tapés après les données, sur la ligne juste au-dessus d'un
// tableau, deviennent les noms de ses colonnes.

/** Ce qu'il faut d'un tableau pour situer la ligne au-dessus de son en-tête. */
export interface TitleTable {
	name: string;
	row: number;
	col: number;
	twoLevel?: boolean;
	columns: unknown[];
}

/** Le tableau dont l'en-tête est juste sous (row, col), sur ses colonnes ;
 * pas un tableau aux en-têtes sur deux lignes (la ligne du dessus est sa
 * bande de groupes). */
export function tableUnderTitle<T extends TitleTable>(tables: T[], row: number, col: number): T | undefined {
	return tables.find((t) => !t.twoLevel && t.row === row + 1 && col >= t.col && col < t.col + t.columns.length);
}

/** Au moins une cellule remplie juste au-dessus des colonnes du tableau. */
export function titleAboveFilled(t: TitleTable, raw: (row: number, col: number) => string | undefined): boolean {
	if (t.twoLevel || t.row < 1) return false;
	for (let i = 0; i < t.columns.length; i++) if ((raw(t.row - 1, t.col + i) ?? '').trim() !== '') return true;
	return false;
}

/** Texte de la proposition de la barre d'état, après la saisie. */
export const headerOfferText = (table: string) => `Cette ligne ressemble aux en-têtes de « ${table} »`;
