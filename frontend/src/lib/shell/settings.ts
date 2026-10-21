// Fichier › Paramètres : la liste déclarative des réglages affichés, par
// section. Chaque réglage est une clé de la configuration (config.json de
// l'utilisateur) que le serveur accepte d'écrire (config.UserEditable).
// Ajouter un réglage : une entrée ici, et sa clé dans UserEditable.
import { themeCatalog, themeLabel } from '#lib/grid/theme.js';
import { LANG_NAMES, LANGS, systemLang, t } from '#lib/i18n/index.js';
import { isWeb } from '#lib/web/platform.js';

/**
 * Pages web des classeurs permises : réglage features.htmlPages de la
 * configuration (coupé par défaut, sans case dans les paramètres : à la main
 * ou par l'organisation), rendu par DocumentService.Platform ; jamais dans la
 * version web ni la démo publique (le JavaScript d'une page peut faire
 * sortir des données en faisant naviguer son cadre).
 */
export function htmlPagesOn(p: { web: boolean; demo?: boolean; htmlPages?: boolean }): boolean {
	return !!p.htmlPages && !p.web && !p.demo;
}

/** État d'un réglage rendu par DocumentService.Settings. */
export interface SettingState {
	path: string;
	value: unknown;
	locked?: boolean;
	fixed?: boolean;
	source?: string;
	/** ui.themes : identifiants venus de l'organisation (pas du fichier de l'utilisateur). */
	managed?: string[] | null;
	/** Réglage lu, jamais écrit par l'interface (ui.builtinThemes). */
	readOnly?: boolean;
}

export interface SettingDef {
	path: string;
	kind: 'bool' | 'choice' | 'text' | 'number';
	label: string;
	help?: string;
	/** Exemple affiché dans un réglage 'text' vide. */
	placeholder?: string;
	/** Choix d'un réglage 'choice' (valeur, libellé). */
	choices?: { value: string; label: string; swatch?: string[] }[];
	/** Choix tirés des réglages (thèmes proposés), à la place de choices. */
	choicesOf?: (list: readonly SettingState[]) => { value: string; label: string; swatch?: string[] }[];
	/** Réglage d'une fonctionnalité à activer : masqué tant qu'elle ne l'est pas. */
	requires?: string;
	/** Bornes et unité d'un réglage 'number' (entier). */
	min?: number;
	max?: number;
	unit?: string;
}

export interface SettingSection {
	id: string;
	title: string;
	settings: SettingDef[];
	/** Section d'une fonctionnalité à activer (ai.enabled…) : masquée tant qu'elle ne l'est pas. */
	requires?: string;
	/** Section de l'application de bureau seulement (mises à jour) : masquée en édition web. */
	desktopOnly?: boolean;
}

// Textes lus dans la langue courante à chaque affichage (getters) : la liste
// est une constante de module, ses libellés suivent la langue.
export const settingSections: SettingSection[] = [
	{
		id: 'features',
		get title() {
			return t('shell.settings.sections.features');
		},
		settings: [
			{
				path: 'features.sharedWorkbooks',
				kind: 'bool',
				get label() {
					return t('shell.settings.sharedWorkbooks.label');
				},
				get help() {
					return t('shell.settings.sharedWorkbooks.help');
				}
			},
			{
				path: 'features.coEditing',
				kind: 'bool',
				get label() {
					return t('shell.settings.coEditing.label');
				},
				get help() {
					return t('shell.settings.coEditing.help');
				}
			},
			{
				path: 'ai.enabled',
				kind: 'bool',
				get label() {
					return t('shell.settings.aiEnabled.label');
				},
				get help() {
					return t('shell.settings.aiEnabled.help');
				}
			}
		]
	},
	{
		id: 'ai',
		get title() {
			return t('shell.settings.sections.ai');
		},
		requires: 'ai.enabled',
		settings: [
			{
				path: 'ai.dataSharing',
				kind: 'choice',
				get label() {
					return t('shell.settings.aiDataSharing.label');
				},
				get choices() {
					return [
						{ value: 'schema-only', label: t('shell.settings.aiDataSharing.schemaOnly') },
						{ value: 'sample', label: t('shell.settings.aiDataSharing.sample') },
						{ value: 'full', label: t('shell.settings.aiDataSharing.full') }
					];
				},
				get help() {
					return t('shell.settings.aiDataSharing.help');
				}
			},
			{
				path: 'ai.keepConversation',
				kind: 'bool',
				get label() {
					return t('shell.settings.aiKeepConversation.label');
				},
				get help() {
					return t('shell.settings.aiKeepConversation.help');
				}
			},
			{
				path: 'ai.localUrl',
				kind: 'text',
				get label() {
					return t('shell.settings.aiLocalUrl.label');
				},
				placeholder: 'http://localhost:11434/v1',
				get help() {
					return t('shell.settings.aiLocalUrl.help');
				}
			}
		]
	},
	{
		id: 'view',
		get title() {
			return t('shell.settings.sections.view');
		},
		settings: [
			{
				path: 'ui.language',
				kind: 'choice',
				get label() {
					return t('shell.settings.language.label');
				},
				// Chaque langue dans sa propre langue ; « auto » nomme celle qu'il prend.
				choicesOf: () => [
					{ value: 'auto', label: t('shell.settings.language.auto', { name: LANG_NAMES[systemLang()] }) },
					...LANGS.map((l) => ({ value: l, label: LANG_NAMES[l] }))
				]
			},
			{
				path: 'ui.theme',
				kind: 'choice',
				get label() {
					return t('shell.settings.theme.label');
				},
				choicesOf: (list) => themeCatalog(list).map((th) => ({ value: th.id, label: themeLabel(th), swatch: [th.table, th.source] }))
			}
		]
	},
	{
		id: 'limits',
		get title() {
			return t('shell.settings.sections.limits');
		},
		settings: [
			{
				path: 'limits.undoDepth',
				kind: 'number',
				get label() {
					return t('shell.settings.undoDepth.label');
				},
				get unit() {
					return t('shell.settings.undoDepth.unit');
				},
				min: 10,
				max: 10000,
				get help() {
					return t('shell.settings.undoDepth.help');
				}
			},
			{
				path: 'limits.scriptSeconds',
				kind: 'number',
				get label() {
					return t('shell.settings.scriptSeconds.label');
				},
				unit: 's',
				min: 1,
				max: 3600,
				get help() {
					return t('shell.settings.scriptSeconds.help');
				}
			},
			{
				path: 'limits.sqlReadSeconds',
				kind: 'number',
				get label() {
					return t('shell.settings.sqlReadSeconds.label');
				},
				unit: 's',
				min: 1,
				max: 3600,
				get help() {
					return t('shell.settings.sqlReadSeconds.help');
				}
			},
			{
				path: 'limits.sqlWriteSeconds',
				kind: 'number',
				get label() {
					return t('shell.settings.sqlWriteSeconds.label');
				},
				unit: 's',
				min: 1,
				max: 3600,
				get help() {
					return t('shell.settings.sqlWriteSeconds.help');
				}
			}
		]
	},
	{
		id: 'updates',
		get title() {
			return t('shell.settings.sections.updates');
		},
		desktopOnly: true,
		settings: [
			{
				path: 'updates.check',
				kind: 'bool',
				get label() {
					return t('shell.settings.updatesCheck.label');
				},
				get help() {
					return t('shell.settings.updatesCheck.help');
				}
			}
		]
	}
];

/** Valeur d'un réglage 'number' saisie (texte) : un entier dans ses bornes, ou un message d'erreur. */
export function parseNumberSetting(d: SettingDef, raw: string): number | string {
	const s = raw.trim().replace(/[\s ]/g, '');
	const n = /^\d+$/.test(s) ? Number(s) : NaN;
	const lo = d.min ?? 0;
	const hi = d.max ?? Number.MAX_SAFE_INTEGER;
	if (!Number.isInteger(n) || n < lo || n > hi) return t('shell.settings.numberRange', { label: d.label, min: String(lo), max: String(hi) });
	return n;
}

/**
 * Sections et réglages à montrer (choix de Louis) : ceux d'une fonctionnalité
 * à activer restent cachés tant qu'elle n'est pas cochée, puisqu'ils ne
 * servent à rien. En édition web, pas de section propre au bureau.
 */
export function visibleSections(list: readonly SettingState[], web = isWeb): SettingSection[] {
	const on = (path?: string) => !path || boolSetting(list, path);
	return settingSections
		.filter((s) => on(s.requires) && !(web && s.desktopOnly))
		.map((s) => ({ ...s, settings: s.settings.filter((d) => on(d.requires)).map((d) => (d.choicesOf ? { ...d, choices: d.choicesOf(list) } : d)) }));
}

/**
 * Réglages remis à leur valeur par défaut par « Réinitialiser » : tous ceux
 * de la fenêtre (sections cachées comprises), pas ceux propres au bureau en
 * web. Le reste du config.json (thèmes créés, serveurs Lattis, fournisseur
 * de l'IA…) n'est pas un réglage de la fenêtre et reste.
 */
export function resetPaths(web = isWeb): string[] {
	return settingSections.filter((s) => !(web && s.desktopOnly)).flatMap((s) => s.settings.map((d) => d.path));
}

/** L'état d'un réglage (sans tenir compte de la casse du chemin). */
export function findSetting(list: readonly SettingState[], path: string): SettingState | undefined {
	const p = path.toLowerCase();
	return list.find((s) => s.path.toLowerCase() === p);
}

/** Valeur booléenne d'un réglage, def s'il est absent ou d'un autre type. */
export function boolSetting(list: readonly SettingState[], path: string, def = false): boolean {
	const v = findSetting(list, path)?.value;
	return typeof v === 'boolean' ? v : def;
}

/** Note d'un réglage qu'on ne peut pas changer ici ('' sinon). */
export function settingNote(s: SettingState | undefined): string {
	if (s?.locked) return t('shell.settings.locked');
	if (s?.fixed) return t('shell.settings.fixed', { source: s.source ?? t('shell.settings.fixedOther') });
	return '';
}
