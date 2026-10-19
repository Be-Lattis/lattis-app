import { describe, expect, it } from 'vitest';
import { midEllipsis } from './midellipsis';

const m = (s: string) => [...s].length * 6; // 6 px par caractère

describe('midEllipsis', () => {
	it('laisse un texte court tel quel', () => {
		expect(midEllipsis('A12', 36, m)).toBe('A12');
	});
	it('coupe au milieu en gardant le début et la fin', () => {
		const s = midEllipsis('SULLY_bat_05', 36, m);
		expect(m(s)).toBeLessThanOrEqual(36);
		expect(s.startsWith('SUL')).toBe(true);
		expect(s.endsWith('05')).toBe(true);
		expect(s).toContain('…');
	});
	it('garde au moins un caractère', () => {
		expect(midEllipsis('ABCDEFGH', 6, m)).toBe('A…');
	});
});
