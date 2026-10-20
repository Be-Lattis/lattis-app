// Contrôles posés sur une feuille (internal/canvas/buttons.go) : bouton
// (un clic exécute un script de la bibliothèque, ou l'une de ses
// fonctions), case à cocher, liste déroulante, zone de saisie et sélecteur
// de fichier (leur valeur est écrite dans la cellule liée). Des écouteurs
// (controls.on dans un script) s'ajoutent à l'action du contrôle.
import { t } from '#lib/i18n/index.js';

/** Genre de contrôle : bouton (''), case à cocher, liste, saisie sur une ligne, sur plusieurs, fichier. */
export type ControlKind = '' | 'checkbox' | 'select' | 'input' | 'textarea' | 'file';

/** Écouteur d'un évènement (« click » d'un bouton, « change » des autres) : fonction d'un script. */
export interface Listener {
	event: string;
	script: string;
	function: string;
}

export interface ButtonConfig {
	/** Nom unique dans le classeur (« Bouton1 »), par lequel les scripts désignent le contrôle ; vide : automatique. */
	name?: string;
	/** Contrôle : bouton (absent), case à cocher, liste déroulante, zone de saisie, sélecteur de fichier. */
	kind?: ControlKind;
	label: string;
	/** Script de la bibliothèque ; vide : aucune action. */
	script: string;
	function?: string;
	/** Cellule liée (« B3 ») d'un contrôle autre qu'un bouton : sa valeur y est écrite ; facultative. */
	link?: string;
	/** Valeur d'un contrôle sans cellule liée (gardée dans le contrôle). */
	value?: string;
	/** Choix d'une liste déroulante. */
	options?: string[];
	/** Colonne de tableau qui donne les choix d'une liste (« Clients[Ville] »), à la place d'options. */
	optionsFrom?: string;
	/** Couleurs (« #rrggbb ») : fond normal, survolé, enfoncé, texte. */
	color?: string;
	hoverColor?: string;
	activeColor?: string;
	textColor?: string;
	/** Écouteurs ajoutés par les scripts (controls.on). */
	listeners?: Listener[];
	/** Contrôle masqué (controls.set(nom, {hidden = true})). */
	hidden?: boolean;
	offX?: number;
	offY?: number;
	width?: number;
	height?: number;
}

export interface SheetButton {
	id: number;
	row: number;
	col: number;
	config: ButtonConfig;
	/** Choix d'une liste alimentée par une colonne (optionsFrom), relus avec la feuille. */
	choices?: string[] | null;
}

/** Choix affichés d'une liste : ceux de sa colonne, ou ceux saisis. */
export function choicesOf(b: { config: ButtonConfig; choices?: string[] | null }): string[] {
	return b.config.optionsFrom ? (b.choices ?? []) : (b.config.options ?? []);
}

export const BUTTON_W = 120;
export const BUTTON_H = 32;

/** Noms des genres de contrôles (dans la langue de l'interface, lus à chaque usage). */
export const CONTROL_KINDS: Record<ControlKind, string> = {
	get ''() {
		return t('grid.controls.kind.button');
	},
	get input() {
		return t('grid.controls.kind.input');
	},
	get textarea() {
		return t('grid.controls.kind.textarea');
	},
	get checkbox() {
		return t('grid.controls.kind.checkbox');
	},
	get select() {
		return t('grid.controls.kind.select');
	},
	get file() {
		return t('grid.controls.kind.file');
	}
};

/** Contrôles liés à une cellule (Insertion › Champ), dans l'ordre du choix du type. */
export const VALUE_KINDS: ControlKind[] = ['input', 'textarea', 'checkbox', 'select', 'file'];

/** Évènement qui déclenche l'action d'un contrôle : un clic pour un bouton, un changement de valeur sinon. */
export function controlEvent(kind: ControlKind | undefined): 'click' | 'change' {
	return kind ? 'change' : 'click';
}

/** Ce qu'exécute l'évènement d'un contrôle : son action, puis ses écouteurs, dans l'ordre. */
export function handlers(cfg: ButtonConfig, event = controlEvent(cfg.kind)): { script: string; function: string; action: boolean }[] {
	const out: { script: string; function: string; action: boolean }[] = [];
	if (cfg.script && event === controlEvent(cfg.kind)) out.push({ script: cfg.script, function: cfg.function ?? '', action: true });
	for (const l of cfg.listeners ?? []) if (l.event === event) out.push({ script: l.script, function: l.function, action: false });
	return out;
}

/** « Script › fonction() » d'une action ou d'un écouteur. */
export function handlerLabel(h: { script: string; function?: string }): string {
	return h.function ? `${h.script} › ${h.function}()` : h.script;
}

/** Info-bulle d'un contrôle : nom, cellule liée, action et écouteurs. */
export function controlTitle(cfg: ButtonConfig): string {
	const lines = [cfg.name ?? ''];
	if (cfg.kind) lines.push(cfg.link ? t('grid.controls.title.link', { cell: cfg.link }) : t('grid.controls.title.value'));
	const all = [...handlers(cfg, 'click'), ...handlers(cfg, 'change')];
	if (!all.length) lines.push(cfg.kind ? '' : t('grid.controls.title.noAction'));
	for (const h of all) lines.push(t(h.action ? 'grid.controls.title.action' : 'grid.controls.title.listener', { event: controlEvent(cfg.kind), handler: handlerLabel(h) }));
	return lines.filter(Boolean).join('\n');
}

/** Nom du fichier d'un chemin (sélecteur de fichier). */
export function baseName(path: string): string {
	return path.split(/[\\/]/).filter(Boolean).pop() ?? '';
}

/**
 * Fonctions globales définies par un script (« function nom() » ou
 * « function nom(e) », qui reçoit l'évènement), dans l'ordre : un clic ou
 * un changement de valeur peut les appeler.
 */
export function scriptFunctions(source: string): string[] {
	const out: string[] = [];
	const code = source.replace(/--\[(=*)\[[\s\S]*?\]\1\]/g, '').replace(/--[^\n]*/g, '');
	for (const m of code.matchAll(/(?<![\w.:])(?<!\blocal\s+)function\s+([A-Za-z_]\w*)\s*\(\s*(?:[A-Za-z_]\w*\s*)?\)/g)) {
		if (!out.includes(m[1])) out.push(m[1]);
	}
	return out;
}
