import { describe, expect, it } from 'vitest';
import { freeDisplay, freeKind } from './freecell';

describe('cellules libres', () => {
	it('reconnaît les saisies à la Excel', () => {
		for (const s of ['12', '-5', '3,5', '1 000', '15%', '12 €', '(12)', '1e3', '€ 5']) expect(freeKind(s), s).toBe('number');
		for (const s of ['16/10/2026', '16/10', '2026-10-16', '12:30', '16/10/2026 12:30']) expect(freeKind(s), s).toBe('date');
		for (const s of ['VRAI', 'faux']) expect(freeKind(s), s).toBe('bool');
		for (const s of ['abc', 'Prix (€)', '()', '€', '12 ans', '']) expect(freeKind(s), s).toBe('text');
	});
	it('dates et nombres à droite, booléens centrés', () => {
		expect(freeDisplay('16/10/2026').align).toBe('right');
		expect(freeDisplay('=AUJOURDHUI()', '16/10/2026').align).toBe('right');
		expect(freeDisplay('=A', '#VALEUR!').align).toBe('left');
		expect(freeDisplay('VRAI').align).toBe('center');
		expect(freeDisplay('Bonjour').align).toBe('left');
	});
	it("l'apostrophe force le texte et n'est pas affichée", () => {
		expect(freeDisplay("'007")).toEqual({ text: '007', kind: 'text', align: 'left' });
		expect(freeDisplay("'16/10/2026").kind).toBe('text');
	});
});
