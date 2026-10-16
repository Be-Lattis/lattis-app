// Onglets de feuilles : feuilles visibles, rang d'un déplacement ou d'une
// copie (dialogue « Déplacer ou copier »), couleurs d'onglet.

export interface TabSheet {
	id: number;
	name: string;
	protected?: boolean;
	/** Couleur de l'onglet (« #rrggbb »), vide : aucune. */
	color?: string;
	/** Feuille masquée : absente de la barre d'onglets. */
	hidden?: boolean;
}

/** Couleurs d'onglet proposées (couleurs standard d'Excel). */
export const TAB_COLORS = ['#c00000', '#ff0000', '#ffc000', '#ffff00', '#92d050', '#00b050', '#00b0f0', '#0070c0', '#002060', '#7030a0'];

export function visibleSheets<T extends TabSheet>(sheets: T[]): T[] {
	return sheets.filter((s) => !s.hidden);
}

/** Feuille à afficher : current si elle existe et est visible, sinon la première visible. */
export function visibleSheet(sheets: TabSheet[], current: number): number {
	const s = sheets.find((x) => x.id === current);
	if (s && !s.hidden) return current;
	return (visibleSheets(sheets)[0] ?? sheets[0])?.id ?? 1;
}

/**
 * Rang final (0 : premier, parmi toutes les feuilles, masquées comprises)
 * d'une feuille déplacée ou de sa copie, placée avant la feuille before
 * (null : en dernier).
 */
export function moveTarget(order: number[], id: number, before: number | null, copy: boolean): number {
	const rest = copy ? order : order.filter((x) => x !== id);
	const k = before === null ? -1 : rest.indexOf(before);
	return k < 0 ? rest.length : k;
}

/** Couleur de texte lisible sur un fond (onglet coloré). */
export function textOn(hex: string): string {
	const m = /^#?([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex);
	if (!m) return '#1f2328';
	const [r, g, b] = m.slice(1).map((x) => parseInt(x, 16) / 255);
	// Luminance relative (sRGB simplifiée).
	const l = 0.2126 * r + 0.7152 * g + 0.0722 * b;
	return l > 0.55 ? '#1f2328' : '#ffffff';
}
