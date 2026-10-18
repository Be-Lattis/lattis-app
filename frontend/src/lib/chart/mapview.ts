// Vue d'une carte (zoom et déplacement) : calculs purs, sans DOM.
//
// Une vue est un centre (latitude, longitude) et un zoom fractionnaire,
// celui des tuiles OpenStreetMap (0 : le monde tient dans 256 px, chaque
// cran double l'échelle). Les tuiles affichées sont celles du niveau entier
// le plus proche, agrandies ou réduites (voir viewTiles).

import { TILE, TILE_MAX_ZOOM, project, type MapView, type ViewTile } from './shapes';

/** Zoom le plus faible (le monde entier dans un petit graphique). */
export const MIN_ZOOM = 1;
/** Zoom le plus fort (la rue ; au-delà, les tuiles sont agrandies). */
export const MAX_ZOOM = 18;
/** Pas des boutons + / − et des touches. */
export const ZOOM_STEP = 1;

const LAT_MAX = 85.05112878;

export interface MapCam {
	lat: number;
	lon: number;
	zoom: number;
}

export function clampZoom(z: number): number {
	return Math.max(MIN_ZOOM, Math.min(MAX_ZOOM, z));
}

/** Inverse de project : latitude et longitude d'un point en pixels du monde au zoom z. */
export function unproject(x: number, y: number, z: number): { lat: number; lon: number } {
	const s = TILE * 2 ** z;
	const n = Math.PI - (2 * Math.PI * y) / s;
	return { lat: (180 / Math.PI) * Math.atan(Math.sinh(n)), lon: (x / s) * 360 - 180 };
}

/** Vue bornée : latitude dans la carte, longitude ramenée entre −180 et 180, zoom borné. */
export function clampCam(c: MapCam): MapCam {
	const lon = ((((c.lon + 180) % 360) + 360) % 360) - 180;
	return { lat: Math.max(-LAT_MAX, Math.min(LAT_MAX, c.lat)), lon, zoom: clampZoom(c.zoom) };
}

/** Vue à l'écran (coin haut gauche en pixels du monde) d'un centre et d'un zoom. */
export function camView(c: MapCam, w: number, h: number): MapView {
	const p = project(c.lat, c.lon, c.zoom);
	return { z: c.zoom, x0: p.x - w / 2, y0: p.y - h / 2 };
}

/** Centre et zoom d'une vue à l'écran. */
export function viewCam(v: MapView, w: number, h: number): MapCam {
	const { lat, lon } = unproject(v.x0 + w / 2, v.y0 + h / 2, v.z);
	return { lat, lon, zoom: v.z };
}

/** Zoom autour d'un point de l'écran (px, py) : ce point reste sous le curseur. */
export function zoomAt(c: MapCam, w: number, h: number, px: number, py: number, zoom: number): MapCam {
	const z = clampZoom(zoom);
	const v = camView(c, w, h);
	const k = 2 ** (z - c.zoom);
	return clampCam(viewCam({ z, x0: (v.x0 + px) * k - px, y0: (v.y0 + py) * k - py }, w, h));
}

/** Déplacement de la carte de (dx, dy) pixels d'écran (la carte suit la souris). */
export function panBy(c: MapCam, dx: number, dy: number): MapCam {
	const p = project(c.lat, c.lon, c.zoom);
	return clampCam({ ...unproject(p.x - dx, p.y - dy, c.zoom), zoom: c.zoom });
}

/** Cran de molette en écart de zoom : un cran de souris (100 px) fait un demi-niveau. */
export function wheelZoom(deltaY: number, deltaMode = 0): number {
	const px = deltaMode === 1 ? deltaY * 33 : deltaMode === 2 ? deltaY * 400 : deltaY;
	return Math.max(-1, Math.min(1, -px / 200));
}

/** Vue arrondie pour l'enregistrer (un dixième de mètre, un millième de niveau). */
export function roundCam(c: MapCam): MapCam {
	const r = (x: number, d: number) => Math.round(x * 10 ** d) / 10 ** d;
	return { lat: r(c.lat, 6), lon: r(c.lon, 6), zoom: r(c.zoom, 3) };
}

/** Vue enregistrée lisible (un objet avec trois nombres finis), sinon null. */
export function parseCam(v: unknown): MapCam | null {
	if (!v || typeof v !== 'object') return null;
	const o = v as Record<string, unknown>;
	if (![o.lat, o.lon, o.zoom].every((x) => typeof x === 'number' && isFinite(x))) return null;
	return clampCam({ lat: o.lat as number, lon: o.lon as number, zoom: o.zoom as number });
}

/**
 * Tuiles de remplacement, déjà chargées, pour celles qui manquent encore :
 * une tuile parente (jusqu'à 4 niveaux plus haut, agrandie) ou, à défaut,
 * les filles du niveau suivant (réduites). Ainsi la carte ne se vide pas
 * pendant qu'on zoome. Les plus grossières d'abord (dessinées dessous).
 */
export function fallbackTiles(tiles: ViewTile[], v: MapView, has: (z: number, x: number, y: number) => boolean): ViewTile[] {
	const out = new Map<string, ViewTile>();
	for (const t of tiles) {
		if (has(t.z, t.x, t.y)) continue;
		let found = false;
		for (let d = 1; d <= 4 && t.z - d >= 0; d++) {
			const z = t.z - d;
			const n = 2 ** z;
			const u = Math.floor(t.u / 2 ** d);
			const y = Math.floor(t.y / 2 ** d);
			const x = ((u % n) + n) % n;
			if (!has(z, x, y)) continue;
			const size = t.size * 2 ** d;
			out.set(`${z}/${u}/${y}`, { z, x, y, u, left: u * size - v.x0, top: y * size - v.y0, size });
			found = true;
			break;
		}
		if (found || t.z + 1 > TILE_MAX_ZOOM) continue;
		const z = t.z + 1;
		const n = 2 ** z;
		const size = t.size / 2;
		for (let dy = 0; dy < 2; dy++)
			for (let dx = 0; dx < 2; dx++) {
				const u = t.u * 2 + dx;
				const y = t.y * 2 + dy;
				const x = ((u % n) + n) % n;
				if (has(z, x, y)) out.set(`${z}/${u}/${y}`, { z, x, y, u, left: u * size - v.x0, top: y * size - v.y0, size });
			}
	}
	return [...out.values()].sort((a, b) => a.z - b.z);
}

/**
 * Rayon de la densité au zoom courant : base au zoom ajusté aux points, puis
 * proportionnel à l'échelle (la tache couvre la même étendue), borné pour
 * rester lisible (pas de points isolés minuscules, pas de nappe uniforme).
 */
export function heatRadius(base: number, zoom: number, fitZoom: number, w: number, h: number): number {
	return Math.max(8, Math.min(Math.max(base, Math.min(w, h) / 4), base * 2 ** (zoom - fitZoom)));
}
