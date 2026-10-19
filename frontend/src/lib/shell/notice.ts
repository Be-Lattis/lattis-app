// Messages de la barre d'état : ils s'effacent seuls au bout d'un moment
// (plus long pour une erreur), et à chaque changement d'onglet.

/**
 * Le message signale-t-il un problème (erreur, refus, avertissement) ? Il est
 * alors montré en avertissement (bulle orange) ; sinon c'est un compte rendu
 * de l'action faite (texte vert clair), choix de Louis.
 */
export function isProblem(msg: string): boolean {
	return /^#[A-Z]|erreur|impossible|refus|introuvable|invalide|échec|n'est pas|n’est pas|ne peut|ne se |protégé|verrouill|lecture seule|attention|trop |aucun|placez|sélectionnez|hors |déjà|manque|interrompu|conflit/i.test(msg);
}

/** Durée d'affichage d'un message, en millisecondes : 8 s, 15 s pour un problème. */
export function noticeTtl(msg: string): number {
	return isProblem(msg) ? 15000 : 8000;
}
