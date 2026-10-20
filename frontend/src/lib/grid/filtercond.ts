// Filtre automatique d'une colonne, comme le menu ▾ d'Excel : liste de
// valeurs à cocher (avec recherche) et filtres par condition (« Filtres
// textuels », « Filtres numériques », « Filtres chronologiques »).

import type { ColumnType } from './format';
import { lower } from '#lib/i18n/index.js';

/** Condition d'un filtre (voir canvas.FilterCond). */
export interface FilterCond {
	op: CondOp;
	value?: string;
	to?: string;
}

export type CondOp =
	| 'eq'
	| 'ne'
	| 'begins'
	| 'ends'
	| 'contains'
	| 'notcontains'
	| 'gt'
	| 'ge'
	| 'lt'
	| 'le'
	| 'between'
	| 'top'
	| 'bottom'
	| 'above'
	| 'below';

/** Filtre d'une colonne : valeurs gardées, erreurs seulement, conditions (toutes, ou l'une avec or). */
export interface ColumnFilter {
	values?: string[]; // valeurs gardées (texte affiché, vide : cellules vides)
	errors?: boolean; // saisies hors type seulement
	conds?: FilterCond[];
	or?: boolean;
}

export type FilterKind = 'text' | 'number' | 'date';

/** Famille de filtres proposée pour un type de colonne. */
export function filterKind(type: ColumnType): FilterKind {
	return type === 'integer' || type === 'decimal' ? 'number' : type === 'date' ? 'date' : 'text';
}

/** Titre du sous-menu, comme Excel. */
export function kindTitle(k: FilterKind): string {
	return k === 'number' ? 'Filtres numériques' : k === 'date' ? 'Filtres chronologiques' : 'Filtres textuels';
}

const LABELS: Record<CondOp, string> = {
	eq: 'Est égal à',
	ne: 'Est différent de',
	begins: 'Commence par',
	ends: 'Se termine par',
	contains: 'Contient',
	notcontains: 'Ne contient pas',
	gt: 'Est supérieur à',
	ge: 'Est supérieur ou égal à',
	lt: 'Est inférieur à',
	le: 'Est inférieur ou égal à',
	between: 'Entre',
	top: '10 premiers',
	bottom: '10 derniers',
	above: 'Au-dessus de la moyenne',
	below: 'En dessous de la moyenne'
};

const DATE_LABELS: Partial<Record<CondOp, string>> = { lt: 'Avant', gt: 'Après', le: 'Avant ou le', ge: 'Après ou le' };

export function condLabel(op: CondOp, k: FilterKind = 'text'): string {
	return (k === 'date' && DATE_LABELS[op]) || LABELS[op];
}

/** Conditions proposées pour une famille, dans l'ordre d'Excel. */
export function condOps(k: FilterKind): CondOp[] {
	if (k === 'number') return ['eq', 'ne', 'gt', 'ge', 'lt', 'le', 'between', 'top', 'bottom', 'above', 'below'];
	if (k === 'date') return ['eq', 'ne', 'lt', 'gt', 'between', 'top', 'bottom'];
	return ['eq', 'ne', 'begins', 'ends', 'contains', 'notcontains'];
}

/** Nombre de valeurs à saisir : 0 (moyenne), 1, ou 2 (entre). */
export function condArgs(op: CondOp): 0 | 1 | 2 {
	return op === 'above' || op === 'below' ? 0 : op === 'between' ? 2 : 1;
}

/** Texte court d'une condition : « Contient lyon », « Entre 10 et 20 », « 5 premiers ». */
export function describeCond(c: FilterCond, k: FilterKind = 'text'): string {
	if (c.op === 'top' || c.op === 'bottom') return `${c.value ?? '10'} ${c.op === 'top' ? 'premiers' : 'derniers'}`;
	const n = condArgs(c.op);
	const label = condLabel(c.op, k);
	if (n === 0) return label;
	if (n === 2) return `${label} ${c.value ?? ''} et ${c.to ?? ''}`;
	return `${label} « ${c.value ?? ''} »`;
}

/** Résumé d'un filtre par condition. */
export function describeConds(f: ColumnFilter | undefined, k: FilterKind = 'text'): string {
	if (!f?.conds?.length) return '';
	return f.conds.map((c) => describeCond(c, k)).join(f.or ? ' ou ' : ' et ');
}

/** Valeur proposée dans la liste à cocher du panneau. */
export interface PanelValue {
	text: string;
	count: number;
	on: boolean;
}

/** Valeurs affichées par la recherche (sans casse). */
export function searchValues<T extends { text: string }>(values: T[], search: string): T[] {
	const q = lower(search.trim());
	return q ? values.filter((v) => lower(v.text).includes(q)) : values;
}

/**
 * Filtre par valeurs d'après le panneau (OK ou Entrée), comme Excel : avec
 * une recherche, seules les valeurs trouvées et cochées sont gardées ; sans
 * recherche, toutes cochées : pas de filtre par valeurs. Renvoie null quand
 * il ne reste aucun critère, ou une erreur quand rien ne serait affiché.
 */
export function valuesFilter(values: PanelValue[], search: string, errors: boolean): { filter: ColumnFilter | null; error?: string } {
	const shown = searchValues(values, search);
	const searching = search.trim() !== '';
	const keep = (searching ? shown : values).filter((v) => v.on);
	const f: ColumnFilter = {};
	if (searching || keep.length < values.length) f.values = keep.map((v) => v.text);
	if (errors) f.errors = true;
	if (f.values && !f.values.length && !errors)
		return { filter: null, error: searching && !shown.length ? 'Aucune valeur ne correspond à la recherche' : 'Cochez au moins une valeur' };
	return { filter: f.values || f.errors ? f : null };
}
