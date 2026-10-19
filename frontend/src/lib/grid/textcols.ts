// Assistant « Convertir le texte en colonnes » (Données › Convertir), comme
// Excel : modèle et calculs de l'aperçu. La conversion elle-même est faite par
// le backend (canvas.TextToColumns), qui donne aussi les parties de l'aperçu.

/** Format d'une colonne produite : Standard (''), Texte, Date ou non importée. */
export type ColFormat = '' | 'text' | 'date' | 'skip';

export interface TextColumnFormat {
	format: string;
	/** Ordre des dates : DMY, MDY, YMD, DYM, MYD, YDM. */
	order: string;
}

export interface TextColumnsOptions {
	fixed: boolean;
	delims: string;
	merge: boolean;
	quote: string;
	breaks: number[] | null;
	columns: TextColumnFormat[] | null;
	decimal: string;
	thousands: string;
	dest: string;
}

export interface TextColumnsPreview {
	table: string;
	column: string;
	header: boolean;
	lines: string[] | null;
	rows: (string[] | null)[] | null;
	count: number;
	total: number;
	overwrite: boolean;
	dest: string;
}

/** Ordres des dates, libellés comme Excel en français (J, M, A). */
export const DATE_ORDERS = [
	{ value: 'DMY', label: 'JMA' },
	{ value: 'MDY', label: 'MJA' },
	{ value: 'YMD', label: 'AMJ' },
	{ value: 'DYM', label: 'JAM' },
	{ value: 'MYD', label: 'MAJ' },
	{ value: 'YDM', label: 'AJM' }
];

export interface Delims {
	tab: boolean;
	semi: boolean;
	comma: boolean;
	space: boolean;
	other: boolean;
	otherChar: string;
}

/** Caractères séparateurs cochés. */
export function delimString(d: Delims): string {
	return (d.tab ? '\t' : '') + (d.semi ? ';' : '') + (d.comma ? ',' : '') + (d.space ? ' ' : '') + (d.other ? d.otherChar.slice(0, 1) : '');
}

/**
 * Séparateurs proposés à l'ouverture : la tabulation, comme Excel, sauf si
 * le texte n'en a pas et a des points-virgules (ou, à défaut, des virgules).
 */
export function guessDelims(lines: string[]): Delims {
	const has = (c: string) => lines.some((l) => l.includes(c));
	const d: Delims = { tab: true, semi: false, comma: false, space: false, other: false, otherChar: '' };
	if (!has('\t')) {
		if (has(';')) d.semi = true;
		else if (has(',')) d.comma = true;
		if (d.semi || d.comma) d.tab = false;
	}
	return d;
}

/** Délimité ou largeur fixe à l'ouverture : largeur fixe pour un texte aligné en colonnes, sans séparateur. */
export function guessFixed(lines: string[]): boolean {
	if (lines.some((l) => /[\t;,]/.test(l))) return false;
	return suggestBreaks(lines).length > 0 && lines.length > 1;
}

/**
 * Coupures proposées en largeur fixe : avant chaque colonne de caractères
 * précédée d'espaces sur toutes les lignes (Excel propose de même).
 */
export function suggestBreaks(lines: string[]): number[] {
	const rows = lines.map((l) => Array.from(l));
	const max = Math.max(0, ...rows.map((r) => r.length));
	const out: number[] = [];
	for (let b = 2; b < max; b++) {
		const blank = rows.every((r) => b - 1 >= r.length || r[b - 1] === ' ');
		if (!blank) continue;
		const starts = rows.some((r) => b < r.length && r[b] !== ' ' && r.slice(0, b - 1).some((c) => c !== ' '));
		if (starts) out.push(b);
	}
	return out;
}

/** Ajoute une coupure en pos (entre 1 et max - 1), sans doublon. */
export function addBreak(breaks: number[], pos: number, max: number): number[] {
	if (pos < 1 || pos >= max || breaks.includes(pos)) return breaks;
	return [...breaks, pos].sort((a, b) => a - b);
}

/** Libellé d'une colonne de l'aperçu de l'étape 3. */
export function formatLabel(f: TextColumnFormat | undefined): string {
	switch (f?.format) {
		case 'text':
			return 'Texte';
		case 'date':
			return DATE_ORDERS.find((o) => o.value === f.order)?.label ?? 'JMA';
		case 'skip':
			return 'Ignorer la colonne';
	}
	return 'Standard';
}

/** Longueur en caractères (et non en unités UTF-16) de la plus longue ligne. */
export function maxChars(lines: string[]): number {
	return Math.max(0, ...lines.map((l) => Array.from(l).length));
}
