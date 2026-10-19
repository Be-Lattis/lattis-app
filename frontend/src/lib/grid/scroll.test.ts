import { describe, expect, it } from 'vitest';
import { MAX_SCROLL, VScroll } from './scroll';

describe('VScroll', () => {
	it('plafonne la zone native et met la position à l’échelle', () => {
		const vs = new VScroll(24_000_000, 600);
		expect(vs.spacer).toBe(MAX_SCROLL);
		vs.fromNative(MAX_SCROLL - 600);
		expect(vs.top).toBeCloseTo(vs.maxTop);
		vs.fromNative(0);
		expect(vs.top).toBe(0);
	});

	it('fait l’aller-retour natif ↔ virtuel', () => {
		const vs = new VScroll(24_000_000, 600);
		vs.to(12_345_678);
		expect(vs.inSync(vs.toNative())).toBe(true);
		expect(vs.inSync(vs.toNative() + 50)).toBe(false);
	});

	it('reste à l’échelle 1 pour un petit tableau', () => {
		const vs = new VScroll(2_400, 600);
		vs.fromNative(500);
		expect(vs.top).toBe(500);
		vs.by(10_000);
		expect(vs.top).toBe(1_800);
	});
});
