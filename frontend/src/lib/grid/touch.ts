// Gestes tactiles de la grille (téléphone, tablette), traités par
// Sheet.svelte avec les Pointer Events (pointerType 'touch') : un doigt qui
// glisse fait défiler dans les deux sens, avec l'élan d'une liste native ;
// un appui bref choisit la cellule (sur la cellule active : la saisie) ;
// un appui long ouvre le menu contextuel (ui/longpress.ts) ; deux doigts
// pincent pour zoomer. La souris et le stylet gardent leur comportement.

import { clampZoom } from './zoom';

/** Déplacement (px de l'écran) en deçà duquel un appui reste un appui. */
export const TAP_SLOP = 10;
/** Durée d'un appui long (menu contextuel). */
export const LONG_PRESS_MS = 500;
/** Constante de temps (ms) de la décélération de l'élan, proche d'iOS. */
export const FLING_TAU = 325;
/** Vitesse (px/ms) sous laquelle l'élan s'arrête. */
export const MIN_SPEED = 0.02;
/** Vitesse maximale (px/ms) donnée à l'élan. */
export const MAX_SPEED = 8;
/** Fenêtre (ms) des derniers mouvements qui donnent la vitesse au lâcher. */
const WINDOW_MS = 100;

/** Le doigt a-t-il assez bougé pour que le geste soit un glissé ? */
export function isDrag(dx: number, dy: number, slop = TAP_SLOP): boolean {
	return Math.hypot(dx, dy) >= slop;
}

/** Vitesse du doigt (px/ms), mesurée sur ses derniers mouvements. */
export class Velocity {
	private samples: { t: number; x: number; y: number }[] = [];

	add(t: number, x: number, y: number) {
		this.samples.push({ t, x, y });
		while (this.samples.length > 2 && t - this.samples[0].t > WINDOW_MS) this.samples.shift();
	}

	/** Vitesse au lâcher (à t) : nulle si le doigt s'est arrêté avant de se lever. */
	get(t = this.samples.at(-1)?.t ?? 0): { vx: number; vy: number } {
		const s = this.samples;
		if (s.length < 2 || t - s[s.length - 1].t > WINDOW_MS / 2) return { vx: 0, vy: 0 };
		const a = s[0];
		const b = s[s.length - 1];
		const dt = b.t - a.t;
		if (dt <= 0) return { vx: 0, vy: 0 };
		const cap = (v: number) => Math.max(-MAX_SPEED, Math.min(MAX_SPEED, v));
		return { vx: cap((b.x - a.x) / dt), vy: cap((b.y - a.y) / dt) };
	}
}

/**
 * Élan pendant dt ms à la vitesse v (px/ms), freinage exponentiel : distance
 * parcourue et vitesse restante. La somme des distances tend vers v × tau.
 */
export function inertia(v: number, dt: number, tau = FLING_TAU): { d: number; v: number } {
	const k = Math.exp(-dt / tau);
	return { d: v * tau * (1 - k), v: v * k };
}

/** Zoom après un pincement : celui du début, dans le rapport des écarts entre les doigts. */
export function pinchZoom(z0: number, d0: number, d: number): number {
	if (!(d0 > 0) || !(d > 0)) return clampZoom(z0);
	return clampZoom(z0 * (d / d0));
}
