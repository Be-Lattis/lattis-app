// Linter des scripts Lua (onglet SCRIPT).
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
					if (lb.end < 0) throw new LintError('commentaire long non terminé : il manque « ]] »', start, start + 4);
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
				const hint = prev?.value === '.' ? `écrivez [${JSON.stringify(word)}] au lieu de .${word}` : 'utilisez un nom sans accent';
				throw new LintError(`un nom Lua ne peut pas contenir d’accent (« ${word} ») : ${hint}`, start, j);
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
				if (i === h) throw new LintError('nombre hexadécimal incomplet', start, i);
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
					if (i === e) throw new LintError('exposant incomplet dans le nombre', start, i);
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
				throw new LintError(`texte non terminé : il manque le ${c === '"' ? 'guillemet " fermant' : "guillemet ' fermant"} (pour plusieurs lignes, utilisez [[ ... ]])`, start, Math.min(i, n));
			i++;
			out.push({ type: 'string', value: src.slice(start, i), from: start, to: i, nl });
			nl = false;
			continue;
		}
		if (c === '[' && (src[i + 1] === '[' || src[i + 1] === '=')) {
			const lb = longBracket(i);
			if (lb) {
				if (lb.end < 0) throw new LintError('texte long non terminé : il manque « ]] »', start, start + 2);
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
	if (c === '!' && src[i + 1] === '=') return at('« différent » s’écrit ~= en Lua, pas !=', 2);
	if (c === '!') return at('« non » s’écrit not en Lua, pas !');
	if (c === '~') return at('« ~ » seul n’existe pas : « différent » s’écrit ~=');
	if (c === '&') return at('« et » s’écrit and en Lua, pas &&', src[i + 1] === '&' ? 2 : 1);
	if (c === '|') return at('« ou » s’écrit or en Lua, pas ||', src[i + 1] === '|' ? 2 : 1);
	if ('«»“”‘’'.includes(c)) return at(`guillemet typographique « ${c} » : utilisez " ou '`);
	if (c === '`') return at('« ` » n’existe pas en Lua : utilisez " ou \'');
	if (isLetter(c)) return at(`un nom Lua ne peut pas contenir d’accent (« ${c} ») : utilisez un nom sans accent`);
	return at(`caractère inattendu « ${c} »`);
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
		if (t.type === 'eof') return 'la fin du script';
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
		let msg = `« ${v} » attendu${what ? ' ' + what : ''}, trouvé ${this.describe(this.tok)}`;
		if ((v === 'then' || v === 'do') && this.is('=')) msg += ' (pour comparer, utilisez ==)';
		this.fail(msg);
	}
	private closeBlock(kw: string, open: Token): void {
		if (this.is('end')) {
			this.next();
			return;
		}
		const line = this.lineOf(open.from);
		let msg = `« end » attendu pour fermer ${opener[kw] ?? `« ${kw} »`} de la ligne ${line}, trouvé ${this.describe(this.tok)}`;
		if (this.tok.type === 'name' && this.tok.value === 'continue') msg = 'Lua n’a pas de « continue » : mettez la suite de la boucle dans un if';
		this.fail(msg);
	}
	private name(what: string): Token {
		const t = this.tok;
		if (t.type === 'name') return this.next();
		if (t.type === 'kw') this.fail(`« ${t.value} » est un mot réservé de Lua : il ne peut pas servir de nom (${what})`);
		this.fail(`nom attendu (${what}), trouvé ${this.describe(t)}`);
	}

	// Portées

	private open() {
		this.scopes.push(newScope());
	}
	private close() {
		const s = this.scopes.pop()!;
		const parent = this.scopes.length - 1;
		for (const g of s.gotos) {
			if (parent < this.fn.scopeBase) this.fail(`aucune étiquette « ::${g.name}:: » visible pour ce goto`, g.tok);
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
		if (old) this.fail(`étiquette « ::${t.value}:: » déjà définie ligne ${old.line}`, t);
		const locals = this.locals();
		this.top.labels.set(t.value, { line, locals });
		this.top.gotos = this.top.gotos.filter((g) => {
			if (g.name !== t.value) return true;
			if (g.locals < locals) this.fail(`le goto de la ligne ${this.lineOf(g.tok.from)} saute dans la portée de la variable locale « ${this.top.last} » : déclarez-la avant le goto`, t);
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
		for (const g of this.scopes[0].gotos) this.fail(`aucune étiquette « ::${g.name}:: » visible pour ce goto`, g.tok);
		if (this.tok.type !== 'eof') {
			const t = this.tok;
			if (t.value === 'end') this.fail('« end » en trop : aucun bloc à fermer', t);
			if (t.value === 'else' || t.value === 'elseif') this.fail(`« ${t.value} » sans « if »`, t);
			if (t.value === 'until') this.fail('« until » sans « repeat »', t);
			this.fail(`instruction inattendue : ${this.describe(t)}`, t);
		}
		const ug = new Set(this.userGlobals.keys());
		for (const p of this.pending) p.run(ug);
		for (const [name, at] of this.userGlobals) {
			if (globals.has(name)) this.warn('warning', at.from, at.to, `cette affectation remplace « ${name} », prédéfini`);
			else
				this.warn(
					'info',
					at.from,
					at.to,
					this.userFuncs.includes(name)
						? `fonction globale « ${name} » : préférez « local function ${name} »`
						: `variable globale « ${name} » : préférez « local ${name} = ... »`
				);
		}
	}

	private block(): void {
		while (!this.atBlockEnd()) {
			if (this.is('return') || this.is('break')) {
				const t = this.next();
				if (t.value === 'return') {
					if (!this.atBlockEnd() && !this.is(';')) this.exprList();
				} else if (this.fn.loops === 0) {
					this.fail('« break » en dehors d’une boucle', t);
				}
				this.accept(';');
				if (!this.atBlockEnd()) this.fail(`« ${t.value} » doit être la dernière instruction de son bloc (avant « end », « else » ou « until »)`, this.tok);
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
					this.expect('then', 'après la condition du if');
					this.scoped(() => this.block());
					while (this.is('elseif')) {
						this.next();
						this.expr();
						this.expect('then', 'après la condition du elseif');
						this.scoped(() => this.block());
					}
					if (this.accept('else')) this.scoped(() => this.block());
					this.closeBlock('if', t);
					return;
				}
				case 'while': {
					this.next();
					this.expr();
					this.expect('do', 'après la condition du while');
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
					this.expect('until', 'pour fermer le repeat');
					this.expr();
					this.close();
					return;
				}
				case 'function': {
					this.next();
					const first = this.name('nom de la fonction');
					let method = false;
					let deep = false;
					while (this.is('.') || this.is(':')) {
						method = this.next().value === ':';
						this.name('nom de la fonction');
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
						const nm = this.name('nom de la fonction');
						this.declare(nm.value, nm.from);
						this.funcBody(false, kw);
						return;
					}
					const names: Token[] = [this.name('nom de variable locale')];
					while (this.accept(',')) names.push(this.name('nom de variable locale'));
					if (this.accept('=')) this.exprList();
					for (const nm of names) this.declare(nm.value, nm.from);
					return;
				}
				case 'goto':
					this.next();
					this.gotoStat(this.name('étiquette du goto'));
					return;
			}
		}
		if (t.type === 'op' && t.value === '::') {
			this.next();
			const nm = this.name('étiquette');
			this.expect('::', 'pour fermer l’étiquette');
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
		const first = this.name('variable de la boucle for');
		if (this.accept('=')) {
			this.expr();
			this.expect(',', 'entre le début et la fin de la boucle for');
			this.expr();
			if (this.accept(',')) this.expr();
			this.expect('do', 'après les bornes de la boucle for');
			this.loop(() => {
				this.declare(first.value, first.from);
				this.block();
			});
		} else {
			const names = [first];
			while (this.accept(',')) names.push(this.name('variable de la boucle for'));
			if (!this.is('in')) this.fail(`« = » ou « in » attendu après la variable de la boucle for, trouvé ${this.describe(this.tok)}`);
			this.next();
			this.exprList();
			this.expect('do', 'après « for ... in ... »');
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
		this.expect('(', 'pour ouvrir les paramètres de la fonction');
		if (!this.is(')')) {
			do {
				if (this.is('...')) {
					this.next();
					this.fn.vararg = true;
					break;
				}
				const p = this.name('paramètre de la fonction');
				this.declare(p.value, p.from);
			} while (this.accept(','));
		}
		this.expect(')', 'pour fermer les paramètres de la fonction');
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
				if (tg.kind === 'call') this.fail('on ne peut pas affecter une valeur au résultat d’un appel de fonction', this.toks[this.p - 1]);
				if (tg.kind !== 'name' && tg.kind !== 'index') this.fail('cette expression ne peut pas recevoir de valeur', start);
			}
			this.expect('=', 'dans l’affectation');
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
			this.fail(`Lua n’a pas d’opérateur « ${t.value}= » : écrivez x = x ${t.value === '.' ? '..' : t.value} valeur`, t);
		if (t.type === 'op' && t.value === '..' && this.peek().value === '=') this.fail('Lua n’a pas d’opérateur « ..= » : écrivez s = s .. valeur', t);
		if (e.kind === 'name' && this.src.slice(e.from, e.to) === 'continue')
			this.fail('Lua n’a pas de « continue » : mettez la suite de la boucle dans un if', start);
		if (t.type === 'op' && t.value === '==') this.fail('« == » compare deux valeurs ; pour affecter, utilisez un seul =', t);
		this.fail(`instruction incomplète : un appel de fonction ou une affectation (=) est attendu${t.type !== 'eof' ? `, trouvé ${this.describe(t)}` : ''}`, t.type === 'eof' ? start : t);
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
					this.fail('Lua 5.1 n’a pas d’opérateur « // » : utilisez math.floor(a / b)', op);
				break;
			}
			this.next();
			if (op.value === '/' && this.is('/') && this.tok.from === op.to) this.fail('Lua 5.1 n’a pas d’opérateur « // » : utilisez math.floor(a / b)', op);
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
					if (!this.fn.vararg) this.fail('« ... » n’est utilisable que dans une fonction déclarée avec « ... » dans ses paramètres', t);
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
				this.expect(']', 'pour fermer la clé');
				this.expect('=', 'après la clé [ ... ]');
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
			this.fail(`« } » attendu pour fermer la table ouverte ligne ${line} (séparez les éléments par des virgules), trouvé ${this.describe(this.tok)}`);
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
				this.warn('warning', t.from, t.to, `« ${name} » n’est pas disponible : ${unavailable[name]}`);
				return;
			}
			const hint = suggest(name, [...globals, ...ug]);
			this.warn('warning', t.from, t.to, `« ${name} » n’est pas défini${hint ? ` : voulez-vous dire « ${hint} » ?` : ' (variable jamais affectée ni déclarée avec local)'}`);
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
			const close = this.expect(')', 'pour fermer la parenthèse');
			// gopher-lua accepte (f()) comme instruction : l'appel entre parenthèses reste un appel.
			e = { kind: inner.kind === 'call' ? 'call' : 'other', from: t.from, to: close.to, onRead: [] };
		} else {
			this.fail(this.unexpected(t), t);
		}
		for (;;) {
			const s = this.tok;
			if (s.type === 'op' && s.value === '.') {
				this.next();
				const key = this.name('nom de champ après le point');
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
				const close = this.expect(']', 'pour fermer l’index');
				e = { kind: 'index', from: e.from, to: close.to, onRead: [] };
			} else if (s.type === 'op' && s.value === ':') {
				for (const f of e.onRead) f();
				this.next();
				const m = this.name('nom de méthode après « : »');
				const base = e.path;
				if (base && !base.includes('.') && libraries[base]) {
					this.later((ug) => {
						if (ug.has(base)) return;
						this.warn('warning', s.from, m.to, `écrivez ${base}.${m.value}(...) avec un point : « : » passerait ${base} comme premier argument`);
					});
				}
				const args = this.args(true);
				e = { kind: 'call', from: e.from, to: args.to, onRead: [] };
			} else if (s.type === 'string' || (s.type === 'op' && (s.value === '(' || s.value === '{'))) {
				// gopher-lua l'accepte (Lua 5.1 le refuse) : f()(g)() à l'exécution.
				if (s.value === '(' && s.nl && this.toks[this.p - 1].value === ')')
					this.warn('warning', s.from, s.to, 'ce « ( » en début de ligne prolonge l’appel de la ligne précédente ; ajoutez « ; » à la fin de la ligne précédente si ce n’est pas voulu');
				const fnPath = e.path;
				const pending = e.onRead;
				const args = this.args(false);
				const callee = { from: e.from, to: e.to };
				e = { kind: 'call', from: e.from, to: args.to, onRead: [] };
				// Le nom appelé est lu, et ses arguments vérifiés, quel que soit l'usage du résultat.
				for (const f of pending) f();
				if (fnPath) this.checkArity(fnPath, callee, args);
			} else break;
		}
		return e;
	}

	private unexpected(t: Token): string {
		if (t.type === 'eof') return 'expression attendue, mais le script s’arrête là';
		if (t.type === 'kw') {
			if (t.value === 'end') return '« end » inattendu : une expression manque avant, ou il y a un « end » en trop';
			return `expression attendue, trouvé le mot réservé « ${t.value} »`;
		}
		if (t.value === '=') return '« = » inattendu : une valeur manque avant, ou pour comparer utilisez ==';
		return `expression attendue, trouvé ${this.describe(t)}`;
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
		if (t.value !== '(') this.fail(`arguments attendus après « : nom », trouvé ${this.describe(t)}`, t);
		const open = this.next();
		let list: Expr[] = [];
		if (!this.is(')')) list = this.exprList();
		if (!this.is(')')) {
			const line = this.lineOf(open.from);
			this.fail(`« ) » attendu pour fermer l’appel ouvert ligne ${line} (séparez les arguments par des virgules), trouvé ${this.describe(this.tok)}`);
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
				if (entry.deprecated) this.warn('info', key.from, key.to, `${full} est un ancien nom : utilisez ${entry.deprecated}`);
				return;
			}
			if (unavailable[full]) {
				this.warn('warning', key.from, key.to, `${full} n’est pas disponible : ${unavailable[full]}`);
				return;
			}
			const hint = suggest(key.value, [...libraries[lib]]);
			this.warn('warning', key.from, key.to, `${full} n’existe pas${hint ? ` : voulez-vous dire ${lib}.${hint} ?` : ''}`);
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
			const want = max === null ? `au moins ${min}` : min === max ? `${min}` : `de ${min} à ${max}`;
			this.warn('warning', callee.from, callee.to, `${path} attend ${want} argument${max === 1 && min === max ? '' : 's'}, ${a.count} donné${a.count > 1 ? 's' : ''} : ${entry.signature}`);
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
