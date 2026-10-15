// Affichage des valeurs selon le type logique de la colonne (locale française).

export type ColumnType = 'integer' | 'decimal' | 'text' | 'boolean' | 'date';

const decimal = new Intl.NumberFormat('fr-FR', {
	minimumFractionDigits: 2,
	maximumFractionDigits: 6
});

/** Texte affiché dans une cellule. */
export function formatValue(type: ColumnType, v: unknown): string {
	if (v === null || v === undefined) return '';
	switch (type) {
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
		default:
			return 'Abc';
	}
}

export function typeLabel(type: ColumnType): string {
	return { integer: 'Entier', decimal: 'Décimal', text: 'Texte', boolean: 'Booléen', date: 'Date' }[type];
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
