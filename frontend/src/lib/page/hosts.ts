// Hôtes HTTP autorisés pour un classeur (onglet Script › Accès réseau).

/**
 * Hôte saisi par l'utilisateur, tel que le rangent les scripts : en
 * minuscules, avec le port s'il est donné. Une adresse complète
 * (https://api.exemple.fr/v1) donne son hôte ; null si ce n'est pas un nom
 * d'hôte.
 */
export function hostOf(input: string): string | null {
	let s = input.trim();
	if (!s) return null;
	if (/^[a-z][a-z0-9+.-]*:\/\//i.test(s)) {
		try {
			const u = new URL(s);
			if (u.protocol !== 'http:' && u.protocol !== 'https:') return null;
			s = u.host;
		} catch {
			return null;
		}
	} else {
		s = s.replace(/[/?#].*$/, '');
	}
	s = s.toLowerCase();
	return /^(\[[0-9a-f:.]+\]|[\p{L}\p{N}](?:[\p{L}\p{N}.-]*[\p{L}\p{N}])?)(:\d{1,5})?$/u.test(s) ? s : null;
}
