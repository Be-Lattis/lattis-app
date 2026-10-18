// Défilement automatique pendant un glisser (sélection, poignée de
// recopie, déplacement par la bordure, pointage d'une plage dans une
// formule), comme Excel : la souris au bord de la zone des cellules, ou
// au-delà, fait défiler la feuille, d'autant plus vite qu'elle s'en
// éloigne ; la sélection suit.

/** Bande intérieure (px) où le défilement commence déjà : une fenêtre plein écran n'a pas d'au-delà en bas. */
export const EDGE = 6;
/** Vitesse au bord, en pixels par image (60 images/s). */
const MIN_SPEED = 3;
/** Gain par pixel au-delà du bord. */
const GAIN = 0.45;
/** Vitesse maximale, en pixels par image. */
const MAX_SPEED = 140;

/**
 * Vitesse de défilement (pixels par image de 1/60 s) pour une souris à dist
 * pixels au-delà du début de la bande du bord (0 : elle y entre) ; 0 en deçà.
 */
export function autoScrollSpeed(dist: number): number {
	if (!(dist >= 0)) return 0;
	return Math.min(MAX_SPEED, MIN_SPEED + dist * GAIN);
}

/**
 * Défilement signé sur un axe : pos est la souris, lo et hi les bords de la
 * zone qui défile (hi exclu). Négatif vers le haut (ou la gauche).
 */
export function edgeScroll(pos: number, lo: number, hi: number, edge = EDGE): number {
	if (hi - lo <= 2 * edge) return 0;
	if (pos < lo + edge) return -autoScrollSpeed(lo + edge - pos);
	if (pos >= hi - edge) return autoScrollSpeed(pos - (hi - edge));
	return 0;
}

/** Pas d'une image, ramené à la durée écoulée (dt en ms ; bornée pour un onglet en arrière-plan). */
export function frameStep(speed: number, dt: number): number {
	return speed * Math.min(Math.max(dt, 0), 64) / (1000 / 60);
}
