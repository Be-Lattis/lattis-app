// La vue d'abord (docs/audits/vue-d-abord-gros-classeurs-2026-10-22.md) :
// la mise en page rapide (LayoutFirst) laisse de côté ce qui parcourt un
// grand tableau sans être en cache (pending : suggestion de clé et
// numérotation d'un tableau, TCD, graphique). En attendant la mise en page
// complète, ce que la feuille affichait déjà reste à l'écran, tel quel,
// plutôt qu'un « calcul… » : après une saisie dans la source d'un TCD, ses
// anciennes valeurs, comme Excel.

import type { ChartInfo, Layout, PivotInfo, TableInfo } from '#bindings/internal/canvas/models.js';

const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);

/**
 * Mise en page rapide next, complétée par prev (la précédente de la même
 * feuille) : un tableau, un TCD ou un graphique encore à calculer garde ce
 * que prev en montrait, s'il y était calculé avec la même configuration.
 * next.pending reste vrai : la mise en page complète suit.
 */
export function keepShown(prev: Layout | null, next: Layout): Layout {
	if (!prev || !next.pending || prev.sheet !== next.sheet) return next;
	const tables = (next.tables ?? []).map((t): TableInfo => {
		if (!t.pending) return t;
		// prev peut être lui-même une mise en page rapide complétée ici (deux
		// relectures avant la fin du calcul) : ses valeurs gardées comptent
		// aussi, sinon l'icône de la clé disparaît et la colonne change de
		// largeur le temps du calcul.
		const o = prev.tables?.find((x) => x.id === t.id);
		return o ? { ...t, keyHint: o.keyHint, autoNumber: o.autoNumber } : t;
	});
	const pivots = (next.pivots ?? []).map((p): PivotInfo => {
		if (!p.pending) return p;
		const o = prev.pivots?.find((x) => x.id === p.id && !x.pending && !x.error && same(x.config, p.config));
		return o ? { ...o, row: p.row, col: p.col, pending: false } : p;
	});
	const charts = (next.charts ?? []).map((c): ChartInfo => {
		if (!c.pending) return c;
		const o = prev.charts?.find((x) => x.id === c.id && !x.pending && !x.error && same(x.config, c.config));
		return o ? { ...o, row: c.row, col: c.col, pending: false } : c;
	});
	return { ...next, tables, pivots, charts };
}
