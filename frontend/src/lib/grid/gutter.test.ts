import { describe, expect, it } from 'vitest';
import { besideGutter, gutterOwner } from './gutter';

describe('gouttière', () => {
	const a = { row: 0, last: 5, col: 0, lastCol: 2 }; // en colonne A
	const b = { row: 1, last: 8, col: 4, lastCol: 6 }; // à droite, décalé d'une ligne
	it('seul le tableau qui touche la gouttière y est numéroté', () => {
		expect(gutterOwner([a, b], 3, 0)).toBe(a);
		expect(gutterOwner([a, b], 7, 0)).toBeUndefined(); // ligne de b, mais b ne touche pas
		expect(gutterOwner([a, b], 0, 0)).toBeUndefined(); // en-tête
	});
	it('défilé vers la droite : le tableau de la première colonne affichée', () => {
		expect(gutterOwner([a, b], 3, 5)).toBe(b);
		expect(gutterOwner([a, b], 3, 3)).toBeUndefined();
	});
	it('numéros à côté du tableau quand il ne touche pas', () => {
		expect(besideGutter(b, 0, 10)).toBe(true);
		expect(besideGutter(b, 4, 10)).toBe(false);
		expect(besideGutter(b, 0, 3)).toBe(false); // hors de l'écran
		expect(besideGutter(a, 0, 10)).toBe(false);
	});
});
