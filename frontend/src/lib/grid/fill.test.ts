import { describe, expect, it } from 'vitest';
import { fillSeries, shiftTableColumns } from './fill';

describe('poignée de recopie', () => {
	it('nombres', () => {
		expect(fillSeries(['1', '2'], 3)).toEqual(['3', '4', '5']);
		expect(fillSeries(['10', '7,5'], 2)).toEqual(['5', '2,5']);
		expect(fillSeries(['4'], 2)).toEqual(['4', '4']);
	});
	it('dates', () => {
		expect(fillSeries(['30/12/2025'], 3)).toEqual(['31/12/2025', '01/01/2026', '02/01/2026']);
		expect(fillSeries(['01/01/2026', '08/01/2026'], 1)).toEqual(['15/01/2026']);
	});
	it('mois et jours', () => {
		expect(fillSeries(['Novembre'], 3)).toEqual(['Décembre', 'Janvier', 'Février']);
		expect(fillSeries(['lundi', 'mercredi'], 2)).toEqual(['vendredi', 'dimanche']);
		expect(fillSeries(['janv.'], 1)).toEqual(['févr.']);
	});
	it('texte numéroté', () => {
		expect(fillSeries(['Article 1'], 2)).toEqual(['Article 2', 'Article 3']);
		expect(fillSeries(['T08', 'T10'], 2)).toEqual(['T12', 'T14']);
	});
	it('recopie sinon', () => {
		expect(fillSeries(['a', 'b'], 3)).toEqual(['a', 'b', 'a']);
		expect(fillSeries(['=SOMME(T[x])'], 2)).toEqual(['=SOMME(T[x])', '=SOMME(T[x])']);
	});
});

describe('recopie à droite des colonnes de tableau', () => {
	const tables = [{ name: 'Villes', columns: ['Ville', 'Région', 'Population'] }];
	it('passe à la colonne voisine, Cell[…] reste fixe', () => {
		expect(shiftTableColumns('=REFERENCE(Villes[Région]; Cell[S7])', 1, tables)).toBe('=REFERENCE(Villes[Population]; Cell[S7])');
		expect(shiftTableColumns('=SOMME(villes[Ville])+Villes[Ville][2]', 1, tables)).toBe('=SOMME(villes[Région])+Villes[Région][2]');
		expect(shiftTableColumns('=Villes[Région]', -1, tables)).toBe('=Villes[Ville]');
	});
	it("ne sort pas du tableau et ne touche ni aux chaînes ni aux autres noms", () => {
		expect(shiftTableColumns('=Villes[Population]', 1, tables)).toBe('=Villes[Population]');
		expect(shiftTableColumns('="Villes[Ville]"&Autre[Ville]', 1, tables)).toBe('="Villes[Ville]"&Autre[Ville]');
		expect(shiftTableColumns('Villes[Ville]', 1, tables)).toBe('Villes[Ville]');
	});
});
