// Affichage des valeurs selon le type logique de la colonne (locale française).

export type ColumnType = 'integer' | 'decimal' | 'text' | 'boolean' | 'date' | 'formula';

const general = new Intl.NumberFormat('fr-FR', { maximumFractionDigits: 10, useGrouping: false });

/** Affichage « Standard » d'Excel pour une valeur calculée. */
function generalValue(v: unknown): string {
	if (typeof v === 'number') return general.format(v);
	if (typeof v === 'boolean') return v ? 'VRAI' : 'FAUX';
	return String(v);
}

/** Vrai pour le libellé d'une erreur de formule (« #DIV/0! »…). */
export function isErrorValue(v: unknown): boolean {
	return typeof v === 'string' && /^#(DIV\/0!|VALEUR!|N\/A|REF!|NOM\?|NOMBRE!)$/.test(v);
}

/** Alignement d'une valeur calculée : nombres à droite, booléens et erreurs centrés. */
export function valueAlignment(v: unknown): 'left' | 'right' | 'center' {
	if (typeof v === 'number') return 'right';
	if (typeof v === 'boolean' || isErrorValue(v)) return 'center';
	return 'left';
}

const decimal = new Intl.NumberFormat('fr-FR', {
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
			return typeof v === 'number' ? decimal.format(v) : String(v);
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

/** Texte proposé à l'édition (sans séparateur de milliers). */
export function editValue(type: ColumnType, v: unknown): string {
	if (type === 'decimal' && typeof v === 'number') return String(v).replace('.', ',');
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

/** Icône de type affichée dans l'en-tête de colonne. */
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

export function typeLabel(type: ColumnType): string {
	return { integer: 'Entier', decimal: 'Décimal', text: 'Texte', boolean: 'Booléen', date: 'Date', formula: 'Calculée' }[type];
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
