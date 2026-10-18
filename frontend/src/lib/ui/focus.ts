// Accessibilité clavier : régions parcourues par F6, boîtes de dialogue
// modales (focus piégé, focus initial, Échap, retour du focus), volets
// latéraux (Échap) et navigation dans les menus (flèches, première lettre).

import { keyChars } from './keytips.js';

// --- Dernière saisie : clavier ou pointeur ----------------------------------

let modality: 'key' | 'pointer' = 'pointer';
if (typeof window !== 'undefined') {
	window.addEventListener('keydown', (e) => !['Shift', 'Control', 'Alt', 'Meta'].includes(e.key) && (modality = 'key'), true);
	window.addEventListener('pointerdown', () => (modality = 'pointer'), true);
}
/** Le dernier geste est-il une touche (menu ouvert au clavier : focus sur sa première entrée) ? */
export const byKeyboard = () => modality === 'key';

// --- Éléments atteignables ---------------------------------------------------

const TABBABLE = 'button, [href], input, select, textarea, [tabindex], [contenteditable="true"]';

const shown = (el: HTMLElement) => el.getClientRects().length > 0 && getComputedStyle(el).visibility !== 'hidden';

/** Éléments atteignables par Tab dans root, dans l'ordre du document. */
export function tabbables(root: HTMLElement): HTMLElement[] {
	return [...root.querySelectorAll<HTMLElement>(TABBABLE)].filter(
		(el) => el.tabIndex >= 0 && !(el as HTMLButtonElement).disabled && !(el instanceof HTMLInputElement && el.type === 'hidden') && el.getAttribute('aria-hidden') !== 'true' && shown(el)
	);
}

/** Champ de saisie : les flèches et les lettres lui reviennent. */
export function isTextField(el: Element | null): boolean {
	if (!(el instanceof HTMLElement)) return false;
	if (el.isContentEditable || el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement) return true;
	return el instanceof HTMLInputElement && !['button', 'checkbox', 'radio', 'color', 'range', 'submit', 'reset', 'file'].includes(el.type);
}

/** Indice suivant (dir 1) ou précédent (-1) dans une liste de n, en boucle ; cur -1 : aucun. */
export function cycle(n: number, cur: number, dir: 1 | -1): number {
	if (n <= 0) return -1;
	if (cur < 0) return dir > 0 ? 0 : n - 1;
	return (cur + dir + n) % n;
}

/** Premier libellé après from (en boucle) qui commence par la touche, ou -1. */
export function letterIndex(labels: string[], from: number, key: string): number {
	const k = keyChars(key);
	if (!k) return -1;
	for (let d = 1; d <= labels.length; d++) {
		const i = (from + d + labels.length) % labels.length;
		if (keyChars(labels[i]).startsWith(k)) return i;
	}
	return -1;
}

// --- Régions (F6) -------------------------------------------------------------

/** Régions de la fenêtre, dans l'ordre de F6 (de haut en bas, la grille puis son volet). */
export const REGIONS = ['ribbon', 'formula', 'grid', 'panel', 'tabs', 'status'] as const;
export type Region = (typeof REGIONS)[number];

/** Région suivante (ou précédente) parmi celles présentes ; current null ou absente : la suivante dans l'ordre complet. */
export function nextRegion(present: Region[], current: Region | null, back = false): Region | null {
	const list = REGIONS.filter((r) => present.includes(r));
	if (!list.length) return null;
	const i = current ? list.indexOf(current) : -1;
	if (i >= 0) return list[cycle(list.length, i, back ? -1 : 1)];
	const at = current ? REGIONS.indexOf(current) : -1;
	if (back) return [...list].reverse().find((r) => REGIONS.indexOf(r) < at) ?? list[list.length - 1];
	return list.find((r) => REGIONS.indexOf(r) > at) ?? list[0];
}

/** Éléments des régions affichées (attribut data-region). */
function regionEls(): Map<Region, HTMLElement> {
	const m = new Map<Region, HTMLElement>();
	for (const el of document.querySelectorAll<HTMLElement>('[data-region]')) {
		const r = el.dataset.region as Region;
		if (!m.has(r) && REGIONS.includes(r) && shown(el)) m.set(r, el);
	}
	return m;
}

/** Région qui contient l'élément (la plus proche). */
export function regionOf(el: Element | null): Region | null {
	const r = (el instanceof HTMLElement ? el.closest<HTMLElement>('[data-region]') : null)?.dataset.region;
	return (r as Region) ?? null;
}

/** Donne le focus à une région : son élément marqué data-region-focus, sinon le premier atteignable. */
export function focusRegion(r: Region): boolean {
	const el = regionEls().get(r);
	if (!el) return false;
	const marked = [...el.querySelectorAll<HTMLElement>('[data-region-focus]')].find(shown);
	const target = marked ?? tabbables(el)[0] ?? el;
	if (target === el && el.tabIndex < 0 && !el.hasAttribute('tabindex')) el.tabIndex = -1;
	target.focus({ preventScroll: true });
	return true;
}

/** F6 (Maj+F6 : en arrière) : passe à la région suivante. */
export function cycleRegions(back: boolean): boolean {
	const present = [...regionEls().keys()];
	const active = document.activeElement;
	const cur = active && active !== document.body ? regionOf(active) : 'grid';
	// Un élément hors région (menu, page) compte comme la grille.
	const next = nextRegion(present, cur ?? 'grid', back);
	return next ? focusRegion(next) : false;
}

// --- Boîtes de dialogue modales ----------------------------------------------

const modals: HTMLElement[] = [];
/** Une boîte de dialogue modale est-elle ouverte (F6 et ruban au clavier suspendus) ? */
export const modalOpen = () => modals.length > 0 || !!document.querySelector('[aria-modal="true"]');

type ModalOpts = { onclose?: () => void } | (() => void) | undefined;

/**
 * Action `use:modal={close}` : focus initial (autofocus, sinon le premier
 * champ, sinon le premier bouton), Tab et Maj+Tab tournent dans la boîte,
 * Échap la ferme (si close est donné) et le focus revient où il était.
 */
export function modal(node: HTMLElement, opts: ModalOpts = undefined) {
	let close = typeof opts === 'function' ? opts : opts?.onclose;
	const prev = document.activeElement as HTMLElement | null;
	modals.push(node);
	if (!node.hasAttribute('tabindex')) node.tabIndex = -1;
	const t = setTimeout(() => {
		if (node.contains(document.activeElement)) return;
		const all = tabbables(node);
		const first = node.querySelector<HTMLElement>('[autofocus], [data-autofocus]') ?? all.find((el) => isTextField(el) && !(el as HTMLInputElement).readOnly) ?? all[0] ?? node;
		first.focus({ preventScroll: true });
	});
	const key = (e: KeyboardEvent) => {
		if (modals[modals.length - 1] !== node) return;
		if (e.key === 'Escape' && close && !e.defaultPrevented) {
			e.preventDefault();
			e.stopPropagation();
			close();
			return;
		}
		if (e.key !== 'Tab' || e.ctrlKey || e.altKey || e.defaultPrevented) return; // Tab pris par un éditeur (indentation)
		const all = tabbables(node);
		if (!all.length) {
			e.preventDefault();
			return;
		}
		const i = all.indexOf(document.activeElement as HTMLElement);
		if (i < 0 || (e.shiftKey && i === 0) || (!e.shiftKey && i === all.length - 1)) {
			e.preventDefault();
			all[e.shiftKey ? all.length - 1 : 0].focus();
		}
	};
	// Le focus ne repart pas vers la fenêtre derrière (clic, F6, programme).
	const guard = (e: FocusEvent) => {
		if (modals[modals.length - 1] !== node) return;
		const tgt = e.target as HTMLElement;
		if (node.contains(tgt) || !regionOf(tgt)) return;
		(tabbables(node)[0] ?? node).focus({ preventScroll: true });
	};
	node.addEventListener('keydown', key);
	document.addEventListener('focusin', guard);
	return {
		update(o: ModalOpts) {
			close = typeof o === 'function' ? o : o?.onclose;
		},
		destroy() {
			clearTimeout(t);
			node.removeEventListener('keydown', key);
			document.removeEventListener('focusin', guard);
			const k = modals.indexOf(node);
			if (k >= 0) modals.splice(k, 1);
			restore(node, prev);
		}
	};
}

/** Rend le focus à prev si le nœud qui se ferme l'avait (ou si plus rien ne l'a). */
export function restore(node: HTMLElement, prev: HTMLElement | null) {
	const a = document.activeElement;
	if (!prev || !prev.isConnected || node.contains(prev)) return;
	if (a && a !== document.body && !node.contains(a)) return;
	prev.focus({ preventScroll: true });
}

// --- Volets latéraux ------------------------------------------------------------

/**
 * Action `use:panelKeys={close}` sur un volet : Échap (s'il n'est pas déjà
 * traité plus bas, par une liste ou un menu) le ferme et rend la main à la grille.
 */
export function panelKeys(node: HTMLElement, close: (() => void) | undefined) {
	let onClose = close;
	const key = (e: KeyboardEvent) => {
		if (e.key !== 'Escape' || e.defaultPrevented || !onClose) return;
		e.preventDefault();
		e.stopPropagation();
		onClose();
		requestAnimationFrame(() => focusRegion('grid'));
	};
	node.addEventListener('keydown', key);
	return {
		update(c: (() => void) | undefined) {
			onClose = c;
		},
		destroy() {
			node.removeEventListener('keydown', key);
		}
	};
}

// --- Menus ------------------------------------------------------------------

/** Entrées d'un menu, dans l'ordre, sans les désactivées ni les masquées. */
export function menuItems(node: HTMLElement): HTMLElement[] {
	return [...node.querySelectorAll<HTMLElement>('button, [role^="menuitem"], a[href], input[type="checkbox"], input[type="radio"], input[type="color"]')].filter(
		(el) => !(el as HTMLButtonElement).disabled && el.getAttribute('aria-disabled') !== 'true' && shown(el)
	);
}

const menus: HTMLElement[] = [];

/**
 * Clavier d'un menu flottant (dismiss.ts) : ↑ ↓ Début Fin, Entrée ou Espace,
 * première lettre ; ouvert au clavier, sa première entrée prend le focus ;
 * fermé, le focus revient où il était. Rend la fonction de nettoyage.
 */
export function menuKeys(node: HTMLElement): () => void {
	const prev = document.activeElement as HTMLElement | null;
	if (!node.hasAttribute('role')) node.setAttribute('role', 'menu');
	menus.push(node);
	const t = setTimeout(() => {
		if (byKeyboard() && !node.contains(document.activeElement)) menuItems(node)[0]?.focus({ preventScroll: true });
	});
	const key = (e: KeyboardEvent) => {
		if (menus[menus.length - 1] !== node || e.ctrlKey || e.altKey || e.metaKey) return;
		const a = document.activeElement as HTMLElement | null;
		if (node.contains(a) && isTextField(a)) return; // saisie dans le menu
		if (a?.getAttribute('aria-haspopup') === 'listbox') return; // liste déroulante (Select) : elle a ses touches
		const items = menuItems(node);
		if (!items.length) return;
		const cur = items.indexOf(a as HTMLElement);
		let to = -1;
		switch (e.key) {
			case 'ArrowDown':
				to = cycle(items.length, cur, 1);
				break;
			case 'ArrowUp':
				to = cycle(items.length, cur, -1);
				break;
			case 'Home':
				to = 0;
				break;
			case 'End':
				to = items.length - 1;
				break;
			case 'Enter':
			case ' ':
				if (cur < 0) return;
				e.preventDefault();
				e.stopPropagation();
				items[cur].click();
				return;
			default:
				if (e.key.length !== 1 || e.shiftKey) return;
				to = letterIndex(
					items.map((el) => el.getAttribute('aria-label') || el.textContent?.trim() || ''),
					cur,
					e.key
				);
				if (to < 0) return;
		}
		e.preventDefault();
		e.stopPropagation();
		for (const el of items) if (!el.hasAttribute('role') && el instanceof HTMLButtonElement) el.setAttribute('role', 'menuitem');
		items[to].focus({ preventScroll: false });
	};
	window.addEventListener('keydown', key, true);
	return () => {
		clearTimeout(t);
		window.removeEventListener('keydown', key, true);
		const k = menus.indexOf(node);
		if (k >= 0) menus.splice(k, 1);
		restore(node, prev);
	};
}

/** Le menu est-il le dernier ouvert (Échap ne ferme que lui) ? */
export const topMenu = (node: HTMLElement) => !menus.length || menus[menus.length - 1] === node;
