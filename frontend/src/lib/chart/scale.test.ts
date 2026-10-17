import { describe, expect, it } from 'vitest';
import { axisFormat, categoryLayout, legendFlow, parseNumber, tickLabel, ticks } from './scale';

// Espaces insécables de Intl (fr-FR) ramenées à des espaces simples.
const plain = (s: string) => s.replace(/[  ]/g, ' ');

describe('graduations', () => {
	it('rondes et couvrant les données', () => {
		expect(ticks(0, 17)).toEqual([0, 5, 10, 15, 20]);
		expect(ticks(3, 9.5, 4)).toEqual([0, 2.5, 5, 7.5, 10]);
		expect(ticks(-12, 30)).toEqual([-20, -10, 0, 10, 20, 30]);
		expect(ticks(0, 0)).toEqual([0, 0.2, 0.4, 0.6, 0.8, 1]);
	});
	it('libellés et nombres', () => {
		expect(tickLabel(2.5)).toBe('2,5');
		expect(parseNumber('1 234,5')).toBe(1234.5);
		expect(parseNumber('FR')).toBeNull();
	});
	it('un seul format par axe', () => {
		const big = [0, 2000, 4000, 6000, 8000, 10000, 12000];
		const f = axisFormat(big);
		expect(big.map((v) => plain(f(v)))).toEqual(['0', '2 k', '4 k', '6 k', '8 k', '10 k', '12 k']);
		const half = [0, 2500, 5000, 7500, 10000];
		const g = axisFormat(half);
		expect(half.map((v) => plain(g(v)))).toEqual(['0', '2,5 k', '5,0 k', '7,5 k', '10,0 k']);
		const small = [0, 2000, 4000, 6000, 8000];
		expect(small.map((v) => plain(axisFormat(small)(v)))).toEqual(['0', '2 000', '4 000', '6 000', '8 000']);
		const dec = [0, 0.5, 1, 1.5];
		expect(dec.map((v) => axisFormat(dec)(v))).toEqual(['0,0', '0,5', '1,0', '1,5']);
	});
});

describe('étiquettes de catégories', () => {
	const services = ['Commercial', 'Comptabilité', 'Marketing', 'Logistique'];
	it('droites quand elles tiennent', () => {
		expect(categoryLayout(['A', 'B', 'C'], 80)).toMatchObject({ every: 1, angle: 0 });
	});
	it('inclinées plutôt qu’une sur deux', () => {
		const l = categoryLayout(services, 50);
		expect(l.every).toBe(1);
		expect(l.angle).toBe(-45);
		expect(l.depth).toBeGreaterThan(40);
	});
	it('une sur plusieurs seulement si même inclinées elles se chevauchent', () => {
		expect(categoryLayout(services, 8).every).toBe(3);
	});
});

describe('légende sur toute la largeur', () => {
	it('enchaîne les entrées puis passe à la rangée suivante, libellés complets', () => {
		const f = legendFlow(['Informatique', 'Audio', 'Stockage', 'Mobilier'], 220, 4);
		expect(f.items.map((i) => i.text)).toEqual(['Informatique', 'Audio', 'Stockage', 'Mobilier']);
		expect(f.rows).toBe(2);
		expect(f.items[2].row).toBe(1);
		expect(f.items[2].x).toBe(10);
	});
	it('omet ce qui dépasse le nombre de rangées', () => {
		expect(legendFlow(['a', 'b', 'c'], 60, 1).items.length).toBeLessThan(3);
	});
});
