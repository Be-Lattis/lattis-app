// Volet d'aide : ouvert ou non, et la cible demandée. Tout le monde peut
// l'ouvrir (F1, Fichier › Aide, bouton « ? » du ruban et des volets) sans
// passer par la page : HelpPanel suit cet état.

import { HOME } from './help';
import { modalOpen } from '#lib/ui/focus.js';

// last : dernière page affichée, où l'aide rouvre sans cible. over : posée
// au-dessus d'une boîte modale (ouverte depuis elle), au lieu d'être logée.
export const helpState = $state({ open: false, target: HOME, seq: 0, focus: true, last: HOME, over: false });

/**
 * Ouvre l'aide sur une cible (« sql », « fonction:SOMME »…) ; sans cible : là
 * où elle en était. focus : la recherche prend le focus (pas quand F1 est
 * pressé pendant une saisie, qui doit continuer).
 */
export function openHelp(target?: string, focus = true) {
	helpState.target = target || helpState.last;
	helpState.focus = focus;
	// Depuis une boîte modale (F1, « ? ») : l'aide passe au-dessus d'elle.
	helpState.over = typeof document !== 'undefined' && modalOpen();
	helpState.open = true;
	helpState.seq++;
}

export function closeHelp() {
	helpState.open = false;
}
