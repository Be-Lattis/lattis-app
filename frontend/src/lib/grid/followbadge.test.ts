import { describe, expect, it } from 'vitest';
import { placeFollowBadge } from './followbadge';

// Cadre de 300 px à x = 100 ; pastille de 40 × 13 ; cellules sous un bandeau de 22 px.
const base = { x: 100, w: 300, bw: 40, bh: 13, paneTop: 22, strip: true };

describe('pastille « suit l’année »', () => {
	it('ligne du dessus libre et visible : au-dessus du cadre, à droite', () => {
		expect(placeFollowBadge({ ...base, y: 66, aboveFree: true })).toEqual({ x: 360, y: 53, place: 'above', radii: [4, 4, 0, 0] });
	});

	it('tableau en ligne 1 : dans le bandeau du haut, vide au-dessus', () => {
		const b = placeFollowBadge({ ...base, y: 22, aboveFree: true });
		expect(b.place).toBe('above');
		expect(b.y).toBe(9);
	});

	it('en-tête collé en haut après défilement, ligne du dessus occupée mais hors écran : bandeau', () => {
		expect(placeFollowBadge({ ...base, y: 22, aboveFree: false }).place).toBe('above');
	});

	it('ligne du dessus occupée (titre) : dans le coin du cadre, à cheval sur son trait', () => {
		const b = placeFollowBadge({ ...base, y: 66, aboveFree: false });
		expect(b.place).toBe('corner');
		expect(b.x + 40).toBeLessThanOrEqual(400);
		expect(b.y).toBeLessThan(66);
		expect(b.y + 13).toBeGreaterThan(66);
	});

	it('volet du bas (lignes figées au-dessus) : pas de bandeau, coin du cadre', () => {
		expect(placeFollowBadge({ ...base, y: 100, paneTop: 100, strip: false, aboveFree: true }).place).toBe('corner');
	});
});
