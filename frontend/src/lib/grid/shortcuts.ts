// Raccourcis clavier d'Excel (grille et éditeur de cellule), indépendants du
// clavier (AZERTY ou QWERTY) quand c'est possible.

import { trimTrailingBreaks } from './textwrap';

export type Shortcut =
	| 'fillDown' // Ctrl+D
	| 'fillRight' // Ctrl+R
	| 'today' // Ctrl+;
	| 'now' // Ctrl+Maj+:
	| 'autoSum' // Alt+=
	| 'formatCells' // Ctrl+1
	| 'selectRow' // Maj+Espace
	| 'toggleFilter' // Ctrl+Maj+L
	| 'nextSheet' // Ctrl+Pg.Suiv
	| 'prevSheet' // Ctrl+Pg.Préc
	| 'nextChoice' // Ctrl+Alt+Pg.Suiv : valeur suivante de l'onglet à choix
	| 'prevChoice' // Ctrl+Alt+Pg.Préc : valeur précédente
	| 'repeat' // F4
	| 'insertSheet' // Maj+F11
	| 'endMode' // Fin (puis flèche)
	| 'refresh' // F5 : actualise la requête du tableau, sinon recalcule
	| 'refreshAll' // Ctrl+F5, Ctrl+Alt+F5 ou Maj+F5 : actualise tout (requêtes et recalcul)
	| 'formatTable' // Ctrl+L ou Ctrl+T : mettre sous forme de tableau
	| 'strike' // Ctrl+5 : barré
	| 'hideRows' // Ctrl+9 : masquer les lignes
	| 'hideCols' // Ctrl+0 : masquer les colonnes
	| 'note' // Maj+F2 : nouvelle note (ou la modifier)
	| 'insertFunction' // Maj+F3 : insérer une fonction
	| 'chart'; // Alt+F1 ou F11 : graphique du tableau actif

type Keys = Pick<KeyboardEvent, 'key' | 'code' | 'ctrlKey' | 'metaKey' | 'shiftKey' | 'altKey'>;

/** Raccourci d'une touche, ou null. */
export function shortcutOf(e: Keys): Shortcut | null {
	const ctrl = e.ctrlKey || e.metaKey;
	const k = e.key;
	const lower = k.length === 1 ? k.toLowerCase() : k;
	if (k === 'F5' && (!e.altKey || ctrl)) return ctrl || e.shiftKey ? 'refreshAll' : 'refresh';
	if (ctrl && !e.altKey) {
		if (k === 'PageDown') return 'nextSheet';
		if (k === 'PageUp') return 'prevSheet';
		if (!e.shiftKey && lower === 'd') return 'fillDown';
		if (!e.shiftKey && lower === 'r') return 'fillRight';
		if (e.shiftKey && lower === 'l') return 'toggleFilter';
		// Heure : Ctrl+Maj+: (AZERTY : touche « : », qui donne « / » avec Maj ;
		// QWERTY : Ctrl+Maj+; donne « : »).
		if (k === ':' || (e.shiftKey && (e.code === 'Period' || e.code === 'Semicolon'))) return 'now';
		if (!e.shiftKey && k === ';') return 'today';
		if (!e.shiftKey && (k === '1' || k === '&' || e.code === 'Digit1' || e.code === 'Numpad1')) return 'formatCells';
		if (!e.shiftKey && (lower === 'l' || lower === 't')) return 'formatTable';
		// Rangée des chiffres (AZERTY : « ( », « ç », « à » sans Maj).
		if (!e.shiftKey && (k === '5' || e.code === 'Digit5')) return 'strike';
		if (!e.shiftKey && (k === '9' || e.code === 'Digit9')) return 'hideRows';
		if (!e.shiftKey && (k === '0' || e.code === 'Digit0')) return 'hideCols';
		return null;
	}
	if (e.altKey && !ctrl && (k === '=' || e.code === 'Equal')) return 'autoSum';
	if (e.altKey && !ctrl && !e.shiftKey && k === 'F1') return 'chart';
	// Équivalent de Ctrl+Pg.Suiv / Pg.Préc, que le navigateur garde pour
	// changer d'onglet (version web).
	if (e.altKey && !ctrl && !e.shiftKey && k === 'PageDown') return 'nextSheet';
	if (e.altKey && !ctrl && !e.shiftKey && k === 'PageUp') return 'prevSheet';
	// Onglet à choix : valeur suivante ou précédente (Ctrl+Pg.Suiv reste le
	// changement d'onglet).
	if (e.altKey && ctrl && !e.shiftKey && k === 'PageDown') return 'nextChoice';
	if (e.altKey && ctrl && !e.shiftKey && k === 'PageUp') return 'prevChoice';
	if (ctrl || e.altKey) return null;
	if (e.shiftKey && (k === ' ' || e.code === 'Space')) return 'selectRow';
	if (e.shiftKey && k === 'F11') return 'insertSheet';
	if (!e.shiftKey && k === 'F11') return 'chart';
	if (e.shiftKey && k === 'F2') return 'note';
	if (e.shiftKey && k === 'F3') return 'insertFunction';
	if (!e.shiftKey && k === 'F4') return 'repeat';
	if (!e.shiftKey && k === 'End') return 'endMode';
	return null;
}

const MOVES = new Set(['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'PageUp', 'PageDown', 'Home', 'End', 'Tab', 'Enter', 'Escape']);

/**
 * Touche permise sur une feuille protégée (elle ne modifie rien) : se
 * déplacer et sélectionner, Rechercher (Ctrl+F), copier, annuler et rétablir
 * (Ctrl+Z, Ctrl+Y, comme le ruban), recalculer (F9). Remplacer, la saisie et
 * le reste sont ignorés.
 */
export function readKey(e: Keys): boolean {
	const ctrl = e.ctrlKey || e.metaKey;
	const k = e.key.length === 1 ? e.key.toLowerCase() : e.key;
	if (e.altKey) return false;
	if (ctrl) return MOVES.has(k) || ['f', 'z', 'y', 'c', 'a', ' '].includes(k);
	return MOVES.has(k) || k === 'F9' || (e.shiftKey && k === ' ');
}

/**
 * Touche qui rechargerait la page de l'application dans la fenêtre (WebView2)
 * ou le navigateur : F5 (avec ou sans modificateur), Ctrl+R, Ctrl+Maj+R.
 */
export function isReloadKey(e: Keys): boolean {
	if (e.key === 'F5' || e.key === 'BrowserRefresh') return true;
	return (e.ctrlKey || e.metaKey) && !e.altKey && (e.key === 'r' || e.key === 'R' || e.code === 'KeyR');
}

const pad = (n: number) => String(n).padStart(2, '0');

/** Date du jour comme on la saisit (jj/mm/aaaa). */
export function todayText(d = new Date()): string {
	return `${pad(d.getDate())}/${pad(d.getMonth() + 1)}/${d.getFullYear()}`;
}

/** Heure du jour (hh:mm), comme Ctrl+Maj+: d'Excel. */
export function timeText(d = new Date()): string {
	return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export interface Range {
	r0: number;
	c0: number;
	r1: number;
	c1: number;
}

/**
 * Recopie Ctrl+D (vers le bas) ou Ctrl+R (vers la droite), comme Excel : la
 * première ligne (colonne) de la sélection est recopiée dans les suivantes ;
 * une seule ligne (colonne) sélectionnée reprend celle du dessus (de gauche).
 * Renvoie la source et la zone écrite, ou null (rien au-dessus).
 */
export function copyFillPlan(sel: Range, down: boolean): { src: Range; dst: Range } | null {
	if (down) {
		const one = sel.r0 === sel.r1;
		const from = one ? sel.r0 - 1 : sel.r0;
		if (from < 0) return null;
		return { src: { ...sel, r0: from, r1: from }, dst: { ...sel, r0: from + 1 } };
	}
	const one = sel.c0 === sel.c1;
	const from = one ? sel.c0 - 1 : sel.c0;
	if (from < 0) return null;
	return { src: { ...sel, c0: from, c1: from }, dst: { ...sel, c0: from + 1 } };
}

/** Bloc à coller pour la recopie : val(r, c) donne le texte d'une cellule source. */
export function copyFillBlock(plan: { src: Range; dst: Range }, down: boolean, val: (r: number, c: number) => string): string[][] {
	const { src, dst } = plan;
	const rows = dst.r1 - dst.r0 + 1;
	const cols = dst.c1 - dst.c0 + 1;
	return Array.from({ length: rows }, (_, i) => Array.from({ length: cols }, (_, j) => (down ? val(src.r0, dst.c0 + j) : val(dst.r0 + i, src.c0))));
}

/** Bloc rempli d'un même texte (Ctrl+Entrée : valider dans toute la sélection). */
export function fillBlock(sel: Range, text: string): string[][] {
	return Array.from({ length: sel.r1 - sel.r0 + 1 }, () => Array.from({ length: sel.c1 - sel.c0 + 1 }, () => text));
}

/**
 * Lignes d'un texte à plusieurs lignes (Alt+Entrée) qui tiennent dans une
 * cellule de max lignes ; la dernière montrée finit par « … » s'il en reste.
 * Les sauts de ligne finaux ne comptent pas (trimTrailingBreaks).
 */
export function cellLines(s: string, max: number): string[] {
	const lines = trimTrailingBreaks(s).split(/\r?\n/);
	const n = Math.max(1, max);
	if (lines.length <= n) return lines;
	const out = lines.slice(0, n);
	out[n - 1] += ' …';
	return out;
}

/** Insère text à la place de la sélection [start, end) de value. */
export function insertAt(value: string, start: number, end: number, text: string): { value: string; caret: number } {
	return { value: value.slice(0, start) + text + value.slice(end), caret: start + text.length };
}
