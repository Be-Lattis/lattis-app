// Accord des pages web (point C2 : sécurité des classeurs reçus). Une page
// web d'un classeur (onglet HTML) ne s'exécute que si sa version
// (empreinte du HTML) est autorisée sur ce poste (internal/trust) : créée
// ou modifiée ici, ou autorisée par l'utilisateur. Sinon, au lieu de
// l'exécuter, l'onglet demande « Autoriser / Ne pas exécuter / Voir le
// code ». « Ne pas exécuter » vaut pour la session (declinedPages) ;
// l'onglet garde un bouton pour décider de nouveau.
import type { DialogSpec } from '#lib/script/ScriptDialog.svelte';
import { pageUrl } from './bridge';

/** Mise en garde commune aux accords d'exécution (voir api.TrustWarning pour les scripts). */
export const PAGE_WARNING =
	'N’autorisez que si vous faites confiance à la personne qui vous a fourni ce classeur. ' +
	'Dans le doute, choisissez « Ne pas exécuter » : le reste du classeur reste utilisable.';

/** Versions de pages que l'utilisateur a choisi de ne pas exécuter pendant la session. */
export const declinedPages = new Set<string>();

let ids = -1;

/** Boîte de dialogue d'accord pour la page de l'onglet name, de contenu source. */
export function pageConsent(name: string, source: string): DialogSpec {
	const url = pageUrl(source);
	const what = url
		? `L’onglet « ${name} » affiche le site externe ${url}, qui n’a encore jamais été autorisé dans ce classeur sur ce poste (classeur reçu, ou onglet modifié ailleurs). Ce site s’exécute dans l’onglet, sans accès au classeur.`
		: `L’onglet « ${name} » contient une page web (HTML et JavaScript) qui n’a encore jamais été autorisée sur ce poste (classeur reçu, ou page modifiée ailleurs). ` +
			'Une page peut lire et modifier le classeur, lancer les actions de ses boutons, vous poser des questions et, avec votre accord, contacter des sites web.';
	return {
		id: ids--,
		title: 'Exécuter une page web du classeur ?',
		message: what,
		warning: PAGE_WARNING,
		fields: [],
		ok: 'Autoriser',
		cancel: 'Ne pas exécuter',
		code: source,
		codeName: 'Code de la page'
	};
}
