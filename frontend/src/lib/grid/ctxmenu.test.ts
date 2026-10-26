import { describe, expect, it } from 'vitest';
import { chartMenu, chartSubmenu, controlMenu, controlSubmenu, flat, freeCellMenu, groups, headerMenu, linesSubmenu, pivotSubmenu, tableCellMenu, tableColumnSubmenu, tableRowSubmenu } from './ctxmenu';

describe('groups', () => {
	it('retire les entrées absentes et les groupes vides (pas de séparateur en trop)', () => {
		expect(groups([false, null], ['a', undefined, 'b'], [], [''], ['c'])).toEqual([['a', 'b'], ['c']]);
	});
});

describe('cellule de tableau', () => {
	const all = { format: true, rules: true, note: true, sort: true, row: true, column: true, table: true };

	it('suit l’ordre demandé : objets, presse-papiers, mise en forme, note (tri dans Colonne)', () => {
		expect(flat(tableCellMenu(all))).toBe('row column table | cut copy paste pasteSpecial | clear format rules | note');
	});

	it('met la situation après les objets, entre deux séparateurs', () => {
		expect(flat(tableCellMenu({ ...all, ref: true, toNote: true }))).toBe('row column table | ref toNote | cut copy paste pasteSpecial | clear format rules | note');
		expect(flat(tableCellMenu({ ...all, promote: true }))).toMatch(/^row column table \| promote \| cut/);
	});

	it('en-tête : du texte ou des groupes au-dessus, pas de sous-menu Ligne', () => {
		const head = { ...all, row: false };
		expect(flat(tableCellMenu({ ...head, toHeader: true, groupsOn: true }))).toBe('column table | toHeader groupsOn | cut copy paste pasteSpecial | clear format rules | note');
		expect(flat(tableCellMenu({ ...head, groupsOff: true }))).toMatch(/^column table \| groupsOff \| cut copy paste/);
	});

	it('sans mise en forme ni note (droits) : l’effacement reste, sans séparateur vide', () => {
		expect(flat(tableCellMenu({ column: true }))).toBe('column | cut copy paste pasteSpecial | clear');
	});

	it('n’a plus d’entrée de l’assistant', () => {
		expect(flat(tableCellMenu({ ...all, ref: true, promote: true, toNote: true }))).not.toMatch(/ask/);
	});

	it('sous-menus Ligne et Colonne : insertion puis suppression en dernier', () => {
		expect(flat(tableRowSubmenu())).toBe('insertAbove insertBelow deleteRow');
		expect(flat(tableColumnSubmenu({ typeKey: true, validation: true, tabChoice: true, toValues: true }))).toBe(
			'rename typeKey validation tabChoice toValues | insertLeft insertRight deleteColumn'
		);
		expect(flat(tableColumnSubmenu({}))).toBe('rename | insertLeft insertRight deleteColumn');
		// Tri et filtre en tête du sous-menu Colonne ; plus d'entrée « Trier et filtrer » à part.
		expect(flat(tableColumnSubmenu({ sort: true }))).toBe('sort | rename | insertLeft insertRight deleteColumn');
		expect(flat(tableCellMenu({ sort: true, column: true }))).not.toContain('sort');
		expect(flat(tableCellMenu({ sort: true }))).toBe('cut copy paste pasteSpecial | clear | sort');
	});
});

describe('cellule libre', () => {
	it('tableau croisé, situation, presse-papiers, mise en forme, note, insertion', () => {
		expect(flat(freeCellMenu({ format: true, rules: true, note: true, lines: true, pivot: true }))).toBe('pivot | cut copy paste pasteSpecial | clear format rules | note | insertDialog clearAll');
		expect(flat(freeCellMenu({ ref: true, asHeader: true, join: true, asTable: true, spill: true, toValues: true, asGroups: true }))).toBe(
			'ref asHeader join asTable spillLinked spillValues toValues asGroups | cut copy paste pasteSpecial | clear'
		);
	});

	it('colle un graphique ou un bouton copié avec le presse-papiers', () => {
		expect(flat(freeCellMenu({ pasteChart: true, pasteButton: true }))).toBe('cut copy paste pasteSpecial pasteChart pasteButton | clear');
	});
});

describe('numéro de ligne, lettre de colonne', () => {
	it('la ligne ou la colonne en tête (sans sous-menu), puis tableau, situation, presse-papiers, mise en forme', () => {
		expect(flat(headerMenu({ format: true, rules: true, lines: true, table: true }))).toBe('linesTop | table | cut copy paste pasteSpecial | clear format rules');
		expect(flat(headerMenu({ toNote: true, asGroups: true, lines: true, table: true }))).toBe('linesTop | table | toNote asGroups | cut copy paste pasteSpecial | clear');
	});

	it('sous-menu des lignes ou colonnes : insertion, taille, affichage', () => {
		expect(flat(linesSubmenu({ insert: true, size: true, hide: true, unhide: true }))).toBe('insert delete | size autoFit | hide unhide');
		expect(linesSubmenu({})).toEqual([]);
	});
});

describe('objets', () => {
	it('tableau croisé : développer, réduire ; source, déplacement ; suppression', () => {
		expect(flat(pivotSubmenu({ fold: true, source: true, move: true, remove: true }))).toBe('expand collapse | expandAll collapseAll | source move | delete');
		expect(flat(pivotSubmenu({ move: true, remove: true }))).toBe('move | delete');
	});

	it('graphique : son sous-menu, puis le presse-papiers', () => {
		expect(flat(chartMenu({ image: true }))).toBe('chart | cut copy copyImage pasteHere');
		expect(flat(chartMenu({}))).toBe('chart | cut copy pasteHere');
		expect(flat(chartSubmenu({ edit: true }))).toBe('edit | delete');
		expect(flat(chartSubmenu({}))).toBe('delete');
	});

	it('bouton : son sous-menu, puis le presse-papiers', () => {
		expect(flat(controlMenu())).toBe('control | cut copy');
		expect(flat(controlSubmenu({ run: true }))).toBe('run edit resize | delete');
		expect(flat(controlSubmenu({}))).toBe('edit resize | delete');
	});
});
