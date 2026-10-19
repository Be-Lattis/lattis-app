import { describe, expect, it } from 'vitest';
import { bandAt, bandLabel, hiddenBand } from './hidden';

describe('lignes masquées par un filtre', () => {
	// En-tête en ligne 2, 8 lignes affichées (3 à 10), 92 masquées (11 à 102).
	const t = { name: 'Commandes', row: 2, col: 1, rows: 8, hidden: 92, columns: [1, 2, 3] };
	it('bande sous les lignes affichées', () => {
		expect(hiddenBand(t)).toEqual({ first: 11, last: 102 });
		expect(hiddenBand({ ...t, hidden: 0 })).toBeNull();
	});
	it('cellules de la bande', () => {
		expect(bandAt([t], 11, 1)).toBe(t);
		expect(bandAt([t], 102, 3)).toBe(t);
		expect(bandAt([t], 103, 2)).toBeUndefined(); // cellule libre, sous la dernière ligne réelle
		expect(bandAt([t], 50, 4)).toBeUndefined(); // à droite du tableau
		expect(bandAt([t], 10, 1)).toBeUndefined(); // ligne affichée
	});
	it('libellé', () => {
		expect(bandLabel(1)).toBe('⧩ 1 ligne masquée par le filtre');
		expect(bandLabel(92)).toBe('⧩ 92 lignes masquées par le filtre');
	});
});
