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

/** Largeur laissée à la grille quand plusieurs volets sont ouverts ensemble. */
export const GRID_MIN = 300;

/**
 * Largeurs de volets ouverts ensemble (assistant, requêtes, TCD…) : chacun
 * la sienne, et si leur somme ne laisse pas gridMin à la grille, tous
 * rétrécissent dans la même proportion, sans passer sous leur minimum.
 */
export function fitWidths(wanted: number[], mins: number[], viewport: number, gridMin = GRID_MIN): number[] {
	const total = wanted.reduce((a, w) => a + w, 0);
	const floor = mins.reduce((a, m) => a + m, 0);
	const avail = Math.max(viewport - gridMin, floor);
	if (total <= avail) return wanted.map(Math.round);
	const extra = total - floor;
	const k = extra > 0 ? (avail - floor) / extra : 0;
	return wanted.map((w, i) => Math.floor(mins[i] + Math.max(0, w - mins[i]) * k));
}

/** Largeur qu'un volet peut prendre à côté des autres (leurs largeurs others), en laissant gridMin à la grille. */
export function roomFor(others: number[], viewport: number, min = MIN_WIDTH, gridMin = GRID_MIN): number {
	return Math.max(min, viewport - gridMin - others.reduce((a, w) => a + w, 0));
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

// Volets ouverts : leurs largeurs se règlent ensemble (fitWidths), pour que
// la grille garde toujours GRID_MIN pixels.
type Open = { node: HTMLElement; wanted: () => number; min: () => number };
const opened = new Set<Open>();

/** Volet logé à côté de la grille (pas flottant ni masqué) : il compte dans la somme des largeurs. */
function docked(node: HTMLElement): boolean {
	if (!node.isConnected) return false;
	const st = getComputedStyle(node);
	return st.position !== 'fixed' && st.visibility !== 'hidden' && st.display !== 'none';
}

function setWidth(node: HTMLElement, w: number) {
	node.style.width = `${w}px`;
	node.style.flexBasis = `${w}px`;
}

/** Pose la largeur de chaque volet ouvert. */
function layoutPanels() {
	if (typeof window === 'undefined') return;
	const all = [...opened];
	const shared = all.filter((p) => docked(p.node));
	const widths = fitWidths(
		shared.map((p) => p.wanted()),
		shared.map((p) => p.min()),
		window.innerWidth
	);
	shared.forEach((p, i) => setWidth(p.node, widths[i]));
	for (const p of all) if (!shared.includes(p)) setWidth(p.node, p.wanted());
}

/** Largeurs actuelles des autres volets logés. */
const othersWidth = (node: HTMLElement) => [...opened].filter((p) => p.node !== node && docked(p.node)).map((p) => p.node.getBoundingClientRect().width);

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
	const me: Open = { node, wanted: () => panelWidth(chosen, window.innerWidth, min), min: () => min };
	opened.add(me);
	const apply = layoutPanels;
	apply();
	// Volet monté avant d'être logé (styles pas encore appliqués) : on revoit l'ensemble.
	requestAnimationFrame(layoutPanels);
	slideIn(node);

	const onEdge = (e: PointerEvent) => e.clientX - node.getBoundingClientRect().left <= EDGE;
	let drag: { x: number; w: number; room: number } | null = null;

	function move(e: PointerEvent) {
		if (drag) {
			// Tiré plus large que la place laissée par les autres volets : borné.
			chosen = Math.min(panelWidth(drag.w + (drag.x - e.clientX), window.innerWidth, min), drag.room);
			apply();
			e.preventDefault();
			return;
		}
		node.style.cursor = onEdge(e) ? 'col-resize' : '';
	}
	function down(e: PointerEvent) {
		if (e.button !== 0 || !onEdge(e)) return;
		drag = { x: e.clientX, w: node.getBoundingClientRect().width, room: docked(node) ? roomFor(othersWidth(node), window.innerWidth, min) : Infinity };
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
			opened.delete(me);
			// Les autres volets reprennent la place libérée (après le démontage).
			requestAnimationFrame(layoutPanels);
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
