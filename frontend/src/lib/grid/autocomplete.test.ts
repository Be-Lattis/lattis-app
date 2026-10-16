import { describe, expect, it } from 'vitest';
import { apply, signature, suggest, type Catalog } from './autocomplete';

const cat: Catalog = {
	functions: [
		{ name: 'SI', other: 'IF', arity: 'de 2 à 3 arguments' },
		{ name: 'SIERREUR', other: 'IFERROR', arity: '2 argument(s)' },
		{ name: 'SOMME', other: 'SUM', arity: 'au moins 1 argument(s)' },
		{ name: 'REFERENCE', other: 'REFERENCE', arity: '3 argument(s)' }
	],
	tables: [
		{ name: 'Ventes', columns: ['Produit', 'Prix', 'Qté'] },
		{ name: 'Clients', columns: ['Id', 'Nom'] }
	]
};

describe('autocomplétion des formules', () => {
	it('propose les fonctions et tableaux qui commencent par le préfixe', () => {
		const c = suggest('=so', 3, cat)!;
		expect(c.from).toBe(1);
		expect(c.items.map((i) => i.label)).toEqual(['SOMME']);
		expect(suggest('=SOMME(v', 8, cat)!.items[0]).toMatchObject({ label: 'Ventes', insert: 'Ventes[' });
	});

	it('propose les colonnes après « [ », avec @ pour la ligne courante', () => {
		expect(suggest('=SOMME(Ventes[p', 15, cat)!.items.map((i) => i.insert)).toEqual(['Produit]', 'Prix]']);
		expect(suggest('=[@q', 4, cat, 'Ventes')!.items[0].insert).toBe('@Qté]');
		expect(suggest('=[@q', 4, cat)).toBeNull(); // pas de tableau courant
	});

	it('dans une ligne du tableau, [@Colonne] avant [Colonne]', () => {
		expect(suggest('=[Prix]*[q', 10, cat, 'Ventes')!.items.map((i) => i.insert)).toEqual(['@Qté]', 'Qté]']);
	});

	it('ignore le texte hors formule et les chaînes', () => {
		expect(suggest('so', 2, cat)).toBeNull();
		expect(suggest('="so', 4, cat)).toBeNull();
	});

	it('applique une suggestion', () => {
		const c = suggest('=1+so', 5, cat)!;
		expect(apply('=1+so', 5, c.from, c.items[0])).toEqual({ text: '=1+SOMME(', caret: 9 });
	});

	it('donne la signature de la fonction en cours', () => {
		expect(signature('=SI(A>1;SOMME(1', 15, cat)).toBe('SOMME : au moins 1 argument(s)');
		expect(signature('=SI(A>1;SOMME(1);', 17, cat)).toBe('SI : de 2 à 3 arguments');
		expect(signature('=IF(', 4, cat)).toBe('SI : de 2 à 3 arguments');
		expect(signature('=1+2', 4, cat)).toBeNull();
	});
});
