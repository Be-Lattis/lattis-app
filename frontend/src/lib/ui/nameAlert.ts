// Nom refusé (règles locales de names.ts ou refus du moteur) : une boîte
// bloquante l'explique, puis le champ reprend la main, toujours en
// saisie, le texte tapé gardé et sélectionné ; jamais de retour silencieux
// à l'ancien nom.
import { mount, unmount } from 'svelte';
import NameAlert from './NameAlert.svelte';
import type { NameProblem } from './names.js';

type Field = HTMLInputElement | HTMLTextAreaElement;

let open: Promise<void> | null = null;

/** La boîte est-elle ouverte ? (le champ perd alors le focus : son onblur ne doit ni valider ni abandonner). */
export const nameAlertOpen = () => open !== null;

/**
 * Affiche la boîte ; à sa fermeture, le champ (ou celui que rend la
 * fonction, s'il a été redessiné) reprend le focus avec text (par défaut
 * son contenu), tout sélectionné. Une seule boîte à la fois.
 */
export function refuseName(field: Field | null | undefined | (() => Field | null | undefined), problem: NameProblem, text?: string): Promise<void> {
	if (open) return open;
	const p = new Promise<void>((resolve) => {
		let app: ReturnType<typeof mount> | null = null;
		const close = () => {
			if (!app) return;
			const a = app;
			app = null;
			unmount(a);
			open = null;
			const el = typeof field === 'function' ? field() : field;
			if (el?.isConnected) {
				if (text !== undefined) el.value = text;
				el.focus({ preventScroll: true });
				el.select();
			}
			resolve();
		};
		app = mount(NameAlert, { target: document.body, props: { title: problem.title, message: problem.message, onclose: close } });
	});
	open = p;
	return p;
}
