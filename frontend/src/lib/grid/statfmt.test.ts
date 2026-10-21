import { describe, expect, it } from 'vitest';
import { formatStat } from './style';

// Barre d'état : moyenne et somme à deux décimales, séparateur des milliers.
describe('formatStat à deux décimales', () => {
	it('moyenne et somme', () => {
		expect(formatStat(64122.0327, { dp: 2 }).replace(/\s/g, ' ')).toBe('64 122,03');
		expect(formatStat(9233572.7066, { dp: 2 }).replace(/\s/g, ' ')).toBe('9 233 572,71');
	});
});
