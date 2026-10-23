// Ajustement automatique de plusieurs colonnes ou lignes, comme Excel : un
// double-clic sur la poignée d'une colonne (ligne) sélectionnée, quand la
// sélection en compte plusieurs (toute la feuille par le coin, colonnes ou
// lignes entières, plage), ajuste toutes celles de la sélection, chacune à
// son contenu, en une seule étape d'annulation. Hors de la sélection, seule
// la colonne (ligne) de la poignée.

/** Le double-clic sur la poignée de i vaut-il pour toute la sélection [lo, hi] ? */
export function fitsSelection(i: number, lo: number, hi: number): boolean {
	return hi > lo && i >= lo && i <= hi;
}

/**
 * Colonnes à ajuster pour un double-clic sur la poignée de clicked, la
 * sélection couvrant les colonnes [lo, hi] : toutes celles de la sélection
 * (sauf les masquées, qui le restent), ou clicked seule.
 */
export function fitColumns(clicked: number, lo: number, hi: number, hidden: ReadonlySet<number>): number[] {
	if (!fitsSelection(clicked, lo, hi)) return [clicked];
	const out: number[] = [];
	for (let c = lo; c <= hi; c++) if (!hidden.has(c)) out.push(c);
	return out;
}

/**
 * Lignes à remettre en hauteur automatique pour un double-clic sur la
 * poignée de clicked, la sélection couvrant les lignes [lo, hi] : celles de
 * la sélection qui ont une hauteur choisie (chosen ; les autres sont déjà
 * automatiques, et la feuille entière peut compter des millions de lignes),
 * ou clicked seule.
 */
export function fitRows(clicked: number, lo: number, hi: number, chosen: Iterable<number>): number[] {
	if (!fitsSelection(clicked, lo, hi)) return [clicked];
	const out: number[] = [];
	for (const r of chosen) if (r >= lo && r <= hi) out.push(r);
	return out.sort((a, b) => a - b);
}
