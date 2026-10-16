// Autocomplétion de l'onglet SQL : noms affichés des tableaux et colonnes,
// mots réservés et fonctions SQLite. Indépendante de CodeMirror (testable).

import { typeLabel } from '#lib/grid/format.js';
import { analyze, quote, statementAt, tokenize, KEYWORDS, type Catalog } from './names';

export interface Suggestion {
	label: string;
	/** Texte inséré à la place de [from, pos). */
	apply: string;
	kind: 'table' | 'column' | 'keyword' | 'function';
	detail?: string;
	boost?: number;
}
export interface Completion {
	from: number;
	options: Suggestion[];
}

export const FUNCTIONS = `abs avg coalesce count date datetime dense_rank first_value format glob group_concat hex ifnull iif instr
	json_extract julianday lag last_value lead length lower ltrim max min nullif ntile percent_rank printf quote random rank
	replace round row_number rtrim strftime substr sum time total trim typeof unicode upper`
	.split(/\s+/)
	.filter(Boolean);

const COMMON = new Set(['SELECT', 'FROM', 'WHERE', 'GROUP', 'BY', 'ORDER', 'LIMIT', 'JOIN', 'LEFT', 'ON', 'AND', 'OR', 'NOT', 'AS', 'DISTINCT', 'HAVING', 'DESC', 'ASC', 'IN', 'LIKE', 'BETWEEN', 'IS', 'NULL', 'CASE', 'WHEN', 'THEN', 'ELSE', 'END', 'WITH', 'UNION']);

const tableWords = ['FROM', 'JOIN', 'INTO', 'UPDATE', 'TABLE'];

/**
 * Propositions à la position pos. explicit : demandé par Ctrl+Espace (sinon
 * rien n'est proposé tant qu'aucun caractère n'est saisi, sauf après un point).
 */
export function complete(doc: string, at: number, catalog: Catalog, explicit = false): Completion | null {
	// Seule l'instruction sous le curseur compte (alias, tableaux cités).
	const st = statementAt(doc, at);
	const base = st.from <= at && at <= st.to ? st.from : at;
	const end = base === st.from ? Math.max(st.to, at) : at;
	const c = completeIn(doc.slice(base, end), at - base, catalog, explicit, doc[at]);
	return c && { ...c, from: c.from + base };
}

function completeIn(src: string, pos: number, catalog: Catalog, explicit: boolean, next: string | undefined): Completion | null {
	const before = tokenize(src.slice(0, pos));
	const sig = before.filter((t) => t.kind !== 'ws' && t.kind !== 'comment');
	const last = before.at(-1);
	if (last && (last.kind === 'string' || last.kind === 'comment' || last.kind === 'number')) return null;

	// Mot en cours : un mot, ou un nom entre guillemets / crochets non refermé.
	let from = pos;
	let opener = '';
	let partial = false;
	if (last && last.kind === 'word' && last.from + last.text.length === pos) {
		from = last.from;
		partial = true;
	} else if (last && last.kind === 'quoted' && !closed(last.text)) {
		opener = last.text[0];
		from = last.from + 1;
		partial = true;
	}
	const prevSig = sig[sig.length - (partial ? 2 : 1)];
	const afterDot = prevSig?.text === '.';
	if (!partial && !afterDot && !explicit) return null;

	const closer = opener === '[' ? ']' : opener;
	const hasCloser = !!closer && next === closer;
	const write = (name: string) => (opener ? name + (hasCloser ? '' : closer) : quote(name));

	const a = analyze(src, catalog);
	const options: Suggestion[] = [];
	const columns = (o: (typeof catalog.objects)[number], boost: number, withTable: boolean) => {
		for (const c of o.columns)
			options.push({ label: c.name, apply: write(c.name), kind: 'column', detail: withTable ? `${o.name} · ${typeLabel(c.type)}` : typeLabel(c.type), boost });
	};

	if (afterDot) {
		const q = sig[sig.length - (partial ? 3 : 2)];
		const o = q && (q.kind === 'word' || q.kind === 'quoted') ? (a.aliases.get(q.value.toLocaleLowerCase('fr-FR')) ?? catalog.table(q.value)) : undefined;
		if (!o) return null;
		columns(o, 10, false);
		return { from, options };
	}

	const tables = () => {
		for (const o of catalog.objects)
			options.push({ label: o.name, apply: write(o.name), kind: 'table', detail: `tableau · ${o.rows.toLocaleString('fr-FR')} lignes`, boost: 5 });
	};
	if (prevSig && prevSig.kind === 'word' && tableWords.includes(prevSig.text.toUpperCase())) {
		tables();
		return { from, options };
	}

	if (a.scope.length > 0) for (const o of a.scope) columns(o, 8, a.scope.length > 1);
	else for (const o of catalog.objects) columns(o, 2, true);
	tables();
	if (!opener) {
		for (const f of FUNCTIONS) options.push({ label: f, apply: f + '(', kind: 'function', boost: -2 });
		for (const k of KEYWORDS) options.push({ label: k, apply: k, kind: 'keyword', boost: COMMON.has(k) ? 0 : -4 });
	}
	return { from, options };
}

function closed(text: string): boolean {
	const c = text[0] === '[' ? ']' : text[0];
	return text.length >= 2 && text.endsWith(c);
}
