// Largeurs et hauteurs calculées : texte, cellules, en-têtes, ajustement
// automatique des lignes (rowfit.ts) et des colonnes. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { clamp } from '../scroll';
import { Axis } from '../axis';
import { type AxisLike } from '../anim';
import { resolveStyle, type CellStyle, type StyleLayer } from '../style';
import { formatCell } from '../numfmt';
import { plain } from '../numcode';
import { autoHeight, cellFont, headerHeight, headerTextWidth } from '../textwrap';
import { AutoRows } from '../rowfit';
import { mergeAt, type MergeRange } from '../sheetfmt';
import { freeDisplay } from '../freecell';
import { groupSpans, headerLabel } from '../twolevel';
import { pivotLabelOffset } from '../pivottree';
import { defaultWidth, typeIcon, typeIconName } from '../format';
import { type SheetPivot, type SheetSource, type Entry, pivotRowOf, pivotBold } from './model';
import { SHEET_COLS, ROW_H, DEF_W, FONT, FONT_BOLD, FONT_SMALL } from './consts';

/** Ce que createMeasure lit et modifie dans Sheet.svelte. */
export interface MeasureHost {
	measurer: CanvasRenderingContext2D | null;
	readonly source: SheetSource;
	readonly entries: Entry[];
	readonly MAX_FIT: number;
	readonly pivots: SheetPivot[];
	readonly userWidths: Map<number, number>;
	readonly hiddenCols: Set<number>;
	colAxis: Axis;
	readonly MAX_AUTO_H: number;
	readonly cax: () => AxisLike;
	readonly merges: MergeRange[];
	readonly cells: Map<string, string>;
	readonly shown: Map<string, { text: string; error: boolean; }>;
	readonly styleAt: (r: number, c: number, e?: Entry | undefined) => CellStyle | undefined;
	readonly sheetRowStyles: Map<number, StyleLayer>;
	readonly colStyles: Map<number, (StyleLayer | undefined)[]>;
	readonly sheetColStyles: Map<number, StyleLayer>;
	readonly singleCol: (e: Entry, i: number) => boolean;
	readonly rh: (r: number) => number;
	readonly bandStyle: (e: Entry, i0: number, i1: number) => CellStyle | undefined;
	readonly headIconW: Map<string, number>;
	readonly HEAD_ICON_W: number;
	readonly visibleRows: () => { first: number; last: number; };
	readonly autoRows: AutoRows;
	readonly fRows: () => number;
	readonly userHeights: Map<number, number>;
	readonly hiddenRows: Set<number>;
	rowAxis: Axis;
	readonly updateExtent: () => void;
}

export function createMeasure(sh: MeasureHost) {
	/** Largeur du texte dans la police de la grille (normale ou grasse). */
	function textWidth(s: string, bold = false): number {
		if (!s) return 0;
		sh.measurer ??= document.createElement('canvas').getContext('2d');
		if (!sh.measurer) return 0;
		sh.measurer.font = bold ? FONT_BOLD : FONT;
		return sh.measurer.measureText(s).width;
	}
	/** Largeur du texte affiché d'une cellule : son format, sa police et sa taille. */
	function cellWidth(text: string, value: unknown, st: CellStyle | undefined): number {
		if (!st) return textWidth(text);
		const s = plain(formatCell(text, value, st));
		sh.measurer ??= document.createElement('canvas').getContext('2d');
		if (!s || !sh.measurer) return 0;
		sh.measurer.font = cellFont(st);
		return sh.measurer.measureText(s).width;
	}
	/**
	 * Largeur du contenu de l'en-tête de la colonne i d'un tableau (nom en
	 * gras, badge du type, ▾, entonnoir, clé posée ou proposée), sans les marges : le nom n'est
	 * pas coupé au-delà (headIcons et le dessin de l'en-tête).
	 */
	function headerWidth(e: Entry, i: number): number {
		const col = e.t.columns[i];
		const icon = (col.filter ? '⧩ ' : '') + (typeIconName(col.type) ? ' ▾' : typeIcon(col.type) + ' ▾') + (e.t.key === i || (i === 0 && (!!e.t.keyHint || !!e.t.keyPending) && e.t.key === undefined && !!sh.source.setKey) ? '🔑 ' : '');
		sh.measurer ??= document.createElement('canvas').getContext('2d');
		if (sh.measurer) sh.measurer.font = FONT_SMALL;
		const iconW = (sh.measurer?.measureText(icon).width ?? 0) + (typeIconName(col.type) ? 13 : 0);
		return textWidth(col.name, true) + iconW + 4;
	}
	function rebuildAxis(override?: { col: number; width: number }) {
		const widths = new Map<number, number>();
		for (const e of sh.entries) {
			e.t.columns.forEach((c, i) => {
				// Largeur selon le type, élargie pour que l'en-tête (nom, badge
				// du type et ▾) ne soit pas coupé.
				if (!widths.has(e.t.col + i)) widths.set(e.t.col + i, Math.max(defaultWidth(c.type), Math.min(sh.MAX_FIT, Math.ceil(headerWidth(e, i)) + 14)));
			});
		}
		// Tableaux croisés : largeur d'après le texte affiché (« Somme de Commande »).
		for (const p of sh.pivots) {
			if (p.error) continue;
			p.cells[0]?.forEach((_, j) => {
				const c = p.col + j;
				if (widths.has(c)) return;
				const w = Math.max(...p.cells.map((r, i) => textWidth(r[j] ?? '', pivotBold(p, p.row + i)) + (j === 0 ? pivotLabelOffset(pivotRowOf(p, p.row + i)) : 0)));
				widths.set(c, clamp(Math.ceil(w) + 14, DEF_W, sh.MAX_FIT));
			});
		}
		for (const [c, w] of sh.userWidths) widths.set(c, w);
		if (override) widths.set(override.col, override.width);
		for (const c of sh.hiddenCols) widths.set(c, 0); // masquée : largeur nulle
		sh.colAxis = new Axis(SHEET_COLS, DEF_W, widths);
	}

	/** Hauteur nécessaire au texte d'une cellule (0 : la hauteur par défaut suffit). */
	function fitHeight(text: string, c: number, st: CellStyle | undefined): number {
		if ((!st || (!st.wr && !st.fs)) && !text.includes('\n')) return 0;
		sh.measurer ??= document.createElement('canvas').getContext('2d');
		if (!sh.measurer) return 0;
		sh.measurer.font = cellFont(st);
		const m = sh.measurer;
		return Math.min(sh.MAX_AUTO_H, autoHeight(text, sh.cax().size(c), st ?? {}, (s) => m.measureText(s).width, ROW_H));
	}

	/** Hauteurs automatiques des lignes, comme Excel : cellules libres qui renvoient à la ligne, sur plusieurs lignes ou en grande police. */
	function measureHeights(): Map<number, number> {
		const out = new Map<number, number>();
		const consider = (r: number, c: number, text: string, st: CellStyle | undefined) => {
			if (mergeAt(sh.merges, r, c)) return;
			const h = fitHeight(text, c, st);
			if (h > (out.get(r) ?? 0)) out.set(r, h);
		};
		for (const [k, raw] of sh.cells) {
			const [r, c] = k.split(',').map(Number);
			consider(r, c, sh.shown.get(k)?.text ?? freeDisplay(raw).text, sh.styleAt(r, c, undefined));
		}
		// Grande police posée sur des lignes entières ou des cellules vides.
		for (const [r, st] of sh.sheetRowStyles) if (st.fs) consider(r, -1, '', resolveStyle(st));
		return out;
	}

	/**
	 * Hauteur nécessaire aux cellules de tableau d'une ligne de la feuille
	 * (0 : la hauteur par défaut suffit), undefined si la ligne n'est pas
	 * encore chargée. Seules les cellules qui peuvent passer à la ligne ou
	 * grossir sont mesurées.
	 */
	function measureTableRow(r: number): number | undefined {
		let h = 0;
		const rowSt = sh.sheetRowStyles.get(r);
		const rowFit = !!rowSt && ('wr' in rowSt || 'fs' in rowSt);
		for (const e of sh.entries) {
			if (r === e.t.row) {
				h = Math.max(h, measureHeader(e));
				continue;
			}
			if (e.t.twoLevel && r === e.t.row - 1) {
				h = Math.max(h, measureBand(e));
				continue;
			}
			if (r < e.t.row || r > e.t.row + e.t.rows) continue;
			const row = e.cache.row(r - e.t.row - 1);
			if (!row) return undefined;
			const cs = sh.colStyles.get(e.t.id);
			for (let i = 0; i < e.t.columns.length; i++) {
				const c = e.t.col + i;
				const text = row.text[i] ?? '';
				if (!sh.cax().size(c) || (!rowFit && !cs?.[i] && !sh.sheetColStyles.has(c) && !row.sty?.has(i) && !text.includes('\n'))) continue;
				h = Math.max(h, fitHeight(text, c, sh.styleAt(r, c, e)));
			}
		}
		return h;
	}

	/**
	 * Hauteur nécessaire à l'en-tête d'un tableau dont des noms renvoient à la
	 * ligne (0 : la hauteur par défaut suffit). Une colonne sur un niveau
	 * (bande et en-tête réunis) profite déjà de la hauteur de la bande.
	 */
	function measureHeader(e: Entry): number {
		let h = 0;
		for (let i = 0; i < e.t.columns.length; i++) {
			const c = e.t.col + i;
			const cw = sh.cax().size(c);
			const single = sh.singleCol(e, i) && !!sh.rh(e.t.row - 1);
			const hs = single ? sh.bandStyle(e, i, i) : sh.styleAt(e.t.row, c, e);
			if (!cw || !hs?.wr) continue;
			const label = single ? e.t.columns[i].name : headerLabel(e.t.columns[i].name, e.t.twoLevel);
			let need = headNeed(label, headerTextWidth(cw, sh.headIconW.get(`${e.t.id}:${i}`) ?? sh.HEAD_ICON_W), hs);
			if (single) need -= sh.rh(e.t.row - 1);
			h = Math.max(h, need);
		}
		return Math.min(sh.MAX_AUTO_H, h > ROW_H ? h : 0);
	}

	/** Hauteur nécessaire à la bande des groupes d'un tableau dont des groupes renvoient à la ligne (0 : la hauteur par défaut suffit). */
	function measureBand(e: Entry): number {
		let h = 0;
		for (const s of groupSpans(e.t.columns.map((k) => k.name))) {
			if (!s.group) continue;
			const st = sh.bandStyle(e, s.i0, s.i1);
			const sw = sh.cax().offset(e.t.col + s.i1 + 1) - sh.cax().offset(e.t.col + s.i0);
			if (sw > 0 && st?.wr) h = Math.max(h, headNeed(s.group, sw - 12, st));
		}
		return Math.min(sh.MAX_AUTO_H, h > ROW_H ? h : 0);
	}

	/** Hauteur d'un nom d'en-tête renvoyé à la ligne dans width (police de l'en-tête). */
	function headNeed(label: string, width: number, st: CellStyle): number {
		sh.measurer ??= document.createElement('canvas').getContext('2d');
		if (!sh.measurer) return 0;
		const m = sh.measurer;
		m.font = (st.i ? 'italic ' : '') + FONT_BOLD;
		return headerHeight(label, width, st, (t) => m.measureText(t).width);
	}

	/** Mesure les lignes de tableau affichées pas encore mesurées et ajuste l'axe des lignes (seulement les lignes changées). */
	function fitVisibleRows() {
		if (!sh.entries.length) return;
		for (let pass = 0; pass < 3; pass++) {
			const { first, last } = sh.visibleRows();
			const changed = sh.autoRows.measure(first, last, measureTableRow);
			if (sh.fRows()) for (const [r, h] of sh.autoRows.measure(0, sh.fRows() - 1, measureTableRow)) changed.set(r, h);
			const sizes = new Map<number, number | undefined>();
			for (const [r, h] of changed) if (!sh.userHeights.has(r) && !sh.hiddenRows.has(r)) sizes.set(r, h);
			if (!sizes.size) return;
			sh.rowAxis = sh.rowAxis.with(sizes);
			sh.updateExtent();
		}
	}

	return {
		textWidth,
		cellWidth,
		headerWidth,
		rebuildAxis,
		measureHeights,
		measureHeader,
		measureBand,
		fitVisibleRows
	};
}
