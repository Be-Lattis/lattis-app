// Étapes des requêtes d'import (équivalent de Power Query) côté interface :
// catalogue des opérations, étape par défaut, libellés. La définition fait
// foi côté Go (internal/query/model.go).

import type { Column, Query, Source, Step } from '#bindings/internal/query/models.js';

export type Op = 'filter' | 'rename' | 'type' | 'remove' | 'select' | 'split' | 'replace' | 'transform' | 'addcolumn' | 'index' | 'merge' | 'append' | 'group' | 'sort' | 'distinct' | 'top' | 'unpivot' | 'pivot';

export interface OpInfo {
	op: Op;
	label: string; // bouton du ruban de l'éditeur
	group: 'Colonnes' | 'Lignes' | 'Transformer' | 'Combiner';
	hint: string;
}

export const OPS: OpInfo[] = [
	{ op: 'select', label: 'Choisir les colonnes', group: 'Colonnes', hint: 'Garder certaines colonnes, dans l’ordre voulu' },
	{ op: 'remove', label: 'Supprimer les colonnes', group: 'Colonnes', hint: 'Retirer des colonnes' },
	{ op: 'rename', label: 'Renommer', group: 'Colonnes', hint: 'Renommer une colonne' },
	{ op: 'type', label: 'Type de données', group: 'Colonnes', hint: 'Changer le type d’une colonne (une valeur non convertible devient vide)' },
	{ op: 'filter', label: 'Filtrer les lignes', group: 'Lignes', hint: 'Garder les lignes qui vérifient des conditions' },
	{ op: 'sort', label: 'Trier', group: 'Lignes', hint: 'Trier les lignes' },
	{ op: 'distinct', label: 'Supprimer les doublons', group: 'Lignes', hint: 'Garder une ligne par valeur (de toutes les colonnes ou de certaines)' },
	{ op: 'top', label: 'Premières lignes', group: 'Lignes', hint: 'Conserver les N premières lignes' },
	{ op: 'split', label: 'Fractionner la colonne', group: 'Transformer', hint: 'Découper une colonne de texte par délimiteur' },
	{ op: 'replace', label: 'Remplacer les valeurs', group: 'Transformer', hint: 'Remplacer une valeur par une autre' },
	{ op: 'transform', label: 'Format', group: 'Transformer', hint: 'SUPPRESPACE, MAJUSCULE, minuscule' },
	{ op: 'addcolumn', label: 'Colonne personnalisée', group: 'Transformer', hint: 'Nouvelle colonne calculée par une expression SQLite ([Colonne] désigne une colonne)' },
	{ op: 'index', label: 'Colonne d’index', group: 'Transformer', hint: 'Numéroter les lignes' },
	{ op: 'group', label: 'Regrouper par', group: 'Transformer', hint: 'Regrouper les lignes et calculer des agrégats' },
	{ op: 'unpivot', label: 'Dépivoter', group: 'Transformer', hint: 'Transformer des colonnes en lignes (attribut, valeur)' },
	{ op: 'pivot', label: 'Pivoter', group: 'Transformer', hint: 'Transformer les valeurs d’une colonne en colonnes' },
	{ op: 'merge', label: 'Fusionner', group: 'Combiner', hint: 'Joindre une autre requête ou un tableau par des colonnes communes' },
	{ op: 'append', label: 'Ajouter', group: 'Combiner', hint: 'Mettre à la suite les lignes d’une autre requête ou d’un tableau' }
];

export const TYPES: [string, string][] = [
	['text', 'Texte'],
	['integer', 'Entier'],
	['decimal', 'Décimal'],
	['date', 'Date'],
	['boolean', 'Booléen']
];

export const CMPS: [string, string][] = [
	['=', 'est égal à'],
	['<>', 'est différent de'],
	['>', 'est supérieur à'],
	['>=', 'est supérieur ou égal à'],
	['<', 'est inférieur à'],
	['<=', 'est inférieur ou égal à'],
	['contains', 'contient'],
	['notcontains', 'ne contient pas'],
	['begins', 'commence par'],
	['ends', 'se termine par'],
	['empty', 'est vide'],
	['notempty', 'n’est pas vide']
];

export const AGGS: [string, string][] = [
	['sum', 'Somme'],
	['avg', 'Moyenne'],
	['count', 'Nombre de lignes'],
	['countdistinct', 'Nombre distinct'],
	['min', 'Min'],
	['max', 'Max']
];

export const JOINS: [string, string][] = [
	['left', 'Externe gauche (toutes les lignes de la première, celles qui correspondent de la seconde)'],
	['inner', 'Interne (seulement les lignes qui correspondent)'],
	['full', 'Externe entière (toutes les lignes des deux)'],
	['right', 'Externe droite (toutes les lignes de la seconde)'],
	['anti', 'Anti gauche (lignes de la première sans correspondance)']
];

/** Libellé d'un type (type vide : à inférer). */
export function typeName(t: string): string {
	return TYPES.find(([k]) => k === t)?.[1] ?? 'Auto';
}

/** Étape par défaut d'une opération, sur la colonne choisie si besoin. */
export function defaultStep(op: Op, cols: Column[], col?: string): Step {
	const c = col ?? cols[0]?.name ?? '';
	switch (op) {
		case 'filter':
			return { op, conditions: [{ column: c, cmp: '=', value: '' }] };
		case 'rename':
			return { op, column: c, name: c };
		case 'type': {
			const t = cols.find((k) => k.name === c)?.type;
			return { op, column: c, type: (t && t !== 'text' ? 'text' : 'integer') as Step['type'] };
		}
		case 'remove':
			return { op, columns: col ? [col] : [] };
		case 'select':
			return { op, columns: cols.map((k) => k.name) };
		case 'split':
			return { op, column: c, delimiter: ';', parts: 2 };
		case 'replace':
			return { op, column: c, value: '', with: '' };
		case 'transform':
			return { op, column: c, fn: 'trim' };
		case 'addcolumn':
			return { op, name: 'Personnalisé', expr: '' };
		case 'index':
			return { op, name: 'Index', start: 1 };
		case 'merge':
			return { op, other: { kind: 'table' }, keys: [c], otherKeys: [''], join: 'left' };
		case 'append':
			return { op, other: { kind: 'table' } };
		case 'group':
			return { op, columns: col ? [col] : [], aggs: [{ name: 'Nombre', op: 'count' }] };
		case 'sort':
			return { op, sort: [{ column: c }] };
		case 'distinct':
			return { op, columns: [] };
		case 'top':
			return { op, count: 100 };
		case 'unpivot':
			return { op, columns: col ? [col] : [], name: 'Attribut', valueName: 'Valeur' };
		case 'pivot':
			return { op, column: c, valueColumn: cols.find((k) => k.name !== c)?.name ?? '', agg: 'sum' };
	}
}

/** Insère une étape après le rang at (-1 : en tête) et renvoie les étapes. */
export function insertStep(steps: Step[], at: number, s: Step): Step[] {
	const out = [...steps];
	out.splice(Math.max(0, Math.min(at + 1, out.length)), 0, s);
	return out;
}

/** Déplace l'étape i d'un cran (d = -1 ou 1). */
export function moveStep(steps: Step[], i: number, d: number): Step[] {
	const j = i + d;
	if (j < 0 || j >= steps.length) return steps;
	const out = [...steps];
	[out[i], out[j]] = [out[j], out[i]];
	return out;
}

/** Nouvelle requête vide sur une source. */
export function newQuery(name: string, source: Source): Query {
	return { id: 0, name, source, steps: [], table: '', sheet: 0, refreshed: '' };
}

/** Désigne une autre requête ou un tableau (fusion, ajout) : « query:Nom » ou « table:Nom ». */
export function otherKey(s?: Source | null): string {
	if (!s) return '';
	if (s.kind === 'query') return 'query:' + (s.query ?? '');
	if (s.kind === 'table') return 'table:' + (s.table ?? '');
	return '';
}

export function otherSource(key: string): Source {
	const i = key.indexOf(':');
	const kind = key.slice(0, i);
	const name = key.slice(i + 1);
	return kind === 'query' ? { kind, query: name } : { kind: 'table', table: name };
}

/** Copie profonde (les objets du pont Go ne passent pas structuredClone). */
export const clone = <T>(x: T): T => JSON.parse(JSON.stringify(x));
