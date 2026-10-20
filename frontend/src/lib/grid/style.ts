// Mise en forme d'une cellule (même JSON que le classeur, voir
// internal/canvas/styles.go).
//
// Un seul système, en couches de la moins à la plus précise (comme Excel :
// cellule > ligne > colonne) : colonne de la feuille, ligne de la feuille,
// colonne d'un tableau, cellule (de tableau, libre ou de tableau croisé),
// puis mise en forme conditionnelle. Une propriété à null dans une couche
// la retire explicitement (elle masque les couches au-dessous).
import { numberFormat } from '#lib/i18n/index.js';

export interface CellStyle {
	b?: boolean; // gras
	i?: boolean; // italique
	u?: boolean; // souligné
	x?: boolean; // barré
	ff?: string; // police (famille)
	fs?: number; // taille en points
	fg?: string; // couleur du texte
	bg?: string; // couleur de remplissage
	al?: 'left' | 'center' | 'right';
	va?: 'top' | 'middle' | 'bottom'; // alignement vertical (milieu par défaut)
	wr?: boolean; // renvoyer à la ligne
	/** Format : nombre ('0', '0,00'), €, %, date, heure ou texte (voir numfmt.ts). */
	nf?: '0' | '0,00' | '%' | '€' | 'date' | 'time' | 'text';
	dp?: number; // décimales imposées (Ajouter / Réduire les décimales)
	ng?: boolean; // nombre sans séparateur de milliers
	df?: string; // motif d'une date ou d'une heure (« jj/mm/aaaa », « hh:mm »)
	bt?: boolean; // bordures haut, bas, gauche, droite
	bb?: boolean;
	bl?: boolean;
	br?: boolean;
	bc?: string; // couleur des bordures
	// Hors mise en forme (ni effacées ni reproduites, voir internal/canvas/cellformat.go).
	nt?: string; // note
	np?: boolean; // note épinglée (toujours affichée)
	dv?: string[] | { src: string }; // validation : valeurs permises (liste déroulante), ou plage source (relue)
	mr?: number; // fusion : lignes et colonnes couvertes (cellule en haut à gauche)
	mc?: number;
	hd?: boolean; // ligne ou colonne masquée (styles de lignes et de colonnes)
}

/** Propriétés de mise en forme (format de pinceau, effacement). */
export const STYLE_KEYS = ['b', 'i', 'u', 'x', 'ff', 'fs', 'fg', 'bg', 'al', 'va', 'wr', 'nf', 'dp', 'ng', 'df', 'bt', 'bb', 'bl', 'br', 'bc'] as const;

/** Couche de style telle qu'enregistrée : null retire la propriété. */
export type StyleLayer = { [K in keyof CellStyle]?: CellStyle[K] | null };

export function parseStyle(s: string | undefined | null): StyleLayer | undefined {
	if (!s) return undefined;
	try {
		return JSON.parse(s) as StyleLayer;
	} catch {
		return undefined;
	}
}

/**
 * Style effectif d'une cellule : les couches, de la moins à la plus précise,
 * se superposent propriété par propriété ; les retraits (null, false, "")
 * disparaissent du résultat. undefined sans aucune propriété.
 */
export function resolveStyle(...layers: (StyleLayer | undefined | null)[]): CellStyle | undefined {
	let acc: Record<string, unknown> | undefined;
	for (const l of layers) if (l) acc = acc ? { ...acc, ...l } : { ...l };
	if (!acc) return undefined;
	for (const k of Object.keys(acc)) if (acc[k] === null || acc[k] === false || acc[k] === '' || acc[k] === undefined) delete acc[k];
	return Object.keys(acc).length ? (acc as CellStyle) : undefined;
}

const EDGES = ['bt', 'bb', 'bl', 'br', 'bc'] as const;

/**
 * Style d'une cellule faite de plusieurs cellules (grid[ligne][colonne]) :
 * cellule de la bande des groupes d'un tableau sur plusieurs colonnes, ou
 * colonne sur un niveau (bande et en-tête réunis). La mise en forme posée sur
 * la sélection va à chaque cellule ; à l'écran, la première cellule qui
 * définit une propriété l'emporte (en haut à gauche d'abord), et chaque bord
 * vient des cellules de ce côté : le haut de la première ligne, la droite de
 * la dernière colonne… (un Contour posé sur la sélection fait le tour de la
 * cellule entière).
 */
export function boxStyle(grid: readonly (readonly (CellStyle | undefined)[])[]): CellStyle | undefined {
	const rows = grid.length;
	const cols = grid[0]?.length ?? 0;
	if (!rows || !cols) return undefined;
	const out: Record<string, unknown> = {};
	const all = grid.flat();
	for (let k = all.length - 1; k >= 0; k--) {
		const s = all[k];
		if (!s) continue;
		for (const [key, v] of Object.entries(s)) if (!(EDGES as readonly string[]).includes(key)) out[key] = v;
	}
	const edge = {
		bt: grid[0].filter((s) => s?.bt),
		bb: grid[rows - 1].filter((s) => s?.bb),
		bl: grid.map((row) => row[0]).filter((s) => s?.bl),
		br: grid.map((row) => row[cols - 1]).filter((s) => s?.br)
	};
	for (const [k, from] of Object.entries(edge)) if (from.length) out[k] = true;
	const bc = [...edge.bt, ...edge.bl, ...edge.br, ...edge.bb].find((s) => s?.bc)?.bc;
	if (bc) out.bc = bc;
	return resolveStyle(out as StyleLayer);
}

/** Correctif qui retire toute la mise en forme (Effacer). */
export function clearPatch(): Record<string, null> {
	return Object.fromEntries(STYLE_KEYS.map((k) => [k, null]));
}

/** Correctif qui reproduit exactement un style (pinceau) : les propriétés absentes sont retirées. */
export function painterPatch(st: CellStyle | undefined): Record<string, unknown> {
	const s = (st ?? {}) as Record<string, unknown>;
	return Object.fromEntries(STYLE_KEYS.map((k) => [k, s[k] ?? null]));
}

/** Style de colonne complété par le style de la cellule. */
export function mergeStyles(a?: CellStyle, b?: CellStyle): CellStyle | undefined {
	if (!a) return b;
	if (!b) return a;
	return { ...a, ...b };
}

/** Le texte affiché est-il un nombre (« 1 234,5 », « -3 », « 12 % ») ? */
export function isNumericText(s: string): boolean {
	return /^[-+]?[\d\s  ]+([,.]\d+)?[\s  ]*%?$/.test(s.trim());
}

/**
 * Texte d'un nombre affiché (« 1 234,5 », « 12,5 % ») mis au format de
 * nombre et aux décimales du style ; les autres textes restent tels quels.
 */
export function styledNumber(text: string, st: CellStyle | undefined): string {
	if (!st || (st.nf === undefined && st.dp === undefined) || !isNumericText(text)) return text;
	let t = text.replace(/[\s  ]/g, '');
	const pct = t.endsWith('%');
	t = t.replace('%', '').replace(',', '.');
	let n = Number(t);
	if (!Number.isFinite(n)) return text;
	if (pct) n /= 100;
	return formatNumber(st.nf, n, st.dp, st.ng) ?? text;
}

const fixed0 = () => numberFormat({ maximumFractionDigits: 0 });
const fixed2 = () => numberFormat({ minimumFractionDigits: 2, maximumFractionDigits: 2 });
const percent = () => numberFormat({ style: 'percent', maximumFractionDigits: 2 });
const euro = () => numberFormat({ style: 'currency', currency: 'EUR' });

/**
 * Moyenne ou somme de la barre d'état, comme Excel : au format de la
 * cellule active quand il en a un (décimales, €, %), sinon arrondie à
 * quatre décimales (plus pour un très petit nombre), avec le séparateur
 * de milliers.
 */
export function formatStat(v: number, st?: CellStyle): string {
	const f = st ? formatNumber(st.nf, v, st.dp, st.ng) : null;
	if (f !== null) return f;
	const a = Math.abs(v);
	const digits = a === 0 || a >= 0.01 ? 4 : Math.min(10, 3 - Math.floor(Math.log10(a)));
	return numberFormat({ maximumFractionDigits: digits }).format(v);
}

/** Texte d'un nombre selon le format choisi (null : pas de format). */
export function formatNumber(nf: CellStyle['nf'], v: number, dp?: number, ng = false): string | null {
	if (nf === 'date' || nf === 'time' || nf === 'text') return null;
	if (ng && nf !== '%' && nf !== '€') {
		const d = dp ?? (nf === '0' ? 0 : nf === '0,00' ? 2 : undefined);
		if (d === undefined) return null;
		return numberFormat({ minimumFractionDigits: d, maximumFractionDigits: d, useGrouping: false }).format(v);
	}
	if (dp !== undefined) {
		const opts: Intl.NumberFormatOptions = { minimumFractionDigits: dp, maximumFractionDigits: dp };
		if (nf === '%') return numberFormat({ ...opts, style: 'percent' }).format(v);
		if (nf === '€') return numberFormat({ ...opts, style: 'currency', currency: 'EUR' }).format(v);
		return numberFormat(opts).format(v);
	}
	switch (nf) {
		case '0':
			return fixed0().format(v);
		case '0,00':
			return fixed2().format(v);
		case '%':
			return percent().format(v);
		case '€':
			return euro().format(v);
	}
	return null;
}
