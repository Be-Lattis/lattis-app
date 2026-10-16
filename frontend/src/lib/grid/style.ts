// Mise en forme d'une cellule (même JSON que le classeur, voir
// internal/canvas/styles.go).
//
// Un seul système, en couches de la moins à la plus précise (comme Excel :
// cellule > ligne > colonne) : colonne de la feuille, ligne de la feuille,
// colonne d'un tableau, cellule (de tableau, libre ou de tableau croisé),
// puis mise en forme conditionnelle. Une propriété à null dans une couche
// la retire explicitement (elle masque les couches au-dessous).

export interface CellStyle {
	b?: boolean; // gras
	i?: boolean; // italique
	u?: boolean; // souligné
	fg?: string; // couleur du texte
	bg?: string; // couleur de remplissage
	al?: 'left' | 'center' | 'right';
	nf?: '0' | '0,00' | '%' | '€';
	dp?: number; // décimales imposées (Ajouter / Réduire les décimales)
	bt?: boolean; // bordures haut, bas, gauche, droite
	bb?: boolean;
	bl?: boolean;
	br?: boolean;
}

/** Propriétés de mise en forme (format de pinceau, effacement). */
export const STYLE_KEYS = ['b', 'i', 'u', 'fg', 'bg', 'al', 'nf', 'dp', 'bt', 'bb', 'bl', 'br'] as const;

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
	return formatNumber(st.nf, n, st.dp) ?? text;
}

const fixed0 = new Intl.NumberFormat('fr-FR', { maximumFractionDigits: 0 });
const fixed2 = new Intl.NumberFormat('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const percent = new Intl.NumberFormat('fr-FR', { style: 'percent', maximumFractionDigits: 2 });
const euro = new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'EUR' });

/** Texte d'un nombre selon le format choisi (null : pas de format). */
export function formatNumber(nf: CellStyle['nf'], v: number, dp?: number): string | null {
	if (dp !== undefined) {
		const opts: Intl.NumberFormatOptions = { minimumFractionDigits: dp, maximumFractionDigits: dp };
		if (nf === '%') return new Intl.NumberFormat('fr-FR', { ...opts, style: 'percent' }).format(v);
		if (nf === '€') return new Intl.NumberFormat('fr-FR', { ...opts, style: 'currency', currency: 'EUR' }).format(v);
		return new Intl.NumberFormat('fr-FR', opts).format(v);
	}
	switch (nf) {
		case '0':
			return fixed0.format(v);
		case '0,00':
			return fixed2.format(v);
		case '%':
			return percent.format(v);
		case '€':
			return euro.format(v);
	}
	return null;
}
