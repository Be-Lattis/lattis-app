import { describe, expect, it } from 'vitest';
import { parseTSV, toTSV } from './clipboard';

describe('presse-papiers', () => {
	it('lit le texte copié depuis Excel', () => {
		expect(parseTSV('Ville\tHabitants\r\nParis\t2 100 000\r\nLyon\t\r\n')).toEqual([
			['Ville', 'Habitants'],
			['Paris', '2 100 000'],
			['Lyon', '']
		]);
	});

	it('gère les champs entre guillemets (tabulation, saut de ligne, guillemet)', () => {
		expect(parseTSV('"a\tb"\t"ligne 1\nligne 2"\t"dit ""oui"""\n')).toEqual([['a\tb', 'ligne 1\nligne 2', 'dit "oui"']]);
	});

	it('accepte un texte sans saut de ligne final', () => {
		expect(parseTSV('x\ty')).toEqual([['x', 'y']]);
	});

	it('fait l’aller-retour', () => {
		const rows = [['a', 'b\tc'], ['"q"', '']];
		expect(parseTSV(toTSV(rows))).toEqual(rows);
	});
});
