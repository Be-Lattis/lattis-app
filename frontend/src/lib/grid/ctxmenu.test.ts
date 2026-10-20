import { describe, expect, it } from 'vitest';
import { chartMenu, chartSubmenu, controlMenu, controlSubmenu, flat, freeCellMenu, groups, headerMenu, linesSubmenu, pivotSubmenu, tableCellMenu, tableColumnSubmenu, tableRowSubmenu } from './ctxmenu';

describe('groups', () => {
	it('retire les entrées absentes et les groupes vides (pas de séparateur en trop)', () => {
		expect(groups([false, null], ['a', undefined, 'b'], [], [''], ['c'])).toEqual([['a', 'b'], ['c']]);
	});
});

describe('cellule de tableau', () => {
	const all = { format: true, rules: true, note: true, sort: true, row: true, column: true, table: true };

	it('suit l’ordre demandé : presse-papiers, mise en forme, note, sous-menus', () => {
		expect(flat(tableCellMenu(all))).toBe('cut copy paste | clear format rules | note | sort row column table');
	});

	it('met la situation en tête, suivie d’un séparateur', () => {
		expect(flat(tableCellMenu({ ...all, ref: true, toNote: true }))).toBe('ref toNote | cut copy paste | clear format rules | note | sort row column table');
		expect(flat(tableCellMenu({ ...all, promote: true }))).toMatch(/^promote \| cut/);
	});

	it('en-tête : du texte ou des groupes au-dessus, pas de sous-menu Ligne', () => {
		const head = { ...all, row: false };
		expect(flat(tableCellMenu({ ...head, toHeader: true, groupsOn: true }))).toBe('toHeader groupsOn | cut copy paste | clear format rules | note | sort column table');
		expect(flat(tableCellMenu({ ...head, groupsOff: true }))).toMatch(/^groupsOff \| cut copy paste/);
	});

	it('sans mise en forme ni note (droits) : l’effacement reste, sans séparateur vide', () => {
		expect(flat(tableCellMenu({ column: true }))).toBe('cut copy paste | clear | column');
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
	});
});

describe('cellule libre', () => {
	it('situation, presse-papiers, mise en forme, note, insertion, tableau croisé', () => {
		expect(flat(freeCellMenu({ format: true, rules: true, note: true, lines: true, pivot: true }))).toBe('cut copy paste | clear format rules | note | insertDialog clearAll | pivot');
		expect(flat(freeCellMenu({ ref: true, asHeader: true, join: true, asTable: true, spill: true, toValues: true, asGroups: true }))).toBe(
			'ref asHeader join asTable spillLinked spillValues toValues asGroups | cut copy paste | clear'
		);
	});

	it('colle un graphique ou un bouton copié avec le presse-papiers', () => {
		expect(flat(freeCellMenu({ pasteChart: true, pasteButton: true }))).toBe('cut copy paste pasteChart pasteButton | clear');
	});
});

describe('numéro de ligne, lettre de colonne', () => {
	it('presse-papiers, mise en forme, puis le sous-menu de la ligne (et du tableau)', () => {
		expect(flat(headerMenu({ format: true, rules: true, lines: true, table: true }))).toBe('cut copy paste | clear format rules | lines table');
		expect(flat(headerMenu({ toNote: true, asGroups: true, lines: true }))).toBe('toNote asGroups | cut copy paste | clear | lines');
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

	it('graphique : presse-papiers, puis son sous-menu', () => {
		expect(flat(chartMenu({ image: true }))).toBe('cut copy copyImage pasteHere | chart');
		expect(flat(chartMenu({}))).toBe('cut copy pasteHere | chart');
		expect(flat(chartSubmenu({ edit: true }))).toBe('edit | delete');
		expect(flat(chartSubmenu({}))).toBe('delete');
	});

	it('bouton : presse-papiers, puis son sous-menu', () => {
		expect(flat(controlMenu())).toBe('cut copy | control');
		expect(flat(controlSubmenu({ run: true }))).toBe('run edit resize | delete');
		expect(flat(controlSubmenu({}))).toBe('edit resize | delete');
	});
});
