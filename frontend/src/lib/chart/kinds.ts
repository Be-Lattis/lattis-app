// Types de graphiques : libellés, icônes et familles (lus par le volet, le
// ruban et le dessin). Le classeur accepte les mêmes (internal/canvas/chart.go).

import type { IconName } from '#lib/icons/icons.js';

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
