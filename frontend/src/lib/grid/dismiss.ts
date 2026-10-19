// Fermeture tolérante d'un menu flottant : il reste ouvert tant que la souris
// ne s'en éloigne pas de plus de `margin` pixels, et se ferme sur un clic
// ailleurs (y compris sur la grille, qui intercepte pointerdown, ou dans un
// éditeur intégré qui prend le focus de la fenêtre), sur Échap et quand la
// fenêtre change de taille (un simple mouseleave le fermait trop facilement).
// Un menu se parcourt aussi au clavier (focus.ts : flèches, première lettre),
// et le focus revient où il était quand il se ferme.

import { menuKeys, topMenu } from '#lib/ui/focus.js';

export const DISMISS_MARGIN = 60;

const CAPTURE = { capture: true };

/** Distance (px) d'un point à un rectangle (0 à l'intérieur). */
export function distance(r: { left: number; top: number; right: number; bottom: number }, x: number, y: number): number {
	const dx = Math.max(r.left - x, 0, x - r.right);
	const dy = Math.max(r.top - y, 0, y - r.bottom);
	return Math.hypot(dx, dy);
}

/** Action Svelte : `use:dismiss={close}`. */
export function dismiss(node: HTMLElement, close: () => void) {
	return watch(node, close, true);
}

/**
 * Panneau ou boîte ancrée sur la grille (filtre, règles, note, taille…) :
 * comme un menu, il se ferme sur un clic ailleurs, Échap, ou quand la
 * fenêtre change de taille, mais pas quand la souris s'en éloigne (on y
 * saisit des valeurs). `use:dismissPanel={close}`.
 */
export function dismissPanel(node: HTMLElement, close: () => void) {
	return watch(node, close, false);
}

/** Marge gardée entre un menu et le bord de la fenêtre. */
const EDGE_GAP = 4;

/**
 * Décalage (en pixels de la fenêtre) qui ramène un rectangle dans la
 * fenêtre : vers le haut s'il dépasse en bas, vers la gauche s'il dépasse à
 * droite, sans passer au-delà du haut ni de la gauche.
 */
export function inside(r: { left: number; top: number; right: number; bottom: number }, w: number, h: number): { dx: number; dy: number } {
	const dx = Math.min(Math.max(0, r.right - (w - EDGE_GAP)), Math.max(0, r.left - EDGE_GAP));
	const dy = Math.min(Math.max(0, r.bottom - (h - EDGE_GAP)), Math.max(0, r.top - EDGE_GAP));
	return { dx, dy };
}

/** Degrés d'effacement d'un menu trop haut (attribut data-shed). */
export const SHED_LEVELS = [1, 2, 3];

/**
 * Menu plus haut que la fenêtre : retire ses entrées les moins utiles, par
 * degrés (data-shed="1" d'abord : accessible par le ruban ou rare ; puis
 * "2" ; puis "3" : courant mais doublé d'un raccourci), jusqu'à ce qu'il
 * rentre, puis les séparateurs devenus inutiles. Les entrées sans data-shed
 * restent toujours. Rend vrai s'il rentre.
 */
function shed(node: HTMLElement, avail: number): boolean {
	for (const el of node.querySelectorAll<HTMLElement>('[data-shed-off]')) {
		el.style.display = '';
		el.removeAttribute('data-shed-off');
	}
	const fits = () => node.getBoundingClientRect().height <= avail;
	if (fits()) return true;
	for (const level of SHED_LEVELS) {
		const items = node.querySelectorAll<HTMLElement>(`[data-shed="${level}"]`);
		if (!items.length) continue;
		for (const el of items) hide(el);
		tidySeparators(node);
		if (fits()) return true;
	}
	return false;
}

function hide(el: HTMLElement) {
	if (el.style.display === 'none') return;
	el.style.display = 'none';
	el.setAttribute('data-shed-off', '');
}

/** Masque les séparateurs en tête, en fin ou à la suite d'un autre. */
function tidySeparators(node: HTMLElement) {
	let prev: 'start' | 'hr' | 'item' = 'start';
	let last: HTMLElement | null = null;
	for (const el of node.children) {
		if (!(el instanceof HTMLElement) || el.style.display === 'none') continue;
		if (el.tagName === 'HR') {
			if (prev !== 'item') hide(el);
			else {
				prev = 'hr';
				last = el;
			}
		} else if (!el.classList.contains('menu-title')) prev = 'item';
	}
	if (prev === 'hr' && last) hide(last);
}

/**
 * Garde un menu ou un panneau flottant dans la fenêtre : décalé vers le haut
 * ou la gauche s'il déborde en bas ou à droite, rétréci par degrés s'il est
 * plus haut que la fenêtre (shed), et en dernier recours avec sa propre
 * barre de défilement. Recalculé quand son contenu change de taille.
 */
/**
 * Partie visible pour un nœud : la fenêtre, réduite par ses parents qui
 * coupent ce qui dépasse (overflow autre que visible), comme la grille.
 */
export function visibleBox(node: HTMLElement): { left: number; top: number; right: number; bottom: number } {
	const b = { left: 0, top: 0, right: window.innerWidth, bottom: window.innerHeight };
	// Un nœud fixe n'est coupé par personne ; un nœud absolu, seulement à partir
	// de son bloc conteneur.
	const pos = getComputedStyle(node).position;
	const first = pos === 'fixed' ? null : pos === 'absolute' ? (node.offsetParent as HTMLElement | null) : node.parentElement;
	for (let el = first; el && el !== document.body; el = el.parentElement) {
		const st = getComputedStyle(el);
		if (st.overflowX === 'visible' && st.overflowY === 'visible') continue;
		const r = el.getBoundingClientRect();
		b.left = Math.max(b.left, r.left);
		b.top = Math.max(b.top, r.top);
		b.right = Math.min(b.right, r.right);
		b.bottom = Math.min(b.bottom, r.bottom);
	}
	return b;
}

function keepInside(node: HTMLElement): () => void {
	if (typeof requestAnimationFrame === 'undefined' || typeof window === 'undefined') return () => {};
	let busy = false;
	const fit = () => {
		if (busy || !node.isConnected) return;
		busy = true;
		node.style.translate = '';
		const r0 = node.getBoundingClientRect();
		// Zoom ou transformation d'un parent : pixels de la fenêtre → pixels du nœud.
		const k = node.offsetWidth ? r0.width / node.offsetWidth : 1;
		const box = visibleBox(node);
		const avail = box.bottom - box.top - 2 * EDGE_GAP;
		if (r0.height > avail && !shed(node, avail)) {
			node.style.maxHeight = `${avail / (k || 1)}px`;
			node.style.overflowY = 'auto';
		}
		const r = node.getBoundingClientRect();
		const { dx, dy } = inside(
			{ left: r.left - box.left, top: r.top - box.top, right: r.right - box.left, bottom: r.bottom - box.top },
			box.right - box.left,
			box.bottom - box.top
		);
		if (dx || dy) node.style.translate = `${-dx / (k || 1)}px ${-dy / (k || 1)}px`;
		requestAnimationFrame(() => (busy = false));
	};
	const raf = requestAnimationFrame(fit);
	const ro = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(() => requestAnimationFrame(fit));
	ro?.observe(node);
	return () => {
		cancelAnimationFrame(raf);
		ro?.disconnect();
	};
}

function watch(node: HTMLElement, close: () => void, far: boolean) {
	let onClose = close;
	const unfit = keepInside(node);
	// Menu (pas un panneau, ni une liste qui a ses propres touches) : clavier.
	const unkeys = far && typeof document !== 'undefined' && node.getAttribute('role') !== 'listbox' && !node.hasAttribute('data-own-keys') ? menuKeys(node) : null;
	const move = (e: MouseEvent) => {
		// Un sous-menu qui déborde du menu compte comme le menu.
		if (!far || node.contains(e.target as Node)) return;
		if (distance(node.getBoundingClientRect(), e.clientX, e.clientY) > DISMISS_MARGIN) onClose();
	};
	const down = (e: MouseEvent) => {
		if (!node.contains(e.target as Node)) onClose();
	};
	const key = (e: KeyboardEvent) => {
		// Menus imbriqués (palette d'un groupe replié) : Échap ferme le dernier.
		if (e.key !== 'Escape' || (unkeys && !topMenu(node))) return;
		e.preventDefault();
		onClose();
	};
	const away = () => onClose();
	// Le clic qui ouvre le menu ne doit pas le refermer aussitôt.
	const t = setTimeout(() => {
		window.addEventListener('mousemove', move);
		window.addEventListener('mousedown', down, CAPTURE);
		window.addEventListener('pointerdown', down, CAPTURE);
		window.addEventListener('keydown', key, CAPTURE);
		window.addEventListener('resize', away);
		// Un panneau reste ouvert quand on passe à une autre fenêtre (saisie en cours).
		if (far) window.addEventListener('blur', away);
	});
	return {
		update(c: () => void) {
			onClose = c;
		},
		destroy() {
			unfit();
			unkeys?.();
			clearTimeout(t);
			window.removeEventListener('mousemove', move);
			window.removeEventListener('mousedown', down, CAPTURE);
			window.removeEventListener('pointerdown', down, CAPTURE);
			window.removeEventListener('keydown', key, CAPTURE);
			window.removeEventListener('resize', away);
			window.removeEventListener('blur', away);
		}
	};
}
