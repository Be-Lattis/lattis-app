// Fermeture tolérante d'un menu flottant : il reste ouvert tant que la souris
// ne s'en éloigne pas de plus de `margin` pixels, et se ferme sur un clic
// ailleurs (y compris sur la grille, qui intercepte pointerdown, ou dans un
// éditeur intégré qui prend le focus de la fenêtre), sur Échap et quand la
// fenêtre change de taille (un simple mouseleave le fermait trop facilement).

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

/**
 * Garde un menu ou un panneau flottant dans la fenêtre : décalé vers le haut
 * ou la gauche s'il déborde en bas ou à droite, et avec sa propre barre de
 * défilement s'il est plus haut que la fenêtre. Recalculé quand son contenu
 * change de taille.
 */
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
		if (r0.height > window.innerHeight - 2 * EDGE_GAP) {
			node.style.maxHeight = `${(window.innerHeight - 2 * EDGE_GAP) / (k || 1)}px`;
			node.style.overflowY = 'auto';
		}
		const { dx, dy } = inside(node.getBoundingClientRect(), window.innerWidth, window.innerHeight);
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
	const move = (e: MouseEvent) => {
		// Un sous-menu qui déborde du menu compte comme le menu.
		if (!far || node.contains(e.target as Node)) return;
		if (distance(node.getBoundingClientRect(), e.clientX, e.clientY) > DISMISS_MARGIN) onClose();
	};
	const down = (e: MouseEvent) => {
		if (!node.contains(e.target as Node)) onClose();
	};
	const key = (e: KeyboardEvent) => {
		if (e.key === 'Escape') onClose();
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
