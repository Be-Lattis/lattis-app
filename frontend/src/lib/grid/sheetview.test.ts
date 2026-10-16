import { describe, expect, it } from 'vitest';
import { HOME, SheetViews } from './sheetview';

const at = (row: number, col: number, top = 0): typeof HOME => ({ active: { row, col }, anchor: { row, col }, top, left: 0 });

describe('position par feuille', () => {
	it('première feuille : rien à restaurer', () => {
		const v = new SheetViews();
		expect(v.switchTo(1, at(0, 0))).toBeNull();
		expect(v.switchTo(1, at(5, 5))).toBeNull(); // même feuille (relecture)
	});
	it('feuille jamais visitée : A1 ; retour : la position quittée', () => {
		const v = new SheetViews();
		v.switchTo(1, HOME);
		expect(v.switchTo(2, at(100, 2, 2400))).toEqual(HOME);
		expect(v.switchTo(1, at(3, 1))).toEqual(at(100, 2, 2400));
		expect(v.switchTo(2, at(0, 0))).toEqual(at(3, 1));
	});
	it('la vue rendue est une copie', () => {
		const v = new SheetViews();
		v.switchTo(1, HOME);
		const r = v.switchTo(2, at(4, 4))!;
		r.active.row = 99;
		expect(HOME.active.row).toBe(0);
		const back = v.switchTo(1, r)!;
		expect(back).toEqual(at(4, 4));
		back.active.row = 50;
		expect(v.switchTo(2, back)!.active.row).toBe(99);
		expect(v.switchTo(1, HOME)!.active.row).toBe(50);
	});
});
