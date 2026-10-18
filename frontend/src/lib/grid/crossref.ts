// Pointage sur une autre feuille pendant la saisie d'une formule (comme
// Excel) : un clic sur un onglet garde la saisie (texte, curseur, cellule et
// feuille d'origine) ; sur l'autre feuille, la formule reste dans la barre de
// formule et un clic y insère la référence au curseur ; Entrée la valide dans
// la cellule d'origine, Échap l'abandonne, et l'on revient à la feuille
// d'origine dans les deux cas.

/** Saisie de formule gardée pendant le passage sur une autre feuille. */
export interface FormulaHold {
	/** Feuille, cellule et éditeur (cellule ou barre de formule) d'origine. */
	sheet: number;
	row: number;
	col: number;
	where: 'cell' | 'fx';
	text: string;
	/** Curseur, ou sélection dans le texte. */
	start: number;
	end: number;
	/** Fin demandée (Entrée : valider, Échap : abandonner), le temps de revenir à la feuille d'origine. */
	finish?: 'commit' | 'cancel';
}

/** Saisie en cours d'une formule (texte commençant par « = ») ? */
export const formulaTyping = (text: string) => text.trimStart().startsWith('=');

/**
 * Garde la saisie d'une formule pour passer sur une autre feuille ; null si
 * le texte n'est pas une formule. Le curseur est ramené dans le texte.
 */
export function holdFormula(at: { sheet: number; row: number; col: number; where: 'cell' | 'fx' }, text: string, start: number | null, end: number | null): FormulaHold | null {
	if (!formulaTyping(text)) return null;
	const clamp = (n: number | null) => Math.max(0, Math.min(n ?? text.length, text.length));
	const a = clamp(start);
	const b = Math.max(a, clamp(end));
	return { ...at, text, start: a, end: b };
}

/** Met à jour la saisie gardée (texte et curseur de la barre de formule sur l'autre feuille). */
export function updateHold(h: FormulaHold, text: string, start: number | null, end: number | null): FormulaHold {
	return holdFormula(h, text, start, end) ?? { ...h, text, start: text.length, end: text.length };
}

/** « Feuille! » ou « 'Ma feuille'! » devant une référence (comme internal/formula). */
export function sheetPrefix(sheet: string): string {
	if (!sheet) return '';
	const name = /^[\p{L}_][\p{L}\p{N}_]*$/u.test(sheet) ? sheet : "'" + sheet.replaceAll("'", "''") + "'";
	return name + '!';
}

/**
 * Référence pointée sur une autre feuille que celle de la formule : les
 * cellules libres prennent le nom de la feuille (Clients!Cell[B2]) ; les
 * références de tableau restent telles quelles, les noms de tableaux étant
 * uniques dans le classeur (Clients[Ville]).
 */
export function qualifyRef(ref: string, sheet: string): string {
	if (/^cell\[/i.test(ref) || /^\$?[A-Za-z]{1,3}\$?\d+(:\$?[A-Za-z]{1,3}\$?\d+)?$/.test(ref)) return sheetPrefix(sheet) + ref;
	return ref;
}

/** Dernière référence insérée au clic : un autre clic la remplace (comme Excel). */
export interface LastRef {
	start: number;
	end: number;
	value: string;
}

/**
 * Insère une référence au curseur d'une formule (sélection remplacée) ;
 * null si le curseur n'est pas à un endroit où une référence peut venir
 * (après « = », « ( », un séparateur ou un opérateur), sauf s'il suit la
 * référence insérée au clic précédent, qui est alors remplacée.
 */
export function insertRefAt(value: string, selStart: number, selEnd: number, ref: string, last?: LastRef | null): { value: string; start: number; end: number } | null {
	const pos = Math.max(0, Math.min(selStart, value.length));
	const to = Math.max(pos, Math.min(selEnd, value.length));
	let start = pos;
	if (last && last.end === pos && last.value === value && pos === to) start = last.start;
	else if (!/[=(;,+*/^&<>:-]$/.test(value.slice(0, pos).trimEnd())) return null;
	const next = value.slice(0, start) + ref + value.slice(to);
	return { value: next, start, end: start + ref.length };
}
