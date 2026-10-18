// Réglages de l'utilisateur (Fichier › Paramètres), partagés par toute
// l'interface : chargés au démarrage, rechargés après chaque modification.
// Les composants lisent userSettings.list : un réglage changé dans la
// fenêtre des paramètres s'applique tout de suite, sans redémarrer.
import { DocumentService } from '#bindings/internal/bridge/index.js';
import { boolSetting, type SettingState } from './settings';

export const userSettings = $state<{ list: SettingState[] }>({ list: [] });

/** Charge les réglages (silencieux en cas d'échec : valeurs par défaut). */
export async function loadSettings() {
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

/** Un Onglet HTML peut être pointé vers une page web externe (« Page web… »). */
export function webPagesEnabled(): boolean {
	return boolSetting(userSettings.list, 'features.webPages');
}
