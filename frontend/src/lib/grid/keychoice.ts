// Question de la clé : une recherche (REFERENCE, RECHERCHEV, EXISTE…) tapée
// dans un tableau sans clé est refusée par le moteur (canvas.ErrKeyChoice,
// « clé à choisir : … ») tant que l'utilisateur n'a pas dit si elle désigne
// les lignes par leur numéro ou par une colonne, qui deviendrait la clé.

export type KeyChoice = 'index' | 'column';

export interface KeyQuestion {
	table: string;
	column: string;
	/** Pourquoi la colonne ne peut pas devenir la clé (vide : elle le peut). */
	why: string;
}

const RE = /clé à choisir : « (.+?) » n'a pas de clé ;.*?par la colonne « (.+?) », qui deviendrait.*?(?: ; « .+? » ne peut pas servir de clé : (.+))?$/s;

/** Lit le refus « clé à choisir » du moteur ; null pour un autre message. */
export function keyQuestion(message: string): KeyQuestion | null {
	const m = RE.exec(message.trim());
	if (!m) return null;
	return { table: m[1], column: m[2], why: m[3] ?? '' };
}
