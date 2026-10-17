import { describe, expect, it } from 'vitest';
import { boxGroups, boxStats, fitView, gaugeValue, heatColor, heatGrid, histogramBins, niceStep, pie3d, project, quantile, shade, squarify, viewTiles, waterfallSteps } from './shapes';
import { KINDS, MORE_KINDS, guessLatLon, seriesShape } from './kinds';
import { ticks } from './scale';
import { loadTile, setTileFetcher } from './tiles';

const plain = (s: string) => s.replace(/[  ]/g, ' ');

describe('fréquences', () => {
	it('classes rondes, communes aux séries', () => {
		const b = histogramBins([[1, 2, 2, 3, 9, 10], [5, null]]);
		expect(b.edges).toEqual([0, 2.5, 5, 7.5, 10]);
		expect(b.counts).toEqual([
			[3, 1, 0, 2],
			[0, 0, 1, 0]
		]);
		expect(b.labels.map(plain)).toEqual(['0 – 2,5', '2,5 – 5', '5 – 7,5', '7,5 – 10']);
	});
	it('nombre de classes imposé, valeur unique, rien', () => {
		expect(histogramBins([[0, 10, 20, 30, 40]], 4).edges).toEqual([0, 10, 20, 30, 40]);
		expect(histogramBins([[7, 7, 7]]).counts[0].reduce((a, v) => a + v, 0)).toBe(3);
		expect(histogramBins([[null]]).labels).toEqual([]);
		expect(niceStep(3.2)).toBe(5);
		expect(niceStep(0.17)).toBe(0.2);
	});
});

describe('boîtes à moustaches', () => {
	it('quartiles comme QUARTILE.INCLURE, valeurs extrêmes', () => {
		expect(quantile([1, 2, 3, 4], 0.25)).toBe(1.75);
		const b = boxStats([1, 2, 3, 4, 5, 6, 7, 8, 100, null])!;
		expect([b.q1, b.median, b.q3]).toEqual([3, 5, 7]);
		expect(b.outliers).toEqual([100]);
		expect(b.max).toBe(8);
		expect(b.n).toBe(9);
		expect(boxStats([null])).toBeNull();
	});
	it('par catégorie si les libellés se répètent, sinon par série', () => {
		const g = boxGroups(['A', 'B', 'A'], [{ name: 'v', values: [1, 2, 3] }]);
		expect(g.labels).toEqual(['A', 'B']);
		expect(g.boxes[0][0]!.median).toBe(2);
		const s = boxGroups(['x', 'y'], [
			{ name: 'P', values: [1, 2] },
			{ name: 'Q', values: [3, 4] }
		]);
		expect(s.labels).toEqual(['P', 'Q']);
		expect(s.boxes[1][0]).toBeNull();
		expect(s.boxes[1][1]!.median).toBe(3.5);
	});
});

describe('cascade', () => {
	it('chaque marche part du cumul, total final', () => {
		expect(waterfallSteps([10, -4, null, 3])).toEqual([
			{ from: 0, to: 10, kind: 'up', value: 10 },
			{ from: 10, to: 6, kind: 'down', value: -4 },
			{ from: 6, to: 6, kind: 'up', value: 0 },
			{ from: 6, to: 9, kind: 'up', value: 3 },
			{ from: 0, to: 9, kind: 'total', value: 9 }
		]);
	});
});

describe('compartimentage', () => {
	it('aires proportionnelles, dans le cadre, sans les valeurs nulles', () => {
		const t = squarify([6, 6, 4, 3, 2, 2, 1, 0, -5, null], 0, 0, 60, 40);
		expect(t.map((r) => r.i).sort()).toEqual([0, 1, 2, 3, 4, 5, 6]);
		const total = 24;
		for (const r of t) {
			expect(r.w * r.h).toBeCloseTo((([6, 6, 4, 3, 2, 2, 1][r.i] as number) / total) * 2400, 6);
			expect(r.x).toBeGreaterThanOrEqual(-1e-9);
			expect(r.y).toBeGreaterThanOrEqual(-1e-9);
			expect(r.x + r.w).toBeLessThanOrEqual(60 + 1e-9);
			expect(r.y + r.h).toBeLessThanOrEqual(40 + 1e-9);
		}
		expect(squarify([0, null], 0, 0, 10, 10)).toEqual([]);
	});
});

describe('jauge, perspective', () => {
	it('dernière valeur, bornes rondes', () => {
		expect(gaugeValue([10, 37, null])).toEqual({ value: 37, lo: 0, hi: 40, frac: 37 / 40 });
		expect(gaugeValue([150], 0, 100).frac).toBe(1);
		expect(gaugeValue([null]).value).toBeNull();
	});
	it('secteurs 3D : parois devant seulement', () => {
		const s = pie3d([1, 1, 2], 100, 100, 80, 40, 12);
		expect(s.map((p) => p.part)).toEqual([0.25, 0.25, 0.5]);
		// Premier quart (de midi à 3 h) : derrière, sans paroi ; deuxième (3 h à 6 h) : devant.
		expect(s[0].walls).toHaveLength(0);
		expect(s[1].walls).toHaveLength(1);
		expect(s[2].walls).toHaveLength(1);
		expect(pie3d([0, null], 0, 0, 1, 1, 1)).toEqual([]);
	});
	it('teintes', () => {
		expect(shade('#808080', 0.5)).toBe('#c0c0c0');
		expect(shade('#808080', -0.5)).toBe('#404040');
		expect(shade('red', 0.5)).toBe('red');
		expect(heatColor(0)).toBe('rgb(49, 130, 189)');
		expect(heatColor(1)).toBe('rgb(227, 26, 28)');
	});
	it('graduations sans zéro (boîtes)', () => {
		expect(ticks(1000, 1100, 5, false)).toEqual([1000, 1020, 1040, 1060, 1080, 1100]);
	});
});

describe('carte', () => {
	it('Mercator : le monde tient dans une tuile au zoom 0', () => {
		expect(project(0, 0, 0)).toEqual({ x: 128, y: 128 });
		const p = project(48.8566, 2.3522, 10);
		expect(Math.floor(p.x / 256)).toBe(518);
		expect(Math.floor(p.y / 256)).toBe(352);
	});
	it('la vue montre tous les points, au zoom le plus fort', () => {
		const pts = [
			{ lat: 48.8566, lon: 2.3522 },
			{ lat: 45.764, lon: 4.8357 },
			{ lat: 43.2965, lon: 5.3698 }
		];
		const v = fitView(pts, 480, 288);
		for (const p of pts) {
			const q = project(p.lat, p.lon, v.z);
			expect(q.x - v.x0).toBeGreaterThanOrEqual(0);
			expect(q.x - v.x0).toBeLessThanOrEqual(480);
			expect(q.y - v.y0).toBeGreaterThanOrEqual(0);
			expect(q.y - v.y0).toBeLessThanOrEqual(288);
		}
		const tighter = fitView(pts, 480, 288, 24, v.z + 1);
		expect(tighter.z).toBe(v.z);
		expect(fitView([pts[0]], 480, 288).z).toBe(12);
		expect(fitView([], 480, 288).z).toBe(5);
	});
	it('tuiles de la vue, repliées autour du globe', () => {
		const t = viewTiles({ z: 1, x0: -100, y0: 0 }, 300, 256);
		expect(t.map((q) => `${q.x}/${q.y}@${q.left}`)).toEqual(['1/0@-156', '0/0@100']);
		expect(viewTiles({ z: 0, x0: 0, y0: -300 }, 256, 256)).toEqual([]);
	});
	it('densité : maximum sur le point le plus lourd', () => {
		const g = heatGrid(
			[
				{ x: 15, y: 15, w: 1 },
				{ x: 45, y: 15, w: 3 }
			],
			60,
			30,
			6,
			12
		);
		expect(g.cols).toBe(10);
		expect(g.rows).toBe(5);
		const best = g.values.indexOf(g.max);
		expect(best % g.cols).toBe(7);
		expect(g.max).toBeGreaterThan(2);
	});
	it('tuiles : par la source branchée, échec gardé un temps', async () => {
		let calls = 0;
		setTileFetcher(async (z, x, y) => {
			calls++;
			if (x === 9) throw new Error('hôte non autorisé');
			return `data:image/png;base64,${z}${x}${y}`;
		});
		expect(await loadTile(1, 0, 1)).toBe('data:image/png;base64,101');
		expect(await loadTile(1, 0, 1)).toBe('data:image/png;base64,101');
		expect(await loadTile(4, 9, 1)).toBeNull();
		expect(await loadTile(4, 9, 1)).toBeNull();
		expect(calls).toBe(2);
		setTileFetcher(null);
		expect(await loadTile(1, 0, 1)).toBeNull();
	});
	it('tuiles : deux demandes à la fois au plus', async () => {
		let live = 0;
		let peak = 0;
		const wait: (() => void)[] = [];
		setTileFetcher(
			() =>
				new Promise((resolve) => {
					live++;
					peak = Math.max(peak, live);
					wait.push(() => {
						live--;
						resolve('data:image/png;base64,');
					});
				})
		);
		const all = Promise.all([0, 1, 2, 3, 4].map((i) => loadTile(3, i, 0)));
		for (let n = 0; n < 5; n++) {
			await new Promise((r) => setTimeout(r, 0));
			wait.shift()?.();
		}
		await all;
		expect(peak).toBe(2);
		setTileFetcher(null);
	});
});

describe('types', () => {
	it('tous proposés, les nouveaux dans « Autres graphiques »', () => {
		expect(KINDS).toHaveLength(18);
		expect(MORE_KINDS.map((k) => k.kind)).not.toContain('bar');
		expect(MORE_KINDS.map((k) => k.kind)).toContain('map');
		expect(new Set(KINDS.map((k) => k.label)).size).toBe(KINDS.length);
	});
	it('combiné : la première série en barres, les autres en courbes', () => {
		expect([0, 1, 2].map((k) => seriesShape(undefined, k))).toEqual(['bar', 'line', 'line']);
		expect(seriesShape(['line', 'area'], 1)).toBe('area');
		expect(seriesShape(['pie'], 0)).toBe('bar');
	});
	it('colonnes de carte devinées', () => {
		expect(guessLatLon(['Ville', 'Latitude', 'Longitude'])).toEqual({ lat: 'Latitude', lon: 'Longitude' });
		expect(guessLatLon(['lat', 'lng', 'Pop'])).toEqual({ lat: 'lat', lon: 'lng' });
		expect(guessLatLon(['Ville', 'Pop'])).toEqual({ lat: undefined, lon: undefined });
	});
});
