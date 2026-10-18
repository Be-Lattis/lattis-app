import { describe, expect, it } from 'vitest';
import { REGIONS, cycle, letterIndex, nextRegion } from './focus.js';

describe('parcours en boucle', () => {
	it('suivant, précédent, et depuis aucun', () => {
		expect(cycle(3, 0, 1)).toBe(1);
		expect(cycle(3, 2, 1)).toBe(0);
		expect(cycle(3, 0, -1)).toBe(2);
		expect(cycle(3, -1, 1)).toBe(0);
		expect(cycle(3, -1, -1)).toBe(2);
		expect(cycle(0, -1, 1)).toBe(-1);
	});
});

describe('première lettre dans un menu', () => {
	const labels = ['Couper', 'Copier', 'Coller', 'Insérer une ligne', 'Effacer'];
	it('entrée suivante qui commence par la lettre, en boucle', () => {
		expect(letterIndex(labels, -1, 'c')).toBe(0);
		expect(letterIndex(labels, 0, 'c')).toBe(1);
		expect(letterIndex(labels, 2, 'C')).toBe(0);
	});
	it('sans accents, et -1 si rien ne commence ainsi', () => {
		expect(letterIndex(labels, 0, 'i')).toBe(3);
		expect(letterIndex(['Écrire', 'Lire'], -1, 'e')).toBe(0);
		expect(letterIndex(labels, 0, 'z')).toBe(-1);
		expect(letterIndex(labels, 0, '?')).toBe(-1);
	});
});

describe('régions (F6)', () => {
	const all = [...REGIONS];
	it('ordre : ruban, barre de formule, grille, volet, onglets, barre d’état, puis le ruban', () => {
		expect(nextRegion(all, 'ribbon')).toBe('formula');
		expect(nextRegion(all, 'grid')).toBe('panel');
		expect(nextRegion(all, 'status')).toBe('ribbon');
	});
	it('Maj+F6 en arrière', () => {
		expect(nextRegion(all, 'ribbon', true)).toBe('status');
		expect(nextRegion(all, 'tabs', true)).toBe('panel');
	});
	it('saute les régions absentes (pas de volet, pas de barre de formule)', () => {
		const some = ['ribbon', 'grid', 'tabs', 'status'] as const;
		expect(nextRegion([...some], 'grid')).toBe('tabs');
		expect(nextRegion([...some], 'ribbon')).toBe('grid');
		expect(nextRegion([...some], 'grid', true)).toBe('ribbon');
	});
	it('depuis une région absente : la suivante dans l’ordre complet', () => {
		expect(nextRegion(['ribbon', 'grid', 'status'], 'panel')).toBe('status');
		expect(nextRegion(['ribbon', 'grid', 'status'], 'panel', true)).toBe('grid');
		expect(nextRegion(['ribbon', 'grid'], null)).toBe('ribbon');
		expect(nextRegion([], 'grid')).toBe(null);
	});
});
