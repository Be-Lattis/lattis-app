import { describe, expect, it } from 'vitest';
import { count, toTSV } from './result';

const plain = (s: string) => s.replace(/[  ]/g, ' ');

describe('nombre de lignes accordé', () => {
	it('singulier jusqu’à 1, pluriel à partir de 2', () => {
		expect(count(0)).toBe('0 ligne');
		expect(count(1)).toBe('1 ligne');
		expect(count(2)).toBe('2 lignes');
		expect(plain(count(1234))).toBe('1 234 lignes');
		expect(count(1, 'tableau', 'tableaux')).toBe('1 tableau');
		expect(count(3, 'tableau', 'tableaux')).toBe('3 tableaux');
	});
});

describe('copie pour Excel', () => {
	it('tabulations, virgule décimale, guillemets si besoin', () => {
		expect(toTSV(['Nom', 'Prix'], [['Pomme', 1.5], ['Poire "williams"', 2], null, ['a\tb', null]])).toBe(
			'Nom\tPrix\r\nPomme\t1,5\r\n"Poire ""williams"""\t2\r\n\r\n"a\tb"\t\r\n'
		);
	});
});
