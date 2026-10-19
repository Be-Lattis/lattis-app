import { describe, expect, it } from 'vitest';
import { autoHeight, BASE_PX, headerHeight, headerTextWidth, lineHeight, trimTrailingBreaks, wrapLines } from './textwrap';

// Largeur fictive : 10 px par caractère.
const measure = (s: string) => s.length * 10;

describe('en-tête de tableau renvoyé à la ligne', () => {
	it('laisse la place des boutons', () => {
		expect(headerTextWidth(120, 30)).toBe(74);
		expect(headerTextWidth(20, 30)).toBe(8);
	});

	it('sans renvoi à la ligne : hauteur par défaut', () => {
		expect(headerHeight('Indices occupation du sol', 64, undefined, measure)).toBe(0);
		expect(headerHeight('Indices occupation du sol', 64, {}, measure)).toBe(0);
	});

	it('grandit d’une ligne par ligne de texte', () => {
		const lh = lineHeight(BASE_PX);
		// 200 - 20 - 16 = 164 px : « Indices » / « occupation du » / « sol ».
		expect(headerHeight('Indices occupation du sol', headerTextWidth(200, 20), { wr: true }, measure)).toBe(3 * lh + 4);
		// Assez large : une seule ligne.
		expect(headerHeight('Nom', 64, { wr: true }, measure)).toBe(lh + 4);
	});
});

describe('sauts de ligne finaux', () => {
	it('sont retirés, avec les lignes vides qui les suivent', () => {
		expect(trimTrailingBreaks('Texte\n\n')).toBe('Texte');
		expect(trimTrailingBreaks('Texte\r\n\r\n')).toBe('Texte');
		expect(trimTrailingBreaks('Texte\n  \n\t')).toBe('Texte');
		expect(trimTrailingBreaks('\n\n')).toBe('');
		// Début et milieu gardés.
		expect(trimTrailingBreaks('\nA\n\nB\n')).toBe('\nA\n\nB');
		expect(trimTrailingBreaks('Texte  ')).toBe('Texte  ');
	});

	it('ne comptent pas dans la hauteur automatique', () => {
		const lh = lineHeight(BASE_PX);
		expect(autoHeight('Texte\n\n', 200, {}, measure, 22)).toBe(0);
		expect(autoHeight('Texte\n\n', 200, { wr: true }, measure, 22)).toBe(0);
		expect(autoHeight('a\nb\r\n\r\n', 200, {}, measure, 22)).toBe(2 * lh + 4);
		expect(autoHeight('a\n\nb\n', 200, { wr: true }, measure, 22)).toBe(3 * lh + 4);
		expect(autoHeight('\na\n', 200, {}, measure, 22)).toBe(2 * lh + 4);
	});

	it('ne sont pas dessinés', () => {
		expect(wrapLines('un deux\n\n', 1000, measure)).toEqual(['un deux']);
		expect(wrapLines('\nun\n \n', 1000, measure)).toEqual(['', 'un']);
	});

	it('ne grandissent pas un en-tête', () => {
		expect(headerHeight('Nom\n\n', 64, { wr: true }, measure)).toBe(lineHeight(BASE_PX) + 4);
	});
});
