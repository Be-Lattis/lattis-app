import { describe, expect, it } from 'vitest';
import { Axis } from './axis';

describe('Axis', () => {
	const a = new Axis(1000, 100, new Map([[2, 150], [3, 50]]));

	it('cumule les tailles particulières', () => {
		expect(a.offset(0)).toBe(0);
		expect(a.offset(2)).toBe(200);
		expect(a.offset(3)).toBe(350);
		expect(a.offset(4)).toBe(400);
		expect(a.offset(10)).toBe(1000);
		expect(a.total).toBe(100_000);
	});

	it('retrouve la case d’une position', () => {
		expect(a.indexAt(-5)).toBe(0);
		expect(a.indexAt(199)).toBe(1);
		expect(a.indexAt(200)).toBe(2);
		expect(a.indexAt(349)).toBe(2);
		expect(a.indexAt(350)).toBe(3);
		expect(a.indexAt(399)).toBe(3);
		expect(a.indexAt(400)).toBe(4);
		expect(a.indexAt(10_000_000)).toBe(999);
		for (let i = 0; i < 20; i++) expect(a.indexAt(a.offset(i))).toBe(i);
	});
});
