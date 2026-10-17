import { describe, expect, it } from 'vitest';
import { isDefaultTableName } from './defaultname';

describe('nom de tableau par défaut', () => {
	it('noms donnés à la création', () => {
		expect(isDefaultTableName('Tableau1')).toBe(true);
		expect(isDefaultTableName('Tableau12')).toBe(true);
		expect(isDefaultTableName('Feuille3')).toBe(true); // premier tableau d'une feuille ajoutée
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
