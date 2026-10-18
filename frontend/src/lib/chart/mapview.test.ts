import { describe, expect, it } from 'vitest';
import { MAX_ZOOM, MIN_ZOOM, camView, clampCam, clampZoom, fallbackTiles, heatRadius, panBy, parseCam, roundCam, unproject, viewCam, wheelZoom, zoomAt } from './mapview';
import { fitView, project, tileZoom, viewTiles } from './shapes';

const W = 480;
const H = 288;
const paris = { lat: 48.8566, lon: 2.3522 };

/** Position à l'écran d'un point géographique dans la vue c. */
function screen(c: { lat: number; lon: number; zoom: number }, lat: number, lon: number) {
	const v = camView(c, W, H);
	const p = project(lat, lon, c.zoom);
	return { x: p.x - v.x0, y: p.y - v.y0 };
}

describe('vue de carte', () => {
	it('unproject inverse project', () => {
		for (const z of [0, 3.5, 12]) {
			const p = project(paris.lat, paris.lon, z);
			const q = unproject(p.x, p.y, z);
			expect(q.lat).toBeCloseTo(paris.lat, 9);
			expect(q.lon).toBeCloseTo(paris.lon, 9);
		}
	});
	it('vue et centre se correspondent', () => {
		const c = { ...paris, zoom: 9.3 };
		const back = viewCam(camView(c, W, H), W, H);
		expect(back.lat).toBeCloseTo(c.lat, 9);
		expect(back.lon).toBeCloseTo(c.lon, 9);
		expect(back.zoom).toBe(9.3);
		// Le centre est au milieu de l'écran.
		const s = screen(c, paris.lat, paris.lon);
		expect(s.x).toBeCloseTo(W / 2, 6);
		expect(s.y).toBeCloseTo(H / 2, 6);
	});
	it('zoomer autour d’un point le garde sous le curseur', () => {
		const c = { lat: 46, lon: 3, zoom: 6 };
		const px = 100;
		const py = 70;
		const v = camView(c, W, H);
		const under = unproject(v.x0 + px, v.y0 + py, c.zoom);
		for (const z of [7, 8.75, 4.2, 12]) {
			const d = zoomAt(c, W, H, px, py, z);
			expect(d.zoom).toBe(z);
			const s = screen(d, under.lat, under.lon);
			expect(s.x).toBeCloseTo(px, 6);
			expect(s.y).toBeCloseTo(py, 6);
		}
		// Au centre : le centre ne bouge pas.
		const m = zoomAt(c, W, H, W / 2, H / 2, 9);
		expect(m.lat).toBeCloseTo(c.lat, 9);
		expect(m.lon).toBeCloseTo(c.lon, 9);
	});
	it('le zoom est borné', () => {
		expect(clampZoom(-3)).toBe(MIN_ZOOM);
		expect(clampZoom(40)).toBe(MAX_ZOOM);
		expect(clampZoom(7.5)).toBe(7.5);
		expect(zoomAt({ ...paris, zoom: 17 }, W, H, 10, 10, 25).zoom).toBe(MAX_ZOOM);
		expect(zoomAt({ ...paris, zoom: 2 }, W, H, 10, 10, -5).zoom).toBe(MIN_ZOOM);
		expect(clampCam({ lat: 89, lon: 190, zoom: 3 })).toEqual({ lat: 85.05112878, lon: -170, zoom: 3 });
	});
	it('déplacer : la carte suit la souris', () => {
		const c = { ...paris, zoom: 10 };
		const d = panBy(c, 40, -25);
		const s = screen(d, paris.lat, paris.lon);
		expect(s.x).toBeCloseTo(W / 2 + 40, 6);
		expect(s.y).toBeCloseTo(H / 2 - 25, 6);
		expect(d.zoom).toBe(10);
	});
	it('molette : un cran fait un demi-niveau, borné', () => {
		expect(wheelZoom(-100)).toBe(0.5);
		expect(wheelZoom(100)).toBe(-0.5);
		expect(wheelZoom(3, 1)).toBeCloseTo(-0.495, 6);
		expect(wheelZoom(-5000)).toBe(1);
	});
	it('vue enregistrée : arrondie, relue, refusée si illisible', () => {
		expect(roundCam({ lat: 48.85661234, lon: 2.35221987, zoom: 9.123456 })).toEqual({ lat: 48.856612, lon: 2.35222, zoom: 9.123 });
		expect(parseCam({ lat: 48, lon: 2, zoom: 30 })).toEqual({ lat: 48, lon: 2, zoom: MAX_ZOOM });
		expect(parseCam(null)).toBeNull();
		expect(parseCam({ lat: '48', lon: 2, zoom: 3 })).toBeNull();
		expect(parseCam({ lat: NaN, lon: 2, zoom: 3 })).toBeNull();
	});
});

describe('tuiles d’une vue zoomée', () => {
	it('niveau le plus proche, tuiles agrandies ou réduites', () => {
		expect(tileZoom(6.4)).toBe(6);
		expect(tileZoom(6.6)).toBe(7);
		expect(tileZoom(25)).toBe(19);
		const v = { z: 1.5, x0: 0, y0: 0 };
		const t = viewTiles(v, 400, 300);
		expect(t[0].z).toBe(2);
		expect(t[0].size).toBeCloseTo(256 / Math.SQRT2, 9);
		// Les tuiles couvrent la vue, sans trou.
		const right = Math.max(...t.map((q) => q.left + q.size));
		const bottom = Math.max(...t.map((q) => q.top + q.size));
		expect(right).toBeGreaterThanOrEqual(400);
		expect(bottom).toBeGreaterThanOrEqual(300);
		expect(Math.min(...t.map((q) => q.left))).toBeLessThanOrEqual(0);
		expect(t.length).toBe(3 * 2); // tuiles de 181 px : 3 colonnes sur 400, 2 lignes sur 300
	});
	it('couverture d’une vue au zoom ajusté', () => {
		const pts = [paris, { lat: 43.2965, lon: 5.3698 }];
		const v = fitView(pts, W, H);
		const t = viewTiles(v, W, H);
		const cols = new Set(t.map((q) => q.u)).size;
		const rows = new Set(t.map((q) => q.y)).size;
		expect(cols * rows).toBe(t.length);
		expect(cols).toBeLessThanOrEqual(Math.ceil(W / 256) + 1);
		expect(rows).toBeLessThanOrEqual(Math.ceil(H / 256) + 1);
	});
	it('remplacement : parente agrandie, sinon filles réduites', () => {
		const v = { z: 3, x0: 0, y0: 0 };
		const t = viewTiles(v, 256, 256); // la tuile 3/0/0
		expect(t.map((q) => `${q.z}/${q.x}/${q.y}`)).toEqual(['3/0/0']);
		// Parente au niveau 1 chargée : agrandie 4 fois, à sa place.
		const up = fallbackTiles(t, v, (z, x, y) => z === 1 && x === 0 && y === 0);
		expect(up).toEqual([{ z: 1, x: 0, y: 0, u: 0, left: 0, top: 0, size: 1024 }]);
		// Rien au-dessus, deux filles au niveau 4 : réduites de moitié.
		const down = fallbackTiles(t, v, (z, x, y) => z === 4 && y === 1 && x <= 1);
		expect(down.map((q) => `${q.z}/${q.x}/${q.y}@${q.left},${q.top}x${q.size}`)).toEqual(['4/0/1@0,128x128', '4/1/1@128,128x128']);
		// Tuile déjà là : pas de remplacement.
		expect(fallbackTiles(t, v, () => true)).toEqual([]);
	});
	it('remplacement replié autour du globe', () => {
		const v = { z: 2, x0: -256, y0: 0 };
		const t = viewTiles(v, 256, 256); // colonne −1 : tuile 2/3/0
		expect(t.map((q) => `${q.x}@${q.u}`)).toEqual(['3@-1']);
		const up = fallbackTiles(t, v, (z, x) => z === 1 && x === 1);
		expect(up).toEqual([{ z: 1, x: 1, y: 0, u: -1, left: -256, top: 0, size: 512 }]);
	});
});

describe('densité zoomée', () => {
	it('rayon : base au zoom ajusté, proportionnel ensuite, borné', () => {
		expect(heatRadius(36, 8, 8, W, H)).toBe(36);
		expect(heatRadius(36, 9, 8, W, H)).toBe(72);
		expect(heatRadius(36, 12, 8, W, H)).toBe(H / 4);
		expect(heatRadius(36, 7, 8, W, H)).toBe(18);
		expect(heatRadius(36, 2, 8, W, H)).toBe(8);
	});
});
