import { describe, expect, it } from 'vitest';
import { DEFAULT_WIDTH, GRID_MIN, fitWidths, panelWidth, roomFor } from './resize.js';

describe('panelWidth', () => {
	it('vaut 500 px par défaut', () => {
		expect(DEFAULT_WIDTH).toBe(500);
		expect(panelWidth(undefined, 4000)).toBe(DEFAULT_WIDTH);
	});
	it('ne dépasse pas la moitié de la fenêtre, par défaut comme tirée', () => {
		expect(panelWidth(undefined, 800)).toBe(400);
		expect(panelWidth(1200, 1600)).toBe(800);
		expect(panelWidth(500.4, 1600)).toBe(500);
	});
	it('garde le minimum même quand la place manque', () => {
		expect(panelWidth(100, 1600)).toBe(220);
		expect(panelWidth(undefined, 300)).toBe(220);
		expect(panelWidth(undefined, 400, 260)).toBe(260);
	});
});

describe('fitWidths', () => {
	it('garde les largeurs quand la grille a sa place', () => {
		expect(fitWidths([500, 400], [220, 220], 1920)).toEqual([500, 400]);
	});
	it('rétrécit les volets dans la même proportion pour laisser sa place à la grille', () => {
		const w = fitWidths([500, 500], [220, 220], 1200);
		expect(w[0] + w[1]).toBeLessThanOrEqual(1200 - GRID_MIN);
		expect(w).toEqual([450, 450]);
		expect(fitWidths([500, 320], [320, 220], 1000)).toEqual([422, 277]);
	});
	it('ne passe pas sous les minimums, même si la grille y perd', () => {
		expect(fitWidths([500, 500, 500], [320, 220, 220], 800)).toEqual([320, 220, 220]);
	});
	it('un seul volet : la règle de panelWidth suffit', () => {
		expect(fitWidths([500], [220], 1366)).toEqual([500]);
	});
});

describe('roomFor', () => {
	it('ce que laissent les autres volets et la grille, au moins le minimum', () => {
		expect(roomFor([500], 1366)).toBe(566);
		expect(roomFor([500, 500], 1366)).toBe(220);
	});
});
