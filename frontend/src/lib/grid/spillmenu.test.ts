import { describe, expect, it } from 'vitest';
import { spillAnchor, spillAt, spillTip } from './spillmenu';

describe('spillmenu', () => {
	const spills = [
		{ row: 1, col: 7, rows: [['N1'], ['N2'], ['N3']] },
		{ row: 10, col: 0, rows: [['a', '1'], ['b', '2']] }
	];

	it('trouve le déversement qui couvre une cellule', () => {
		expect(spillAt(spills, 1, 7)).toBe(spills[0]);
		expect(spillAt(spills, 3, 7)).toBe(spills[0]);
		expect(spillAt(spills, 4, 7)).toBeUndefined();
		expect(spillAt(spills, 0, 7)).toBeUndefined();
		expect(spillAt(spills, 11, 1)).toBe(spills[1]);
		expect(spillAt(spills, 11, 2)).toBeUndefined();
	});

	it('nomme l’ancre en A1 dans l’infobulle', () => {
		expect(spillAnchor(spills[0])).toBe('H2');
		expect(spillTip(spills[0])).toBe('Résultat déversé de la formule en H2 — clic droit : convertir en tableau');
		expect(spillAnchor({ row: 0, col: 26, rows: [['x']] })).toBe('AA1');
	});
});
