// Thème de couleur appliqué (Affichage › Thème) : la grille le lit
// (tableTheme.current), le reste de l'interface par les variables CSS
// --lattis-table… et --lattis-source… de la racine du document ; les
// couleurs de style prises dans le thème se résolvent par paint() (colors.ts).
import { setPaintColors } from './colors';
import { resolveTheme, themeVars, type ResolvedTheme, type WorkbookTheme } from './theme';
import type { SettingState } from '#lib/shell/settings.js';

export const tableTheme = $state<{ current: ResolvedTheme }>({ current: resolveTheme(null, []) });

/** Applique le thème du classeur (ou le défaut des réglages) : grille redessinée, variables CSS posées. */
export function applyTheme(wb: WorkbookTheme | null | undefined, list: readonly SettingState[], root: HTMLElement | undefined = globalThis.document?.documentElement) {
	const t = resolveTheme(wb, list);
	tableTheme.current = t;
	setPaintColors(t.colors);
	if (root) for (const [k, v] of Object.entries(themeVars(t))) root.style.setProperty(k, v);
	return t;
}
