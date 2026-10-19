// Règles des noms saisis dans l'interface (tableau, feuille, cellule nommée,
// colonne, script, favori), vérifiées avant l'envoi pour que la boîte
// « nom invalide » s'ouvre tout de suite. Ce sont celles du moteur
// (internal/canvas : RenameTable, RenameSheet, SetName, en-têtes de
// colonnes, SaveScript, RenameFavorite) ; un refus du moteur (classeur
// partagé, nom pris ailleurs…) s'affiche de la même façon (refusedName).

/** Un nom refusé : titre de la boîte et explication (pourquoi, ce qui est permis). */
export interface NameProblem {
	title: string;
	message: string;
}

const lower = (s: string) => s.toLocaleLowerCase('fr');
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
	if (/^\p{Nd}$/u.test(chars[0])) return 'il commence par un chiffre';
	const ok = new RegExp(`^[\\p{L}\\p{Nd}_${extra}]$`, 'u');
	for (const [i, ch] of chars.entries()) {
		if (i === 0 && !/^[\p{L}_]$/u.test(ch)) return `il commence par « ${ch} »`;
		if (/^\s$/u.test(ch)) return 'il contient une espace';
		if (!ok.test(ch)) return `le caractère « ${ch} » n'est pas permis`;
	}
	return '';
}

const TABLE_RULE = "Un nom de tableau commence par une lettre (ou _) et ne contient que des lettres, des chiffres et _, sans espace : il s'écrit tel quel dans les formules (exemple : Ventes_2024).";

/**
 * Nom de tableau (zone de nom, onglet SQL) : identifiant, pas une adresse
 * de cellule, libre parmi others (sans tenir compte des majuscules).
 */
export function tableNameProblem(raw: string, others: Iterable<string> = []): NameProblem | null {
	const name = raw.trim();
	const title = 'Nom de tableau invalide';
	if (!name) return { title, message: `Le nom du tableau ne peut pas être vide.\n\n${TABLE_RULE}` };
	const fault = identifierFault(name);
	if (fault) return { title, message: `« ${name} » : ${fault}.\n\n${TABLE_RULE}` };
	if (a1Name(name)) return { title, message: `« ${name} » est une adresse de cellule : la zone de nom et les formules ne sauraient pas lequel des deux on veut dire.\n\nEssayez par exemple « ${name}_ » ou « T_${name} ».` };
	const other = taken(name, others);
	if (other) return { title: 'Nom de tableau déjà pris', message: `Un autre tableau s'appelle déjà « ${other} » (les majuscules ne comptent pas).\n\nChoisissez un nom qui n'est pas encore utilisé dans le classeur.` };
	return null;
}

/** Nom de feuille (onglet) : non vide et libre parmi others (sans tenir compte des majuscules). */
export function sheetNameProblem(raw: string, others: Iterable<string> = []): NameProblem | null {
	const name = raw.trim();
	if (!name) return { title: 'Nom de feuille invalide', message: "Le nom de la feuille ne peut pas être vide.\n\nTapez un nom, ou Échap pour garder l'ancien." };
	const other = taken(name, others);
	if (other) return { title: 'Nom de feuille déjà pris', message: `Une autre feuille s'appelle déjà « ${other} » (les majuscules ne comptent pas).\n\nChoisissez un nom qui n'est pas encore utilisé dans le classeur.` };
	return null;
}

const CELL_RULE = 'Un nom de cellule commence par une lettre (ou _) et ne contient que des lettres, des chiffres, _ et des points, sans espace (exemple : Taux_TVA).';

/** Nom d'une cellule libre (zone de nom) ; vide : retire le nom, permis. */
export function cellNameProblem(raw: string): NameProblem | null {
	const name = raw.trim();
	if (!name) return null;
	const title = 'Nom de cellule invalide';
	const fault = identifierFault(name, '.');
	if (fault) return { title, message: `« ${name} » : ${fault}.\n\n${CELL_RULE}` };
	if (['VRAI', 'FAUX', 'TRUE', 'FALSE'].includes(name.toUpperCase())) return { title, message: `« ${name} » est un mot réservé des formules.\n\n${CELL_RULE}` };
	if (a1Name(name) || /^cell_[a-z]{1,4}[1-9][0-9]{0,8}$/i.test(name)) return { title, message: `« ${name} » est une adresse de cellule.\n\n${CELL_RULE}` };
	return null;
}

/** Nom de colonne (en-tête d'un tableau) : libre parmi les autres colonnes du tableau. Vide : « Colonne n », permis. */
export function columnNameProblem(raw: string, others: Iterable<string> = [], table = ''): NameProblem | null {
	const name = raw.trim();
	if (!name) return null;
	const other = taken(name, others);
	if (other) return { title: 'Nom de colonne déjà pris', message: `Le tableau${table ? ` ${table}` : ''} a déjà une colonne « ${other} » (les majuscules ne comptent pas) : ${table || 'Tableau'}[${name}] serait ambigu dans les formules.\n\nChoisissez un autre nom.` };
	return null;
}

/** Nom de script : non vide et libre parmi others. */
export function scriptNameProblem(raw: string, others: Iterable<string> = []): NameProblem | null {
	const name = raw.trim();
	if (!name) return { title: 'Nom de script invalide', message: "Le nom du script ne peut pas être vide.\n\nTapez un nom, ou Échap pour garder l'ancien." };
	for (const o of others) if (o.trim() === name) return { title: 'Nom de script déjà pris', message: `Un autre script s'appelle déjà « ${o} ».\n\nChoisissez un autre nom.` };
	return null;
}

/** Nom de requête favorite : non vide. */
export function favoriteNameProblem(raw: string): NameProblem | null {
	if (raw.trim()) return null;
	return { title: 'Nom de favori invalide', message: "Le nom de la requête favorite ne peut pas être vide.\n\nTapez un nom, ou Échap pour garder l'ancien." };
}

/**
 * Refus du moteur ou du serveur (exception) mis en forme pour la boîte :
 * le préfixe technique (« tableau : », « feuille : »…) retiré.
 */
export function refusedName(title: string, e: unknown): NameProblem {
	let m = String(e instanceof Error ? e.message : e).trim();
	m = m.replace(/^(?:tableau|feuille|onglet|script|requête|favori)\s*:\s*/i, '');
	m = m.replace(/^nom « (.*) » refusé : /s, '« $1 » : ');
	if (!m) return { title, message: 'Le nom a été refusé.' };
	m = m.charAt(0).toLocaleUpperCase('fr') + m.slice(1);
	if (/existe déjà|déjà utilisé/.test(m) && !/choisissez/i.test(m)) m += ".\n\nChoisissez un nom qui n'est pas encore utilisé dans le classeur.";
	return { title, message: m };
}
