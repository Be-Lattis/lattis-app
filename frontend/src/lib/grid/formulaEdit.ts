// Saisie des formules dans la cellule et la barre de formule : vérification
// de la syntaxe avant l'envoi (mêmes règles et mêmes messages que l'analyseur
// Go, internal/formula/parse.go), parenthèses fermées d'office, références
// repérées pour être colorées dans l'éditeur et encadrées sur la grille, et
// références de plage insérées au glissé.
// (t est ici un jeton : la traduction s'appelle tr.)
import { formulaSep, t as tr } from '#lib/i18n/index.js';

/** La saisie est-elle une formule ? */
export const isFormula = (text: string) => /^\s*=/.test(text) && text.trim().length > 1;

// --- Analyse lexicale ----------------------------------------------------------

type Kind = 'num' | 'str' | 'ident' | 'ref' | 'err' | 'op' | 'lp' | 'rp' | 'sep' | 'range' | 'eof';

interface Tok {
	k: Kind;
	text: string; // texte tapé (contenu entre crochets pour ref)
	pos: number; // position dans le texte complet
	end: number;
}

export interface FormulaError {
	pos: number;
	end: number;
	msg: string;
}

class SyntaxErr extends Error {
	constructor(
		public pos: number,
		public end: number,
		msg: string
	) {
		super(msg);
	}
}

const ERRORS = ['#DIV/0!', '#VALEUR!', '#N/A', '#REF!', '#NOM?', '#NOMBRE!', '#CIRC!', '#VALUE!', '#NAME?', '#NUM!'];
const A1_RANGE = /^(?:(?:[\p{L}\w]+|'(?:[^']|'')+')!)?\$?[A-Za-z]{1,3}\$?\d+:\$?[A-Za-z]{1,3}\$?\d+/u;
const A1 = /^([A-Za-z]{1,3})([1-9][0-9]{0,6})$/;
const isLetter = (c: string) => /\p{L}/u.test(c);
const isDigit = (c: string) => c >= '0' && c <= '9';
const isSpace = (c: string) => /\s/.test(c);

/** Lit une adresse A1 (« C14 », « Feuille2!C14 ») sans les $. */
export function a1Address(text: string): { sheet: string; row: number; col: number } | null {
	let sheet = '';
	let addr = text;
	const k = text.lastIndexOf('!');
	if (k >= 0) {
		sheet = text.slice(0, k);
		addr = text.slice(k + 1);
		if (!sheet) return null;
	}
	const m = A1.exec(addr);
	if (!m) return null;
	let col = 0;
	for (const ch of m[1].toUpperCase()) col = col * 26 + ch.charCodeAt(0) - 64;
	return { sheet, row: Number(m[2]) - 1, col: col - 1 };
}

function lex(src: string, base: number, sep: string): Tok[] {
	const toks: Tok[] = [];
	const dec = sep === ';' ? ',' : '.';
	let i = 0;
	const push = (k: Kind, text: string, start: number) => toks.push({ k, text, pos: base + start, end: base + i });
	while (i < src.length) {
		const c = src[i];
		const start = i;
		if (isSpace(c)) {
			i++;
			continue;
		}
		if (c === '"') {
			i++;
			for (;;) {
				if (i >= src.length) throw new SyntaxErr(base + start, base + start + 1, tr('grid.syntax.unclosedQuote'));
				if (src[i] === '"') {
					if (src[i + 1] === '"') {
						i += 2;
						continue;
					}
					i++;
					break;
				}
				i++;
			}
			push('str', src.slice(start, i), start);
			continue;
		}
		if (isDigit(c) || (c === dec && isDigit(src[i + 1] ?? ''))) {
			while (i < src.length && (isDigit(src[i]) || src[i] === dec)) i++;
			if (src[i] === 'e' || src[i] === 'E') {
				i++;
				if (src[i] === '+' || src[i] === '-') i++;
				while (i < src.length && isDigit(src[i])) i++;
			}
			if (src.slice(start, i).split(dec).length > 2) throw new SyntaxErr(base + start, base + i, tr('grid.syntax.badNumber'));
			push('num', src.slice(start, i), start);
			continue;
		}
		if (c === '#') {
			const up = src.slice(i).toUpperCase();
			const name = ERRORS.find((e) => up.startsWith(e));
			if (!name) throw new SyntaxErr(base + start, base + start + 1, tr('grid.syntax.unknownError'));
			i += name.length;
			push('err', name, start);
			continue;
		}
		if (c === '[') {
			let depth = 0;
			let j = i;
			for (; j < src.length; j++) {
				if (src[j] === '[') depth++;
				else if (src[j] === ']' && --depth === 0) break;
			}
			if (j >= src.length) throw new SyntaxErr(base + start, base + start + 1, tr('grid.syntax.unclosedBracket'));
			i = j + 1;
			push('ref', src.slice(start + 1, j), start);
			continue;
		}
		if (c === '(' || c === ')') {
			i++;
			push(c === '(' ? 'lp' : 'rp', c, start);
			continue;
		}
		if (c === sep) {
			i++;
			push('sep', c, start);
			continue;
		}
		if ('+-*/^&%'.includes(c)) {
			i++;
			push('op', c, start);
			continue;
		}
		// « != » vaut « <> » (différent de).
		if (c === '!' && src[i + 1] === '=') {
			i += 2;
			push('op', '<>', start);
			continue;
		}
		if (c === '<' || c === '>' || c === '=') {
			let op = c;
			const n = src[i + 1];
			if ((c === '<' && (n === '>' || n === '=')) || (c === '>' && n === '=')) op += n;
			i += op.length;
			push('op', op, start);
			continue;
		}
		const m = A1_RANGE.exec(src.slice(i));
		if (m) {
			i += m[0].length;
			push('range', m[0], start);
			continue;
		}
		if (c === "'") {
			let j = i + 1;
			while (j < src.length && src[j] !== "'") j++;
			if (j + 1 < src.length && src[j + 1] === '!') {
				i = j + 2;
				while (i < src.length && (isLetter(src[i]) || isDigit(src[i]) || src[i] === '_' || src[i] === '$')) i++;
				push('ident', src.slice(start, i), start);
				continue;
			}
		}
		if (isLetter(c) || c === '_' || c === '$') {
			while (
				i < src.length &&
				(isLetter(src[i]) || isDigit(src[i]) || '_.$'.includes(src[i]) || (src[i] === '!' && (isLetter(src[i + 1] ?? '') || src[i + 1] === '$')))
			)
				i++;
			const text = src.slice(start, i);
			if (text.includes('$') && !a1Address(text.replaceAll('$', ''))) throw new SyntaxErr(base + start, base + i, tr('grid.syntax.unexpectedChar', { char: '$' }));
			push('ident', text, start);
			continue;
		}
		throw new SyntaxErr(base + start, base + start + 1, tr('grid.syntax.unexpectedChar', { char: c }));
	}
	toks.push({ k: 'eof', text: '', pos: base + src.length, end: base + src.length });
	return toks;
}

// --- Analyse syntaxique (vérification seule) -------------------------------------

const CELL = /^\s*[A-Za-z]{1,4}[1-9][0-9]{0,8}\s*$/;

class Checker {
	i = 0;
	constructor(
		private toks: Tok[],
		private sep: string
	) {}

	peek() {
		return this.toks[this.i];
	}
	next() {
		return this.toks[this.i++];
	}
	word(t: Tok) {
		if (t.k === 'ref') return `[${t.text}]`;
		return t.text.length > 20 ? t.text.slice(0, 20) : t.text;
	}

	comparison(): void {
		this.binary(0);
	}

	private static levels = [['=', '<>', '<', '<=', '>', '>='], ['&'], ['+', '-'], ['*', '/'], ['^']];

	binary(level: number): void {
		if (level >= Checker.levels.length) return this.unary();
		this.binary(level + 1);
		for (;;) {
			const t = this.peek();
			if (t.k !== 'op' || !Checker.levels[level].includes(t.text)) return;
			this.next();
			this.binary(level + 1);
		}
	}

	unary(): void {
		const t = this.peek();
		if (t.k === 'op' && (t.text === '-' || t.text === '+')) {
			this.next();
			return this.unary();
		}
		this.primary();
		while (this.peek().k === 'op' && this.peek().text === '%') this.next();
	}

	primary(): void {
		const t = this.next();
		switch (t.k) {
			case 'num':
			case 'str':
			case 'err':
			case 'range':
				return;
			case 'ref':
				return this.item();
			case 'lp': {
				this.comparison();
				const e = this.next();
				if (e.k !== 'rp') {
					if (e.k === 'eof') throw new SyntaxErr(t.pos, t.end, tr('grid.syntax.missingCloseParen'));
					throw new SyntaxErr(e.pos, e.end, tr('grid.syntax.missingOperatorOrParen', { word: this.word(e) }));
				}
				return;
			}
			case 'ident': {
				const p = this.peek();
				if (p.k === 'lp') return this.call(t);
				if (p.k === 'ref') {
					const inner = this.next();
					const word = t.text.slice(t.text.lastIndexOf('!') + 1);
					if (word.toLowerCase() === 'cell') {
						const parts = inner.text.split(':');
						if (parts.length > 2 || !parts.every((x) => CELL.test(x)))
							throw new SyntaxErr(t.pos, inner.end, tr('grid.syntax.badCellAddress', { addr: inner.text }));
						return;
					}
					return this.item();
				}
				return;
			}
		}
		throw this.missing(t);
	}

	/** [ligne] après une colonne : une expression, ou une plage de rangs 1:3. */
	item() {
		if (this.peek().k !== 'ref') return;
		const kt = this.next();
		const span = /^\s*(\d+)\s*:\s*(\d+)\s*$/.exec(kt.text);
		if (span) {
			if (Math.min(Number(span[1]), Number(span[2])) < 1)
				throw new SyntaxErr(kt.pos, kt.end, tr('grid.syntax.rowFromOne'));
			return;
		}
		const sub = new Checker(lex(kt.text, kt.pos + 1, this.sep), this.sep);
		sub.comparison();
		if (sub.peek().k !== 'eof') throw new SyntaxErr(kt.pos, kt.end, tr('grid.syntax.rowExpected'));
	}

	call(name: Tok) {
		this.next(); // (
		if (this.peek().k === 'rp') {
			this.next();
			return;
		}
		for (;;) {
			const k = this.peek().k;
			if (k !== 'sep' && k !== 'rp') this.comparison();
			const t = this.next();
			if (t.k === 'rp') return;
			if (t.k === 'eof') throw new SyntaxErr(t.pos, t.end, tr('grid.syntax.missingCloseParenOf', { name: name.text.toUpperCase() }));
			if (t.k !== 'sep') throw new SyntaxErr(t.pos, t.end, tr('grid.syntax.missingSepOrParen', { sep: this.sep, word: this.word(t) }));
		}
	}

	missing(t: Tok): SyntaxErr {
		const prev = this.i >= 2 ? this.toks[this.i - 2] : undefined;
		const after = prev && (prev.k === 'op' || prev.k === 'lp' || prev.k === 'sep') ? prev.text : null;
		const missingValue = () => (after !== null ? tr('grid.syntax.missingValueAfter', { word: after }) : tr('grid.syntax.missingValue'));
		// La faute est montrée à l'endroit où la valeur manque.
		const at = (msg: string) => (t.k === 'eof' && prev ? new SyntaxErr(prev.pos, prev.end, msg) : new SyntaxErr(t.pos, Math.max(t.end, t.pos + 1), msg));
		switch (t.k) {
			case 'eof':
				return prev ? at(missingValue()) : new SyntaxErr(t.pos, t.pos, tr('grid.syntax.empty'));
			case 'rp':
			case 'sep':
			case 'op':
				return at(after !== null ? missingValue() : tr('grid.syntax.missingValueBefore', { word: this.word(t) }));
		}
		return at(tr('grid.syntax.unexpected', { word: this.word(t) }));
	}

	extra(t: Tok): SyntaxErr {
		if (t.k === 'rp') return new SyntaxErr(t.pos, t.end, tr('grid.syntax.extraCloseParen'));
		if (t.k === 'sep') return new SyntaxErr(t.pos, t.end, tr('grid.syntax.sepOutside', { sep: this.sep }));
		return new SyntaxErr(t.pos, t.end, tr('grid.syntax.missingOperator', { word: this.word(t) }));
	}
}

/**
 * Vérifie la syntaxe d'une formule (texte complet, « = » compris) ; null si
 * elle est correcte. pos et end situent la faute dans le texte.
 */
export function checkFormula(text: string, sep: string = formulaSep()): FormulaError | null {
	const eq = text.indexOf('=');
	if (eq < 0) return null;
	try {
		const toks = lex(text.slice(eq + 1), eq + 1, sep);
		const c = new Checker(toks, sep);
		c.comparison();
		if (c.peek().k !== 'eof') throw c.extra(c.peek());
		return null;
	} catch (e) {
		if (e instanceof SyntaxErr) return { pos: e.pos, end: Math.max(e.end, e.pos), msg: e.message };
		throw e;
	}
}

/**
 * Ferme les parenthèses laissées ouvertes en fin de formule (comme Excel) :
 * « =SOMME(1;2 » devient « =SOMME(1;2) ». Les parenthèses des chaînes et
 * des crochets ne comptent pas ; une chaîne non fermée est laissée telle quelle.
 */
export function closeParens(text: string): string {
	if (!isFormula(text)) return text;
	let depth = 0;
	let brackets = 0;
	let str = false;
	for (const c of text) {
		if (str) {
			if (c === '"') str = false;
			continue;
		}
		if (c === '"') str = true;
		else if (c === '[') brackets++;
		else if (c === ']') brackets = Math.max(0, brackets - 1);
		else if (brackets === 0 && c === '(') depth++;
		else if (brackets === 0 && c === ')') depth--;
	}
	if (str || brackets > 0 || depth <= 0) return text;
	return text.trimEnd() + ')'.repeat(depth);
}

/** Position (dans le texte saisi) d'une erreur de syntaxe renvoyée par le serveur (« formule : position 9 : … »). */
export function serverErrorPos(text: string, message: string): number | null {
	const m = /position (\d+)/.exec(message);
	if (!m) return null;
	const eq = text.indexOf('=');
	return Math.min(text.length, eq + Number(m[1]));
}

// --- Références : coloration et cadres -----------------------------------------

/** Une référence d'une formule, telle qu'elle est écrite. */
export interface FormulaRef {
	start: number;
	end: number;
	text: string;
	/** Couleur : la même pour un même texte de référence. */
	color: string;
}

/** Couleurs des références (comme Excel : bleu, rouge, violet, vert…). */
export const REF_COLORS = ['#1f5fbf', '#c8302c', '#7a3fb8', '#1f8a3e', '#b8650a', '#0f8686', '#b0337f'];

/** Repère les références d'une formule (vide si le texte n'en est pas une ou ne se découpe pas). */
export function formulaRefs(text: string, sep: string = formulaSep()): FormulaRef[] {
	if (!isFormula(text)) return [];
	const eq = text.indexOf('=');
	let toks: Tok[];
	try {
		toks = lex(text.slice(eq + 1), eq + 1, sep);
	} catch {
		// Texte en cours de frappe (chaîne ou crochet non fermé) : ce qui précède.
		const cut = (() => {
			for (let n = text.length - 1; n > eq; n--) {
				try {
					return lex(text.slice(eq + 1, n), eq + 1, sep);
				} catch {
					/* plus court */
				}
			}
			return [];
		})();
		toks = cut;
	}
	const out: FormulaRef[] = [];
	const colors = new Map<string, string>();
	const add = (start: number, end: number) => {
		const t = text.slice(start, end);
		const k = t.toLowerCase().replace(/\s+/g, '');
		if (!colors.has(k)) colors.set(k, REF_COLORS[colors.size % REF_COLORS.length]);
		out.push({ start, end, text: t, color: colors.get(k)! });
	};
	// Index de ligne : un nombre ou un décalage ([2], [-1]) fait partie de la
	// référence ; une expression ([1+Tableau[Col][1]]) est colorée pour ses
	// propres références, la référence s'arrêtant avant elle.
	const index = (toks: Tok[], i: number, end: number): number => {
		const k = toks[i + 1];
		if (k?.k !== 'ref' || k.pos !== end) return end;
		if (!k.text.includes('[')) return toks[i + 1].end;
		let inner: Tok[] = [];
		try {
			inner = lex(k.text, k.pos + 1, sep);
		} catch {
			/* index en cours de frappe */
		}
		scan(inner);
		return end;
	};
	const scan = (toks: Tok[]) => {
		for (let i = 0; i < toks.length; i++) {
			const t = toks[i];
			if (t.k === 'range') {
				add(t.pos, t.end);
			} else if (t.k === 'ident' && toks[i + 1]?.k === 'ref' && toks[i + 1].pos === t.end) {
				i++;
				const col = toks[i].end;
				add(t.pos, index(toks, i, col));
				if (toks[i + 1]?.k === 'ref' && toks[i + 1].pos === col) i++;
			} else if (t.k === 'ref') {
				const end = index(toks, i, t.end);
				add(t.pos, end);
				if (toks[i + 1]?.k === 'ref' && toks[i + 1].pos === t.end) i++;
			} else if (t.k === 'ident' && toks[i + 1]?.k !== 'lp' && a1Address(t.text.replaceAll('$', '').replace(/^'(.*)'!/, '$1!'))) {
				add(t.pos, t.end);
			}
		}
	};
	scan(toks);
	return out.sort((a, b) => a.start - b.start);
}

/** Découpe un texte en morceaux colorés (références) et ordinaires. */
export function highlightParts(text: string, refs: FormulaRef[], err?: { pos: number; end: number } | null) {
	const parts: { text: string; color?: string; err?: boolean }[] = [];
	let at = 0;
	const plain = (to: number) => {
		if (to <= at) return;
		const s = text.slice(at, to);
		if (err && err.end > at && err.pos < to) {
			const a = Math.max(err.pos, at);
			const b = Math.min(Math.max(err.end, err.pos + 1), to);
			if (a > at) parts.push({ text: text.slice(at, a) });
			parts.push({ text: text.slice(a, b) || ' ', err: true });
			if (b < to) parts.push({ text: text.slice(b, to) });
		} else parts.push({ text: s });
		at = to;
	};
	for (const r of refs) {
		plain(r.start);
		parts.push({ text: text.slice(r.start, r.end), color: r.color });
		at = r.end;
	}
	plain(text.length);
	if (err && err.pos >= text.length) parts.push({ text: ' ', err: true });
	return parts;
}

/** Tableau de la feuille, pour situer les références. */
export interface RefTable {
	name: string;
	row: number; // en-tête
	col: number;
	columns: string[];
	rows: number; // lignes de données
	/** Rang (0 = première ligne) de la ligne de clé key, si le tableau a une clé ; undefined si inconnue. */
	keyRow?: (key: string) => number | undefined;
	keyed?: boolean;
}

export interface Rect {
	r0: number;
	c0: number;
	r1: number;
	c1: number;
}

const unquote = (s: string) => s.trim().replace(/^"(.*)"$/, '$1').replaceAll('""', '"');

/**
 * Cellules désignées par une référence, sur la feuille affichée (null si
 * elle est ailleurs ou introuvable). here : tableau et ligne de la formule.
 * away : la formule est sur une autre feuille (pointage inter-feuilles) ; une
 * adresse sans nom de feuille désigne alors la feuille de la formule.
 */
export function refRect(ref: string, tables: RefTable[], sheet: string, here?: { table: RefTable; row: number }, away = false): Rect | null {
	let text = ref.trim();
	// Feuille citée : seulement la feuille affichée.
	const bang = /^(?:'((?:[^']|'')+)'|([\p{L}\w]+))!/u.exec(text);
	if (bang) {
		const name = bang[1]?.replaceAll("''", "'") ?? bang[2];
		if (name.toLowerCase() !== sheet.toLowerCase()) return null;
		text = text.slice(bang[0].length);
	} else if (away && (/^cell\[/i.test(text) || /^\$?[A-Za-z]{1,3}\$?\d+(:|$)/.test(text))) return null;
	const addr = (s: string) => a1Address(s.trim().replaceAll('$', ''));
	const cell = /^cell\[([^\]]+)\]$/i.exec(text);
	if (cell || /^\$?[A-Za-z]{1,3}\$?\d+(:\$?[A-Za-z]{1,3}\$?\d+)?$/.test(text)) {
		const [a, b] = (cell ? cell[1] : text).split(':');
		const p = addr(a);
		const q = b ? addr(b) : p;
		if (!p || !q) return null;
		return { r0: Math.min(p.row, q.row), c0: Math.min(p.col, q.col), r1: Math.max(p.row, q.row), c1: Math.max(p.col, q.col) };
	}
	const m = /^([^[]*)\[([^\]]*)\](?:\[(.*)\])?$/.exec(text);
	if (!m) return null;
	const t = m[1] ? tables.find((x) => x.name.toLowerCase() === m[1].toLowerCase()) : here?.table;
	if (!t) return null;
	const ci = t.columns.findIndex((c) => c.toLowerCase() === m[2].replace(/^@/, '').trim().toLowerCase());
	if (ci < 0) return null;
	const c = t.col + ci;
	const last = t.row + t.rows;
	if (m[3] === undefined) {
		// [Col] : la ligne de la formule ; Tableau[Col] : toute la colonne.
		if (!m[1] && here) return { r0: here.row, c0: c, r1: here.row, c1: c };
		return { r0: t.row, c0: c, r1: last, c1: c };
	}
	const span = /^\s*(\d+)\s*:\s*(\d+)\s*$/.exec(m[3]);
	if (span) {
		const a = Math.min(Number(span[1]), Number(span[2]));
		const b = Math.max(Number(span[1]), Number(span[2]));
		if (a < 1 || a > t.rows) return null;
		return { r0: t.row + a, c0: c, r1: t.row + Math.min(b, t.rows), c1: c };
	}
	// [Col][-1], [Col][+2] : relatif à la ligne de la formule.
	const rel = /^\s*([+-])\s*(\d+)\s*$/.exec(m[3]);
	if (rel) {
		if (!here || here.table.name !== t.name) return null;
		const row = here.row + (rel[1] === '-' ? -1 : 1) * Number(rel[2]);
		if (row <= t.row || row > last) return null;
		return { r0: row, c0: c, r1: row, c1: c };
	}
	let n: number | undefined;
	if (t.keyed) n = t.keyRow?.(unquote(m[3]));
	else if (/^\s*\d+\s*$/.test(m[3])) n = Number(m[3]) - 1;
	if (n === undefined || n < 0 || n >= t.rows) return null;
	return { r0: t.row + 1 + n, c0: c, r1: t.row + 1 + n, c1: c };
}

/**
 * Référence d'une plage glissée en mode formule : Cell[A1:B3] sur des
 * cellules libres, Tableau[Col] sur toute une colonne de tableau,
 * Tableau[Col][1:3] sur une partie ([Col][1:3] dans le tableau de la
 * formule) ; null si la plage déborde d'une colonne de tableau.
 */
export function rangeRef(r: Rect, tables: RefTable[], hereTable?: RefTable): string | null {
	const r0 = Math.min(r.r0, r.r1);
	const r1 = Math.max(r.r0, r.r1);
	const c0 = Math.min(r.c0, r.c1);
	const c1 = Math.max(r.c0, r.c1);
	for (const t of tables) {
		const right = t.col + t.columns.length - 1;
		const last = t.row + t.rows;
		if (r0 > last || r1 < t.row || c0 > right || c1 < t.col) continue;
		if (c0 !== c1) return null;
		const col = t.columns[c0 - t.col];
		const name = t === hereTable ? '' : t.name;
		const from = Math.max(r0 - t.row, 1);
		const to = Math.min(r1 - t.row, t.rows);
		if ((from <= 1 && to >= t.rows) || from > to) return `${t.name}[${col}]`;
		return `${name}[${col}][${from}:${to}]`;
	}
	const a = (row: number, col: number) => {
		let s = '';
		for (let n = col + 1; n > 0; n = Math.floor((n - 1) / 26)) s = String.fromCharCode(65 + ((n - 1) % 26)) + s;
		return s + (row + 1);
	};
	return r0 === r1 && c0 === c1 ? `Cell[${a(r0, c0)}]` : `Cell[${a(r0, c0)}:${a(r1, c1)}]`;
}

// --- Largeur de l'éditeur ---------------------------------------------------------

/**
 * Largeur de l'éditeur de cellule : celle de la cellule, élargie vers la
 * droite tant que le texte dépasse et que les cellules voisines sont vides
 * (comme Excel), sans dépasser maxWidth ; au-delà, le texte passe à la ligne.
 */
export function editorWidth(textWidth: number, col: number, colWidth: (c: number) => number, empty: (c: number) => boolean, maxWidth: number): number {
	let w = colWidth(col);
	for (let c = col + 1; w < textWidth && empty(c); c++) {
		const next = w + colWidth(c);
		if (next > maxWidth) return Math.max(colWidth(col), maxWidth);
		w = next;
	}
	return Math.min(w, Math.max(colWidth(col), maxWidth));
}

/**
 * Saisie dans la barre de formule montrée dans sa cellule : le texte est
 * coupé au bord de la cellule (il ne passe pas sur les valeurs voisines) et
 * défile vers la gauche pour que le curseur reste visible, comme dans un
 * champ de saisie. caretX : largeur du texte jusqu'au curseur ; textW : du
 * texte entier ; avail : largeur visible. Renvoie le décalage (≥ 0).
 */
export function previewShift(caretX: number, textW: number, avail: number): number {
	if (textW <= avail || avail <= 0) return 0;
	return Math.min(Math.max(0, caretX - avail), textW - avail);
}

/**
 * Référence d'une cellule pour une formule du tableau here (ligne here.row),
 * comme un clic l'écrit : [Col] sur la même ligne, [Col][-1] / [Col][+2] sur
 * une autre ligne du même tableau (relatif, choix de Louis), Tableau[Col][n]
 * dans un autre tableau, Tableau[Col] sur un en-tête, Cell[A1] hors tableau.
 */
export function cellRefText(row: number, col: number, tables: RefTable[], here?: { table: RefTable; row: number }): string {
	const t = tables.find((x) => row >= x.row && row <= x.row + x.rows && col >= x.col && col < x.col + x.columns.length);
	if (!t) return 'Cell[' + colLetters(col) + (row + 1) + ']';
	const name = t.columns[col - t.col];
	if (row === t.row) return `${t.name}[${name}]`;
	if (here && here.table.name === t.name && here.row > t.row) {
		const k = row - here.row;
		return k === 0 ? `[${name}]` : `[${name}][${k > 0 ? '+' : ''}${k}]`;
	}
	return `${t.name}[${name}][${row - t.row}]`;
}

/**
 * Référence d'une cellule pointée au clic pendant la saisie d'une formule
 * écrite en at (null : formule d'une autre feuille). Une formule dans un
 * tableau, ou juste à sa droite ou à sa gauche sur une de ses lignes (elle
 * y devient une colonne calculée), lit sa ligne ([Col], [Col][-1]). Une
 * formule de cellule libre veut une valeur : une cellule de données donne
 * Tableau[Col][n], cette seule valeur (n : rang de la ligne, 1 = première,
 * même si le tableau a une clé, choix de Louis), jamais toute la colonne ;
 * seul l'en-tête donne Tableau[Col] (qui se déverse sous une cellule libre).
 */
export function pointedRef(row: number, col: number, tables: RefTable[], at: { row: number; col: number } | null): string {
	const inside = (t: RefTable, c: number) => c >= t.col && c < t.col + t.columns.length;
	const home = at
		? tables.find(
				(t) =>
					at.row > t.row &&
					at.row <= t.row + t.rows &&
					(inside(t, at.col) || at.col === t.col + t.columns.length || at.col === t.col - 1)
			)
		: undefined;
	return cellRefText(row, col, tables, home && at ? { table: home, row: at.row } : undefined);
}

function colLetters(c: number): string {
	let s = '';
	for (c++; c > 0; c = Math.floor((c - 1) / 26)) s = String.fromCharCode(65 + ((c - 1) % 26)) + s;
	return s;
}

/**
 * Nouvelle référence d'un marqueur de formule déplacé de (dr, dc) sur la
 * grille (rect : sa plage avant) : la même forme qu'au clic pour une
 * cellule, une plage sinon ; null si la plage ne se décrit pas. Une colonne
 * entière d'un tableau reste une colonne entière : déposée dans un tableau
 * (to : la cellule sous la souris), celle de ce tableau ; ailleurs, la même
 * hauteur, sans changer de lignes.
 */
export function movedRef(rect: Rect, dr: number, dc: number, tables: RefTable[], here?: { table: RefTable; row: number }, to?: { row: number; col: number }): string | null {
	const whole = (x: RefTable, r: Rect) => r.r0 === x.row && r.r1 === x.row + x.rows && r.c0 === r.c1 && r.c0 >= x.col && r.c0 < x.col + x.columns.length;
	if (tables.some((x) => whole(x, rect))) {
		const c = rect.c0 + dc;
		const row = to?.row ?? rect.r0 + dr;
		const t = tables.find((x) => row >= x.row && row <= x.row + x.rows && c >= x.col && c < x.col + x.columns.length);
		if (t) return `${t.name}[${t.columns[c - t.col]}]`;
		dr = 0;
	}
	const r: Rect = { r0: rect.r0 + dr, r1: rect.r1 + dr, c0: rect.c0 + dc, c1: rect.c1 + dc };
	if (r.r0 < 0 || r.c0 < 0) return null;
	if (r.r0 === r.r1 && r.c0 === r.c1) return cellRefText(r.r0, r.c0, tables, here);
	// Colonne entière d'un tableau (en-tête compris) : Tableau[Col].
	const t = tables.find((x) => r.r0 === x.row && r.r1 === x.row + x.rows && r.c0 === r.c1 && r.c0 >= x.col && r.c0 < x.col + x.columns.length);
	if (t) return `${t.name}[${t.columns[r.c0 - t.col]}]`;
	return rangeRef(r, tables, here?.table);
}
