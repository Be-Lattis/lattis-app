// Nom par défaut d'un tableau : celui que Lattis lui donne à la création
// (internal/canvas, tableName) tant qu'on ne l'a pas renommé. C'est
// « TableauN », ou le nom de la feuille quand le tableau en est le premier :
// « FeuilleN » pour une feuille ajoutée, « RequêteN » pour le résultat d'une
// requête. Ces noms ne disent rien du contenu : l'interface les montre en
// rouge pour inviter à renommer le tableau.
//
// Les tableaux croisés (« TCDn ») ne sont pas concernés : aucun écran ne
// permet de les renommer.
import { t } from '#lib/i18n/index.js';

const DEFAULT = /^(Tableau|Feuille|Requête)\d+$/i;

/** Couleur des noms par défaut : rouge discret, lisible sur fond blanc. */
export const DEFAULT_NAME_COLOR = '#c42b1c';

/** Info-bulle d'un nom par défaut, dans la langue de l'interface. */
export function defaultNameHint(): string {
	return t('grid.defaultName.hint');
}

/**
 * Info-bulle d'un nom par défaut, en français (lue au chargement : ne suit
 * pas la langue).
 * @deprecated defaultNameHint().
 */
export const DEFAULT_NAME_HINT = 'Nom par défaut : renommez le tableau';

/** Vrai si name est encore un nom de tableau donné automatiquement. */
export function isDefaultTableName(name: string | null | undefined): boolean {
	return !!name && DEFAULT.test(name.trim());
}

// Nom de colonne donné par Lattis (internal/canvas, autoColRe) : « Colonne n »
// (collage sans en-têtes, tableau tapé dont la 1re ligne contient un nombre
// ou une date, tableau vide d'Insertion › Tableau, colonne ajoutée sans
// nom). La grille l'écrit en gris italique ;
// taper dans l'en-tête le remplace.
const GENERATED_COLUMN = /^Colonne \d+$/;

/** Vrai si name est un nom de colonne donné automatiquement. */
export function isGeneratedColumnName(name: string | null | undefined): boolean {
	return !!name && GENERATED_COLUMN.test(name);
}

/** Vrai si toutes les colonnes ont un nom donné par Lattis : tableau sans en-têtes. */
export function allGenerated(names: readonly string[]): boolean {
	return names.length > 0 && names.every(isGeneratedColumnName);
}
