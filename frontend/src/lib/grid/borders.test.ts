import { describe, expect, it } from 'vitest';
import { borderParts, BorderSet, edgeCss, edgeValue, edgeWeight, parseEdge } from './borders';
import { cssOfStyle, numberFormatOf, parseDecls, styleOfDecls } from './clipboardHtml';
import { cellCss } from '#lib/print/cellcss.js';

describe('bordures', () => {
	it('lit les bords d’avant (true : trait fin) et les styles', () => {
		expect(parseEdge(true)).toEqual({ style: 'thin', color: undefined });
		expect(parseEdge(true, '#c00000')).toEqual({ style: 'thin', color: '#c00000' });
		expect(parseEdge('double')).toEqual({ style: 'double', color: undefined });
		expect(parseEdge('thick theme:accent1:-25', '#000000')).toEqual({ style: 'thick', color: 'theme:accent1:-25' });
		expect(parseEdge('inconnu')).toEqual({ style: 'thin', color: undefined });
		expect(parseEdge(null)).toBeNull();
		expect(parseEdge(false)).toBeNull();
	});

	it('écrit true pour un trait fin automatique', () => {
		expect(edgeValue({ style: 'thin' })).toBe(true);
		expect(edgeValue({ style: 'dashed' })).toBe('dashed');
		expect(edgeValue({ style: 'thin', color: '#ff0000' })).toBe('thin #ff0000');
		expect(edgeValue(null)).toBeNull();
		for (const v of [true, 'double', 'medium #123456']) expect(edgeValue(parseEdge(v))).toBe(v);
	});

	it('aperçu CSS', () => {
		expect(edgeCss({ style: 'double' }, '#000')).toBe('3px double #000');
		expect(edgeCss({ style: 'dotted' }, 'red')).toBe('1px dotted red');
		expect(edgeCss(null, 'red')).toBe('none');
	});

	it('commandes du ruban', () => {
		const pen = { style: 'medium' as const, color: '#ff0000' };
		expect(borderParts('bottom', pen, 1, 2, 3, 4)).toEqual([{ r0: 3, c0: 2, r1: 3, c1: 4, patch: { bb: 'medium #ff0000' } }]);
		expect(borderParts('all', { style: 'thin' }, 0, 0, 1, 1)[0].patch).toEqual({ bt: true, bb: true, bl: true, br: true });
		expect(borderParts('none', pen, 0, 0, 1, 1)[0].patch).toEqual({ bt: null, bb: null, bl: null, br: null });
		const box = borderParts('thickOutline', { style: 'thin' }, 0, 0, 2, 2);
		expect(box).toHaveLength(4);
		expect(box.map((p) => Object.values(p.patch)[0])).toEqual(['thick', 'thick', 'thick', 'thick']);
		expect(borderParts('inside', pen, 0, 0, 0, 0)).toEqual([]);
		expect(borderParts('inside', pen, 0, 0, 1, 1).map((p) => Object.keys(p.patch)[0])).toEqual(['bb', 'br']);
	});

	it('impression et collage : même lecture des bords', () => {
		expect(cellCss({ bb: 'double #c00000', bt: true })).toContain('border-bottom:3px double #c00000');
		expect(cellCss({ bt: true })).toContain('border-top:1px solid #000');
		expect(cellCss({ bl: 'thin dashed' } as never)).toContain('border-left:1px dashed #000');
		expect(styleOfDecls(parseDecls('border-bottom:2.0pt double #ff0000;border-top:.5pt dashed windowtext;border-left:1.5pt solid #000000;border-right:.5pt solid #000000'))).toMatchObject({ bb: 'double #ff0000', bt: 'dashed', bl: 'medium', br: true });
		expect(cssOfStyle({ bb: 'thick #00ff00' })).toContain('border-bottom:1.5pt solid #00ff00');
		expect(numberFormatOf('0.00E+00')).toEqual({ nc: '0.00E+00' });
		expect(numberFormatOf('#,##0.00;[Red]-#,##0.00')).toEqual({ nc: '#,##0.00;[Red]-#,##0.00' });
		expect(numberFormatOf('# ?/?')).toEqual({ nc: '# ?/?' });
	});

	it('un bord partagé n’est tracé qu’une fois, le plus visible', () => {
		const calls: string[] = [];
		const ctx = {
			set strokeStyle(v: string) {
				calls.push('color ' + v);
			},
			set lineWidth(v: number) {
				calls.push('width ' + v);
			},
			setLineDash: (d: number[]) => calls.push('dash ' + d.join(',')),
			beginPath: () => {},
			moveTo: (x: number, y: number) => calls.push(`move ${x},${y}`),
			lineTo: () => {},
			stroke: () => {}
		} as unknown as CanvasRenderingContext2D;
		const set = new BorderSet();
		set.add({ bb: true }, 0, 0, 100, 20); // A1 : bas fin
		set.add({ bt: 'thick' }, 0, 20, 100, 20); // A2 : haut épais, même trait
		set.add({ br: 'double #00ff00' }, 0, 40, 100, 20);
		expect(set.empty).toBe(false);
		set.stroke(ctx, (c) => c ?? 'auto', 'white');
		expect(calls.filter((c) => c.startsWith('move')).length).toBe(4); // épais : 1, double : 3 (intervalle et deux traits)
		expect(calls).toContain('width 3');
		expect(calls).toContain('color #00ff00');
		expect(calls).toContain('color white');
		expect(calls).not.toContain('width 1\ncolor auto');
		expect(set.empty).toBe(true);
		expect(edgeWeight({ style: 'thick' })).toBeGreaterThan(edgeWeight({ style: 'thin' }));
	});
});
