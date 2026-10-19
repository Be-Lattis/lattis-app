// Menus contextuels : largeur bornée, noms longs abrégés, sous-menus gardés
// dans la fenêtre (à gauche du menu quand la place manque à droite).

/** Largeur maximale d'un menu contextuel (px) ; au-delà, les libellés s'abrègent (…). */
export const MENU_MAX_WIDTH = 320;

/** Longueur au-delà de laquelle un nom (tableau, colonne, bouton…) est abrégé dans un menu. */
export const NAME_MAX = 30;

/** Nom abrégé à max caractères (le dernier devient « … ») ; tel quel s'il tient. */
export function clip(text: string | undefined | null, max = NAME_MAX): string {
	const s = (text ?? '').replace(/\s+/g, ' ').trim();
	const chars = Array.from(s);
	if (chars.length <= max) return s;
	return chars.slice(0, Math.max(1, max - 1)).join('').trimEnd() + '…';
}

/** Le nom est-il abrégé par clip (son infobulle donne alors le nom entier) ? */
export const clipped = (text: string | undefined | null, max = NAME_MAX) => clip(text, max) !== (text ?? '').trim();

type Box = { left: number; top: number; right: number; bottom: number };

/** Marge gardée entre un sous-menu et le bord de la zone visible. */
const GAP = 4;
/** Le sous-menu s'aligne sur son entrée, au padding du menu près. */
export const FLYOUT_TOP = -5;

/**
 * Place d'un sous-menu de taille w × h ouvert depuis l'entrée `item`, dans la
 * zone visible `box` : à droite de l'entrée, à gauche si la place manque à
 * droite (le côté le plus large si aucun ne suffit), et remonté de -dy pixels
 * s'il dépasse en bas (sans passer au-dessus du haut).
 */
export function flyout(item: Box, w: number, h: number, box: Box): { left: boolean; dy: number } {
	const right = box.right - GAP - item.right;
	const leftRoom = item.left - (box.left + GAP);
	const left = w > right && (w <= leftRoom || leftRoom > right);
	const top = item.top + FLYOUT_TOP;
	const over = top + h - (box.bottom - GAP);
	const dy = over > 0 ? -Math.min(over, Math.max(0, top - (box.top + GAP))) : 0;
	return { left, dy };
}
