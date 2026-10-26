// Réglages de l'utilisateur (Fichier › Paramètres), partagés par toute
// l'interface : chargés au démarrage, rechargés après chaque modification.
// Les composants lisent userSettings.list : un réglage changé dans la
// fenêtre des paramètres s'applique tout de suite, sans redémarrer.
import { DocumentService } from '#bindings/internal/bridge/index.js';
import { isWeb } from '#lib/web/platform.js';
import { languageFor, setLang, setSystemLocale } from '#lib/i18n/index.js';
import { boolSetting, findSetting, htmlPagesOn, resetPaths, type SettingState } from './settings';
import { demoClock, type DemoClock } from './demoend';

export const userSettings = $state<{ list: SettingState[] }>({ list: [] });

/**
 * L'hôte (DocumentService.Platform) : version web, scripting permis. En
 * web, le scripting (Lua, boutons, pages web) est coupé tant que le serveur
 * ne l'a pas autorisé (web_scripting) ; le serveur MCP local n'existe pas.
 * demo : démo publique (lattis-engine --public-demo), jetable : ni Serveur
 * Lattis, ni connexion sortante, ni assistant ; Enregistrer télécharge.
 * htmlPages : pages web permises (features.htmlPages, jamais en web).
 * demoDeadline, demoWarnBefore, now : fin de la démo publique annoncée par
 * lattisd, son préavis et l'heure du moteur (ms) ; voir demoEnd.
 */
export const platform = $state<{ web: boolean; scripting: boolean; locale?: string; demo?: boolean; htmlPages?: boolean; demoDeadline?: number; demoWarnBefore?: number; now?: number }>({ web: isWeb, scripting: !isWeb });

/** Fin de la démo publique rapportée à l'horloge du navigateur (null : pas de fin annoncée). */
export const demoEnd = $state<{ clock: DemoClock | null }>({ clock: null });

/** Charge les réglages (silencieux en cas d'échec : valeurs par défaut). */
export async function loadSettings() {
	try {
		const p = await DocumentService.Platform();
		if (p) {
			Object.assign(platform, p);
			demoEnd.clock = demoClock(p, Date.now());
		}
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
 * Les pages web sont permises (htmlPagesOn) : ajout, affichage et
 * « Site web… ». Coupées (par défaut), un onglet page existant montre
 * seulement qu'elles sont désactivées, sans exécuter la page.
 */
export function htmlPagesEnabled(): boolean {
	return htmlPagesOn(platform);
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

/** Analyse de scénarios activée (opt-in, features.whatIf) : Données › Prévision. */
export function whatIfEnabled(): boolean {
	return boolSetting(userSettings.list, 'features.whatIf');
}

/** Création de graphiques sparkline activée (opt-in, features.sparklines) : ceux d'un classeur restent dessinés. */
export function sparklinesEnabled(): boolean {
	return boolSetting(userSettings.list, 'features.sparklines');
}

/** L'Assistant IA est activé (opt-in, coupé par défaut ; jamais dans la démo publique). */
export function aiEnabled(): boolean {
	return !platform.demo && boolSetting(userSettings.list, 'ai.enabled');
}

/** Sources jointes par le réseau (bases de données, API web) : pas dans la démo publique. */
export function networkSourcesEnabled(): boolean {
	return !platform.demo;
}
