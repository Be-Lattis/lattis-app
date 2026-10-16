import { describe, expect, it } from 'vitest';
import { distance } from './dismiss';

describe('distance au menu', () => {
	const r = { left: 10, top: 10, right: 110, bottom: 60 };
	it('nulle à l’intérieur', () => expect(distance(r, 50, 30)).toBe(0));
	it('horizontale, verticale, diagonale', () => {
		expect(distance(r, 150, 30)).toBe(40);
		expect(distance(r, 50, 0)).toBe(10);
		expect(distance(r, 113, 64)).toBe(5);
	});
});
