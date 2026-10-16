import { describe, expect, it } from 'vitest';
import { formatNumber, mergeStyles, parseStyle } from './style';

describe('mise en forme', () => {
	it('lit et fusionne les styles', () => {
		expect(parseStyle('{"b":true}')).toEqual({ b: true });
		expect(parseStyle('')).toBeUndefined();
		expect(mergeStyles({ nf: '0,00', al: 'center' }, { b: true, al: 'left' })).toEqual({ nf: '0,00', al: 'left', b: true });
	});

	it('formate les nombres', () => {
		expect(formatNumber('0,00', 1234.5)).toBe('1\u202f234,50');
		expect(formatNumber('%', 0.125)).toBe('12,5\u00a0%');
		expect(formatNumber('€', 3)).toBe('3,00\u00a0€');
		expect(formatNumber(undefined, 3)).toBeNull();
	});
});
