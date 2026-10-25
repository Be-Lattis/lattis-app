import { describe, expect, it } from 'vitest';
import { clipValues, cssColor, hasClipStyles, numberFormatOf, parseClipHtml, parseDecls, toClipHtml } from './clipboardHtml';
import { parseTSV } from './clipboard';
import classes from './testdata/clip-tableur-classes.html?raw';
import inline from './testdata/clip-tableur-en-ligne.html?raw';

describe('presse-papiers HTML', () => {
	it('lit un tableau à classes CSS (fusion, couleurs, bordures, formats)', () => {
		const c = parseClipHtml(classes)!;
		expect(c.rows).toEqual([
			['Ventes 2026', '', '', ''],
			['Ville', 'Montant', 'Part', 'Date'],
			['Paris & banlieue', '1 234,50 €', '45%', '21/10/2026'],
			['Lyon\n(Rhône)', '8,00 €', '5%', '22/10/2026']
		]);
		// Titre fusionné sur trois colonnes, en gras 14 pt centré (police par défaut non reprise).
		expect(c.styles[0][0]).toEqual({ b: true, fs: 14, al: 'center', mr: 1, mc: 3 });
		expect(c.styles[0][3]).toBeNull();
		expect(c.styles[1][1]).toEqual({ b: true, fg: '#ffffff', bg: '#4472c4', bt: true, bb: true, bl: true, br: true });
		expect(c.styles[2][1]).toEqual({ al: 'right', bt: true, bb: true, bl: true, br: true, nf: '€' });
		expect(c.styles[2][2]).toEqual({ i: true, fg: '#ff0000', al: 'right', bb: true, bl: true, br: true, nf: '%', dp: 0 });
		expect(c.styles[2][3]).toEqual({ u: true, ff: 'Courier New', al: 'right', va: 'top', wr: true, nf: 'date' });
		// Cellule sans classe : rien (le style d'office du tableur n'est pas une mise en forme).
		expect(c.styles[2][0]).toBeNull();
		expect(hasClipStyles(c)).toBe(true);
	});

	it('lit un tableau à styles en ligne (format de nombre en attribut)', () => {
		const c = parseClipHtml(inline)!;
		expect(c.rows).toEqual([
			['Produit', 'Prix'],
			['Pomme', '1,50'],
			['Poire williams', '25%']
		]);
		expect(c.styles[0][0]).toEqual({ b: true, bg: '#ffff00', bb: true });
		expect(c.styles[0][1]).toEqual({ b: true, bg: '#ffff00', al: 'right', bb: true });
		expect(c.styles[1][0]).toEqual({ fg: '#ff0000', x: true });
		expect(c.styles[1][1]).toEqual({ al: 'right', nf: '0,00', ng: true });
		// Police différente de celle du tableau (Arial 10 pt), renvoi à la ligne.
		expect(c.styles[2][0]).toEqual({ fs: 12, ff: 'Georgia', wr: true });
		expect(c.styles[2][1]).toEqual({ fg: '#008000', al: 'right', nf: '%', dp: 0 });
	});

	it('garde les valeurs du texte tabulé quand il a la même forme', () => {
		const c = parseClipHtml(inline)!;
		const tsv = parseTSV('Produit\tPrix\r\nPomme\t1,5\r\nPoire williams\t25%\r\n');
		expect(clipValues(c, tsv)[1]).toEqual(['Pomme', '1,5']);
		expect(clipValues(c, [['autre']])).toBe(c.rows);
	});

	it('refuse ce qui n’est pas un tableau', () => {
		expect(parseClipHtml('<p>Bonjour</p>')).toBeNull();
		expect(parseClipHtml('')).toBeNull();
	});

	it('lit les codes de format d’un tableur', () => {
		expect(numberFormatOf('0.00')).toEqual({ nf: '0,00', ng: true });
		expect(numberFormatOf('#,##0')).toEqual({ nf: '0' });
		expect(numberFormatOf('#,##0.000')).toEqual({ nf: '0,00', dp: 3 });
		expect(numberFormatOf('0.0%')).toEqual({ nf: '%', dp: 1 });
		expect(numberFormatOf('[$€-x-euro2] #,##0.00')).toEqual({ nf: '€' });
		expect(numberFormatOf('dd/mm/yyyy')).toEqual({ nf: 'date', df: 'jj/mm/aaaa' });
		expect(numberFormatOf('yyyy-mm-dd')).toEqual({ nf: 'date', df: 'aaaa-mm-jj' });
		expect(numberFormatOf('h:mm')).toEqual({ nf: 'time', df: 'hh:mm' });
		expect(numberFormatOf('@')).toEqual({ nf: 'text' });
		expect(numberFormatOf('General')).toEqual({});
		// Inconnu de Lattis : gardé tel quel.
		expect(numberFormatOf('$#,##0.00')).toEqual({ format: '$#,##0.00' });
	});

	it('lit couleurs et déclarations CSS', () => {
		expect(cssColor('#ABC')).toBe('#aabbcc');
		expect(cssColor('rgb(255, 0, 16)')).toBe('#ff0010');
		expect(cssColor('rgba(0,0,0,0)')).toBeNull();
		expect(cssColor('yellow')).toBe('#ffff00');
		expect(cssColor('auto')).toBeNull();
		expect(parseDecls(`mso-number-format:"0\\;0";color:red`)).toEqual({ 'mso-number-format': '"0\\;0"', color: 'red' });
	});

	it('écrit une sélection en HTML et la relit à l’identique', () => {
		const values = [
			['Titre', ''],
			['a < b', '0,5'],
			['ligne 1\nligne 2', '12']
		];
		const styles = [
			[{ b: true, mr: 1, mc: 2, bg: '#ffff00' }, null],
			[{ i: true, fg: '#ff0000', bt: true, bb: true, bl: true, br: true }, { nf: '%' as const }],
			[{ wr: true, va: 'top' as const }, { nf: '0,00' as const, ng: true, al: 'right' as const }]
		];
		const html = toClipHtml(values, (r, c) => styles[r][c]);
		expect(html).toContain('colspan="2"');
		expect(html).toContain('a &lt; b');
		const back = parseClipHtml(html)!;
		expect(back.rows).toEqual(values);
		expect(back.styles[0][0]).toEqual({ b: true, bg: '#ffff00', mr: 1, mc: 2 });
		expect(back.styles[1][0]).toEqual({ i: true, fg: '#ff0000', bt: true, bb: true, bl: true, br: true });
		expect(back.styles[1][1]).toEqual({ nf: '%', dp: 0 });
		expect(back.styles[2][0]).toEqual({ wr: true, va: 'top' });
		expect(back.styles[2][1]).toEqual({ al: 'right', nf: '0,00', ng: true });
	});
});
