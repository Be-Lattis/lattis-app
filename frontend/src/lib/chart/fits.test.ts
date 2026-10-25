import { describe, expect, it } from 'vitest';
import { bubbleRadii, candles, equationText, errorSpan, r2Text, trendEquation } from './fits';

const num = (v: number) => String(Math.round(v * 1e4) / 1e4).replace('.', ',');

describe('équation de la courbe de tendance', () => {
	it('écrit droite et polynôme comme un tableur', () => {
		expect(equationText(trendEquation({ type: 'linear', coef: [1, 2], r2: 1, points: [] }, num))).toBe('y = 2x + 1');
		expect(equationText(trendEquation({ type: 'linear', coef: [-1.5, 1], r2: 1, points: [] }, num))).toBe('y = x − 1,5');
		expect(equationText(trendEquation({ type: 'poly', coef: [3, -2, 0.1], r2: 1, points: [] }, num))).toBe('y = 0,1x² − 2x + 3');
		expect(equationText(trendEquation({ type: 'poly', coef: [0, 0, -1, 0, 0, 0, 2], r2: 1, points: [] }, num))).toBe('y = 2x⁶ − x²');
	});
	it('met en exposant exponentielle et puissance', () => {
		const e = trendEquation({ type: 'exp', coef: [2, 0.5], r2: 1, points: [] }, num);
		expect(e[1].sup).toBe(true);
		expect(equationText(e)).toBe('y = 2e^(0,5x)');
		expect(equationText(trendEquation({ type: 'power', coef: [4, 1.5], r2: 1, points: [] }, num))).toBe('y = 4x^(1,5)');
		expect(equationText(trendEquation({ type: 'log', coef: [3, -1], r2: 1, points: [] }, num))).toBe('y = 3ln(x) − 1');
		expect(trendEquation({ type: 'movavg', coef: null, r2: 0, points: [] }, num)).toEqual([]);
	});
	it('écrit le R² à 4 décimales', () => {
		expect(r2Text(0.98765, (v, d) => v.toFixed(d))).toBe('R² = 0.9877');
	});
});

describe('barres d’erreur', () => {
	it('part de la valeur, ou de la moyenne (écart type)', () => {
		const e = { plus: [1, null, 2], minus: [0.5, null, null] };
		expect(errorSpan(e, 0, 10)).toEqual({ lo: 9.5, hi: 11, plus: true, minus: true });
		expect(errorSpan(e, 1, 10)).toBeNull();
		expect(errorSpan(e, 2, 10)).toEqual({ lo: 10, hi: 12, plus: true, minus: false });
		expect(errorSpan({ ...e, center: 4 }, 0, 10)).toEqual({ lo: 3.5, hi: 5, plus: true, minus: true });
		expect(errorSpan(e, 0, null)).toBeNull();
	});
});

describe('bulles et bougies', () => {
	it('donne aux bulles une aire proportionnelle à la taille', () => {
		const r = bubbleRadii([100, 25, 0, null, -4], 20);
		expect(r[0]).toBe(20);
		expect(r[1]).toBeCloseTo(10);
		expect(r.slice(2)).toEqual([0, 0, 0]);
	});
	it('lit ouverture, haut, bas, clôture (ou haut, bas, clôture)', () => {
		const s = (v: (number | null)[]) => ({ values: v });
		const c = candles([s([10, 12]), s([15, 13]), s([9, 8]), s([14, 9])]);
		expect(c[0]).toEqual({ open: 10, high: 15, low: 9, close: 14, up: true });
		expect(c[1]?.up).toBe(false);
		const hlc = candles([s([5, null]), s([2, null]), s([4, null])]);
		expect(hlc[0]).toEqual({ open: null, high: 5, low: 2, close: 4, up: true });
		expect(hlc[1]).toBeNull();
	});
});
