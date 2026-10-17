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
