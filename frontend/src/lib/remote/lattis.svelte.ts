// État partagé des Serveurs Lattis : serveurs connus et utilisateur
// connecté à chacun (menu Compte), demandes de connexion (page de
// connexion affichée par LattisHost). Rien n'est chargé tant que les
// classeurs partagés sont coupés (Paramètres › Fonctionnalités).
import { DocumentService } from '#bindings/internal/bridge/index.js';
import type { LattisState } from '#bindings/internal/bridge/models.js';

export const lattis = $state<{ state: LattisState; login: LoginRequest | null }>({
	state: { enabled: false, allowUserServers: true, servers: [] },
	login: null
});

export interface LoginRequest {
	/** Adresse du serveur ('' : à saisir dans la page de connexion). */
	url: string;
	done: (ok: boolean) => void;
}

/** Relit l'état (silencieux en cas d'échec : rien d'affiché). */
export async function loadLattis() {
	try {
		lattis.state = await DocumentService.LattisState();
	} catch {
		// Pas de pont (tests).
	}
}

/**
 * Ouvre la page de connexion d'un serveur ; vrai une fois connecté, faux si
 * l'utilisateur abandonne. Une demande déjà ouverte est abandonnée.
 */
export async function requestLogin(url: string): Promise<boolean> {
	lattis.login?.done(false);
	if (!lattis.state.enabled) await loadLattis();
	if (!lattis.state.enabled) return false;
	return new Promise((resolve) => {
		lattis.login = {
			url,
			done: (ok) => {
				lattis.login = null;
				loadLattis();
				resolve(ok);
			}
		};
	});
}

/**
 * Classeur partagé ouvert dont l'utilisateur gère les droits (propriétaire)
 * ou les consulte (concepteur) : le menu Compte propose « Gérer l'accès… »
 * (écran des droits), ouvert par open ; null sinon.
 */
export const shareRights = $state<{ current: { title: string; open: () => void } | null }>({ current: null });

/** Ferme la session d'un serveur. */
export async function signOut(url: string) {
	lattis.state = await DocumentService.LattisSignOut(url);
}
