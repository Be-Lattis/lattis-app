import { describe, expect, it } from 'vitest';
import { EMPTY_COLOR, MAX_COLORS, OTHER_COLOR, categoryColors, categoryHue, colorCategories, colorDistance, groupColor, hueSlot, oklchHex } from './categories';

const hueGap = (a: number, b: number) => {
	const d = Math.abs(a - b) % 360;
	return Math.min(d, 360 - d);
};

describe('couleurs par catégorie', () => {
	it('chaque rang a sa teinte : toutes les places du spectre sont prises', () => {
		for (let n = 1; n <= 24; n++) {
			const slots = new Set(Array.from({ length: n }, (_, k) => hueSlot(k, n)));
			expect(slots.size).toBe(n);
		}
	});

	it('teintes les plus écartées possible (n ≤ 12 : au moins 360/n degrés)', () => {
		for (let n = 2; n <= 12; n++) {
			const hues = Array.from({ length: n }, (_, k) => categoryHue(k, n));
			let min = Infinity;
			for (let i = 0; i < n; i++) for (let j = i + 1; j < n; j++) min = Math.min(min, hueGap(hues[i], hues[j]));
			expect(min).toBeGreaterThanOrEqual(360 / n - 1e-9);
		}
	});

	it('deux rangs voisins (voisins dans la légende) ont des teintes éloignées', () => {
		for (let n = 5; n <= 20; n++) {
			for (let k = 0; k + 1 < n; k++) expect(hueGap(categoryHue(k, n), categoryHue(k + 1, n))).toBeGreaterThanOrEqual(360 / n);
			// Bien plus que la teinte voisine sur le cercle (au moins 60°).
			for (let k = 0; k + 1 < n; k++) expect(hueGap(categoryHue(k, n), categoryHue(k + 1, n))).toBeGreaterThanOrEqual(60 - 1e-9);
		}
	});

	it('couleurs distinctes à l’œil (écart OKLab), jusqu’à 20 valeurs', () => {
		for (let n = 2; n <= MAX_COLORS; n++) {
			const c = categoryColors(n);
			expect(new Set(c).size).toBe(n);
			let min = Infinity;
			for (let i = 0; i < n; i++) for (let j = i + 1; j < n; j++) min = Math.min(min, colorDistance(c[i], c[j]));
			expect(min, `n = ${n}`).toBeGreaterThan(n <= 12 ? 0.06 : 0.035);
			// Aucune ne se confond avec le gris des « Autres ».
			for (const x of c) expect(colorDistance(x, OTHER_COLOR)).toBeGreaterThan(0.05);
		}
	});

	it('couleurs valides, dans sRGB', () => {
		expect(oklchHex(0.63, 0.4, 250)).toMatch(/^#[0-9a-f]{6}$/);
		expect(oklchHex(1, 0, 0)).toBe('#ffffff');
		for (const c of categoryColors(20)) expect(c).toMatch(/^#[0-9a-f]{6}$/);
	});

	it('stable : même jeu de valeurs, mêmes couleurs, quel que soit l’ordre des points', () => {
		const cats = ['Bretagne', 'Normandie', 'Occitanie'];
		const a = colorCategories(cats, [0, 1, 2, 2]);
		const b = colorCategories(cats, [2, 2, 1, 0]);
		expect(a.colors).toEqual(b.colors);
		expect(colorCategories(cats, [0]).colors).toEqual(a.colors);
		expect(categoryColors(7)).toEqual(categoryColors(7));
	});

	it('légende : valeurs triées, vides à part, au-delà du maximum « Autres »', () => {
		const c = colorCategories(['A', 'B'], [0, 1, 1, -1]);
		expect(c.legend.map((e) => [e.label, e.count])).toEqual([
			['A', 1],
			['B', 2],
			['(vide)', 1]
		]);
		expect(groupColor(c, -1)).toBe(EMPTY_COLOR);
		expect(groupColor(c, 1)).toBe(c.colors[1]);

		// 25 valeurs : la dernière très fréquente garde sa couleur, 6 rares deviennent « Autres ».
		const cats = Array.from({ length: 25 }, (_, i) => `V${String(i).padStart(2, '0')}`);
		const groups = [...cats.map((_, i) => i), 24, 24, 24];
		const many = colorCategories(cats, groups);
		const colored = many.colors.filter((x) => x !== OTHER_COLOR);
		expect(colored.length).toBe(MAX_COLORS - 1);
		expect(many.colors[24]).not.toBe(OTHER_COLOR);
		expect(many.colors[0]).not.toBe(OTHER_COLOR);
		expect(many.legend.at(-1)).toEqual({ label: 'Autres (6 valeurs)', color: OTHER_COLOR, count: 6 });
		expect(many.legend.length).toBe(MAX_COLORS);
	});
});
