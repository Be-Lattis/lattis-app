import { describe, expect, it } from 'vitest';
import { Axis } from './axis';

describe('Axis', () => {
	const a = new Axis(1000, 100, new Map([[2, 150], [3, 50]]));

	it('cumule les tailles particulières', () => {
		expect(a.offset(0)).toBe(0);
		expect(a.offset(2)).toBe(200);
		expect(a.offset(3)).toBe(350);
		expect(a.offset(4)).toBe(400);
		expect(a.offset(10)).toBe(1000);
		expect(a.total).toBe(100_000);
	});

	it('retrouve la case d’une position', () => {
		expect(a.indexAt(-5)).toBe(0);
		expect(a.indexAt(199)).toBe(1);
		expect(a.indexAt(200)).toBe(2);
		expect(a.indexAt(349)).toBe(2);
		expect(a.indexAt(350)).toBe(3);
		expect(a.indexAt(399)).toBe(3);
		expect(a.indexAt(400)).toBe(4);
		expect(a.indexAt(10_000_000)).toBe(999);
		for (let i = 0; i < 20; i++) expect(a.indexAt(a.offset(i))).toBe(i);
	});
});

describe('Axis : beaucoup de tailles particulières', () => {
	// Référence : calcul naïf case par case.
	const naive = (sizes: Map<number, number>, def: number) => (i: number) => {
		let px = 0;
		for (let k = 0; k < i; k++) px += sizes.get(k) ?? def;
		return px;
	};

	it('donne les mêmes positions qu’un calcul naïf, cases nulles comprises', () => {
		const sizes = new Map<number, number>();
		for (let i = 0; i < 300; i += 3) sizes.set(i, (i * 7) % 5 === 0 ? 0 : 10 + (i % 40));
		sizes.set(301, 0);
		sizes.set(302, 0);
		const a = new Axis(400, 22, sizes);
		const offset = naive(sizes, 22);
		for (let i = 0; i <= 400; i++) expect(a.offset(i)).toBe(offset(i));
		for (let i = 0; i < 400; i++) if (a.size(i) > 0) expect(a.indexAt(a.offset(i))).toBe(i);
		// Une case masquée (taille nulle) n'est jamais trouvée.
		for (let px = 0; px < a.total; px += 5) expect(a.size(a.indexAt(px))).toBeGreaterThan(0);
	});

	it('with change quelques tailles sans toucher aux autres', () => {
		const a = new Axis(100, 20, new Map([[5, 40]]));
		const b = a.with(new Map<number, number | undefined>([[2, 60], [5, undefined]]));
		expect(b.offset(3)).toBe(100);
		expect(b.size(5)).toBe(20);
		expect(a.size(5)).toBe(40); // l'axe d'origine ne change pas
		expect(a.with(new Map())).toBe(a);
	});

	it('reste rapide avec des milliers de lignes ajustées sur un million', () => {
		const sizes = new Map<number, number>();
		for (let i = 0; i < 50_000; i++) sizes.set(i * 20, 44);
		const a = new Axis(1_048_576, 22, sizes);
		const t0 = performance.now();
		let s = 0;
		for (let i = 0; i < 100_000; i++) s += a.indexAt(a.offset((i * 9973) % 1_048_576));
		expect(s).toBeGreaterThan(0);
		expect(performance.now() - t0).toBeLessThan(1000);
	});
});
