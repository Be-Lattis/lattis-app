// Ruban adaptatif, comme Excel : quand la place manque, les groupes se
// compactent l'un après l'autre en partant de la droite (grandes icônes
// avec libellé, puis petites icônes avec libellé sur trois lignes, puis
// petites icônes seules, puis un bouton qui ouvre le groupe en menu) ; quand
// il y a de la place, ils reprennent leur forme la plus large.

export type Mode = 'large' | 'small' | 'icon' | 'collapsed';
export const MODES: Mode[] = ['large', 'small', 'icon', 'collapsed'];

export interface GroupShape {
	title: string;
	/** Libellés des commandes ; null pour une commande de taille fixe (couleur). */
	labels: (string | null)[];
}

const CHAR = 6.4; // largeur moyenne d'un caractère à 11-12 px
const GROUP_PAD = 17; // marges et séparateur d'un groupe
const FIXED = 40; // commande de taille fixe (couleur avec son ▾)

const text = (s: string) => Math.ceil(s.length * CHAR);

/** Largeur estimée d'un groupe dans un mode. */
export function groupWidth(g: GroupShape, mode: Mode): number {
	const title = text(g.title) + 16;
	if (mode === 'collapsed') return Math.max(64, title) + GROUP_PAD;
	let w = 0;
	if (mode === 'large') {
		for (const l of g.labels) w += (l === null ? FIXED : Math.max(54, text(l) + 14)) + 2;
	} else {
		// Colonnes de trois commandes.
		for (let i = 0; i < g.labels.length; i += 3) {
			let col = 0;
			for (const l of g.labels.slice(i, i + 3)) {
				const cw = l === null ? FIXED : mode === 'icon' ? 28 : 28 + text(l) + 8;
				col = Math.max(col, cw);
			}
			w += col + 2;
		}
	}
	return Math.max(w, title) + GROUP_PAD;
}

/** Modes des groupes pour tenir dans width (le plus large possible). */
export function fit(groups: GroupShape[], width: number): Mode[] {
	const modes: Mode[] = groups.map(() => 'large');
	const total = () => groups.reduce((s, g, i) => s + groupWidth(g, modes[i]), 0);
	for (let level = 1; level < MODES.length && total() > width; level++) {
		for (let i = groups.length - 1; i >= 0 && total() > width; i--) {
			if (MODES.indexOf(modes[i]) < level) modes[i] = MODES[level];
		}
	}
	return modes;
}
