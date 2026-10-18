// Pipette de l'onglet SCRIPT : « Choisir sur la feuille » bascule sur la
// grille ; un clic sur un contrôle (feuille ou Onglet Document) le désigne,
// Échap annule. Les contrôles appellent pickCapture sur pointerdown,
// mousedown et click (phase de capture) : pendant la pipette, le clic ne
// déclenche ni l'action du contrôle ni sa sélection.

export const picker = $state({ active: false });

let resolve: ((id: number | null) => void) | null = null;
// Le clic qui suit le mousedown qui a choisi est avalé lui aussi.
let swallowUntil = 0;

function onKey(ev: KeyboardEvent) {
	if (ev.key !== 'Escape') return;
	ev.preventDefault();
	ev.stopPropagation();
	finishPick(null);
}

/** Lance la pipette ; la promesse donne l'identifiant du contrôle cliqué, ou null (Échap, Annuler). */
export function startPick(): Promise<number | null> {
	finishPick(null);
	picker.active = true;
	window.addEventListener('keydown', onKey, true);
	return new Promise((r) => (resolve = r));
}

export function finishPick(id: number | null) {
	if (!picker.active) return;
	picker.active = false;
	window.removeEventListener('keydown', onKey, true);
	const r = resolve;
	resolve = null;
	r?.(id);
}

/** À brancher en capture sur un contrôle : pendant la pipette, le désigne et arrête l'évènement. */
export function pickCapture(ev: Event, id: number) {
	if (!picker.active && Date.now() > swallowUntil) return;
	ev.preventDefault();
	ev.stopPropagation();
	if (!picker.active) return;
	swallowUntil = Date.now() + 600;
	finishPick(id);
}
