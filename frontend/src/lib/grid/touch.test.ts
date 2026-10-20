import { describe, expect, it } from 'vitest';
import { FLING_TAU, MAX_SPEED, Velocity, inertia, isDrag, pinchZoom } from './touch';

describe('isDrag', () => {
	it('un appui qui bouge à peine reste un appui', () => {
		expect(isDrag(3, 4)).toBe(false);
		expect(isDrag(6, 8)).toBe(true);
	});
});

describe('Velocity', () => {
	it('mesure la vitesse sur les derniers mouvements', () => {
		const v = new Velocity();
		for (let t = 0; t <= 200; t += 10) v.add(t, t * 2, -t);
		const { vx, vy } = v.get();
		expect(vx).toBeCloseTo(2);
		expect(vy).toBeCloseTo(-1);
	});

	it('vaut zéro quand le doigt s’est arrêté avant de se lever', () => {
		const v = new Velocity();
		v.add(0, 0, 0);
		v.add(10, 20, 0);
		expect(v.get(200)).toEqual({ vx: 0, vy: 0 });
	});

	it('borne la vitesse', () => {
		const v = new Velocity();
		v.add(0, 0, 0);
		v.add(1, 1000, 0);
		expect(v.get().vx).toBe(MAX_SPEED);
	});
});

describe('inertia', () => {
	it('freine et parcourt au plus v × tau', () => {
		let v = 2;
		let total = 0;
		for (let i = 0; i < 400; i++) {
			const s = inertia(v, 16);
			total += s.d;
			v = s.v;
		}
		expect(v).toBeLessThan(0.001);
		expect(total).toBeCloseTo(2 * FLING_TAU, 0);
	});

	it('ne dépend pas du découpage des images', () => {
		const a = inertia(1.5, 32);
		const b1 = inertia(1.5, 16);
		const b2 = inertia(b1.v, 16);
		expect(b1.d + b2.d).toBeCloseTo(a.d);
		expect(b2.v).toBeCloseTo(a.v);
	});
});

describe('pinchZoom', () => {
	it('suit le rapport des écarts, borné', () => {
		expect(pinchZoom(1, 100, 150)).toBe(1.5);
		expect(pinchZoom(1, 100, 50)).toBe(0.5);
		expect(pinchZoom(2, 100, 1000)).toBe(4);
		expect(pinchZoom(1.2, 0, 50)).toBe(1.2);
	});
});
