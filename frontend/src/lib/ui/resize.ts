// Volets qui se déplient à droite (graphique, tableau croisé, contrôles,
// requêtes, référence Lua, aide, volets des Documents) : tous ont la même
// largeur par défaut (500 px, choix de Louis), bornée à la moitié de la fenêtre (et rebornée quand la
// fenêtre change). Leur bord gauche se tire pour les élargir ou les
// rétrécir, comme les volets d'Excel ; la largeur choisie est gardée sur ce
// poste, par volet, sous la même borne. Un double-clic sur le bord revient à
// la largeur par défaut.

/** Zone de prise du bord gauche, en pixels. */
export const EDGE = 5;

/** Largeur par défaut d'un volet. */
export const DEFAULT_WIDTH = 500;
/** Part maximale de la fenêtre qu'un volet peut prendre. */
export const MAX_SHARE = 0.5;
/** Largeur minimale d'un volet. */
export const MIN_WIDTH = 220;

/** Largeur d'un volet : celle choisie (ou celle par défaut), au moins min, au plus la moitié de la fenêtre. */
export function panelWidth(w: number | undefined, viewport: number, min = MIN_WIDTH): number {
	return Math.round(Math.max(min, Math.min(w ?? DEFAULT_WIDTH, viewport * MAX_SHARE)));
}

type Options = { key: string; min?: number; /** Volet inclus dans un autre, qui se redimensionne à sa place. */ off?: boolean };

// « panel2 » : les largeurs tirées avant l'harmonisation à 500 px sont oubliées.
const storeKey = (key: string) => `lattis.panel2.${key}`;

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

/** Animations voulues : Affichage › Animations (gardé sur ce poste) et pas de mouvement réduit demandé par le système. */
export function animationsOn(): boolean {
	try {
		if (localStorage.getItem('lattis.animations') === 'off') return false;
	} catch {
		// stockage indisponible : animations par défaut
	}
	return !(typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches);
}

/** Le volet arrive par la droite (demande de Louis), comme ceux d'Excel. */
function slideIn(node: HTMLElement) {
	if (typeof node.animate !== 'function' || !animationsOn()) return;
	node.animate([{ transform: 'translateX(32px)', opacity: 0 }, { transform: 'none', opacity: 1 }], { duration: 180, easing: 'cubic-bezier(0.2, 0.8, 0.2, 1)' });
}

/**
 * Le volet repart par la droite : une copie figée, posée par-dessus à sa
 * place, glisse et s'efface pendant que la page reprend la largeur libérée.
 */
function slideOut(node: HTMLElement) {
	if (typeof node.animate !== 'function' || !node.isConnected || !animationsOn()) return;
	const r = node.getBoundingClientRect();
	if (!r.width || !r.height) return;
	const ghost = node.cloneNode(true) as HTMLElement;
	Object.assign(ghost.style, { position: 'fixed', left: `${r.left}px`, top: `${r.top}px`, width: `${r.width}px`, height: `${r.height}px`, margin: '0', zIndex: '1000', pointerEvents: 'none' });
	ghost.setAttribute('aria-hidden', 'true');
	document.body.appendChild(ghost);
	const a = ghost.animate([{ transform: 'none', opacity: 1 }, { transform: 'translateX(32px)', opacity: 0 }], { duration: 160, easing: 'cubic-bezier(0.4, 0, 1, 1)' });
	a.onfinish = a.oncancel = () => ghost.remove();
}

/** Action Svelte : donne à un volet de droite la largeur commune et le rend redimensionnable par son bord gauche. */
export function resizePanel(node: HTMLElement, opts: Options) {
	if (opts.off) return {};
	// Région du volet pour F6 (focus.ts).
	node.dataset.region ??= 'panel';
	let { key, min = MIN_WIDTH } = opts;
	// Largeur tirée par l'utilisateur, undefined : largeur par défaut.
	let chosen = load(key);
	// Largeur bordure comprise : celle qu'on mesure est celle qu'on pose.
	node.style.boxSizing = 'border-box';
	const apply = () => {
		const w = `${panelWidth(chosen, window.innerWidth, min)}px`;
		node.style.width = w;
		node.style.flexBasis = w;
	};
	apply();
	slideIn(node);

	const onEdge = (e: PointerEvent) => e.clientX - node.getBoundingClientRect().left <= EDGE;
	let drag: { x: number; w: number } | null = null;

	function move(e: PointerEvent) {
		if (drag) {
			chosen = panelWidth(drag.w + (drag.x - e.clientX), window.innerWidth, min);
			apply();
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
		// La prise du bord n'est pas un clic dans la page (qui fermerait le volet).
		e.preventDefault();
		e.stopPropagation();
	}
	function up(e: PointerEvent) {
		if (!drag) return;
		drag = null;
		node.releasePointerCapture(e.pointerId);
		document.body.style.userSelect = '';
		save(key, chosen);
	}
	function dbl(e: MouseEvent) {
		if (e.clientX - node.getBoundingClientRect().left > EDGE) return;
		chosen = undefined;
		apply();
		save(key, undefined);
	}
	function leave() {
		if (!drag) node.style.cursor = '';
	}
	function edgeDown(e: MouseEvent) {
		if (e.button === 0 && e.clientX - node.getBoundingClientRect().left <= EDGE) e.stopPropagation();
	}
	node.addEventListener('pointermove', move);
	node.addEventListener('pointerdown', down);
	node.addEventListener('mousedown', edgeDown);
	node.addEventListener('pointerup', up);
	node.addEventListener('pointercancel', up);
	node.addEventListener('pointerleave', leave);
	node.addEventListener('dblclick', dbl);
	window.addEventListener('resize', apply);
	return {
		update(o: Options) {
			if (o.key !== key) chosen = load(o.key);
			({ key, min = MIN_WIDTH } = o);
			apply();
		},
		destroy() {
			slideOut(node);
			node.removeEventListener('pointermove', move);
			node.removeEventListener('pointerdown', down);
			node.removeEventListener('mousedown', edgeDown);
			node.removeEventListener('pointerup', up);
			node.removeEventListener('pointercancel', up);
			node.removeEventListener('pointerleave', leave);
			node.removeEventListener('dblclick', dbl);
			window.removeEventListener('resize', apply);
		}
	};
}
