// Panneaux qui se déplient à droite (graphique, tableau croisé, requêtes,
// référence Lua…) : leur bord gauche se tire pour les élargir ou les
// rétrécir, comme les volets d'Excel. La largeur choisie est gardée sur ce
// poste, par panneau ; un double-clic sur le bord revient à la largeur
// d'origine.

/** Zone de prise du bord gauche, en pixels. */
export const EDGE = 5;

/** Largeur bornée : au moins min, au plus max et la place disponible. */
export function clampWidth(w: number, min: number, max: number, room: number): number {
	return Math.round(Math.max(min, Math.min(w, max, room)));
}

type Options = { key: string; min?: number; max?: number; /** Panneau inclus dans un autre, qui se redimensionne à sa place. */ off?: boolean };

const storeKey = (key: string) => `lattis.panel.${key}`;

function load(key: string): number | undefined {
	try {
		const v = Number(localStorage.getItem(storeKey(key)));
		return Number.isFinite(v) && v > 0 ? v : undefined;
	} catch {
		return undefined;
	}
}

function save(key: string, w: number | undefined) {
	try {
		if (w === undefined) localStorage.removeItem(storeKey(key));
		else localStorage.setItem(storeKey(key), String(w));
	} catch {
		// stockage indisponible : la largeur vaut pour la session
	}
}

/** Action Svelte : rend un panneau de droite redimensionnable par son bord gauche. */
export function resizePanel(node: HTMLElement, opts: Options) {
	if (opts.off) return {};
	let { key, min = 220, max = 900 } = opts;
	const room = () => Math.max(min, (node.parentElement?.clientWidth ?? window.innerWidth) - 160);
	// Largeur bordure comprise : celle qu'on mesure est celle qu'on pose.
	node.style.boxSizing = 'border-box';
	const apply = (w: number | undefined) => {
		node.style.width = w === undefined ? '' : `${w}px`;
		node.style.flexBasis = w === undefined ? '' : `${w}px`;
	};
	const saved = load(key);
	if (saved !== undefined) apply(clampWidth(saved, min, max, room()));

	const onEdge = (e: PointerEvent) => e.clientX - node.getBoundingClientRect().left <= EDGE;
	let drag: { x: number; w: number } | null = null;

	function move(e: PointerEvent) {
		if (drag) {
			apply(clampWidth(drag.w + (drag.x - e.clientX), min, max, room()));
			e.preventDefault();
			return;
		}
		node.style.cursor = onEdge(e) ? 'col-resize' : '';
	}
	function down(e: PointerEvent) {
		if (e.button !== 0 || !onEdge(e)) return;
		drag = { x: e.clientX, w: node.getBoundingClientRect().width };
		node.setPointerCapture(e.pointerId);
		document.body.style.userSelect = 'none';
		// La prise du bord n'est pas un clic dans la page (qui fermerait le panneau).
		e.preventDefault();
		e.stopPropagation();
	}
	function up(e: PointerEvent) {
		if (!drag) return;
		drag = null;
		node.releasePointerCapture(e.pointerId);
		document.body.style.userSelect = '';
		save(key, Math.round(node.getBoundingClientRect().width));
	}
	function dbl(e: MouseEvent) {
		if (e.clientX - node.getBoundingClientRect().left > EDGE) return;
		apply(undefined);
		save(key, undefined);
	}
	function leave() {
		if (!drag) node.style.cursor = '';
	}
	node.addEventListener('pointermove', move);
	node.addEventListener('pointerdown', down);
	node.addEventListener('mousedown', (e) => {
		if (e.button === 0 && e.clientX - node.getBoundingClientRect().left <= EDGE) e.stopPropagation();
	});
	node.addEventListener('pointerup', up);
	node.addEventListener('pointercancel', up);
	node.addEventListener('pointerleave', leave);
	node.addEventListener('dblclick', dbl);
	return {
		update(o: Options) {
			({ key, min = 220, max = 900 } = o);
		},
		destroy() {
			node.removeEventListener('pointermove', move);
			node.removeEventListener('pointerdown', down);
			node.removeEventListener('pointerup', up);
			node.removeEventListener('pointercancel', up);
			node.removeEventListener('pointerleave', leave);
			node.removeEventListener('dblclick', dbl);
		}
	};
}
