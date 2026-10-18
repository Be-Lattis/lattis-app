// Écran de récupération : les sessions interrompues regroupées par classeur.

/** Session interrompue (archive.Orphan). */
export interface Session {
	dir: string;
	source: string;
	/** Date de la dernière modification (ISO 8601). */
	modified: unknown;
	/** Identifiant aléatoire du classeur (_lattis_info, clé id) ; vide : illisible. */
	workbook?: string;
}

/** Sessions d'un même classeur : la plus récente, puis les plus anciennes. */
export interface SessionGroup<T extends Session = Session> {
	key: string;
	latest: T;
	older: T[];
}

const time = (s: Session) => {
	const t = Date.parse(String(s.modified));
	return Number.isNaN(t) ? 0 : t;
};

/**
 * Regroupe les sessions par classeur, d'après son identifiant et non son
 * nom : deux « Classeur sans titre » distincts restent deux entrées, deux
 * copies du même fichier n'en font qu'une. Une session sans identifiant
 * forme son propre groupe. Groupes et sessions du plus récent au plus ancien.
 */
export function groupSessions<T extends Session>(sessions: readonly T[]): SessionGroup<T>[] {
	const sorted = [...sessions].sort((a, b) => time(b) - time(a) || a.dir.localeCompare(b.dir));
	const groups = new Map<string, SessionGroup<T>>();
	for (const s of sorted) {
		const key = s.workbook ? `id:${s.workbook}` : `dir:${s.dir}`;
		const g = groups.get(key);
		if (g) g.older.push(s);
		else groups.set(key, { key, latest: s, older: [] });
	}
	return [...groups.values()];
}

/** Nom affiché d'une session : le fichier, ou « Classeur sans titre ». */
export function sessionName(s: Session): string {
	return s.source ? (s.source.split(/[\\/]/).pop() ?? s.source) : 'Classeur sans titre';
}

/** « + 1 version plus ancienne », « + 2 versions plus anciennes ». */
export function olderLabel(n: number): string {
	return n === 1 ? '+ 1 version plus ancienne' : `+ ${n} versions plus anciennes`;
}
