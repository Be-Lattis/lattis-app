// Version web ou bureau, connue à la construction (npm run build:web :
// mode « web »). Le scripting permis se lit à l'exécution
// (DocumentService.Platform, voir shell/settings.svelte.ts).

import { lang, t, tAny } from '#lib/i18n/index.js';

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

/** Touches à afficher pour un raccourci : son équivalent en web s'il en a un, Cmd sur macOS, noms des touches dans la langue de l'interface. */
export function keyLabel(keys: string, web = isWeb, mac = isMac): string {
	return keyNames(hintKeys(web ? (BROWSER_KEYS[keys] ?? keys) : keys, mac));
}

// Noms des touches tels que les raccourcis les écrivent (clavier français),
// et leur message (common.keys.*) : « Ctrl+Maj+L » devient « Ctrl+Shift+L » en anglais.
const KEY_IDS: Record<string, string> = {
	Maj: 'shift',
	Entrée: 'enter',
	Espace: 'space',
	Échap: 'esc',
	Suppr: 'del',
	Origine: 'home',
	Fin: 'end',
	'Pg.Suiv': 'pageDown',
	'Pg.Préc': 'pageUp',
	Flèches: 'arrows',
	Flèche: 'arrow',
	Haut: 'up',
	Bas: 'down',
	Gauche: 'left',
	Droite: 'right',
	Lettre: 'letter',
	puis: 'then',
	clic: 'click',
	molette: 'wheel',
	Menu: 'menu'
};
const KEY_RE = new RegExp(
	`(?<![\\p{L}.])(${Object.keys(KEY_IDS)
		.sort((a, b) => b.length - a.length)
		.map((k) => k.replace('.', '\\.'))
		.join('|')})(?![\\p{L}])`,
	'gu'
);

/** Noms des touches d'un raccourci dans la langue de l'interface (le français les écrit déjà). */
export function keyNames(keys: string): string {
	return lang() === 'fr' ? keys : keys.replace(KEY_RE, (k) => tAny(`common.keys.${KEY_IDS[k]}`));
}

/** Comment obtenir un PDF depuis la fenêtre d'impression, selon le système. */
export function pdfPrinterHint(windows = isWindowsDesktop): string {
	return windows ? t('doc.web.pdf.windows') : t('doc.web.pdf.browser');
}
