// « Filtrer l'onglet par cette colonne » (menus d'une colonne de tableau) et
// « Filtrer l'onglet par ce champ » (menus d'un champ de tableau croisé) :
// l'entrée de menu qui fait de la feuille un onglet à choix par cette
// colonne (comme « Choisir par… » de l'onglet, sans fusionner d'onglets), ou
// qui le retire quand l'onglet filtre déjà par elle. Mêmes conditions que
// « Choisir par… » : une colonne saisie (pas calculée), avec au moins une
// valeur et au plus MAX_CHOICE_VALUES valeurs différentes.
import { t } from '#lib/i18n/index.js';

/** Borne de « Choisir par… » (canvas.MaxChoiceValues). */
export const MAX_CHOICE_VALUES = 100;

/** Colonne proposée par « Choisir par… » (canvas.ChoiceCandidate). */
export interface ChoiceCandidate {
	table: string;
	column: string;
	values: number;
}

export interface TabChoiceItem {
	label: string;
	title: string;
	disabled: boolean;
	/** L'onglet filtre déjà par cette colonne : l'entrée le retire. */
	clear: boolean;
}

const same = (a: string, b: string) => a.trim().toLowerCase() === b.trim().toLowerCase();

/**
 * Entrée de menu pour la colonne column du tableau table (pour un champ de
 * tableau croisé, field : table est son tableau source). choice : l'onglet à
 * choix actuel de la feuille ; candidates : les colonnes que « Choisir
 * par… » propose (null : pas encore connues, l'entrée reste permise).
 */
export function tabChoiceItem(o: {
	table: string;
	column: string;
	formula?: boolean;
	field?: boolean;
	choice?: { name: string } | null;
	candidates?: readonly ChoiceCandidate[] | null;
}): TabChoiceItem {
	if (o.choice && same(o.choice.name, o.column))
		return {
			label: t('grid.tabChoice.clear'),
			title: t('grid.tabChoice.clear.tooltip', { name: o.choice.name }),
			disabled: false,
			clear: true
		};
	const label = o.field ? t('grid.tabChoice.byField') : t('grid.tabChoice.byColumn');
	const no = (title: string): TabChoiceItem => ({ label, title, disabled: true, clear: false });
	if (o.formula) return no(t('grid.tabChoice.computed'));
	if (o.candidates) {
		const c = o.candidates.find((x) => same(x.table, o.table) && same(x.column, o.column));
		if (!c) return no(t('grid.tabChoice.tooMany', { column: o.column, max: MAX_CHOICE_VALUES }));
	}
	const title = o.field ? t('grid.tabChoice.byField.tooltip', { column: o.column }) : t('grid.tabChoice.byColumn.tooltip', { column: o.column });
	return {
		label,
		title: o.choice ? t('grid.tabChoice.replaces', { text: title, name: o.choice.name }) : title,
		disabled: false,
		clear: false
	};
}

/** Message de la barre d'état après « Filtrer l'onglet par… ». */
export function tabChoiceNotice(column: string, clear: boolean): string {
	return clear
		? t('grid.tabChoice.notice.cleared', { column })
		: t('grid.tabChoice.notice.set', { column });
}
