// Colonnes saisies à droite d'un tableau croisé (décision 0008, partie TCD ;
// internal/canvas/pivothand.go) : les dernières colonnes de ses cellules,
// dessinées dans son cadre avec un en-tête vert, modifiables (valeurs par
// ligne, ou une formule pour toute la colonne).

/** Ce qu'il faut d'un TCD affiché (SheetPivot). */
export interface HandPivot {
	row: number;
	col: number;
	cells: string[][] | null;
	error?: string;
	hand?: { name: string; formula?: string }[] | null;
}

/** Première colonne (de la feuille) des colonnes saisies du TCD. */
export function handStart(p: HandPivot): number {
	const w = p.cells?.[0]?.length ?? 0;
	return p.col + w - (p.hand?.length ?? 0);
}

/** Rang de la colonne saisie du TCD en (r, c) (en-tête compris), -1 hors d'elles. */
export function pivotHandAt(p: HandPivot, r: number, c: number): number {
	const n = p.hand?.length ?? 0;
	if (!n || p.error || !p.cells?.length) return -1;
	const s = handStart(p);
	if (r < p.row || r >= p.row + p.cells.length || c < s || c >= s + n) return -1;
	return c - s;
}

/** Texte à éditer d'une cellule de colonne saisie : la formule de la colonne (lignes sous l'en-tête), sinon la valeur affichée. */
export function handEditText(p: HandPivot, r: number, c: number, shown: string): string {
	const j = pivotHandAt(p, r, c);
	if (j < 0) return shown;
	const f = p.hand![j].formula;
	return r > p.row && f ? f : shown;
}

/** Alignement d'une valeur de colonne saisie : un nombre à droite, un texte à gauche. */
export function handAlign(text: string): 'left' | 'right' {
	const t = text.replace(/[\s  ]/g, '').replace(/%$/, '');
	return /^[-+]?(\d+([.,]\d*)?|[.,]\d+)(e[-+]?\d+)?$/i.test(t) ? 'right' : 'left';
}

/** Note du volet : valeurs saisies mises de côté (étiquettes absentes du TCD). */
export function handKeptNote(kept: number | undefined, note: string | undefined): string {
	if (note) return note;
	if (!kept) return '';
	return kept > 1
		? `Valeurs saisies de ${kept} lignes absentes du tableau croisé, mises de côté : elles reviendront avec leurs étiquettes.`
		: `Valeurs saisies d'une ligne absente du tableau croisé, mises de côté : elles reviendront avec son étiquette.`;
}
