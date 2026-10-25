import { describe, expect, it } from 'vitest';
import { allGenerated, isDefaultTableName, isGeneratedColumnName } from './defaultname';

describe('nom de tableau par défaut', () => {
	it('noms donnés à la création', () => {
		expect(isDefaultTableName('Tableau1')).toBe(true);
		expect(isDefaultTableName('Tableau12')).toBe(true);
		expect(isDefaultTableName('Feuille3')).toBe(true); // premier tableau d'une feuille ajoutée
		// Noms par défaut des autres langues (classeur créé en anglais, en allemand…).
		for (const n of ['Table1', 'Sheet2', 'Tabelle3', 'Tabla1', 'Hoja2', '表1']) expect(isDefaultTableName(n)).toBe(true);
		expect(isDefaultTableName('Requête2')).toBe(true); // résultat d'une requête
		expect(isDefaultTableName('tableau1')).toBe(true);
	});
	it('noms choisis', () => {
		expect(isDefaultTableName('Clients')).toBe(false);
		expect(isDefaultTableName('Tableau')).toBe(false);
		expect(isDefaultTableName('Tableau1_2024')).toBe(false);
		expect(isDefaultTableName('MonTableau1')).toBe(false);
		expect(isDefaultTableName('Ventes2024')).toBe(false);
	});
	it('tableaux croisés et vide', () => {
		expect(isDefaultTableName('TCD1')).toBe(false);
		expect(isDefaultTableName('')).toBe(false);
		expect(isDefaultTableName(undefined)).toBe(false);
	});
});

describe('nom de colonne donné par Lattis', () => {
	it('« Colonne n »', () => {
		expect(isGeneratedColumnName('Colonne 1')).toBe(true);
		expect(isGeneratedColumnName('Colonne 12')).toBe(true);
	});
	it('noms choisis', () => {
		expect(isGeneratedColumnName('Colonne')).toBe(false);
		expect(isGeneratedColumnName('Colonne A')).toBe(false);
		expect(isGeneratedColumnName('Colonne 1 bis')).toBe(false);
		expect(isGeneratedColumnName('colonne 1')).toBe(false);
		expect(isGeneratedColumnName('Nom')).toBe(false);
		expect(isGeneratedColumnName('')).toBe(false);
		expect(isGeneratedColumnName(undefined)).toBe(false);
	});
	it('tableau sans en-têtes', () => {
		expect(allGenerated(['Colonne 1', 'Colonne 2'])).toBe(true);
		expect(allGenerated(['Colonne 1', 'Ville'])).toBe(false);
		expect(allGenerated([])).toBe(false);
	});
});
