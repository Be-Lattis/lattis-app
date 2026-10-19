import { describe, expect, it } from 'vitest';
import { Axis } from './axis';
import { ShiftAxis, ease, flashAlpha, lerpSeries, motionAt, progress, sameSeries, shiftAxis, tableMotions, zeroBar, FLASH_MS } from './anim';

describe('ease, progress', () => {
	it('va de 0 à 1, bornée', () => {
		expect(ease(0)).toBe(0);
		expect(ease(1)).toBe(1);
		expect(ease(2)).toBe(1);
		expect(ease(-1)).toBe(0);
		expect(ease(0.5)).toBeGreaterThan(0.5);
		expect(progress(100, 100, 200)).toBe(0);
		expect(progress(200, 100, 200)).toBe(0.5);
		expect(progress(900, 100, 200)).toBe(1);
	});
});

describe('ShiftAxis', () => {
	const base = new Axis(1000, 20, new Map([[5, 40]]));

	it('insertion : la bande grandit, la suite glisse', () => {
		// Lignes 3 et 4 insérées, à moitié de leur taille.
		const a = new ShiftAxis(base, 3, 5, 0.5, 0);
		expect(a.offset(2)).toBe(40);
		expect(a.offset(3)).toBe(60);
		expect(a.offset(4)).toBe(70);
		expect(a.offset(5)).toBe(80);
		expect(a.size(3)).toBe(10);
		expect(a.size(5)).toBe(40);
		expect(a.total).toBe(base.total - 20);
		for (let i = 0; i < 12; i++) expect(a.indexAt(a.offset(i))).toBe(i);
	});

	it('suppression : un vide qui se referme', () => {
		const a = new ShiftAxis(base, 3, 3, 1, 30);
		expect(a.offset(2)).toBe(40);
		expect(a.offset(3)).toBe(90);
		expect(a.indexAt(65)).toBe(3);
		expect(a.indexAt(95)).toBe(3);
		for (let i = 0; i < 12; i++) expect(a.indexAt(a.offset(i))).toBe(i);
	});

	it('shiftAxis : axe d’arrivée une fois fini, bande nulle au départ', () => {
		expect(shiftAxis(base, { at: 3, n: 2, px: 0 }, 1)).toBe(base);
		const a = shiftAxis(base, { at: 3, n: 2, px: 0 }, 0);
		expect(a.offset(5)).toBe(base.offset(3));
		expect(a.indexAt(base.offset(3))).toBe(5);
		const d = shiftAxis(base, { at: 3, n: -2, px: 40 }, 0);
		expect(d.offset(3)).toBe(base.offset(3) + 40);
	});
});

describe('tableMotions', () => {
	const g = (x: number, y: number, w = 100, h = 50) => ({ x, y, w, h });

	it('bougés, agrandis, nés ; les autres non', () => {
		const drawn = new Map([
			[1, g(0, 0)],
			[2, g(200, 0)],
			[3, g(0, 100)]
		]);
		const now = new Map([
			[1, g(0, 0)],
			[2, g(300, 0)],
			[3, g(0, 100, 100, 70)],
			[4, g(500, 0)],
			[5, g(900, 0)]
		]);
		const m = tableMotions(drawn, new Set([1, 2, 3, 5]), now, 7);
		expect([...m.keys()].sort()).toEqual([2, 3, 4]);
		expect(m.get(2)).toEqual({ t0: 7, from: g(200, 0) });
		expect(m.get(4)?.born).toBe(true);
	});

	it('motionAt : interpolation, naissance en fondu depuis l’en-tête', () => {
		const to = g(100, 0, 100, 100);
		expect(motionAt({ t0: 0, from: g(0, 0, 100, 50) }, to, 0.5, 20)).toEqual({ x: 50, y: 0, w: 100, h: 75, alpha: 1 });
		const b = motionAt({ t0: 0, from: to, born: true }, to, 0, 20);
		expect(b.h).toBe(20);
		expect(b.alpha).toBe(0);
	});
});

describe('barres, éclat, séries', () => {
	it('zeroBar : sur l’axe', () => {
		expect(zeroBar({ a: 0, b: 0.7 })).toEqual({ a: 0, b: 0 });
		expect(zeroBar({ a: 0.2, b: 0.4, axis: 0.4, rev: true })).toEqual({ a: 0.4, b: 0.4 });
		expect(zeroBar({ a: 0.3, b: 1, rev: true })).toEqual({ a: 1, b: 1 });
	});

	it('flashAlpha : de 1 à 0', () => {
		expect(flashAlpha(0, 0)).toBe(1);
		expect(flashAlpha(FLASH_MS / 2, 0)).toBeLessThan(0.5);
		expect(flashAlpha(FLASH_MS, 0)).toBe(0);
	});

	it('lerpSeries, sameSeries', () => {
		const a = [{ name: 'A', values: [10, null, 4] }];
		const b = [{ name: 'A', values: [20, 5, 4, 8] }];
		expect(lerpSeries(a, b, 0.5)[0].values).toEqual([15, 2.5, 4, 4]);
		expect(lerpSeries(null, b, 0)[0].values).toEqual([0, 0, 0, 0]);
		expect(lerpSeries(a, b, 1)).toBe(b);
		expect(sameSeries(a, a.map((s) => ({ ...s, values: [...s.values] })))).toBe(true);
		expect(sameSeries(a, b)).toBe(false);
	});

	it('lerpSeries après un tri : chaque barre part de sa propre catégorie', () => {
		const a = [{ name: 'Ventes', values: [10, 20, 30] }];
		const b = [{ name: 'Ventes', values: [30, 10, 40] }];
		// FR, IT, ES triés en ES, FR, puis DE ajoutée.
		expect(lerpSeries(a, b, 0.5, ['FR', 'IT', 'ES'], ['ES', 'FR', 'DE'])[0].values).toEqual([30, 10, 20]);
		// Libellés répétés : par position.
		expect(lerpSeries(a, b, 0.5, ['FR', 'IT', 'ES'], ['FR', 'FR', 'IT'])[0].values).toEqual([20, 15, 35]);
		// Série retirée devant : la suivante garde ses valeurs (par nom).
		const two = [{ name: 'Coût', values: [1] }, { name: 'Ventes', values: [10] }];
		expect(lerpSeries(two, [{ name: 'Ventes', values: [20] }], 0.5)[0].values).toEqual([15]);
	});
});
