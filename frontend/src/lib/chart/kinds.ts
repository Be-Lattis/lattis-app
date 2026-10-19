// Types de graphiques : libellés, icônes et familles (lus par le volet, le
// ruban et le dessin). Le classeur accepte les mêmes (internal/canvas/chart.go).

import type { IconName } from '#lib/icons/icons.js';

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
	label: string;
	icon: IconName;
	hint: string;
}

/** Tous les types, dans l'ordre du volet. */
export const KINDS: KindInfo[] = [
	{ kind: 'bar', label: 'Histogramme', icon: 'chart-bar', hint: 'Barres verticales' },
	{ kind: 'column', label: 'Barres horizontales', icon: 'chart-column', hint: 'Barres horizontales' },
	{ kind: 'line', label: 'Courbes', icon: 'chart-line', hint: 'Courbes' },
	{ kind: 'area', label: 'Aires', icon: 'chart-area', hint: 'Aires' },
	{ kind: 'pie', label: 'Secteurs', icon: 'chart-pie', hint: 'Secteurs' },
	{ kind: 'doughnut', label: 'Anneau', icon: 'chart-doughnut', hint: 'Anneau' },
	{ kind: 'scatter', label: 'Nuage de points', icon: 'chart-scatter', hint: 'Nuage de points' },
	{ kind: 'combo', label: 'Combiné', icon: 'chart-combo', hint: 'Histogramme et courbes sur le même graphique (axe secondaire possible)' },
	{ kind: 'bar3d', label: 'Histogramme 3D', icon: 'chart-bar3d', hint: 'Histogramme en perspective' },
	{ kind: 'pie3d', label: 'Secteurs 3D', icon: 'chart-pie3d', hint: 'Secteurs en perspective' },
	{ kind: 'map', label: 'Carte', icon: 'chart-map', hint: 'Points ou densité sur une carte OpenStreetMap, à partir de colonnes latitude et longitude' },
	{ kind: 'waterfall', label: 'Cascade', icon: 'chart-waterfall', hint: 'Hausses et baisses successives, jusqu’au total' },
	{ kind: 'treemap', label: 'Compartimentage', icon: 'chart-treemap', hint: 'Rectangles proportionnels aux valeurs (treemap)' },
	{ kind: 'radar', label: 'Radar', icon: 'chart-radar', hint: 'Une branche par catégorie, un polygone par série' },
	{ kind: 'funnel', label: 'Entonnoir', icon: 'chart-funnel', hint: 'Étapes successives d’un processus' },
	{ kind: 'boxplot', label: 'Boîte à moustaches', icon: 'chart-boxplot', hint: 'Médiane, quartiles et extrêmes de chaque série' },
	{ kind: 'histogram', label: 'Fréquences', icon: 'chart-histogram', hint: 'Histogramme de fréquences : répartition des valeurs par classes' },
	{ kind: 'gauge', label: 'Jauge', icon: 'chart-gauge', hint: 'Une valeur sur un cadran, entre un minimum et un maximum' }
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

const int = (v: number) => v.toLocaleString('fr-FR');

/** Coupure des données d'un graphique, à signaler sous le dessin et dans le volet (vide : aucune). */
export function chartNote(info?: { total?: number; truncated?: boolean; sampled?: boolean } | null): string {
	if (!info?.total) return '';
	if (info.truncated) return `${int(MAX_POINTS)} premiers points sur ${int(info.total)}`;
	if (info.sampled) return `Courbe réduite à ${int(MAX_POINTS)} points sur ${int(info.total)} (extrêmes gardés)`;
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
	if (neg && ['pie', 'doughnut', 'pie3d', 'treemap'].includes(cfg.kind)) out.push(neg === 1 ? '1 valeur négative ignorée.' : `${int(neg)} valeurs négatives ignorées.`);
	if (neg && cfg.kind === 'funnel') out.push(neg === 1 ? '1 valeur négative prise en valeur absolue.' : `${int(neg)} valeurs négatives prises en valeur absolue.`);
	if (cfg.kind === 'scatter' && info?.labels?.length && !info.xValues?.some((v) => v !== null)) out.push('L’axe X n’est ni un nombre ni une date : chaque point est placé à son rang.');
	if (SINGLE_SERIES.has(cfg.kind) && cfg.series.length > 1) out.push('Ce type ne dessine que la première série.');
	return out;
}

/** Résumé d'un graphique pour les lecteurs d'écran : type, séries, étendue. */
export function chartSummary(kind: string, labels: string[], series: { name: string; values: (number | null)[] }[], total?: number): string {
	const name = kindInfo(kind)?.label ?? 'Graphique';
	const parts = [`${name}, ${series.length > 1 ? `${series.length} séries` : '1 série'}`];
	if (labels.length) parts.push(`${int(total && total > labels.length ? total : labels.length)} points, de « ${labels[0]} » à « ${labels[labels.length - 1]} »`);
	for (const s of series.slice(0, 6)) {
		const vs = s.values.filter((v): v is number => v !== null);
		if (vs.length) parts.push(`${s.name} : de ${Math.min(...vs).toLocaleString('fr-FR')} à ${Math.max(...vs).toLocaleString('fr-FR')}`);
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
