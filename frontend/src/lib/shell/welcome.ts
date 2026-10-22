// Accueil sans classeur ouvert : premiers pas et classeur d'exemple
// (audit 6, I8).
import { LANG_NAMES, LANGS, type LanguageSetting, systemLang, t } from '#lib/i18n/index.js';

export type DemoProgress = { done: number; total: number; cells: number; totalCells: number };


/** Avancement de la fabrication d'une démonstration ou de l'exemple. */
export function demoProgressText(label: string, p: DemoProgress | null): string {
	if (!p || p.total <= 0) return t('shell.welcome.progressStart', { label });
	return p.totalCells
		? t('shell.welcome.progressCells', { label, cells: p.cells, totalCells: p.totalCells })
		: t('shell.welcome.progress', { label, done: p.done, total: p.total });
}

/** Ce qui fait la différence de Lattis, en trois points (cible : page d'aide). */
// Titre et texte lus dans la langue courante à chaque affichage (getters).
const step = (key: 'table' | 'formula' | 'sql', help: string) => ({
	get title() {
		return t(`shell.welcome.steps.${key}.title`);
	},
	get text() {
		return t(`shell.welcome.steps.${key}.text`);
	},
	help
});
export const firstSteps: { title: string; text: string; help: string }[] = [step('table', 'prise-en-main#tableaux-detectes'), step('formula', 'formules#colonnes-calculees'), step('sql', 'sql')];

/** Page d'aide « Prise en main » (accueil du premier lancement). */
export const guidePage = 'prise-en-main';

/**
 * Choix de la langue de l'accueil du premier lancement (réglage
 * ui.language) : « automatique » nommant la langue qu'il prend, puis chaque
 * langue écrite dans sa langue, comme Fichier › Paramètres.
 */
export function languageChoices(): { value: LanguageSetting; label: string }[] {
	return [{ value: 'auto', label: t('shell.settings.language.auto', { name: LANG_NAMES[systemLang()] }) }, ...LANGS.map((l) => ({ value: l, label: LANG_NAMES[l] }))];
}

/** Valeur du réglage ui.language à cocher (auto pour une valeur absente ou inconnue). */
export function languageValue(v: unknown): LanguageSetting {
	return typeof v === 'string' && (LANGS as readonly string[]).includes(v) ? (v as LanguageSetting) : 'auto';
}

const KEY = 'lattis.welcome.hidden';

/** Les premiers pas sont-ils affichés (pas masqués par « Ne plus afficher ») ? */
export function stepsShown(store: Pick<Storage, 'getItem'> | null = storage()): boolean {
	try {
		return store?.getItem(KEY) !== '1';
	} catch {
		return true;
	}
}

/** « Ne plus afficher » les premiers pas. */
export function hideSteps(store: Pick<Storage, 'setItem'> | null = storage()) {
	try {
		store?.setItem(KEY, '1');
	} catch {
		// stockage indisponible : masqué pour cette fois seulement
	}
}

function storage(): Storage | null {
	try {
		return typeof localStorage === 'undefined' ? null : localStorage;
	} catch {
		return null;
	}
}
