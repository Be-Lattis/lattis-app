// Aide à la saisie des formules : suggestions de fonctions, de tableaux et
// de colonnes selon ce qui précède le curseur, et signature de la fonction
// en cours.

/** Argument d'une fonction (nom français, description, facultatif). */
export interface ArgInfo {
	name: string;
	desc: string;
	optional?: boolean;
	/** Ce que l'argument attend : "table" (nom de tableau), "column" (Tableau[Colonne]), "" (une valeur). */
	kind?: string;
}

/** Fonction du catalogue, avec sa documentation (internal/formula/docs.go). */
export interface FunctionDoc {
	name: string;
	other: string;
	arity: string;
	category?: string;
	description?: string;
	signature?: string;
	args?: ArgInfo[] | null;
	repeat?: number; // nombre de derniers arguments qui se répètent
	example?: string;
}

export interface Catalog {
	functions: FunctionDoc[];
	tables: { name: string; columns: string[] | null }[];
	names?: string[]; // cellules nommées
}

export interface Suggestion {
	label: string;
	insert: string; // texte qui remplace le préfixe
	kind: 'function' | 'table' | 'column' | 'name';
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
 * currentTable : tableau de la cellule éditée, pour [Colonne] (valeur de la ligne).
 */
export function suggest(text: string, caret: number, cat: Catalog, currentTable?: string, max = 8): Completion | null {
	if (!text.trimStart().startsWith('=')) return null;
	const before = text.slice(0, caret);
	if (inString(before)) return null;

	// Colonne : après « [ » non refermé, éventuellement précédé d'un tableau.
	const open = before.lastIndexOf('[');
	if (open >= 0 && before.indexOf(']', open) < 0) {
		const explicit = /[\p{L}_][\p{L}\p{N}_.]*$/u.exec(before.slice(0, open))?.[0];
		const table = explicit ?? currentTable;
		const t = cat.tables.find((x) => fold(x.name) === fold(table ?? ''));
		if (!t) return null;
		let prefix = before.slice(open + 1);
		if (prefix.startsWith('@')) prefix = prefix.slice(1); // forme Excel, acceptée
		const cols = (t.columns ?? []).filter((c) => fold(c).startsWith(fold(prefix)));
		// Nomenclature de Lattis : [Colonne] dans le tableau est la valeur de
		// la ligne ; Tableau[Colonne] toute la colonne.
		const row = !explicit && !!currentTable;
		const items = cols
			.slice(0, max)
			.map((c) => ({ label: c, insert: c + ']', kind: 'column' as const, detail: row ? 'valeur de la ligne' : t.name }));
		return items.length ? { from: open + 1, items } : null;
	}

	// Fonction ou tableau : identifiant en cours de frappe.
	const m = /[\p{L}_][\p{L}\p{N}_.]*$/u.exec(before);
	if (!m) return null;
	const prefix = fold(m[0]);
	const items: Suggestion[] = [];
	for (const f of cat.functions) {
		if (fold(f.name).startsWith(prefix)) items.push({ label: f.name, insert: f.name + '(', kind: 'function', detail: f.description || f.arity });
	}
	for (const t of cat.tables) {
		if (fold(t.name).startsWith(prefix)) items.push({ label: t.name, insert: t.name + '[', kind: 'table', detail: 'tableau' });
	}
	for (const n of cat.names ?? []) {
		if (fold(n).startsWith(prefix)) items.push({ label: n, insert: n, kind: 'name', detail: 'cellule nommée' });
	}
	// Un nom déjà complet n'a pas besoin de suggestion.
	const exact = items.length === 1 && fold(items[0].label) === prefix;
	return items.length && !exact ? { from: caret - m[0].length, items: items.slice(0, max) } : null;
}

/** Appel de fonction qui contient le curseur : nom et rang (0 = premier) de l'argument en cours. */
export interface CallAt {
	name: string;
	index: number;
}

/**
 * Fonction dont le curseur est dans les parenthèses, et argument en cours :
 * les séparateurs sont comptés au niveau de cet appel, sans tenir compte des
 * chaînes, des appels imbriqués, des noms de colonnes entre crochets
 * ([Prix (€)]), des noms de feuilles entre apostrophes ni des constantes
 * matricielles ({",";";"} : un seul argument). La ligne ou la clé
 * de Tableau[Colonne][clé] est une expression, qui peut contenir un appel.
 */
export function callAt(text: string, caret: number, sep = ';'): CallAt | null {
	if (!text.trimStart().startsWith('=')) return null;
	const before = text.slice(0, caret);
	// Pile des parenthèses (avec le nom de la fonction, vide pour un simple
	// regroupement) et des crochets de clé.
	const stack: { kind: '(' | '[' | '{'; name: string; index: number }[] = [];
	for (let i = 0; i < before.length; i++) {
		const c = before[i];
		if (c === '"' || c === "'") {
			const end = before.indexOf(c, i + 1);
			if (end < 0) break; // curseur dans la chaîne
			i = end; // "" (guillemet doublé) : deux chaînes accolées, même résultat
		} else if (c === '[') {
			if (before[i - 1] === ']') {
				stack.push({ kind: '[', name: '', index: 0 });
			} else {
				// Nom de colonne ou adresse : texte littéral jusqu'au « ] ».
				const end = before.indexOf(']', i + 1);
				if (end < 0) break; // curseur dans le nom
				i = end;
			}
		} else if (c === '(') {
			const name = /[\p{L}_][\p{L}\p{N}_.]*$/u.exec(before.slice(0, i))?.[0] ?? '';
			stack.push({ kind: '(', name, index: 0 });
		} else if (c === '{') {
			// Constante matricielle {",";";"} : ses « ; » séparent des lignes.
			stack.push({ kind: '{', name: '', index: 0 });
		} else if (c === ')' || c === ']' || c === '}') {
			const k = c === ')' ? '(' : c === ']' ? '[' : '{';
			while (stack.length && stack[stack.length - 1].kind !== k) stack.pop();
			stack.pop();
		} else if (c === sep && stack.length) {
			stack[stack.length - 1].index++;
		}
	}
	for (let i = stack.length - 1; i >= 0; i--) {
		const f = stack[i];
		if (f.kind === '(' && f.name) return { name: f.name, index: f.index };
	}
	return null;
}

/** Fonction dont le curseur est dans les parenthèses, et rang de l'argument en cours. */
export function signature(text: string, caret: number, cat: Catalog): { fn: FunctionDoc; index: number } | null {
	const c = callAt(text, caret);
	if (!c) return null;
	const fn = cat.functions.find((x) => fold(x.name) === fold(c.name) || fold(x.other) === fold(c.name));
	return fn ? { fn, index: c.index } : null;
}

/** Morceau de signature ; on : argument en cours (en gras). */
export interface SigPart {
	text: string;
	on?: boolean;
}

/** Nom d'un argument répété pour la n-ième occurrence (valeur1 → valeur3). */
const numbered = (name: string, n: number) => name.replace(/1$/, String(n));

/**
 * Signature à afficher pendant la saisie, comme Excel :
 * SOMME.SI(plage; **critère**; [plage_somme]), SOMME(valeur1; [valeur2]; …),
 * avec l'argument en cours (nom numéroté s'il est répété) et sa description.
 */
export function signatureParts(fn: FunctionDoc, index: number): { parts: SigPart[]; arg: ArgInfo | null } {
	const args = fn.args ?? [];
	if (pairsForm(fn) && args.length === 3) return pairsParts(fn, args, index);
	const rep = Math.min(fn.repeat ?? 0, args.length);
	const off = args.length - rep;
	// Occurrence (2, 3…) et position dans le groupe répété de l'argument en cours.
	const group = rep && index >= args.length ? Math.floor((index - off) / rep) + 1 : 0;
	const inGroup = group ? (index - off) % rep : -1;
	const parts: SigPart[] = [{ text: fn.name + '(' }];
	let arg: ArgInfo | null = null;
	const push = (text: string, on: boolean) => {
		if (parts.length > 1) parts.push({ text: '; ' });
		parts.push(on ? { text, on } : { text });
	};
	args.forEach((a, i) => {
		if (i === index) arg = a;
		push(a.optional ? `[${a.name}]` : a.name, i === index);
	});
	if (rep) {
		const n = Math.max(group, 2);
		if (n > 2) push('…', false);
		push('[', false);
		for (let j = 0; j < rep; j++) {
			const a = args[off + j];
			const name = numbered(a.name, n);
			if (j === inGroup) arg = { ...a, name };
			if (j > 0) parts.push({ text: '; ' });
			parts.push(j === inGroup ? { text: name, on: true } : { text: name });
		}
		parts.push({ text: ']' });
		push('…', false);
	}
	parts.push({ text: ')' });
	return { parts, arg };
}

/** LET : des paires nom; valeur qui se répètent, puis le calcul (args : nom1, valeur1, calcul). */
const pairsForm = (fn: FunctionDoc) => fold(fn.name) === 'let' || fold(fn.other) === 'let';

/**
 * Signature de LET : LET(nom1; valeur1; [nom2; valeur2]; …; calcul). Après la
 * première paire, un argument de rang pair est le nom suivant ou le calcul
 * (les deux en gras, comme le « calcul_ou_nom2 » d'Excel).
 */
function pairsParts(fn: FunctionDoc, args: ArgInfo[], index: number): { parts: SigPart[]; arg: ArgInfo | null } {
	const [nom, valeur, calcul] = args;
	const k = index < 2 ? 2 : Math.floor(index / 2) + 1; // paire en cours (2, 3…)
	const nameOrCalc = index >= 2 && index % 2 === 0;
	const value = index >= 3 && index % 2 === 1;
	const nk = numbered(nom.name, k);
	const parts: SigPart[] = [{ text: fn.name + '(' }];
	const push = (text: string, on: boolean) => {
		if (parts.length > 1) parts.push({ text: '; ' });
		parts.push(on ? { text, on } : { text });
	};
	push(nom.name, index === 0);
	push(valeur.name, index === 1);
	if (k > 2) push('…', false);
	push('[', false);
	parts.push(nameOrCalc ? { text: nk, on: true } : { text: nk });
	parts.push({ text: '; ' });
	const vk = numbered(valeur.name, k);
	parts.push(value ? { text: vk, on: true } : { text: vk });
	parts.push({ text: ']' });
	push('…', false);
	push(calcul.name, nameOrCalc);
	parts.push({ text: ')' });
	let arg: ArgInfo | null = null;
	if (index === 0) arg = nom;
	else if (index === 1) arg = valeur;
	else if (value) arg = { ...valeur, name: vk };
	else arg = { ...calcul, name: `${calcul.name} ou ${nk}`, desc: `${calcul.desc} Ou un nom de plus (${nk}), suivi de sa valeur.` };
	return { parts, arg };
}

/** Applique une suggestion : nouveau texte et position du curseur. */
export function apply(text: string, caret: number, from: number, s: Suggestion): { text: string; caret: number } {
	const out = text.slice(0, from) + s.insert + text.slice(caret);
	return { text: out, caret: from + s.insert.length };
}

/**
 * Ce qu'attend l'argument en cours de saisie, pour le clic dans la grille :
 * un tableau (son nom), une colonne (Tableau[Colonne]) ou une valeur (la
 * cellule : [Col], Tableau[Col][k], Cell[A1]). "" hors d'une fonction connue.
 */
export function argKindAt(text: string, caret: number, cat: Catalog): '' | 'table' | 'column' {
	const s = signature(text, caret, cat);
	if (!s) return '';
	const k = signatureParts(s.fn, s.index).arg?.kind;
	return k === 'table' || k === 'column' ? k : '';
}

/**
 * Après une référence pointée à la souris (choix de Louis) : la fonction en
 * cours attend-elle encore un argument obligatoire après celui-ci ? Alors
 * la saisie passe d'elle-même à l'argument suivant. Pas après le dernier
 * argument, ni avant un argument facultatif.
 */
export function needsNextArg(text: string, caret: number, cat: Catalog): boolean {
	const s = signature(text, caret, cat);
	if (!s) return false;
	const args = s.fn.args ?? [];
	const rep = Math.min(s.fn.repeat ?? 0, args.length);
	const off = args.length - rep;
	const i = s.index;
	// LET : après une valeur vient un nom ou le calcul ; après le calcul, rien.
	if (pairsForm(s.fn)) return i % 2 === 1 || i === 0;
	if (i < args.length - 1) return !args[i + 1].optional;
	// Groupe répété (plage_critères2; critère2) : le reste du groupe suit.
	return rep > 1 && i >= off && (i - off) % rep < rep - 1;
}

/**
 * Passe à l'argument suivant : le curseur saute un « ; » déjà là, sinon
 * « ; » est inséré. Rend le nouveau texte et la position du curseur.
 */
export function toNextArg(text: string, caret: number): { text: string; caret: number } {
	const rest = text.slice(caret);
	const m = /^\s*;\s?/.exec(rest);
	if (m) return { text, caret: caret + m[0].length };
	return { text: text.slice(0, caret) + '; ' + rest, caret: caret + 2 };
}
