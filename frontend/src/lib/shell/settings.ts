// Fichier › Paramètres : la liste déclarative des réglages affichés, par
// section. Chaque réglage est une clé de la configuration (config.json de
// l'utilisateur) que le serveur accepte d'écrire (config.UserEditable).
// Ajouter un réglage : une entrée ici, et sa clé dans UserEditable.

/** État d'un réglage rendu par DocumentService.Settings. */
export interface SettingState {
	path: string;
	value: unknown;
	locked?: boolean;
	fixed?: boolean;
	source?: string;
}

export interface SettingDef {
	path: string;
	kind: 'bool';
	label: string;
	help?: string;
}

export interface SettingSection {
	id: string;
	title: string;
	settings: SettingDef[];
}

export const settingSections: SettingSection[] = [
	{
		id: 'features',
		title: 'Fonctionnalités',
		settings: [
			{
				path: 'features.webPages',
				kind: 'bool',
				label: 'Pages web externes dans les Onglets HTML',
				help: 'Permet d’afficher une page d’un site dans un Onglet HTML. Les classeurs qui en contiennent déjà s’ouvrent normalement.'
			}
		]
	}
];

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
	if (s?.locked) return '🔒 Imposé par la stratégie de votre organisation.';
	if (s?.fixed) return `Fixé par ${s.source ?? 'une autre configuration'} : le changer ici est sans effet.`;
	return '';
}
