import { describe, expect, it } from 'vitest';
import { anchorOf, arrowHead, copyOf, downscaled, fitImage, lineBox, lineEnds, moveBox, objectBox, resizeBox, type Geometry } from './objects';

// Colonnes de 100 px, lignes de 20 px.
const geo: Geometry = {
	colOffset: (c) => c * 100,
	rowTop: (r) => r * 20,
	colAt: (x) => Math.floor(x / 100),
	rowAt: (y) => Math.floor(y / 20)
};

describe('objets posés : ancrage', () => {
	it('la boîte part de la cellule d’ancrage et de son décalage', () => {
		expect(objectBox({ id: 1, row: 2, col: 1, config: { kind: 'rect', offX: 5, offY: 3, width: 120, height: 80 } }, geo)).toEqual({ x: 105, y: 43, w: 120, h: 80 });
	});

	it('une boîte s’ancre à la cellule sous son coin haut gauche', () => {
		expect(anchorOf({ x: 250, y: 47, w: 10, h: 10 }, geo)).toEqual({ row: 2, col: 2, offX: 50, offY: 7 });
		// Hors de la feuille : ramenée au coin A1.
		expect(anchorOf({ x: -30, y: -5, w: 10, h: 10 }, geo)).toEqual({ row: 0, col: 0, offX: 0, offY: 0 });
	});

	it('déplacer ne sort pas de la feuille', () => {
		expect(moveBox({ x: 10, y: 10, w: 50, h: 50 }, -40, 5.6)).toEqual({ x: 0, y: 16, w: 50, h: 50 });
	});
});

describe('objets posés : redimensionner', () => {
	const b0 = { x: 100, y: 100, w: 200, h: 100 };

	it('un bord étire, le bord opposé reste en place', () => {
		expect(resizeBox(b0, 'e', 50, 999)).toEqual({ x: 100, y: 100, w: 250, h: 100 });
		expect(resizeBox(b0, 'n', 0, -30)).toEqual({ x: 100, y: 70, w: 200, h: 130 });
		expect(resizeBox(b0, 'w', 400, 0)).toEqual({ x: 292, y: 100, w: 8, h: 100 }); // côté minimal
	});

	it('un coin garde les proportions si demandé (image)', () => {
		expect(resizeBox(b0, 'se', 100, 0, true)).toEqual({ x: 100, y: 100, w: 300, h: 150 });
		expect(resizeBox(b0, 'nw', -200, 0, true)).toEqual({ x: 0, y: 50, w: 300, h: 150 }); // arrêtée au bord de la feuille
		expect(resizeBox(b0, 'nw', -40, -10, true)).toEqual({ x: 60, y: 80, w: 240, h: 120 });
		// Sans proportions : chaque côté suit la souris.
		expect(resizeBox(b0, 'se', 100, 0)).toEqual({ x: 100, y: 100, w: 300, h: 100 });
		// Un bord étire même avec proportions demandées.
		expect(resizeBox(b0, 'e', 100, 0, true)).toEqual({ x: 100, y: 100, w: 300, h: 100 });
	});
});

describe('lignes et flèches', () => {
	it('le sens donne les extrémités', () => {
		expect(lineEnds({ width: 100, height: 40 })).toEqual({ x1: 0, y1: 0, x2: 100, y2: 40 });
		expect(lineEnds({ width: 100, height: 40, flipH: true })).toEqual({ x1: 100, y1: 0, x2: 0, y2: 40 });
	});

	it('une extrémité tirée de l’autre côté retourne la ligne', () => {
		expect(lineBox({ x: 300, y: 50 }, { x: 100, y: 90 })).toEqual({ x: 100, y: 50, w: 200, h: 40, flipH: true, flipV: false });
		expect(lineBox({ x: 10, y: 10 }, { x: 10, y: 60 })).toEqual({ x: 10, y: 10, w: 0, h: 50, flipH: false, flipV: false });
	});

	it('la pointe est au second point', () => {
		expect(arrowHead(0, 0, 100, 0, 2)).toBe('100,0 92,4 92,-4');
		expect(arrowHead(0, 0, 0, 0, 2)).toBe('');
	});
});

describe('images', () => {
	it('posée à 480 pixels de large au plus, proportions gardées', () => {
		expect(fitImage(1200, 600)).toEqual({ width: 480, height: 240 });
		expect(fitImage(64, 32)).toEqual({ width: 64, height: 32 });
	});

	it('une très grande image est réduite à l’insertion', () => {
		expect(downscaled(1000, 800)).toBeNull();
		expect(downscaled(6000, 3000)).toEqual({ width: 2000, height: 1000 });
		expect(downscaled(1000, 4000, 2000)).toEqual({ width: 500, height: 2000 });
	});

	it('une copie (Ctrl+D) est décalée, sans nom ni rang', () => {
		const c = copyOf({ id: 3, row: 1, col: 2, config: { kind: 'text', text: 'Note', name: 'N', z: 4, width: 100, height: 50, offX: 2 } });
		expect(c).toEqual({ row: 1, col: 2, config: { kind: 'text', text: 'Note', width: 100, height: 50, offX: 14, offY: 12 } });
	});
});
