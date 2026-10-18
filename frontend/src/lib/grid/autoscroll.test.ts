import { describe, expect, it } from 'vitest';
import { EDGE, autoScrollSpeed, edgeScroll, frameStep } from './autoscroll';

describe('autoScrollSpeed', () => {
	it('nulle en deçà du bord', () => {
		expect(autoScrollSpeed(-1)).toBe(0);
		expect(autoScrollSpeed(NaN)).toBe(0);
	});
	it('croît avec la distance, bornée', () => {
		const a = autoScrollSpeed(0);
		const b = autoScrollSpeed(20);
		const c = autoScrollSpeed(200);
		expect(a).toBeGreaterThan(0);
		expect(b).toBeGreaterThan(a);
		expect(c).toBeGreaterThan(b);
		expect(autoScrollSpeed(100_000)).toBe(autoScrollSpeed(1_000_000));
	});
});

describe('edgeScroll', () => {
	it('rien au milieu', () => {
		expect(edgeScroll(300, 22, 600)).toBe(0);
		expect(edgeScroll(22 + EDGE, 22, 600)).toBe(0);
		expect(edgeScroll(600 - EDGE - 1, 22, 600)).toBe(0);
	});
	it('vers le haut au bord haut et au-delà', () => {
		expect(edgeScroll(22, 22, 600)).toBeLessThan(0);
		expect(edgeScroll(-50, 22, 600)).toBeLessThan(edgeScroll(10, 22, 600));
	});
	it('vers le bas au bord bas et au-delà', () => {
		expect(edgeScroll(599, 22, 600)).toBeGreaterThan(0);
		expect(edgeScroll(700, 22, 600)).toBeGreaterThan(edgeScroll(610, 22, 600));
	});
	it('zone trop petite : pas de défilement', () => {
		expect(edgeScroll(5, 0, 10)).toBe(0);
	});
});

describe('frameStep', () => {
	it('proportionnel au temps écoulé, borné', () => {
		expect(frameStep(10, 1000 / 60)).toBeCloseTo(10);
		expect(frameStep(10, 2000 / 60)).toBeCloseTo(20);
		expect(frameStep(10, 5000)).toBe(frameStep(10, 64));
		expect(frameStep(10, -5)).toBe(0);
	});
});
