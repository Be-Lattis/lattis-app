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
