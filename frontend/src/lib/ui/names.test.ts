import { describe, expect, it } from 'vitest';
import { a1Name, cellNameProblem, columnNameProblem, favoriteNameProblem, refusedName, scriptNameProblem, sheetNameProblem, tableNameProblem } from './names.js';

describe('tableNameProblem', () => {
	// Mêmes cas que internal/canvas/names_rules_test.go (RenameTable).
	it.each(['Ventes', 'Ventes_2024', '_brouillon', 'Élèves', 'T1x', 'F10_'])('accepte « %s »', (n) => {
		expect(tableNameProblem(n)).toBeNull();
	});
	it.each(['2024 ventes', 'Mes ventes', 'Ventes-2024', '2024', 'Prix€', 'F10', 'AB12', 'r1c1', '', '   '])('refuse « %s »', (n) => {
		expect(tableNameProblem(n)?.title).toBe('Nom de tableau invalide');
	});
	it('dit pourquoi et ce qui est permis', () => {
		const p = tableNameProblem('2024 ventes')!;
		expect(p.message).toContain('« 2024 ventes » : il commence par un chiffre');
		expect(p.message).toContain('lettres, des chiffres et _');
		expect(tableNameProblem('Mes ventes')!.message).toContain('il contient une espace');
		expect(tableNameProblem('Ventes-2024')!.message).toContain('le caractère « - » n\'est pas permis');
		expect(tableNameProblem('F10')!.message).toContain('adresse de cellule');
		expect(tableNameProblem('')!.message).toContain('ne peut pas être vide');
	});
	it('refuse un nom déjà pris, sans tenir compte des majuscules', () => {
		const p = tableNameProblem('ventes', ['Clients', 'Ventes']);
		expect(p?.title).toBe('Nom de tableau déjà pris');
		expect(p?.message).toContain('« Ventes »');
		expect(tableNameProblem('Ventes2', ['Ventes'])).toBeNull();
	});
	it('ignore les espaces autour', () => {
		expect(tableNameProblem('  Ventes ')).toBeNull();
	});
});

describe('a1Name', () => {
	it('reconnaît les adresses A1 et L1C1', () => {
		expect(a1Name('F10')).toBe(true);
		expect(a1Name('xfd1048576')).toBe(true);
		expect(a1Name('R2C3')).toBe(true);
		expect(a1Name('F10_')).toBe(false);
		expect(a1Name('Ventes')).toBe(false);
	});
});

describe('sheetNameProblem', () => {
	it('refuse un nom vide', () => {
		expect(sheetNameProblem('  ')?.title).toBe('Nom de feuille invalide');
	});
	it('refuse un nom pris (majuscules comprises), accepte le reste', () => {
		expect(sheetNameProblem('feuil1', ['Feuil1'])?.title).toBe('Nom de feuille déjà pris');
		expect(sheetNameProblem('Bilan 2024 (final)', ['Feuil1'])).toBeNull();
	});
});

describe('cellNameProblem', () => {
	it('accepte un identifiant, avec points, et le vide (nom retiré)', () => {
		expect(cellNameProblem('Taux_TVA')).toBeNull();
		expect(cellNameProblem('taux.reduit')).toBeNull();
		expect(cellNameProblem('')).toBeNull();
	});
	it('refuse espaces, chiffre initial, mots réservés et adresses', () => {
		expect(cellNameProblem('taux tva')?.message).toContain('il contient une espace');
		expect(cellNameProblem('1er')?.message).toContain('il commence par un chiffre');
		expect(cellNameProblem('vrai')?.message).toContain('mot réservé');
		expect(cellNameProblem('cell_B3')?.message).toContain('adresse de cellule');
	});
});

describe('columnNameProblem', () => {
	it('refuse une colonne déjà présente dans le tableau', () => {
		const p = columnNameProblem('prix', ['Produit', 'Prix'], 'Ventes');
		expect(p?.title).toBe('Nom de colonne déjà pris');
		expect(p?.message).toContain('Ventes[prix]');
	});
	it('accepte un nom libre, avec espaces, et le vide (« Colonne n »)', () => {
		expect(columnNameProblem('Prix unitaire', ['Prix'])).toBeNull();
		expect(columnNameProblem('', ['Prix'])).toBeNull();
	});
});

describe('scriptNameProblem et favoriteNameProblem', () => {
	it('refusent le vide', () => {
		expect(scriptNameProblem(' ')?.title).toBe('Nom de script invalide');
		expect(favoriteNameProblem('')?.title).toBe('Nom de favori invalide');
		expect(favoriteNameProblem('Top clients')).toBeNull();
	});
	it('un script pris (même casse, comme le moteur)', () => {
		expect(scriptNameProblem('Relances', ['Relances'])?.title).toBe('Nom de script déjà pris');
		expect(scriptNameProblem('relances', ['Relances'])).toBeNull();
	});
});

describe('refusedName', () => {
	it('retire le préfixe technique du moteur', () => {
		expect(refusedName('Nom de tableau refusé', new Error('tableau : « Ventes » existe déjà')).message).toBe('« Ventes » existe déjà.\n\nChoisissez un nom qui n’est pas encore utilisé dans le classeur.');
		expect(refusedName('x', new Error('la colonne « Prix » existe déjà dans le tableau Ventes : choisissez un autre nom')).message).not.toContain('\n');
		expect(refusedName('x', 'onglet : nom vide').message).toBe('Nom vide');
		expect(refusedName('x', new Error('nom « 1a » refusé : mot réservé')).message).toBe('« 1a » : mot réservé');
		expect(refusedName('x', '').message).toBe('Le nom a été refusé.');
	});
});
