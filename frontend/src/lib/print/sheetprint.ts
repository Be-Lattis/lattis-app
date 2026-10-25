// Impression et export PDF d'une feuille (Fichier › Imprimer / Exporter en
// PDF). Le classeur découpe la feuille en pages (DocumentService.PrintPlan :
// papier, marges, échelle, bandes de lignes et de colonnes, titres, en-tête
// et pied de page) ; ce module écrit chaque page en HTML avec ce que la
// grille affiche (PrintSource, tiré de Sheet.svelte : texte formaté,
// mise en forme, mise en forme conditionnelle, fusions, tableaux croisés,
// graphiques), page après page, sans figer l'interface : 100 000 lignes et
// plus s'impriment.
//
// Chaque page est un bloc de la taille du papier (@page margin: 0) ; la
// feuille y est posée en pixels de la grille, réduite par zoom à l'échelle
// retenue.
import type { PrintPlan } from '#bindings/internal/transfer/models.js';
import { Axis } from '#lib/grid/axis.js';
import { colLetters } from '#lib/grid/cfmodel.js';
import type { IconSetId } from '#lib/grid/cficons.js';
import type { BarOut } from '#lib/grid/rules.js';
import type { MergeRange } from '#lib/grid/sheetfmt.js';
import type { CellStyle } from '#lib/grid/style.js';
import { BASE_PX, GRID_FAMILY, lineHeight } from '#lib/grid/textwrap.js';
import { cellCss } from './cellcss';

/** Une cellule telle que la grille l'affiche. */
export interface PrintCell {
	/** Texte affiché (format de nombre, de date… appliqué). */
	text: string;
	/** Style effectif, mise en forme conditionnelle comprise. */
	style?: CellStyle;
	align?: 'left' | 'center' | 'right';
	/** Couleur du texte sans couleur choisie (erreur, en-tête). */
	color?: string;
	/** Fond sans remplissage choisi (en-tête de tableau, tableau croisé). */
	fill?: string;
	bold?: boolean;
	/** Mise en forme conditionnelle : barre de données, icône. */
	bar?: BarOut;
	icon?: { set: IconSetId; index: number };
	/** Retrait du texte (pixels : niveaux d'un tableau croisé). */
	indent?: number;
	/** Texte qui déborde sur les cellules vides à sa droite (cellule libre). */
	overflow?: boolean;
	/** Cellule d'un tableau ou d'un tableau croisé : encadrée sans quadrillage. */
	framed?: boolean;
	/** Graphique sparkline de la cellule (SVG à sa taille). */
	spark?: string;
}

/** Un graphique posé sur la feuille. */
export interface PrintChart {
	row: number;
	col: number;
	/** Décalage dans la cellule d'ancrage et taille (pixels de la feuille). */
	x: number;
	y: number;
	w: number;
	h: number;
	/** Son dessin (SVG de la grille). */
	html: string;
	/** Image, forme ou zone de texte (SheetObjects) : sans fond blanc, le trait d'une ligne peut déborder de sa boîte. */
	plain?: boolean;
}

/** Ce que la grille donne à imprimer. */
export interface PrintSource {
	/** Charge les lignes r0..r1 (colonnes c0..c1) avant cell(). */
	prepare(r0: number, r1: number, c0: number, c1: number): Promise<void>;
	cell(r: number, c: number): PrintCell | undefined;
	/** Fusions (cellules libres, groupes des en-têtes de tableaux). */
	merges: readonly MergeRange[];
	charts: readonly PrintChart[];
}

export interface PrintOptions {
	/** Pages écrites entre deux pauses (l'interface reste fluide). */
	batch?: number;
	/** Avancement : pages écrites sur le total. */
	progress?: (done: number, total: number) => void;
	/** Image (URL data:) d'une icône de mise en forme conditionnelle. */
	icon?: (set: IconSetId, index: number) => string;
	/** Pause entre deux lots (par défaut : une tâche du navigateur). */
	pause?: () => Promise<void>;
	/** Texte d'une feuille vide. */
	empty?: string;
}

const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

function mm(v: number): string {
	return `${Math.round(v * 100) / 100}mm`;
}

const nextTask = () => new Promise<void>((r) => setTimeout(r, 0));

/** Feuille de style des pages (dans la zone imprimée, #lattis-print : voir shell/print.ts). */
export function pageCss(plan: PrintPlan): string {
	return (
		`@page { size: ${mm(plan.paperW)} ${mm(plan.paperH)}; margin: 0; }` +
		`@media print { html, body { margin: 0 !important; padding: 0 !important; } }` +
		`#lattis-print .lp-page { position: relative; overflow: hidden; break-after: page; box-sizing: border-box; font: ${BASE_PX}px ${GRID_FAMILY}; color: #000; }` +
		`#lattis-print .lp-page:last-child { break-after: auto; }` +
		`#lattis-print .lp-bw { filter: grayscale(1); }` +
		`#lattis-print .lp-body { position: absolute; display: flex; flex-direction: column; align-items: flex-start; justify-content: flex-start; overflow: hidden; }` +
		`#lattis-print .lp-hf { position: absolute; display: grid; grid-template-columns: 1fr auto 1fr; gap: 6pt; font: 9pt ${GRID_FAMILY}; white-space: pre; }` +
		`#lattis-print .lp-hf .c { text-align: center; } #lattis-print .lp-hf .r { text-align: right; }` +
		`#lattis-print .lp-sheet { position: relative; }` +
		`#lattis-print .lp-page table { border-collapse: collapse; table-layout: fixed; }` +
		`#lattis-print .lp-page td { border: none; padding: 0 6px; line-height: ${lineHeight(BASE_PX)}px; vertical-align: middle; box-sizing: border-box; overflow: hidden; white-space: pre; position: relative; }` +
		`#lattis-print .lp-page td.t { border: 1px solid #c8d0cb; }` +
		`#lattis-print .lp-lines td { border: 1px solid #d4d7db; }` +
		`#lattis-print .lp-page th { font-weight: 400; font-size: 11px; background: #f0f0f0; border: 1px solid #b0b0b0; color: #444; text-align: center; padding: 0 2px; overflow: hidden; }` +
		`#lattis-print .lp-page td.ov { overflow: visible; }` +
		`#lattis-print .lp-ov { overflow: hidden; }` +
		`#lattis-print .lp-bar { position: absolute; top: 2px; bottom: 2px; }` +
		`#lattis-print .lp-axis { position: absolute; top: 1px; bottom: 1px; border-left: 1px dashed #000; }` +
		`#lattis-print .lp-ic { float: left; width: 16px; height: 16px; margin: 0 4px 0 -3px; vertical-align: middle; }` +
		`#lattis-print .lp-txt { position: relative; }` +
		`#lattis-print .lp-spark { position: absolute; inset: 0; } #lattis-print .lp-spark svg { display: block; }` +
		`#lattis-print .lp-objs { position: absolute; overflow: hidden; }` +
		`#lattis-print .lp-chart { position: absolute; overflow: hidden; background: #fff; }` +
		`#lattis-print .lp-chart svg { display: block; }` +
		`#lattis-print .lp-chart.lp-plain { background: none; overflow: visible; }`
	);
}

/** Tailles du plan : les lignes et colonnes, comme les axes de la grille. */
function axes(plan: PrintPlan) {
	const map = (list: number[][] | null) => new Map((list ?? []).map(([i, s]) => [i, s] as [number, number]));
	return { rows: new Axis(Number.MAX_SAFE_INTEGER, plan.rowPx || 22, map(plan.heights)), cols: new Axis(Number.MAX_SAFE_INTEGER, plan.colPx || 100, map(plan.widths)) };
}

/** Lignes (ou colonnes) d'une page : les titres qui précèdent la bande, puis la bande, sans les masquées. */
export function pageLines(band: number[], title: { from: number; to: number } | null | undefined, size: (i: number) => number): number[] {
	const out: number[] = [];
	if (title && title.from < band[0]) for (let i = title.from; i <= Math.min(title.to, band[0] - 1); i++) out.push(i);
	for (let i = band[0]; i <= band[1]; i++) out.push(i);
	return out.filter((i) => size(i) > 0);
}

/**
 * Pages HTML de la feuille, par lots (à ajouter tels quels à la zone
 * imprimée) ; le dernier morceau est la feuille de style des cellules.
 */
export async function* printChunks(plan: PrintPlan, src: PrintSource, opts: PrintOptions = {}): AsyncGenerator<string> {
	const batch = opts.batch ?? 10;
	const pause = opts.pause ?? nextTask;
	const { rows: rax, cols: cax } = axes(plan);
	const m = plan.margins;
	const pw = plan.paperW;
	// Bloc d'une page un peu plus court que le papier : un arrondi ne pousse pas une page blanche.
	const ph = plan.paperH - 0.6;
	const classes = new Map<string, string>();
	const classOf = (css: string) => {
		let k = classes.get(css);
		if (!k) classes.set(css, (k = 's' + classes.size));
		return k;
	};
	const icons = new Map<string, string>();
	const iconOf = (set: IconSetId, index: number) => {
		const k = `${set}:${index}`;
		if (!icons.has(k)) icons.set(k, opts.icon?.(set, index) ?? '');
		return icons.get(k)!;
	};
	const hfHtml = (h: { left?: string; center?: string; right?: string }, pos: 'top' | 'bottom', at: number) => {
		if (!h.left && !h.center && !h.right) return '';
		return `<div class="lp-hf" style="${pos}:${mm(at * 10)};left:${mm(m.left * 10)};right:${mm(m.right * 10)}"><span class="l">${esc(h.left ?? '')}</span><span class="c">${esc(h.center ?? '')}</span><span class="r">${esc(h.right ?? '')}</span></div>`;
	};
	const section = (body: string, header = '', footer = '') => {
		let style = `top:${mm(m.top * 10)};left:${mm(m.left * 10)};width:${mm(pw - (m.left + m.right) * 10)};height:${mm(ph - (m.top + m.bottom) * 10)}`;
		if (plan.centerH) style += ';align-items:center';
		if (plan.centerV) style += ';justify-content:center';
		return `<section class="lp-page${plan.blackWhite ? ' lp-bw' : ''}" style="width:${mm(pw)};height:${mm(ph)}">${header}<div class="lp-body" style="${style}">${body}</div>${footer}</section>`;
	};

	const pages = plan.pages ?? [];
	if (plan.empty || !pages.length) {
		yield section(`<p class="print-empty">${esc(opts.empty ?? '')}</p>`);
		yield `<style>${pageCss(plan)}</style>`;
		return;
	}
	const rowBands = plan.rows ?? [];
	const colBands = plan.cols ?? [];
	const area = plan.area;
	let out = '';
	let prepared = -1;
	for (let n = 0; n < pages.length; n++) {
		const pg = pages[n];
		const rb = rowBands[pg.row];
		const cb = colBands[pg.col];
		const rows = pageLines(rb, plan.titleRows, (r) => rax.size(r));
		const cols = pageLines(cb, plan.titleCols, (c) => cax.size(c));
		// Lignes chargées une fois par bande, sur toute la largeur imprimée ;
		// les lignes de titre à part (elles sont loin de la bande).
		const c0 = Math.min(area.c0, plan.titleCols?.from ?? area.c0);
		if (pg.row !== prepared) {
			const t = plan.titleRows;
			if (t && t.from < rb[0]) await src.prepare(t.from, Math.min(t.to, rb[0] - 1), c0, area.c1);
			await src.prepare(rb[0], rb[1], c0, area.c1);
			prepared = pg.row;
		}
		out += section(pageTable(rows, cols, rb, cb), hfHtml(pg.header, 'top', m.header), hfHtml(pg.footer, 'bottom', m.footer));
		if ((n + 1) % batch === 0 || n === pages.length - 1) {
			yield out;
			out = '';
			opts.progress?.(n + 1, pages.length);
			if (n < pages.length - 1) await pause();
		}
	}
	let css = pageCss(plan);
	for (const [decl, k] of classes) css += `#lattis-print .lp-page td.${k}{${decl}}`;
	yield `<style>${css}</style>`;

	/** Tableau d'une page : titres répétés, puis la bande ; graphiques par-dessus. */
	function pageTable(rows: number[], cols: number[], rb: number[], cb: number[]): string {
		const hw = plan.headings ? plan.headingsW : 0;
		const hh = plan.headings ? plan.headingsH : 0;
		let width = hw;
		for (const c of cols) width += cax.size(c);
		let b = `<div class="lp-sheet"${plan.scale !== 1 ? ` style="zoom:${plan.scale}"` : ''}><table class="print-grid${plan.gridlines ? ' lp-lines' : ''}" style="width:${width}px"><colgroup>`;
		if (hw) b += `<col style="width:${hw}px">`;
		for (const c of cols) b += `<col style="width:${cax.size(c)}px">`;
		b += '</colgroup><tbody>';
		if (hh) {
			b += `<tr style="height:${hh}px"><th></th>`;
			for (const c of cols) b += `<th>${colLetters(c)}</th>`;
			b += '</tr>';
		}
		const rowPos = new Map(rows.map((r, i) => [r, i]));
		const colPos = new Map(cols.map((c, j) => [c, j]));
		// Fusions : la cellule de départ s'étend sur ce qui en est visible ici.
		const spans = new Map<string, { rows: number; cols: number; w: number }>();
		const covered = new Set<string>();
		for (const g of src.merges) {
			const i0 = rowPos.get(g.row);
			const j0 = colPos.get(g.col);
			if (i0 === undefined || j0 === undefined) continue;
			let nr = 0;
			let nc = 0;
			let w = 0;
			for (let i = i0; i < rows.length && rows[i] < g.row + g.rows; i++) nr++;
			for (let j = j0; j < cols.length && cols[j] < g.col + g.cols; j++) {
				nc++;
				w += cax.size(cols[j]);
			}
			if (nr * nc <= 1) continue;
			spans.set(`${g.row},${g.col}`, { rows: nr, cols: nc, w });
			for (let i: number = i0; i < i0 + nr; i++) for (let j: number = j0; j < j0 + nc; j++) if (i !== i0 || j !== j0) covered.add(`${rows[i]},${cols[j]}`);
		}
		for (const r of rows) {
			b += `<tr style="height:${rax.size(r)}px">`;
			if (hw) b += `<th>${r + 1}</th>`;
			const line = cols.map((c) => (covered.has(`${r},${c}`) ? undefined : src.cell(r, c)));
			for (let j = 0; j < cols.length; j++) {
				const c = cols[j];
				const k = `${r},${c}`;
				if (covered.has(k)) continue;
				const x = line[j];
				const span = spans.get(k);
				let attr = span ? `${span.rows > 1 ? ` rowspan="${span.rows}"` : ''}${span.cols > 1 ? ` colspan="${span.cols}"` : ''}` : '';
				if (!x) {
					b += `<td${attr}></td>`;
					continue;
				}
				const look = { align: x.align, color: x.color, fill: x.fill, bold: x.bold };
				const cls: string[] = [];
				if (x.framed) cls.push('t');
				const css = cellCss(x.style, look);
				if (css) cls.push(classOf(css));
				// Texte qui déborde à droite, jusqu'à la première cellule occupée de la page.
				let ov = 0;
				if (x.overflow && x.text && !span && !x.style?.wr && (x.style?.al ?? x.align ?? 'left') === 'left') {
					ov = cax.size(c);
					for (let jj = j + 1; jj < cols.length; jj++) {
						const y = line[jj];
						if (y?.text || covered.has(`${r},${cols[jj]}`) || spans.has(`${r},${cols[jj]}`)) break;
						ov += cax.size(cols[jj]);
					}
					if (ov > cax.size(c)) cls.push('ov');
					else ov = 0;
				}
				if (cls.length) attr += ` class="${cls.join(' ')}"`;
				let inner = '';
				if (x.spark) inner += `<div class="lp-spark">${x.spark}</div>`;
				if (x.bar) {
					const bar = x.bar;
					const a = Math.max(0, Math.min(1, bar.a));
					const z = Math.max(a, Math.min(1, bar.b));
					const bg = bar.gradient ? `linear-gradient(${bar.rev ? 'to left' : 'to right'}, ${bar.color}, #fff)` : bar.color;
					inner += `<div class="lp-bar" style="left:calc(2px + ${a} * (100% - 4px));width:calc(${z - a} * (100% - 4px));background:${bg}${bar.border || bar.gradient ? `;border:1px solid ${bar.color}` : ''}"></div>`;
					if (bar.axis !== undefined) inner += `<div class="lp-axis" style="left:calc(2px + ${bar.axis} * (100% - 4px))"></div>`;
				}
				if (x.icon) {
					const src = iconOf(x.icon.set, x.icon.index);
					if (src) inner += `<img class="lp-ic" src="${esc(src)}" alt="">`;
				}
				if (x.text) {
					const pad = x.indent ? `padding-left:${x.indent}px` : '';
					const text = esc(x.text);
					if (ov) inner += `<div class="lp-ov" style="width:${ov - 12}px${pad ? ';' + pad : ''}">${text}</div>`;
					else if (pad || inner) inner += `<span class="lp-txt"${pad ? ` style="${pad};display:inline-block"` : ''}>${text}</span>`;
					else inner += text;
				}
				b += `<td${attr}>${inner}</td>`;
			}
			b += '</tr>';
		}
		b += '</tbody></table>';
		b += chartsHtml(rows, cols, rb, cb, hw, hh);
		return b + '</div>';
	}

	/** Graphiques de la bande (coupés à ses bords), au-dessus des cellules. */
	function chartsHtml(rows: number[], cols: number[], rb: number[], cb: number[], hw: number, hh: number): string {
		if (!src.charts.length) return '';
		// Place de la bande dans la page : après les en-têtes et les titres répétés.
		let tw = hw;
		for (const c of cols) if (c < cb[0]) tw += cax.size(c);
		let th = hh;
		for (const r of rows) if (r < rb[0]) th += rax.size(r);
		const x0 = cax.offset(cb[0]);
		const y0 = rax.offset(rb[0]);
		const bw = cax.offset(cb[1] + 1) - x0;
		const bh = rax.offset(rb[1] + 1) - y0;
		let s = '';
		for (const ch of src.charts) {
			const x = cax.offset(ch.col) + ch.x - x0;
			const y = rax.offset(ch.row) + ch.y - y0;
			if (x >= bw || y >= bh || x + ch.w <= 0 || y + ch.h <= 0) continue;
			s += `<div class="lp-chart${ch.plain ? ' lp-plain' : ''}" style="left:${x}px;top:${y}px;width:${ch.w}px;height:${ch.h}px">${ch.html}</div>`;
		}
		return s ? `<div class="lp-objs" style="left:${tw}px;top:${th}px;width:${bw}px;height:${bh}px">${s}</div>` : '';
	}
}

/** Toutes les pages en une chaîne (essais, petites feuilles). */
export async function printHtml(plan: PrintPlan, src: PrintSource, opts: PrintOptions = {}): Promise<string> {
	let s = '';
	for await (const chunk of printChunks(plan, src, { pause: async () => {}, ...opts })) s += chunk;
	return s;
}
