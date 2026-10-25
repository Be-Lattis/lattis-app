// Menus du clic droit de la grille : l'ordre de leurs entrées (demande de
// Louis). Partout le même raisonnement :
//   0. les actions propres à la situation (dernière ligne, première ligne,
//      en-tête avec du texte ou des groupes au-dessus, cellule libre près d'un
//      tableau, résultat déversé…), seulement quand il y en a ;
//   1. le presse-papiers (Couper, Copier, Coller) ;
//   2. effacer et mettre en forme (contenu, format de cellule, mise en forme
//      conditionnelle) ;
//   3. la note ;
//   4. un sous-menu par objet, titré de son nom : Trier et filtrer, Ligne
//      « … », Colonne « … », Tableau « … », Tableau croisé « … »,
//      Graphique « … », Bouton « … ».
// Chaque fonction rend des groupes d'identifiants d'entrées : Sheet.svelte
// les affiche dans cet ordre, un séparateur entre deux groupes ; les groupes
// vides disparaissent (pas de séparateur en trop).

export type MenuGroups = string[][];

type Entry = string | false | null | undefined;

/** Groupes sans les entrées absentes, sans les groupes vides. */
export function groups(...gs: Entry[][]): MenuGroups {
	return gs.map((g) => g.filter((x): x is string => typeof x === 'string' && x !== '')).filter((g) => g.length > 0);
}

/** Entrées d'un menu, dans l'ordre, séparateurs compris ('|'), pour les tests. */
export const flat = (gs: MenuGroups) => gs.map((g) => g.join(' ')).join(' | ');

/** Le presse-papiers, commun à tous les menus de cellules. */
const CLIPBOARD = ['cut', 'copy', 'paste', 'pasteSpecial'];

/** Cellule d'un tableau (données, en-tête ou bande des groupes). */
export interface TableCellMenu {
	/** Colonne de référence (Basculer sur…) ou clé citée : propre à la cellule cliquée. */
	ref?: boolean;
	/** 1re ligne d'un tableau sans en-têtes : « Utiliser la première ligne comme en-têtes ». */
	promote?: boolean;
	/** Dernière ligne de données : « Sortir cette ligne du tableau ». */
	toNote?: boolean;
	/** En-tête avec du texte au-dessus : « Utiliser la ligne du dessus comme en-têtes ». */
	toHeader?: boolean;
	/** En-tête avec du texte au-dessus : « En-têtes sur 2 lignes ». */
	groupsOn?: boolean;
	/** En-tête (ou bande) de colonnes groupées : « Retirer les groupes d'en-têtes ». */
	groupsOff?: boolean;
	format?: boolean;
	rules?: boolean;
	note?: boolean;
	/** Commentaire en fil : « Nouveau commentaire » ou « Répondre ». */
	comment?: boolean;
	/** Tri, filtre (au moins une entrée). */
	sort?: boolean;
	/** Ligne de données (pas l'en-tête) : sous-menu Ligne « … ». */
	row?: boolean;
	/** Sous-menu Colonne « … » (pas sur un numéro de ligne). */
	column?: boolean;
	/** Sous-menu Tableau « … ». */
	table?: boolean;
}

export function tableCellMenu(o: TableCellMenu): MenuGroups {
	return groups(
		[o.ref && 'ref', o.promote && 'promote', o.toNote && 'toNote', o.toHeader && 'toHeader', o.groupsOn && 'groupsOn', o.groupsOff && 'groupsOff'],
		CLIPBOARD,
		['clear', o.format && 'format', o.rules && 'rules'],
		[o.note && 'note', o.comment && 'comment'],
		[o.sort && 'sort', o.row && 'row', o.column && 'column', o.table && 'table']
	);
}

/** Sous-menu Ligne « … » d'une ligne de données de tableau. */
export function tableRowSubmenu(): MenuGroups {
	return groups(['insertAbove', 'insertBelow', 'deleteRow']);
}

/** Sous-menu Colonne « … » d'une colonne de tableau ; sa suppression en dernier. */
export function tableColumnSubmenu(o: { typeKey?: boolean; validation?: boolean; tabChoice?: boolean; toValues?: boolean }): MenuGroups {
	return groups(['rename', o.typeKey && 'typeKey', o.validation && 'validation', o.tabChoice && 'tabChoice', o.toValues && 'toValues'], ['insertLeft', 'insertRight', 'deleteColumn']);
}

/** Cellule libre (hors tableau). */
export interface FreeCellMenu {
	ref?: boolean;
	/** Cellule remplie au-dessus des colonnes d'un tableau : « Utiliser comme en-têtes de… ». */
	asHeader?: boolean;
	/** Cellule gardée hors du tableau voisin : « Ajouter au tableau ». */
	join?: boolean;
	/** Plage (tableau converti) : « Mettre sous forme de tableau ». */
	asTable?: boolean;
	/** Résultat déversé : « Convertir en tableau lié / (valeurs) ». */
	spill?: boolean;
	/** Formules dans la sélection : « Convertir en valeurs ». */
	toValues?: boolean;
	/** Ligne juste au-dessus de l'en-tête d'un tableau : « Intégrer comme groupes d'en-têtes ». */
	asGroups?: boolean;
	/** Graphique ou bouton copié : « Coller le graphique », « Coller le bouton ». */
	pasteChart?: boolean;
	pasteButton?: boolean;
	format?: boolean;
	rules?: boolean;
	note?: boolean;
	comment?: boolean;
	/** Insertion et suppression de lignes ou de colonnes de la feuille. */
	lines?: boolean;
	/** Sous-menu Tableau croisé « … ». */
	pivot?: boolean;
}

export function freeCellMenu(o: FreeCellMenu): MenuGroups {
	return groups(
		[o.ref && 'ref', o.asHeader && 'asHeader', o.join && 'join', o.asTable && 'asTable', o.spill && 'spillLinked', o.spill && 'spillValues', o.toValues && 'toValues', o.asGroups && 'asGroups'],
		[...CLIPBOARD, o.pasteChart && 'pasteChart', o.pasteButton && 'pasteButton'],
		['clear', o.format && 'format', o.rules && 'rules'],
		[o.note && 'note', o.comment && 'comment'],
		[o.lines && 'insertDialog', o.lines && 'clearAll'],
		[o.pivot && 'pivot']
	);
}

/** Numéro de ligne ou lettre de colonne (feuille ou ligne d'un tableau). */
export interface HeaderMenu {
	/** Situation (ligne d'un tableau) : sa 1re ligne sans en-têtes, sa dernière ligne. */
	promote?: boolean;
	toNote?: boolean;
	/** Ligne juste au-dessus de l'en-tête d'un tableau : « Intégrer comme groupes d'en-têtes ». */
	asGroups?: boolean;
	pasteChart?: boolean;
	pasteButton?: boolean;
	format?: boolean;
	rules?: boolean;
	/** Sous-menu Ligne « … » ou Colonne « … » (au moins une entrée). */
	lines?: boolean;
	/** Ligne d'un tableau : sous-menu Tableau « … ». */
	table?: boolean;
}

// Gouttière (numéro de ligne, lettre de colonne) : on vise presque toujours
// la ligne ou la colonne ; ses actions (insertion, taille, affichage) sont en
// tête du menu, après la situation, sans sous-menu (demande de Louis, 21/10).
// 'linesTop' : les groupes de linesSubmenu, montés tels quels.
export function headerMenu(o: HeaderMenu): MenuGroups {
	return groups(
		[o.promote && 'promote', o.toNote && 'toNote', o.asGroups && 'asGroups'],
		[o.lines && 'linesTop'],
		[...CLIPBOARD, o.pasteChart && 'pasteChart', o.pasteButton && 'pasteButton'],
		['clear', o.format && 'format', o.rules && 'rules'],
		[o.table && 'table']
	);
}

/** Sous-menu Ligne « … » ou Colonne « … » d'un numéro ou d'une lettre : insertion, taille, affichage. */
export function linesSubmenu(o: { insert?: boolean; size?: boolean; hide?: boolean; unhide?: boolean }): MenuGroups {
	return groups([o.insert && 'insert', o.insert && 'delete'], [o.size && 'size', o.size && 'autoFit'], [o.hide && 'hide', o.unhide && 'unhide']);
}

/** Sous-menu Tableau croisé « … » : développer, réduire ; source, déplacement ; sa suppression en dernier. */
export function pivotSubmenu(o: { fold?: boolean; source?: boolean; move?: boolean; remove?: boolean }): MenuGroups {
	return groups([o.fold && 'expand', o.fold && 'collapse'], [o.fold && 'expandAll', o.fold && 'collapseAll'], [o.source && 'source', o.move && 'move'], [o.remove && 'delete']);
}

/** Graphique : presse-papiers, puis son sous-menu. */
export function chartMenu(o: { image?: boolean }): MenuGroups {
	return groups(['cut', 'copy', o.image && 'copyImage', 'pasteHere'], ['chart']);
}

/** Sous-menu Graphique « … ». */
export function chartSubmenu(o: { edit?: boolean }): MenuGroups {
	return groups([o.edit && 'edit'], ['delete']);
}

/** Bouton ou contrôle : presse-papiers, puis son sous-menu. */
export function controlMenu(): MenuGroups {
	return groups(['cut', 'copy'], ['control']);
}

/** Sous-menu Bouton « … » (ou Case à cocher, Liste…). */
export function controlSubmenu(o: { run?: boolean }): MenuGroups {
	return groups([o.run && 'run', 'edit', 'resize'], ['delete']);
}
