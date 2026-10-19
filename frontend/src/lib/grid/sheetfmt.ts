// Fusions, lignes et colonnes masquées, validation par liste : lues dans
// les couches de style (voir internal/canvas/cellformat.go).

import type { StyleLayer } from './style';

/** Cellules fusionnées : cellule en haut à gauche, lignes et colonnes couvertes. */
export interface MergeRange {
	row: number;
	col: number;
	rows: number;
	cols: number;
}

/** Fusions d'après les styles des cellules libres (clé « ligne,colonne »). */
export function mergeList(styles: Iterable<[string, StyleLayer]>): MergeRange[] {
	const out: MergeRange[] = [];
	for (const [k, s] of styles) {
		const rows = s.mr ?? 0;
		const cols = s.mc ?? 0;
		if (rows < 1 || cols < 1 || rows * cols < 2) continue;
		const [row, col] = k.split(',').map(Number);
		out.push({ row, col, rows, cols });
	}
	return out;
}

/** Fusion qui couvre la cellule (r, c), s'il y en a une. */
export function mergeAt(list: MergeRange[], r: number, c: number): MergeRange | undefined {
	for (const g of list) if (r >= g.row && r < g.row + g.rows && c >= g.col && c < g.col + g.cols) return g;
	return undefined;
}

/** Lignes (ou colonnes) masquées d'après les styles de lignes (ou de colonnes). */
export function hiddenLines(styles: Map<number, StyleLayer>): Set<number> {
	const out = new Set<number>();
	for (const [i, s] of styles) if (s.hd) out.add(i);
	return out;
}

/**
 * Première ligne visible à partir de i dans la direction d (+1 ou -1 ; 0 :
 * vers le bas puis vers le haut), bornée à [0, count[.
 */
export function skipHidden(i: number, d: number, hidden: Set<number>, count: number): number {
	if (!hidden.has(i)) return i;
	for (const dir of d ? [d, -d] : [1, -1]) {
		let k = i;
		while (k >= 0 && k < count && hidden.has(k)) k += dir;
		if (k >= 0 && k < count) return k;
	}
	return i;
}

/**
 * Validation par liste (liste déroulante) : la saisie est-elle permise ?
 * Une cellule vide l'est toujours. La comparaison ignore la casse et les
 * espaces autour ; la valeur retenue est celle de la liste.
 */
export function validate(list: string[] | undefined, text: string): { ok: true; value: string } | { ok: false; message: string } {
	if (!list?.length || text.trim() === '') return { ok: true, value: text };
	const t = text.trim().toLocaleLowerCase('fr');
	const hit = list.find((v) => v.trim().toLocaleLowerCase('fr') === t);
	if (hit !== undefined) return { ok: true, value: hit };
	const shown = list.length > 8 ? list.slice(0, 8).join(', ') + '…' : list.join(', ');
	return { ok: false, message: `« ${text.trim()} » n'est pas permis ici : choisissez dans la liste (${shown}), Alt+↓ pour l'ouvrir` };
}

/** Liste de valeurs saisie dans le dialogue (« a; b; c » ou une valeur par ligne). */
export function parseList(text: string): string[] {
	const seen = new Set<string>();
	const out: string[] = [];
	for (const v of text.split(/[;\n]/)) {
		const t = v.trim();
		if (t && !seen.has(t.toLocaleLowerCase('fr'))) {
			seen.add(t.toLocaleLowerCase('fr'));
			out.push(t);
		}
	}
	return out;
}

// --- Validation tirée d'une plage --------------------------------------------

/** Liste d'une validation tirée d'une plage, relue sur le serveur. */
export interface DvList {
	values: string[];
	error?: string;
}

/** Source d'une validation tirée d'une plage ({ src: "Clients[Ville]" }), sinon undefined. */
export function dvSource(dv: unknown): string | undefined {
	if (!dv || typeof dv !== 'object' || Array.isArray(dv)) return undefined;
	const s = (dv as { src?: unknown }).src;
	return typeof s === 'string' && s.trim() ? s : undefined;
}

/** Source tapée dans le dialogue : sans le « = » de tête ni les espaces. */
export function cleanSource(text: string): string {
	return text.trim().replace(/^=\s*/, '').trim();
}

/** État initial du dialogue Validation des données d'après la propriété dv de la cellule active. */
export function dvDialog(dv: unknown): { mode: 'list' | 'range'; text: string; src: string } {
	const src = dvSource(dv);
	if (src) return { mode: 'range', text: '', src };
	return { mode: 'list', text: Array.isArray(dv) ? dv.join('\n') : '', src: '' };
}

/**
 * Propriété dv à poser d'après le dialogue (liste saisie ou plage), ou le
 * message qui dit ce qui manque.
 */
export function dvPatch(mode: 'list' | 'range', text: string, src: string): { dv: string[] | { src: string } } | { error: string } {
	if (mode === 'range') {
		const s = cleanSource(src);
		if (!s) return { error: 'Indiquez la plage source : Tableau[Colonne], Cell[A1:A10] ou Feuille2!Cell[A1:A10] (« Sélectionner » pour la désigner)' };
		return { dv: { src: s } };
	}
	const list = parseList(text);
	if (!list.length) return { error: 'Indiquez au moins une valeur (une par ligne, ou séparées par « ; »)' };
	return { dv: list };
}

/** Aperçu des valeurs d'une plage source, dans le dialogue. */
export function dvPreview(l: DvList | null | undefined): string {
	if (!l) return '';
	if (l.error) return `Source introuvable : ${l.error}`;
	if (!l.values.length) return 'La plage ne contient aucune valeur';
	const shown = l.values.length > 12 ? l.values.slice(0, 12).join(', ') + '…' : l.values.join(', ');
	return `${l.values.length} valeur${l.values.length > 1 ? 's' : ''} : ${shown}`;
}

/**
 * Saisie vérifiée par une validation tirée d'une plage : la liste relue
 * (undefined : pas encore lue, la saisie passe), une source en erreur ou
 * vide refuse tout, sinon comme validate.
 */
export function validateSource(src: string, l: DvList | undefined, text: string): { ok: true; value: string } | { ok: false; message: string } {
	if (!l || text.trim() === '') return { ok: true, value: text };
	if (l.error) return { ok: false, message: `« ${text.trim()} » ne peut pas être vérifié : la source de la liste (${src}) ne se lit pas (${l.error})` };
	if (!l.values.length) return { ok: false, message: `« ${text.trim()} » n'est pas permis ici : la plage de la liste (${src}) est vide` };
	return validate(l.values, text);
}

/** Lettres d'une colonne de la feuille (0 : A). */
function colLetters(col: number): string {
	let s = '';
	for (let n = col + 1; n > 0; n = Math.floor((n - 1) / 26)) s = String.fromCharCode(65 + ((n - 1) % 26)) + s;
	return s;
}

/** Ce que vise une validation, pour le titre du dialogue. */
export interface DvTarget {
	/** Sélection (à partir de 0). */
	sel: { r0: number; c0: number; r1: number; c1: number };
	/** Lignes ou colonnes entières de la feuille. */
	whole?: 'rows' | 'cols' | null;
	/** Tableau de la cellule active : en-tête (row), dernière ligne, première colonne, noms des colonnes. */
	table?: { name: string; row: number; last: number; col: number; columns: string[] } | null;
	sheet: string;
}

/**
 * Cible d'une validation, dite dans le titre du dialogue : « colonne
 * « Ville » du tableau Clients », « cellules B2:B10 (Feuille1) », « cellule
 * C3 (Feuille1) », « colonnes B:C (Feuille1) »…
 */
export function dvTarget(t: DvTarget): string {
	const { r0, c0, r1, c1 } = t.sel;
	const on = t.sheet ? ` (${t.sheet})` : '';
	if (t.whole === 'cols') return c0 === c1 ? `colonne ${colLetters(c0)}${on}` : `colonnes ${colLetters(c0)}:${colLetters(c1)}${on}`;
	if (t.whole === 'rows') return r0 === r1 ? `ligne ${r0 + 1}${on}` : `lignes ${r0 + 1}:${r1 + 1}${on}`;
	const tb = t.table;
	if (tb) {
		// Une règle posée dans un tableau vaut pour ses colonnes entières si la
		// sélection en couvre toutes les lignes.
		const a = Math.max(c0, tb.col) - tb.col;
		const b = Math.min(c1, tb.col + tb.columns.length - 1) - tb.col;
		const all = r0 <= tb.row + 1 && r1 >= tb.last;
		if (all && a >= 0 && b >= a) {
			return a === b ? `colonne « ${tb.columns[a]} » du tableau ${tb.name}` : `colonnes « ${tb.columns[a]} » à « ${tb.columns[b]} » du tableau ${tb.name}`;
		}
	}
	const addr = (r: number, c: number) => colLetters(c) + (r + 1);
	if (r0 === r1 && c0 === c1) return `cellule ${addr(r0, c0)}${on}`;
	const what = `cellules ${addr(r0, c0)}:${addr(r1, c1)}`;
	return tb ? `${what} du tableau ${tb.name}` : what + on;
}
