import { describe, expect, it } from 'vitest';
import type { PrintPlan } from '#bindings/internal/transfer/models.js';
import { borderCss, cellCss } from './cellcss';
import { pageLines, printChunks, printHtml, type PrintCell, type PrintSource } from './sheetprint';

/** Plan d'une page A4 portrait (marges normales) sur les bandes données. */
function plan(rows: number[][], cols: number[][], extra: Partial<PrintPlan> = {}): PrintPlan {
	const pages = [];
	for (let j = 0; j < cols.length; j++) for (let i = 0; i < rows.length; i++) pages.push({ row: i, col: j, header: {}, footer: { center: `Page ${pages.length + 1}` } });
	return {
		empty: false,
		paperW: 210,
		paperH: 297,
		margins: { top: 1.905, bottom: 1.905, left: 1.778, right: 1.778, header: 0.762, footer: 0.762 },
		scale: 1,
		rows,
		cols,
		pages,
		titleRows: null,
		titleCols: null,
		area: { r0: rows[0][0], c0: cols[0][0], r1: rows[rows.length - 1][1], c1: cols[cols.length - 1][1] },
		gridlines: false,
		headings: false,
		centerH: false,
		centerV: false,
		blackWhite: false,
		rowPx: 22,
		colPx: 100,
		heights: [],
		widths: [],
		headingsW: 36,
		headingsH: 18,
		...extra
	} as PrintPlan;
}

/** Source faite d'une table de cellules ; prepared : plages chargées. */
function source(cells: Record<string, PrintCell>, extra: Partial<PrintSource> = {}) {
	const prepared: number[][] = [];
	const src: PrintSource = {
		merges: [],
		charts: [],
		async prepare(r0, r1) {
			prepared.push([r0, r1]);
		},
		cell: (r, c) => cells[`${r},${c}`],
		...extra
	};
	return { src, prepared };
}

const pagesOf = (html: string) => html.split('<section class="lp-page').slice(1);

describe('impression d’une feuille', () => {
	it('écrit la mise en forme de la grille', async () => {
		const { src } = source({
			'0,0': { text: 'Produit', bold: true, fill: '#e3efe7', color: '#14532d', framed: true, align: 'left' },
			'1,0': { text: 'Pomme & co', style: { bg: '#ffff00', fg: '#c00000', i: true, u: true, x: true, fs: 14, ff: 'Georgia', al: 'center', va: 'top', wr: true, bt: true, bb: true, bc: '#0070c0' }, align: 'left' },
			'1,1': { text: '1 234,50 €', style: { nf: '€' }, align: 'right' }
		});
		const html = await printHtml(plan([[0, 1]], [[0, 1]]), src);
		expect(html).toContain('@page { size: 210mm 297mm; margin: 0; }');
		expect(html).toContain('>Pomme &amp; co</td>');
		expect(html).toContain('>1 234,50 €</td>');
		// Les déclarations de chaque cellule sont dans la feuille de style des pages.
		const css = html.slice(html.indexOf('td.s0'));
		for (const want of ['background:#ffff00', 'color:#c00000', 'font-style:italic', 'text-decoration:underline line-through', 'font-size:19px', 'font-family:"Georgia"', 'text-align:center', 'vertical-align:top', 'white-space:pre-wrap', 'border-top:1px solid #0070c0', 'border-bottom:1px solid #0070c0', 'background:#e3efe7;color:#14532d;font-weight:600', 'text-align:right'])
			expect(css).toContain(want);
		expect(css).not.toContain('border-left');
		expect(html).toMatch(/<td class="t s\d+">Produit<\/td>/);
		expect(pagesOf(html)).toHaveLength(1);
		expect(html).toContain('<span class="c">Page 1</span>');
	});

	it('découpe les pages selon le plan : titres répétés, en-têtes, échelle', async () => {
		const cells: Record<string, PrintCell> = {};
		for (let r = 0; r < 30; r++) for (let c = 0; c < 3; c++) cells[`${r},${c}`] = { text: `R${r}C${c}`, align: 'left' };
		const { src, prepared } = source(cells);
		const p = plan(
			[
				[0, 9],
				[10, 19],
				[20, 29]
			],
			[
				[0, 1],
				[2, 2]
			],
			{ titleRows: { from: 0, to: 0 }, titleCols: { from: 0, to: 0 }, headings: true, gridlines: true, scale: 0.5, heights: [[5, 0]], widths: [[1, 150]] }
		);
		const pages = pagesOf(await printHtml(p, src));
		expect(pages).toHaveLength(6);
		// Vers le bas, puis à droite : page 4 = lignes 1 à 10, colonne C avec A répétée.
		expect(pages[3]).toContain('>R1C0<');
		expect(pages[3]).toContain('>R1C2<');
		expect(pages[3]).not.toContain('>R1C1<');
		// Ligne de titre en tête de chaque page, ligne 6 masquée.
		for (const pg of pages.slice(1, 3)) expect(pg).toContain('>R0C0<');
		expect(pages[0]).not.toContain('>R5C0<');
		expect(pages[0]).toContain('<th>A</th><th>B</th>');
		expect(pages[0]).toContain('<th>1</th>');
		expect(pages[0]).toContain('zoom:0.5');
		expect(pages[0]).toContain('class="print-grid lp-lines"');
		expect(pages[0]).toContain('<col style="width:150px">');
		// Lignes chargées bande par bande, les titres à part.
		expect(prepared).toEqual([
			[0, 9],
			[0, 0],
			[10, 19],
			[0, 0],
			[20, 29],
			[0, 9],
			[0, 0],
			[10, 19],
			[0, 0],
			[20, 29]
		]);
	});

	it('fusions, débordement, mise en forme conditionnelle', async () => {
		const { src } = source(
			{
				'0,0': { text: 'Titre fusionné', align: 'left', style: { al: 'center' } },
				'1,0': { text: 'Un long texte qui déborde', align: 'left', overflow: true },
				'1,2': { text: 'x', align: 'left' },
				'2,0': { text: '12', align: 'right', bar: { a: 0, b: 0.5, color: '#638ec6', gradient: true, border: false } },
				'2,1': { text: '8', align: 'right', icon: { set: '3arrows', index: 2 } }
			},
			{ merges: [{ row: 0, col: 0, rows: 1, cols: 3 }] }
		);
		const html = await printHtml(plan([[0, 2]], [[0, 2]]), src, { icon: (set, i) => `data:image/png;base64,${set}${i}` });
		expect(html).toMatch(/<td colspan="3" class="s\d+">Titre fusionné<\/td><\/tr>/);
		// Le texte déborde sur B (vide), pas sur C (occupée).
		expect(html).toContain('<div class="lp-ov" style="width:188px">Un long texte qui déborde</div>');
		expect(html).toContain('linear-gradient(to right, #638ec6, #fff)');
		expect(html).toContain('width:calc(0.5 * (100% - 4px))');
		expect(html).toContain('<img class="lp-ic" src="data:image/png;base64,3arrows2" alt="">');
	});

	it('pose les graphiques sur leur page, coupés à la bande', async () => {
		const chart = { row: 1, col: 1, x: 10, y: 5, w: 300, h: 200, html: '<svg width="300" height="200"><rect/></svg>' };
		const { src } = source({ '0,0': { text: 'a' } }, { charts: [chart] });
		const pages = pagesOf(
			await printHtml(
				plan(
					[
						[0, 9],
						[10, 19]
					],
					[[0, 3]]
				),
				src
			)
		);
		expect(pages[0]).toContain('<div class="lp-chart" style="left:110px;top:27px;width:300px;height:200px"><svg width="300" height="200"><rect/></svg></div>');
		// Le graphique (22 + 5 à 227 px) dépasse sur la page 2 (à partir de 220 px).
		expect(pages[1]).toContain('class="lp-chart" style="left:110px;top:-193px');
	});

	it('imprime les images, formes et zones de texte sans fond', async () => {
		const shape = { row: 0, col: 0, x: 4, y: 2, w: 120, h: 0, html: '<div class="obj line"></div>', plain: true };
		const { src } = source({ '0,0': { text: 'a' } }, { charts: [shape] });
		const html = await printHtml(plan([[0, 9]], [[0, 3]]), src);
		expect(html).toContain('<div class="lp-chart lp-plain" style="left:4px;top:2px;width:120px;height:0px"><div class="obj line"></div></div>');
	});

	it('imprime une feuille vide', async () => {
		const { src } = source({});
		const html = await printHtml({ ...plan([[0, 0]], [[0, 0]]), empty: true, pages: [] }, src, { empty: 'Feuille vide.' });
		expect(pagesOf(html)).toHaveLength(1);
		expect(html).toContain('Feuille vide.');
	});

	it('imprime 100 000 lignes par lots, avec l’avancement', async () => {
		const rows: number[][] = [];
		for (let r = 0; r < 100_000; r += 44) rows.push([r, Math.min(r + 43, 99_999)]);
		const { src } = source({}, { cell: (r, c) => ({ text: c === 0 ? String(r + 1) : 'ligne', align: c === 0 ? 'right' : 'left', framed: true }) });
		const steps: number[] = [];
		let pauses = 0;
		let last = '';
		let n = 0;
		const t0 = performance.now();
		for await (const chunk of printChunks(plan(rows, [[0, 3]]), src, { batch: 50, progress: (d) => steps.push(d), pause: async () => void pauses++ })) {
			n += pagesOf(chunk).length;
			last = chunk;
		}
		expect(n).toBe(rows.length);
		expect(steps[steps.length - 1]).toBe(rows.length);
		expect(pauses).toBe(Math.ceil(rows.length / 50) - 1);
		expect(last.startsWith('<style>')).toBe(true);
		expect(performance.now() - t0).toBeLessThan(20_000);
	});
});

describe('mise en forme en CSS', () => {
	it('lit les bordures plus riches', () => {
		expect(borderCss(true, '#ff0000')).toBe('1px solid #ff0000');
		expect(borderCss('medium dashed #00ff00')).toBe('2px dashed #00ff00');
		expect(borderCss('thick')).toBe('3px solid #000');
		expect(borderCss('2px dotted', '#123456')).toBe('2px dotted #123456');
		expect(borderCss({ style: 'double', color: '#abcdef' })).toBe('1px double #abcdef');
		expect(borderCss('none')).toBeUndefined();
		expect(borderCss(false)).toBeUndefined();
		expect(cellCss({ border_left: 'thin dashed' } as never)).toBe('border-left:1px dashed #000');
	});

	it('police : ff ou font', () => {
		expect(cellCss({ font: 'Arial' } as never)).toContain('font-family:"Arial"');
		expect(cellCss(undefined, { align: 'left' })).toBe('');
	});

	it('lignes d’une page', () => {
		expect(pageLines([10, 14], { from: 0, to: 1 }, (i) => (i === 12 ? 0 : 22))).toEqual([0, 1, 10, 11, 13, 14]);
		expect(pageLines([0, 2], { from: 0, to: 1 }, () => 22)).toEqual([0, 1, 2]);
	});
});
