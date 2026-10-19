// Numéros des lignes des tableaux (ou leur clé), sans lettres ni numéros de
// feuille hors mode A1.
//
// La gouttière ne numérote que le tableau qui la touche : celui qui occupe la
// première colonne affichée. Ses numéros sont alors en face de ses propres
// lignes, sans cellules d'un autre objet entre eux. Un tableau plus à droite
// (cellules libres ou autre tableau à sa gauche) n'a pas de numéros dans la
// gouttière, qui seraient en face de lignes sans rapport : quand il est
// actif, ses numéros s'écrivent en petit, en gris, juste à gauche de sa
// première colonne.

export interface Block {
	row: number; // ligne d'en-tête
	last: number; // dernière ligne de données
	col: number;
	lastCol: number;
}

/** Le tableau dont la ligne de données r touche la gouttière (première colonne affichée c0). */
export function gutterOwner<T extends Block>(blocks: T[], r: number, c0: number): T | undefined {
	return blocks.find((b) => r > b.row && r <= b.last && b.col <= c0 && b.lastCol >= c0);
}

/** Le tableau est-il visible sans toucher la gouttière (numéros à côté de lui) ? */
export function besideGutter(b: Block, c0: number, c1: number): boolean {
	return b.col > c0 && b.col <= c1;
}
