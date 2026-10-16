import { describe, expect, it } from 'vitest';
import { AutoRows } from './rowfit';

describe('AutoRows', () => {
	it('mesure une fois les lignes affichées et chargées', () => {
		const a = new AutoRows();
		let calls = 0;
		const measure = (r: number) => {
			calls++;
			return r === 3 ? undefined : r === 2 ? 60 : 0;
		};
		const ch = a.measure(0, 4, measure);
		expect([...ch]).toEqual([[2, 60]]);
		expect(calls).toBe(5);
		// Seule la ligne pas encore chargée est redemandée.
		a.measure(0, 4, measure);
		expect(calls).toBe(6);
		expect(a.height(2)).toBe(60);
	});

	it('combine cellules libres et tableaux (la plus grande hauteur)', () => {
		const a = new AutoRows();
		a.free = new Map([[1, 40]]);
		a.measure(0, 2, (r) => (r === 1 ? 30 : r === 2 ? 50 : 0));
		expect(a.height(1)).toBe(40);
		expect(a.height(2)).toBe(50);
		expect([...a.all()].sort()).toEqual([
			[1, 40],
			[2, 50]
		]);
	});

	it('remesure après forget, et rend la hauteur par défaut quand le contenu rapetisse', () => {
		const a = new AutoRows();
		a.measure(10, 10, () => 70);
		expect(a.measure(10, 10, () => 0).size).toBe(0); // déjà mesurée
		a.forget(0, 200);
		const ch = a.measure(10, 10, () => 0);
		expect(ch.has(10)).toBe(true);
		expect(ch.get(10)).toBeUndefined();
		expect(a.height(10)).toBeUndefined();
	});

	it('forgetAll(keep) oublie tout de suite les lignes sorties des tableaux', () => {
		const a = new AutoRows();
		a.measure(0, 5, () => 50);
		a.forgetAll((r) => r < 3);
		expect(a.height(2)).toBe(50);
		expect(a.height(4)).toBeUndefined();
	});

	it('ne mesure pas plus de max lignes d’un coup', () => {
		const a = new AutoRows();
		let calls = 0;
		a.measure(0, 1_000_000, () => (calls++, 0), 100);
		expect(calls).toBe(100);
	});
});
