import { describe, expect, it } from 'vitest';
import { handQuestion, keyQuestion } from './keychoice';

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

const hand =
	'jointure à choisir : « Suivi » est chargé par une requête ; les valeurs saisies à côté suivront ses lignes, d’une actualisation à l’autre, par une colonne qui les identifie.';

describe('question de la jointure (colonnes saisies)', () => {
	it('colonnes possibles', () => {
		expect(handQuestion("Error: jointure à choisir : « Suivi » est chargé par une requête ; les valeurs saisies à côté suivront ses lignes, d'une actualisation à l'autre, par une colonne qui les identifie. Colonnes possibles : « Nid », « Code site »."))
			.toEqual({ table: 'Suivi', columns: ['Nid', 'Code site'] });
	});
	it('aucune colonne : le rang seulement', () => {
		expect(handQuestion(hand + ' Aucune colonne ne convient (valeurs en double ou cases vides) : elles suivront le rang des lignes.')).toEqual({ table: 'Suivi', columns: [] });
	});
	it('autre refus', () => {
		expect(handQuestion(base)).toBeNull();
	});
});

describe('cause du moteur (texte traduit)', () => {
	const err = (id: string, args: Record<string, unknown>) => Object.assign(new Error('x'), { cause: { id, args, message: 'key to choose…' } });
	it('clé', () => {
		expect(keyQuestion('key to choose', err('key.chooseWhy', { table: 'Produits', column: 'Code', why: 'duplicates' }))).toEqual({ table: 'Produits', column: 'Code', why: 'duplicates' });
		expect(keyQuestion('key to choose', err('key.choose', { table: 'Produits', column: 'Code' }))?.why).toBe('');
	});
	it('jointure', () => {
		expect(handQuestion('join', err('hand.chooseJoin', { table: 'Suivi', columns: '« Nid », « Code site »' }))).toEqual({ table: 'Suivi', columns: ['Nid', 'Code site'] });
		expect(handQuestion('join', err('hand.chooseJoinNone', { table: 'Suivi' }))).toEqual({ table: 'Suivi', columns: [] });
	});
});
