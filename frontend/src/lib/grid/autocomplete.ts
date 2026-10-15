// Aide à la saisie des formules : suggestions de fonctions, de tableaux et
// de colonnes selon ce qui précède le curseur, et signature de la fonction
// en cours.

export interface Catalog {
	functions: { name: string; other: string; arity: string }[];
	tables: { name: string; columns: string[] | null }[];
}

export interface Suggestion {
	label: string;
	insert: string; // texte qui remplace le préfixe
	kind: 'function' | 'table' | 'column';
	detail: string;
}

export interface Completion {
	from: number; // début du préfixe remplacé
	items: Suggestion[];
}

const fold = (s: string) =>
	s
		.normalize('NFD')
		.replace(/[̀-ͯ]/g, '')
		.toLowerCase();

/** Vrai si le curseur est dans une chaîne entre guillemets. */
function inString(before: string): boolean {
	return ((before.match(/"/g) ?? []).length & 1) === 1;
}

/**
 * Suggestions pour le texte d'une formule et la position du curseur.
 * currentTable : tableau de la cellule éditée, pour [@Colonne].
 */
export function suggest(text: string, caret: number, cat: Catalog, currentTable?: string, max = 8): Completion | null {
	if (!text.trimStart().startsWith('=')) return null;
	const before = text.slice(0, caret);
	if (inString(before)) return null;

	// Colonne : après « [ » non refermé, éventuellement précédé d'un tableau.
	const open = before.lastIndexOf('[');
	if (open >= 0 && before.indexOf(']', open) < 0) {
		const table = /[\p{L}_][\p{L}\p{N}_.]*$/u.exec(before.slice(0, open))?.[0] ?? currentTable;
		const t = cat.tables.find((x) => fold(x.name) === fold(table ?? ''));
		if (!t) return null;
		let prefix = before.slice(open + 1);
		const at = prefix.startsWith('@');
		if (at) prefix = prefix.slice(1);
		const items = (t.columns ?? [])
			.filter((c) => fold(c).startsWith(fold(prefix)))
			.slice(0, max)
			.map((c) => ({ label: c, insert: (at ? '@' : '') + c + ']', kind: 'column' as const, detail: t.name }));
		return items.length ? { from: open + 1, items } : null;
	}

	// Fonction ou tableau : identifiant en cours de frappe.
	const m = /[\p{L}_][\p{L}\p{N}_.]*$/u.exec(before);
	if (!m) return null;
	const prefix = fold(m[0]);
	const items: Suggestion[] = [];
	for (const f of cat.functions) {
		if (fold(f.name).startsWith(prefix)) items.push({ label: f.name, insert: f.name + '(', kind: 'function', detail: f.arity });
	}
	for (const t of cat.tables) {
		if (fold(t.name).startsWith(prefix)) items.push({ label: t.name, insert: t.name + '[', kind: 'table', detail: 'tableau' });
	}
	// Un nom déjà complet n'a pas besoin de suggestion.
	const exact = items.length === 1 && fold(items[0].label) === prefix;
	return items.length && !exact ? { from: caret - m[0].length, items: items.slice(0, max) } : null;
}

/** Signature de la fonction dont le curseur est dans les parenthèses. */
export function signature(text: string, caret: number, cat: Catalog): string | null {
	if (!text.trimStart().startsWith('=')) return null;
	const before = text.slice(0, caret);
	let depth = 0;
	let quoted = false;
	for (let i = before.length - 1; i >= 0; i--) {
		const c = before[i];
		if (c === '"') quoted = !quoted;
		if (quoted) continue;
		if (c === ')') depth++;
		else if (c === '(') {
			if (depth === 0) {
				const name = /[\p{L}_][\p{L}\p{N}_.]*$/u.exec(before.slice(0, i))?.[0];
				const f = name && cat.functions.find((x) => fold(x.name) === fold(name) || fold(x.other) === fold(name));
				return f ? `${f.name} : ${f.arity}` : null;
			}
			depth--;
		}
	}
	return null;
}

/** Applique une suggestion : nouveau texte et position du curseur. */
export function apply(text: string, caret: number, from: number, s: Suggestion): { text: string; caret: number } {
	const out = text.slice(0, from) + s.insert + text.slice(caret);
	return { text: out, caret: from + s.insert.length };
}
