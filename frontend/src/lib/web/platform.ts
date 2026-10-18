// Version web ou bureau, connue à la construction (npm run build:web :
// mode « web »). Le scripting permis se lit à l'exécution
// (DocumentService.Platform, voir shell/settings.svelte.ts).

export const isWeb: boolean = import.meta.env.MODE === 'web';

/**
 * Raccourcis que le navigateur garde pour lui (nouvelle fenêtre, fermer
 * l'onglet, onglet suivant, outils de développement) et leur équivalent,
 * reconnu partout (bureau et web) mais affiché seulement en web.
 */
export const BROWSER_KEYS: Record<string, string> = {
	'Ctrl+N': 'Ctrl+Alt+N',
	'Ctrl+W': 'Ctrl+Alt+W',
	F12: 'Ctrl+Maj+S',
	'Ctrl+Pg.Suiv': 'Alt+Pg.Suiv',
	'Ctrl+Pg.Préc': 'Alt+Pg.Préc'
};

/** Touches à afficher pour un raccourci : son équivalent en web s'il en a un. */
export function keyLabel(keys: string, web = isWeb): string {
	return web ? (BROWSER_KEYS[keys] ?? keys) : keys;
}
