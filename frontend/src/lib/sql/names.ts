// Noms d'affichage dans l'onglet SQL.
//
// Les tableaux et colonnes portent en base des noms internes (t_1, c_0…) ; on
// écrit les requêtes avec les noms affichés sur la feuille (Clients, "Nom
// client", [Code postal]) et elles sont traduites juste avant l'exécution.
// Les colonnes du résultat reprennent le texte saisi comme en-tête.

import type { ColumnType } from '#lib/grid/format.js';

export interface SqlColumn {
	name: string;
	sql: string;
	type: ColumnType;
}
export interface SqlObject {
	name: string;
	sqlName: string;
	rows: number;
	columns: SqlColumn[];
	/** « table », ou « pivot » pour un tableau croisé (vue en lecture seule). */
	kind?: 'table' | 'pivot';
	/** Rang de la colonne clé, s'il y en a une. */
	key?: number;
}

export type TokenKind = 'ws' | 'comment' | 'string' | 'number' | 'word' | 'quoted' | 'punct';
export interface Token {
	kind: TokenKind;
	text: string;
	from: number;
	/** Nom désigné : le mot lui-même, ou le contenu d'un identifiant entre guillemets. */
	value: string;
}

const wordStart = /[\p{L}_]/u;
const wordPart = /[\p{L}\p{N}_$]/u;

/** Découpe un texte SQL en jetons ; tolère une saisie incomplète. */
export function tokenize(src: string): Token[] {
	const out: Token[] = [];
	let i = 0;
	const push = (kind: TokenKind, end: number, value?: string) => {
		const text = src.slice(i, end);
		out.push({ kind, text, from: i, value: value ?? text });
		i = end;
	};
	const until = (close: string, from: number, doubled: boolean) => {
		let j = from;
		for (;;) {
			const k = src.indexOf(close, j);
			if (k < 0) return src.length;
			if (doubled && src.startsWith(close, k + 1)) {
				j = k + 2;
				continue;
			}
			return k + close.length;
		}
	};
	while (i < src.length) {
		const c = src[i];
		if (/\s/.test(c)) {
			let j = i + 1;
			while (j < src.length && /\s/.test(src[j])) j++;
			push('ws', j);
		} else if (src.startsWith('--', i)) {
			const k = src.indexOf('\n', i);
			push('comment', k < 0 ? src.length : k);
		} else if (src.startsWith('/*', i)) {
			const k = src.indexOf('*/', i + 2);
			push('comment', k < 0 ? src.length : k + 2);
		} else if (c === "'") {
			push('string', until("'", i + 1, true));
		} else if (c === '"' || c === '`') {
			const end = until(c, i + 1, true);
			const closed = end > i + 1 && src[end - 1] === c && end - i >= 2;
			const inner = src.slice(i + 1, closed ? end - 1 : end);
			push('quoted', end, inner.replaceAll(c + c, c));
		} else if (c === '[') {
			const end = until(']', i + 1, false);
			push('quoted', end, src.slice(i + 1, src[end - 1] === ']' ? end - 1 : end));
		} else if (/[0-9]/.test(c) || (c === '.' && /[0-9]/.test(src[i + 1] ?? ''))) {
			const m = /^(0x[0-9a-f]+|[0-9]*\.?[0-9]+(e[+-]?[0-9]+)?)/i.exec(src.slice(i))!;
			push('number', i + m[0].length);
		} else if (wordStart.test(c)) {
			let j = i + 1;
			while (j < src.length && wordPart.test(src[j])) j++;
			push('word', j);
		} else {
			const two = src.slice(i, i + 2);
			push('punct', i + (['<=', '>=', '<>', '!=', '==', '||', '<<', '>>', '->'].includes(two) ? 2 : 1));
		}
	}
	return out;
}

/** Mots réservés : jamais pris pour un nom de tableau ou de colonne s'ils ne sont pas entre guillemets. */
export const KEYWORDS = new Set(
	`ABORT ACTION ADD ALL ALTER ALWAYS ANALYZE AND AS ASC ATTACH AUTOINCREMENT BEFORE BEGIN BETWEEN BY CASCADE CASE CAST CHECK
	COLLATE COLUMN COMMIT CONFLICT CONSTRAINT CREATE CROSS CURRENT_DATE CURRENT_TIME CURRENT_TIMESTAMP DEFAULT DEFERRABLE
	DELETE DESC DETACH DISTINCT DO DROP EACH ELSE END ESCAPE EXCEPT EXCLUDE EXISTS EXPLAIN FALSE FILTER FIRST FOLLOWING
	FOR FOREIGN FROM FULL GLOB GROUP GROUPS HAVING IF IGNORE IN INDEX INDEXED INNER INSERT INSTEAD INTERSECT INTO IS
	ISNULL JOIN KEY LAST LEFT LIKE LIMIT MATCH MATERIALIZED NATURAL NO NOT NOTHING NOTNULL NULL NULLS OF OFFSET ON OR ORDER
	OTHERS OUTER OVER PARTITION PLAN PRAGMA PRECEDING PRIMARY QUERY RANGE RECURSIVE REFERENCES REGEXP REINDEX RELEASE
	RENAME REPLACE RESTRICT RETURNING RIGHT ROLLBACK ROW ROWS SAVEPOINT SELECT SET TABLE TEMP TEMPORARY THEN TIES TO
	TRANSACTION TRIGGER TRUE UNBOUNDED UNION UNIQUE UPDATE USING VACUUM VALUES VIEW VIRTUAL WHEN WHERE WINDOW WITH WITHOUT`
		.split(/\s+/)
		.filter(Boolean)
);

const isKeyword = (t: Token) => t.kind === 'word' && KEYWORDS.has(t.text.toUpperCase());
const isName = (t: Token | undefined): t is Token => !!t && (t.kind === 'quoted' || (t.kind === 'word' && !isKeyword(t)));
const named = (t: Token): boolean => isName(t);
const kw = (t: Token | undefined, ...words: string[]) => !!t && t.kind === 'word' && words.includes(t.text.toUpperCase());
const fold = (s: string) => s.toLocaleLowerCase('fr-FR');

/** Index des objets par nom (affiché ou interne), insensible à la casse. */
export class Catalog {
	readonly objects: SqlObject[];
	private tables = new Map<string, SqlObject>();

	constructor(objects: SqlObject[]) {
		this.objects = objects;
		for (const o of objects) this.tables.set(fold(o.sqlName), o);
		for (const o of objects) this.tables.set(fold(o.name), o);
	}

	table(name: string): SqlObject | undefined {
		return this.tables.get(fold(name));
	}

	/** Colonne par nom affiché (un nom interne passe tel quel). */
	column(o: SqlObject, name: string): SqlColumn | undefined {
		const f = fold(name);
		return o.columns.find((c) => fold(c.name) === f);
	}
}

export type Role = 'table' | 'column' | 'alias';
export interface Resolved {
	/** Position du jeton dans le tableau de jetons. */
	token: number;
	role: Role;
	table?: SqlObject;
	column?: SqlColumn;
}
export interface Analysis {
	tokens: Token[];
	resolved: Resolved[];
	/** Tableaux cités dans la requête (sous-requêtes comprises), et alias vers leur tableau. */
	scope: SqlObject[];
	aliases: Map<string, SqlObject>;
	/** Colonnes présentes dans plusieurs tableaux cités, écrites sans préfixe. */
	ambiguous: { token: number; name: string; tables: SqlObject[] }[];
}

const clauseEnd = ['WHERE', 'GROUP', 'ORDER', 'LIMIT', 'HAVING', 'WINDOW', 'UNION', 'EXCEPT', 'INTERSECT', 'SELECT', 'ON', 'USING', 'SET', 'VALUES', 'RETURNING'];

/** Repère les tableaux, alias et colonnes désignés par nom dans une requête. */
export function analyze(src: string, catalog: Catalog): Analysis {
	const tokens = tokenize(src);
	const sig = tokens.map((t, i) => i).filter((i) => tokens[i].kind !== 'ws' && tokens[i].kind !== 'comment');
	const at = (k: number) => (k >= 0 && k < sig.length ? tokens[sig[k]] : undefined);
	const resolved: Resolved[] = [];
	const done = new Set<number>();
	const aliases = new Map<string, SqlObject>();

	// Portées : chaque sous-requête « (SELECT … ) » a ses tableaux ; une colonne
	// se cherche d'abord dans sa sous-requête, puis dans les requêtes englobantes.
	const scopes: { parent: number; tables: SqlObject[] }[] = [{ parent: -1, tables: [] }];
	const scopeOf: number[] = [];
	const stack: { depth: number; scope: number }[] = [];
	let depth = 0;
	let cur = 0;
	for (let k = 0; k < sig.length; k++) {
		const t = at(k)!;
		if (t.text === '(') {
			depth++;
			if (kw(at(k + 1), 'SELECT', 'WITH', 'VALUES')) {
				stack.push({ depth, scope: cur });
				scopes.push({ parent: cur, tables: [] });
				cur = scopes.length - 1;
			}
		}
		scopeOf[k] = cur;
		if (t.text === ')') {
			if (stack.length && stack[stack.length - 1].depth === depth) cur = stack.pop()!.scope;
			depth--;
		}
	}
	const addScope = (k: number, o: SqlObject) => {
		const s = scopes[scopeOf[k]].tables;
		if (!s.includes(o)) s.push(o);
	};

	// 1. Tableaux : après FROM, JOIN, INTO, UPDATE, TABLE, ou dans la liste du FROM ;
	//    un alias peut suivre (AS facultatif).
	let inFrom = false;
	for (let k = 0; k < sig.length; k++) {
		const t = at(k)!;
		if (kw(t, 'FROM')) inFrom = true;
		else if (kw(t, ...clauseEnd) || t.text === ')') inFrom = false;
		const prev = at(k - 1);
		const tablePlace = kw(prev, 'FROM', 'JOIN', 'INTO', 'UPDATE', 'TABLE') || (inFrom && prev?.text === ',');
		if (!tablePlace || !isName(t) || at(k + 1)?.text === '(' || at(k + 1)?.text === '.') continue;
		const o = catalog.table(t.value);
		if (!o) continue;
		resolved.push({ token: sig[k], role: 'table', table: o });
		done.add(k);
		addScope(k, o);
		let a = k + 1;
		if (kw(at(a), 'AS')) a++;
		const alias = at(a);
		if (isName(alias) && at(a + 1)?.text !== '(' && !catalog.table(alias.value)) {
			aliases.set(fold(alias.value), o);
			resolved.push({ token: sig[a], role: 'alias', table: o });
			done.add(a);
			k = a;
		}
	}

	const qualifier = (name: string) => aliases.get(fold(name)) ?? catalog.table(name);

	// 2. Préfixes Tableau.colonne, puis colonnes seules cherchées dans les tableaux cités.
	const ambiguous: Analysis['ambiguous'] = [];
	for (let k = 0; k < sig.length; k++) {
		if (done.has(k)) continue;
		const t = at(k)!;
		if (!isName(t) || at(k + 1)?.text === '(') continue;
		if (at(k + 1)?.text === '.') {
			const o = qualifier(t.value);
			if (!o) continue;
			const isAlias = aliases.has(fold(t.value));
			resolved.push({ token: sig[k], role: isAlias ? 'alias' : 'table', table: o });
			if (!isAlias) addScope(k, o);
			done.add(k);
			const c = at(k + 2);
			if (isName(c)) {
				const col = catalog.column(o, c.value);
				if (col) resolved.push({ token: sig[k + 2], role: 'column', table: o, column: col });
				done.add(k + 2);
			}
			continue;
		}
		if (at(k - 1)?.text === '.' || kw(at(k - 1), 'AS')) continue;
		if (aliases.has(fold(t.value))) continue;
		for (let s = scopeOf[k]; s >= 0; s = scopes[s].parent) {
			const hits = scopes[s].tables.flatMap((o) => {
				const col = catalog.column(o, t.value);
				return col ? [{ o, col }] : [];
			});
			if (hits.length === 1) resolved.push({ token: sig[k], role: 'column', table: hits[0].o, column: hits[0].col });
			else if (hits.length > 1) ambiguous.push({ token: sig[k], name: t.value, tables: hits.map((h) => h.o) });
			if (hits.length > 0) break;
		}
	}
	const scope = scopes.flatMap((s) => s.tables).filter((o, i, all) => all.indexOf(o) === i);
	resolved.sort((a, b) => a.token - b.token);
	return { tokens, resolved, scope, aliases, ambiguous };
}

export interface Translation {
	/** Requête envoyée à SQLite. */
	sql: string;
	/** Tableaux cités, pour retrouver les en-têtes d'un SELECT *. */
	scope: SqlObject[];
}

/** Traduit les noms affichés en noms internes ; échoue sur une colonne ambiguë. */
export function translate(src: string, catalog: Catalog): Translation {
	const a = analyze(src, catalog);
	if (a.ambiguous.length > 0) {
		const x = a.ambiguous[0];
		throw new Error(`Colonne « ${x.name} » ambiguë : présente dans ${x.tables.map((o) => `« ${o.name} »`).join(' et ')}. Préciser le tableau, par exemple ${quote(x.tables[0].name)}.${quote(x.name)}.`);
	}
	const repl = new Map<number, string>();
	for (const r of a.resolved) {
		if (r.role === 'table') repl.set(r.token, r.table!.sqlName);
		else if (r.role === 'column') repl.set(r.token, r.column!.sql);
	}
	const headers = selectHeaders(a, repl);
	let out = '';
	a.tokens.forEach((t, i) => {
		out += repl.get(i) ?? t.text;
		const h = headers.get(i);
		if (h !== undefined) out += ` AS ${quoteAlways(h)}`;
	});
	return { sql: out, scope: a.scope };
}

// Pour chaque élément sans alias du premier SELECT, dont un nom a été traduit,
// ajoute « AS "texte saisi" » : l'en-tête du résultat reste lisible.
function selectHeaders(a: Analysis, repl: Map<number, string>): Map<number, string> {
	const out = new Map<number, string>();
	const t = a.tokens;
	let i = t.findIndex((x) => x.kind !== 'ws' && x.kind !== 'comment');
	if (i < 0 || !kw(t[i], 'SELECT')) return out;
	i++;
	const items: [number, number][] = [];
	let depth = 0;
	let start = i;
	for (; i < t.length; i++) {
		const x = t[i];
		if (x.text === '(') depth++;
		else if (x.text === ')') depth--;
		if (depth < 0) return out;
		if (depth === 0 && (x.text === ',' || kw(x, 'FROM') || x.text === ';')) {
			items.push([start, i]);
			if (x.text !== ',') break;
			start = i + 1;
		}
	}
	if (i === t.length) items.push([start, i]);
	for (const [s, e] of items) {
		const idx = [];
		for (let j = s; j < e; j++) if (t[j].kind !== 'ws' && t[j].kind !== 'comment') idx.push(j);
		while (idx.length && kw(t[idx[0]], 'DISTINCT', 'ALL')) idx.shift();
		if (idx.length === 0) continue;
		const last = t[idx[idx.length - 1]];
		if (last.text === '*') continue;
		if (!idx.some((j) => repl.has(j))) continue;
		if (idx.length > 1) {
			const before = t[idx[idx.length - 2]];
			const aliased = isName(last) && (kw(before, 'AS') || named(before) || before.text === ')' || before.kind === 'string' || before.kind === 'number');
			if (aliased) continue;
		}
		const text = t
			.slice(idx[0], idx[idx.length - 1] + 1)
			.map((x) => x.text)
			.join('')
			.replace(/\s+/g, ' ');
		const shown = idx.length === 1 && last.kind === 'quoted' ? last.value : text;
		out.set(idx[idx.length - 1], shown);
	}
	return out;
}

/** En-têtes du résultat : un nom interne (SELECT *) redevient le nom affiché. */
export function displayHeaders(columns: string[], scope: SqlObject[]): string[] {
	return columns.map((c) => {
		const f = fold(c);
		const hits = scope.flatMap((o) => o.columns.filter((x) => fold(x.sql) === f));
		return hits.length === 1 ? hits[0].name : c;
	});
}

const plain = /^[\p{L}_][\p{L}\p{N}_]*$/u;

/** Nom tel qu'on l'écrit dans une requête : entre guillemets s'il le faut. */
export function quote(name: string): string {
	return plain.test(name) && !KEYWORDS.has(name.toUpperCase()) ? name : quoteAlways(name);
}

function quoteAlways(name: string): string {
	return `"${name.replaceAll('"', '""')}"`;
}

export interface Statement {
	from: number;
	to: number;
	text: string;
}

/**
 * Instructions d'un texte : séparées par « ; » ou par une ligne vide (hors
 * chaînes et commentaires), comme dans la plupart des éditeurs SQL.
 */
export function statements(src: string): Statement[] {
	const cuts: [number, number][] = [];
	for (const t of tokenize(src)) {
		if (t.kind === 'punct' && t.text === ';') cuts.push([t.from, t.from + 1]);
		else if (t.kind === 'ws' && /\n[^\S\n]*\n/.test(t.text)) cuts.push([t.from, t.from + t.text.length]);
	}
	const out: Statement[] = [];
	let from = 0;
	for (const [a, b] of [...cuts, [src.length, src.length] as [number, number]]) {
		const r = trimRange(src, from, a);
		if (r.text) out.push(r);
		from = b;
	}
	return out;
}

/** Instruction sous le curseur, ou à défaut celle qui le précède. */
export function statementAt(src: string, pos: number): Statement {
	const all = statements(src);
	let best: Statement = { from: pos, to: pos, text: '' };
	for (const s of all) {
		if (s.from > pos) break;
		best = s;
	}
	return best.text ? best : (all[0] ?? best);
}

function trimRange(src: string, from: number, to: number): Statement {
	const text = src.slice(from, to);
	const lead = text.length - text.trimStart().length;
	const trimmed = text.trim();
	return { from: from + lead, to: from + lead + trimmed.length, text: trimmed };
}

/** Position dans la requête de l'erreur signalée par SQLite (« near "X" »,
 * « no such column: X », ou en français « Colonne inconnue : « X » »…), pour la souligner dans l'éditeur ; null si le
 * message ne la désigne pas ou si le mot n'est pas dans le texte. */
export function errorSpan(text: string, message: string): { from: number; to: number } | null {
	const m =
		/near "([^"]+)"/.exec(message) ??
		/no such (?:column|table|function): ([^\s,]+)/i.exec(message) ??
		/(?:près de|inconnue? :) « ([^»]+) »/.exec(message);
	if (!m) return null;
	const word = m[1];
	const at = text.toLowerCase().indexOf(word.toLowerCase());
	return at < 0 ? null : { from: at, to: at + word.length };
}
