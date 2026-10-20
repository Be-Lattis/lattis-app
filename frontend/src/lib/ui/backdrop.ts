// Fond d'une boîte modale : un clic dessus la ferme, mais seulement si le
// bouton a été enfoncé ET relâché sur le fond. Un cliqué-glissé commencé dans
// la boîte (sélection d'un texte, curseur) et relâché dehors ne la ferme pas :
// le « click » d'un tel geste vise l'ancêtre commun, c'est-à-dire le fond.

/** use:backdrop={fermer} sur le fond (l'élément qui couvre l'écran autour de la boîte). */
export function backdrop(node: HTMLElement, close: () => unknown) {
	let fn = close;
	let downOnSelf = false;
	const down = (e: PointerEvent) => {
		downOnSelf = e.target === node;
	};
	const click = (e: MouseEvent) => {
		if (downOnSelf && e.target === node) fn();
		downOnSelf = false;
	};
	node.addEventListener('pointerdown', down);
	node.addEventListener('click', click);
	return {
		update(next: () => unknown) {
			fn = next;
		},
		destroy() {
			node.removeEventListener('pointerdown', down);
			node.removeEventListener('click', click);
		}
	};
}
