// Règles des noms saisis dans l'interface (tableau, feuille, cellule nommée,
// colonne, script, favori), vérifiées avant l'envoi pour que la boîte
// « nom invalide » s'ouvre tout de suite. Ce sont celles du moteur
// (internal/canvas : RenameTable, RenameSheet, SetName, en-têtes de
// colonnes, SaveScript, RenameFavorite) ; un refus du moteur (classeur
// partagé, nom pris ailleurs…) s'affiche de la même façon (refusedName).
import { errorText, lower, msgCause, t, upper } from '#lib/i18n/index.js';

/** Un nom refusé : titre de la boîte et explication (pourquoi, ce qui est permis). */
export interface NameProblem {
	title: string;
	message: string;
}

const taken = (name: string, others: Iterable<string>) => {
	const k = lower(name);
	for (const o of others) if (lower(o.trim()) === k) return o;
	return null;
};

/** Adresse de cellule (F10, AB12, L1C1 en R1C1) : refusée comme nom de tableau. */
export function a1Name(name: string): boolean {
	return /^(?:[a-z]{1,4}[1-9][0-9]{0,8}|r[1-9][0-9]*c[1-9][0-9]*)$/i.test(name);
}

/** Pourquoi un identifiant (lettre ou _, puis lettres, chiffres, _ et extra) n'en est pas un ; vide s'il l'est. */
function identifierFault(name: string, extra = ''): string {
	const chars = [...name];
	if (/^\p{Nd}$/u.test(chars[0])) return t('shell.names.fault.digit');
	const ok = new RegExp(`^[\\p{L}\\p{Nd}_${extra}]$`, 'u');
	for (const [i, ch] of chars.entries()) {
		if (i === 0 && !/^[\p{L}_]$/u.test(ch)) return t('shell.names.fault.start', { char: ch });
		if (/^\s$/u.test(ch)) return t('shell.names.fault.space');
		if (!ok.test(ch)) return t('shell.names.fault.char', { char: ch });
	}
	return '';
}

/**
 * Nom de tableau (zone de nom, onglet SQL) : identifiant, pas une adresse
 * de cellule, libre parmi others (sans tenir compte des majuscules).
 */
export function tableNameProblem(raw: string, others: Iterable<string> = []): NameProblem | null {
	const name = raw.trim();
	const title = t('shell.names.table.invalid');
	const rule = t('shell.names.table.rule');
	if (!name) return { title, message: t('shell.names.table.empty', { rule }) };
	const fault = identifierFault(name);
	if (fault) return { title, message: t('shell.names.fault.message', { name, fault, rule }) };
	if (a1Name(name)) return { title, message: t('shell.names.table.address', { name }) };
	const other = taken(name, others);
	if (other) return { title: t('shell.names.table.taken'), message: t('shell.names.table.takenMessage', { name: other }) };
	return null;
}

/** Nom de feuille (onglet) : non vide et libre parmi others (sans tenir compte des majuscules). */
export function sheetNameProblem(raw: string, others: Iterable<string> = []): NameProblem | null {
	const name = raw.trim();
	if (!name) return { title: t('shell.names.sheet.invalid'), message: t('shell.names.sheet.empty') };
	const other = taken(name, others);
	if (other) return { title: t('shell.names.sheet.taken'), message: t('shell.names.sheet.takenMessage', { name: other }) };
	return null;
}

/** Nom d'une cellule libre (zone de nom) ; vide : retire le nom, permis. */
export function cellNameProblem(raw: string): NameProblem | null {
	const name = raw.trim();
	if (!name) return null;
	const title = t('shell.names.cell.invalid');
	const rule = t('shell.names.cell.rule');
	const fault = identifierFault(name, '.');
	if (fault) return { title, message: t('shell.names.fault.message', { name, fault, rule }) };
	if (['VRAI', 'FAUX', 'TRUE', 'FALSE'].includes(name.toUpperCase())) return { title, message: t('shell.names.cell.reserved', { name, rule }) };
	if (a1Name(name) || /^cell_[a-z]{1,4}[1-9][0-9]{0,8}$/i.test(name)) return { title, message: t('shell.names.cell.address', { name, rule }) };
	return null;
}

/** Nom de colonne (en-tête d'un tableau) : libre parmi les autres colonnes du tableau. Vide : « Colonne n », permis. */
export function columnNameProblem(raw: string, others: Iterable<string> = [], table = ''): NameProblem | null {
	const name = raw.trim();
	if (!name) return null;
	const other = taken(name, others);
	if (other)
		return {
			title: t('shell.names.column.taken'),
			message: table ? t('shell.names.column.takenMessage', { table, name: other, ref: `${table}[${name}]` }) : t('shell.names.column.takenMessageNoTable', { name: other, ref: `${t('shell.names.column.tableWord')}[${name}]` })
		};
	return null;
}

/** Nom de script : non vide et libre parmi others. */
export function scriptNameProblem(raw: string, others: Iterable<string> = []): NameProblem | null {
	const name = raw.trim();
	if (!name) return { title: t('shell.names.script.invalid'), message: t('shell.names.script.empty') };
	for (const o of others) if (o.trim() === name) return { title: t('shell.names.script.taken'), message: t('shell.names.script.takenMessage', { name: o }) };
	return null;
}

/** Nom de requête favorite : non vide. */
export function favoriteNameProblem(raw: string): NameProblem | null {
	if (raw.trim()) return null;
	return { title: t('shell.names.favorite.invalid'), message: t('shell.names.favorite.empty') };
}

/** Identifiants des messages du moteur (internal/msg) qui disent qu'un nom est pris. */
const TAKEN_IDS = ['table.nameTaken', 'sheet.nameTaken'];

/**
 * Refus du moteur ou du serveur (exception) mis en forme pour la boîte :
 * le préfixe technique (« tableau : », « feuille : »…) retiré.
 */
export function refusedName(title: string, e: unknown): NameProblem {
	let m = errorText(e).trim();
	// Préfixes et formes des messages français du moteur (les messages traduits n'en ont pas besoin).
	m = m.replace(/^(?:tableau|feuille|onglet|script|requête|favori)\s*:\s*/i, '');
	m = m.replace(/^nom « (.*) » refusé : /s, '« $1 » : ');
	if (!m) return { title, message: t('shell.names.refused') };
	m = upper(m.charAt(0)) + m.slice(1);
	// Nom pris : par l'identifiant du message du moteur ; les messages pas encore
	// au catalogue (lot 3) se reconnaissent encore à leur texte français.
	const isTaken = TAKEN_IDS.includes(msgCause(e)?.id ?? '') || /existe déjà|déjà utilisé/.test(m);
	if (isTaken && !/choisissez/i.test(m)) m += t('shell.names.takenAdvice');
	return { title, message: m };
}
