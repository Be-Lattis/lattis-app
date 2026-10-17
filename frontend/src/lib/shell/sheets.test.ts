import { describe, expect, it } from 'vitest';
import { moveTarget, tabWindow, textOn, visibleSheet, visibleSheets } from './sheets';

describe('feuilles masquées', () => {
	const sheets = [
		{ id: 1, name: 'A', hidden: true },
		{ id: 2, name: 'B' },
		{ id: 3, name: 'C' }
	];
	it('la barre ne montre que les feuilles visibles', () => {
		expect(visibleSheets(sheets).map((s) => s.id)).toEqual([2, 3]);
	});
	it('une feuille masquée ou disparue cède la place à la première visible', () => {
		expect(visibleSheet(sheets, 3)).toBe(3);
		expect(visibleSheet(sheets, 1)).toBe(2);
		expect(visibleSheet(sheets, 9)).toBe(2);
	});
});

describe('déplacer ou copier', () => {
	const order = [1, 2, 3, 4];
	it('déplacer avant une feuille', () => {
		expect(moveTarget(order, 4, 1, false)).toBe(0);
		expect(moveTarget(order, 1, 3, false)).toBe(1);
		expect(moveTarget(order, 2, 4, false)).toBe(2);
	});
	it('déplacer en dernier', () => {
		expect(moveTarget(order, 1, null, false)).toBe(3);
	});
	it('copier avant une feuille ou en dernier', () => {
		expect(moveTarget(order, 2, 1, true)).toBe(0);
		expect(moveTarget(order, 2, 3, true)).toBe(2);
		expect(moveTarget(order, 2, null, true)).toBe(4);
	});
});

describe('couleur du texte d’un onglet coloré', () => {
	it('blanc sur foncé, sombre sur clair', () => {
		expect(textOn('#002060')).toBe('#ffffff');
		expect(textOn('#ffff00')).toBe('#1f2328');
		expect(textOn('rouge')).toBe('#1f2328');
	});
});

describe('onglets qui tiennent dans la barre', () => {
	const w = [100, 100, 100, 100, 100];
	it('tous quand ils tiennent', () => {
		expect(tabWindow(w, 4, 500, 30)).toEqual({ from: 0, to: 4 });
	});
	it('depuis le premier si l’actif y tient, place gardée pour « … »', () => {
		expect(tabWindow(w, 1, 330, 30)).toEqual({ from: 0, to: 2 });
	});
	it('jusqu’à l’actif sinon', () => {
		expect(tabWindow(w, 4, 330, 30)).toEqual({ from: 2, to: 4 });
		expect(tabWindow(w, 3, 230, 30)).toEqual({ from: 2, to: 3 });
	});
	it('l’actif reste, même trop large', () => {
		expect(tabWindow(w, 2, 50, 30)).toEqual({ from: 2, to: 2 });
	});
});
