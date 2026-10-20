// Types de graphiques : libellés, icônes et familles (lus par le volet, le
// ruban et le dessin). Le classeur accepte les mêmes (internal/canvas/chart.go).

import type { IconName } from '#lib/icons/icons.js';
import { fmtNumber, t, tAny } from '#lib/i18n/index.js';

// Attention, les clés bar et column sont l'inverse du vocabulaire anglais
// d'Excel : « bar » est l'histogramme (barres verticales, c:barChart
// barDir="col" en DrawingML) et « column » les barres horizontales
// (barDir="bar"). Elles sont gardées telles quelles dans les classeurs.
export type ChartKind =
	| 'bar'
	| 'column'
	| 'line'
	| 'area'
	| 'pie'
	| 'doughnut'
	| 'scatter'
	| 'combo'
	| 'bar3d'
	| 'pie3d'
	| 'map'
	| 'waterfall'
	| 'treemap'
	| 'radar'
	| 'funnel'
	| 'boxplot'
	| 'histogram'
	| 'gauge';

export interface KindInfo {
	kind: ChartKind;
	readonly label: string;
	icon: IconName;
	readonly hint: string;
}

/** Type de graphique : libellé et aide relus dans la langue de l'interface. */
const kind = (k: ChartKind, icon: IconName): KindInfo => ({
kind: k,
icon,
get label() {
return tAny(`chart.kind.${k}.label`);
},
get hint() {
return tAny(`chart.kind.${k}.hint`);
}
});

/** Tous les types, dans l'ordre du volet. */
export const KINDS: KindInfo[] = [
	kind('bar', 'chart-bar'),
	kind('column', 'chart-column'),
	kind('line', 'chart-line'),
	kind('area', 'chart-area'),
	kind('pie', 'chart-pie'),
	kind('doughnut', 'chart-doughnut'),
	kind('scatter', 'chart-scatter'),
	kind('combo', 'chart-combo'),
	kind('bar3d', 'chart-bar3d'),
	kind('pie3d', 'chart-pie3d'),
	kind('map', 'chart-map'),
	kind('waterfall', 'chart-waterfall'),
	kind('treemap', 'chart-treemap'),
	kind('radar', 'chart-radar'),
	kind('funnel', 'chart-funnel'),
	kind('boxplot', 'chart-boxplot'),
	kind('histogram', 'chart-histogram'),
	kind('gauge', 'chart-gauge')
];

/** Types proposés par « Autres graphiques » (ruban) : au-delà des classiques. */
export const MORE_KINDS = KINDS.filter((k) => !['bar', 'line', 'pie', 'scatter'].includes(k.kind));

export const kindInfo = (k: string) => KINDS.find((i) => i.kind === k);

/** Graphique à axes (catégories et valeurs). */
export const CARTESIAN = new Set<string>(['bar', 'column', 'line', 'area', 'scatter', 'combo', 'bar3d', 'waterfall', 'boxplot', 'histogram']);
/** Graphique dont la légende nomme les catégories (une couleur par catégorie). */
export const BY_CATEGORY = new Set<string>(['pie', 'doughnut', 'pie3d', 'treemap', 'funnel']);
/** Une seule série dessinée (la première). */
export const SINGLE_SERIES = new Set<string>(['pie', 'doughnut', 'pie3d', 'treemap', 'funnel', 'waterfall', 'gauge']);
/** Empilement possible. */
export const STACKABLE = new Set<string>(['bar', 'column', 'area', 'bar3d']);
/** Axe secondaire possible. */
export const SECONDARY = new Set<string>(['bar', 'line', 'area', 'scatter', 'combo']);
/** Lignes lues une à une (pas de regroupement par catégorie). */
export const RAW_ROWS = new Set<string>(['map', 'histogram', 'boxplot']);

/**
 * Axe X et séries d'un nouveau graphique d'un tableau (grille et Onglet
 * Document, même règle) : X est la colonne active si elle n'est pas
 * numérique (un nuage de points : si elle l'est), sinon la première qui
 * convient ; les séries sont les autres colonnes de nombres (la dernière
 * seule pour un graphique à une série ; pour les fréquences, l'active).
 */
export function chartFields(kind: string, cols: { name: string; numeric: boolean }[], cur = -1): { x: string; series: string[] } {
	const c = cols[cur];
	const x =
		kind === 'scatter'
			? c?.numeric ? c : (cols.find((k) => k.numeric) ?? cols[0])
			: c && !c.numeric ? c : (cols.find((k) => !k.numeric) ?? cols[0]);
	let series = cols.filter((k) => k !== x && k.numeric).map((k) => k.name);
	if (SINGLE_SERIES.has(kind)) series = series.slice(-1);
	if (kind === 'histogram' && series.length) series = [c?.numeric && c !== x ? c.name : series[0]];
	return { x: x?.name ?? '', series };
}

/**
 * Axe X et séries d'après les colonnes sélectionnées (rangs dans le
 * tableau), comme Excel : X est la première non numérique de la sélection
 * (pour un nuage, la première numérique ; sinon celle que donne
 * chartFields), les séries les autres colonnes de nombres sélectionnées.
 * Moins de deux colonnes sélectionnées, ou aucune série : chartFields.
 */
export function chartFieldsFrom(kind: string, cols: { name: string; numeric: boolean }[], selected: number[], cur = -1): { x: string; series: string[] } {
	const base = chartFields(kind, cols, cur);
	const sel = selected.map((i) => cols[i]).filter(Boolean);
	if (sel.length < 2) return base;
	const x = (kind === 'scatter' ? sel.find((c) => c.numeric) : sel.find((c) => !c.numeric))?.name ?? base.x;
	let series = sel.filter((c) => c.numeric && c.name !== x).map((c) => c.name);
	if (!series.length) return base;
	if (SINGLE_SERIES.has(kind) || kind === 'histogram') series = series.slice(0, 1);
	return { x, series };
}

/** Titre par défaut d'un nouveau graphique, comme Excel : le nom de la série quand il n'y en a qu'une. */
export const defaultChartTitle = (series: string[]): string | undefined => (series.length === 1 ? series[0] : undefined);

/**
 * Nouvelle liste de séries (ajout, retrait, ordre) : couleurs, axe
 * secondaire et formes du combiné suivent chaque série par son nom au lieu
 * de rester à leur rang. Les couleurs des secteurs (par catégorie) et de la
 * cascade (hausse, baisse, total) ne sont pas celles des séries : gardées.
 */
export function reorderSeries(
	cfg: { kind: string; series: string[]; colors?: string[]; secondary?: boolean[]; seriesKinds?: string[] },
	next: string[]
): { series: string[]; colors?: string[]; secondary?: boolean[]; seriesKinds?: string[] } {
	const old = (n: string) => cfg.series.indexOf(n);
	const out: { series: string[]; colors?: string[]; secondary?: boolean[]; seriesKinds?: string[] } = { series: next };
	if (!BY_CATEGORY.has(cfg.kind) && cfg.kind !== 'waterfall') {
		out.colors = cfg.colors?.length ? next.map((n) => (old(n) >= 0 ? (cfg.colors![old(n)] ?? '') : '')) : undefined;
		if (out.colors && !out.colors.some(Boolean)) out.colors = undefined;
	}
	const sec = next.map((n) => old(n) >= 0 && !!cfg.secondary?.[old(n)]);
	out.secondary = sec.some(Boolean) ? sec : undefined;
	if (cfg.kind === 'combo') out.seriesKinds = next.map((n, k) => (old(n) >= 0 ? seriesShape(cfg.seriesKinds, old(n)) : k === 0 ? 'bar' : 'line'));
	return out;
}

/** Points lus au plus par graphique (canvas.MaxChartPoints). */
export const MAX_POINTS = 2000;


/** Coupure des données d'un graphique, à signaler sous le dessin et dans le volet (vide : aucune). */
export function chartNote(info?: { total?: number; truncated?: boolean; sampled?: boolean } | null): string {
	if (!info?.total) return '';
	if (info.truncated) return t('chart.note.truncated', { max: MAX_POINTS, total: info.total });
	if (info.sampled) return t('chart.note.sampled', { max: MAX_POINTS, total: info.total });
	return '';
}

/**
 * Remarques du volet sur les données dessinées : coupure, valeurs négatives
 * ignorées, X non numérique d'un nuage, séries au-delà de la première.
 */
export function chartNotes(
	cfg: { kind: string; series: string[] },
	info?: { labels?: string[] | null; series?: { values: (number | null)[] | null }[] | null; xValues?: (number | null)[] | null; total?: number; truncated?: boolean; sampled?: boolean } | null
): string[] {
	const out: string[] = [];
	const cut = chartNote(info);
	if (cut) out.push(cut + '.');
	const first = info?.series?.[0]?.values ?? [];
	const neg = first.filter((v) => v !== null && v < 0).length;
	if (neg && ['pie', 'doughnut', 'pie3d', 'treemap'].includes(cfg.kind)) out.push(t('chart.note.negativeIgnored', { n: neg }));
	if (neg && cfg.kind === 'funnel') out.push(t('chart.note.negativeAbs', { n: neg }));
	if (cfg.kind === 'scatter' && info?.labels?.length && !info.xValues?.some((v) => v !== null)) out.push(t('chart.note.scatterRank'));
	if (SINGLE_SERIES.has(cfg.kind) && cfg.series.length > 1) out.push(t('chart.note.firstSeries'));
	return out;
}

/** Résumé d'un graphique pour les lecteurs d'écran : type, séries, étendue. */
export function chartSummary(kind: string, labels: string[], series: { name: string; values: (number | null)[] }[], total?: number): string {
	const name = kindInfo(kind)?.label ?? t('chart.chart');
	const parts = [t('chart.summary.series', { name, n: series.length })];
	if (labels.length) parts.push(t('chart.summary.points', { n: total && total > labels.length ? total : labels.length, first: labels[0], last: labels[labels.length - 1] }));
	for (const s of series.slice(0, 6)) {
		const vs = s.values.filter((v): v is number => v !== null);
		if (vs.length) parts.push(t('chart.summary.range', { name: s.name, min: fmtNumber(Math.min(...vs)), max: fmtNumber(Math.max(...vs)) }));
	}
	return parts.join(' ; ') + '.';
}

/** Forme d'une série d'un graphique combiné : la première en barres, les autres en courbes. */
export function seriesShape(kinds: string[] | undefined, k: number): 'bar' | 'line' | 'area' {
	const s = kinds?.[k];
	return s === 'bar' || s === 'line' || s === 'area' ? s : k === 0 ? 'bar' : 'line';
}

/** Colonnes de coordonnées repérées par le classeur pour une carte (canvas.MapGuess). */
export interface MapGuess {
	mode: string;
	lat?: string;
	lon?: string;
	coord?: string;
	name?: string;
}

/** Choix de coordonnées d'une carte : deux colonnes (latitude, longitude) ou une seule. */
export interface MapChoice {
	coords: 'pair' | 'single';
	cols: string[];
	value: string;
	name: string;
	draft: boolean;
}

/**
 * Choix affiché par le volet : celui de la carte, ou, quand elle attend ses
 * coordonnées sans qu'aucune soit choisie, les colonnes repérées par le
 * classeur. Séries : [latitude, longitude, valeur] ou [coordonnées, valeur].
 */
export function mapChoice(cfg: { x: string; series: string[]; mapCoords?: string; mapDraft?: boolean }, guess?: MapGuess | null): MapChoice {
	const draft = !!cfg.mapDraft;
	const fresh = draft && !cfg.series.some(Boolean);
	const coords = cfg.mapCoords === 'single' || (!cfg.mapCoords && fresh && guess?.mode === 'single') ? 'single' : 'pair';
	const nc = coords === 'single' ? 1 : 2;
	const own = cfg.series.slice(0, nc);
	const cols =
		draft && !own.some(Boolean) && guess
			? coords === 'single'
				? [guess.coord ?? '']
				: [guess.lat ?? '', guess.lon ?? '']
			: Array.from({ length: nc }, (_, i) => own[i] ?? '');
	return { coords, cols, value: cfg.series[nc] ?? '', name: cfg.x || (draft ? (guess?.name ?? '') : ''), draft };
}

/** Choix complet : chaque colonne de coordonnées choisie, sans doublon. */
export const mapComplete = (c: MapChoice) => c.cols.every(Boolean) && new Set(c.cols).size === c.cols.length;

/**
 * Réglages d'une carte pour un choix. validate : l'utilisateur confirme, la
 * carte se dessine si le choix est complet. Une carte déjà dessinée le reste
 * tant que son choix est complet ; incomplet, elle repasse en attente.
 */
export function mapPatch(c: MapChoice, validate: boolean): { mapCoords: 'pair' | 'single'; series: string[]; x: string; mapDraft: true | undefined } {
	const drawn = mapComplete(c) && (validate || !c.draft);
	return { mapCoords: c.coords, series: [...c.cols, ...(c.value ? [c.value] : [])], x: c.name, mapDraft: drawn ? undefined : true };
}
