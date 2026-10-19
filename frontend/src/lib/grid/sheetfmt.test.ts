import { describe, expect, it } from 'vitest';
import { cleanSource, dvDialog, dvTarget, dvPatch, dvPreview, dvSource, hiddenLines, mergeAt, mergeList, parseList, skipHidden, validate, validateSource } from './sheetfmt';

describe('fusions', () => {
	const list = mergeList(
		new Map([
			['2,3', { mr: 2, mc: 3, al: 'center' as const }],
			['9,9', { b: true }],
			['5,0', { mr: 1, mc: 1 }]
		])
	);
	it('lit les fusions des styles', () => {
		expect(list).toEqual([{ row: 2, col: 3, rows: 2, cols: 3 }]);
	});
	it('trouve la fusion d’une cellule', () => {
		expect(mergeAt(list, 3, 5)).toEqual(list[0]);
		expect(mergeAt(list, 4, 5)).toBeUndefined();
		expect(mergeAt(list, 2, 6)).toBeUndefined();
	});
});

describe('lignes masquées', () => {
	const hidden = hiddenLines(new Map([[2, { hd: true }], [3, { hd: true, b: true }], [5, { b: true }]]));
	it('lit le masquage', () => {
		expect([...hidden]).toEqual([2, 3]);
	});
	it('saute les lignes masquées dans le sens du déplacement', () => {
		expect(skipHidden(2, 1, hidden, 100)).toBe(4);
		expect(skipHidden(3, -1, hidden, 100)).toBe(1);
		expect(skipHidden(1, 1, hidden, 100)).toBe(1);
		// En bout de feuille : l'autre sens.
		expect(skipHidden(0, -1, new Set([0]), 100)).toBe(1);
	});
});

describe('validation par liste', () => {
	it('accepte la liste (casse ignorée) et le vide', () => {
		expect(validate(['Oui', 'Non'], ' oui ')).toEqual({ ok: true, value: 'Oui' });
		expect(validate(['Oui', 'Non'], '')).toEqual({ ok: true, value: '' });
		expect(validate(undefined, 'x')).toEqual({ ok: true, value: 'x' });
	});
	it('refuse le reste avec un message', () => {
		const r = validate(['Oui', 'Non'], 'Peut-être');
		expect(r.ok).toBe(false);
		if (!r.ok) expect(r.message).toContain('Oui, Non');
	});
	it('lit la liste saisie', () => {
		expect(parseList('Lyon; Paris ;\nNice;lyon;;')).toEqual(['Lyon', 'Paris', 'Nice']);
	});
});

describe('validation tirée d’une plage', () => {
	it('reconnaît la source et garde les listes d’avant', () => {
		expect(dvSource({ src: 'Clients[Ville]' })).toBe('Clients[Ville]');
		expect(dvSource(['Oui', 'Non'])).toBeUndefined();
		expect(dvSource({ src: ' ' })).toBeUndefined();
		expect(dvSource(undefined)).toBeUndefined();
	});
	it('ouvre le dialogue sur la forme de la cellule', () => {
		expect(dvDialog(['Oui', 'Non'])).toEqual({ mode: 'list', text: 'Oui\nNon', src: '' });
		expect(dvDialog({ src: 'Cell[A1:A3]' })).toEqual({ mode: 'range', text: '', src: 'Cell[A1:A3]' });
		expect(dvDialog(undefined)).toEqual({ mode: 'list', text: '', src: '' });
	});
	it('pose une liste ou une plage', () => {
		expect(cleanSource(' = Feuille2!Cell[A1:A3] ')).toBe('Feuille2!Cell[A1:A3]');
		expect(dvPatch('range', '', '=Clients[Ville]')).toEqual({ dv: { src: 'Clients[Ville]' } });
		expect(dvPatch('list', 'a;b', 'ignoré')).toEqual({ dv: ['a', 'b'] });
		expect('error' in dvPatch('range', 'a', ' ')).toBe(true);
		expect('error' in dvPatch('list', ' ; ', 'Clients[Ville]')).toBe(true);
	});
	it('montre un aperçu', () => {
		expect(dvPreview({ values: ['FR', 'IT'] })).toBe('2 valeurs : FR, IT');
		expect(dvPreview({ values: [], error: 'le tableau « X » n’existe plus' })).toContain('Source introuvable');
		expect(dvPreview({ values: [] })).toContain('aucune valeur');
		expect(dvPreview(null)).toBe('');
	});
	it('vérifie la saisie avec la liste relue', () => {
		expect(validateSource('T[C]', { values: ['FR', 'IT'] }, 'fr')).toEqual({ ok: true, value: 'FR' });
		expect(validateSource('T[C]', { values: ['FR', 'IT'] }, 'DE').ok).toBe(false);
		// Pas encore relue : la saisie passe ; vide : toujours permise.
		expect(validateSource('T[C]', undefined, 'DE')).toEqual({ ok: true, value: 'DE' });
		expect(validateSource('T[C]', { values: [], error: 'x' }, '')).toEqual({ ok: true, value: '' });
		const r = validateSource('T[C]', { values: [], error: 'le tableau « T » n’existe plus' }, 'DE');
		expect(r.ok).toBe(false);
		if (!r.ok) expect(r.message).toContain('T[C]');
		expect(validateSource('T[C]', { values: [] }, 'DE').ok).toBe(false);
	});
});

describe('cible d’une validation (titre du dialogue)', () => {
	const table = { name: 'Clients', row: 0, last: 9, col: 1, columns: ['Nom', 'Ville', 'CP'] };
	it('colonne entière d’un tableau', () => {
		expect(dvTarget({ sel: { r0: 1, c0: 2, r1: 9, c1: 2 }, table, sheet: 'Feuille1' })).toBe('colonne « Ville » du tableau Clients');
		expect(dvTarget({ sel: { r0: 0, c0: 2, r1: 9, c1: 3 }, table, sheet: 'Feuille1' })).toBe('colonnes « Ville » à « CP » du tableau Clients');
	});
	it('cellules d’un tableau ou libres', () => {
		expect(dvTarget({ sel: { r0: 1, c0: 2, r1: 3, c1: 2 }, table, sheet: 'Feuille1' })).toBe('cellules C2:C4 du tableau Clients');
		expect(dvTarget({ sel: { r0: 1, c0: 1, r1: 9, c1: 1 }, sheet: 'Feuille1' })).toBe('cellules B2:B10 (Feuille1)');
		expect(dvTarget({ sel: { r0: 2, c0: 2, r1: 2, c1: 2 }, sheet: 'Feuille1' })).toBe('cellule C3 (Feuille1)');
	});
	it('lignes ou colonnes de la feuille', () => {
		expect(dvTarget({ sel: { r0: 0, c0: 1, r1: 99, c1: 2 }, whole: 'cols', sheet: 'Feuille1' })).toBe('colonnes B:C (Feuille1)');
		expect(dvTarget({ sel: { r0: 4, c0: 0, r1: 4, c1: 99 }, whole: 'rows', sheet: 'Feuille1' })).toBe('ligne 5 (Feuille1)');
	});
});
