// Codes de format de nombre (Format de cellule › Personnalisé), comme un
// tableur : « # ##0,00 "kg" », « 0,00E+00 », « # ?/? », « [Rouge]-# ##0 »,
// sections positif;négatif;zéro;texte, conditions [>100], dates
// « jj/mm/aaaa hh:mm », durées [h]:mm, texte entre guillemets, \x, _x
// (espace) et *x (remplissage : alignement de la comptabilité).
//
// Même moteur que internal/numfmt (Go, pour TEXTE() et les exports) : les
// deux suivent les cas de internal/numfmt/testdata/cases.json. Un code
// s'écrit avec les séparateurs français ou anglais ; le résultat suit la
// langue de la session.

import { lang, numberFormat } from '#lib/i18n/index.js';

/** Place du remplissage (*x) dans le texte, suivie du caractère qui remplit. */
export const FILL = '\x01';

export interface NumResult {
	text: string;
	/** Couleur du code ([Rouge] : '#FF0000'). */
	color?: string;
	/** Faux : valeur hors des dates permises (« ##### »). */
	ok: boolean;
}

export interface NumLocale {
	decimal: string;
	group: string;
	months: string[];
	monthsShort: string[];
	days: string[]; // dimanche d'abord
	daysShort: string[];
}

export const FR: NumLocale = {
	decimal: ',',
	group: ' ',
	months: ['janvier', 'février', 'mars', 'avril', 'mai', 'juin', 'juillet', 'août', 'septembre', 'octobre', 'novembre', 'décembre'],
	monthsShort: ['janv.', 'févr.', 'mars', 'avr.', 'mai', 'juin', 'juil.', 'août', 'sept.', 'oct.', 'nov.', 'déc.'],
	days: ['dimanche', 'lundi', 'mardi', 'mercredi', 'jeudi', 'vendredi', 'samedi'],
	daysShort: ['dim.', 'lun.', 'mar.', 'mer.', 'jeu.', 'ven.', 'sam.']
};
export const EN: NumLocale = {
	decimal: '.',
	group: ',',
	months: ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'],
	monthsShort: ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'],
	days: ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'],
	daysShort: ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
};
const DE: NumLocale = {
	decimal: ',',
	group: '.',
	months: ['Januar', 'Februar', 'März', 'April', 'Mai', 'Juni', 'Juli', 'August', 'September', 'Oktober', 'November', 'Dezember'],
	monthsShort: ['Jan.', 'Feb.', 'März', 'Apr.', 'Mai', 'Juni', 'Juli', 'Aug.', 'Sept.', 'Okt.', 'Nov.', 'Dez.'],
	days: ['Sonntag', 'Montag', 'Dienstag', 'Mittwoch', 'Donnerstag', 'Freitag', 'Samstag'],
	daysShort: ['So.', 'Mo.', 'Di.', 'Mi.', 'Do.', 'Fr.', 'Sa.']
};
const ES: NumLocale = {
	decimal: ',',
	group: '.',
	months: ['enero', 'febrero', 'marzo', 'abril', 'mayo', 'junio', 'julio', 'agosto', 'septiembre', 'octubre', 'noviembre', 'diciembre'],
	monthsShort: ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sept', 'oct', 'nov', 'dic'],
	days: ['domingo', 'lunes', 'martes', 'miércoles', 'jueves', 'viernes', 'sábado'],
	daysShort: ['dom', 'lun', 'mar', 'mié', 'jue', 'vie', 'sáb']
};
const ZH: NumLocale = {
	decimal: '.',
	group: ',',
	months: ['一月', '二月', '三月', '四月', '五月', '六月', '七月', '八月', '九月', '十月', '十一月', '十二月'],
	monthsShort: ['1月', '2月', '3月', '4月', '5月', '6月', '7月', '8月', '9月', '10月', '11月', '12月'],
	days: ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六'],
	daysShort: ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
};

/** Locale d'une langue de l'interface (français par défaut). */
export function localeFor(l: string): NumLocale {
	return ({ en: EN, de: DE, es: ES, zh: ZH } as Record<string, NumLocale>)[l] ?? FR;
}

let sessionCache: { lang: string; loc: NumLocale } | null = null;

/** Locale de la session : noms de localeFor, séparateurs du reste de l'affichage (Intl). */
export function sessionLocale(): NumLocale {
	const l = lang();
	if (sessionCache?.lang === l) return sessionCache.loc;
	const parts = numberFormat({ minimumFractionDigits: 1, maximumFractionDigits: 1 }).formatToParts(12345.5);
	const base = localeFor(l);
	const loc = { ...base, decimal: parts.find((p) => p.type === 'decimal')?.value ?? base.decimal, group: parts.find((p) => p.type === 'group')?.value ?? base.group };
	sessionCache = { lang: l, loc };
	return loc;
}

/** Texte sans la marque de remplissage (le caractère reste une fois). */
export function plain(s: string): string {
	return s.split(FILL).join('');
}

// --- Lecture du code ------------------------------------------------------------

type Kind = 'lit' | 'digit' | 'dec' | 'exp' | 'slash' | 'den' | 'text' | 'fill' | 'date' | 'ampm' | 'elapsed' | 'general';

interface Tok {
	k: Kind;
	s?: string;
	r?: string;
	n?: number;
}

interface Section {
	toks: Tok[];
	color?: string;
	cond?: { op: string; v: number };
	date: boolean;
	text: boolean;
	group: boolean;
	scale: number;
	percent: number;
	digits: boolean;
	general: boolean;
}

const COLORS: Record<string, string> = {
	black: '#000000', noir: '#000000', schwarz: '#000000', negro: '#000000',
	blue: '#0000FF', bleu: '#0000FF', blau: '#0000FF', azul: '#0000FF',
	cyan: '#00FFFF', zyan: '#00FFFF', cian: '#00FFFF',
	green: '#00FF00', vert: '#00FF00', grün: '#00FF00', verde: '#00FF00',
	magenta: '#FF00FF',
	red: '#FF0000', rouge: '#FF0000', rot: '#FF0000', rojo: '#FF0000',
	white: '#FFFFFF', blanc: '#FFFFFF', weiß: '#FFFFFF', weiss: '#FFFFFF', blanco: '#FFFFFF',
	yellow: '#FFFF00', jaune: '#FFFF00', gelb: '#FFFF00', amarillo: '#FFFF00'
};
const PALETTE = ['#000000', '#FFFFFF', '#FF0000', '#00FF00', '#0000FF', '#FFFF00', '#FF00FF', '#00FFFF', '#800000', '#008000', '#000080', '#808000', '#800080', '#008080', '#C0C0C0', '#808080'];
const GENERAL_WORDS = ['g/standard', 'standard', 'general', 'estándar', '常规'];
const GEN = '\x02';
const NO_DEC = '\x03';

const isDigitPh = (r: string | undefined) => r === '0' || r === '#' || r === '?';
const isSpace = (r: string | undefined) => r === ' ' || r === ' ' || r === ' ';
const FRAC_RE = /[0#?]\s*\/\s*[0-9#?]/;

function dateLetter(r: string | undefined): string {
	switch (r?.toLowerCase()) {
		case 'j':
		case 'd':
			return 'j';
		case 'm':
			return 'm';
		case 'a':
		case 'y':
			return 'a';
		case 'h':
			return 'h';
		case 's':
			return 's';
	}
	return '';
}

/** Sections d'un code (« ; » hors guillemets, \x et crochets). */
function splitSections(code: string): string[] {
	const rs = Array.from(code);
	const out: string[] = [];
	let start = 0;
	for (let i = 0; i < rs.length; i++) {
		switch (rs[i]) {
			case '"':
				for (i++; i < rs.length && rs[i] !== '"'; i++);
				break;
			case '\\':
			case '_':
			case '*':
				i++;
				break;
			case '[':
				for (i++; i < rs.length && rs[i] !== ']'; i++);
				break;
			case ';':
				out.push(rs.slice(start, i).join(''));
				start = i + 1;
		}
	}
	out.push(rs.slice(start).join(''));
	return out;
}

/** Code sans texte entre guillemets, \x, _x, *x ni crochets. */
function skeleton(rs: string[]): string[] {
	const out: string[] = [];
	for (let i = 0; i < rs.length; i++) {
		switch (rs[i]) {
			case '"':
				for (i++; i < rs.length && rs[i] !== '"'; i++);
				break;
			case '\\':
			case '_':
			case '*':
				i++;
				break;
			case '[':
				for (i++; i < rs.length && rs[i] !== ']'; i++);
				break;
			default:
				out.push(rs[i]);
		}
	}
	return out;
}

/** Remplace le mot Standard (General…) hors guillemets par GEN. */
function markGeneral(rs: string[]): [string[], boolean] {
	const low = rs.map((r) => r.toLowerCase());
	for (let i = 0; i < rs.length; i++) {
		const r = rs[i];
		if (r === '"') {
			for (i++; i < rs.length && rs[i] !== '"'; i++);
			continue;
		}
		if (r === '\\' || r === '_' || r === '*') {
			i++;
			continue;
		}
		if (r === '[') {
			for (i++; i < rs.length && rs[i] !== ']'; i++);
			continue;
		}
		for (const w of GENERAL_WORDS) {
			const wr = Array.from(w);
			if (i + wr.length <= low.length && low.slice(i, i + wr.length).join('') === w) return [[...rs.slice(0, i), GEN, ...rs.slice(i + wr.length)], true];
		}
	}
	return [rs, false];
}

function parseSection(code: string): Section {
	const sec: Section = { toks: [], date: false, text: false, group: false, scale: 0, percent: 0, digits: false, general: false };
	const [rs, gen] = markGeneral(Array.from(code));
	sec.general = gen;
	const sk = skeleton(rs);
	for (const r of sk) {
		if (isDigitPh(r)) {
			sec.digits = true;
			break;
		}
		if (dateLetter(r)) {
			sec.date = true;
			break;
		}
	}
	if (!sec.date) for (const r of sk) if (isDigitPh(r)) sec.digits = true;
	const skStr = sk.join('');
	const en = !sec.date && (skStr.includes('.') || skStr.includes('#,#'));
	const dec = en ? '.' : ',';
	let seenDec = false;
	let seenExp = false;
	const frac = !sec.date && FRAC_RE.test(skStr);
	const nextPh = (i: number) => isDigitPh(rs[i + 1]);
	const prevPh = () => sec.toks[sec.toks.length - 1]?.k === 'digit';
	const lit = (s: string) => sec.toks.push({ k: 'lit', s });
	for (let i = 0; i < rs.length; i++) {
		const r = rs[i];
		if (r === GEN) sec.toks.push({ k: 'general' });
		else if (r === '"') {
			let j = i + 1;
			while (j < rs.length && rs[j] !== '"') j++;
			lit(rs.slice(i + 1, Math.min(j, rs.length)).join(''));
			i = j;
		} else if (r === '\\') {
			if (i + 1 < rs.length) lit(rs[++i]);
		} else if (r === '_') {
			if (i + 1 < rs.length) i++;
			lit(' ');
		} else if (r === '*') {
			if (i + 1 < rs.length) sec.toks.push({ k: 'fill', s: rs[++i] });
		} else if (r === '[') {
			let j = i + 1;
			while (j < rs.length && rs[j] !== ']') j++;
			bracket(sec, rs.slice(i + 1, Math.min(j, rs.length)).join(''));
			i = j;
		} else if (sec.date) {
			const c = dateLetter(r);
			if (c) {
				const up = rs.slice(i, i + 5).join('').toUpperCase();
				if (up.startsWith('AM/PM')) {
					sec.toks.push({ k: 'ampm', s: 'AM/PM' });
					i += 4;
					continue;
				}
				if (up.startsWith('A/P')) {
					sec.toks.push({ k: 'ampm', s: 'A/P' });
					i += 2;
					continue;
				}
				let n = 1;
				while (i + 1 < rs.length && dateLetter(rs[i + 1]) === c) i++, n++;
				sec.toks.push({ k: 'date', r: c, n });
				continue;
			}
			if ((r === '.' || r === ',') && rs[i + 1] === '0') {
				// Fractions de seconde : ss,00 ou ss.00.
				let j = i + 1;
				while (j < rs.length && rs[j] === '0') j++;
				sec.toks.push({ k: 'dec', n: j - i - 1 });
				i = j - 1;
				continue;
			}
			if (r === '@') {
				sec.text = true;
				sec.toks.push({ k: 'text' });
				continue;
			}
			lit(r);
		} else if (isDigitPh(r)) sec.toks.push({ k: 'digit', r });
		else if (r === dec && !seenDec && !seenExp) {
			seenDec = true;
			sec.toks.push({ k: 'dec' });
		} else if ((en && r === ',') || (!en && isSpace(r))) {
			if (prevPh() && nextPh(i) && !seenDec && !frac) sec.group = true;
			else if (en && prevPh() && (i + 1 === rs.length || rs[i + 1] === ',' || rs[i + 1] === '.' || !isDigitPh(rs[i + 1]))) sec.scale++;
			else lit(r);
		} else if (r === '%') {
			sec.percent++;
			lit('%');
		} else if ((r === 'E' || r === 'e') && (rs[i + 1] === '+' || rs[i + 1] === '-') && sec.toks.some((t) => t.k === 'digit')) {
			sec.toks.push({ k: 'exp', s: r + rs[i + 1] });
			seenExp = true;
			i++;
		} else if (r === '/' && prevPh() && i + 1 < rs.length && (isDigitPh(rs[i + 1]) || (rs[i + 1] >= '1' && rs[i + 1] <= '9'))) {
			sec.toks.push({ k: 'slash' });
			if (rs[i + 1] >= '1' && rs[i + 1] <= '9') {
				let j = i + 1;
				while (j < rs.length && rs[j] >= '0' && rs[j] <= '9') j++;
				sec.toks.push({ k: 'den', s: rs.slice(i + 1, j).join('') });
				i = j - 1;
			}
		} else if (r === '@') {
			sec.text = true;
			sec.toks.push({ k: 'text' });
		} else lit(r);
	}
	return sec;
}

/** Un [..] : couleur, condition, symbole monétaire, durée. */
function bracket(s: Section, inner: string) {
	const low = inner.trim().toLowerCase();
	if (COLORS[low]) {
		s.color = COLORS[low];
		return;
	}
	for (const p of ['color', 'couleur', 'farbe']) {
		if (low.startsWith(p) && /^\s*\d+\s*$/.test(low.slice(p.length))) {
			const n = Number(low.slice(p.length));
			if (n >= 1 && n <= PALETTE.length) s.color = PALETTE[n - 1];
			return;
		}
	}
	if (inner.startsWith('$')) {
		const sym = inner.slice(1).split('-')[0];
		if (sym) s.toks.push({ k: 'lit', s: sym });
		return;
	}
	for (const op of ['<=', '>=', '<>', '<', '>', '=']) {
		if (low.startsWith(op)) {
			const v = Number(low.slice(op.length).trim().replace(',', '.'));
			if (low.slice(op.length).trim() !== '' && Number.isFinite(v)) s.cond = { op, v };
			return;
		}
	}
	if (low) {
		const c = dateLetter(low[0]);
		if ((c === 'h' || c === 'm' || c === 's') && low.split(low[0]).join('') === '') {
			s.date = true;
			s.toks.push({ k: 'elapsed', r: c, n: low.length });
		}
	}
}

function matches(c: { op: string; v: number }, x: number): boolean {
	switch (c.op) {
		case '<':
			return x < c.v;
		case '<=':
			return x <= c.v;
		case '>':
			return x > c.v;
		case '>=':
			return x >= c.v;
		case '=':
			return x === c.v;
		case '<>':
			return x !== c.v;
	}
	return false;
}

/** Code de format lu (gardé en cache). */
export class NumFormat {
	readonly secs: Section[];
	constructor(code: string) {
		this.secs = splitSections(code).map(parseSection);
	}

	/** Le code met-il en forme des dates ou des heures ? */
	isDate(): boolean {
		return !!this.secs[0]?.date;
	}

	/** Format texte (@ seul) ? */
	isText(): boolean {
		const s = this.secs[0];
		return this.secs.length === 1 && s.text && !s.digits && !s.date;
	}

	/** Un nombre mis en forme. */
	number(x: number, loc: NumLocale = sessionLocale()): NumResult {
		const [sec, neg] = this.pick(x);
		if (!sec) return { text: general(x, loc), ok: true };
		if (sec.text && !sec.digits && !sec.date && !sec.general) return withColor({ text: general(x, loc), ok: true }, sec.color);
		const v = neg ? -x : x;
		if (sec.date) {
			const d = formatDate(sec, x, loc);
			return withColor({ text: d ?? '', ok: d !== null }, sec.color);
		}
		let out = formatNumber(sec, Math.abs(v), loc);
		if (v < 0 && /[1-9]/.test(plain(out))) out = '-' + out;
		return withColor({ text: out, ok: true }, sec.color);
	}

	/** Un texte mis en forme : section du texte (quatrième, ou @), sinon tel quel. */
	text(s: string): NumResult {
		const last = this.secs[this.secs.length - 1];
		const sec = this.secs.length >= 4 ? this.secs[3] : last?.text ? last : undefined;
		if (!sec) return { text: s, ok: true };
		let b = '';
		for (const t of sec.toks) {
			if (t.k === 'text') b += s;
			else if (t.k === 'fill') b += FILL + t.s;
			else if (t.k === 'lit') b += t.s;
		}
		return withColor({ text: b, ok: true }, sec.color);
	}

	private pick(x: number): [Section | undefined, boolean] {
		const secs = this.secs.length > 1 ? this.secs.slice(0, 3) : this.secs;
		if (!secs.length) return [undefined, false];
		if (secs.some((s) => s.cond)) {
			const hit = secs.find((s) => s.cond && matches(s.cond, x)) ?? secs.find((s) => !s.cond) ?? secs[secs.length - 1];
			return [hit, false];
		}
		if (x < 0 && secs.length >= 2) return [secs[1], true];
		if (x === 0 && secs.length >= 3) return [secs[2], false];
		return [secs[0], false];
	}
}

function withColor(r: NumResult, color: string | undefined): NumResult {
	if (color) r.color = color;
	return r;
}

const cache = new Map<string, NumFormat>();

/** Code lu, gardé en cache. */
export function parseCode(code: string): NumFormat {
	let f = cache.get(code);
	if (!f) {
		if (cache.size > 500) cache.clear();
		f = new NumFormat(code);
		cache.set(code, f);
	}
	return f;
}

/** Écriture décimale d'un nombre (sans exposant). */
function plainDecimal(s: string): string {
	const m = /^(-?)(\d+)(?:\.(\d+))?e([+-]\d+)$/i.exec(s);
	if (!m) return s;
	const digits = m[2] + (m[3] ?? '');
	const point = m[2].length + Number(m[4]);
	if (point <= 0) return m[1] + '0.' + '0'.repeat(-point) + digits;
	if (point >= digits.length) return m[1] + digits + '0'.repeat(point - digits.length);
	return m[1] + digits.slice(0, point) + '.' + digits.slice(point);
}

/** Affichage standard (10 chiffres significatifs au plus). */
function general(x: number, loc: NumLocale): string {
	const a = Math.abs(x);
	let s: string;
	if (a !== 0 && (a >= 1e11 || a < 1e-9)) {
		const [m0, e] = x.toExponential(5).split('e');
		const m = m0.includes('.') ? m0.replace(/0+$/, '').replace(/\.$/, '') : m0;
		const sign = e[0] === '-' ? '-' : '+';
		s = m + 'E' + sign + e.slice(1).replace(/^0+/, '').padStart(2, '0');
	} else {
		s = plainDecimal(String(Number(x.toPrecision(10))));
		if (s.includes('.')) s = s.replace(/0+$/, '').replace(/\.$/, '');
	}
	return s.replace('.', loc.decimal);
}

/** x ≥ 0 écrit avec d décimales, arrondi au plus loin de zéro sur 15 chiffres significatifs. */
function fixed(x: number, d: number): string {
	const s = plainDecimal(String(Number(x.toPrecision(15))));
	const [ip, fp = ''] = s.split('.');
	if (fp.length <= d) return joinFixed(ip, fp + '0'.repeat(d - fp.length));
	const digits = (ip + fp.slice(0, d)).split('');
	if (fp[d] >= '5') {
		let i = digits.length - 1;
		for (; i >= 0; i--) {
			if (digits[i] === '9') {
				digits[i] = '0';
				continue;
			}
			digits[i] = String(Number(digits[i]) + 1);
			break;
		}
		if (i < 0) digits.unshift('1');
	}
	const n = digits.length - d;
	return joinFixed(digits.slice(0, n).join(''), digits.slice(n).join(''));
}

function joinFixed(ip: string, fp: string): string {
	return (ip || '0') + (fp ? '.' + fp : '');
}

function formatNumber(s: Section, x: number, loc: NumLocale): string {
	x *= 100 ** s.percent;
	x /= 1000 ** s.scale;
	const intPh: number[] = [];
	const fracPh: number[] = [];
	const expPh: number[] = [];
	let decAt = -1;
	let expAt = -1;
	let slashAt = -1;
	s.toks.forEach((t, i) => {
		if (t.k === 'digit') (expAt >= 0 ? expPh : decAt >= 0 ? fracPh : intPh).push(i);
		else if (t.k === 'dec' && decAt < 0 && expAt < 0) decAt = i;
		else if (t.k === 'exp') expAt = i;
		else if (t.k === 'slash') slashAt = i;
	});
	const out: string[] = s.toks.map(() => '');
	if (s.general) {
		s.toks.forEach((t, i) => t.k === 'general' && (out[i] = general(x, loc)));
		if (!s.digits) return join(s, out, loc);
	}
	if (slashAt >= 0 && decAt < 0 && expAt < 0) return formatFraction(s, x, loc, slashAt);
	if (expAt >= 0) {
		const k = Math.max(1, intPh.length);
		const hash = intPh.some((i) => s.toks[i].r === '#');
		const step = hash && k > 1 ? k : 1;
		let exp = 0;
		if (x !== 0) {
			exp = Math.floor(Math.log10(x));
			if (step > 1) exp = Math.floor(exp / step) * step;
			else exp -= k - 1;
			if (Number(fixed(x / 10 ** exp, fracPh.length)) >= 10 ** k) exp += step;
			x /= 10 ** exp;
		}
		const t = s.toks[expAt].s!;
		out[expAt] = t[0] + (exp < 0 ? '-' : t.endsWith('+') ? '+' : '');
		fillInt(s, out, expPh, String(Math.abs(exp)), false, loc);
	}
	const parts = fixed(x, fracPh.length).split('.');
	const ip = parts[0] === '0' ? '' : parts[0];
	const fp = parts[1] ?? '';
	fillInt(s, out, intPh, ip, s.group, loc);
	let keep = fp.length;
	while (keep > 0 && fp[keep - 1] === '0' && s.toks[fracPh[keep - 1]].r !== '0') keep--;
	fracPh.forEach((i, j) => {
		if (j < keep) out[i] = fp[j];
		else if (s.toks[i].r === '?') out[i] = ' ';
	});
	if (keep === 0 && fracPh.length > 0 && out[fracPh[fracPh.length - 1]] === '') out[decAt] = NO_DEC; // « 0,## » : 3 et non « 3, »
	return join(s, out, loc);
}

/** Chiffres d'une partie entière répartis sur ses 0 # ? (de droite à gauche ; le surplus va au premier). */
function fillInt(s: Section, out: string[], ph: number[], digits: string, group: boolean, loc: NumLocale) {
	if (!ph.length) return;
	if (group) {
		const b: string[] = [];
		for (let j = ph.length - 1; j >= 0; j--) {
			const k = digits.length - (ph.length - j);
			if (k >= 0) b.unshift(digits[k]);
			else if (s.toks[ph[j]].r === '0') b.unshift('0');
			else if (s.toks[ph[j]].r === '?') b.unshift(' ');
		}
		const extra = digits.length - ph.length;
		if (extra > 0) b.unshift(...digits.slice(0, extra).split(''));
		let n = b.filter((r) => r >= '0' && r <= '9').length;
		let g = '';
		for (const r of b) {
			if (r >= '0' && r <= '9') {
				g += r;
				n--;
				if (n > 0 && n % 3 === 0) g += loc.group;
			} else g += r;
		}
		out[ph[0]] = g;
		return;
	}
	let d = digits;
	for (let j = ph.length - 1; j >= 0; j--) {
		const i = ph[j];
		if (j === 0 && d.length) out[i] = d;
		else if (d.length) {
			out[i] = d.slice(-1);
			d = d.slice(0, -1);
		} else if (s.toks[i].r === '0') out[i] = '0';
		else if (s.toks[i].r === '?') out[i] = ' ';
	}
}

function join(s: Section, out: string[], loc: NumLocale): string {
	let b = '';
	s.toks.forEach((t, i) => {
		switch (t.k) {
			case 'lit':
				b += t.s;
				break;
			case 'dec':
				if (out[i] !== NO_DEC) b += loc.decimal;
				break;
			case 'fill':
				b += FILL + t.s;
				break;
			case 'den':
				b += out[i] || t.s;
				break;
			default:
				b += out[i];
		}
	});
	return b;
}

function formatFraction(s: Section, x: number, loc: NumLocale, slash: number): string {
	let numStart = slash;
	while (numStart > 0 && s.toks[numStart - 1].k === 'digit') numStart--;
	const intPh: number[] = [];
	const numPh: number[] = [];
	const denPh: number[] = [];
	let fixedDen = 0;
	s.toks.forEach((t, i) => {
		if (t.k === 'digit') (i < numStart ? intPh : i < slash ? numPh : denPh).push(i);
		else if (t.k === 'den') fixedDen = Number(t.s);
	});
	let whole = 0;
	let frac = x;
	if (intPh.length) {
		whole = Math.floor(x);
		frac = x - whole;
	}
	let num: number;
	let den: number;
	if (fixedDen > 0) {
		den = fixedDen;
		num = Math.round(frac * den);
	} else [num, den] = approx(frac, 10 ** Math.max(1, denPh.length) - 1);
	if (intPh.length && num === den) {
		whole++;
		num = 0;
	}
	const out: string[] = s.toks.map(() => '');
	let wholeDigits = whole > 0 ? whole.toFixed(0) : '';
	const blank = intPh.length > 0 && num === 0;
	if (blank && !wholeDigits) wholeDigits = '0';
	fillInt(s, out, intPh, wholeDigits, s.group, loc);
	if (blank) {
		for (let i = numStart; i < s.toks.length; i++) {
			const t = s.toks[i];
			if (t.k === 'digit' || t.k === 'slash') out[i] = ' ';
			else if (t.k === 'den') out[i] = ' '.repeat(t.s!.length);
		}
		return join(s, out, loc);
	}
	fillInt(s, out, numPh, String(num), false, loc);
	out[slash] = '/';
	const ds = String(den);
	denPh.forEach((i, j) => {
		if (j < ds.length && j === denPh.length - 1) out[i] = ds.slice(j);
		else if (j < ds.length) out[i] = ds[j];
		else if (s.toks[i].r === '0') out[i] = '0';
		else if (s.toks[i].r === '?') out[i] = ' ';
	});
	return join(s, out, loc);
}

/** Meilleure fraction de x avec un dénominateur au plus maxDen (Stern-Brocot). */
function approx(x: number, maxDen: number): [number, number] {
	maxDen = Math.max(1, maxDen);
	const whole = Math.floor(x);
	const f = x - whole;
	let [ln, ld, rn, rd] = [0, 1, 1, 1];
	let [bn, bd] = f > 0.5 ? [1, 1] : [0, 1];
	let best = Math.abs(f - bn / bd);
	for (;;) {
		const mn = ln + rn;
		const md = ld + rd;
		if (md > maxDen) break;
		const v = mn / md;
		const d = Math.abs(f - v);
		if (d < best - 1e-12) [best, bn, bd] = [d, mn, md];
		if (f < v) [rn, rd] = [mn, md];
		else if (f > v) [ln, ld] = [mn, md];
		else break;
	}
	return [bn + whole * bd, bd];
}

// --- Dates -----------------------------------------------------------------------

const EPOCH = Date.UTC(1899, 11, 30);
const pad = (n: number, w: number) => String(n).padStart(w, '0');

function formatDate(s: Section, x: number, loc: NumLocale): string | null {
	if (x < 0 || x >= 2958466) return null;
	let fd = 0;
	for (const t of s.toks) if (t.k === 'dec') fd = Math.max(fd, t.n ?? 0);
	const unit = 10 ** fd;
	const ms = Math.round(x * 86400 * unit);
	const days = Math.floor(ms / (86400 * unit));
	const rest = ms - days * 86400 * unit;
	const tm = new Date(EPOCH + days * 86400000 + Math.floor(rest / unit) * 1000);
	const sub = rest % unit;
	const ampm = s.toks.some((t) => t.k === 'ampm');
	let b = '';
	s.toks.forEach((t, i) => {
		switch (t.k) {
			case 'lit':
				b += t.s;
				break;
			case 'fill':
				b += FILL + t.s;
				break;
			case 'dec':
				b += loc.decimal + pad(sub, t.n ?? 0);
				break;
			case 'ampm': {
				const pm = tm.getUTCHours() >= 12;
				b += t.s === 'A/P' ? (pm ? 'P' : 'A') : pm ? 'PM' : 'AM';
				break;
			}
			case 'elapsed': {
				const total = ms / unit;
				b += pad(Math.floor(t.r === 'h' ? total / 3600 : t.r === 'm' ? total / 60 : total), t.n ?? 1);
				break;
			}
			case 'date': {
				const n = t.n ?? 1;
				switch (t.r) {
					case 'j':
						b += n >= 4 ? loc.days[tm.getUTCDay()] : n === 3 ? loc.daysShort[tm.getUTCDay()] : pad(tm.getUTCDate(), n);
						break;
					case 'm':
						if (n <= 2 && minuteAt(s.toks, i)) b += pad(tm.getUTCMinutes(), n);
						else if (n >= 5) b += Array.from(loc.months[tm.getUTCMonth()])[0].toUpperCase();
						else if (n === 4) b += loc.months[tm.getUTCMonth()];
						else if (n === 3) b += loc.monthsShort[tm.getUTCMonth()];
						else b += pad(tm.getUTCMonth() + 1, n);
						break;
					case 'a':
						b += n <= 2 ? pad(tm.getUTCFullYear() % 100, 2) : pad(tm.getUTCFullYear(), 4);
						break;
					case 'h': {
						let h = tm.getUTCHours();
						if (ampm) h = h % 12 || 12;
						b += pad(h, Math.min(n, 2));
						break;
					}
					case 's':
						b += pad(tm.getUTCSeconds(), Math.min(n, 2));
				}
			}
		}
	});
	return b;
}

/** « m » ou « mm » : minutes juste après des heures ou juste avant des secondes. */
function minuteAt(toks: Tok[], i: number): boolean {
	for (let j = i - 1; j >= 0; j--) {
		if (toks[j].k === 'date' || toks[j].k === 'elapsed') {
			if (toks[j].r === 'h') return true;
			break;
		}
	}
	for (let j = i + 1; j < toks.length; j++) if (toks[j].k === 'date' || toks[j].k === 'elapsed') return toks[j].r === 's';
	return false;
}

/** Numéro de série d'une date (jours depuis le 30/12/1899, la fraction est l'heure). */
export function toSerial(p: { y: number; m: number; d: number; h: number; mi: number; s: number }): number {
	return (Date.UTC(p.y, p.m - 1, p.d) - EPOCH) / 86400000 + (p.h * 3600 + p.mi * 60 + p.s) / 86400;
}
