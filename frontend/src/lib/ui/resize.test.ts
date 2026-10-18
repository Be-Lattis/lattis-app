import { describe, expect, it } from 'vitest';
import { BASE_WIDTH, DEFAULT_WIDTH, panelWidth } from './resize.js';

describe('panelWidth', () => {
	it('vaut par défaut trois fois la largeur de référence', () => {
		expect(DEFAULT_WIDTH).toBe(3 * BASE_WIDTH);
		expect(panelWidth(undefined, 4000)).toBe(DEFAULT_WIDTH);
	});
	it('ne dépasse pas la moitié de la fenêtre, par défaut comme tirée', () => {
		expect(panelWidth(undefined, 1600)).toBe(800);
		expect(panelWidth(1200, 1600)).toBe(800);
		expect(panelWidth(500.4, 1600)).toBe(500);
	});
	it('garde le minimum même quand la place manque', () => {
		expect(panelWidth(100, 1600)).toBe(220);
		expect(panelWidth(undefined, 300)).toBe(220);
		expect(panelWidth(undefined, 400, 260)).toBe(260);
	});
});
