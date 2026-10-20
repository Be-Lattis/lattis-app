import { describe, expect, it } from 'vitest';
import { tabChoiceItem, tabChoiceNotice } from './tabchoice';

const cands = [
	{ table: 'Ventes', column: 'Année', values: 2 },
	{ table: 'Ventes', column: 'Produit', values: 3 }
];

describe('tabChoiceItem', () => {
	it('propose de filtrer l’onglet par une colonne éligible', () => {
		const it = tabChoiceItem({ table: 'Ventes', column: 'Année', candidates: cands });
		expect(it.label).toBe("Filtrer l'onglet par cette colonne");
		expect(it.label.length).toBeLessThanOrEqual(40);
		expect(it.disabled).toBe(false);
		expect(it.clear).toBe(false);
		expect(it.title).toContain('▾');
		expect(it.title).toContain('« Année »');
	});

	it('reste permise tant que les colonnes proposées ne sont pas connues', () => {
		expect(tabChoiceItem({ table: 'Ventes', column: 'Montant', candidates: null }).disabled).toBe(false);
	});

	it('refuse une colonne calculée, vide ou de trop de valeurs, en disant pourquoi', () => {
		const calc = tabChoiceItem({ table: 'Ventes', column: 'TVA', formula: true, candidates: cands });
		expect(calc.disabled).toBe(true);
		expect(calc.title).toContain('calculée');
		const many = tabChoiceItem({ table: 'Ventes', column: 'Montant', candidates: cands });
		expect(many.disabled).toBe(true);
		expect(many.title).toContain('100');
	});

	it('retire le choix quand l’onglet filtre déjà par cette colonne (casse ignorée)', () => {
		const it = tabChoiceItem({ table: 'Ventes', column: 'année', choice: { name: 'Année' }, candidates: cands });
		expect(it.label).toBe("Ne plus filtrer l'onglet");
		expect(it.clear).toBe(true);
		expect(it.disabled).toBe(false);
	});

	it('remplace un autre choix', () => {
		const it = tabChoiceItem({ table: 'Ventes', column: 'Produit', choice: { name: 'Année' }, candidates: cands });
		expect(it.clear).toBe(false);
		expect(it.title).toContain('Remplace le choix par « Année »');
	});

	it('parle de champ pour un tableau croisé, et du tableau source', () => {
		const it = tabChoiceItem({ table: 'Ventes', column: 'Année', field: true, candidates: cands });
		expect(it.label).toBe("Filtrer l'onglet par ce champ");
		expect(it.title).toContain('tableau source');
	});
});

describe('tabChoiceNotice', () => {
	it('dit quoi faire ensuite', () => {
		expect(tabChoiceNotice('Année', false)).toContain('▾');
		expect(tabChoiceNotice('Année', true)).toContain('ne filtre plus');
	});
});
