// Accessibilité de la grille : la feuille est dessinée dans un <canvas>,
// invisible pour un lecteur d'écran. Ce module construit ce qu'on lui dit de
// la cellule active (GridA11y.svelte le porte dans une cellule cachée,
// désignée par aria-activedescendant, et dans une région aria-live).

/** Ce que l'on sait de la cellule active. */
export interface CellSpeech {
	/** Adresse A1 (B3). */
	addr: string;
	/** Texte affiché de la cellule. */
	value: string;
	/** Nom défini sur la cellule, s'il y en a un. */
	name?: string;
	/** Tableau qui contient la cellule. */
	table?: string;
	/** Colonne du tableau. */
	column?: string;
	/** Ligne d'en-tête du tableau. */
	header?: boolean;
	/** Ligne de données dans le tableau (1 pour la première). */
	row?: number;
	/** Colonne calculée (formule de colonne). */
	computed?: boolean;
	/** Erreur de formule affichée (#DIV/0!…). */
	error?: boolean;
}

/** Libellé de la cellule active : « B3, Prix, 12,50 € ». */
export function cellLabel(s: CellSpeech): string {
	const parts = [s.addr];
	if (s.name) parts.push(s.name);
	if (s.table && s.column) {
		if (s.header) parts.push(`en-tête de colonne ${s.column}`, `tableau ${s.table}`);
		else parts.push(s.column);
	}
	const value = s.value.trim();
	if (!s.header) parts.push(value === '' ? 'vide' : value);
	if (s.computed && !s.header) parts.push('colonne calculée');
	if (s.error) parts.push('erreur de formule');
	return parts.join(', ');
}

/** Taille de la sélection, quand elle dépasse une cellule. */
export function selectionLabel(range: string, rows: number, cols: number): string {
	if (rows <= 1 && cols <= 1) return '';
	const l = `${rows} ligne${rows > 1 ? 's' : ''}`;
	const c = `${cols} colonne${cols > 1 ? 's' : ''}`;
	return `Sélection ${range}, ${l} sur ${c}`;
}

/** Ce que la région aria-live annonce : le mode, puis la sélection ou le tableau. */
export function announcement(s: CellSpeech, editing: boolean, selection = '', previousTable?: string): string {
	if (editing) return `Modification de ${s.addr}`;
	if (selection) return selection;
	// Entrée dans un tableau : son nom une fois, pas à chaque déplacement.
	if (s.table && s.table !== previousTable && !s.header) return `Tableau ${s.table}`;
	return '';
}
