// « Filtrer l'onglet par cette colonne » (menus d'une colonne de tableau) et
// « Filtrer l'onglet par ce champ » (menus d'un champ de tableau croisé) :
// l'entrée de menu qui fait de la feuille un onglet à choix par cette
// colonne (comme « Choisir par… » de l'onglet, sans fusionner d'onglets), ou
// qui le retire quand l'onglet filtre déjà par elle. Mêmes conditions que
// « Choisir par… » : une colonne saisie (pas calculée), avec au moins une
// valeur et au plus MAX_CHOICE_VALUES valeurs différentes.

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
	const what = o.field ? 'ce champ' : 'cette colonne';
	if (o.choice && same(o.choice.name, o.column))
		return {
			label: "Ne plus filtrer l'onglet",
			title: `L'onglet perd son ▾ « ${o.choice.name} » : les tableaux et tableaux croisés de la feuille montrent de nouveau toutes leurs lignes (Ctrl+Z pour annuler)`,
			disabled: false,
			clear: true
		};
	const label = `Filtrer l'onglet par ${what}`;
	const no = (title: string): TabChoiceItem => ({ label, title, disabled: true, clear: false });
	if (o.formula) return no("Une colonne calculée ne peut pas servir de choix d'onglet : prenez une colonne saisie (Année, Mois, Ville…)");
	if (o.candidates) {
		const c = o.candidates.find((x) => same(x.table, o.table) && same(x.column, o.column));
		if (!c) return no(`« ${o.column} » est vide, ou a plus de ${MAX_CHOICE_VALUES} valeurs différentes : trop pour la liste de l'onglet ; prenez une colonne comme Année, Mois ou Ville`);
	}
	const follow = o.field
		? `ce tableau croisé, les autres tableaux croisés de la feuille construits sur un tableau qui a « ${o.column} » et les tableaux de la feuille qui l'ont ne montrent que les lignes de cette valeur (le tableau source, s'il est sur une autre feuille, n'est pas filtré)`
		: `chaque tableau de la feuille qui a la colonne « ${o.column} », et les tableaux croisés construits sur eux, ne montrent que les lignes de cette valeur`;
	const replace = o.choice ? ` Remplace le choix par « ${o.choice.name} ».` : '';
	return {
		label,
		title: `L'onglet de la feuille reçoit un ▾ pour choisir une valeur de « ${o.column} » : ${follow}. Il commence sur toutes les valeurs.${replace} Ctrl+Z pour annuler.`,
		disabled: false,
		clear: false
	};
}

/** Message de la barre d'état après « Filtrer l'onglet par… ». */
export function tabChoiceNotice(column: string, clear: boolean): string {
	return clear
		? `L'onglet ne filtre plus par « ${column} » : toutes les lignes sont de nouveau affichées (Ctrl+Z pour annuler)`
		: `L'onglet filtre par « ${column} » : sa flèche ▾ choisit la valeur affichée (Alt+↓) ; Ctrl+Z pour annuler`;
}
