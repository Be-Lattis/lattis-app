import { describe, expect, it } from 'vitest';
import { isProblem, noticeTtl } from './notice';

describe('messages de la barre d’état', () => {
	it('garde une erreur plus longtemps qu’une information', () => {
		expect(isProblem('Erreur du script : x')).toBe(true);
		expect(isProblem('Feuille protégée')).toBe(true);
		expect(noticeTtl('Classeur enregistré')).toBe(8000);
		expect(noticeTtl('Erreur du script : x')).toBe(15000);
	});
});

describe('niveau des messages', () => {
	it('compte rendu ou avertissement', () => {
		expect(isProblem('Cellules déplacées (Ctrl+Z pour annuler)')).toBe(false);
		expect(isProblem('Graphique collé')).toBe(false);
		expect(isProblem('#VALEUR! : type de valeur inattendu')).toBe(true);
		expect(isProblem('Placez la cellule active dans un tableau pour insérer un graphique')).toBe(true);
		expect(isProblem('Tableau en lecture seule : vos droits ne permettent pas de le modifier')).toBe(true);
	});
});
