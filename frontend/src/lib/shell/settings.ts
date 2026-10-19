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
	kind: 'bool' | 'choice' | 'text';
	label: string;
	help?: string;
	/** Exemple affiché dans un réglage 'text' vide. */
	placeholder?: string;
	/** Choix d'un réglage 'choice' (valeur, libellé). */
	choices?: { value: string; label: string }[];
	/** Réglage d'une fonctionnalité à activer : masqué tant qu'elle ne l'est pas. */
	requires?: string;
}

export interface SettingSection {
	id: string;
	title: string;
	settings: SettingDef[];
	/** Section d'une fonctionnalité à activer (ai.enabled…) : masquée tant qu'elle ne l'est pas. */
	requires?: string;
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
				help: "Permet d'ajouter des pages web : une page HTML, CSS et JavaScript qui lit et modifie le classeur, ou une page d'un site. Les classeurs qui en contiennent déjà s'ouvrent et s'utilisent normalement."
			},
			{
				path: 'features.sharedWorkbooks',
				kind: 'bool',
				label: 'Classeurs partagés (Serveur Lattis)',
				help: "Permet de se connecter à un Serveur Lattis de votre organisation (menu Compte de la barre de titre) et de l'ajouter comme source de données. Les classeurs partagés y vivent ; la connexion se fait par votre compte (Google, Microsoft, Keycloak…) ou un mot de passe Lattis."
			},
			{
				path: 'ai.enabled',
				kind: 'bool',
				label: 'Assistant IA',
				help: "Le bouton « Assistant ✦ », en haut à droite du ruban (Alt+I), ouvre un volet à droite de la grille : un assistant qui aide à construire une analyse et agit sur le classeur, chaque réponse s'annulant d'un coup. Il faut une clé d'API d'un fournisseur (Anthropic, Mistral, OpenAI…), gardée dans le trousseau du système, un modèle installé sur ce poste, ou Claude Desktop avec un abonnement Claude."
			}
		]
	},
	{
		id: 'ai',
		title: 'Assistant IA',
		requires: 'ai.enabled',
		settings: [
			{
				path: 'ai.dataSharing',
				kind: 'choice',
				label: "Données transmises à l'assistant",
				choices: [
					{ value: 'schema-only', label: 'Structure seulement (noms, types, nombre de lignes et de valeurs distinctes)' },
					{ value: 'sample', label: "Structure et quelques lignes d'exemple (20 au plus par lecture)" },
					{ value: 'full', label: "Tout le classeur (l'assistant peut lire toutes les valeurs)" }
				],
				help: 'Vaut aussi pour Claude Desktop et Claude Code par le serveur MCP. Le fournisseur reçoit en plus vos messages, la feuille active et la sélection ; avec un modèle local, rien ne quitte le poste.'
			},
			{
				path: 'ai.keepConversation',
				kind: 'bool',
				label: 'Garder la conversation dans le classeur',
				help: "La conversation est enregistrée avec le classeur et réapparaît à sa prochaine ouverture : elle voyage avec le fichier, y compris les résultats de requêtes montrés et les lignes lues par l'assistant. Elle n'est enregistrée que si le classeur a aussi été modifié. Sinon, elle s'efface quand on ferme le classeur ou qu'on en ouvre un autre."
			},
			{
				path: 'ai.localUrl',
				kind: 'text',
				label: 'Adresse du modèle local',
				placeholder: 'http://localhost:11434/v1',
				help: "Fournisseur « Modèle local » : l'adresse d'Ollama, LM Studio ou d'un autre serveur compatible OpenAI sur ce poste. Aucune donnée ne quitte l'ordinateur."
			}
		]
	}
];

/**
 * Sections et réglages à montrer (choix de Louis) : ceux d'une fonctionnalité
 * à activer restent cachés tant qu'elle n'est pas cochée, puisqu'ils ne
 * servent à rien.
 */
export function visibleSections(list: readonly SettingState[]): SettingSection[] {
	const on = (path?: string) => !path || boolSetting(list, path);
	return settingSections.filter((s) => on(s.requires)).map((s) => ({ ...s, settings: s.settings.filter((d) => on(d.requires)) }));
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
	if (s?.locked) return '🔒 Imposé par la stratégie de votre organisation.';
	if (s?.fixed) return `Fixé par ${s.source ?? 'une autre configuration'} : le changer ici est sans effet.`;
	return '';
}
