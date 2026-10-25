// Raccourcis clavier de l'application, par domaine : la seule liste affichée
// à l'utilisateur (aide › Raccourcis clavier). Le traitement des touches est
// ailleurs (grid/shortcuts.ts, Sheet.svelte, +page.svelte, éditeurs) : une
// touche ajoutée là s'ajoute ici. Les lettres du ruban (KeyTips) viennent
// de ui/keytips.ts.

import { t, tAny } from '#lib/i18n/index.js';
import { TAB_TIPS } from '#lib/ui/keytips.js';
import { isMac, isWeb, keyLabel } from '#lib/web/platform.js';
import { tabLabel } from './ribbon.js';

export interface ShortcutInfo {
	/** Touches, écrites comme sur le clavier français : « Ctrl+Maj+L ». */
	keys: string;
	/** Ce que fait le raccourci, en une phrase courte. */
	label: string;
}

export interface ShortcutGroup {
	id: string;
	title: string;
	items: ShortcutInfo[];
}

/**
 * Groupe de raccourcis : titre et libellés lus dans la langue courante à
 * chaque affichage (getters), sous shell.shortcuts.<groupe>.
 */
function group(id: string, items: (readonly [keys: string, label: string | (() => string)])[]): ShortcutGroup {
	return {
		id,
		get title() {
			return tAny(`shell.shortcuts.${id}.title`);
		},
		items: items.map(([keys, label]) => ({
			keys,
			get label() {
				return typeof label === 'function' ? label() : tAny(`shell.shortcuts.${id}.${label}`);
			}
		}))
	};
}

const groups: ShortcutGroup[] = [
	group('fichier', [
		['Ctrl+N', 'new'],
		['Ctrl+O', 'open'],
		['Ctrl+S', 'save'],
		['F12', 'saveAs'],
		['Ctrl+P', 'print'],
		['Ctrl+W', 'close'],
		['Ctrl+Z', 'undo'],
		['Ctrl+Y', 'redo'],
		['F1', 'help']
	]),
	group('clavier', [
		['Alt / F10', 'keyTips'],
		['Ctrl+F1', 'fold'],
		['Alt puis F', 'fileMenu'],
		...Object.entries(TAB_TIPS)
			.filter(([id]) => id !== 'file' && id !== 'help')
			.map(([id, k]) => [`Alt puis ${k}`, () => t('shell.shortcuts.clavier.tab', { tab: tabLabel(id), key: k })] as const),
		['Flèches', 'arrows'],
		['F6 / Maj+F6', 'regions'],
		['Maj+F10 / Menu', 'contextMenu'],
		['Haut / Bas', 'menuUpDown'],
		['Lettre', 'menuLetter'],
		['Entrée / Espace', 'menuChoose'],
		['Tab / Maj+Tab', 'dialogFields'],
		['Échap', 'escape']
	]),
	group('feuilles', [
		['Ctrl+Pg.Suiv', 'next'],
		['Ctrl+Pg.Préc', 'previous'],
		['Maj+F11', 'insert'],
		['Gauche / Droite', 'tabArrows'],
		['Alt+Bas', 'choiceList'],
		['Ctrl+Alt+Pg.Suiv', 'choiceNext'],
		['Ctrl+Alt+Pg.Préc', 'choicePrevious']
	]),
	group('deplacement', [
		['Flèches', 'arrows'],
		['Ctrl+Flèche', 'edge'],
		['Fin puis Flèche', 'endArrow'],
		['Maj+Flèche', 'extend'],
		['Pg.Suiv / Pg.Préc', 'page'],
		['Origine', 'home'],
		['Ctrl+Origine', 'a1'],
		['Ctrl+Fin', 'lastCell'],
		['Tab / Maj+Tab', 'tab'],
		['Ctrl+A', 'selectTable'],
		['Ctrl+Espace', 'selectColumn'],
		['Maj+Espace', 'selectRow']
	]),
	group('saisie', [
		['F2', 'edit'],
		['Entrée', 'enter'],
		['Ctrl+Entrée', 'enterAll'],
		['Alt+Entrée', 'lineBreak'],
		['Échap', 'cancel'],
		['Suppr', 'delete'],
		['Ctrl+;', 'today'],
		['Ctrl+Maj+:', 'now'],
		['Alt+=', 'autoSum'],
		['Alt+Bas', 'dropDown'],
		['Ctrl+D', 'fillDown'],
		['Ctrl+R', 'fillRight'],
		['F4', 'repeat']
	]),
	group('edition', [
		['Ctrl+C', 'copy'],
		['Ctrl+X', 'cut'],
		['Ctrl+V', 'paste'],
		['Ctrl+Alt+V', 'pasteSpecial'],
		['Ctrl+F', 'find'],
		['Ctrl+H', 'replace'],
		['Ctrl+B / Ctrl+I / Ctrl+U', 'bold'],
		['Ctrl+1', 'format'],
		['Ctrl++ / Ctrl+-', 'insertDelete'],
		['Ctrl+5', 'strike'],
		['Ctrl+9 / Ctrl+0', 'hide'],
		['Ctrl+L / Ctrl+T', 'table'],
		['Ctrl+Maj+L', 'filter'],
		['Maj+F2', 'note'],
		['Ctrl+K', 'link'],
		['Alt+Maj+→ / Alt+Maj+←', 'group'],
		['Alt+F1 / F11', 'chart']
	]),
	group('calcul', [
		['F5', 'refresh'],
		['Ctrl+Alt+F5 / Ctrl+F5 / Maj+F5', 'refreshAll'],
		['Maj+F3', 'insertFunction'],
		['F9', 'recalc']
	]),
	group('editeurs', [
		['Ctrl+Entrée', 'sqlRun'],
		['Ctrl+Espace', 'sqlNames'],
		['Alt+Haut / Alt+Bas', 'sqlHistory'],
		['Ctrl+Entrée', 'scriptRun'],
		['Ctrl+S', 'save'],
		['Ctrl+Entrée', 'htmlPreview']
	])
];

/**
 * Les raccourcis affichés. En web, ceux que le navigateur garde pour lui
 * (Ctrl+N, Ctrl+W, F12, Ctrl+Pg.Suiv…) sont remplacés par leur équivalent
 * (web/platform.ts), reconnu aussi sur le bureau ; sur macOS, Ctrl s'écrit Cmd.
 */
export const shortcutGroups: ShortcutGroup[] = isWeb || isMac ? webGroups(groups, isWeb) : groups;

/** Touches réécrites (web, macOS) ; titres et libellés restent lus dans la langue courante. */
export function webGroups(list: ShortcutGroup[], web = true): ShortcutGroup[] {
	return list.map((g) => ({
		id: g.id,
		get title() {
			return g.title;
		},
		items: g.items.map((s) => ({
			keys: keyLabel(s.keys, web),
			get label() {
				return s.label;
			}
		}))
	}));
}
