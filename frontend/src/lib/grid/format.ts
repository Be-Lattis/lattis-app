// Affichage des valeurs selon le type logique de la colonne, dans la langue de
// l'interface (#lib/i18n) ; la valeur stockée ne change pas.
import { decimalSep, numberFormat, t, tAny } from '#lib/i18n/index.js';

export type ColumnType = 'integer' | 'decimal' | 'text' | 'boolean' | 'date' | 'formula';

const general = () => numberFormat({ maximumFractionDigits: 10, useGrouping: false });

/** Affichage « Standard » d'Excel pour une valeur calculée. */
function generalValue(v: unknown): string {
	if (typeof v === 'number') return general().format(v);
	if (typeof v === 'boolean') return v ? 'VRAI' : 'FAUX';
	return String(v);
}

/** Vrai pour le libellé d'une erreur de formule (« #DIV/0! »…). */
export function isErrorValue(v: unknown): boolean {
	return typeof v === 'string' && /^#(DIV\/0!|VALEUR!|N\/A|REF!|NOM\?|NOMBRE!|CIRC!)$/.test(v);
}

/** Alignement d'une valeur calculée : nombres à droite, booléens et erreurs centrés. */
export function valueAlignment(v: unknown): 'left' | 'right' | 'center' {
	if (typeof v === 'number') return 'right';
	if (typeof v === 'boolean' || isErrorValue(v)) return 'center';
	return 'left';
}

const decimal = () => numberFormat({
	minimumFractionDigits: 2,
	maximumFractionDigits: 6
});

/** Texte affiché dans une cellule. */
export function formatValue(type: ColumnType, v: unknown): string {
	if (v === null || v === undefined) return '';
	switch (type) {
		case 'formula':
			return generalValue(v);
		case 'integer':
			return String(v);
		case 'decimal':
			return typeof v === 'number' ? decimal().format(v) : String(v);
		case 'boolean':
			return v ? 'VRAI' : 'FAUX';
		case 'date': {
			const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(v));
			return m ? `${m[3]}/${m[2]}/${m[1]}` : String(v);
		}
		default:
			return String(v);
	}
}

/**
 * Texte d'une cellule de tableau. Une colonne calculée dont le résultat est
 * décimal (result, deviné côté Go) s'affiche comme une colonne Décimal : le
 * même nombre de décimales sur toutes ses lignes (1,44 / 3,00 / 0,96).
 */
export function formatColumnValue(col: { type: ColumnType; result?: string } | undefined, v: unknown): string {
	if (col?.type === 'formula' && col.result === 'decimal' && typeof v === 'number') return formatValue('decimal', v);
	return formatValue(col?.type ?? 'text', v);
}

/** Texte proposé à l'édition (sans séparateur de milliers). */
export function editValue(type: ColumnType, v: unknown): string {
	if (type === 'decimal' && typeof v === 'number') return String(v).replace('.', decimalSep());
	return formatValue(type, v);
}

/** Alignement par défaut, comme dans Excel : nombres à droite, booléens centrés. */
export function alignment(type: ColumnType): 'left' | 'right' | 'center' {
	switch (type) {
		case 'integer':
		case 'decimal':
		case 'date':
			return 'right';
		case 'boolean':
			return 'center';
		default:
			return 'left';
	}
}

/** Icône SVG du type (voir #lib/icons), à la place du texte de typeIcon. */
export function typeIconName(type: ColumnType): 'type-date' | 'type-boolean' | null {
	return type === 'date' ? 'type-date' : type === 'boolean' ? 'type-boolean' : null;
}

/** Icône de type affichée dans l'en-tête de colonne (texte ; voir aussi typeIconName). */
export function typeIcon(type: ColumnType): string {
	switch (type) {
		case 'integer':
			return '123';
		case 'decimal':
			return '1,5';
		case 'boolean':
			return '☑';
		case 'date':
			return '📅';
		case 'formula':
			return 'fx';
		default:
			return 'Abc';
	}
}

/**
 * Libellés du tri croissant et décroissant selon le type, comme Excel :
 * nombres par ordre croissant / décroissant, dates du plus ancien au plus
 * récent, textes de A à Z. result : type du résultat d'une colonne calculée.
 */
export function sortLabels(type: ColumnType | string, result?: string): [string, string] {
	const k = type === 'formula' ? (result ?? 'text') : type;
	if (k === 'integer' || k === 'decimal') return [t('grid.sort.number.asc'), t('grid.sort.number.desc')];
	if (k === 'date') return [t('grid.sort.date.asc'), t('grid.sort.date.desc')];
	return [t('grid.sort.text.asc'), t('grid.sort.text.desc')];
}

export function typeLabel(type: ColumnType): string {
	return tAny(`grid.type.${type}`);
}

/** Largeur initiale d'une colonne, en pixels CSS. */
export function defaultWidth(type: ColumnType): number {
	switch (type) {
		case 'boolean':
			return 80;
		case 'integer':
		case 'date':
			return 110;
		case 'decimal':
			return 120;
		default:
			return 150;
	}
}
