// Fermeture tolérante d'un menu flottant : il reste ouvert tant que la souris
// ne s'en éloigne pas de plus de `margin` pixels, et se ferme sur un clic
// ailleurs ou sur Échap (un simple mouseleave le fermait trop facilement).

export const DISMISS_MARGIN = 60;

/** Distance (px) d'un point à un rectangle (0 à l'intérieur). */
export function distance(r: { left: number; top: number; right: number; bottom: number }, x: number, y: number): number {
	const dx = Math.max(r.left - x, 0, x - r.right);
	const dy = Math.max(r.top - y, 0, y - r.bottom);
	return Math.hypot(dx, dy);
}

/** Action Svelte : `use:dismiss={close}`. */
export function dismiss(node: HTMLElement, close: () => void) {
	let onClose = close;
	const move = (e: MouseEvent) => {
		if (distance(node.getBoundingClientRect(), e.clientX, e.clientY) > DISMISS_MARGIN) onClose();
	};
	const down = (e: MouseEvent) => {
		if (!node.contains(e.target as Node)) onClose();
	};
	const key = (e: KeyboardEvent) => {
		if (e.key === 'Escape') onClose();
	};
	// Le clic qui ouvre le menu ne doit pas le refermer aussitôt.
	const t = setTimeout(() => {
		window.addEventListener('mousemove', move);
		window.addEventListener('mousedown', down, true);
		window.addEventListener('keydown', key, true);
	});
	return {
		update(c: () => void) {
			onClose = c;
		},
		destroy() {
			clearTimeout(t);
			window.removeEventListener('mousemove', move);
			window.removeEventListener('mousedown', down, true);
			window.removeEventListener('keydown', key, true);
		}
	};
}
