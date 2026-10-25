// Plan de la feuille (Données › Grouper) : groupes de lignes ou de colonnes,
// comme un tableur. Le classeur garde le niveau de chaque ligne groupée et
// l'état réduit (canvas/outline.go) ; ce module en tire les groupes, la
// place de leurs boutons [−]/[+] et des boutons de niveau 1 2 3, et le
// groupe visé par Afficher / Masquer le détail (testé par vitest).

/** Ligne (ou colonne) groupée : niveau 1 à 8 ; folded : groupes réduits qui s'arrêtent ici (bit n-1 : niveau n). */
export interface OutlineLine {
	index: number;
	level: number;
	folded?: number;
}

/** Plan d'une feuille tel que le classeur le donne (Layout.outline). */
export interface SheetOutline {
	rows: OutlineLine[] | null;
	cols: OutlineLine[] | null;
	rowsAbove?: boolean;
	colsLeft?: boolean;
	hiddenRows: number[] | null;
	hiddenCols: number[] | null;
}

/** Groupe : lignes from..to de niveau level. */
export interface OutlineGroup {
	level: number;
	from: number;
	to: number;
	folded: boolean;
}

/** Ce que la grille affiche, pour la barre du plan (pixels de la feuille, avant le zoom). */
export interface OutlineGeom {
	zoom: number;
	/** Hauteur de la ligne des lettres de colonnes, largeur de la gouttière. */
	top: number;
	margin: number;
	/** Début et taille d'une ligne ou d'une colonne à l'écran, null si masquée. */
	row(r: number): [number, number] | null;
	col(c: number): [number, number] | null;
}

/** Commandes de Données › Plan (ruban, Alt+Maj+→ / ←). */
export type OutlineAction = 'group' | 'ungroup' | 'groupRows' | 'groupCols' | 'ungroupRows' | 'ungroupCols' | 'clear' | 'show' | 'hide';

/** Largeur d'un niveau dans la barre du plan (pixels). */
export const LEVEL_W = 16;

/** Groupes, du niveau 1 au plus profond ; before : synthèse avant les détails. */
export function outlineGroups(lines: readonly OutlineLine[] | null | undefined, before = false): OutlineGroup[] {
	const ls = [...(lines ?? [])].sort((a, b) => a.index - b.index);
	const folded = new Map(ls.map((l) => [l.index, l.folded ?? 0]));
	const out: OutlineGroup[] = [];
	const deep = outlineDepth(ls);
	for (let lv = 1; lv <= deep; lv++) {
		let start = -1;
		let prev = -2;
		const flush = () => {
			if (start >= 0) {
				const marker = before ? start : prev;
				out.push({ level: lv, from: start, to: prev, folded: ((folded.get(marker) ?? 0) & (1 << (lv - 1))) !== 0 });
			}
			start = -1;
		};
		for (const l of ls) {
			if (l.level < lv) continue;
			if (l.index !== prev + 1) flush();
			if (start < 0) start = l.index;
			prev = l.index;
		}
		flush();
	}
	return out;
}

/** Niveau le plus profond (0 sans groupe). */
export function outlineDepth(lines: readonly OutlineLine[] | null | undefined): number {
	let n = 0;
	for (const l of lines ?? []) n = Math.max(n, l.level);
	return n;
}

/** Largeur (ou hauteur) de la barre du plan : un bouton par niveau, plus « tout afficher » ; 0 sans groupe. */
export function bandSize(depth: number): number {
	return depth > 0 ? (depth + 1) * LEVEL_W + 4 : 0;
}

/** Centre de la colonne du niveau (1 à n+1) dans la barre. */
export function levelCenter(level: number): number {
	return 2 + (level - 1) * LEVEL_W + LEVEL_W / 2;
}

/** Ligne de synthèse d'un groupe, où se pose son bouton [−]/[+] (-1 : aucune). */
export function summaryLine(g: OutlineGroup, before = false): number {
	return before ? g.from - 1 : g.to + 1;
}

/**
 * Groupe visé par Masquer le détail (fold) ou Afficher le détail depuis la
 * ligne active : d'abord celui dont elle est la synthèse, sinon (masquer)
 * le plus intérieur qui la contient et reste ouvert. null : rien à faire.
 */
export function detailTarget(lines: readonly OutlineLine[] | null | undefined, before: boolean, line: number, fold: boolean): { level: number; line: number } | null {
	const gs = outlineGroups(lines, before);
	const summed = gs.filter((g) => summaryLine(g, before) === line && g.folded !== fold).sort((a, b) => a.level - b.level);
	if (summed.length) return { level: summed[0].level, line: summed[0].from };
	if (!fold) return null;
	const inside = gs.filter((g) => line >= g.from && line <= g.to && !g.folded).sort((a, b) => b.level - a.level);
	return inside.length ? { level: inside[0].level, line } : null;
}

/** Ce qu'un clic vise dans la barre : bouton de niveau, ou bouton d'un groupe. */
export type OutlineHit = { kind: 'level'; level: number } | { kind: 'group'; group: OutlineGroup };

/**
 * Bouton sous un point de la barre : pos le long de la barre (pixels de la
 * barre), across en travers ; head : longueur de l'en-tête (boutons de
 * niveau) ; at(i) : début et taille de la ligne i, ou null si cachée.
 */
export function outlineHit(
	groups: readonly OutlineGroup[],
	depth: number,
	before: boolean,
	pos: number,
	across: number,
	head: number,
	at: (i: number) => [number, number] | null
): OutlineHit | null {
	const lv = Math.floor((across - 2) / LEVEL_W) + 1;
	if (lv < 1 || lv > depth + 1) return null;
	if (pos < head) return { kind: 'level', level: lv };
	for (const g of groups) {
		if (g.level !== lv) continue;
		const s = summaryLine(g, before);
		const p = s >= 0 ? at(s) : null;
		if (p && pos >= p[0] && pos < p[0] + p[1]) return { kind: 'group', group: g };
	}
	return null;
}
