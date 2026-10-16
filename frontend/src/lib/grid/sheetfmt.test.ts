import { describe, expect, it } from 'vitest';
import { hiddenLines, mergeAt, mergeList, parseList, skipHidden, validate } from './sheetfmt';

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
