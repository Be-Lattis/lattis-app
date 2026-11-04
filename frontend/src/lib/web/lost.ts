// Appel perdu avec la liaison (version web) : la connexion au moteur s'est
// coupée avant sa réponse. Il a pu s'exécuter ou non ; ce qui peut être
// refait sans risque (écrire la même valeur dans la même cellule) l'est,
// une fois, à la reconnexion (l'appel attend dans la file de la page).

/** Marque l'erreur d'un appel perdu avec la liaison. */
export function markLost<E extends Error>(e: E): E {
	return Object.assign(e, { lost: true });
}

/** L'appel a été perdu avec la liaison (pas refusé par le moteur). */
export function isLost(e: unknown): boolean {
	return !!e && typeof e === 'object' && (e as { lost?: boolean }).lost === true;
}

/** fn, refait une fois si la liaison s'est coupée avant sa réponse (fn doit pouvoir être refait). */
export async function againIfLost<T>(fn: () => Promise<T>): Promise<T> {
	try {
		return await fn();
	} catch (e) {
		if (!isLost(e)) throw e;
		return fn();
	}
}
