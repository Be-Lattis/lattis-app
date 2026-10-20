// Affichage des cellules libres (hors tableaux), comme Excel : les nombres,
// dates et heures à droite, les booléens centrés, les textes à gauche ; une
// apostrophe de tête force le texte et n'est pas affichée. Même
// reconnaissance que la saisie côté Go (internal/canvas/freevalue.go).

import { isNumericText } from './style';

export type FreeKind = 'number' | 'date' | 'bool' | 'text';

const amount = /^\(?[-+]?[\d\s  ]*[,.]?\d+\)?\s*€?$|^€\s*[-+]?[\d\s  ]*[,.]?\d+$/;
const exponent = /^[-+]?\d+([,.]\d+)?[eE][-+]?\d+$/;
// Dates courtes de toutes les langues (19/10/2026, 10/19/2026, 19.10.2026,
// 2026/10/19) et ISO.
const date = /^(\d{1,2}[/.]\d{1,2}([/.]\d{4})?|\d{4}[-/]\d{2}[-/]\d{2})( \d{1,2}:\d{2}(:\d{2})?)?$/;
const time = /^\d{1,2}:\d{2}(:\d{2})?$/;

/** Nature d'un texte affiché (saisie ou résultat de formule). */
export function freeKind(text: string): FreeKind {
	const s = text.trim();
	if (!s) return 'text';
	if (isNumericText(s) || exponent.test(s) || (/\d/.test(s) && amount.test(s) && (s.includes('€') || s.startsWith('(')))) return 'number';
	if (date.test(s) || time.test(s)) return 'date';
	if (/^(vrai|faux|true|false)$/i.test(s)) return 'bool';
	return 'text';
}

export interface FreeDisplay {
	text: string;
	kind: FreeKind;
	align: 'left' | 'right' | 'center';
}

const ALIGN: Record<FreeKind, FreeDisplay['align']> = { number: 'right', date: 'right', bool: 'center', text: 'left' };

/**
 * Texte et alignement d'une cellule libre : raw est la saisie, result le
 * résultat d'une formule (affiché à sa place).
 */
export function freeDisplay(raw: string, result?: string): FreeDisplay {
	if (result !== undefined) {
		const kind = freeKind(result);
		return { text: result, kind, align: ALIGN[kind] };
	}
	if (raw.trimStart().startsWith("'")) return { text: raw.trimStart().slice(1), kind: 'text', align: 'left' };
	const kind = freeKind(raw);
	return { text: raw, kind, align: ALIGN[kind] };
}
