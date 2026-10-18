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
	kind: 'bool' | 'choice';
	label: string;
	help?: string;
	/** Choix d'un réglage 'choice' (valeur, libellé). */
	choices?: { value: string; label: string }[];
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
				path: 'features.htmlTabs',
				kind: 'bool',
				label: 'Pages web',
				help: 'Permet d’ajouter des pages web : une page HTML, CSS et JavaScript qui lit et modifie le classeur, ou une page d’un site. Les classeurs qui en contiennent déjà s’ouvrent et s’utilisent normalement.'
			},
			{
				path: 'ai.enabled',
				kind: 'bool',
				label: 'Onglet IA (assistant)',
				help: 'Ajoute l’onglet IA, à gauche de SQL : un assistant qui aide à construire une analyse et agit sur le classeur. Il faut se connecter au compte du fournisseur, ou enregistrer une clé d’API personnelle, gardée dans le trousseau du système.'
			}
		]
	},
	{
		id: 'ai',
		title: 'Assistant IA',
		settings: [
			{
				path: 'ai.dataSharing',
				kind: 'choice',
				label: 'Données transmises à l’assistant',
				choices: [
					{ value: 'schema-only', label: 'Structure seulement (noms, types, nombre de lignes et de valeurs distinctes)' },
					{ value: 'sample', label: 'Structure et quelques lignes d’exemple' }
				],
				help: 'L’assistant ne reçoit jamais tout le contenu du classeur. Le fournisseur reçoit aussi vos messages.'
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
