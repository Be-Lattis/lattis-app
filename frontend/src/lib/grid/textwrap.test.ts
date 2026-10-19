import { describe, expect, it } from 'vitest';
import { BASE_PX, headerHeight, headerTextWidth, lineHeight } from './textwrap';

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
