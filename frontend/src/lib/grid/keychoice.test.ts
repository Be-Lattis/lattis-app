import { describe, expect, it } from 'vitest';
import { keyQuestion } from './keychoice';

const base =
	"clé à choisir : « Produits » n'a pas de clé ; la recherche peut désigner ses lignes par leur numéro, ou par la colonne « Code produit », qui deviendrait la clé du tableau (les doublons y seraient refusés)";

describe('question de la clé', () => {
	it('tableau et colonne', () => {
		expect(keyQuestion(base)).toEqual({ table: 'Produits', column: 'Code produit', why: '' });
	});
	it('colonne impossible : la raison', () => {
		expect(keyQuestion(base + ' ; « Code produit » ne peut pas servir de clé : 2 valeur(s) en double')).toEqual({
			table: 'Produits',
			column: 'Code produit',
			why: '2 valeur(s) en double'
		});
	});
	it('message préfixé (erreur relayée)', () => {
		expect(keyQuestion('Error: ' + base)?.table).toBe('Produits');
	});
	it('autre refus', () => {
		expect(keyQuestion('formule : parenthèse manquante')).toBeNull();
	});
});
