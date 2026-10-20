// Édition web servie par lattisd : /w/<id> › « Ouvrir dans le navigateur »
// mène à /app/?workbook=<id> (contrat du serveur, édition web). Au
// chargement, la page ouvre ce classeur partagé, puis retire le paramètre
// de l'adresse (un rechargement ne le rouvre pas par-dessus).

/** Identifiant du classeur demandé par l'adresse ('' : aucun). */
export function workbookParam(search: string): string {
	return new URLSearchParams(search).get('workbook')?.trim() ?? '';
}

/** Adresse sans le paramètre workbook (les autres restent). */
export function withoutWorkbookParam(href: string): string {
	const u = new URL(href);
	u.searchParams.delete('workbook');
	return u.pathname + u.search + u.hash;
}
