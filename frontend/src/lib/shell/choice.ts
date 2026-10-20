// Onglet à choix (docs/conception/regrouper-onglets-semblables.md §6.3) :
// libellé de l'onglet (« Ventes · 2025 ▾ »), entrées de sa liste
// (« (Toutes les années) », « + Nouvelle année… »), valeur suivante proposée
// (recopie en série), valeur voisine (Ctrl+Alt+Pg.Suiv / Pg.Préc) et
// colonnes de catégorie masquées dans la grille.

import { fillSeries } from '#lib/grid/fill.js';
import type { RowFormulaHelp, RowFormulaPreview } from '#lib/ui/rowformula.js';
import { t } from '#lib/i18n/index.js';

/** Onglet à choix d'une feuille (canvas.SheetChoice). */
export interface TabChoice {
	/** Nom de la catégorie : celui de sa colonne (il suit un renommage). */
	name: string;
	value: string;
	/** « (Toutes) » : aucun filtre. */
	all: boolean;
	type?: string;
	/** Catégorie calculée par une formule (=ANNEE([Date])) : pas de colonne. */
	formula?: string;
}


/** Actions de l'onglet à choix, fournies par la page (DocumentService). */
export interface ChoiceActions {
	/** Valeurs de la liste (count : lignes ; 0 pour une valeur ajoutée sans ligne). */
	values(sheet: number): Promise<{ text: string; count: number }[]>;
	/** Règle l'onglet sur une valeur (all : toutes) ; de la navigation. */
	choose(sheet: number, value: string, all: boolean): unknown;
	/** « Nouvelle année… » : ajoute la valeur et l'affiche. */
	add(sheet: number, value: string): unknown;
	/** « Choisir par… » : colonnes proposées, puis le choix d'une colonne. */
	candidates(sheet: number): Promise<{ table: string; column: string; values: number }[]>;
	by(sheet: number, table: string, column: string): unknown;
	/** « Choisir par › Une formule… » : aperçu, puis la catégorie calculée. */
	preview(sheet: number, formula: string): Promise<RowFormulaPreview>;
	byFormula(sheet: number, name: string, formula: string): Promise<unknown>;
	/** Fonctions et colonnes proposées à la saisie de la formule. */
	formulaHelp?(sheet: number): RowFormulaHelp;
	/** Retire l'onglet à choix. */
	clear(sheet: number): unknown;
}

/**
 * Catégories courantes : pluriel et genre, pour les entrées de la liste.
 * Ce sont des noms de colonnes français (tapés par l'utilisateur), accordés
 * en français ; un autre nom prend l'entrée générique, traduite.
 */
const KNOWN: Record<string, { all: string; add: string }> = {
	année: { all: 'Toutes les années', add: 'Nouvelle année…' },
	annee: { all: 'Toutes les années', add: 'Nouvelle année…' },
	mois: { all: 'Tous les mois', add: 'Nouveau mois…' },
	trimestre: { all: 'Tous les trimestres', add: 'Nouveau trimestre…' },
	semaine: { all: 'Toutes les semaines', add: 'Nouvelle semaine…' },
	date: { all: 'Toutes les dates', add: 'Nouvelle date…' },
	jour: { all: 'Tous les jours', add: 'Nouveau jour…' },
	ville: { all: 'Toutes les villes', add: 'Nouvelle ville…' },
	agence: { all: 'Toutes les agences', add: 'Nouvelle agence…' },
	client: { all: 'Tous les clients', add: 'Nouveau client…' },
	région: { all: 'Toutes les régions', add: 'Nouvelle région…' },
	pays: { all: 'Tous les pays', add: 'Nouveau pays…' },
	exercice: { all: 'Tous les exercices', add: 'Nouvel exercice…' },
	lot: { all: 'Tous les lots', add: 'Nouveau lot…' },
	catégorie: { all: 'Toutes les catégories', add: 'Nouvelle catégorie…' }
};

function known(name: string) {
	return KNOWN[name.trim().toLowerCase()];
}

/** Libellé de l'onglet : « Ventes · 2025 », « Ventes · Année » sur (Toutes). */
export function choiceLabel(sheet: string, c: TabChoice): string {
	return `${sheet} · ${c.all ? c.name : c.value || t('shell.choice.empty')}`;
}

/** Entrée « (Toutes les années) » de la liste. */
export function allLabel(name: string): string {
	return `(${known(name)?.all ?? t('shell.choice.allValues', { name })})`;
}

/** Entrée « + Nouvelle année… » de la liste. */
export function newLabel(name: string): string {
	return known(name)?.add ?? t('shell.choice.newValue');
}

/** Rappel pour la barre d'état et l'infobulle : « Année : 2025 (choix de l'onglet) ». */
export function choiceStatus(c: TabChoice, hidden = 0): string {
	const v = c.all ? allLabel(c.name) : c.value;
	const status = t('shell.choice.status', { name: c.name, value: v });
	return hidden > 0 ? t('shell.choice.statusHidden', { status, n: hidden }) : status;
}

const DAY1 = /^01\/(\d{2})\/(\d{4})$/;

/**
 * Valeur proposée par « Nouvelle année… » : la suivante de la série des
 * valeurs (dans l'ordre de la liste), comme la recopie en série (2026 après
 * 2025, Décembre après Novembre, Lot 4 après Lot 3, 01/02/2026 après
 * 01/01/2026) ; vide si rien ne se devine ou si elle existe déjà.
 */
export function nextChoice(values: readonly string[]): string {
	const vs = values.filter((v) => v.trim() !== '');
	if (vs.length === 0) return '';
	const last = vs[vs.length - 1];
	let next = '';
	// Premiers du mois : mois suivant (la recopie compterait des jours).
	const m = DAY1.exec(last.trim());
	if (m && vs.slice(-2).every((v) => DAY1.test(v.trim()))) {
		let mo = Number(m[1]) + 1;
		let y = Number(m[2]);
		if (mo > 12) {
			mo = 1;
			y++;
		}
		next = `01/${String(mo).padStart(2, '0')}/${y}`;
	} else if (vs.length === 1 && /^-?\d+$/.test(last.trim())) {
		next = String(Number(last.trim()) + 1); // une seule année : la suivante
	} else {
		next = fillSeries(vs.slice(-2), 1)[0] ?? '';
	}
	return next && !vs.includes(next) ? next : '';
}

/**
 * Valeur voisine (Ctrl+Alt+Pg.Suiv : d = 1, Pg.Préc : d = -1) : la suivante
 * ou la précédente de la liste ; depuis « (Toutes) », la première ou la
 * dernière. null au bout de la liste.
 */
export function choiceStep(values: readonly string[], c: TabChoice, d: 1 | -1): string | null {
	if (values.length === 0) return null;
	if (c.all) return d > 0 ? values[0] : values[values.length - 1];
	const i = values.indexOf(c.value);
	const j = i < 0 ? (d > 0 ? 0 : values.length - 1) : i + d;
	return j >= 0 && j < values.length ? values[j] : null;
}

/** Tableau de la feuille, vu par choiceHiddenCols. */
export interface ChoiceTable {
	row: number;
	col: number;
	rows: number;
	hidden?: number;
	columns: unknown[];
	/** Le tableau suit le choix de l'onglet ; choiceCol : rang de sa colonne de catégorie. */
	follows?: boolean;
	choiceCol?: number;
	/** Il suit une catégorie calculée : pas de colonne à masquer. */
	byFormula?: boolean;
}

/**
 * Colonnes de la feuille masquées par l'onglet à choix : quand une seule
 * valeur est choisie, la colonne de catégorie vaut la même chose partout et
 * n'est pas affichée (elle réapparaît avec « (Toutes) »). Une colonne de la
 * feuille ne se masque que si rien d'autre ne l'occupe (un tableau qui ne
 * suit pas, une autre colonne d'un tableau, une cellule libre).
 */
export function choiceHiddenCols(choice: TabChoice | null | undefined, tables: readonly ChoiceTable[], cells: readonly { col: number }[] = []): Set<number> {
	const out = new Set<number>();
	if (!choice || choice.all) return out;
	for (const t of tables) if (t.follows && !t.byFormula && t.choiceCol !== undefined) out.add(t.col + t.choiceCol);
	for (const c of [...out]) {
		const busy =
			cells.some((x) => x.col === c) ||
			tables.some((t) => c >= t.col && c < t.col + t.columns.length && !(t.follows && !t.byFormula && t.col + (t.choiceCol ?? -1) === c));
		if (busy) out.delete(c);
	}
	return out;
}
