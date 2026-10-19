// Réglages de l'utilisateur (Fichier › Paramètres), partagés par toute
// l'interface : chargés au démarrage, rechargés après chaque modification.
// Les composants lisent userSettings.list : un réglage changé dans la
// fenêtre des paramètres s'applique tout de suite, sans redémarrer.
import { DocumentService } from '#bindings/internal/bridge/index.js';
import { isWeb } from '#lib/web/platform.js';
import { boolSetting, type SettingState } from './settings';

export const userSettings = $state<{ list: SettingState[] }>({ list: [] });

/**
 * L'hôte (DocumentService.Platform) : version web, scripting permis. En
 * web, le scripting (Lua, boutons, pages web) est coupé tant que le serveur
 * ne l'a pas autorisé (web_scripting) ; le serveur MCP local n'existe pas.
 */
export const platform = $state({ web: isWeb, scripting: !isWeb });

/** Charge les réglages (silencieux en cas d'échec : valeurs par défaut). */
export async function loadSettings() {
	try {
		const p = await DocumentService.Platform();
		if (p) Object.assign(platform, p);
	} catch {
		// Pas de pont (tests) : valeurs de la construction.
	}
	try {
		userSettings.list = (await DocumentService.Settings()) ?? [];
	} catch {
		// Pas de pont (tests) : les valeurs par défaut s'appliquent.
	}
}

/** Écrit un réglage ; lève l'erreur du serveur (verrouillé, invalide…). */
export async function setSetting(path: string, value: unknown) {
	userSettings.list = (await DocumentService.SetSetting(path, value)) ?? [];
}

/** Les pages web sont proposés (ajout, « Site web… »). */
export function htmlTabsEnabled(): boolean {
	return platform.scripting && boolSetting(userSettings.list, 'features.htmlTabs');
}

/** Lua, boutons d'action et pages web sont permis (toujours sur le bureau). */
export function scriptingEnabled(): boolean {
	return platform.scripting;
}

/** Le serveur MCP local est proposé : bureau seulement, et assistant IA
 * activé (le couper coupe aussi le serveur des assistants externes). */
export function mcpAvailable(): boolean {
	return platform.scripting && !platform.web && aiEnabled();
}

/** Classeurs partagés (Serveur Lattis) activés : menu Compte, connexions lattis. */
export function sharedWorkbooksEnabled(): boolean {
	return boolSetting(userSettings.list, 'features.sharedWorkbooks', true);
}

/** L'Assistant IA est activé (opt-in, coupé par défaut). */
export function aiEnabled(): boolean {
	return boolSetting(userSettings.list, 'ai.enabled');
}
