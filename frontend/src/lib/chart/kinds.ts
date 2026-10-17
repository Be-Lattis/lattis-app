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

/** Forme d'une série d'un graphique combiné : la première en barres, les autres en courbes. */
export function seriesShape(kinds: string[] | undefined, k: number): 'bar' | 'line' | 'area' {
	const s = kinds?.[k];
	return s === 'bar' || s === 'line' || s === 'area' ? s : k === 0 ? 'bar' : 'line';
}

/** Colonnes d'une carte, devinées d'après leur nom : latitude, longitude. */
export function guessLatLon(names: string[]): { lat?: string; lon?: string } {
	const find = (re: RegExp) => names.find((n) => re.test(n.trim()));
	return {
		lat: find(/^(lat|latitude|lat\.)$/i) ?? find(/lat/i),
		lon: find(/^(lon|lng|long|longitude|lon\.)$/i) ?? find(/(lon|lng)/i)
	};
}
