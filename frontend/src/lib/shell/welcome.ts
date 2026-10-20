// Accueil sans classeur ouvert : premiers pas et classeur d'exemple
// (audit 6, I8).
import { t } from '#lib/i18n/index.js';

export type DemoProgress = { done: number; total: number; cells: number; totalCells: number };


/** Avancement de la fabrication d'une démonstration ou de l'exemple. */
export function demoProgressText(label: string, p: DemoProgress | null): string {
	if (!p || p.total <= 0) return t('shell.welcome.progressStart', { label });
	return p.totalCells ? t('shell.welcome.progressCells', { label, done: p.done, total: p.total, cells: p.cells, totalCells: p.totalCells }) : t('shell.welcome.progress', { label, done: p.done, total: p.total });
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
