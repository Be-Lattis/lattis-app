// Vue gardée en vie (onglet SQL) : montée à sa première ouverture, seulement
// cachée quand on passe à une autre vue ou une autre feuille, retrouvée telle
// quelle au retour (texte, curseur, annulation de l'éditeur, résultat,
// défilements, liste des objets) ; un autre classeur la remet à zéro.

/**
 * Génération du classeur pour laquelle la vue reste montée, après un rendu
 * où elle est (shown) ou n'est pas affichée ; null : jamais ouverte.
 */
export function keptFor(kept: number | null, shown: boolean, workbook: number): number | null {
	return shown ? workbook : kept;
}

/** La vue est-elle montée ? Affichée, ou déjà ouverte sur ce même classeur. */
export function keepMounted(kept: number | null, shown: boolean, workbook: number): boolean {
	return shown || kept === workbook;
}
