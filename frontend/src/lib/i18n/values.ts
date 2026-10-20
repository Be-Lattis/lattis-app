// Formules et valeurs tapées dans la langue de la session (docs/i18n.md,
// § 5), comme Excel suit la langue du système : séparateur d'arguments,
// dates courtes, VRAI/FAUX, codes d'erreur, noms de fonctions. Le moteur
// (internal/canvas/session.go) reçoit et rend ces textes dans la langue de
// la session et garde la forme qui n'en dépend pas ; ces fonctions servent
// à l'interface pour écrire et relire les mêmes textes. Chaque fonction
// prend la langue en dernier paramètre (par défaut celle de la session).

import type { Lang } from './core.js';
import { lang } from './i18n.svelte.js';

/** Ordre du jour, du mois et de l'année d'une date courte. */
export type DateOrder = 'dmy' | 'mdy' | 'ymd';

/** Écriture des dates courtes : 19/10/2026 (fr, es), 19.10.2026 (de), 10/19/2026 (en), 2026/10/19 (zh). */
export function dateStyle(l: Lang = lang()): { order: DateOrder; sep: string } {
	switch (l) {
		case 'en':
			return { order: 'mdy', sep: '/' };
		case 'zh':
			return { order: 'ymd', sep: '/' };
		case 'de':
			return { order: 'dmy', sep: '.' };
	}
	return { order: 'dmy', sep: '/' };
}

const pad = (n: number, w = 2) => String(n).padStart(w, '0');

/** Date courte (année, mois 1 à 12, jour) écrite dans la langue. */
export function shortDate(y: number, m: number, d: number, l: Lang = lang()): string {
	const { order, sep } = dateStyle(l);
	const [dd, mm, yy] = [pad(d), pad(m), pad(y, 4)];
	return order === 'mdy' ? [mm, dd, yy].join(sep) : order === 'ymd' ? [yy, mm, dd].join(sep) : [dd, mm, yy].join(sep);
}

/** Date ISO (aaaa-mm-jj) d'une colonne Date écrite dans la langue ; un autre texte tel quel. */
export function isoToShort(iso: string, l: Lang = lang()): string {
	const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
	return m ? shortDate(+m[1], +m[2], +m[3], l) : iso;
}

/** Motif d'une date courte pour les indications (« jj/mm/aaaa », « mm/dd/yyyy »…). */
export function datePattern(l: Lang = lang()): string {
	switch (l) {
		case 'en':
			return 'mm/dd/yyyy';
		case 'zh':
			return 'yyyy/mm/dd';
		case 'de':
			return 'TT.MM.JJJJ';
		case 'es':
			return 'dd/mm/aaaa';
	}
	return 'jj/mm/aaaa';
}

function validDate(y: number, m: number, d: number): { y: number; m: number; d: number } | null {
	if (m < 1 || m > 12 || d < 1) return null;
	const t = new Date(Date.UTC(y, m - 1, d));
	return t.getUTCFullYear() === y && t.getUTCMonth() === m - 1 && t.getUTCDate() === d ? { y, m, d } : null;
}

/**
 * Date courte tapée dans la langue (année sur 2 ou 4 chiffres ; séparateur
 * « / », « . » ou « - ») ou ISO (aaaa-mm-jj, aaaa/mm/jj) ; null sinon.
 */
export function parseShortDate(text: string, l: Lang = lang()): { y: number; m: number; d: number } | null {
	const s = text.trim();
	let m = /^(\d{4})[-/.](\d{1,2})[-/.](\d{1,2})$/.exec(s);
	if (m) return validDate(+m[1], +m[2], +m[3]);
	m = /^(\d{1,2})[/.-](\d{1,2})[/.-](\d{4}|\d{2})$/.exec(s);
	if (!m) return null;
	const y = m[3].length === 2 ? (+m[3] < 30 ? 2000 : 1900) + +m[3] : +m[3];
	const [a, b] = [+m[1], +m[2]];
	return dateStyle(l).order === 'dmy' ? validDate(y, b, a) : validDate(y, a, b);
}

/** Séparateur des arguments d'une formule : « , » en anglais et en chinois, « ; » ailleurs. */
export function formulaSep(l: Lang = lang()): ',' | ';' {
	return l === 'en' || l === 'zh' ? ',' : ';';
}

/** Décimale d'un nombre tapé dans une formule ou une cellule : « . » avec le séparateur « , », « , » sinon. */
export function formulaDecimal(l: Lang = lang()): '.' | ',' {
	return formulaSep(l) === ',' ? '.' : ',';
}

/**
 * Nom d'une fonction de formule dans la langue : français en français,
 * anglais ailleurs. L'allemand et l'espagnol prendront les leurs avec leurs
 * colonnes de noms (lot 5 de docs/i18n.md, formula.funcDef.name côté Go) :
 * c'est ici qu'il faudra les ajouter, avec le catalogue rendu par le moteur.
 */
export function fnName(en: string, fr: string, l: Lang = lang()): string {
	return l === 'fr' ? fr : en;
}

/** VRAI ou FAUX dans la langue des formules (TRUE/FALSE hors français, comme les noms de fonctions). */
export function boolText(b: boolean, l: Lang = lang()): string {
	if (l === 'fr') return b ? 'VRAI' : 'FAUX';
	return b ? 'TRUE' : 'FALSE';
}

/**
 * Mots acceptés pour un booléen tapé (en minuscules) : ceux de la langue,
 * plus true/false et les mots français (vrai, faux, oui, non) partout, et
 * 1/0.
 */
export function boolWords(l: Lang = lang()): { yes: string[]; no: string[] } {
	const yes = ['true', 'vrai', 'oui', '1'];
	const no = ['false', 'faux', 'non', '0'];
	const own: Record<Lang, [string[], string[]]> = {
		fr: [[], []],
		en: [['yes'], ['no']],
		de: [['wahr', 'ja'], ['falsch', 'nein']],
		es: [['verdadero', 'sí', 'si'], ['falso', 'no']],
		zh: [['是', '真'], ['否', '假']]
	};
	return { yes: [...own[l][0], ...yes], no: [...own[l][1], ...no] };
}

/** Booléen tapé dans la langue (voir boolWords) ; null si ce n'en est pas un. */
export function parseBool(text: string, l: Lang = lang()): boolean | null {
	const s = text.trim().toLowerCase();
	const w = boolWords(l);
	if (w.yes.includes(s)) return true;
	if (w.no.includes(s)) return false;
	return null;
}

const ERRORS_EN: Record<string, string> = {
	'#VALEUR!': '#VALUE!',
	'#NOM?': '#NAME?',
	'#NOMBRE!': '#NUM!',
	'#DEVERSEMENT!': '#SPILL!'
};

/** Code d'erreur d'une formule (« #VALEUR! ») écrit dans la langue (« #VALUE! » hors français). */
export function errorCode(code: string, l: Lang = lang()): string {
	return l === 'fr' ? code : (ERRORS_EN[code] ?? code);
}

/** Code d'erreur en français, quelle que soit la langue où il est écrit (pour l'expliquer). */
export function frenchErrorCode(code: string): string {
	for (const [fr, en] of Object.entries(ERRORS_EN)) if (en === code) return fr;
	return code;
}

/**
 * Nombre écrit dans la langue (« 1 234,5 », « 1,234.5 », « 1.234,5 »,
 * « 12,5 % ») ; null si le texte n'en est pas un. La décimale de la
 * langue l'emporte quand le texte est ambigu ; l'autre reste acceptée.
 */
export function parseNumberText(text: string, l: Lang = lang()): number | null {
	let s = text.trim().replace(/[\s  ]/g, '');
	const pct = s.endsWith('%');
	if (pct) s = s.slice(0, -1);
	if (!/^[-+]?[\d.,]*\d[\d.,]*$/.test(s)) return null;
	const dec = formulaDecimal(l);
	// Séparateur des milliers écrit sans espace : « , » en anglais, « . » en
	// allemand et en espagnol ; aucun en français (« 1.234 » y vaut 1,234,
	// comme pour le moteur).
	const group = dec === '.' ? ',' : l === 'fr' ? '' : '.';
	const lastDot = s.lastIndexOf('.');
	const lastComma = s.lastIndexOf(',');
	let d = dec;
	if (lastDot >= 0 && lastComma >= 0) d = lastDot > lastComma ? '.' : ',';
	else if (!group && lastDot >= 0) d = '.';
	else if (group && s.includes(group) && !s.includes(dec)) {
		// Un seul séparateur, celui des milliers de la langue : des milliers
		// s'il est suivi de groupes de trois chiffres, sinon une décimale.
		d = new RegExp(`^[-+]?\\d{1,3}(\\${group}\\d{3})+$`).test(s) ? dec : group;
	}
	const other = d === '.' ? ',' : '.';
	const n = Number(s.split(other).join('').replace(d, '.'));
	if (!Number.isFinite(n)) return null;
	return pct ? n / 100 : n;
}
