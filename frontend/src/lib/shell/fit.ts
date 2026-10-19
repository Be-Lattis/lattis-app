// Ruban adaptatif, comme Excel : quand la place manque, les groupes se
// compactent l'un après l'autre en partant de la droite (grandes icônes
// avec libellé, puis petites icônes avec libellé sur trois lignes, puis
// petites icônes seules, puis un bouton qui ouvre le groupe en menu) ; quand
// il y a de la place, ils reprennent leur forme la plus large. Les groupes
// aux icônes parlantes (gras, alignement, bordures…) perdent leurs libellés
// en premier : ceux dont le libellé dit tout (Styles, Cellules, Édition) les
// gardent tant que c'est possible.
//
// Les largeurs suivent les styles de Ribbon.svelte (marges, icônes, polices :
// 11 px sous une grande icône et pour les titres, 12 px à côté d'une petite) ;
// les libellés ne passent jamais à la ligne, la largeur estimée est donc
// celle du ruban. measure donne la largeur réelle d'un texte (canevas) ; à
// défaut, une largeur moyenne par caractère.

export type Mode = 'large' | 'small' | 'icon' | 'collapsed';
export const MODES: Mode[] = ['large', 'small', 'icon', 'collapsed'];

export interface GroupShape {
	title: string;
	/**
	 * Libellés des commandes (avec leur « ▾ ») ; null pour une commande de
	 * taille fixe (couleur), un nombre pour un élément de largeur fixe
	 * (listes Police et Taille), qui occupe une colonne entière.
	 */
	labels: (string | null | number)[];
	/** Icônes assez parlantes pour se passer de libellé en premier. */
	iconOk?: boolean;
	/**
	 * Forme la plus large permise : 'icon' pour un groupe de petites icônes
	 * parlantes (Alignement, comme Excel), qui ne prend jamais de grands
	 * boutons à libellé, même quand la place abonde. Il se replie encore
	 * en menu quand la place manque.
	 */
	widest?: Mode;
	/**
	 * Groupe compact : icons commandes en petites icônes seules (en colonnes
	 * de trois) avant les commandes de labels, qui gardent leur forme du mode
	 * (grand bouton, petite icône à libellé) ; en mode icônes, tout est icône.
	 */
	icons?: number;
}

/** Largeur d'un texte en px à la taille de police size (Segoe UI). */
export type Measure = (s: string, size: number) => number;

/** Estimation sans canevas : largeur moyenne d'un caractère de Segoe UI. */
export const estimate: Measure = (s, size) => s.length * size * 0.48;

// Mesures des styles de Ribbon.svelte.
const GROUP_PAD = 17; // .group : padding 0 8 et séparateur
const TITLE_PAD = 24; // .title : padding 0 12
const GAP = 2; // .commands : gap
const LARGE_MIN = 54; // .cmd : min-width
const CMD_PAD = 14; // .cmd : padding 0 6 et bordure
const SMALL_ICON = 16 + 6; // petite icône et son écart au libellé
const FIXED = 56; // commande de couleur (bouton de 54 px et sa bordure)

/** Largeur estimée d'un groupe dans un mode. */
export function groupWidth(g: GroupShape, mode: Mode, measure: Measure = estimate): number {
	const text = (s: string, size: number) => Math.ceil(measure(s, size));
	const title = text(g.title, 11) + TITLE_PAD;
	if (mode === 'collapsed') return Math.max(LARGE_MIN, text(g.title + ' ▾', 11) + CMD_PAD) + GROUP_PAD;
	let w = 0;
	const iconCol = SMALL_ICON - 6 + CMD_PAD;
	if (g.icons) {
		if (mode === 'icon') return groupWidth({ ...g, icons: 0, labels: [...g.labels, ...Array<string>(g.icons).fill('')] }, mode, measure);
		w += Math.ceil(g.icons / 3) * (iconCol + GAP);
	}
	const fixed = g.labels.filter((l): l is number => typeof l === 'number');
	const labels = g.labels.filter((l): l is string | null => typeof l !== 'number');
	for (const f of fixed) w += f + GAP;
	if (mode === 'large') {
		for (const l of labels) w += (l === null ? FIXED : Math.max(LARGE_MIN, text(l, 11) + CMD_PAD)) + GAP;
	} else {
		// Colonnes de trois commandes.
		for (let i = 0; i < labels.length; i += 3) {
			let col = 0;
			for (const l of labels.slice(i, i + 3)) {
				const cw = l === null ? FIXED : mode === 'icon' ? iconCol : SMALL_ICON + text(l, 12) + CMD_PAD;
				col = Math.max(col, cw);
			}
			w += col + GAP;
		}
	}
	return Math.max(w - GAP, title) + GROUP_PAD;
}

/** Modes des groupes pour tenir dans width (le plus large possible). */
export function fit(groups: GroupShape[], width: number, measure: Measure = estimate): Mode[] {
	const modes: Mode[] = groups.map((g) => g.widest ?? 'large');
	const gw = (i: number, m: Mode) => groupWidth(groups[i], m, measure);
	const total = () => groups.reduce((s, _, i) => s + gw(i, modes[i]), 0);
	for (let level = 1; level < MODES.length && total() > width; level++) {
		// Sans libellés (icônes seules) : les groupes aux icônes parlantes d'abord.
		const passes = MODES[level] === 'icon' ? [true, false] : [null];
		for (const iconOk of passes) {
			for (let i = groups.length - 1; i >= 0 && total() > width; i--) {
				if (iconOk !== null && !!groups[i].iconOk !== iconOk) continue;
				// Petites icônes : sautées quand elles ne font rien gagner (un long
				// libellé seul est plus étroit en grand qu'en petit).
				if (MODES[level] === 'small' && gw(i, 'small') >= gw(i, modes[i])) continue;
				if (MODES.indexOf(modes[i]) < level) modes[i] = MODES[level];
			}
		}
	}
	return modes;
}
