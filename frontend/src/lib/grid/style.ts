// Mise en forme d'une cellule (même JSON que le classeur, voir
// internal/canvas/styles.go).

export interface CellStyle {
	b?: boolean; // gras
	i?: boolean; // italique
	u?: boolean; // souligné
	fg?: string; // couleur du texte
	bg?: string; // couleur de remplissage
	al?: 'left' | 'center' | 'right';
	nf?: '0' | '0,00' | '%' | '€';
}

export function parseStyle(s: string | undefined | null): CellStyle | undefined {
	if (!s) return undefined;
	try {
		return JSON.parse(s) as CellStyle;
	} catch {
		return undefined;
	}
}

/** Style de colonne complété par le style de la cellule. */
export function mergeStyles(a?: CellStyle, b?: CellStyle): CellStyle | undefined {
	if (!a) return b;
	if (!b) return a;
	return { ...a, ...b };
}

const fixed0 = new Intl.NumberFormat('fr-FR', { maximumFractionDigits: 0 });
const fixed2 = new Intl.NumberFormat('fr-FR', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const percent = new Intl.NumberFormat('fr-FR', { style: 'percent', maximumFractionDigits: 2 });
const euro = new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'EUR' });

/** Texte d'un nombre selon le format choisi (null : pas de format). */
export function formatNumber(nf: CellStyle['nf'], v: number): string | null {
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
