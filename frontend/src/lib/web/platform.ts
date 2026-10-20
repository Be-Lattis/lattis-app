// Version web ou bureau, connue à la construction (npm run build:web :
// mode « web »). Le scripting permis se lit à l'exécution
// (DocumentService.Platform, voir shell/settings.svelte.ts).

import { t } from '#lib/i18n/index.js';

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

const agent = typeof navigator === 'undefined' ? '' : navigator.userAgent;

/** macOS : les raccourcis s'écrivent avec Cmd (la grille prend Cmd comme Ctrl). */
export const isMac: boolean = /Mac/.test(agent);

/** Bureau Windows : son imprimante PDF a un nom connu. */
export const isWindowsDesktop: boolean = !isWeb && /Windows/.test(agent);

/** Texte d'aide ou d'infobulle : Ctrl+… devient Cmd+… sur macOS. */
export function hintKeys(text: string, mac = isMac): string {
	return mac ? text.replace(/\bCtrl\+/g, 'Cmd+') : text;
}

/** Touches à afficher pour un raccourci : son équivalent en web s'il en a un, Cmd sur macOS. */
export function keyLabel(keys: string, web = isWeb, mac = isMac): string {
	return hintKeys(web ? (BROWSER_KEYS[keys] ?? keys) : keys, mac);
}

/** Comment obtenir un PDF depuis la fenêtre d'impression, selon le système. */
export function pdfPrinterHint(windows = isWindowsDesktop): string {
	return windows ? t('doc.web.pdf.windows') : t('doc.web.pdf.browser');
}
