// Accord des pages web (point C2 : sécurité des classeurs reçus). Une page
// web d'un classeur (onglet HTML) ne s'exécute que si sa version
// (empreinte du HTML) est autorisée sur ce poste (internal/trust) : créée
// ou modifiée ici, ou autorisée par l'utilisateur. Sinon, au lieu de
// l'exécuter, l'onglet demande « Autoriser / Ne pas exécuter / Voir le
// code ». « Ne pas exécuter » vaut pour la session (declinedPages) ;
// l'onglet garde un bouton pour décider de nouveau.
import type { DialogSpec } from '#lib/script/ScriptDialog.svelte';
import { pageUrl } from './bridge';
import { t } from '#lib/i18n/index.js';

/** Mise en garde commune aux accords d'exécution (voir api.TrustWarning pour les scripts). */
export function pageWarning(): string {
	return t('doc.consent.warning');
	}

/** Versions de pages que l'utilisateur a choisi de ne pas exécuter pendant la session. */
export const declinedPages = new Set<string>();

let ids = -1;

/** Boîte de dialogue d'accord pour la page de l'onglet name, de contenu source. */
export function pageConsent(name: string, source: string): DialogSpec {
	const url = pageUrl(source);
	const what = url ? t('doc.consent.site', { name, url }) : t('doc.consent.page', { name });
	return {
		id: ids--,
		title: t('doc.consent.title'),
		message: what,
		warning: pageWarning(),
		fields: [],
		ok: t('doc.consent.ok'),
		cancel: t('doc.consent.cancel'),
		code: source,
		codeName: t('doc.consent.code')
	};
}
