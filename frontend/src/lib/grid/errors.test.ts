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
		expect(explainError('#VALEUR!', '=Ventes[Prix]*2')).toContain('Tableau[Colonne][ligne]');
		expect(explainError('#VALEUR!', '=SOMME([Prix])')).toContain('type de valeur');
	});
	it('cellule libre qui contient du texte', () => {
		const text = (r: number, c: number) => (r === 0 && c === 2 ? 'abc' : r === 1 && c === 2 ? '16/10/2026' : undefined);
		expect(explainError('#VALEUR!', '=Cell[C1]+1', text)).toBe('#VALEUR! : Cell[C1] contient du texte, pas un nombre ni une date.');
		expect(explainError('#VALEUR!', '=Cell[C2]+Cell[c1]', text)).toContain('Cell[C1] contient du texte');
		// Une adresse Cell[…] n'est pas une colonne entière.
		expect(explainError('#VALEUR!', '=Cell[C2]+1', text)).toContain('type de valeur');
		expect(explainError('#VALEUR!', '=Feuille2!Cell[C1]+1', text)).toContain('type de valeur');
	});
	it('autres codes', () => {
		expect(explainError('#DIV/0!')).toContain('division');
		// Audit n° 3, B1 : une boucle n'est pas annoncée comme un tableau supprimé.
		expect(explainError('#CIRC!', '=SOMME([X])')).toContain('référence circulaire');
		expect(explainError('#CIRC!')).not.toContain('supprimé');
		expect(explainError('#XYZ')).toBe('');
	});
});
