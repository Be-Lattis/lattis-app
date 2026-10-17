import { describe, expect, it } from 'vitest';
import { clampWidth } from './resize.js';

describe('clampWidth', () => {
	it('borne la largeur entre le minimum, le maximum et la place disponible', () => {
		expect(clampWidth(100, 220, 900, 1200)).toBe(220);
		expect(clampWidth(400.4, 220, 900, 1200)).toBe(400);
		expect(clampWidth(1000, 220, 900, 1200)).toBe(900);
		expect(clampWidth(800, 220, 900, 600)).toBe(600);
	});
	it('garde le minimum même quand la place manque', () => {
		expect(clampWidth(300, 220, 900, 100)).toBe(220);
	});
});
