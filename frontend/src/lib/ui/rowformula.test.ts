import { describe, expect, it } from 'vitest';
import { guessCategoryName, previewText, rowSuggest } from './rowformula';

describe('formule par ligne : nom proposé', () => {
	it('devine la catégorie des fonctions de date', () => {
		expect(guessCategoryName('=ANNEE([Date])')).toBe('Année');
		expect(guessCategoryName('=année([Date])')).toBe('Année');
		expect(guessCategoryName('=MOIS([Date])')).toBe('Mois');
		expect(guessCategoryName('=TEXTE([Date];"mmmm")')).toBe('Mois');
		expect(guessCategoryName('=TEXTE([Date];"aaaa")')).toBe('Année');
		expect(guessCategoryName('=NO.SEMAINE([Date])')).toBe('Semaine');
	});
	it('reprend la colonne d’une fonction de texte', () => {
		expect(guessCategoryName('=GAUCHE([Code];2)')).toBe('Code');
		expect(guessCategoryName('=MAJUSCULE([@Ville])')).toBe('Ville');
	});
	it('Catégorie sinon', () => {
		expect(guessCategoryName('=SI([Montant]>1000;"Gros";"Petit")')).toBe('Catégorie');
		expect(guessCategoryName('')).toBe('Catégorie');
	});
});

describe('formule par ligne : saisie', () => {
	const help = {
		functions: [
			{ name: 'ANNEE', other: 'YEAR', arity: '1' },
			{ name: 'ARRONDI', other: 'ROUND', arity: '2' }
		],
		columns: ['Date', 'Montant']
	};
	it('propose les colonnes après [', () => {
		const c = rowSuggest('=ANNEE([D', 9, help);
		expect(c?.items.map((i) => i.insert)).toEqual(['Date]']);
		expect(c?.from).toBe(8);
	});
	it('propose les fonctions, même sans =', () => {
		expect(rowSuggest('=AN', 3, help)?.items.map((i) => i.label)).toEqual(['ANNEE']);
		const c = rowSuggest('AN', 2, help);
		expect(c?.items.map((i) => i.label)).toEqual(['ANNEE']);
		expect(c?.from).toBe(0);
	});
	it('rien sans aide', () => {
		expect(rowSuggest('=AN', 3, undefined)).toBeNull();
	});
});

describe('formule par ligne : aperçu', () => {
	it('liste les valeurs, et leur nombre au-delà', () => {
		expect(previewText([{ text: '2024' }, { text: '2025' }], 2)).toBe('2024, 2025');
		expect(previewText([{ text: 'A' }, { text: 'B' }], 1200)).toBe('A, B… (1 200 valeurs)');
		expect(previewText([], 0)).toMatch(/^Aucune valeur/);
	});
});
