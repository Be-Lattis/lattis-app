// En-têtes sur deux lignes (voir internal/canvas/twolevel.go) : le nom
// « Groupe.Sous » d'une colonne s'affiche en deux parties, le groupe dans une
// bande au-dessus de l'en-tête, fusionnée sur les colonnes voisines du même
// groupe. Le premier point sépare ; sans point, pas de groupe.

/** Groupe (vide sans point) et sous-en-tête d'un nom de colonne. */
export function splitGroup(name: string): { group: string; sub: string } {
	const i = name.indexOf('.');
	if (i > 0 && i < name.length - 1) return { group: name.slice(0, i), sub: name.slice(i + 1) };
	return { group: '', sub: name };
}

/** Texte de l'en-tête : sans le groupe quand les en-têtes sont sur deux lignes. */
export function headerLabel(name: string, twoLevel: boolean | undefined): string {
	return twoLevel ? splitGroup(name).sub : name;
}

/** Une cellule de la bande : le groupe group couvre les colonnes i0 à i1 (rangs dans le tableau). */
export interface GroupSpan {
	group: string;
	i0: number;
	i1: number;
}

/** Cellules de la bande : colonnes consécutives du même groupe réunies ; une colonne sans groupe a sa cellule vide. */
export function groupSpans(names: readonly string[]): GroupSpan[] {
	const out: GroupSpan[] = [];
	names.forEach((n, i) => {
		const g = splitGroup(n).group;
		const last = out[out.length - 1];
		if (g && last && last.group === g && last.i1 === i - 1) last.i1 = i;
		else out.push({ group: g, i0: i, i1: i });
	});
	return out;
}

/** Cellule de la bande qui contient la colonne i. */
export function spanAt(names: readonly string[], i: number): GroupSpan | undefined {
	return groupSpans(names).find((s) => s.i0 <= i && i <= s.i1);
}

/** Ce qu'il faut d'un tableau pour placer sa bande. */
export interface BandTable {
	name?: string;
	row: number; // ligne d'en-tête
	col: number;
	twoLevel?: boolean;
	columns: readonly { name: string }[];
}

/** Première ligne d'un tableau : sa bande des groupes, sinon son en-tête. */
export function tableTop(t: { row: number; twoLevel?: boolean }): number {
	return t.twoLevel ? t.row - 1 : t.row;
}

/**
 * Colonne sur un niveau (nom sans groupe) dans un tableau à deux lignes : sa
 * cellule de la bande et son en-tête ne font qu'une cellule. split : colonnes
 * que l'on vient de défusionner pour leur taper un groupe (séparées tant
 * qu'elles n'en ont pas).
 */
export function isSingle(name: string, split = false): boolean {
	return !split && !splitGroup(name).group;
}

/** Colonnes séparées à la main (Défusionner) : tableau et nom de colonne. */
export type SplitTest = (table: BandTable, name: string) => boolean;

/**
 * Groupes sur plusieurs colonnes et colonnes sur un niveau (bande et en-tête
 * réunis), au format des fusions de la feuille : le curseur, les flèches et
 * la sélection les passent comme une cellule fusionnée.
 */
export function bandMerges(tables: readonly BandTable[], split?: SplitTest): { row: number; col: number; rows: number; cols: number }[] {
	const out: { row: number; col: number; rows: number; cols: number }[] = [];
	for (const t of tables) {
		if (!t.twoLevel || t.row < 1) continue;
		const names = t.columns.map((k) => k.name);
		for (const s of groupSpans(names)) {
			if (s.i1 > s.i0) out.push({ row: t.row - 1, col: t.col + s.i0, rows: 1, cols: s.i1 - s.i0 + 1 });
			else if (isSingle(names[s.i0], split?.(t, names[s.i0]))) out.push({ row: t.row - 1, col: t.col + s.i0, rows: 2, cols: 1 });
		}
	}
	return out;
}
