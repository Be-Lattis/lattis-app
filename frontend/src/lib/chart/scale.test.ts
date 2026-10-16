import { describe, expect, it } from 'vitest';
import { parseNumber, tickLabel, ticks } from './scale';

describe('graduations', () => {
	it('rondes et couvrant les données', () => {
		expect(ticks(0, 17)).toEqual([0, 5, 10, 15, 20]);
		expect(ticks(3, 9.5, 4)).toEqual([0, 2.5, 5, 7.5, 10]);
		expect(ticks(-12, 30)).toEqual([-20, -10, 0, 10, 20, 30]);
		expect(ticks(0, 0)).toEqual([0, 0.2, 0.4, 0.6, 0.8, 1]);
	});
	it('libellés et nombres', () => {
		expect(tickLabel(2.5)).toBe('2,5');
		expect(parseNumber('1 234,5')).toBe(1234.5);
		expect(parseNumber('FR')).toBeNull();
	});
});
