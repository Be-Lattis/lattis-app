// Formats d'affichage du dialogue « Format de cellule » (Ctrl+1), comme
// Excel : nombre, monétaire, pourcentage (voir style.ts), date, heure et
// texte. Le texte affiché d'une cellule passe par formatCell : la valeur
// stockée ne change pas, seul son affichage.

import { dateTimeFormat, lang } from '#lib/i18n/index.js';
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
	let m = /^(\d{4})-(\d{1,2})-(\d{1,2})(?:[T ](\d{1,2}):(\d{2})(?::(\d{2}))?)?/.exec(t);
	if (m) return valid({ y: +m[1], m: +m[2], d: +m[3], h: +(m[4] ?? 0), mi: +(m[5] ?? 0), s: +(m[6] ?? 0) });
	m = /^(\d{1,2})\/(\d{1,2})\/(\d{4}|\d{2})(?: (\d{1,2}):(\d{2})(?::(\d{2}))?)?$/.exec(t);
	if (m) {
		const y = m[3].length === 2 ? 2000 + +m[3] : +m[3];
		return valid({ y, m: +m[2], d: +m[1], h: +(m[4] ?? 0), mi: +(m[5] ?? 0), s: +(m[6] ?? 0) });
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

/**
 * Texte affiché d'une cellule selon son format (style) : text est le texte
 * déjà affiché, value la valeur quand on la connaît (cellule de tableau).
 * Ce qui ne se lit pas dans le format reste tel quel.
 */
export function formatCell(text: string, value: unknown, st: CellStyle | undefined): string {
	if (!st || !text) return text;
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
export type Category = 'standard' | 'number' | 'currency' | 'percent' | 'date' | 'time' | 'text';

export function categoryOf(st: CellStyle | undefined): Category {
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

/** Correctif de style d'une catégorie (toutes les propriétés de format sont posées ou retirées). */
export function formatPatch(cat: Category, opts: { dp?: number; sep?: boolean; df?: string } = {}): Record<string, unknown> {
	const p: Record<string, unknown> = { nf: null, dp: null, ng: null, df: null };
	switch (cat) {
		case 'number':
			p.nf = '0,00';
			p.dp = opts.dp ?? 2;
			if (opts.sep === false) p.ng = true;
			break;
		case 'currency':
			p.nf = '€';
			p.dp = opts.dp ?? 2;
			break;
		case 'percent':
			p.nf = '%';
			p.dp = opts.dp ?? 0;
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
