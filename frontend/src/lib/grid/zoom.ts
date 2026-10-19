// Zoom de la feuille (Affichage › Zoom, Ctrl+molette, curseur de la barre
// d'état), comme Excel : de 10 % à 400 %, enregistré par feuille.
//
// Mise en œuvre (Sheet.svelte) : la zone de la grille (canevas, zone de
// défilement, graphiques, boutons, éditeur de cellule, infobulles des
// notes) est dans un calque CSS `zoom`. Tous les calculs de la grille
// restent en pixels « de la feuille » (100 %) ; seuls changent la
// résolution du canevas (devicePixelRatio × zoom), la conversion des
// positions de la souris (toLocal) et la position des menus et dialogues,
// qui restent hors du calque et ne sont pas agrandis (toView). Les barres
// de défilement natives ne grossissent pas avec le zoom.

export const ZOOM_MIN = 0.1;
export const ZOOM_MAX = 4;

/** Niveaux proposés par le ruban. */
export const ZOOM_LEVELS = [0.5, 0.75, 0.9, 1, 1.1, 1.25, 1.5, 2];

/** Zoom borné et arrondi au pour cent. */
export function clampZoom(z: number): number {
	if (!Number.isFinite(z) || z <= 0) return 1;
	return Math.round(Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, z)) * 100) / 100;
}

/** « 125 % ». */
export function zoomLabel(z: number): string {
	return `${Math.round(z * 100)} %`;
}

/** Zoom saisi (« 120 », « 120 % », « 1,2 » n'est pas compris) ; null si illisible. */
export function parseZoom(s: string): number | null {
	const m = /^\s*(\d{1,3})\s*%?\s*$/.exec(s);
	if (!m) return null;
	const z = Number(m[1]) / 100;
	return z >= ZOOM_MIN && z <= ZOOM_MAX ? z : null;
}

/**
 * Zoom suivant (dir > 0) ou précédent : Ctrl+molette et boutons − / + de la
 * barre d'état, par pas de 10 % calés sur les dizaines (115 % → 120 %).
 */
export function stepZoom(z: number, dir: number): number {
	const pct = Math.round(z * 100);
	const next = dir > 0 ? Math.floor(pct / 10) * 10 + 10 : Math.ceil(pct / 10) * 10 - 10;
	return clampZoom(Math.max(ZOOM_MIN * 100, next) / 100);
}

/** Position du curseur de la barre d'état (0..1), 100 % au milieu comme Excel. */
export function sliderOf(z: number): number {
	return z <= 1 ? ((z - ZOOM_MIN) / (1 - ZOOM_MIN)) * 0.5 : 0.5 + ((z - 1) / (ZOOM_MAX - 1)) * 0.5;
}

/** Zoom d'une position du curseur (0..1). */
export function zoomOfSlider(t: number): number {
	const v = Math.min(1, Math.max(0, t));
	return clampZoom(v <= 0.5 ? ZOOM_MIN + (v / 0.5) * (1 - ZOOM_MIN) : 1 + ((v - 0.5) / 0.5) * (ZOOM_MAX - 1));
}
