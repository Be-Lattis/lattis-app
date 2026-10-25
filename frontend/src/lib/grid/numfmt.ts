// Formats d'affichage du dialogue « Format de cellule » (Ctrl+1), comme
// Excel : nombre, monétaire, pourcentage (voir style.ts), date, heure et
// texte. Le texte affiché d'une cellule passe par formatCell : la valeur
// stockée ne change pas, seul son affichage.

import { dateStyle, dateTimeFormat, lang, parseNumberText, shortDate } from '#lib/i18n/index.js';
import { isErrorValue } from './format';
import { FILL, parseCode, plain, toSerial } from './numcode';
import { formatNumber, isNumericText, styledNumber, type CellStyle } from './style';

/** Formats de date proposés (motifs d'Excel en français). */
export const DATE_FORMATS = ['jj/mm/aaaa', 'jj/mm/aa', 'jj mmm aaaa', 'jj mmmm aaaa', 'jjjj jj mmmm aaaa', 'aaaa-mm-jj', 'mm/aaaa', 'mmmm aaaa', 'jj/mm'];

/** Formats d'heure proposés. */
export const TIME_FORMATS = ['hh:mm', 'hh:mm:ss', 'jj/mm/aaaa hh:mm'];

export const DEFAULT_DATE = 'jj/mm/aaaa';
export const DEFAULT_TIME = 'hh:mm';

export interface DateTime {
	y: number;
	m: number; // 1 à 12
	d: number;
	h: number;
	mi: number;
	s: number;
}

const MONTHS = ['janvier', 'février', 'mars', 'avril', 'mai', 'juin', 'juillet', 'août', 'septembre', 'octobre', 'novembre', 'décembre'];
const MONTHS_SHORT = ['janv.', 'févr.', 'mars', 'avr.', 'mai', 'juin', 'juil.', 'août', 'sept.', 'oct.', 'nov.', 'déc.'];
const DAYS = ['dimanche', 'lundi', 'mardi', 'mercredi', 'jeudi', 'vendredi', 'samedi'];
const DAYS_SHORT = ['dim.', 'lun.', 'mar.', 'mer.', 'jeu.', 'ven.', 'sam.'];

/**
 * Nom d'un mois (0 à 11) ou d'un jour (0 : dimanche) dans la langue de
 * l'interface : les listes ci-dessus en français, Intl ailleurs (le motif
 * du format, lui, reste écrit à la française : jj/mm/aaaa).
 */
function dateName(kind: 'month' | 'weekday', width: 'long' | 'short', i: number): string {
	if (lang() === 'fr') return (kind === 'month' ? (width === 'long' ? MONTHS : MONTHS_SHORT) : width === 'long' ? DAYS : DAYS_SHORT)[i];
	// 2023-01-01 était un dimanche.
	const d = kind === 'month' ? Date.UTC(2023, i, 1) : Date.UTC(2023, 0, 1 + i);
	return dateTimeFormat({ [kind]: width, timeZone: 'UTC' }).format(d);
}

function valid(p: DateTime): DateTime | null {
	if (p.m < 1 || p.m > 12 || p.d < 1 || p.h > 23 || p.mi > 59 || p.s > 59) return null;
	const t = new Date(Date.UTC(p.y, p.m - 1, p.d));
	return t.getUTCDate() === p.d && t.getUTCMonth() === p.m - 1 ? p : null;
}

/**
 * Date ou heure lue dans un texte affiché : « 16/10/2026 », « 16/10/26 »,
 * « 2026-10-16 », avec ou sans heure (« 2026-10-16T08:30 »,
 * « 16/10/2026 08:30:15 »), ou une heure seule (« 08:30 »).
 */
export function parseDateTime(text: string): DateTime | null {
	const t = text.trim();
	let m = /^(\d{4})-(\d{1,2})-(\d{1,2})(?:[T ](\d{1,2}):(\d{2})(?::(\d{2}))?)?/.exec(t) ?? /^(\d{4})\/(\d{1,2})\/(\d{1,2})(?: (\d{1,2}):(\d{2})(?::(\d{2}))?)?$/.exec(t);
	if (m) return valid({ y: +m[1], m: +m[2], d: +m[3], h: +(m[4] ?? 0), mi: +(m[5] ?? 0), s: +(m[6] ?? 0) });
	// Date courte de la langue de la session (le moteur l'écrit ainsi) :
	// 16/10/2026, 10/16/2026 (anglais), 16.10.2026 (allemand).
	m = /^(\d{1,2})[/.](\d{1,2})[/.](\d{4}|\d{2})(?: (\d{1,2}):(\d{2})(?::(\d{2}))?)?$/.exec(t);
	if (m) {
		const y = m[3].length === 2 ? 2000 + +m[3] : +m[3];
		const [d, mo] = dateStyle().order === 'dmy' ? [+m[1], +m[2]] : [+m[2], +m[1]];
		return valid({ y, m: mo, d, h: +(m[4] ?? 0), mi: +(m[5] ?? 0), s: +(m[6] ?? 0) });
	}
	m = /^(\d{1,2}):(\d{2})(?::(\d{2}))?$/.exec(t);
	if (m) return valid({ y: 1899, m: 12, d: 30, h: +m[1], mi: +m[2], s: +(m[3] ?? 0) });
	return null;
}

/** Numéro de série d'Excel (jours depuis le 30/12/1899, la fraction est l'heure). */
export function fromSerial(n: number): DateTime | null {
	if (!Number.isFinite(n) || n < 0 || n > 2958465) return null;
	const ms = Math.round(n * 86400) * 1000;
	const t = new Date(Date.UTC(1899, 11, 30) + ms);
	return { y: t.getUTCFullYear(), m: t.getUTCMonth() + 1, d: t.getUTCDate(), h: t.getUTCHours(), mi: t.getUTCMinutes(), s: t.getUTCSeconds() };
}

const pad = (n: number, w = 2) => String(n).padStart(w, '0');

/**
 * Date mise au motif : aaaa, aa (année), mmmm, mmm, mm (mois), jjjj, jjj,
 * jj (jour), hh, mm, ss (heure, minutes, secondes). Comme Excel, « mm »
 * désigne les minutes juste après « hh » ou juste avant « ss ».
 */
export function formatDateTime(p: DateTime, pattern: string): string {
	// Date courte (le format par défaut, ruban › Date courte) : celle de la
	// langue, comme la date courte d'Excel suit le système.
	if (pattern === DEFAULT_DATE) return shortDate(p.y, p.m, p.d);
	if (pattern === 'jj/mm/aaaa hh:mm') return `${shortDate(p.y, p.m, p.d)} ${pad(p.h)}:${pad(p.mi)}`;
	const tokens = /aaaa|aa|mmmm|mmm|mm|jjjj|jjj|jj|hh|ss|[^amjhs]+|./g;
	const parts = pattern.match(tokens) ?? [];
	const isField = (t: string) => /^(aaaa|aa|mmmm|mmm|mm|jjjj|jjj|jj|hh|ss)$/.test(t);
	const prevField = (k: number) => {
		for (let j = k - 1; j >= 0; j--) if (isField(parts[j])) return parts[j];
		return '';
	};
	const nextField = (k: number) => {
		for (let j = k + 1; j < parts.length; j++) if (isField(parts[j])) return parts[j];
		return '';
	};
	const weekday = new Date(Date.UTC(p.y, p.m - 1, p.d)).getUTCDay();
	return parts
		.map((t, k) => {
			switch (t) {
				case 'aaaa':
					return pad(p.y, 4);
				case 'aa':
					return pad(p.y % 100);
				case 'mmmm':
					return dateName('month', 'long', p.m - 1);
				case 'mmm':
					return dateName('month', 'short', p.m - 1);
				case 'mm':
					return prevField(k) === 'hh' || nextField(k) === 'ss' ? pad(p.mi) : pad(p.m);
				case 'jjjj':
					return dateName('weekday', 'long', weekday);
				case 'jjj':
					return dateName('weekday', 'short', weekday);
				case 'jj':
					return pad(p.d);
				case 'hh':
					return pad(p.h);
				case 'ss':
					return pad(p.s);
			}
			return t;
		})
		.join('');
}

/** Date ou heure d'une cellule : valeur numérique (numéro de série) ou texte affiché. */
function dateOf(text: string, value: unknown): DateTime | null {
	if (typeof value === 'number') return fromSerial(value);
	const p = parseDateTime(text);
	if (p) return p;
	if (isNumericText(text) && !text.includes('%')) return fromSerial(Number(text.replace(/[\s  ]/g, '').replace(',', '.')));
	return null;
}

/** Valeur d'une cellule pour un code : nombre (valeur, texte d'un nombre ou d'une date) ou texte. */
function codeValue(text: string, value: unknown): number | string {
	if (typeof value === 'number') return value;
	if (typeof value === 'boolean' || isErrorValue(text)) return text;
	if (isNumericText(text)) {
		const n = parseNumberText(text.replace(/[\s  ]/g, ''));
		if (n !== null) return n;
	}
	const p = parseDateTime(text);
	return p ? toSerial(p) : text;
}

/**
 * Texte affiché d'une cellule et couleur de son code personnalisé (nc) :
 * le remplissage (FILL, comptabilité) reste marqué pour le dessin de la
 * grille, qui aligne ce qui le suit à droite. Sans code : formatCell.
 */
export function shownCell(text: string, value: unknown, st: CellStyle | undefined): { text: string; color?: string } {
	if (!st?.nc || !text) return { text: formatCell(text, value, st) };
	const v = codeValue(text, value);
	const f = parseCode(st.nc);
	const r = typeof v === 'number' ? f.number(v) : f.text(v);
	return r.ok ? { text: r.text, color: r.color } : { text: '#####' };
}

/** Le texte affiché a-t-il un remplissage (comptabilité) ? */
export const hasFill = (s: string) => s.includes(FILL);

/**
 * Texte affiché d'une cellule selon son format (style) : text est le texte
 * déjà affiché, value la valeur quand on la connaît (cellule de tableau).
 * Ce qui ne se lit pas dans le format reste tel quel.
 */
export function formatCell(text: string, value: unknown, st: CellStyle | undefined): string {
	if (!st || !text) return text;
	if (st.nc) return plain(shownCell(text, value, st).text);
	switch (st.nf) {
		case 'text':
			return text;
		case 'date':
		case 'time': {
			const p = dateOf(text, value);
			return p ? formatDateTime(p, st.df || (st.nf === 'date' ? DEFAULT_DATE : DEFAULT_TIME)) : text;
		}
	}
	if (st.nf === undefined && st.dp === undefined && !st.ng) return text;
	if (typeof value === 'number') return formatNumber(st.nf, value, st.dp, st.ng) ?? text;
	if (st.ng && st.nf === undefined && st.dp === undefined && isNumericText(text)) return text.replace(/[\s  ](?=\d)/g, '');
	return styledNumber(text, st);
}

/** Catégorie du dialogue Format de cellule d'après un style. */
export type Category = 'standard' | 'number' | 'currency' | 'accounting' | 'date' | 'time' | 'percent' | 'fraction' | 'scientific' | 'text' | 'custom';

/** Symboles monétaires proposés (Monétaire, Comptabilité) ; l'euro sans code garde nf '€'. */
export const CURRENCIES = ['€', '$', '£', 'CHF', '¥', '₹'];

/** Fractions proposées (codes). */
export const FRACTIONS = ['# ?/?', '# ??/??', '# ???/???', '# ?/2', '# ?/4', '# ?/8', '# ??/16', '# ?/10', '# ??/100'];

/** Codes d'exemple de la catégorie Personnalisé. */
export const CUSTOM_CODES = [
	'0',
	'0,00',
	'# ##0',
	'# ##0,00',
	'# ##0,00 "kg"',
	'# ##0;[Rouge]-# ##0',
	'0,00 %',
	'0,00E+00',
	'# ?/?',
	'0;-0;"–"',
	'jj/mm/aaaa hh:mm',
	'jjjj j mmmm aaaa',
	'[h]:mm:ss',
	'"N° "000',
	'@'
];

/** Options de format lues dans un style (dialogue Format de cellule). */
export interface FormatOpts {
	dp?: number;
	sep?: boolean;
	df?: string;
	/** Symbole monétaire et sa place (avant le montant). */
	sym?: string;
	before?: boolean;
	/** Nombres négatifs en rouge. */
	red?: boolean;
	/** Code (Fraction, Personnalisé). */
	code?: string;
}

/** Partie numérique d'un code : milliers et décimales. */
function numPart(dp: number, sep = true): string {
	return (sep ? '# ##0' : '0') + (dp > 0 ? ',' + '0'.repeat(dp) : '');
}

/** Code monétaire : symbole avant ou après, négatifs en rouge. */
export function currencyCode(sym: string, before: boolean, dp: number, red = false): string {
	const s = `[$${sym}]`;
	const n = numPart(dp);
	const pos = before ? (/[A-Za-z]/.test(sym) ? `${s} ${n}` : `${s}${n}`) : `${n} ${s}`;
	return red ? `${pos};[Rouge]-${pos}` : pos;
}

/** Code comptabilité : symbole à gauche ou à droite, montant aligné à droite, zéro en tiret. */
export function accountingCode(sym: string, before: boolean, dp: number): string {
	const s = `[$${sym}]`;
	const n = numPart(dp);
	const q = '?'.repeat(dp);
	return before ? `_-${s}* ${n}_-;-${s}* ${n}_-;_-${s}* "-"${q}_-;_-@_-` : `_-* ${n} ${s}_-;-* ${n} ${s}_-;_-* "-"${q} ${s}_-;_-@_-`;
}

/** Code équivalent à une catégorie (Personnalisé part du format en cours). */
export function codeOf(cat: Category, opts: FormatOpts = {}): string {
	const p = formatPatch(cat, opts);
	if (typeof p.nc === 'string') return p.nc;
	const dp = Number(p.dp ?? 0);
	switch (p.nf) {
		case '0,00':
			return numPart(dp, opts.sep !== false);
		case '€':
			return `${numPart(dp)} [$€]`;
		case '%':
			return '0' + (dp > 0 ? ',' + '0'.repeat(dp) : '') + '%';
		case 'date':
		case 'time':
			return String(p.df);
		case 'text':
			return '@';
	}
	return 'Standard';
}

/**
 * Une décimale de plus ou de moins dans un code (Ajouter / Réduire les
 * décimales), dans chaque section ; un code de date ne change pas.
 */
export function bumpDecimals(code: string, delta: 1 | -1): string {
	if (parseCode(code).isDate()) return code;
	return code
		.split(';')
		.map((sec) => {
			const m = /,(0+)/.exec(sec);
			if (m) {
				const n = m[1].length + delta;
				return sec.slice(0, m.index) + (n > 0 ? ',' + '0'.repeat(n) : '') + sec.slice(m.index + m[0].length);
			}
			return delta > 0 ? sec.replace(/0(?![0-9#?]| [0#])/, '0,0') : sec;
		})
		.join(';');
}

/** Symbole par défaut avant le montant ? */
export const symbolBefore = (sym: string) => sym === '$' || sym === '£' || sym === '¥' || sym === '₹';

export function categoryOf(st: CellStyle | undefined): Category {
	if (st?.nc) {
		const c = st.nc;
		if (/^_-/.test(c) && c.includes('*')) return 'accounting';
		if (/^(\[\$[^\]]+\] ?)?(?:# ##0|0)(,0+)?( \[\$[^\]]+\])?(;\[Rouge\]-.*)?$/.test(c)) return c.includes('[$') ? 'currency' : 'number';
		if (/^0(,0+)?E\+00$/.test(c)) return 'scientific';
		if (FRACTIONS.includes(c)) return 'fraction';
		return 'custom';
	}
	switch (st?.nf) {
		case '0':
		case '0,00':
			return 'number';
		case '€':
			return 'currency';
		case '%':
			return 'percent';
		case 'date':
			return 'date';
		case 'time':
			return 'time';
		case 'text':
			return 'text';
	}
	return st?.dp !== undefined || st?.ng ? 'number' : 'standard';
}

/** Options d'un style (pour rouvrir le dialogue sur le format en cours). */
export function formatOpts(st: CellStyle | undefined): FormatOpts {
	const c = st?.nc;
	if (!c) return { dp: st?.dp ?? (st?.nf === '0' || st?.nf === '%' ? 0 : 2), sep: !st?.ng, df: st?.df, sym: '€', before: false };
	const first = c.split(';')[0];
	const m = /\[\$([^\]-]+)/.exec(first);
	const at = first.search(/[0#?]/);
	return {
		dp: /,(0+)/.exec(first)?.[1].length ?? 0,
		sep: first.includes('# ##'),
		sym: m?.[1] ?? '€',
		before: !!m && first.indexOf('[$') < at,
		red: /;\[Rouge\]/i.test(c),
		code: c
	};
}

/** Correctif de style d'une catégorie (toutes les propriétés de format sont posées ou retirées). */
export function formatPatch(cat: Category, opts: FormatOpts = {}): Record<string, unknown> {
	const p: Record<string, unknown> = { nf: null, dp: null, ng: null, df: null, nc: null };
	const dp = opts.dp ?? 2;
	switch (cat) {
		case 'number':
			if (opts.red) {
				const n = numPart(dp, opts.sep !== false);
				p.nc = `${n};[Rouge]-${n}`;
				break;
			}
			p.nf = '0,00';
			p.dp = dp;
			if (opts.sep === false) p.ng = true;
			break;
		case 'currency':
			// L'euro après le montant : le format monétaire de la langue (nf '€').
			if ((opts.sym ?? '€') === '€' && !opts.before && !opts.red) {
				p.nf = '€';
				p.dp = dp;
			} else p.nc = currencyCode(opts.sym ?? '€', !!opts.before, dp, opts.red);
			break;
		case 'accounting':
			p.nc = accountingCode(opts.sym ?? '€', !!opts.before, dp);
			break;
		case 'percent':
			p.nf = '%';
			p.dp = opts.dp ?? 0;
			break;
		case 'scientific':
			p.nc = '0' + (dp > 0 ? ',' + '0'.repeat(dp) : '') + 'E+00';
			break;
		case 'fraction':
			p.nc = opts.code && FRACTIONS.includes(opts.code) ? opts.code : FRACTIONS[0];
			break;
		case 'custom':
			p.nc = opts.code?.trim() || null;
			break;
		case 'date':
		case 'time':
			p.nf = cat;
			p.df = opts.df ?? (cat === 'date' ? DEFAULT_DATE : DEFAULT_TIME);
			break;
		case 'text':
			p.nf = 'text';
			break;
	}
	return p;
}
