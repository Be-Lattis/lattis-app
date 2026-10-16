import { describe, expect, it } from 'vitest';
import { panes, reveal, toGrid, toScreen } from './panes';

describe('volets figés', () => {
	it('un seul volet sans rien de figé', () => {
		expect(panes(0, 0, 52, 22, 800, 600, 30, 40)).toEqual([{ left: 30, top: 40, x0: 52, x1: 800, y0: 22, y1: 600 }]);
	});

	it('quatre volets quand lignes et colonnes sont figées', () => {
		const p = panes(100, 44, 52, 22, 800, 600, 30, 40);
		expect(p).toHaveLength(4);
		expect(p[0]).toEqual({ left: 0, top: 0, x0: 52, x1: 152, y0: 22, y1: 66 });
		expect(p[3]).toEqual({ left: 30, top: 40, x0: 152, x1: 800, y0: 66, y1: 600 });
		// Lignes seulement : deux volets.
		expect(panes(0, 44, 52, 22, 800, 600, 0, 0)).toHaveLength(2);
	});

	it('positions et clics', () => {
		// Colonne figée : pas de défilement.
		expect(toScreen(88, true, 500, 52)).toBe(140);
		expect(toScreen(880, false, 500, 52)).toBe(432);
		// Clic dans la partie figée, puis dans la partie qui défile.
		expect(toGrid(100, 52, 88, 500)).toBe(48);
		expect(toGrid(200, 52, 88, 500)).toBe(648);
	});

	it('faire apparaître une case sous la partie figée', () => {
		// Case figée : rien ne bouge.
		expect(reveal(0, 22, 300, 400, 44)).toBe(300);
		// Case cachée sous les lignes figées : on remonte.
		expect(reveal(310, 332, 300, 400, 44)).toBe(266);
		// Case sous le bas de la vue : on descend.
		expect(reveal(800, 822, 300, 400, 44)).toBe(422);
		expect(reveal(400, 422, 300, 400, 44)).toBe(300);
	});
});
