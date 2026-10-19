import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import { announcement, cellLabel, selectionLabel } from './a11y';
import GridA11y from './GridA11y.svelte';

describe('cellLabel', () => {
	it('cellule libre : adresse et valeur', () => {
		expect(cellLabel({ addr: 'B3', value: '12,50 €' })).toBe('B3, 12,50 €');
		expect(cellLabel({ addr: 'A1', value: '  ' })).toBe('A1, vide');
	});
	it('cellule de tableau : colonne avant la valeur', () => {
		expect(cellLabel({ addr: 'B3', value: '12,50 €', table: 'Ventes', column: 'Prix', row: 2 })).toBe('B3, Prix, 12,50 €');
	});
	it('en-tête : colonne et tableau, sans valeur', () => {
		expect(cellLabel({ addr: 'B1', value: 'Prix', table: 'Ventes', column: 'Prix', header: true })).toBe('B1, en-tête de colonne Prix, tableau Ventes');
	});
	it('nom défini, colonne calculée et erreur', () => {
		expect(cellLabel({ addr: 'C2', value: '#DIV/0!', name: 'Total', table: 'T', column: 'Marge', computed: true, error: true })).toBe(
			'C2, Total, Marge, #DIV/0!, colonne calculée, erreur de formule'
		);
	});
});

describe('selectionLabel', () => {
	it('rien pour une cellule seule', () => expect(selectionLabel('A1:A1', 1, 1)).toBe(''));
	it('accorde lignes et colonnes', () => {
		expect(selectionLabel('A1:C4', 4, 3)).toBe('Sélection A1:C4, 4 lignes sur 3 colonnes');
		expect(selectionLabel('A1:A2', 2, 1)).toBe('Sélection A1:A2, 2 lignes sur 1 colonne');
	});
});

describe('announcement', () => {
	const s = { addr: 'B3', value: '1', table: 'Ventes', column: 'Prix' };
	it('mode modification en priorité', () => expect(announcement(s, true, 'Sélection …')).toBe('Modification de B3'));
	it('sélection ensuite', () => expect(announcement(s, false, 'Sélection A1:B2, 2 lignes sur 2 colonnes')).toBe('Sélection A1:B2, 2 lignes sur 2 colonnes'));
	it("nom du tableau à l'entrée seulement", () => {
		expect(announcement(s, false, '', undefined)).toBe('Tableau Ventes');
		expect(announcement(s, false, '', 'Ventes')).toBe('');
	});
});

describe('GridA11y (rendu)', () => {
	it('porte la cellule active et une région aria-live', () => {
		const { body } = render(GridA11y, {
			props: { cellId: 'c1', speech: { addr: 'B3', value: '12,50 €', table: 'Ventes', column: 'Prix' }, row: 3, col: 2, editing: false }
		});
		expect(body).toContain('role="row"');
		expect(body).toContain('aria-rowindex="3"');
		expect(body).toMatch(/id="c1"[^>]*role="gridcell"/);
		expect(body).toContain('aria-colindex="2"');
		expect(body).toContain('B3, Prix, 12,50 €');
		expect(body).toContain('aria-live="polite"');
	});
});
