// Serveur Lattis (classeurs partagés) : logique de la page de connexion et
// du menu Compte, sans composant (testée par vitest). L'état vient de
// DocumentService.LattisState (internal/bridge/lattis.go).
import type { Provider, ServerState, User } from '#bindings/internal/remote/auth/models.js';

export const PASSWORD = 'password';

/** Libellé du bouton d'un fournisseur : « Continuer avec Google », « Continuer avec un mot de passe ». */
export function providerButton(p: Provider): string {
	if (p.kind === PASSWORD && (!p.label || p.label === p.id)) return 'Continuer avec un mot de passe';
	return `Continuer avec ${p.label || p.id}`;
}

/** Une rangée de la page de connexion : le bouton d'un fournisseur. */
export type LoginRow = { kind: 'button'; provider: Provider };

/**
 * Rangées de la page de connexion, dans l'ordre du serveur : un bouton par
 * fournisseur. Le mot de passe se saisit lui aussi dans le navigateur, sur
 * la page de lattisd (choix de Louis : la fenêtre de Lattis ne voit jamais
 * le mot de passe).
 */
export function loginRows(providers: readonly Provider[] | null | undefined): LoginRow[] {
	return (providers ?? []).map((p) => ({ kind: 'button', provider: p }));
}

/** Icône intégrée d'un genre de fournisseur, quand le serveur n'en donne pas. */
export function providerGlyph(kind: string): 'google' | 'microsoft' | 'key' | 'lock' {
	switch (kind) {
		case 'google':
		case 'google_workspace':
			return 'google';
		case 'microsoft':
			return 'microsoft';
		case PASSWORD:
			return 'key';
	}
	return 'lock';
}

/** Nom à afficher d'un utilisateur. */
export function userName(u: User | null | undefined): string {
	if (!u) return '';
	return u.displayName || u.email || u.pgRole || u.id || '?';
}

/** Ligne secondaire (adresse) quand elle diffère du nom. */
export function userDetail(u: User | null | undefined): string {
	if (!u?.email || u.email === userName(u)) return '';
	return u.email;
}

/** Initiales pour la pastille du menu Compte. */
export function initials(u: User | null | undefined): string {
	const n = userName(u).replace(/@.*/, '').trim();
	if (!n) return '';
	const parts = n.split(/[\s._-]+/).filter(Boolean);
	const s = parts.length > 1 ? parts[0][0] + parts[parts.length - 1][0] : n.slice(0, 2);
	return s.toUpperCase();
}

/** Hôte d'une adresse de serveur (affichage). */
export function hostOf(url: string): string {
	try {
		return new URL(url).host;
	} catch {
		return url;
	}
}

/** Serveurs où l'utilisateur est connecté. */
export function connected(servers: readonly ServerState[] | null | undefined): ServerState[] {
	return (servers ?? []).filter((s) => !!s.user);
}

/** Libellé d'un serveur dans une liste de choix : « Acme — Alice » (ou « non connecté »). */
export function serverOption(s: ServerState): string {
	return `${s.name || hostOf(s.url)} — ${s.user ? userName(s.user) : 'non connecté'}`;
}

/**
 * Serveur choisi après « Gérer les serveurs » : celui qui vient d'être
 * ajouté s'il est proposé, sinon le choix courant s'il l'est encore, sinon
 * le premier ('' : aucun).
 */
export function pickServer(urls: readonly string[], current: string, added = ''): string {
	if (added && urls.includes(added)) return added;
	if (current && urls.includes(current)) return current;
	return urls[0] ?? '';
}

/** Titre du bouton Compte de la barre de titre. */
export function accountTitle(servers: readonly ServerState[] | null | undefined): string {
	const c = connected(servers);
	if (!c.length) return 'Compte : non connecté à un Serveur Lattis';
	return 'Compte : ' + c.map((s) => `${userName(s.user)} sur ${s.name}`).join(', ');
}

/** Un Serveur Lattis s'adresse en https:// (http seulement en local, pour les essais). */
export function serverUrlOk(url: string): boolean {
	let s = url.trim();
	if (!s) return false;
	if (!s.includes('://')) s = 'https://' + s;
	try {
		const u = new URL(s);
		if (u.protocol === 'https:') return !!u.hostname;
		return u.protocol === 'http:' && /^(localhost|127\.\d+\.\d+\.\d+|\[::1\])$/i.test(u.hostname);
	} catch {
		return false;
	}
}

/** Temps restant d'un code de connexion (« 9 min 59 s »), '' une fois expiré. */
export function remaining(expiresAt: string, now = Date.now()): string {
	const ms = Date.parse(expiresAt) - now;
	if (!(ms > 0)) return '';
	const s = Math.ceil(ms / 1000);
	const m = Math.floor(s / 60);
	return m ? `${m} min ${String(s % 60).padStart(2, '0')} s` : `${s} s`;
}
