// Exemples de requêtes proposés par le clic droit sur un objet de l'onglet
// SQL : SELECT, INSERT, UPDATE, DELETE, écrits avec les vrais noms de colonnes
// et prêts à être modifiés.

import type { ColumnType } from '#lib/grid/format.js';
import { quote, type SqlObject } from './names';
import { count } from './result';

export type ExampleKind = 'select' | 'insert' | 'update' | 'delete';

/** Un tableau croisé est une vue : on ne peut que le lire. */
export function isReadOnly(o: SqlObject): boolean {
	return o.kind === 'pivot' || (o.kind === undefined && o.sqlName.startsWith('"'));
}

/** Valeur d'exemple selon le type de la colonne. */
export function placeholder(type: ColumnType, today = new Date()): string {
	switch (type) {
		case 'integer':
			return '0';
		case 'decimal':
			return '0.0';
		case 'boolean':
			return '1';
		case 'date': {
			const p = (n: number) => String(n).padStart(2, '0');
			return `'${today.getFullYear()}-${p(today.getMonth() + 1)}-${p(today.getDate())}'`;
		}
		default:
			return "'texte'";
	}
}

const typeHint: Record<ColumnType, string> = {
	integer: 'entier',
	decimal: 'décimal',
	text: 'texte',
	boolean: 'booléen : 1 ou 0',
	date: 'date AAAA-MM-JJ',
	formula: 'calculée'
};

/** Colonnes qu'on peut écrire (les colonnes calculées se remplissent seules). */
const writable = (o: SqlObject) => o.columns.filter((c) => c.type !== 'formula');

/** Colonne qui désigne une ligne : la clé, sinon la première colonne saisie. */
function keyColumn(o: SqlObject) {
	const k = o.key !== undefined && o.key >= 0 ? o.columns[o.key] : undefined;
	return k ?? writable(o)[0] ?? o.columns[0];
}

/** Liste alignée « nom, -- commentaire » sur plusieurs lignes. */
function list(items: { text: string; note?: string }[], indent: string, sep = ','): string {
	const width = Math.max(0, ...items.map((x) => x.text.length)) + sep.length;
	return items
		.map((x, i) => {
			const t = x.text + (i < items.length - 1 ? sep : '');
			return x.note ? `${indent}${t.padEnd(width)}  -- ${x.note}` : `${indent}${t}`;
		})
		.join('\n');
}

/** Requête d'exemple pour l'objet, avec ses vrais noms de colonnes. */
export function example(o: SqlObject, kind: ExampleKind, today = new Date()): string {
	const table = quote(o.name);
	const cols = o.columns;
	const first = cols[0] ? quote(cols[0].name) : '';
	const lines = count(o.rows ?? 0);
	switch (kind) {
		case 'select': {
			const what = isReadOnly(o) ? 'du tableau croisé' : 'du tableau';
			return [
				`-- Toutes les colonnes ${what} ${o.name} (${lines})`,
				'SELECT',
				cols.length ? list(cols.map((c) => ({ text: quote(c.name) })), '  ') : '  *',
				`FROM ${table}`,
				`-- WHERE ${first || 'colonne'} = ${cols[0] ? placeholder(cols[0].type, today) : "'valeur'"}`,
				`-- ORDER BY ${first || 'colonne'} DESC`,
				'LIMIT 100;'
			].join('\n');
		}
		case 'insert': {
			const w = writable(o);
			const skipped = cols.length - w.length;
			return [
				`-- Ajoute une ligne à la fin de ${o.name}${skipped ? ' (les colonnes calculées se remplissent seules)' : ''}`,
				'-- Ctrl+Z annule l\'ajout.',
				`INSERT INTO ${table} (`,
				list(w.map((c) => ({ text: quote(c.name) })), '  '),
				') VALUES (',
				list(w.map((c) => ({ text: placeholder(c.type, today), note: `${c.name} (${typeHint[c.type]})` })), '  '),
				');'
			].join('\n');
		}
		case 'update': {
			const key = keyColumn(o);
			const set = writable(o).filter((c) => c !== key);
			const assigned = set.length ? set : writable(o);
			return [
				`-- Modifie les lignes de ${o.name} qui vérifient la condition`,
				'-- Sans WHERE, toutes les lignes sont modifiées. Ctrl+Z annule.',
				`UPDATE ${table}`,
				'SET',
				list(assigned.map((c) => ({ text: `${quote(c.name)} = ${placeholder(c.type, today)}`, note: typeHint[c.type] })), '  '),
				key ? `WHERE ${quote(key.name)} = ${placeholder(key.type, today)};` : 'WHERE 0;'
			].join('\n');
		}
		case 'delete': {
			const key = keyColumn(o);
			return [
				`-- Supprime les lignes de ${o.name} qui vérifient la condition`,
				'-- Sans WHERE, tout le tableau est vidé. Ctrl+Z annule.',
				`DELETE FROM ${table}`,
				key ? `WHERE ${quote(key.name)} = ${placeholder(key.type, today)};` : 'WHERE 0;'
			].join('\n');
		}
	}
}
