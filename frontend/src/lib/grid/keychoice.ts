// Question de la clé : une recherche (REFERENCE, RECHERCHEV, EXISTE…) tapée
// dans un tableau sans clé est refusée par le moteur (canvas.ErrKeyChoice,
// « clé à choisir : … ») tant que l'utilisateur n'a pas dit si elle désigne
// les lignes par leur numéro ou par une colonne, qui deviendrait la clé.

// Colonnes saisies à côté d'un tableau chargé (docs/decisions/0008) : la
// réponse « hand:<colonne> » (ou « hand:# », rang des lignes) donne la clé de
// jointure demandée par canvas.ErrHandKey (« jointure à choisir : … »).
export type KeyChoice = 'index' | 'column' | `hand:${string}`;

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

export interface HandQuestion {
	table: string;
	/** Colonnes chargées qui peuvent servir de clé ; vide : seulement le rang des lignes. */
	columns: string[];
}

const HAND = /jointure à choisir : « (.+?) » est chargé par une requête/;

/** Lit le refus « jointure à choisir » du moteur (première valeur d'une colonne saisie) ; null pour un autre message. */
export function handQuestion(message: string): HandQuestion | null {
	const m = HAND.exec(message);
	if (!m) return null;
	const at = message.indexOf('Colonnes possibles : ');
	const columns = at < 0 ? [] : [...message.slice(at).matchAll(/« ([^»]+) »/g)].map((x) => x[1]);
	return { table: m[1], columns };
}
