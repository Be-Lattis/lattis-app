// Formule par ligne (« Choisir par › Une formule… » d'un onglet, « Champ
// calculé… » d'un tableau croisé) : nom proposé d'après la formule,
// suggestions de fonctions et de colonnes à la saisie, résumé de l'aperçu.

import { suggest, type Catalog, type Completion, type FunctionDoc } from '#lib/grid/autocomplete.js';
import { fmtNumber } from '#lib/i18n/index.js';

/** Fonctions et colonnes proposées à la saisie. */
export interface RowFormulaHelp {
	functions: FunctionDoc[];
	columns: string[];
}

/** Tableau fictif qui porte les colonnes : [Col] y est la valeur de la ligne. */
const ROW = '\u0001';

/** Suggestions pour la formule (fonctions, [Colonne]) à la position du curseur. */
export function rowSuggest(text: string, caret: number, help: RowFormulaHelp | undefined): Completion | null {
	if (!help) return null;
	const cat: Catalog = { functions: help.functions, tables: [{ name: ROW, columns: help.columns }] };
	const c = suggest(text.trimStart().startsWith('=') ? text : '=' + text, caret + (text.trimStart().startsWith('=') ? 0 : 1), cat, ROW);
	if (!c || text.trimStart().startsWith('=')) return c;
	return { ...c, from: c.from - 1 };
}

const MONTH = /^m{3,4}$/i;
const YEAR = /^(a{2}|a{4}|y{2}|y{4})$/i;
const DAY = /^(j{3,4}|d{3,4})$/i;

/**
 * Nom proposé pour la catégorie d'après la formule : =ANNEE([Date]) → Année,
 * =TEXTE([Date];"mmmm") → Mois, =GAUCHE([Code];2) → Code ; Catégorie sinon.
 */
export function guessCategoryName(formula: string): string {
	const f = formula.trim().replace(/^=/, '').trim();
	const call = /^([\p{L}.]+)\s*\(/u.exec(f)?.[1]?.toUpperCase() ?? '';
	switch (call) {
		case 'ANNEE':
		case 'ANNÉE':
		case 'YEAR':
			return 'Année';
		case 'MOIS':
		case 'MONTH':
			return 'Mois';
		case 'JOUR':
		case 'DAY':
			return 'Jour';
		case 'JOURSEM':
		case 'WEEKDAY':
			return 'Jour';
		case 'NO.SEMAINE':
		case 'NO.SEMAINE.ISO':
		case 'WEEKNUM':
		case 'ISOWEEKNUM':
			return 'Semaine';
		case 'TEXTE':
		case 'TEXT': {
			const fmt = /"([^"]*)"\s*\)\s*$/.exec(f)?.[1] ?? '';
			if (MONTH.test(fmt)) return 'Mois';
			if (YEAR.test(fmt)) return 'Année';
			if (DAY.test(fmt)) return 'Jour';
			break;
		}
		case 'GAUCHE':
		case 'DROITE':
		case 'STXT':
		case 'MAJUSCULE':
		case 'MINUSCULE':
		case 'NOMPROPRE':
		case 'SUPPRESPACE':
		case 'LEFT':
		case 'RIGHT':
		case 'MID':
		case 'UPPER':
		case 'LOWER':
		case 'PROPER':
		case 'TRIM':
		case 'ARRONDI':
		case 'ROUND':
		case 'ENT':
		case 'INT': {
			const col = /^[^(]*\(\s*\[@?([^\]]+)\]/.exec(f)?.[1];
			if (col) return col.trim();
			break;
		}
	}
	return 'Catégorie';
}

/** Aperçu d'une formule par ligne (canvas.ChoicePreview). */
export interface RowFormulaPreview {
	values: { text: string; count: number }[];
	count: number;
	tables: string[];
	others: string[];
	error?: string;
}

/** Aperçu lu du moteur (listes nulles, erreur vide). */
export function previewOf(p: { values?: { text: string; count: number }[] | null; count?: number; tables?: string[] | null; others?: string[] | null; error?: string }): RowFormulaPreview {
	return { values: p.values ?? [], count: p.count ?? 0, tables: p.tables ?? [], others: p.others ?? [], error: p.error || undefined };
}

/** Résumé de l'aperçu : « 2024, 2025 » ou « A, B, C… (25 valeurs) ». */
export function previewText(values: readonly { text: string }[], count: number): string {
	if (count === 0) return 'Aucune valeur : la formule ne donne que des cellules vides';
	const list = values.map((v) => v.text).join(', ');
	return count > values.length ? `${list}… (${fmtNumber(count)} valeurs)` : list;
}
