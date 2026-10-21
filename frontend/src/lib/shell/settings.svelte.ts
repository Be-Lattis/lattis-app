// Réglages de l'utilisateur (Fichier › Paramètres), partagés par toute
// l'interface : chargés au démarrage, rechargés après chaque modification.
// Les composants lisent userSettings.list : un réglage changé dans la
// fenêtre des paramètres s'applique tout de suite, sans redémarrer.
import { DocumentService } from '#bindings/internal/bridge/index.js';
import { isWeb } from '#lib/web/platform.js';
import { languageFor, setLang, setSystemLocale } from '#lib/i18n/index.js';
import { boolSetting, findSetting, resetPaths, type SettingState } from './settings';

export const userSettings = $state<{ list: SettingState[] }>({ list: [] });

/**
 * L'hôte (DocumentService.Platform) : version web, scripting permis. En
 * web, le scripting (Lua, boutons, pages web) est coupé tant que le serveur
 * ne l'a pas autorisé (web_scripting) ; le serveur MCP local n'existe pas.
 * demo : démo publique (lattis-engine --public-demo), jetable : ni Serveur
 * Lattis, ni connexion sortante, ni assistant ; Enregistrer télécharge.
 */
export const platform = $state<{ web: boolean; scripting: boolean; locale?: string; demo?: boolean }>({ web: isWeb, scripting: !isWeb });

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
	await applyLanguage();
}

/** Écrit un réglage ; lève l'erreur du serveur (verrouillé, invalide…). */
export async function setSetting(path: string, value: unknown) {
	userSettings.list = (await DocumentService.SetSetting(path, value)) ?? [];
	if (path === 'ui.language') await applyLanguage();
}

/**
 * Remet les réglages de la fenêtre à leur valeur par défaut (ceux imposés
 * par l'organisation restent) ; langue et thème suivent sans redémarrer.
 */
export async function resetSettings() {
	userSettings.list = (await DocumentService.ResetSettings(resetPaths())) ?? [];
	await applyLanguage();
}

/**
 * Applique la langue de l'interface (réglage ui.language, auto : celle du
 * système sur le bureau, du navigateur en web) : textes et formats changent
 * sans redémarrer, et le moteur la reçoit pour ses messages et la locale des
 * formules (DocumentService.SetLanguage).
 */
async function applyLanguage() {
	setSystemLocale(platform.locale);
	const l = languageFor(findSetting(userSettings.list, 'ui.language')?.value);
	await setLang(l);
	try {
		await DocumentService.SetLanguage(l);
	} catch {
		// Pas de pont (tests).
	}
}

/**
 * Les pages web sont proposées (ajout, « Site web… »), avec ou sans
 * scripting : sans scripting (version web sans web_scripting, démo
 * publique), une page ne fait que lire le classeur (SQL en lecture seule,
 * requêtes favorites, DocumentService.PageCall). Les pages déjà posées
 * s'affichent dans tous les cas.
 */
export function htmlTabsEnabled(): boolean {
	return boolSetting(userSettings.list, 'features.htmlTabs');
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
	return !platform.demo && boolSetting(userSettings.list, 'features.sharedWorkbooks', true);
}

/** L'Assistant IA est activé (opt-in, coupé par défaut ; jamais dans la démo publique). */
export function aiEnabled(): boolean {
	return !platform.demo && boolSetting(userSettings.list, 'ai.enabled');
}

/** Sources jointes par le réseau (bases de données, API web) : pas dans la démo publique. */
export function networkSourcesEnabled(): boolean {
	return !platform.demo;
}
