// Onglet SQL : « ALTER TABLE ancien RENAME TO nouveau » renomme un tableau du
// classeur par la même commande que la grille (références réécrites,
// annulable), au lieu de toucher la table SQLite directement.

// Nom SQL : "entre guillemets", [entre crochets], `entre accents graves`, ou simple.
const NAME = '(?:"((?:[^"]|"")+)"|\\[([^\\]]+)\\]|`([^`]+)`|([\\p{L}_][\\p{L}\\p{N}_]*))';
const ALTER = new RegExp(`^\\s*ALTER\\s+TABLE\\s+${NAME}\\s+RENAME\\s+TO\\s+${NAME}\\s*;?\\s*$`, 'iu');

const pick = (m: RegExpExecArray, i: number) => (m[i] !== undefined ? m[i].replace(/""/g, '"') : (m[i + 1] ?? m[i + 2] ?? m[i + 3]));

/** Renommage d'un tableau demandé en SQL : { from, to }, ou null. */
export function alterRename(sql: string): { from: string; to: string } | null {
	const m = ALTER.exec(sql);
	if (!m) return null;
	return { from: pick(m, 1), to: pick(m, 5) };
}
