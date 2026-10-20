// Téléphone et tablette (édition web) : appui long = clic droit, hauteur
// de l'application calée sur la partie visible (barres du navigateur et
// clavier à l'écran), barres qui défilent au doigt.

import { LONG_PRESS_MS, TAP_SLOP } from '#lib/grid/touch.js';

/** Écran étroit (téléphone) : ruban réduit par défaut, onglets qui défilent. */
export const NARROW = 600;

export function isNarrow(): boolean {
	return typeof window !== 'undefined' && window.innerWidth < NARROW;
}

/** Pointeur grossier (doigt) : cibles agrandies, barre horizontale de la grille inutile. */
export function isCoarse(): boolean {
	return typeof matchMedia !== 'undefined' && matchMedia('(pointer: coarse)').matches;
}

/**
 * Défilement (scrollLeft) qui montre l'élément [left, right] (en px du
 * contenu) dans une vue de largeur view, avec une marge pad ; inchangé s'il
 * se voit déjà.
 */
export function revealX(left: number, right: number, scroll: number, view: number, pad = 16): number {
	if (right - left + 2 * pad >= view) return Math.max(0, left - pad);
	if (left - pad < scroll) return Math.max(0, left - pad);
	if (right + pad > scroll + view) return right + pad - view;
	return scroll;
}

/**
 * Hauteur et décalage de l'application quand la partie visible de la page
 * (visualViewport) est plus petite que la fenêtre : clavier à l'écran (iOS,
 * Android récents), barres du navigateur. null : toute la fenêtre, et
 * aussi pendant un zoom de la page au doigt (scale > 1), qui ne doit pas
 * rétrécir l'application.
 */
export function viewportBox(vvHeight: number, vvTop: number, innerHeight: number, scale = 1): { h: number; top: number } | null {
	if (!(vvHeight > 0) || vvHeight >= innerHeight - 1 || scale > 1.01) return null;
	return { h: Math.round(vvHeight), top: Math.max(0, Math.round(vvTop)) };
}

/** Pose --app-h et --app-top (voir viewportBox) sur la racine ; rend de quoi arrêter. */
export function followViewport(): () => void {
	const vv = typeof window !== 'undefined' ? window.visualViewport : null;
	if (!vv) return () => {};
	const root = document.documentElement;
	const apply = () => {
		const b = viewportBox(vv.height, vv.offsetTop, window.innerHeight, vv.scale);
		if (b) {
			root.style.setProperty('--app-h', `${b.h}px`);
			root.style.setProperty('--app-top', `${b.top}px`);
		} else {
			root.style.removeProperty('--app-h');
			root.style.removeProperty('--app-top');
		}
	};
	vv.addEventListener('resize', apply);
	vv.addEventListener('scroll', apply);
	apply();
	return () => {
		vv.removeEventListener('resize', apply);
		vv.removeEventListener('scroll', apply);
	};
}

/** Cible où l'appui long garde son rôle natif (sélection du texte, loupe). */
function nativeTarget(t: Element): boolean {
	return !!t.closest('input, textarea, select, [contenteditable=""], [contenteditable="true"], .cm-editor');
}

/**
 * Appui long au doigt = clic droit : un « contextmenu » est envoyé à
 * l'élément touché (iOS n'en envoie pas ; Android, si, et le second est
 * ignoré), et le clic qui suit le lever du doigt est avalé. Rend de quoi
 * arrêter.
 */
export function installLongPress(): () => void {
	if (typeof window === 'undefined') return () => {};
	let press: { id: number; x: number; y: number; target: Element; timer: ReturnType<typeof setTimeout> } | null = null;
	let firedAt = -Infinity;
	let native = false;
	const cancel = () => {
		if (press) clearTimeout(press.timer);
		press = null;
	};
	const down = (e: PointerEvent) => {
		cancel();
		if (e.pointerType !== 'touch' || !e.isPrimary || !(e.target instanceof Element) || nativeTarget(e.target)) return;
		const target = e.target;
		native = false;
		press = {
			id: e.pointerId,
			x: e.clientX,
			y: e.clientY,
			target,
			timer: setTimeout(() => {
				const p = press;
				press = null;
				if (!p || native || !p.target.isConnected) return;
				firedAt = performance.now();
				p.target.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, composed: true, clientX: p.x, clientY: p.y, button: 2, buttons: 0, view: window }));
			}, LONG_PRESS_MS)
		};
	};
	const move = (e: PointerEvent) => {
		if (press && e.pointerId === press.id && Math.hypot(e.clientX - press.x, e.clientY - press.y) >= TAP_SLOP) cancel();
	};
	const up = (e: PointerEvent) => {
		if (press && e.pointerId === press.id) cancel();
	};
	const menu = (e: MouseEvent) => {
		if (!e.isTrusted) return;
		// Menu natif d'Android pendant l'appui : il remplace le nôtre ; juste après le nôtre : ignoré.
		if (press) {
			native = true;
			cancel();
		} else if (performance.now() - firedAt < 1000) {
			e.preventDefault();
			e.stopImmediatePropagation();
		}
	};
	const click = (e: MouseEvent) => {
		if (performance.now() - firedAt < 800) {
			firedAt = -Infinity;
			e.preventDefault();
			e.stopImmediatePropagation();
		}
	};
	const opts = { capture: true };
	window.addEventListener('pointerdown', down, opts);
	window.addEventListener('pointermove', move, opts);
	window.addEventListener('pointerup', up, opts);
	window.addEventListener('pointercancel', up, opts);
	window.addEventListener('contextmenu', menu, opts);
	window.addEventListener('click', click, opts);
	return () => {
		cancel();
		window.removeEventListener('pointerdown', down, opts);
		window.removeEventListener('pointermove', move, opts);
		window.removeEventListener('pointerup', up, opts);
		window.removeEventListener('pointercancel', up, opts);
		window.removeEventListener('contextmenu', menu, opts);
		window.removeEventListener('click', click, opts);
	};
}
