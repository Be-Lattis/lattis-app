// Options de Rechercher / remplacer, comme le dialogue d'un tableur
// (« Options >> ») : dans la feuille ou le classeur, par lignes ou par
// colonnes, dans les formules, les valeurs ou les notes, casse, cellule
// entière. La recherche elle-même se fait côté Go (canvas/search.go), une
// feuille par appel : les résultats du classeur arrivent feuille après
// feuille.
import { t } from '#lib/i18n/index.js';
import type { Zone } from './findzone';

export type Look = 'formulas' | 'values' | 'notes';

export interface FindOpts {
	book: boolean;
	byCols: boolean;
	look: Look;
	matchCase: boolean;
	whole: boolean;
}

export const defaultFindOpts = (): FindOpts => ({ book: false, byCols: false, look: 'formulas', matchCase: false, whole: false });

/** Requête envoyée au Go (canvas.SearchQuery). */
export interface SearchQuery {
	text: string;
	sheet: number;
	book: boolean;
	in?: Zone;
	byCols: boolean;
	look: Look;
	case: boolean;
	whole: boolean;
}

/** Résultat d'une feuille (canvas.SearchResult). */
export interface SearchResult {
	sheet: number;
	name: string;
	found: { sheet: number; row: number; col: number; table?: string; text: string }[] | null;
	total: number;
	next: number;
}

/** Bilan de Remplacer tout (canvas.ReplaceResult). */
export interface ReplaceResult {
	replaced: number;
	skipped: number;
	reason?: string;
}

/** Cellule trouvée : feuille (rang k dans l'ordre parcouru, nom), position, tableau, texte. */
export interface FoundCell {
	sheet: number;
	k: number;
	name: string;
	row: number;
	col: number;
	table?: string;
	text: string;
}

/** La requête : la sélection ne borne que la recherche dans la feuille. */
export function searchQuery(text: string, sheet: number, zone: Zone | null, o: FindOpts): SearchQuery {
	const q: SearchQuery = { text, sheet: o.book ? 0 : sheet, book: o.book, byCols: o.byCols, look: o.look, case: o.matchCase, whole: o.whole };
	if (zone && !o.book) q.in = { ...zone };
	return q;
}

/** Clé des options (cache des occurrences). */
export function optsKey(o: FindOpts): string {
	return `${+o.book}${+o.byCols}${o.look}${+o.matchCase}${+o.whole}`;
}

/** Remplacer est-il possible dans ce mode ? Les valeurs se remplacent dans les formules, comme un tableur. */
export function replaceLook(o: FindOpts): FindOpts {
	return o.look === 'values' ? { ...o, look: 'formulas' } : o;
}

/**
 * Cherche dans la feuille, ou feuille après feuille dans le classeur (onPart
 * à chaque feuille, pour afficher au fur et à mesure) ; au plus cap
 * cellules, total compte tout. stale : une recherche plus récente a commencé.
 */
export async function searchAll(
	run: (q: SearchQuery) => Promise<SearchResult>,
	q: SearchQuery,
	cap: number,
	onPart?: (hits: FoundCell[], total: number) => void,
	stale?: () => boolean
): Promise<{ hits: FoundCell[]; total: number; order: number[] } | null> {
	const hits: FoundCell[] = [];
	const order: number[] = [];
	let total = 0;
	let sheet = q.sheet;
	for (;;) {
		const r = await run({ ...q, sheet });
		if (stale?.()) return null;
		const k = order.length;
		order.push(r.sheet);
		for (const f of r.found ?? []) if (hits.length < cap) hits.push({ ...f, sheet: r.sheet, k, name: r.name });
		total += r.total;
		onPart?.(hits, total);
		if (!q.book || !r.next) break;
		sheet = r.next;
	}
	return { hits, total, order };
}

/**
 * Indice de la cellule suivante (ou précédente) après from, dans l'ordre des
 * feuilles parcourues puis par lignes (ou par colonnes), en repartant du
 * début (ou de la fin) ; from absent : la première (ou la dernière). -1 si
 * aucune.
 */
export function nextFound(hits: readonly FoundCell[], from: { k: number; row: number; col: number } | null, back: boolean, byCols: boolean): number {
	if (!hits.length) return -1;
	if (!from) return back ? hits.length - 1 : 0;
	const cmp = (m: FoundCell) => {
		if (m.k !== from.k) return m.k - from.k;
		const [a, b] = byCols ? [m.col - from.col, m.row - from.row] : [m.row - from.row, m.col - from.col];
		return a || b;
	};
	const i = back ? hits.findLastIndex((m) => cmp(m) < 0) : hits.findIndex((m) => cmp(m) > 0);
	return i >= 0 ? i : back ? hits.length - 1 : 0;
}

/** Bilan affiché sous la liste : « 12 cellules trouvées », avec la part affichée au-delà de la limite. */
export function listLabel(shown: number, total: number): string {
	const n = t('grid.find.list.count', { n: total });
	return total > shown ? `${n} · ${t('grid.find.list.capped', { shown })}` : n;
}

/** Bilan de Remplacer tout : remplacées, laissées (protection, lecture seule) et pourquoi. */
export function replaceSummary(r: ReplaceResult, scope: 'sheet' | 'selection' | 'book'): string {
	let s: string;
	if (!r.replaced && r.skipped) return `${t('grid.find.skipped', { n: r.skipped })}${r.reason ? ` : ${r.reason}` : ''}`;
	if (scope === 'book') s = r.replaced ? t('grid.find.replaced.book', { n: r.replaced }) : t('grid.find.notFound.book');
	else if (scope === 'selection') s = r.replaced ? t('grid.find.replaced.selection', { n: r.replaced }) : t('grid.find.notFound.selection');
	else s = r.replaced ? t('grid.find.replaced.all', { n: r.replaced }) : t('grid.find.notFound.all');
	if (r.skipped) s += ` · ${t('grid.find.skipped', { n: r.skipped })}${r.reason ? ` : ${r.reason}` : ''}`;
	return s;
}
