import { describe, expect, it } from 'vitest';
import { editMode, viewKey } from './readonlyedit';

describe('ouverture de l’éditeur', () => {
	it('cellule modifiable : saisie normale, au clavier comme au double clic', () => {
		expect(editMode('', false)).toBe('edit');
		expect(editMode('', true)).toBe('edit');
	});
	it('lecture seule : double clic ou F2 ouvrent en lecture seule', () => {
		expect(editMode('Tableau chargé par une requête', false)).toBe('view');
	});
	it('lecture seule : une frappe directe reste refusée', () => {
		expect(editMode('Tableau chargé par une requête', true)).toBe('refuse');
	});
});

describe('touches dans l’éditeur en lecture seule', () => {
	it('Échap et Ctrl+Entrée referment', () => {
		expect(viewKey({ key: 'Escape' })).toBe('close');
		expect(viewKey({ key: 'Enter', ctrlKey: true })).toBe('close');
	});
	it('Entrée et Tab referment en passant à la cellule suivante', () => {
		expect(viewKey({ key: 'Enter' })).toBe('enter');
		expect(viewKey({ key: 'Tab' })).toBe('tab');
	});
	it('copie, tout sélectionner et curseur laissés au navigateur', () => {
		expect(viewKey({ key: 'c', ctrlKey: true })).toBe('native');
		expect(viewKey({ key: 'C', metaKey: true })).toBe('native');
		expect(viewKey({ key: 'a', ctrlKey: true })).toBe('native');
		expect(viewKey({ key: 'Insert', ctrlKey: true })).toBe('native');
		expect(viewKey({ key: 'ArrowLeft' })).toBe('native');
		expect(viewKey({ key: 'End' })).toBe('native');
		expect(viewKey({ key: 'Shift' })).toBe('native');
	});
	it('frappe, collage, coupe, suppression et saut de ligne ignorés', () => {
		expect(viewKey({ key: 'x' })).toBe('block');
		expect(viewKey({ key: 'v', ctrlKey: true })).toBe('block');
		expect(viewKey({ key: 'x', ctrlKey: true })).toBe('block');
		expect(viewKey({ key: 'z', ctrlKey: true })).toBe('block');
		expect(viewKey({ key: 'Backspace' })).toBe('block');
		expect(viewKey({ key: 'Delete' })).toBe('block');
		expect(viewKey({ key: 'Enter', altKey: true })).toBe('block');
		expect(viewKey({ key: ';', ctrlKey: true })).toBe('block');
	});
});
