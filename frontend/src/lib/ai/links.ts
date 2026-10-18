// Liens externes de l'assistant (console du fournisseur, liens des
// réponses). Sur le bureau, le navigateur du système (Browser.OpenURL) ; en
// version web, le navigateur de l'utilisateur : Browser.OpenURL ouvrirait
// celui du serveur.
import { isWeb } from '#lib/web/platform.js';
import { platform } from '#lib/shell/settings.svelte.js';

/** Ouvre une adresse http(s) hors de l'application. */
export function openOutside(u: string) {
	if (!/^https?:\/\//i.test(u)) return;
	const tab = () => void window.open(u, '_blank', 'noopener');
	if (isWeb || platform.web) return tab();
	import('@wailsio/runtime').then(
		(rt) => Promise.resolve(rt.Browser.OpenURL(u)).catch(tab),
		tab
	);
}
