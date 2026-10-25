import { describe, expect, it } from 'vitest';
import { a1, defaultLocation, freeSparkAt, sparkGeometry, sparkSvg, sparkValue, SPARK_COLORS } from './sparkline';

describe('géométrie des sparklines', () => {
	it('trace une courbe sur toute la cellule, coupée aux cellules vides', () => {
		const s = sparkGeometry([1, 3, null, 2, 5], { kind: 'line' }, 106, 26);
		expect(s.lines.length).toBe(2);
		expect(s.lines[0][0]).toEqual({ x: 3, y: 23 }); // minimum en bas, marge de 3
		expect(s.lines[1][1]).toEqual({ x: 103, y: 3 }); // maximum en haut, à droite
		expect(s.points).toEqual([]);
	});
	it('met en valeur haut, bas, premier, dernier et négatifs', () => {
		const s = sparkGeometry([2, -1, 4, 4, 0], { kind: 'line', high: true, low: true, first: true, last: true, axis: true }, 50, 20);
		const at = (x: number) => s.points.find((p) => Math.abs(p.x - x) < 0.01)?.color;
		expect(s.points.length).toBe(5);
		expect(at(3)).toBe(SPARK_COLORS.firstColor);
		expect(at(14)).toBe(SPARK_COLORS.lowColor);
		expect(at(25)).toBe(SPARK_COLORS.highColor);
		expect(at(36)).toBe(SPARK_COLORS.highColor); // les deux maximums
		expect(at(47)).toBe(SPARK_COLORS.lastColor);
		expect(s.axis).not.toBeNull();
	});
	it('dessine l’histogramme depuis zéro, négatifs en couleur', () => {
		const s = sparkGeometry([4, -2, 2], { kind: 'column', negative: true }, 36, 26);
		expect(s.bars.length).toBe(3);
		const base = s.bars[0].y + s.bars[0].h;
		expect(s.bars[1].y).toBeCloseTo(base); // sous l'axe
		expect(s.bars[1].color).toBe(SPARK_COLORS.negColor);
		expect(s.bars[0].h).toBeCloseTo(2 * s.bars[2].h);
	});
	it('positif/négatif : même hauteur, au-dessus ou au-dessous du milieu', () => {
		const s = sparkGeometry([5, -1, 0, 9], { kind: 'winloss' }, 43, 20);
		expect(s.bars.length).toBe(3); // zéro : rien
		expect(s.bars[0].h).toBe(s.bars[1].h);
		expect(s.bars[0].y + s.bars[0].h).toBe(10);
		expect(s.bars[1].y).toBe(10);
		expect(s.bars[1].color).toBe(SPARK_COLORS.negColor);
	});
	it('suit l’échelle commune et les bornes fixes', () => {
		const a = sparkGeometry([5, 10], { kind: 'line' }, 20, 26, { lo: 0, hi: 20 });
		expect(a.lines[0][1].y).toBeCloseTo(13);
		const b = sparkGeometry([5, 10], { kind: 'line', min: 5, max: 15 }, 20, 26);
		expect(b.lines[0][0].y).toBeCloseTo(23);
	});
	it('ne dessine rien sans valeur ni place', () => {
		expect(sparkGeometry([null, null], { kind: 'line' }, 50, 20).lines).toEqual([]);
		expect(sparkGeometry([1, 2], { kind: 'column' }, 4, 20).bars).toEqual([]);
	});
	it('écrit le même dessin en SVG (impression)', () => {
		const svg = sparkSvg(sparkGeometry([1, 2, 3], { kind: 'line', last: true }, 30, 20), 30, 20);
		expect(svg).toContain('<polyline');
		expect(svg).toContain(`fill="${SPARK_COLORS.lastColor}"`);
		expect(sparkSvg(sparkGeometry([1, -2], { kind: 'column' }, 30, 20), 30, 20).match(/<rect/g)?.length).toBe(2);
	});
});

describe('aides', () => {
	it('lit les valeurs des cellules de tableau', () => {
		expect([sparkValue(3), sparkValue('4.5'), sparkValue(''), sparkValue('abc'), sparkValue(null), sparkValue(NaN)]).toEqual([3, 4.5, null, null, null, null]);
	});
	it('écrit les plages et propose l’emplacement', () => {
		expect(a1(1, 1, 4, 12)).toBe('B2:M5');
		expect(a1(0, 27, 0, 27)).toBe('AB1');
		expect(defaultLocation(1, 1, 4, 12)).toBe('N2:N5');
		expect(defaultLocation(1, 2, 6, 2)).toBe('C8');
	});
	it('trouve le graphique d’une cellule libre', () => {
		const g = { id: 1, config: { kind: 'line' }, atCol: -1, cells: [{ row: 3, col: 4, values: [1, 2] }] };
		expect(freeSparkAt([g], 3, 4)?.values).toEqual([1, 2]);
		expect(freeSparkAt([g], 3, 5)).toBeNull();
	});
});
