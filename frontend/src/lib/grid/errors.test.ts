import { describe, expect, it } from 'vitest';
import { explainError } from './errors';

describe('explication des erreurs', () => {
	it('adresse A1 inconnue', () => {
		expect(explainError('#NOM?', '=B2*2')).toContain("B2 n'existe pas");
		expect(explainError('#NOM?', '=SOMME($A$1:A3)')).toContain('$A$1');
		expect(explainError('#NOM?', '=SOMMME([@Prix])')).toContain('nom inconnu');
		expect(explainError('#NOM?', '=Ventes[Prix2]')).toContain('nom inconnu');
	});
	it('colonne entière au lieu de la ligne', () => {
		expect(explainError('#VALEUR!', '=Ventes[Prix]*2')).toContain('Tableau[Colonne][ligne ou clé]');
		expect(explainError('#VALEUR!', '=SOMME([Prix])')).toContain('type de valeur');
	});
	it('autres codes', () => {
		expect(explainError('#DIV/0!')).toContain('division');
		expect(explainError('#XYZ')).toBe('');
	});
});
