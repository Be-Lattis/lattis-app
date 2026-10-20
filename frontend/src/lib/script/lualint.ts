// Linter des scripts Lua (onglet Script).
//
// Analyse le texte à chaque frappe, sans passer par le backend :
// - erreurs de syntaxe, avec la grammaire de gopher-lua (Lua 5.1 plus goto),
//   et un message en français qui dit quoi corriger (« end » manquant, != au
//   lieu de ~=, accent dans un nom, += ...) ;
// - avertissements tirés de la référence (reference.ts) : nom non défini,
//   fonction absente d'une bibliothèque (db.qury) ou retirée du bac à sable
//   (io, os.execute), mauvais nombre d'arguments, db:query au lieu de
//   db.query, variable globale implicite.
//
// Le moteur revérifie la syntaxe avant d'exécuter (script.Check) : le linter
// sert à voir les erreurs plus tôt, pas à les garantir.

import { byName, globals, libraries, unavailable, type RefEntry } from './reference';
// t s'appelle tr ici : t désigne partout un jeton (Token) dans l'analyseur.
import { t as tr } from '#lib/i18n/index.js';

export type Severity = 'error' | 'warning' | 'info';

export interface LintDiagnostic {
	from: number;
	to: number;
	severity: Severity;
	message: string;
}

// --- Lexique -------------------------------------------------------------------

type TokType = 'name' | 'kw' | 'number' | 'string' | 'op' | 'eof';

interface Token {
	type: TokType;
	value: string;
	from: number;
	to: number;
	/** Un saut de ligne précède le jeton. */
	nl: boolean;
}

const keywords = new Set([
	'and', 'break', 'do', 'else', 'elseif', 'end', 'false', 'for', 'function', 'goto', 'if', 'in',
	'local', 'nil', 'not', 'or', 'repeat', 'return', 'then', 'true', 'until', 'while'
]);

const ops3 = ['...'];
const ops2 = ['==', '~=', '<=', '>=', '..', '::'];
const ops1 = '+-*/%^#<>=(){}[];:,.';

class LintError extends Error {
	from: number;
	to: number;
	constructor(message: string, from: number, to: number) {
		super(message);
		this.from = from;
		this.to = to;
	}
}

const isAlpha = (c: string) => (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c === '_';
const isDigit = (c: string) => c >= '0' && c <= '9';
const isHex = (c: string) => isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F');
const isSpace = (c: string) => c === ' ' || c === '\t' || c === '\n' || c === '\r' || c === '\f' || c === '\v';
const isLetter = (c: string) => /\p{L}/u.test(c);

function tokenize(src: string): Token[] {
	const out: Token[] = [];
	let i = 0;
	let nl = false;
	const n = src.length;

	// Fin d'un texte ou commentaire long ouvert en i (sur « [ »), ou -1.
	const longBracket = (start: number): { level: number; end: number } | null => {
		let j = start + 1;
		let level = 0;
		while (src[j] === '=') {
			level++;
			j++;
		}
		if (src[j] !== '[') return null;
		const close = ']' + '='.repeat(level) + ']';
		const k = src.indexOf(close, j + 1);
		return { level, end: k < 0 ? -1 : k + close.length };
	};

	while (i < n) {
		const c = src[i];
		if (isSpace(c)) {
			if (c === '\n' || c === '\r') nl = true;
			i++;
			continue;
		}
		const start = i;
		if (c === '-' && src[i + 1] === '-') {
			if (src[i + 2] === '[') {
				const lb = longBracket(i + 2);
				if (lb) {
					if (lb.end < 0) throw new LintError(tr('script.lint.longComment'), start, start + 4);
					i = lb.end;
					continue;
				}
			}
			while (i < n && src[i] !== '\n' && src[i] !== '\r') i++;
			continue;
		}
		if (isAlpha(c)) {
			while (i < n && (isAlpha(src[i]) || isDigit(src[i]))) i++;
			if (i < n && isLetter(src[i])) {
				let j = i;
				while (j < n && (isAlpha(src[j]) || isDigit(src[j]) || isLetter(src[j]))) j++;
				const word = src.slice(start, j);
				const prev = out[out.length - 1];
				const hint = prev?.value === '.' ? tr('script.lint.accent.index', { quoted: JSON.stringify(word), word }) : tr('script.lint.accent.plain');
				throw new LintError(tr('script.lint.accent.name', { word, hint }), start, j);
			}
			const word = src.slice(start, i);
			out.push({ type: keywords.has(word) ? 'kw' : 'name', value: word, from: start, to: i, nl });
			nl = false;
			continue;
		}
		if (isDigit(c) || (c === '.' && isDigit(src[i + 1] ?? ''))) {
			if (c === '0' && (src[i + 1] === 'x' || src[i + 1] === 'X')) {
				i += 2;
				const h = i;
				while (i < n && isHex(src[i])) i++;
				if (i === h) throw new LintError(tr('script.lint.hex'), start, i);
			} else {
				while (i < n && isDigit(src[i])) i++;
				// Comme gopher-lua : un point après les chiffres appartient au nombre (3..2 est une erreur).
				if (src[i] === '.') {
					i++;
					while (i < n && isDigit(src[i])) i++;
				}
				if (src[i] === 'e' || src[i] === 'E') {
					i++;
					if (src[i] === '+' || src[i] === '-') i++;
					const e = i;
					while (i < n && isDigit(src[i])) i++;
					if (i === e) throw new LintError(tr('script.lint.exponent'), start, i);
				}
			}
			out.push({ type: 'number', value: src.slice(start, i), from: start, to: i, nl });
			nl = false;
			continue;
		}
		if (c === '"' || c === "'") {
			i++;
			while (i < n && src[i] !== c) {
				if (src[i] === '\n' || src[i] === '\r') break;
				if (src[i] === '\\') {
					i++;
					if (src[i] === '\r' && src[i + 1] === '\n') i++;
				}
				i++;
			}
			if (i >= n || src[i] !== c)
				throw new LintError(c === '"' ? tr('script.lint.unterminated.double') : tr('script.lint.unterminated.single'), start, Math.min(i, n));
			i++;
			out.push({ type: 'string', value: src.slice(start, i), from: start, to: i, nl });
			nl = false;
			continue;
		}
		if (c === '[' && (src[i + 1] === '[' || src[i + 1] === '=')) {
			const lb = longBracket(i);
			if (lb) {
				if (lb.end < 0) throw new LintError(tr('script.lint.longString'), start, start + 2);
				i = lb.end;
				out.push({ type: 'string', value: src.slice(start, i), from: start, to: i, nl });
				nl = false;
				continue;
			}
		}
		const three = src.slice(i, i + 3);
		const two = src.slice(i, i + 2);
		let op = '';
		if (ops3.includes(three)) op = three;
		else if (ops2.includes(two)) op = two;
		else if (ops1.includes(c)) op = c;
		if (op) {
			i += op.length;
			out.push({ type: 'op', value: op, from: start, to: i, nl });
			nl = false;
			continue;
		}
		throw badChar(src, i);
	}
	out.push({ type: 'eof', value: '', from: n, to: n, nl });
	return out;
}

function badChar(src: string, i: number): LintError {
	const c = String.fromCodePoint(src.codePointAt(i)!);
	const at = (msg: string, len = c.length) => new LintError(msg, i, i + len);
	if (c === '!' && src[i + 1] === '=') return at(tr('script.lint.char.neq'), 2);
	if (c === '!') return at(tr('script.lint.char.not'));
	if (c === '~') return at(tr('script.lint.char.tilde'));
	if (c === '&') return at(tr('script.lint.char.and'), src[i + 1] === '&' ? 2 : 1);
	if (c === '|') return at(tr('script.lint.char.or'), src[i + 1] === '|' ? 2 : 1);
	if ('«»“”‘’'.includes(c)) return at(tr('script.lint.char.typographic', { char: c }));
	if (c === '`') return at(tr('script.lint.char.backtick'));
	if (isLetter(c)) return at(tr('script.lint.accent.name', { word: c, hint: tr('script.lint.accent.plain') }));
	return at(tr('script.lint.char.unexpected', { char: c }));
}

// --- Analyse -----------------------------------------------------------------

/** Description d'une expression, juste ce qu'il faut pour les vérifications. */
interface Expr {
	kind: 'call' | 'vararg' | 'name' | 'index' | 'other';
	from: number;
	to: number;
	/** Chemin statique depuis une globale : "db", "db.query". */
	path?: string;
	/** Vérifications à faire si l'expression est lue (et pas affectée). */
	onRead: (() => void)[];
}

interface FuncState {
	vararg: boolean;
	loops: number;
	parent: FuncState | null;
	/** Indice de la première portée de la fonction dans Parser.scopes. */
	scopeBase: number;
}

interface Scope {
	vars: Map<string, number>;
	/** Déclarations locales, doublons compris (comme les registres de gopher-lua). */
	count: number;
	last: string;
	labels: Map<string, { line: number; locals: number }>;
	/** goto pas encore résolus (étiquette plus loin, ou dans un bloc englobant). */
	gotos: { name: string; tok: Token; locals: number }[];
}

const newScope = (): Scope => ({ vars: new Map(), count: 0, last: '', labels: new Map(), gotos: [] });

interface Pending {
	run: (userGlobals: Set<string>) => void;
}

const blockEnd = new Set(['eof', 'end', 'else', 'elseif', 'until']);

const opener: Record<string, string> = {
	if: '« if »',
	while: '« while »',
	for: '« for »',
	do: '« do »',
	function: '« function »',
	repeat: '« repeat »'
};

// Priorités des opérateurs binaires [gauche, droite], comme dans Lua 5.1.
const binPriority: Record<string, [number, number]> = {
	or: [1, 1],
	and: [2, 2],
	'<': [3, 3], '>': [3, 3], '<=': [3, 3], '>=': [3, 3], '==': [3, 3], '~=': [3, 3],
	'..': [5, 4],
	'+': [6, 6], '-': [6, 6],
	'*': [7, 7], '/': [7, 7], '%': [7, 7],
	'^': [10, 9]
};
const unaryPriority = 8;

class Parser {
	private toks: Token[];
	private p = 0;
	private scopes: Scope[] = [newScope()];
	private fn: FuncState = { vararg: true, loops: 0, parent: null, scopeBase: 0 };
	private pending: Pending[] = [];
	/** Globales affectées par le script : nom → première affectation. */
	userGlobals = new Map<string, { from: number; to: number }>();
	private userFuncs: string[] = [];
	/** Noms globaux appelés dans le script (f(), f"x", f{}). */
	private calledGlobals = new Set<string>();
	diags: LintDiagnostic[] = [];

	constructor(private src: string) {
		this.toks = tokenize(src);
	}

	// Jetons

	private get tok(): Token {
		return this.toks[this.p];
	}
	private peek(k = 1): Token {
		return this.toks[Math.min(this.p + k, this.toks.length - 1)];
	}
	private next(): Token {
		const t = this.tok;
		if (t.type !== 'eof') this.p++;
		return t;
	}
	private is(v: string): boolean {
		const t = this.tok;
		return (t.type === 'kw' || t.type === 'op') && t.value === v;
	}
	private accept(v: string): boolean {
		if (this.is(v)) {
			this.p++;
			return true;
		}
		return false;
	}
	private describe(t: Token): string {
		if (t.type === 'eof') return tr('script.lint.eof');
		const v = t.value.length > 20 ? t.value.slice(0, 20) + '…' : t.value;
		return `« ${v} »`;
	}
	private fail(msg: string, t: Token = this.tok): never {
		const from = t.type === 'eof' ? Math.max(0, this.src.length - 1) : t.from;
		const to = t.type === 'eof' ? this.src.length : t.to;
		throw new LintError(msg, from, Math.max(to, from + 1));
	}
	private expect(v: string, what?: string): Token {
		if (this.is(v)) return this.next();
		const found = this.describe(this.tok);
		let msg = what ? tr('script.lint.expectedWhat', { token: v, what, found }) : tr('script.lint.expected', { token: v, found });
		if ((v === 'then' || v === 'do') && this.is('=')) msg += tr('script.lint.compareHint');
		this.fail(msg);
	}
	private closeBlock(kw: string, open: Token): void {
		if (this.is('end')) {
			this.next();
			return;
		}
		const line = this.lineOf(open.from);
		let msg = tr('script.lint.endExpected', { opener: opener[kw] ?? `« ${kw} »`, line: String(line), found: this.describe(this.tok) });
		if (this.tok.type === 'name' && this.tok.value === 'continue') msg = tr('script.lint.noContinue');
		this.fail(msg);
	}
	private name(what: string): Token {
		const t = this.tok;
		if (t.type === 'name') return this.next();
		if (t.type === 'kw') this.fail(tr('script.lint.reserved', { word: t.value, what }));
		this.fail(tr('script.lint.nameExpected', { what, found: this.describe(t) }));
	}

	// Portées

	private open() {
		this.scopes.push(newScope());
	}
	private close() {
		const s = this.scopes.pop()!;
		const parent = this.scopes.length - 1;
		for (const g of s.gotos) {
			if (parent < this.fn.scopeBase) this.fail(tr('script.lint.noLabel', { name: g.name }), g.tok);
			g.locals = Math.min(g.locals, this.locals());
			this.scopes[parent].gotos.push(g);
		}
	}
	private get top(): Scope {
		return this.scopes[this.scopes.length - 1];
	}
	private declare(name: string, at: number) {
		this.top.vars.set(name, at);
		this.top.count++;
		this.top.last = name;
	}
	private isLocal(name: string): boolean {
		for (let i = this.scopes.length - 1; i >= 0; i--) if (this.scopes[i].vars.has(name)) return true;
		return false;
	}
	/** Variables locales actives dans la fonction en cours. */
	private locals(): number {
		let n = 0;
		for (let i = this.fn.scopeBase; i < this.scopes.length; i++) n += this.scopes[i].count;
		return n;
	}
	private gotoStat(t: Token) {
		for (let i = this.scopes.length - 1; i >= this.fn.scopeBase; i--) if (this.scopes[i].labels.has(t.value)) return;
		this.top.gotos.push({ name: t.value, tok: t, locals: this.locals() });
	}
	private label(t: Token) {
		const line = this.lineOf(t.from);
		const old = this.top.labels.get(t.value);
		if (old) this.fail(tr('script.lint.labelDefined', { name: t.value, line: String(old.line) }), t);
		const locals = this.locals();
		this.top.labels.set(t.value, { line, locals });
		this.top.gotos = this.top.gotos.filter((g) => {
			if (g.name !== t.value) return true;
			if (g.locals < locals) this.fail(tr('script.lint.gotoScope', { line: String(this.lineOf(g.tok.from)), name: this.top.last }), t);
			return false;
		});
	}
	private lineOf(pos: number): number {
		return this.src.slice(0, pos).split('\n').length;
	}
	private warn(severity: Severity, from: number, to: number, message: string) {
		this.diags.push({ from, to, severity, message });
	}
	private later(run: (userGlobals: Set<string>) => void) {
		this.pending.push({ run });
	}

	// Programme

	run(): void {
		this.block();
		for (const g of this.scopes[0].gotos) this.fail(tr('script.lint.noLabel', { name: g.name }), g.tok);
		if (this.tok.type !== 'eof') {
			const t = this.tok;
			if (t.value === 'end') this.fail(tr('script.lint.extraEnd'), t);
			if (t.value === 'else' || t.value === 'elseif') this.fail(tr('script.lint.withoutIf', { word: t.value }), t);
			if (t.value === 'until') this.fail(tr('script.lint.untilWithoutRepeat'), t);
			this.fail(tr('script.lint.unexpectedStatement', { found: this.describe(t) }), t);
		}
		const ug = new Set(this.userGlobals.keys());
		for (const p of this.pending) p.run(ug);
		for (const [name, at] of this.userGlobals) {
			if (globals.has(name)) this.warn('warning', at.from, at.to, tr('script.lint.overridesGlobal', { name }));
			// Une fonction globale que le script n'appelle pas lui-même reste sans
			// remarque : c'est ce qu'appellent un bouton, un contrôle ou un
			// évènement du classeur (controls.on, events.on).
			else if (!this.userFuncs.includes(name))
				this.warn('info', at.from, at.to, tr('script.lint.globalVar', { name }));
			else if (this.calledGlobals.has(name)) this.warn('info', at.from, at.to, tr('script.lint.globalFunc', { name }));
		}
	}

	private block(): void {
		while (!this.atBlockEnd()) {
			if (this.is('return') || this.is('break')) {
				const t = this.next();
				if (t.value === 'return') {
					if (!this.atBlockEnd() && !this.is(';')) this.exprList();
				} else if (this.fn.loops === 0) {
					this.fail(tr('script.lint.breakOutside'), t);
				}
				this.accept(';');
				if (!this.atBlockEnd()) this.fail(tr('script.lint.lastStatement', { word: t.value }), this.tok);
				return;
			}
			this.statement();
		}
	}

	private atBlockEnd(): boolean {
		const t = this.tok;
		return t.type === 'eof' || (t.type === 'kw' && blockEnd.has(t.value));
	}

	private statement(): void {
		const t = this.tok;
		if (t.type === 'op' && t.value === ';') {
			this.next();
			return;
		}
		if (t.type === 'kw') {
			switch (t.value) {
				case 'if': {
					this.next();
					this.expr();
					this.expect('then', tr('script.lint.what.ifCond'));
					this.scoped(() => this.block());
					while (this.is('elseif')) {
						this.next();
						this.expr();
						this.expect('then', tr('script.lint.what.elseifCond'));
						this.scoped(() => this.block());
					}
					if (this.accept('else')) this.scoped(() => this.block());
					this.closeBlock('if', t);
					return;
				}
				case 'while': {
					this.next();
					this.expr();
					this.expect('do', tr('script.lint.what.whileCond'));
					this.loop(() => this.block());
					this.closeBlock('while', t);
					return;
				}
				case 'do':
					this.next();
					this.scoped(() => this.block());
					this.closeBlock('do', t);
					return;
				case 'for':
					this.forStat(t);
					return;
				case 'repeat': {
					this.next();
					this.open();
					this.fn.loops++;
					this.block();
					this.fn.loops--;
					this.expect('until', tr('script.lint.what.closeRepeat'));
					this.expr();
					this.close();
					return;
				}
				case 'function': {
					this.next();
					const first = this.name(tr('script.lint.what.funcName'));
					let method = false;
					let deep = false;
					while (this.is('.') || this.is(':')) {
						method = this.next().value === ':';
						this.name(tr('script.lint.what.funcName'));
						deep = true;
						if (method) break;
					}
					if (!deep) {
						if (!this.isLocal(first.value)) {
							if (!this.userGlobals.has(first.value)) this.userGlobals.set(first.value, { from: first.from, to: first.to });
							this.userFuncs.push(first.value);
						}
					} else this.readName(first);
					this.funcBody(method, t);
					return;
				}
				case 'local': {
					this.next();
					if (this.is('function')) {
						const kw = this.next();
						const nm = this.name(tr('script.lint.what.funcName'));
						this.declare(nm.value, nm.from);
						this.funcBody(false, kw);
						return;
					}
					const names: Token[] = [this.name(tr('script.lint.what.localName'))];
					while (this.accept(',')) names.push(this.name(tr('script.lint.what.localName')));
					if (this.accept('=')) this.exprList();
					for (const nm of names) this.declare(nm.value, nm.from);
					return;
				}
				case 'goto':
					this.next();
					this.gotoStat(this.name(tr('script.lint.what.gotoLabel')));
					return;
			}
		}
		if (t.type === 'op' && t.value === '::') {
			this.next();
			const nm = this.name(tr('script.lint.what.label'));
			this.expect('::', tr('script.lint.what.closeLabel'));
			this.label(nm);
			return;
		}
		this.exprStat();
	}

	private scoped(f: () => void) {
		this.open();
		f();
		this.close();
	}
	private loop(f: () => void) {
		this.fn.loops++;
		this.scoped(f);
		this.fn.loops--;
	}

	private forStat(kw: Token): void {
		this.next();
		const first = this.name(tr('script.lint.what.forVar'));
		if (this.accept('=')) {
			this.expr();
			this.expect(',', tr('script.lint.what.forBounds'));
			this.expr();
			if (this.accept(',')) this.expr();
			this.expect('do', tr('script.lint.what.forDo'));
			this.loop(() => {
				this.declare(first.value, first.from);
				this.block();
			});
		} else {
			const names = [first];
			while (this.accept(',')) names.push(this.name(tr('script.lint.what.forVar')));
			if (!this.is('in')) this.fail(tr('script.lint.forIn', { found: this.describe(this.tok) }));
			this.next();
			this.exprList();
			this.expect('do', tr('script.lint.what.forInDo'));
			this.loop(() => {
				for (const nm of names) this.declare(nm.value, nm.from);
				this.block();
			});
		}
		this.closeBlock('for', kw);
	}

	private funcBody(method: boolean, kw: Token): void {
		const outer = this.fn;
		this.fn = { vararg: false, loops: 0, parent: outer, scopeBase: this.scopes.length };
		this.open();
		if (method) this.declare('self', kw.from);
		this.expect('(', tr('script.lint.what.openParams'));
		if (!this.is(')')) {
			do {
				if (this.is('...')) {
					this.next();
					this.fn.vararg = true;
					break;
				}
				const p = this.name(tr('script.lint.what.param'));
				this.declare(p.value, p.from);
			} while (this.accept(','));
		}
		this.expect(')', tr('script.lint.what.closeParams'));
		this.block();
		this.closeBlock('function', kw);
		this.close();
		this.fn = outer;
	}

	private exprStat(): void {
		const start = this.tok;
		const e = this.suffixed();
		if (this.is('=') || this.is(',')) {
			const targets = [e];
			while (this.accept(',')) targets.push(this.suffixed());
			for (const tg of targets) {
				if (tg.kind === 'call') this.fail(tr('script.lint.assignCall'), this.toks[this.p - 1]);
				if (tg.kind !== 'name' && tg.kind !== 'index') this.fail(tr('script.lint.assignExpr'), start);
			}
			this.expect('=', tr('script.lint.what.assignment'));
			this.exprList();
			for (const tg of targets) {
				if (tg.kind === 'name') {
					const nm = this.src.slice(tg.from, tg.to);
					if (!this.isLocal(nm) && !this.userGlobals.has(nm)) this.userGlobals.set(nm, { from: tg.from, to: tg.to });
				}
				// Écrire string.x ou db.x n'est pas une lecture : pas de vérification du membre.
			}
			return;
		}
		if (e.kind === 'call') {
			for (const f of e.onRead) f();
			return;
		}
		const t = this.tok;
		if (t.type === 'op' && '+-*/%^.'.includes(t.value) && this.peek().value === '=' && this.peek().from === t.to)
			this.fail(tr('script.lint.compound', { op: t.value, bin: t.value === '.' ? '..' : t.value }), t);
		if (t.type === 'op' && t.value === '..' && this.peek().value === '=') this.fail(tr('script.lint.concatAssign'), t);
		if (e.kind === 'name' && this.src.slice(e.from, e.to) === 'continue')
			this.fail(tr('script.lint.noContinue'), start);
		if (t.type === 'op' && t.value === '==') this.fail(tr('script.lint.compareNotAssign'), t);
		this.fail(t.type !== 'eof' ? tr('script.lint.incompleteFound', { found: this.describe(t) }) : tr('script.lint.incomplete'), t.type === 'eof' ? start : t);
	}

	// Expressions

	private exprList(): Expr[] {
		const list = [this.expr()];
		while (this.accept(',')) list.push(this.expr());
		return list;
	}

	private expr(limit = 0): Expr {
		const t = this.tok;
		let left: Expr;
		if ((t.type === 'kw' && t.value === 'not') || (t.type === 'op' && (t.value === '-' || t.value === '#'))) {
			this.next();
			const inner = this.expr(unaryPriority);
			left = { kind: 'other', from: t.from, to: inner.to, onRead: [] };
		} else {
			left = this.simple();
		}
		for (;;) {
			const op = this.tok;
			if (op.type !== 'op' && op.type !== 'kw') break;
			const prio = binPriority[op.value];
			if (!prio || prio[0] <= limit) {
				if (op.type === 'op' && op.value === '/' && this.peek().value === '/' && this.peek().from === op.to)
					this.fail(tr('script.lint.floorDiv'), op);
				break;
			}
			this.next();
			if (op.value === '/' && this.is('/') && this.tok.from === op.to) this.fail(tr('script.lint.floorDiv'), op);
			const right = this.expr(prio[1]);
			left = { kind: 'other', from: left.from, to: right.to, onRead: [] };
		}
		return left;
	}

	private simple(): Expr {
		const t = this.tok;
		const lit = (): Expr => {
			this.next();
			return { kind: 'other', from: t.from, to: t.to, onRead: [] };
		};
		switch (t.type) {
			case 'number':
			case 'string':
				return lit();
			case 'kw':
				if (t.value === 'nil' || t.value === 'true' || t.value === 'false') return lit();
				if (t.value === 'function') {
					this.next();
					this.funcBody(false, t);
					return { kind: 'other', from: t.from, to: this.toks[this.p - 1].to, onRead: [] };
				}
				break;
			case 'op':
				if (t.value === '...') {
					if (!this.fn.vararg) this.fail(tr('script.lint.vararg'), t);
					this.next();
					return { kind: 'vararg', from: t.from, to: t.to, onRead: [] };
				}
				if (t.value === '{') return this.table();
				break;
		}
		const e = this.suffixed();
		for (const f of e.onRead) f();
		e.onRead = [];
		return e;
	}

	private table(): Expr {
		const open = this.next();
		while (!this.is('}')) {
			if (this.tok.type === 'eof') break;
			if (this.is('[')) {
				this.next();
				this.expr();
				this.expect(']', tr('script.lint.what.closeKey'));
				this.expect('=', tr('script.lint.what.afterKey'));
				this.expr();
			} else if (this.tok.type === 'name' && this.peek().value === '=' && this.peek().type === 'op') {
				this.next();
				this.next();
				this.expr();
			} else {
				this.expr();
			}
			// Comme gopher-lua : plusieurs séparateurs de suite sont acceptés ({1, , 2}).
			let sep = false;
			while (this.accept(',') || this.accept(';')) sep = true;
			if (!sep) break;
		}
		if (!this.is('}')) {
			const line = this.lineOf(open.from);
			this.fail(tr('script.lint.tableClose', { line: String(line), found: this.describe(this.tok) }));
		}
		const close = this.next();
		return { kind: 'other', from: open.from, to: close.to, onRead: [] };
	}

	/** Nom lu directement (pas affecté). */
	private readName(t: Token): void {
		const name = t.value;
		if (this.isLocal(name)) return;
		this.later((ug) => {
			if (ug.has(name) || globals.has(name)) return;
			if (unavailable[name]) {
				this.warn('warning', t.from, t.to, tr('script.lint.unavailableName', { name, why: unavailable[name] }));
				return;
			}
			const hint = suggest(name, [...globals, ...ug]);
			this.warn('warning', t.from, t.to, hint ? tr('script.lint.undefinedHint', { name, hint }) : tr('script.lint.undefined', { name }));
		});
	}

	private suffixed(): Expr {
		const t = this.tok;
		let e: Expr;
		if (t.type === 'name') {
			this.next();
			const local = this.isLocal(t.value);
			e = { kind: 'name', from: t.from, to: t.to, path: local ? undefined : t.value, onRead: [() => this.readName(t)] };
		} else if (t.type === 'op' && t.value === '(') {
			this.next();
			const inner = this.expr();
			for (const f of inner.onRead) f();
			const close = this.expect(')', tr('script.lint.what.closeParen'));
			// gopher-lua accepte (f()) comme instruction : l'appel entre parenthèses reste un appel.
			e = { kind: inner.kind === 'call' ? 'call' : 'other', from: t.from, to: close.to, onRead: [] };
		} else {
			this.fail(this.unexpected(t), t);
		}
		for (;;) {
			const s = this.tok;
			if (s.type === 'op' && s.value === '.') {
				this.next();
				const key = this.name(tr('script.lint.what.field'));
				// Indexer lit toujours la base ; le membre n'est vérifié que s'il est lu.
				for (const f of e.onRead) f();
				const base = e.path && !e.path.includes('.') ? e.path : undefined;
				e = {
					kind: 'index',
					from: e.from,
					to: key.to,
					path: base ? base + '.' + key.value : undefined,
					onRead: base && libraries[base] ? [() => this.checkMember(base, key)] : []
				};
			} else if (s.type === 'op' && s.value === '[') {
				for (const f of e.onRead) f();
				this.next();
				this.expr();
				const close = this.expect(']', tr('script.lint.what.closeIndex'));
				e = { kind: 'index', from: e.from, to: close.to, onRead: [] };
			} else if (s.type === 'op' && s.value === ':') {
				for (const f of e.onRead) f();
				this.next();
				const m = this.name(tr('script.lint.what.method'));
				const base = e.path;
				if (base && !base.includes('.') && libraries[base]) {
					this.later((ug) => {
						if (ug.has(base)) return;
						this.warn('warning', s.from, m.to, tr('script.lint.colonCall', { base, name: m.value }));
					});
				}
				const args = this.args(true);
				e = { kind: 'call', from: e.from, to: args.to, onRead: [] };
			} else if (s.type === 'string' || (s.type === 'op' && (s.value === '(' || s.value === '{'))) {
				// gopher-lua l'accepte (Lua 5.1 le refuse) : f()(g)() à l'exécution.
				if (s.value === '(' && s.nl && this.toks[this.p - 1].value === ')')
					this.warn('warning', s.from, s.to, tr('script.lint.parenLine'));
				const fnPath = e.path;
				const pending = e.onRead;
				const args = this.args(false);
				const callee = { from: e.from, to: e.to };
				e = { kind: 'call', from: e.from, to: args.to, onRead: [] };
				// Le nom appelé est lu, et ses arguments vérifiés, quel que soit l'usage du résultat.
				for (const f of pending) f();
				if (fnPath) this.checkArity(fnPath, callee, args);
				if (fnPath && !fnPath.includes('.')) this.calledGlobals.add(fnPath);
			} else break;
		}
		return e;
	}

	private unexpected(t: Token): string {
		if (t.type === 'eof') return tr('script.lint.exprEof');
		if (t.type === 'kw') {
			if (t.value === 'end') return tr('script.lint.exprEnd');
			return tr('script.lint.exprKeyword', { word: t.value });
		}
		if (t.value === '=') return tr('script.lint.exprEquals');
		return tr('script.lint.exprFound', { found: this.describe(t) });
	}

	private args(method: boolean): { count: number; multi: boolean; to: number } {
		const t = this.tok;
		if (t.type === 'string') {
			this.next();
			return { count: 1 + (method ? 1 : 0), multi: false, to: t.to };
		}
		if (t.value === '{') {
			const tb = this.table();
			return { count: 1 + (method ? 1 : 0), multi: false, to: tb.to };
		}
		if (t.value !== '(') this.fail(tr('script.lint.argsExpected', { found: this.describe(t) }), t);
		const open = this.next();
		let list: Expr[] = [];
		if (!this.is(')')) list = this.exprList();
		if (!this.is(')')) {
			const line = this.lineOf(open.from);
			this.fail(tr('script.lint.callClose', { line: String(line), found: this.describe(this.tok) }));
		}
		const close = this.next();
		const last = list[list.length - 1];
		return { count: list.length + (method ? 1 : 0), multi: !!last && (last.kind === 'call' || last.kind === 'vararg'), to: close.to };
	}

	private checkMember(lib: string, key: Token): void {
		const full = `${lib}.${key.value}`;
		this.later((ug) => {
			if (ug.has(lib)) return;
			const entry = byName.get(full);
			if (entry && libraries[lib].has(key.value)) {
				if (entry.deprecated) this.warn('info', key.from, key.to, tr('script.lint.deprecated', { name: full, use: entry.deprecated }));
				return;
			}
			if (unavailable[full]) {
				this.warn('warning', key.from, key.to, tr('script.lint.unavailableMember', { name: full, why: unavailable[full] }));
				return;
			}
			const hint = suggest(key.value, [...libraries[lib]]);
			this.warn('warning', key.from, key.to, hint ? tr('script.lint.noMemberHint', { name: full, hint: `${lib}.${hint}` }) : tr('script.lint.noMember', { name: full }));
		});
	}

	private checkArity(path: string, callee: { from: number; to: number }, a: { count: number; multi: boolean }): void {
		const entry: RefEntry | undefined = byName.get(path);
		if (!entry?.args || entry.category === 'syntaxe') return;
		const base = path.split('.')[0];
		if (path.includes('.') && !libraries[base]?.has(path.slice(base.length + 1))) return;
		this.later((ug) => {
			if (ug.has(base)) return;
			const [min, max] = entry.args!;
			const known = a.multi ? a.count - 1 : a.count;
			const tooFew = !a.multi && a.count < min;
			const tooMany = max !== null && known > max;
			if (!tooFew && !tooMany) return;
			const want = max === null ? tr('script.lint.arity.atLeast', { min }) : min === max ? tr('script.lint.arity.exactly', { n: min }) : tr('script.lint.arity.between', { min, max });
			const given = tr('script.lint.arity.given', { n: a.count });
			this.warn('warning', callee.from, callee.to, tr('script.lint.arity.message', { path, want, given, signature: entry.signature }));
		});
	}
}

/** Nom le plus proche (distance d'édition ≤ 2), ou undefined. */
function suggest(word: string, candidates: string[]): string | undefined {
	let best: string | undefined;
	let bestD = 3;
	const w = word.toLowerCase();
	for (const c of candidates) {
		if (c === word) continue;
		const d = distance(w, c.toLowerCase());
		if (d < bestD || (d === bestD && best && c.length < best.length)) {
			bestD = d;
			best = c;
		}
	}
	return bestD <= Math.min(2, Math.max(1, Math.floor(word.length / 3))) ? best : undefined;
}

function distance(a: string, b: string): number {
	const row = Array.from({ length: b.length + 1 }, (_, j) => j);
	for (let i = 1; i <= a.length; i++) {
		let prev = row[0];
		row[0] = i;
		for (let j = 1; j <= b.length; j++) {
			const cur = row[j];
			row[j] = Math.min(row[j] + 1, row[j - 1] + 1, prev + (a[i - 1] === b[j - 1] ? 0 : 1));
			prev = cur;
		}
	}
	return row[b.length];
}

/** Analyse un script ; une erreur de syntaxe arrête l'analyse et est seule renvoyée. */
export function lintLua(src: string): LintDiagnostic[] {
	try {
		const p = new Parser(src);
		p.run();
		return p.diags.sort((a, b) => a.from - b.from);
	} catch (e) {
		if (e instanceof LintError) {
			const from = Math.min(e.from, src.length);
			return [{ from, to: Math.min(Math.max(e.to, from), src.length), severity: 'error', message: e.message }];
		}
		throw e;
	}
}
