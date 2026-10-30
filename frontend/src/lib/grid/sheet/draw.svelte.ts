// Dessin de la grille sur le canvas, volet par volet : cellules (tableaux,
// TCD, cellules libres, mise en forme), bandeau du haut et gouttière,
// fusions, sparklines, pastilles, reflets d'une action en cours. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { syncAt, type SyncKind } from '../cache';
import { VScroll } from '../scroll';
import { isDefaultTableName, isGeneratedColumnName } from '../defaultname';
import { type AxisLike, type BarSpan, type Geo } from '../anim';
import { sweepAt, workAlpha, type WorkZone } from '../working';
import { type Clip } from '../clipboard';
import { resolveStyle, type CellStyle, type StyleLayer } from '../style';
import { formatCell, shownCell } from '../numfmt';
import { FILL, plain } from '../numcode';
import { BorderSet } from '../borders';
import { BASE_PX, cellFont, fontPx, lineHeight, wrapLines } from '../textwrap';
import { AutoRows } from '../rowfit';
import { arrowSpan, spanBox, spanPoint, type ArrowGeometry } from '../auditarrows';
import { panes, type Pane } from '../panes';
import { type MergeRange } from '../sheetfmt';
import { intersect, type CellRange } from '../mergesel';
import { inZone, type Zone } from '../findzone';
import { type FindOpts, type FoundCell } from '../findopts';
import { drawSparkline, freeSparkAt, sparkGeometry, sparkValue, type SparklineGroup } from '../sparkline';
import { linkKind } from '../hyperlink';
import { freeDisplay } from '../freecell';
import { bandLabel, bandOpen, hiddenBand } from '../hidden';
import { placeFollowBadge, showNameBadge } from '../followbadge';
import { groupSpans, headerLabel } from '../twolevel';
import { pivotValueText } from '../pivotformat';
import { pivotBoxLeft, pivotLabelOffset, PIVOT_BOX } from '../pivottree';
import { paint as themed } from '../colors';
import { cellLines } from '../shortcuts';
import { besideGutter, gutterOwner } from '../gutter';
import { clip } from '#lib/ui/menu.js';
import { midEllipsis } from '../midellipsis';
import { handAlign, handStart } from '../pivothand';
import { type Placement } from '../pivotplace';
import { type CfOut, type Rule } from '../rules';
import { ICON_SETS, drawCfIcon } from '../cficons';
import { drawIcon } from '#lib/icons/canvas.js';
import { formulaRefs, previewShift, refRect, type Rect } from '../formulaEdit';
import { type FormulaHold } from '../crossref';
import OutlineBar from '../OutlineBar.svelte';
import { isErrorValue, valueAlignment, alignment, NULL_MARK, showsNull, typeIcon, typeIconName } from '../format';
import { t as tr } from '#lib/i18n/index.js';
import { type PivotConfig, type SheetPivot, type PageBreaksView, type SheetData, type Box, type TableAnim, type HiddenMark, type Link, type Range, type Entry, pivotTotalRow, pivotRowOf, pivotBold, type Layer, type DvPick } from './model';
import { SHEET_ROWS, TOP_H, MIN_MARGIN, GUTTER_PAD, GUTTER_MAX, GUTTER_KEY_MAX, FONT, FONT_BOLD, FONT_SMALL, FONT_TINY } from './consts';
import type { Grown } from '../grown';
import type { Loc } from '../cfmodel';
import type { RefTable } from '../formulaEdit';

/** Ce que createDraw lit et modifie dans Sheet.svelte. */
export interface DrawHost {
	readonly ctx: CanvasRenderingContext2D;
	readonly a1: boolean;
	readonly extentRows: number;
	readonly entries: Entry[];
	readonly keyWidths: Map<string, number>;
	readonly barShown: (k: string, b: BarSpan & { axis?: number; rev?: boolean; }, r: number, c: number) => BarSpan;
	frame: number;
	readonly borderSet: BorderSet;
	viewPos: { x: number; y: number; };
	readonly scroller: HTMLDivElement;
	readonly vs: VScroll;
	drawing: boolean;
	readonly fitVisibleRows: () => void;
	readonly beginAnimFrame: (now: number) => void;
	readonly viewW: number;
	readonly viewH: number;
	readonly zoom: number;
	selBox: Box;
	readonly frameNow: (now: number) => Box;
	findCross: Zone | null;
	readonly foundRect: () => Zone | null;
	pinnedFrame: { x: number; y: number; text: string }[];
	keyHintBoxes: { e: Entry; x0: number; x1: number; y0: number; y1: number }[];
	followBadges: { kind: 'follow' | 'name' | 'unnamed' | 'local'; e: Entry; label: string; radii: number[]; x0: number; x1: number; y0: number; y1: number; clip: { x0: number; x1: number; y0: number; y1: number } }[];
	pivotBadges: { p: SheetPivot; label: string; radii: number[]; x0: number; x1: number; y0: number; y1: number; clip: { x0: number; x1: number; y0: number; y1: number } }[];
	hiddenMarks: HiddenMark[];
	readonly C: { bg: string; calc: string; grid: string; strip: string; stripLine: string; pageBreak: string; stripText: string; head: string; linkHead: string; linkHeadText: string; linkBorder: string; linkIcon: string; followBadge: string; nameBadge: string; unnamedBadge: string; sourceBadge: string; localBadge: string; headText: string; icon: string; border: string; gray: string; text: string; placeholder: string; select: string; active: string; activeStrip: string; error: string; link: string; spill: string; found: string; findCross: string; findGutter: string; frozen: string; nullMark: string; note: string; pending: string; conflict: string; };
	MARGIN: number;
	readonly updateExtent: () => void;
	keyGrew: boolean;
	readonly cax: () => AxisLike;
	readonly fCols: () => number;
	readonly rowTop: (r: number) => number;
	readonly fRows: () => number;
	curPane: Pane | null;
	missingFrames: number;
	readonly motion: () => boolean;
	viewNow: { r0: number; r1: number; c0: number; c1: number } | null;
	readonly rax: () => AxisLike;
	readonly editing: boolean;
	readonly placeEditor: () => void;
	readonly drawValidation: () => void;
	pinnedShown: { x: number; y: number; text: string }[];
	readonly schedule: () => void;
	readonly outlineBar: ReturnType<typeof OutlineBar> | undefined;
	readonly endAnimFrame: () => void;
	readonly pushStat: (a: number[], v: number) => void;
	readonly drawTimes: number[];
	readonly frameStamps: number[];
	readonly topRow: (t: { row: number; twoLevel?: boolean; }) => number;
	readonly selection: () => CellRange;
	readonly sparkGroups: SparklineGroup[];
	readonly rh: (r: number) => number;
	paneTop: number;
	readonly visibleData: (e: Entry, first: number, last: number) => { first: number; last: number; } | null;
	readonly grown: (e: Entry) => Grown;
	readonly enterTable: (e: Entry, cx: (c: number) => number, ry: (r: number) => number) => boolean;
	readonly tableAnim: (e: Entry) => TableAnim | null;
	readonly singleSelection: () => boolean;
	readonly activeRect: () => CellRange;
	readonly rspan: (r0: number, r1: number) => number;
	readonly finder: {
		replace: boolean;
		text: string;
		by: string;
		count: string;
		info: string;
		zone: Zone | null;
		at: { row: number; col: number } | null;
		opts: FindOpts;
		more: boolean;
		list: FoundCell[] | null;
		total: number;
		busy: boolean;
		current: number;
	} | null;
	readonly matches: FoundCell[];
	readonly matchKey: string;
	readonly matchesKey: (text: string, zone: Zone | null, o: FindOpts) => string;
	readonly data: SheetData;
	readonly singleCol: (e: Entry, i: number) => boolean;
	readonly bandStyle: (e: Entry, i0: number, i1: number) => CellStyle | undefined;
	readonly styleAt: (r: number, c: number, e?: Entry | undefined) => CellStyle | undefined;
	readonly borders: (st: CellStyle, x: number, y: number, w: number, h: number) => void;
	readonly singleIcons: Map<string, number>;
	readonly colRules: Map<number, (Rule[] | undefined)[]>;
	readonly cfAt: (r: number, c: number, val: { v: unknown; text: string; error: boolean; }, own?: { rules: Rule[] | undefined; loc: Loc; mark: (r: Rule, k: number) => boolean; }) => CfOut | undefined;
	readonly hoverLocked: number;
	readonly columnFlashes: (table: number, name: string, k: string) => void;
	readonly paintFlash: (a: number, x: number, y: number, w: number, h: number) => void;
	readonly flashOf: (k: string, v: string, r: number, c: number) => number;
	readonly key: (r: number, c: number) => string;
	readonly freeCf: (r: number, c: number) => CfOut | undefined;
	readonly tableAt: (r: number, c: number) => Entry | undefined;
	readonly groupBandAt: (r: number, c: number) => Entry | undefined;
	readonly mergeOf: (r: number, c: number) => MergeRange | undefined;
	readonly sheetRowStyles: Map<number, StyleLayer>;
	readonly sheetColStyles: Map<number, StyleLayer>;
	readonly sheetRowRules: Map<number, Rule[]>;
	readonly sheetColRules: Map<number, Rule[]>;
	readonly freeStyles: Map<string, StyleLayer>;
	readonly spills: { row: number; col: number; rows: string[][] }[];
	readonly pivots: SheetPivot[];
	readonly pivotFilterText: (values: string[]) => string;
	readonly PIVOT_DROP: number;
	readonly pivotFiltered: (p: SheetPivot, f: string) => boolean;
	readonly cells: Map<string, string>;
	readonly pivotAt: (r: number, c: number) => SheetPivot | undefined;
	readonly fitText: (t: string, w: number) => string;
	readonly spilledAt: (r: number, c: number) => boolean;
	readonly fxEditing: boolean;
	readonly hold: FormulaHold | null;
	readonly active: { row: number; col: number; };
	readonly shown: Map<string, { text: string; error: boolean; }>;
	readonly gridlines: boolean;
	readonly tableGeo: (e: Entry) => Geo;
	readonly geoDrawn: Map<number, Geo>;
	readonly onaddshared: ((table: string) => void) | undefined;
	readonly sheetName: string;
	readonly pageBreaks: PageBreaksView | undefined;
	readonly clipRange: Clip | null;
	readonly sheetId: () => number;
	readonly animNow: number;
	animBusy: boolean;
	readonly moving: {
		sel: { r0: number; r1: number; c0: number; c1: number };
		from: { row: number; col: number };
		to?: { row: number; col: number };
		/** Ctrl enfoncé : copie au lieu de déplacer (Excel). */
		copy?: boolean;
		/** Tableau croisé emporté en entier (sélection prise dans un seul TCD) : sel est sa zone. */
		pivot?: number;
	} | null;
	readonly placing: Placement<PivotConfig> | null;
	readonly placeHover: { row: number; col: number } | null;
	readonly placeSize: { h: number; w: number; };
	readonly ownerAt: (r: number, c: number) => Entry | undefined;
	readonly scopeTints: Range[];
	readonly dvPick: DvPick | null;
	readonly dvPickRef: string;
	readonly refContext: () => { tables: RefTable[]; at: { table: RefTable; row: number; } | undefined; };
	readonly formulaMode: boolean;
	readonly editValueText: string;
	readonly fxText: string;
	readonly refMove: { start: number; end: number; rect: Rect; from: { row: number; col: number }; to: { row: number; col: number } } | null;
	readonly locked: string;
	readonly listOf: (r: number, c: number) => string[] | undefined;
	readonly dropButton: () => { x: number; y: number; w: number; h: number; };
	readonly refAt: (r: number, c: number) => { src: string; table: string; key: string; value: string; } | null;
	readonly linkButton: () => { x: number; y: number; w: number; h: number; };
	tableHintBox: { x: number; y: number; w: number; h: number } | null;
	readonly asTableHint: () => boolean;
	readonly links: Link[];
	readonly sheetIdOf: () => number;
	readonly fxInput: HTMLInputElement | undefined;
	readonly selChart: number | null;
	readonly fillHandle: () => { x: number; y: number; };
	readonly filling: { sel: Range; to?: Range } | null;
	readonly drawCommentMarks: (L: Layer) => void;
	readonly addrA1: (r: number, c: number) => string;
	readonly hiddenRuns: (set: Set<number>) => [number, number][];
	readonly styleHiddenCols: Set<number>;
	readonly hiddenMark: (m: HiddenMark) => void;
	readonly styleHiddenRows: Set<number>;
	readonly merges: MergeRange[];
	readonly isNumeric: (s: string) => boolean;
	readonly headIconW: Map<string, number>;
	readonly HEAD_ICON_W: number;
	readonly autoRows: AutoRows;
	readonly calcEdit: { e: Entry; i: number; } | null;
	readonly hasKeyHint: (e: Entry, i: number) => boolean;
	readonly working: { id: number; zones: WorkZone[]; t0: number }[];
	readonly rowAtY: (y: number) => number;
	readonly colAtX: (x: number) => number;
	readonly screenX: (c: number) => number;
	readonly screenY: (r: number) => number;
	readonly jumpFlash: { r: number; c0: number; c1: number; t: number } | null;
	readonly JUMP_FLASH_MS: number;
}

export function createDraw(sh: DrawHost) {
	function gutterWidth(): number {
		const { ctx } = sh;
		const m = (s: string) => ctx.measureText(s).width;
		let need = m('99');
		if (sh.a1) need = Math.max(need, m(String(sh.extentRows)));
		else
			for (const e of sh.entries) {
				need = Math.max(need, m(String(e.t.rows)));
				if (e.t.key !== undefined) need = Math.max(need, sh.keyWidths.get(e.t.name) ?? 0);
			}
		return Math.min(GUTTER_MAX, Math.max(MIN_MARGIN, Math.ceil(need) + GUTTER_PAD));
	}

	/** Barre de données dans la cellule (x, y, w, h). */
	function drawBar(b: NonNullable<CfOut['bar']>, x: number, y: number, w: number, h: number, k: string, r: number, c: number) {
		const { ctx } = sh;
		const pad = 2;
		const span = Math.max(0, w - 2 * pad);
		const shown = sh.barShown(k, b, r, c);
		const x0 = x + pad + shown.a * span;
		const x1 = x + pad + shown.b * span;
		const by = y + 2;
		const bh = Math.max(2, h - 4);
		if (x1 - x0 >= 0.5) {
			if (b.gradient) {
				const g = b.rev ? ctx.createLinearGradient(x1, 0, x0, 0) : ctx.createLinearGradient(x0, 0, x1, 0);
				g.addColorStop(0, b.color);
				g.addColorStop(1, '#ffffff');
				ctx.fillStyle = g;
			} else ctx.fillStyle = b.color;
			ctx.fillRect(x0, by, x1 - x0, bh);
			if (b.border || b.gradient) {
				ctx.strokeStyle = b.color;
				ctx.lineWidth = 1;
				ctx.strokeRect(Math.round(x0) + 0.5, by + 0.5, Math.max(0, Math.round(x1 - x0) - 1), bh - 1);
			}
		}
		if (b.axis !== undefined) {
			const ax = Math.round(x + pad + b.axis * span) + 0.5;
			ctx.save();
			ctx.strokeStyle = '#000';
			ctx.setLineDash([2, 2]);
			ctx.beginPath();
			ctx.moveTo(ax, y + 1);
			ctx.lineTo(ax, y + h - 1);
			ctx.stroke();
			ctx.restore();
		}
	}

	/** Icône d'un jeu d'icônes à gauche de la cellule ; renvoie la place prise. */
	function drawIconAt(icon: NonNullable<CfOut['icon']>, x: number, y: number, h: number): number {
		const { ctx } = sh;
		const spec = ICON_SETS[icon.set]?.icons[icon.index];
		if (!spec) return 0;
		const s = Math.min(16, h - 4);
		drawCfIcon(ctx, spec, x + 3, y + (h - s) / 2, s);
		return s + 4;
	}

	function draw() {
		const { ctx, C, cax, rax, rowTop } = sh;
		sh.frame = 0;
		sh.borderSet.clear();
		if (sh.viewPos.x !== sh.scroller.scrollLeft || sh.viewPos.y !== sh.vs.top) sh.viewPos = { x: sh.scroller.scrollLeft, y: sh.vs.top };
		const t0 = performance.now();
		sh.drawing = false;
		sh.fitVisibleRows();
		sh.beginAnimFrame(t0);
		const w = sh.viewW;
		const h = sh.viewH;
		const dpr = (window.devicePixelRatio || 1) * sh.zoom;
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		sh.selBox = sh.frameNow(t0);
		sh.findCross = sh.foundRect();
		sh.pinnedFrame = [];
		sh.keyHintBoxes = [];
		sh.followBadges = [];
		sh.pivotBadges = [];
		sh.hiddenMarks = [];
		// Texte centré verticalement partout (les volets dessinent entre save et
		// restore : la ligne de base doit être posée avant, pour la marge et le bandeau).
		ctx.textBaseline = 'middle';
		ctx.fillStyle = C.bg;
		ctx.fillRect(0, 0, w, h);

		ctx.font = FONT_SMALL;
		const margin = gutterWidth();
		if (margin !== sh.MARGIN) {
			sh.MARGIN = margin;
			sh.updateExtent();
		}
		sh.keyGrew = false;
		const fw = cax().offset(sh.fCols());
		const fh = rowTop(sh.fRows());
		const all = panes(fw, fh, sh.MARGIN, TOP_H, w, h, sh.scroller.scrollLeft, sh.vs.top);
		const main = all[all.length - 1] ?? { left: sh.scroller.scrollLeft, top: sh.vs.top, x0: sh.MARGIN, x1: w, y0: TOP_H, y1: h };
		let incomplete = false;
		for (const p of all) {
			ctx.save();
			ctx.beginPath();
			ctx.rect(p.x0, p.y0, p.x1 - p.x0, p.y1 - p.y0);
			ctx.clip();
			sh.curPane = p;
			if (drawCells(layer(p))) incomplete = true;
			sh.curPane = null;
			ctx.restore();
		}
		if (incomplete) sh.missingFrames++;
		drawJumpFlash(t0);
		drawWorking(t0);
		if (sh.motion())
			sh.viewNow = {
				r0: rax().indexAt(main.y0 - TOP_H + main.top),
				r1: rax().indexAt(main.y1 - 1 - TOP_H + main.top),
				c0: cax().indexAt(main.x0 - sh.MARGIN + main.left),
				c1: cax().indexAt(main.x1 - 1 - sh.MARGIN + main.left)
			};
		// Bandeau du haut (par volet de colonnes) et marge de gauche (par volet de lignes).
		for (const p of all.filter((p) => p.y0 === main.y0)) {
			ctx.save();
			ctx.beginPath();
			ctx.rect(p.x0, 0, p.x1 - p.x0, TOP_H);
			ctx.clip();
			drawTopStrip(layer({ ...p, top: main.top, y0: main.y0, y1: main.y1 }));
			ctx.restore();
		}
		for (const p of all.filter((p) => p.x0 === main.x0)) {
			ctx.save();
			ctx.beginPath();
			ctx.rect(0, p.y0, sh.MARGIN, p.y1 - p.y0);
			ctx.clip();
			drawMargin(layer({ ...p, left: main.left, x0: main.x0, x1: main.x1 }));
			ctx.restore();
		}
		drawFollowBadges();
		ctx.fillStyle = C.strip;
		ctx.fillRect(0, 0, sh.MARGIN, TOP_H);
		ctx.beginPath();
		ctx.strokeStyle = C.stripLine;
		ctx.lineWidth = 1;
		ctx.moveTo(0, TOP_H - 0.5);
		ctx.lineTo(w, TOP_H - 0.5);
		ctx.moveTo(sh.MARGIN - 0.5, 0);
		ctx.lineTo(sh.MARGIN - 0.5, h);
		ctx.stroke();
		// Séparations des volets figés : un trait plus marqué, comme Excel.
		if (fh || fw) {
			ctx.beginPath();
			ctx.strokeStyle = C.frozen;
			if (fh) {
				ctx.moveTo(0, Math.round(TOP_H + fh) - 0.5);
				ctx.lineTo(w, Math.round(TOP_H + fh) - 0.5);
			}
			if (fw) {
				ctx.moveTo(Math.round(sh.MARGIN + fw) - 0.5, 0);
				ctx.lineTo(Math.round(sh.MARGIN + fw) - 0.5, h);
			}
			ctx.stroke();
		}

		if (sh.editing) sh.placeEditor();
		sh.drawValidation();

		const seen = new Set<string>();
		const pins = sh.pinnedFrame.filter((p) => !seen.has(p.x + ',' + p.y) && !!seen.add(p.x + ',' + p.y));
		if (JSON.stringify(pins) !== JSON.stringify(sh.pinnedShown)) sh.pinnedShown = pins;
		// Une clé plus large que toutes celles vues : gouttière élargie à
		// l'image suivante (les clés ne sont connues qu'une fois chargées).
		if (sh.keyGrew) sh.schedule();
		sh.outlineBar?.paint();
		sh.endAnimFrame();

		const t1 = performance.now();
		sh.pushStat(sh.drawTimes, t1 - t0);
		sh.frameStamps.push(t1);
		while (sh.frameStamps.length && sh.frameStamps[0] < t1 - 1000) sh.frameStamps.shift();
	}

	/** Ce qu'un volet affiche : lignes et colonnes visibles, positions à l'écran. */
	function layer(p: Pane): Layer {
		const { cax, rax, topRow, rowTop, MARGIN } = sh;
		const w = sh.viewW;
		const h = sh.viewH;
		const { left, top } = p;
		const r0 = rax().indexAt(p.y0 - TOP_H + top);
		const r1 = Math.min(SHEET_ROWS - 1, rax().indexAt(Math.max(p.y0, p.y1 - 1) - TOP_H + top));
		const c0 = cax().indexAt(p.x0 - MARGIN + left);
		const c1 = cax().indexAt(Math.max(p.x0, p.x1 - 1) - MARGIN + left);
		const cx = (c: number) => MARGIN + cax().offset(c) - left;
		const ry = (r: number) => TOP_H + rowTop(r) - top;
		const xEnd = Math.min(w, cx(c1 + 1));
		const spanW = (a: number, b: number) => cax().offset(b + 1) - cax().offset(a);
		const visible = sh.entries.filter((e) => e.last + 1 >= r0 && topRow(e.t) <= r1 && e.lastCol + 1 >= c0 && e.t.col <= c1);
		// Plage sélectionnée.
		const { r0: s0, r1: s1, c0: k0, c1: k1 } = sh.selection();
		return { w, h, top, r0, r1, c0, c1, cx, ry, xEnd, spanW, visible, s0, s1, k0, k1 };
	}

	/**
	 * Graphiques sparkline des cellules visibles du volet : ceux d'un tableau
	 * lisent les lignes déjà chargées (rien n'est demandé de plus), ceux des
	 * cellules libres leurs valeurs données avec la mise en page.
	 */
	function drawSparklines(L: Layer) {
		const { ctx, rh, cax } = sh;
		if (!sh.sparkGroups.length) return;
		const { r0, r1, c0, c1, cx, ry } = L;
		const one = (g: SparklineGroup, values: (number | null)[], r: number, c: number) => {
			const w = cax().size(c);
			const h = rh(r);
			if (!w || !h) return;
			const x = cx(c);
			const y = ry(r);
			ctx.save();
			ctx.beginPath();
			ctx.rect(x, y, w, h);
			ctx.clip();
			drawSparkline(ctx, sparkGeometry(values, g.config, w, h, g.config.same ? { lo: g.lo, hi: g.hi } : undefined), x, y, w);
			ctx.restore();
		};
		for (const g of sh.sparkGroups) {
			if (g.error) continue;
			if (g.config.table) {
				const e = sh.entries.find((x) => x.t.name === g.config.table);
				if (!e || !g.src || g.atCol < c0 || g.atCol > c1) continue;
				for (let r = Math.max(r0, e.t.row + 1); r <= Math.min(r1, e.last); r++) {
					const row = e.cache.row(r - e.t.row - 1);
					if (row) one(g, g.src.map((i) => sparkValue(row.cells[i])), r, g.atCol);
				}
				continue;
			}
			for (const s of g.cells ?? []) if (s.row >= r0 && s.row <= r1 && s.col >= c0 && s.col <= c1) one(g, s.values ?? [], s.row, s.col);
		}
	}

	/** Groupe sparkline dessiné en (r, c), et ses valeurs (lignes de tableau chargées). */
	function sparkAt(r: number, c: number): { g: SparklineGroup; values: (number | null)[] } | null {
		const f = freeSparkAt(sh.sparkGroups, r, c);
		if (f) return f;
		for (const g of sh.sparkGroups) {
			if (!g.config.table || g.atCol !== c || !g.src) continue;
			const e = sh.entries.find((x) => x.t.name === g.config.table);
			if (!e || r <= e.t.row || r > e.last) continue;
			const row = e.cache.row(r - e.t.row - 1);
			return { g, values: row ? g.src.map((i) => sparkValue(row.cells[i])) : [] };
		}
		return null;
	}

	/** Cellules d'un volet (dans sa découpe) ; vrai s'il manque des lignes pas encore chargées. */
	function drawCells(L: Layer): boolean {
		const { ctx, C, rh, cax, rspan, styleAt, topRow, tableAt, mergeOf, borders, key, MARGIN } = sh;
		const { w, h, r0, r1, c0, c1, cx, ry, xEnd, spanW, visible, s0, s1, k0, k1 } = L;
		sh.paneTop = L.top;
		let incomplete = false;
		for (const e of visible) {
			const d = sh.visibleData(e, r0, r1);
			if (d) {
				e.cache.want(d.first, d.last);
				if (!e.cache.has(d.first, d.last)) incomplete = true;
			}
		}

		// En-têtes (agrandis à l'écran quand la cellule active les prolonge).
		for (const e of visible) {
			const g = sh.grown(e);
			const moved = sh.enterTable(e, cx, ry);
			ctx.fillStyle = e.t.query ? C.linkHead : C.head;
			// En mouvement : l'en-tête suit le cadre animé (il se rétracte avec lui).
			ctx.fillRect(cx(g.firstCol) + (moved ? sh.tableAnim(e)!.ox : 0), ry(e.t.row), moved ? sh.tableAnim(e)!.w : spanW(g.firstCol, g.lastCol), rh(e.t.row));
			// Colonnes saisies à côté du résultat chargé : en-tête d'un tableau ordinaire (vert).
			if (e.t.query && !moved) {
				ctx.fillStyle = C.head;
				e.t.columns.forEach((k, i) => {
					if (k.hand) ctx.fillRect(cx(e.t.col + i), ry(e.t.row), cax().size(e.t.col + i), rh(e.t.row));
				});
			}
			if (moved) ctx.restore();
		}

		// Plage sélectionnée.
		// Comme Excel, la cellule active (ou sa fusion) reste blanche.
		// Fusion active à cheval sur des lignes entières sélectionnées : seule sa
		// partie sélectionnée est réservée (le reste n'est pas à surligner).
		if (!sh.singleSelection()) {
			const a = intersect(sh.activeRect(), { r0: s0, r1: s1, c0: k0, c1: k1 });
			ctx.fillStyle = C.select;
			ctx.beginPath();
			ctx.rect(cx(k0), ry(s0), spanW(k0, k1), rspan(s0, s1));
			if (a) ctx.rect(cx(a.c0), ry(a.r0), spanW(a.c0, a.c1), rspan(a.r0, a.r1));
			ctx.fill('evenodd');
		}

		// Recherche : toutes les occurrences visibles, en léger (l'active reste encadrée).
		if (sh.finder && sh.matches.length && sh.matchKey === sh.matchesKey(sh.finder.text, sh.finder.zone, sh.finder.opts)) {
			ctx.fillStyle = C.found;
			const here = sh.data.sheet ?? 0;
			for (const m of sh.matches)
				if (m.sheet === here && m.row >= r0 && m.row <= r1 && m.col >= c0 && m.col <= c1) ctx.fillRect(cx(m.col), ry(m.row), cax().size(m.col), rh(m.row));
		}
		// Cellule trouvée : sa ligne et sa colonne entières (toute la fusion), en
		// croix sous le contenu ; les deux bandes se superposent sur la cellule.
		if (sh.findCross) {
			const f = sh.findCross;
			ctx.fillStyle = C.findCross;
			if (f.r1 >= r0 && f.r0 <= r1) ctx.fillRect(cx(c0), ry(f.r0), xEnd - cx(c0), rspan(f.r0, f.r1));
			if (f.c1 >= c0 && f.c0 <= c1) ctx.fillRect(cx(f.c0), ry(r0), spanW(f.c0, f.c1), ry(r1 + 1) - ry(r0));
		}

		// Lignes masquées par un filtre : elles gardent leur place (les cellules
		// libres dessous ne bougent pas) ; une bande grisée les représente, avec
		// son libellé sur sa première ligne visible.
		for (const e of sh.entries) {
			const b = hiddenBand(e.t);
			if (!b || b.last < r0 || b.first > r1 || e.lastCol < c0 || e.t.col > c1) continue;
			const top = Math.max(b.first, r0);
			const bottom = Math.min(b.last, r1);
			ctx.fillStyle = C.gray;
			ctx.fillRect(cx(e.t.col), ry(top), spanW(e.t.col, e.lastCol), rspan(top, bottom));
			ctx.save();
			ctx.beginPath();
			ctx.rect(cx(e.t.col), ry(top), spanW(e.t.col, e.lastCol), rspan(top, bottom));
			ctx.clip();
			ctx.font = 'italic ' + FONT_SMALL;
			ctx.fillStyle = C.icon;
			ctx.textAlign = 'left';
			ctx.textBaseline = 'middle';
			ctx.fillText(bandLabel(e.t.hidden ?? 0, bandOpen(e.t) ? sh.data?.choice?.name : undefined), cx(e.t.col) + 6, ry(top) + rh(top) / 2);
			ctx.restore();
		}

		// Contenu des tableaux, colonne par colonne (une découpe par colonne).
		ctx.textBaseline = 'middle';
		for (const e of visible) {
			const tr0 = Math.max(r0, e.t.row);
			const tr1 = Math.min(r1, e.last);
			const mv = sh.tableAnim(e);
			const moved = sh.enterTable(e, cx, ry);
			// Colonne calculée : ses valeurs qui changent s'éclairent un instant.
			const calcCols = sh.motion() ? e.t.columns.map((k) => !!k.formula || k.type === 'formula') : null;
			for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol) && tr0 <= tr1; c++) {
				const i = c - e.t.col;
				const col = e.t.columns[i];
				const x = cx(c) + (mv ? mv.colOff(c) : 0);
				const cw = cax().size(c);
				if (!cw) continue; // colonne masquée
				ctx.save();
				ctx.beginPath();
				ctx.rect(x, TOP_H, cw, h - TOP_H);
				ctx.clip();
				for (let r = tr0; r <= tr1; r++) {
					const y = ry(r) + (mv ? mv.rowOff(r) : 0);
					if (!rh(r)) continue; // ligne masquée
					if (r === e.t.row) {
						// Colonne sur un niveau : une cellule sur la bande et l'en-tête ;
						// ses bordures et sa note sont celles de la cellule entière (singleCell).
						const single = sh.singleCol(e, i) && !!rh(r - 1);
						const hs = single ? sh.bandStyle(e, i, i) : styleAt(r, c, e);
						if (hs && !single) borders(hs, x, y, cw, rh(r));
						if (hs?.bg) {
							ctx.fillStyle = themed(hs.bg);
							ctx.fillRect(x, y, cw, rh(r));
						}
						if (hs?.nt && !single) noteCorner(x + cw, y, hs);
						const iconW = headIcons(e, i, x, y, cw, rh(r));
						noteHeadIcons(e, i, iconW, hs);
						// Colonne sur un niveau : son nom s'écrit sur la bande et l'en-tête
						// réunis (drawGroupBands, qui passe après le quadrillage).
						if (single) {
							sh.singleIcons.set(`${e.t.id}:${i}`, iconW);
							continue;
						}
						// Nom donné par Lattis (« Colonne n ») : gris italique, à remplacer.
						const gen = isGeneratedColumnName(col.name);
						headLabel(headerLabel(col.name, e.t.twoLevel), x, y, cw, rh(r), iconW + 10, gen ? { ...hs, i: true } : hs, gen ? C.unnamedBadge : e.t.query && !col.hand ? C.linkHeadText : C.headText, 'left');
						continue;
					}
					const row = e.cache.row(r - e.t.row - 1);
					if (!row) {
						ctx.fillStyle = C.placeholder;
						ctx.fillRect(x + 6, y + 7, cw * 0.6, rh(r) - 14);
						continue;
					}
					const v = row.cells[i];
					const bad = row.bad?.has(i) || (col.type === 'formula' && isErrorValue(v));
					const own = sh.colRules.get(e.t.id)?.[i];
					const cf = sh.cfAt(r, c, { v, text: row.text[i] ?? '', error: !!bad }, own ? { rules: own, loc: { k: 'tbl', t: e.t.id, c: i }, mark: (_, k) => !!row.marks?.has(i * 1000 + k) } : undefined);
					const cs = resolveStyle(styleAt(r, c, e), cf?.style);
					if (cs?.bg) {
						ctx.fillStyle = themed(cs.bg);
						ctx.fillRect(x, y, cw, rh(r));
					}
					if (r === sh.hoverLocked && row.locked) {
						ctx.fillStyle = 'rgba(107, 114, 128, 0.12)';
						ctx.fillRect(x, y, cw, rh(r));
					}
					if (cf?.bar) drawBar(cf.bar, x, y, cw, rh(r), `t${e.t.id}:${row.id}:${col.name}`, r, c);
					if (cs) borders(cs, x, y, cw, rh(r));
					if (cs?.nt) noteCorner(x + cw, y, cs);
					const sk = syncAt(row.sync, i);
					if (sk) syncCorner(x + cw, y + rh(r), sk);
					let s = row.text[i];
					if (calcCols?.[i]) {
						const fk = `t${e.t.id}:${row.id}:${col.name}`;
						if (sh.motion()) sh.columnFlashes(e.t.id, col.name, fk);
						sh.paintFlash(sh.flashOf(fk, s ?? '', r, c), x, y, cw, rh(r));
					}
					// Case NULL d'une colonne saisie : tiret gris centré (dessiné seulement).
					if (showsNull(col.type, v, row.bad?.has(i))) {
						drawText(NULL_MARK, x, y, cw, 'center', undefined, C.nullMark);
						continue;
					}
					if (!s) continue;
					// Jeu d'icônes : à gauche de la cellule ; barre ou icône seule : valeur masquée.
					const iconW = cf?.icon && !row.bad?.has(i) ? drawIconAt(cf.icon, x, y, rh(r)) : 0;
					if (cf?.hide && !row.bad?.has(i)) continue;
					let ds = cs;
					if (cs && !row.bad?.has(i)) [s, ds] = codeShown(s, row.cells[i], cs);
					if (row.bad?.has(i)) {
						// Saisie hors type : valeur brute en rouge, coin rouge comme le
						// triangle d'Excel.
						ctx.font = FONT;
						ctx.textAlign = 'left';
						ctx.fillStyle = C.error;
						ctx.fillText(s, x + 6, y + rh(r) / 2);
						ctx.beginPath();
						ctx.moveTo(x + cw - 7, y + 1);
						ctx.lineTo(x + cw - 1, y + 1);
						ctx.lineTo(x + cw - 1, y + 7);
						ctx.closePath();
						ctx.fill();
						continue;
					}
					const align = cs?.al ?? (col.type === 'formula' ? valueAlignment(v) : alignment(col.type));
					drawText(s, x + iconW, y, cw - iconW, align, ds, col.type === 'formula' && isErrorValue(v) ? C.error : C.text);
					if (col.type === 'formula' && isErrorValue(v)) errorCorner(x, y);
				}
				ctx.restore();
			}
			if (moved) ctx.restore();
		}

		// Mise en forme hors tableaux (cellules libres, tableaux croisés, cellules
		// vides des lignes et colonnes mises en forme) : remplissage et bordures.
		ctx.font = FONT;
		ctx.fillStyle = C.text;
		// Mise en forme conditionnelle des cellules libres, calculée une fois par dessin.
		const frameCf = new Map<string, CfOut | undefined>();
		const cfOf = (r: number, c: number) => {
			const k = key(r, c);
			if (!frameCf.has(k)) frameCf.set(k, sh.freeCf(r, c));
			return frameCf.get(k);
		};
		const paint = (r: number, c: number) => {
			if (tableAt(r, c) || sh.groupBandAt(r, c) || !rh(r) || !cax().size(c) || mergeOf(r, c)) return;
			const cf = cfOf(r, c);
			const fs = resolveStyle(styleAt(r, c, undefined), cf?.style);
			if (!fs && !cf?.bar) return;
			if (fs?.bg) {
				ctx.fillStyle = themed(fs.bg);
				ctx.fillRect(cx(c), ry(r), cax().size(c), rh(r));
			}
			if (cf?.bar) drawBar(cf.bar, cx(c), ry(r), cax().size(c), rh(r), `f${r},${c}`, r, c);
			if (!fs) return;
			borders(fs, cx(c), ry(r), cax().size(c), rh(r));
			if (fs.nt) noteCorner(cx(c + 1), ry(r), fs);
		};
		// Lignes ou colonnes mises en forme (hors masquage seul) : toutes les cellules visibles.
		const lineFormats = [...sh.sheetRowStyles.values(), ...sh.sheetColStyles.values()].some((s) => Object.keys(s).some((k) => k !== 'hd'));
		if (lineFormats || sh.sheetRowRules.size || sh.sheetColRules.size) {
			for (let r = r0; r <= r1; r++) for (let c = c0; c <= c1; c++) paint(r, c);
		} else
			for (const k of sh.freeStyles.keys()) {
				const [r, c] = k.split(',').map(Number);
				if (r >= r0 && r <= r1 && c >= c0 && c <= c1) paint(r, c);
			}
		drawSparklines(L);
		// Valeurs déversées : comme des cellules, dans un cadre bleu.
		for (const sp of sh.spills) {
			const sr1 = sp.row + sp.rows.length - 1;
			const sc1 = sp.col + sp.rows[0].length - 1;
			if (sr1 < r0 || sp.row > r1 || sc1 < c0 || sp.col > c1) continue;
			for (let i = Math.max(0, r0 - sp.row); i < sp.rows.length && sp.row + i <= r1; i++)
				sp.rows[i].forEach((v, j) => {
					if ((i === 0 && j === 0) || !v) return;
					const c = sp.col + j;
					const st = styleAt(sp.row + i, c, undefined);
					sh.paintFlash(sh.flashOf(`s${sp.row},${sp.col}:${i},${j}`, v, sp.row + i, c), cx(c), ry(sp.row + i), cax().size(c), rh(sp.row + i));
					drawText(formatCell(v, undefined, st), cx(c), ry(sp.row + i), cax().size(c), st?.al ?? freeDisplay('', v).align, st, isErrorValue(v) ? C.error : C.text);
				});
			ctx.save();
			ctx.strokeStyle = C.spill;
			ctx.lineWidth = 1;
			ctx.strokeRect(Math.round(cx(sp.col)) + 0.5, Math.round(ry(sp.row)) + 0.5, spanW(sp.col, sc1) - 1, rspan(sp.row, sp.row + sp.rows.length - 1) - 1);
			ctx.restore();
		}
		// Tableaux croisés : en-tête et total en gras, dans un cadre.
		for (const p of sh.pivots) {
			if (p.pending) {
				// Grand tableau source : calculé après l'affichage de la feuille.
				drawText(tr('grid.pivot.computing'), cx(p.col), ry(p.row), cax().size(p.col), 'left', undefined, C.stripText, w);
				continue;
			}
			if (p.error) {
				drawText(tr('grid.pivot.error', { error: p.error }), cx(p.col), ry(p.row), cax().size(p.col), 'left', undefined, C.error, w);
				continue;
			}
			const pr1 = p.row + p.cells.length - 1;
			const pc1 = p.col + p.cells[0].length - 1;
			// Filtres de rapport au-dessus du tableau (Excel) : champ, valeur retenue, ▾.
			(p.config.filters ?? []).forEach((f, k) => {
				const r = (p.anchor ?? p.row) + k;
				if (r < r0 || r > r1 || p.col + 1 < c0 || p.col > c1) return;
				const w0 = cax().size(p.col);
				const w1 = cax().size(p.col + 1);
				ctx.fillStyle = C.linkHead; // comme les tableaux d'une requête : des données calculées
				ctx.fillRect(cx(p.col), ry(r), w0, rh(r));
				drawText(f.field, cx(p.col), ry(r), w0, 'left', { b: true }, C.text);
				drawText(sh.pivotFilterText(f.values), cx(p.col + 1), ry(r), w1 - sh.PIVOT_DROP, 'left', undefined, C.text);
				ctx.save();
				ctx.strokeStyle = C.linkBorder;
				ctx.lineWidth = 1;
				ctx.strokeRect(Math.round(cx(p.col)) + 0.5, Math.round(ry(r)) + 0.5, w0 + w1 - 1, rh(r) - 1);
				ctx.restore();
				pivotDrop(cx(p.col + 1) + w1, ry(r), rh(r), f.values.length > 0);
			});
			if (pr1 < r0 || pivotHead(p) > r1 || pc1 < c0 || p.col > c1) continue;
			// En-têtes imbriqués (plusieurs champs en colonnes), au-dessus de l'en-tête.
			drawPivotColHead(p, pc1, r0, r1, { cx, ry, spanW });
			// Ligne du total général : la dernière, quand il est affiché.
			const tot = pivotTotalRow(p);
			// En-tête et total sur fond gris, sauf remplissage choisi.
			// Colonnes saisies à droite (pivothand.ts) : en-tête vert, comme un tableau ordinaire.
			const hs = handStart(p);
			for (const r of tot >= 0 ? [p.row, tot] : [p.row])
				for (let c = p.col; c <= pc1; c++)
					if (!styleAt(r, c, undefined)?.bg) {
						ctx.fillStyle = r === p.row && c >= hs ? C.head : C.linkHead; // comme les tableaux d'une requête : des données calculées
						ctx.fillRect(cx(c), ry(r), cax().size(c), rh(r));
					}
			for (let i = Math.max(0, r0 - p.row); i < p.cells.length && p.row + i <= r1; i++)
				p.cells[i].forEach((v, j) => {
					const c = p.col + j;
					// Arborescence : retrait du niveau et case −/+ des groupes (colonne des étiquettes).
					const info = j === 0 ? pivotRowOf(p, p.row + i) : undefined;
					if (info?.group) pivotFoldBox(cx(c) + pivotBoxLeft(info.depth), ry(p.row + i), rh(p.row + i), !!info.collapsed);
					// Groupe de colonnes plié : case + devant son étiquette.
					const folded = i === 0 && !!p.cols?.[j]?.collapsed;
					if (folded) pivotFoldBox(cx(c) + pivotBoxLeft(0), ry(p.row), rh(p.row), true);
					if (!v) return;
					const bold = pivotBold(p, p.row + i);
					// Mise en forme posée sur les cellules du TCD, leurs lignes ou colonnes (format de nombre, décimales, police…).
					const st = resolveStyle(styleAt(p.row + i, c, undefined), sh.freeCf(p.row + i, c)?.style);
					const style = bold ? { ...st, b: st?.b ?? true } : st;
					const ind = pivotLabelOffset(info) + (folded ? PIVOT_BOX + 5 : 0);
					// Recalculé : seules les cellules dont la valeur change s'éclairent.
					sh.paintFlash(sh.flashOf(`p${p.id}:${i},${j}`, v, p.row + i, c), cx(c), ry(p.row + i), cax().size(c), rh(p.row + i));
					drawText(pivotValueText(formatCell(v, undefined, st), i, j), cx(c) + ind, ry(p.row + i), cax().size(c) - ind, st?.al ?? (c >= hs && i > 0 ? handAlign(v) : j > 0 && i > 0 ? 'right' : 'left'), style, i === 0 && c >= hs ? C.headText : i === 0 || p.row + i === tot ? C.linkHeadText : C.text);
				});
			// ▾ des étiquettes des lignes : trier et filtrer le champ (Excel) ; en
			// arborescence, un seul ▾ sur « Étiquettes de lignes » (choix du champ dans le menu).
			if (p.row >= r0 && p.row <= r1)
				(p.rows ? (p.config.rowFields ?? []).slice(0, 1) : (p.config.rowFields ?? [])).forEach((f, j) => {
					const c = p.col + j;
					const on = p.rows ? (p.config.rowFields ?? []).some((x) => sh.pivotFiltered(p, x)) : sh.pivotFiltered(p, f);
					if (c >= c0 && c <= c1) pivotDrop(cx(c) + cax().size(c), ry(p.row), rh(p.row), on);
				});
			ctx.save();
			ctx.strokeStyle = C.linkBorder;
			ctx.lineWidth = 1;
			ctx.strokeRect(Math.round(cx(p.col)) + 0.5, Math.round(ry(pivotHead(p))) + 0.5, spanW(p.col, pc1) - 1, rspan(pivotHead(p), pr1) - 1);
			// Traits sous l'en-tête et au-dessus du total.
			ctx.beginPath();
			for (const y of tot >= 0 ? [ry(p.row + 1), ry(tot)] : [ry(p.row + 1)]) {
				ctx.moveTo(Math.round(cx(p.col)), Math.round(y) + 0.5);
				ctx.lineTo(Math.round(cx(p.col)) + spanW(p.col, pc1), Math.round(y) + 0.5);
			}
			// Trait entre le TCD calculé et ses colonnes saisies.
			if (hs > p.col && hs <= pc1) {
				ctx.moveTo(Math.round(cx(hs)) + 0.5, Math.round(ry(p.row)));
				ctx.lineTo(Math.round(cx(hs)) + 0.5, Math.round(ry(p.row)) + rspan(p.row, pr1));
			}
			ctx.stroke();
			ctx.restore();
			// Pastille du nom (TCD1), comme celle d'un tableau : un TCD se lit
			// dans les formules comme un tableau nommé (TCD1[Nombre]). Toujours
			// montrée : l'onglet « TCD 1 » ne porte pas son nom. Un clic ouvre le volet.
			if (sh.curPane && p.config.name) {
				ctx.font = FONT_SMALL;
				const bh = 13;
				const bx = Math.round(cx(p.col)) - 0.5;
				const by = Math.round(ry(pivotHead(p))) - 0.5;
				let aboveFree = true;
				for (let r = pivotHead(p) - 1; aboveFree && r >= 0 && ry(r + 1) > by - bh; r--) {
					if (!rh(r)) continue;
					for (let c = p.col; c <= pc1; c++) {
						const v = sh.cells.get(key(r, c));
						if ((v !== undefined && v !== '') || tableAt(r, c) || (sh.pivotAt(r, c) && sh.pivotAt(r, c) !== p) || mergeOf(r, c)) {
							aboveFree = false;
							break;
						}
					}
				}
				const fw = spanW(p.col, pc1);
				const label = sh.fitText(p.config.name, fw - 10);
				const bw = Math.ceil(ctx.measureText(label).width) + 10;
				const pp = sh.curPane;
				if (fw >= 30) {
					const b = placeFollowBadge({ x: bx, y: by, w: fw, bw, bh, paneTop: pp.y0, strip: pp.y0 === TOP_H, aboveFree, left: true });
					sh.pivotBadges.push({ p, label, radii: b.radii, x0: b.x, x1: b.x + bw, y0: b.y, y1: b.y + bh, clip: { x0: pp.x0, x1: pp.x1, y0: pp.y0 === TOP_H ? 0 : pp.y0, y1: pp.y1 } });
				}
			}
		}
		// Texte aligné à gauche qui déborde (comme Excel) : seulement sur les
		// cellules voisines vides ; il est coupé à la première cellule occupée
		// (valeur, tableau, tableau croisé, fusion).
		const overflowW = (r: number, c: number): number => {
			let ow = cax().size(c);
			for (let cc = c + 1; cc <= c1 + 1; cc++) {
				const v = sh.cells.get(key(r, cc));
				if ((v !== undefined && v !== '') || tableAt(r, cc) || sh.pivotAt(r, cc) || mergeOf(r, cc) || sh.spilledAt(r, cc)) break;
				ow += cax().size(cc);
			}
			return ow;
		};
		for (const [k, rawText] of sh.cells) {
			const [r, c] = k.split(',').map(Number);
			if (r < r0 || r > r1 || c < c0 || c > c1 || !rh(r) || !cax().size(c) || mergeOf(r, c)) continue;
			// Cellule en cours d'édition : l'éditeur montre sa saisie ; sa valeur,
			// qui peut déborder plus loin que lui, ne se dessine pas dessous.
			// Même chose pour une saisie dans la barre de formule, montrée dans la cellule.
			if ((sh.editing || (sh.fxEditing && !sh.hold)) && r === sh.active.row && c === sh.active.col) continue;
			const cf = cfOf(r, c);
			const fstyle = resolveStyle(styleAt(r, c, undefined), cf?.style);
			let x = cx(c);
			let cw = cax().size(c);
			const f = sh.shown.get(k);
			if (f) sh.paintFlash(sh.flashOf('f' + k, f.text, r, c), x, ry(r), cw, rh(r));
			if (f?.error) errorCorner(x, ry(r));
			// Jeu d'icônes à gauche ; barre ou icône seule : valeur masquée.
			if (cf?.icon) {
				const iw = drawIconAt(cf.icon, x, ry(r), rh(r));
				x += iw;
				cw -= iw;
			}
			if (cf?.hide) continue;
			// Nombres, dates et heures à droite, booléens centrés, apostrophe de tête masquée ;
			// format de la cellule (nombre, date, heure, texte : numfmt.ts).
			const fd = freeDisplay(rawText, f?.text);
			const [raw, dstyle] = fd.kind === 'text' ? [fd.text, fstyle] : codeShown(fd.text, undefined, fstyle);
			const boxed = fd.align !== 'left';
			const spread = boxed ? cw : overflowW(r, c) - (x - cx(c));
			if (fstyle || raw.includes('\n')) {
				drawText(raw, x, ry(r), cw, fstyle?.al ?? fd.align, dstyle, f?.error ? C.error : C.text, spread);
				continue;
			}
			ctx.fillStyle = f?.error ? C.error : C.text;
			ctx.save();
			ctx.beginPath();
			ctx.rect(Math.max(x, MARGIN), TOP_H, spread - Math.max(0, MARGIN - x), h - TOP_H);
			ctx.clip();
			ctx.textAlign = fd.align;
			ctx.fillText(raw, fd.align === 'right' ? x + cw - 6 : fd.align === 'center' ? x + cw / 2 : x + 6, ry(r) + rh(r) / 2);
			ctx.restore();
		}

		// Quadrillage de toute la feuille ; masqué, il ne reste qu'à
		// l'intérieur des tableaux.
		ctx.save();
		if (!sh.gridlines) {
			ctx.beginPath();
			for (const e of visible) {
				const g = sh.grown(e);
				ctx.rect(cx(g.firstCol), ry(topRow(e.t)), spanW(g.firstCol, g.lastCol), rspan(topRow(e.t), g.last));
			}
			ctx.clip();
		} else if (sh.pivots.length) {
			// Comme Excel : pas de quadrillage dans un tableau croisé (seulement
			// son cadre et les traits sous l'en-tête et au-dessus du total).
			ctx.beginPath();
			ctx.rect(0, 0, w, h);
			for (const p of sh.pivots) {
				if (p.error) continue;
				const pc1 = p.col + p.cells[0].length - 1;
				ctx.rect(cx(p.col) + 1, ry(pivotHead(p)) + 1, spanW(p.col, pc1) - 2, rspan(pivotHead(p), p.row + p.cells.length - 1) - 2);
			}
			ctx.clip('evenodd');
		}
		// Pas de quadrillage dans les en-têtes des tableaux (bande des groupes
		// comprise) : leurs bordures sont celles du cadre (headerLines), sans
		// liseré pâle à côté quand le zoom ou l'écran tombent entre deux pixels.
		const heads = visible.filter((e) => !sh.tableAnim(e));
		if (heads.length) {
			ctx.beginPath();
			ctx.rect(0, 0, w, h);
			for (const e of heads) ctx.rect(cx(e.t.col), ry(topRow(e.t)), spanW(e.t.col, e.lastCol), ry(e.t.row + 1) - ry(topRow(e.t)));
			ctx.clip('evenodd');
		}
		ctx.beginPath();
		ctx.strokeStyle = C.grid;
		ctx.lineWidth = 1;
		for (let r = r0; r <= r1 + 1; r++) {
			const y = Math.round(ry(r)) - 0.5;
			ctx.moveTo(MARGIN, y);
			ctx.lineTo(xEnd, y);
		}
		for (let c = c0; c <= c1 + 1; c++) {
			const x = Math.round(cx(c)) - 0.5;
			ctx.moveTo(x, TOP_H);
			ctx.lineTo(x, h);
		}
		ctx.stroke();
		ctx.restore();
		drawMerges(L);
		drawGroupBands(L);

		ctx.save();
		ctx.beginPath();
		ctx.rect(MARGIN, TOP_H, w - MARGIN, h - TOP_H);
		ctx.clip();
		// Bordure discrète de chaque tableau, trait sous l'en-tête, bordures des en-têtes.
		for (const e of visible) {
			const g = sh.grown(e);
			const a = sh.tableAnim(e);
			const to = sh.tableGeo(e);
			const fw = a ? a.w : to.w;
			const fh = a ? a.h : to.h;
			if (sh.motion()) sh.geoDrawn.set(e.t.id, { x: to.x + (a?.dx ?? 0) + (a?.ox ?? 0), y: to.y + (a?.dy ?? 0), w: fw, h: fh });
			const x = Math.round(cx(g.firstCol) + (a?.dx ?? 0) + (a?.ox ?? 0)) - 0.5;
			const y = Math.round(ry(topRow(e.t)) + (a?.dy ?? 0)) - 0.5;
			ctx.save();
			if (a) ctx.globalAlpha *= a.alpha;
			// Cadre, trait sous l'en-tête et bordures par défaut des en-têtes
			// (séparations des colonnes, trait sous la bande des groupes) :
			// même couleur et même épaisseur, nets à tout zoom (crispLines).
			const border = e.t.query ? C.linkBorder : C.border;
			const fx = cx(g.firstCol) + (a?.dx ?? 0) + (a?.ox ?? 0);
			const fy = ry(topRow(e.t)) + (a?.dy ?? 0);
			const fu = ry(e.t.row + 1) + (a?.dy ?? 0);
			const head = headerLines(e, L, a);
			const grownOut = g.last !== e.last || g.lastCol !== e.lastCol || g.firstCol !== e.t.col;
			const frame = [fx, fy, fx, fy + fh, fx + fw, fy, fx + fw, fy + fh, fx, fy, fx + fw, fy, fx, fy + fh, fx + fw, fy + fh, fx, fu, fx + fw, fu];
			// Cadre agrandi par la cellule active : en vert foncé, par-dessus les traits des en-têtes.
			ctx.fillStyle = border;
			crispLines(grownOut ? head : [...frame, ...head]);
			if (grownOut) {
				ctx.fillStyle = C.active;
				crispLines(frame);
			}
			// Pastilles du cadre, au-dessus de lui s'il y a la place (bandeau du
			// haut compris pour un tableau en ligne 1), sinon dans son coin
			// (followbadge.ts) ; dessinées après le bandeau, dans la découpe du
			// volet. À gauche, le nom du tableau (cliquer : le renommer) ; à
			// droite, la valeur de l'onglet à choix qu'il suit.
			if (sh.curPane) {
				ctx.font = FONT_SMALL;
				const bh = 13;
				const top = topRow(e.t);
				let aboveFree = true;
				for (let r = top - 1; aboveFree && r >= 0 && ry(r + 1) > y - bh; r--) {
					if (!rh(r)) continue;
					for (let c = e.t.col; c <= e.lastCol; c++) {
						const v = sh.cells.get(key(r, c));
						if ((v !== undefined && v !== '') || tableAt(r, c) || sh.pivotAt(r, c) || mergeOf(r, c)) {
							aboveFree = false;
							break;
						}
					}
				}
				const p = sh.curPane;
				const clip = { x0: p.x0, x1: p.x1, y0: p.y0 === TOP_H ? 0 : p.y0, y1: p.y1 };
				let room = fw;
				if (e.t.follows && sh.data?.choice && !sh.data.choice.all) {
					const label = sh.data.choice.value || tr('grid.follow.empty');
					const bw = Math.ceil(ctx.measureText(label).width) + 10;
					const b = placeFollowBadge({ x, y, w: fw, bw, bh, paneTop: p.y0, strip: p.y0 === TOP_H, aboveFree });
					sh.followBadges.push({ kind: 'follow', e, label, radii: b.radii, x0: b.x, x1: b.x + bw, y0: b.y, y1: b.y + bh, clip });
					room -= bw + 4;
				}
				// Classeur collaboratif : tableau resté sur ce poste (pas dans
				// le classeur partagé) ; un clic propose de l'y ajouter.
				if (e.t.shared === 'private' && sh.onaddshared) {
					const label = tr('grid.local.badge');
					const bw = Math.ceil(ctx.measureText(label).width) + 10;
					if (room >= bw + 30) {
						const b = placeFollowBadge({ x, y, w: room, bw, bh, paneTop: p.y0, strip: p.y0 === TOP_H, aboveFree });
						sh.followBadges.push({ kind: 'local', e, label, radii: b.radii, x0: b.x, x1: b.x + bw, y0: b.y, y1: b.y + bh, clip });
						room -= bw + 4;
					}
				}
				if (e.t.name && room >= 30 && showNameBadge(e.t.name, sh.sheetName)) {
					const dflt = isDefaultTableName(e.t.name);
					const label = sh.fitText(e.t.name + (dflt ? ' ✎' : ''), room - 10);
					const bw = Math.ceil(ctx.measureText(label).width) + 10;
					const b = placeFollowBadge({ x, y, w: fw, bw, bh, paneTop: p.y0, strip: p.y0 === TOP_H, aboveFree, left: true });
					sh.followBadges.push({ kind: dflt ? 'unnamed' : 'name', e, label, radii: b.radii, x0: b.x, x1: b.x + bw, y0: b.y, y1: b.y + bh, clip });
				}
			}
			// Classeur collaboratif : lecture filtrée par les droits (§8.1).
			if (e.t.filtered) {
				ctx.font = 'italic ' + FONT_SMALL;
				ctx.fillStyle = C.icon;
				ctx.textAlign = 'right';
				ctx.textBaseline = 'top';
				ctx.fillText(tr('grid.rights.filtered'), x + fw, y + fh + 2);
			}
			ctx.restore();
		}
		// Bordures posées par le ruban : par-dessus le cadre discret des
		// tableaux (un Contour sur un en-tête ou sa bande reste visible).
		strokeBorders();
		// Sauts de page (Affichage › Sauts de page) : pointillés fins, plus
		// marqués pour un saut manuel, et la zone imprimée encadrée.
		if (sh.pageBreaks) {
			const b = sh.pageBreaks;
			const a = b.area;
			ctx.save();
			ctx.strokeStyle = C.pageBreak;
			for (const [manual, dash, lw] of [
				[false, [3, 3], 1],
				[true, [8, 3], 1.5]
			] as const) {
				ctx.beginPath();
				ctx.setLineDash(dash as unknown as number[]);
				ctx.lineWidth = lw;
				for (const r of manual ? b.manualRows : b.rows)
					if (r >= r0 && r <= r1 + 1 && (manual || !b.manualRows.includes(r))) {
						const y = Math.round(ry(r)) - 0.5;
						ctx.moveTo(Math.max(cx(a.c0), MARGIN), y);
						ctx.lineTo(Math.min(cx(a.c1 + 1), xEnd), y);
					}
				for (const c of manual ? b.manualCols : b.cols)
					if (c >= c0 && c <= c1 + 1 && (manual || !b.manualCols.includes(c))) {
						const x = Math.round(cx(c)) - 0.5;
						ctx.moveTo(x, Math.max(ry(a.r0), TOP_H));
						ctx.lineTo(x, Math.min(ry(a.r1 + 1), h));
					}
				ctx.stroke();
			}
			if (b.framed && a.r1 >= r0 && a.r0 <= r1 + 1 && a.c1 >= c0 && a.c0 <= c1 + 1) {
				ctx.setLineDash([3, 3]);
				ctx.lineWidth = 1;
				ctx.strokeRect(Math.round(cx(a.c0)) - 0.5, Math.round(ry(a.r0)) - 0.5, spanW(a.c0, a.c1), rspan(a.r0, a.r1));
			}
			ctx.restore();
		}
		// Zone copiée ou coupée : cadre pointillé qui défile (« fourmis »), comme Excel.
		if (sh.clipRange && sh.clipRange.sheet === sh.sheetId()) {
			const k = sh.clipRange;
			ctx.save();
			ctx.setLineDash([4, 3]);
			if (sh.motion()) {
				ctx.lineDashOffset = -((sh.animNow / 60) % 7);
				sh.animBusy = true;
			}
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 1.5;
			ctx.strokeRect(cx(k.c0) + 1, ry(k.r0) + 1, spanW(k.c0, k.c1) - 2, rspan(k.r0, k.r1) - 2);
			ctx.restore();
		}
				// Déplacement en cours : contour de la destination.
		if (sh.moving?.to) {
			const { r0, c0, r1, c1 } = sh.moving.sel;
			const dr = sh.moving.to.row - sh.moving.from.row;
			const dc = sh.moving.to.col - sh.moving.from.col;
			ctx.save();
			ctx.setLineDash([5, 3]);
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 2;
			ctx.strokeRect(cx(c0 + dc) + 1, ry(r0 + dr) + 1, spanW(c0 + dc, c1 + dc) - 2, rspan(r0 + dr, r1 + dr) - 2);
			ctx.restore();
		}
		// Pose d'un tableau croisé : contour de l'emplacement sous la souris.
		if (sh.placing && sh.placeHover) {
			const { row, col } = sh.placeHover;
			ctx.save();
			ctx.setLineDash([5, 3]);
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 2;
			ctx.strokeRect(cx(col) + 1, ry(row) + 1, spanW(col, col + sh.placeSize.w - 1) - 2, rspan(row, row + sh.placeSize.h - 1) - 2);
			ctx.restore();
		}
		// Tableau actif qui ne touche pas la gouttière : ses numéros de ligne (ou
		// sa clé) en petit, juste à gauche de sa première colonne, là où la
		// cellule voisine est vide.
		const act = sh.a1 ? undefined : (sh.ownerAt(sh.active.row, sh.active.col) ?? sh.entries.find((e) => sh.grown(e).last !== e.last));
		if (act && besideGutter({ row: act.t.row, last: act.last, col: act.t.col, lastCol: act.lastCol }, c0, c1)) {
			const x = cx(act.t.col);
			const gw = cax().size(act.t.col - 1);
			ctx.save();
			ctx.beginPath();
			ctx.rect(Math.max(MARGIN, x - gw), TOP_H, Math.min(gw, x - MARGIN), h - TOP_H);
			ctx.clip();
			ctx.font = FONT_TINY;
			ctx.fillStyle = C.stripText;
			ctx.textAlign = 'right';
			ctx.textBaseline = 'middle';
			const last = sh.grown(act).last;
			for (let r = Math.max(r0, act.t.row + 1); r <= Math.min(r1, last); r++) {
				if (tableAt(r, act.t.col - 1) || sh.cells.has(key(r, act.t.col - 1))) continue;
				ctx.fillText(rowLabel(act, r), x - 3, ry(r) + rh(r) / 2);
			}
			ctx.restore();
		}
		// Colonne calculée en cours d'édition, ou cible d'une modification
		// de colonne (scopeTints) : toute sa portée teintée.
		for (const t of sh.scopeTints) {
			const a = Math.max(t.r0, r0);
			const b = Math.min(t.r1, r1);
			if (a <= b && t.c0 <= t.c1) {
				ctx.fillStyle = C.calc;
				ctx.fillRect(cx(t.c0), ry(a), spanW(t.c0, t.c1), rspan(a, b));
			}
		}
		// Plage source d'une validation en cours de pointage : toute la colonne
		// (ou la plage) teintée et encadrée, comme une colonne calculée en cours
		// d'édition.
		if (sh.dvPick && sh.dvPickRef) {
			const k = refRect(sh.dvPickRef, sh.refContext().tables, sh.sheetName);
			if (k && k.r1 >= r0 - 1 && k.r0 <= r1 + 1) {
				const a = Math.max(k.r0, r0);
				const b = Math.min(k.r1, r1);
				ctx.fillStyle = C.calc;
				ctx.fillRect(cx(k.c0), ry(a), spanW(k.c0, k.c1), rspan(a, b));
			}
		}
		// Saisie d'une formule : chaque référence encadrée de sa couleur, comme
		// dans l'éditeur (Excel).
		if (sh.formulaMode) {
			const { tables, at } = sh.refContext();
			ctx.save();
			ctx.lineWidth = 2;
			// Référence glissée : son cadre coloré suit la souris, la zone
			// d'origine garde le même cadre en gris (choix de Louis).
			const box = (k: { r0: number; r1: number; c0: number; c1: number }, color: string) => {
				if (k.r1 < r0 - 1 || k.r0 > r1 + 1) return;
				ctx.strokeStyle = color;
				ctx.fillStyle = color + '14';
				const x = cx(k.c0) + 1;
				const y = ry(k.r0) + 1;
				ctx.fillRect(x, y, spanW(k.c0, k.c1) - 2, rspan(k.r0, k.r1) - 2);
				ctx.strokeRect(x, y, spanW(k.c0, k.c1) - 2, rspan(k.r0, k.r1) - 2);
			};
			for (const f of formulaRefs(sh.editing ? sh.editValueText : sh.fxText)) {
				const k = refRect(f.text, tables, sh.sheetName, at, !!sh.hold);
				if (!k) continue;
				if (sh.refMove && f.start === sh.refMove.start) {
					const dr = sh.refMove.to.row - sh.refMove.from.row;
					const dc = sh.refMove.to.col - sh.refMove.from.col;
					const moved = { r0: k.r0 + dr, r1: k.r1 + dr, c0: k.c0 + dc, c1: k.c1 + dc };
					if (dr || dc) box(k, '#9ca3af');
					if (moved.r0 >= 0 && moved.c0 >= 0) box(moved, f.color);
					continue;
				}
				box(k, f.color);
			}
			ctx.restore();
		}
		// Cellule active (toute la fusion quand elle en est une), et la flèche de
		// sa liste déroulante (validation).
		// Plage de plusieurs cellules : cadre épais autour d'elle, trait fin
		// autour de la cellule active (Excel).
		const ar = sh.activeRect();
		const many = !sh.singleSelection();
		// Cadre extérieur (animé) : la plage, ou la cellule active seule.
		ctx.strokeStyle = C.active;
		ctx.lineWidth = 2;
		// Feuille protégée aussi : on y sélectionne pour lire, copier ou figer les volets.
		ctx.strokeRect(cx(0) + sh.selBox.x + 0.5, ry(0) + sh.selBox.y + 0.5, sh.selBox.w - 1, sh.selBox.h - 1);
		// Rechercher dans la sélection : la cellule trouvée, encadrée (la sélection ne bouge pas).
		if (sh.finder?.zone && sh.finder.at && inZone(sh.finder.at, sh.finder.zone)) {
			const f = sh.finder.at;
			ctx.lineWidth = 2;
			ctx.strokeRect(cx(f.col) + 2, ry(f.row) + 2, cax().size(f.col) - 4, rh(f.row) - 4);
		}
		if (!sh.locked && ar.r1 >= r0 - 1 && ar.r0 <= r1 + 1) {
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 1;
			if (many) ctx.strokeRect(cx(ar.c0) + 1.5, ry(ar.r0) + 1.5, spanW(ar.c0, ar.c1) - 3, rspan(ar.r0, ar.r1) - 3);
			if (!sh.editing && sh.listOf(sh.active.row, sh.active.col)) {
				const b = sh.dropButton();
				ctx.fillStyle = C.strip;
				ctx.fillRect(b.x, b.y, b.w, b.h);
				ctx.strokeStyle = C.stripLine;
				ctx.lineWidth = 1;
				ctx.strokeRect(b.x + 0.5, b.y + 0.5, b.w - 1, b.h - 1);
				ctx.fillStyle = C.stripText;
				ctx.beginPath();
				ctx.moveTo(b.x + 5, b.y + b.h / 2 - 2);
				ctx.lineTo(b.x + b.w - 5, b.y + b.h / 2 - 2);
				ctx.lineTo(b.x + b.w / 2, b.y + b.h / 2 + 3);
				ctx.closePath();
				ctx.fill();
				// Colonne de référence : petit bouton ↗ « Basculer sur » (aussi Ctrl+clic).
				if (!many && sh.refAt(sh.active.row, sh.active.col)) {
					const l = sh.linkButton();
					ctx.fillStyle = C.strip;
					ctx.fillRect(l.x, l.y, l.w, l.h);
					ctx.strokeStyle = C.stripLine;
					ctx.strokeRect(l.x + 0.5, l.y + 0.5, l.w - 1, l.h - 1);
					ctx.save();
					ctx.font = FONT_SMALL;
					ctx.textAlign = 'center';
					ctx.fillStyle = C.stripText;
					ctx.fillText('↗', l.x + l.w / 2, l.y + l.h / 2 + 1);
					ctx.restore();
				}
			}
			sh.tableHintBox = null;
			if (!sh.editing && !many && sh.asTableHint()) {
				// Liste ou bloc resté en cellules libres : petite icône à côté de la
				// cellule, un clic en fait un tableau (comme la plage convertie).
				const b = sh.dropButton();
				ctx.fillStyle = C.strip;
				ctx.fillRect(b.x, b.y, b.w + 2, b.h);
				ctx.strokeStyle = C.stripLine;
				ctx.lineWidth = 1;
				ctx.strokeRect(b.x + 0.5, b.y + 0.5, b.w + 1, b.h - 1);
				const s = Math.min(14, b.h - 2);
				// Petit tableau dessiné à la main (l'icône « table » a un <rect>,
				// que drawIcon refuse) : cadre, ligne d'en-tête pleine, quadrillage.
				const ix = Math.round(b.x + (b.w + 2 - s) / 2) + 0.5;
				const iy = Math.round(b.y + (b.h - s) / 2) + 0.5;
				ctx.fillStyle = C.icon;
				ctx.fillRect(ix, iy, s - 1, Math.round(s / 3));
				ctx.strokeStyle = C.icon;
				ctx.strokeRect(ix, iy, s - 1, s - 1);
				ctx.beginPath();
				ctx.moveTo(ix, iy + Math.round((2 * s) / 3));
				ctx.lineTo(ix + s - 1, iy + Math.round((2 * s) / 3));
				ctx.moveTo(ix + Math.round(s / 2), iy);
				ctx.lineTo(ix + Math.round(s / 2), iy + s - 1);
				ctx.stroke();
				sh.tableHintBox = { x: b.x, y: b.y, w: b.w + 2, h: b.h };
			}
		}
		// Flèches d'audit : bleues vers la cellule (antécédents), rouges depuis
		// elle (dépendants), comme Excel.
		// Une même flèche (antécédent de l'une, dépendant de l'autre, toutes deux
		// sélectionnées) n'est dessinée qu'une fois.
		const drawn = new Set<string>();
		const geo: ArrowGeometry = { x: cx, w: (c) => cax().size(c), y: ry, h: rspan };
		for (const arrows of sh.links) {
			const me = spanPoint({ row: arrows.row, rows: 1, col: arrows.col }, geo);
			const draw = (from: { x: number; y: number }, to: { x: number; y: number }, color: string) => {
				ctx.save();
				ctx.strokeStyle = color;
				ctx.fillStyle = color;
				ctx.lineWidth = 1.5;
				ctx.beginPath();
				ctx.arc(from.x, from.y, 3, 0, Math.PI * 2);
				ctx.fill();
				ctx.beginPath();
				ctx.moveTo(from.x, from.y);
				ctx.lineTo(to.x, to.y);
				ctx.stroke();
				const a = Math.atan2(to.y - from.y, to.x - from.x);
				ctx.beginPath();
				ctx.moveTo(to.x, to.y);
				ctx.lineTo(to.x - 9 * Math.cos(a - 0.4), to.y - 9 * Math.sin(a - 0.4));
				ctx.lineTo(to.x - 9 * Math.cos(a + 0.4), to.y - 9 * Math.sin(a + 0.4));
				ctx.closePath();
				ctx.fill();
				ctx.restore();
			};
			// Colonne du même tableau, depuis une ligne de données : la flèche part de
			// la cellule de cette ligne ([Quantité]*[Prix unitaire]), pas de l'en-tête ;
			// une ligne précise (Tableau[Col][1], [Col][-1], REFERENCE) part de sa
			// cellule, une plage (Tableau[Col][2:4]) de son cadre (auditarrows.ts).
			const own = tableAt(arrows.row, arrows.col);
			const ownT = own ? { name: own.t.name, row: own.t.row } : null;
			const once = (k: string) => !drawn.has(k) && !!drawn.add(k);
			const frame = (s: ReturnType<typeof arrowSpan>, color: string) => {
				if (s.rows <= 1) return;
				const b = spanBox(s, geo);
				ctx.save();
				ctx.strokeStyle = color;
				ctx.lineWidth = 1.5;
				ctx.strokeRect(b.x + 1.5, b.y + 1.5, b.w - 3, b.h - 3);
				ctx.restore();
			};
			for (const p of arrows.pre) {
				const s = arrowSpan(p, arrows, ownT);
				if (p.sheet === sh.sheetIdOf() && once(`${s.row}+${s.rows},${s.col}>${arrows.row},${arrows.col}`)) {
					frame(s, '#2b6cb0');
					draw(spanPoint(s, geo), me, '#2b6cb0');
				}
			}
			for (const d of arrows.dep) {
				const s = arrowSpan(d, arrows, ownT);
				if (d.sheet === sh.sheetIdOf() && once(`${arrows.row},${arrows.col}>${s.row}+${s.rows},${s.col}`)) {
					frame(s, '#c0392b');
					draw(me, spanPoint(s, geo), '#c0392b');
				}
			}
		}
		// Saisie dans la barre de formule : la cellule active l'affiche en direct
		// (pas une formule d'une autre feuille, en cours de pointage ici).
		if (sh.fxEditing && !sh.editing && !sh.hold) {
			const x = cx(ar.c0);
			const y = ry(ar.r0);
			const cw = spanW(ar.c0, ar.c1);
			const ch = rspan(ar.r0, ar.r1);
			ctx.fillStyle = C.bg;
			ctx.fillRect(x + 1, y + 1, cw - 2, ch - 2);
			// Coupée au bord de la cellule (ou de la fusion), sur fond opaque : la
			// saisie ne passe plus sur les valeurs voisines (une formule qui déverse,
			// demande de Louis) ; le texte défile pour garder le curseur visible.
			ctx.font = FONT;
			const caret = sh.fxInput?.selectionEnd ?? sh.fxText.length;
			const shift = previewShift(ctx.measureText(sh.fxText.slice(0, caret)).width, ctx.measureText(sh.fxText).width, cw - 12);
			ctx.save();
			ctx.beginPath();
			const left = Math.max(x, MARGIN);
			const top = Math.max(y, TOP_H);
			ctx.rect(left, top, x + cw - left, y + ch - top);
			ctx.clip();
			drawText(sh.fxText, x - shift, y, cw + shift, 'left', undefined, C.text, cw + shift, ch);
			ctx.restore();
		}
		// Poignée de recopie (coin bas droit de la sélection) et aperçu.
		if (!sh.editing && sh.selChart === null) {
			const h = sh.fillHandle();
			ctx.fillStyle = C.active;
			ctx.fillRect(h.x - 3, h.y - 3, 6, 6);
			ctx.fillStyle = C.bg;
			ctx.fillRect(h.x - 3, h.y + 3, 6, 1);
			ctx.fillRect(h.x + 3, h.y - 3, 1, 7);
		}
		if (sh.filling?.to) {
			const t = sh.filling.to;
			ctx.save();
			ctx.setLineDash([3, 2]);
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 1.5;
			ctx.strokeRect(cx(t.c0) + 1, ry(t.r0) + 1, spanW(t.c0, t.c1) - 2, rspan(t.r0, t.r1) - 2);
			ctx.restore();
		}
		sh.drawCommentMarks(L);
		ctx.restore();
		return incomplete;
	}

	/** Bandeau du haut d'un volet de colonnes. */
	function drawTopStrip(L: Layer) {
		const { ctx, C, cax, MARGIN } = sh;
		const { w, r0, r1, c0, c1, cx, ry, visible, k0, k1 } = L;
		// Bandeau du haut : vide (pas de lettres de colonnes), sauf quand
		// l'en-tête d'un tableau visible est sorti de l'écran.
		ctx.fillStyle = C.strip;
		ctx.fillRect(0, 0, w, TOP_H);
		// Colonnes de la sélection : même surbrillance que les lignes dans la marge.
		ctx.fillStyle = C.activeStrip;
		for (let c = Math.max(c0, k0); c <= Math.min(c1, k1); c++) {
			ctx.fillRect(Math.max(cx(c), MARGIN), 0, cx(c + 1) - Math.max(cx(c), MARGIN), TOP_H);
		}
		// Colonne de la cellule trouvée (Ctrl+F).
		if (sh.findCross && sh.findCross.c1 >= c0 && sh.findCross.c0 <= c1) {
			ctx.fillStyle = C.findGutter;
			const x = Math.max(cx(sh.findCross.c0), MARGIN);
			ctx.fillRect(x, 0, cx(sh.findCross.c1 + 1) - x, TOP_H);
		}
		if (sh.a1) {
			ctx.font = FONT_SMALL;
			ctx.textAlign = 'center';
			for (let c = c0; c <= c1; c++) {
				const x = cx(c);
				if (x + cax().size(c) <= MARGIN || !cax().size(c)) continue;
				ctx.fillStyle = c >= k0 && c <= k1 ? C.headText : C.stripText;
				ctx.fillText(sh.addrA1(0, c).slice(0, -1), x + cax().size(c) / 2, TOP_H / 2);
			}
		}
		for (const e of sh.a1 ? [] : visible) {
			// En-tête encore visible (ou dans les lignes figées) : pas de rappel.
			if (e.t.row >= r0 || e.last < r0 || e.t.row < sh.fRows()) continue;
			// Groupes d'en-têtes : en petit au-dessus des sous-en-têtes, sur toutes leurs colonnes.
			const gh = e.t.twoLevel ? 10 : 0;
			for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol); c++) {
				const x = cx(c);
				ctx.save();
				ctx.beginPath();
				ctx.rect(Math.max(x, MARGIN), 0, cax().size(c), TOP_H);
				ctx.clip();
				const linked = !!e.t.query && !e.t.columns[c - e.t.col]?.hand;
				ctx.fillStyle = c >= k0 && c <= k1 ? C.activeStrip : linked ? C.linkHead : C.head;
				ctx.fillRect(x, 0, cax().size(c), TOP_H);
				// Boutons ▾, type et entonnoir, cliquables comme dans l'en-tête.
				const iw = headIcons(e, c - e.t.col, x, gh, cax().size(c), TOP_H - gh);
				ctx.beginPath();
				ctx.rect(Math.max(x, MARGIN), 0, Math.max(0, cax().size(c) - iw - 10), TOP_H);
				ctx.clip();
				ctx.font = gh ? '600 11px "Segoe UI", system-ui, -apple-system, sans-serif' : FONT_BOLD;
				ctx.fillStyle = linked ? C.linkHeadText : C.headText;
				ctx.textAlign = 'left';
				ctx.fillText(headerLabel(e.t.columns[c - e.t.col].name, e.t.twoLevel), x + 6, (gh + TOP_H) / 2);
				ctx.restore();
			}
			if (!gh) continue;
			ctx.save();
			ctx.beginPath();
			ctx.rect(MARGIN, 0, w - MARGIN, gh);
			ctx.clip();
			for (const s of groupSpans(e.t.columns.map((k) => k.name))) {
				const a = e.t.col + s.i0;
				const b = e.t.col + s.i1;
				if (b < c0 || a > c1) continue;
				const x = cx(a);
				const sw = cax().offset(b + 1) - cax().offset(a);
				ctx.strokeStyle = e.t.query ? C.linkBorder : C.border;
				ctx.lineWidth = 1;
				ctx.beginPath();
				ctx.moveTo(Math.round(x) - 0.5, 0);
				ctx.lineTo(Math.round(x) - 0.5, gh);
				ctx.moveTo(x, gh - 0.5);
				ctx.lineTo(x + sw, gh - 0.5);
				ctx.stroke();
				if (!s.group) continue;
				ctx.font = '600 ' + FONT_TINY;
				ctx.fillStyle = e.t.query ? C.linkHeadText : C.headText;
				ctx.textAlign = 'center';
				ctx.fillText(ellipsize(s.group, sw - 8), Math.max(MARGIN, x) / 2 + Math.min(x + sw, w) / 2, gh / 2 + 1);
			}
			ctx.restore();
		}
		// Petits traits aux séparations de colonnes : on voit où redimensionner.
		ctx.beginPath();
		ctx.strokeStyle = C.stripLine;
		ctx.lineWidth = 1;
		for (let c = c0; c <= c1; c++) {
			const x = Math.round(cx(c + 1)) - 0.5;
			if (x <= MARGIN) continue;
			ctx.moveTo(x, TOP_H * 0.75);
			ctx.lineTo(x, TOP_H);
		}
		ctx.stroke();
		for (let c = Math.max(c0, k0); c <= Math.min(c1, k1); c++) {
			ctx.fillStyle = C.active;
			ctx.fillRect(cx(c), TOP_H - 2, cax().size(c), 2);
		}
		// Colonnes masquées : même repère, un clic les réaffiche.
		for (const [i0, i1] of sh.hiddenRuns(sh.styleHiddenCols)) {
			if (i1 < c0 - 1 || i0 > c1 + 1) continue;
			const x = Math.round(cx(i0));
			if (x <= MARGIN) continue;
			sh.hiddenMark({ rows: false, i0, i1, x: x - 4, y: 3, w: 8, h: TOP_H - 6 });
		}
	}

	/** Marge de gauche d'un volet de lignes. */
	function drawMargin(L: Layer) {
		const { ctx, C, rh, MARGIN } = sh;
		const { h, r0, r1, ry, visible, s0, s1 } = L;
		// Première colonne affichée (la gouttière) : la première figée s'il y en a.
		const c0 = sh.fCols() > 0 ? 0 : L.c0;
		// Marge de gauche : numéros 1..n des lignes de données des tableaux.
		ctx.fillStyle = C.strip;
		ctx.fillRect(0, TOP_H, MARGIN, h - TOP_H);
		ctx.font = FONT_SMALL;
		ctx.textAlign = 'right';
		// Numéros : ceux du tableau de la sélection (ou de celui qu'elle
		// prolonge) ; ceux des autres tableaux, plus discrets.
		const cur = sh.ownerAt(sh.active.row, sh.active.col) ?? sh.entries.find((e) => sh.grown(e).last !== e.last);
		const curLast = cur ? sh.grown(cur).last : -1;
		const blocks = visible.map((e) => ({ row: e.t.row, last: e.last, col: e.t.col, lastCol: e.lastCol, e }));
		for (let r = r0; r <= r1; r++) {
			const y = ry(r);
			if (!rh(r)) continue; // ligne masquée
			if (r >= s0 && r <= s1) {
				ctx.fillStyle = C.activeStrip;
				ctx.fillRect(0, y, MARGIN, rh(r));
				ctx.fillStyle = C.active;
				ctx.fillRect(MARGIN - 2, y, 2, rh(r));
			}
			// Ligne de la cellule trouvée (Ctrl+F).
			if (sh.findCross && r >= sh.findCross.r0 && r <= sh.findCross.r1) {
				ctx.fillStyle = C.findGutter;
				ctx.fillRect(0, y, MARGIN - 2, rh(r));
			}
			if (sh.a1) {
				ctx.fillStyle = r >= s0 && r <= s1 ? C.headText : C.stripText;
				ctx.fillText(String(r + 1), MARGIN - 8, y + rh(r) / 2);
				continue;
			}
			// Seul le tableau qui touche la gouttière (première colonne affichée)
			// y est numéroté : pas de numéros en face de lignes sans rapport.
			const mine = !!cur && r > cur.t.row && r <= curLast && cur.t.col <= c0 && cur.lastCol >= c0;
			const e = mine ? cur : gutterOwner(blocks, r, c0)?.e;
			if (e) {
				ctx.fillStyle = mine || !cur ? C.stripText : C.stripLine;
				let label = rowLabel(e, r);
				if (e.t.key !== undefined) {
					label = midEllipsis(label, GUTTER_KEY_MAX - GUTTER_PAD, (s) => ctx.measureText(s).width);
					const kw = ctx.measureText(label).width;
					if (kw > (sh.keyWidths.get(e.t.name) ?? 0)) {
						sh.keyWidths.set(e.t.name, kw);
						sh.keyGrew = true;
					}
				}
				ctx.save();
				ctx.beginPath();
				ctx.rect(0, y, MARGIN - 4, rh(r));
				ctx.clip();
				ctx.fillText(label, MARGIN - 8, y + rh(r) / 2);
				ctx.restore();
			}
		}
		// Petits traits aux séparations de lignes (un quart de la marge, à droite) :
		// on voit où redimensionner, comme pour les colonnes.
		ctx.beginPath();
		ctx.strokeStyle = C.stripLine;
		ctx.lineWidth = 1;
		for (let r = r0; r <= r1; r++) {
			const y = Math.round(ry(r + 1)) - 0.5;
			if (y <= TOP_H || !rh(r)) continue;
			ctx.moveTo(MARGIN * 0.75, y);
			ctx.lineTo(MARGIN, y);
		}
		ctx.stroke();
		// Lignes masquées : un repère à leur place (on s'en souvient), un clic
		// les réaffiche.
		for (const [i0, i1] of sh.hiddenRuns(sh.styleHiddenRows)) {
			if (i1 < r0 - 1 || i0 > r1 + 1) continue;
			const y = Math.round(ry(i0));
			if (y <= TOP_H) continue;
			const m = { rows: true, i0, i1, x: 2, y: y - 4, w: MARGIN - 4, h: 8 };
			sh.hiddenMark(m);
		}
	}

	/**
	 * Cellules fusionnées d'un volet, par-dessus le quadrillage : remplissage,
	 * texte sur toute la fusion, bordures et note.
	 */
	function drawMerges(L: Layer) {
		const { ctx, C, rspan, styleAt, borders, key } = sh;
		const { r0, r1, c0, c1, cx, ry, spanW, s0, s1, k0, k1 } = L;
		const many = !sh.singleSelection();
		for (const g of sh.merges) {
			const gr1 = g.row + g.rows - 1;
			const gc1 = g.col + g.cols - 1;
			if (gr1 < r0 || g.row > r1 || gc1 < c0 || g.col > c1) continue;
			const k = key(g.row, g.col);
			const st = resolveStyle(styleAt(g.row, g.col, undefined), sh.freeCf(g.row, g.col)?.style);
			const x = cx(g.col);
			const y = ry(g.row);
			const gw = spanW(g.col, gc1);
			const gh = rspan(g.row, gr1);
			if (gw <= 0 || gh <= 0) continue;
			// Le fond couvre le quadrillage à l'intérieur de la fusion seulement.
			ctx.fillStyle = themed(st?.bg) ?? C.bg;
			ctx.fillRect(Math.round(x), Math.round(y), gw - 1, gh - 1);
			// Dans la sélection (sauf la fusion active) : surlignée comme les autres
			// cellules ; traversée par des lignes ou colonnes entières, sur leur
			// partie seulement (comme Excel).
			const isActive = sh.active.row === g.row && sh.active.col === g.col;
			const part = many && !isActive ? intersect({ r0: g.row, r1: gr1, c0: g.col, c1: gc1 }, { r0: s0, r1: s1, c0: k0, c1: k1 }) : null;
			if (part) {
				const px = cx(part.c0);
				const py = ry(part.r0);
				ctx.fillStyle = C.select;
				ctx.fillRect(Math.round(px), Math.round(py), spanW(part.c0, part.c1) - (part.c1 === gc1 ? 1 : 0), rspan(part.r0, part.r1) - (part.r1 === gr1 ? 1 : 0));
			}
			const f = sh.shown.get(k);
			const raw = f ? f.text : sh.cells.get(k);
			if (raw) {
				const num = sh.isNumeric(raw);
				const [mt, mst] = codeShown(raw, undefined, st);
				drawText(mt, x, y, gw, st?.al ?? (num ? 'right' : 'left'), mst, f?.error ? C.error : C.text, gw, gh);
			}
			if (st) borders(st, x, y, gw, gh);
			if (st?.nt) noteCorner(x + gw, y, st);
		}
	}

	/**
	 * Bandes des groupes des tableaux aux en-têtes sur deux lignes, par-dessus
	 * le quadrillage : une cellule par groupe, fusionnée sur ses colonnes,
	 * centrée et en gras comme l'en-tête (vide pour une colonne sans groupe).
	 */
	function drawGroupBands(L: Layer) {
		const { ctx, C, rh, borders } = sh;
		const { r0, r1, c0, c1, cx, ry, spanW, s0, s1, k0, k1 } = L;
		const many = !sh.singleSelection();
		const ar = sh.activeRect();
		for (const e of L.visible) {
			const r = e.t.row - 1;
			if (!e.t.twoLevel || r > r1 || e.t.row < r0 || !rh(r)) continue;
			const y = ry(r);
			for (const s of groupSpans(e.t.columns.map((k) => k.name))) {
				const a = e.t.col + s.i0;
				const b = e.t.col + s.i1;
				if (b < c0 || a > c1) continue;
				const x = cx(a);
				const sw = spanW(a, b);
				if (sw <= 0) continue;
				if (s.i0 === s.i1 && sh.singleCol(e, s.i0)) {
					singleCell(e, s.i0, x, y, sw, many, ar, L);
					continue;
				}
				if (r < r0) continue;
				// Aspect d'un en-tête (fond, quadrillage, gras), sous la mise en
				// forme posée par le ruban sur ses cellules (fond, bordures, police…).
				const st = sh.bandStyle(e, s.i0, s.i1);
				ctx.fillStyle = themed(st?.bg) ?? (e.t.query ? C.linkHead : C.head);
				ctx.fillRect(x, y, sw, rh(r));
				// Dans la sélection (sauf la cellule active) : surlignée comme l'en-tête.
				if (many && r >= s0 && r <= s1 && b >= k0 && a <= k1 && !(ar.r0 === r && ar.c0 === a)) {
					ctx.fillStyle = C.select;
					ctx.fillRect(cx(Math.max(a, k0)), y, spanW(Math.max(a, k0), Math.min(b, k1)), rh(r));
				}
				// Ses bordures par défaut se tracent avec le cadre du tableau (headerLines).
				if (st) borders(st, x, y, sw, rh(r));
				if (st?.nt) noteCorner(x + sw, y, st);
				if (s.group) headLabel(s.group, x, y, sw, rh(r), 6, st, e.t.query ? C.linkHeadText : C.headText, 'center');
			}
		}
	}

	/**
	 * Colonne sur un niveau d'un tableau à deux lignes : une cellule sur la
	 * bande et l'en-tête, avec le nom de la colonne comme un en-tête (en gras,
	 * à gauche, centré en hauteur) ; les boutons restent dans l'en-tête.
	 */
	function singleCell(e: Entry, i: number, x: number, y: number, w: number, many: boolean, ar: { r0: number; c0: number }, L: Layer) {
		const { ctx, C, rh, borders } = sh;
		const c = e.t.col + i;
		const hr = e.t.row;
		const h2 = rh(hr - 1) + rh(hr);
		const hs = sh.bandStyle(e, i, i);
		ctx.fillStyle = e.t.query ? C.linkHead : C.head;
		ctx.fillRect(x, y, w, rh(hr - 1));
		if (hs?.bg) {
			ctx.fillStyle = themed(hs.bg);
			ctx.fillRect(x, y, w, rh(hr - 1));
		}
		// Partie de la bande : surlignée quand sa ligne est dans la sélection (la
		// ligne d'en-tête seule, choisie dans la gouttière, ne la prend pas).
		if (many && L.s0 <= hr - 1 && L.s1 >= hr - 1 && c >= L.k0 && c <= L.k1 && !(ar.r0 === hr - 1 && ar.c0 === c)) {
			ctx.fillStyle = C.select;
			ctx.fillRect(x, y, w, rh(hr - 1));
		}
		// Ses bordures par défaut se tracent avec le cadre du tableau (headerLines).
		if (hs) borders(hs, x, y, w, h2);
		if (hs?.nt) noteCorner(x + w, y, hs);
		const iw = sh.singleIcons.get(`${e.t.id}:${i}`) ?? 0;
		headLabel(e.t.columns[i].name, x, y, w, h2, iw + 10, hs, e.t.query ? C.linkHeadText : C.headText, 'left');
	}

	/**
	 * Bordures par défaut des en-têtes d'un tableau, comme son cadre : une
	 * séparation entre chaque colonne de l'en-tête, et sur un tableau aux
	 * en-têtes sur deux lignes, une entre les cellules de la bande des groupes
	 * et un trait entre la bande et l'en-tête (sauf sous une colonne sur un
	 * niveau, d'un seul tenant). Segments [x0, y0, x1, y1] posés sur les
	 * limites des cellules, pour crispLines ; le cadre et le trait sous
	 * l'en-tête sont à part. La mise en forme du ruban se trace par-dessus.
	 */
	function headerLines(e: Entry, L: Layer, a: TableAnim | null): number[] {
		const { rh } = sh;
		const out: number[] = [];
		const hr = e.t.row;
		const X = (c: number) => L.cx(c) + (a ? a.dx + a.colOff(c) : 0);
		const Y = (r: number) => L.ry(r) + (a ? a.dy + a.rowOff(r) : 0);
		// Colonnes visibles seulement (une limite de plus de chaque côté).
		const from = Math.max(e.t.col + 1, L.c0);
		const to = Math.min(e.lastCol, L.c1 + 1);
		if (rh(hr) && hr >= L.r0 && hr <= L.r1) for (let c = from; c <= to; c++) out.push(X(c), Y(hr), X(c), Y(hr + 1));
		const br = hr - 1;
		if (!e.t.twoLevel || !rh(br) || br > L.r1 || hr < L.r0) return out;
		for (const s of groupSpans(e.t.columns.map((k) => k.name))) {
			const c0 = e.t.col + s.i0;
			const c1 = e.t.col + s.i1;
			if (c1 + 1 < L.c0 || c0 > L.c1 + 1) continue;
			if (s.i0 > 0) out.push(X(c0), Y(br), X(c0), Y(hr));
			if (!(s.i0 === s.i1 && sh.singleCol(e, s.i0))) out.push(X(c0), Y(hr), X(c1 + 1), Y(hr));
		}
		return out;
	}

	/**
	 * Traits nets d'un pixel de l'écran (deux à partir de 150 % : comme un
	 * trait de 1 px agrandi), de la couleur de remplissage courante : chaque
	 * segment [x0, y0, x1, y1] (vertical ou horizontal, posé sur une limite
	 * de cellules) devient un rectangle aligné sur les pixels de l'écran, juste
	 * avant la limite (à gauche ou au-dessus), comme le quadrillage, quels que
	 * soient le zoom et la densité de l'écran. Tous les traits se remplissent
	 * d'un coup : leurs croisements ne foncent pas et ne laissent pas de trou.
	 */
	function crispLines(l: number[]) {
		const { ctx } = sh;
		if (!l.length) return;
		const m = ctx.getTransform();
		const lw = Math.max(1, Math.round(m.a));
		const px = (v: number) => Math.round(v * m.a + m.e);
		const py = (v: number) => Math.round(v * m.d + m.f);
		ctx.save();
		ctx.setTransform(1, 0, 0, 1, 0, 0);
		ctx.beginPath();
		for (let i = 0; i < l.length; i += 4) {
			const x0 = px(Math.min(l[i], l[i + 2]));
			const x1 = px(Math.max(l[i], l[i + 2]));
			const y0 = py(Math.min(l[i + 1], l[i + 3]));
			const y1 = py(Math.max(l[i + 1], l[i + 3]));
			ctx.rect(x0 - lw, y0 - lw, x1 - x0 + lw, y1 - y0 + lw);
		}
		ctx.fill();
		ctx.restore();
	}

	/**
	 * Nom d'un en-tête de tableau ou d'un groupe de la bande, en gras, entre
	 * 6 px à gauche et right px à droite (ses boutons) : raccourci avec « … »,
	 * ou renvoyé à la ligne (wr) sur la hauteur de l'en-tête (grandie par
	 * measureHeader, measureBand). Couleur, italique, souligné, barré et
	 * alignements de la mise en forme l'emportent sur l'aspect par défaut.
	 */
	function headLabel(text: string, x: number, y: number, w: number, h: number, right: number, hs: CellStyle | undefined, color: string, align: 'left' | 'center') {
		const { ctx } = sh;
		const avail = Math.max(8, w - 6 - right);
		ctx.save();
		ctx.beginPath();
		ctx.rect(x, y, Math.max(0, w - right), h);
		ctx.clip();
		ctx.font = (hs?.i ? 'italic ' : '') + FONT_BOLD;
		ctx.fillStyle = themed(hs?.fg) ?? color;
		const al = hs?.al ?? align;
		ctx.textAlign = al;
		ctx.textBaseline = 'middle';
		const lines = hs?.wr ? wrapLines(text, avail, (t) => ctx.measureText(t).width) : [ellipsize(text, w - 6 - right)];
		const lh = lineHeight(BASE_PX);
		const total = lines.length * lh;
		const top = hs?.va === 'top' ? y + 2 : hs?.va === 'bottom' ? y + h - 2 - total : y + (h - total) / 2;
		const tx = al === 'left' ? x + 6 : al === 'right' ? x + 6 + avail : x + 6 + avail / 2;
		lines.forEach((line, k) => {
			const cy = top + lh * k + lh / 2;
			ctx.fillText(line, tx, cy);
			if (hs?.u || hs?.x) {
				const lw = ctx.measureText(line).width;
				const lx = al === 'left' ? tx : al === 'right' ? tx - lw : tx - lw / 2;
				if (hs.u) ctx.fillRect(lx, cy + 7, lw, 1);
				if (hs.x) ctx.fillRect(lx, cy, lw, 1);
			}
		});
		ctx.restore();
	}

	/** Boutons d'un en-tête renvoyé à la ligne plus larges ou plus étroits qu'à sa mesure : l'en-tête se remesure. */
	function noteHeadIcons(e: Entry, i: number, iconW: number, hs: CellStyle | undefined) {
		const k = `${e.t.id}:${i}`;
		const before = sh.headIconW.get(k);
		sh.headIconW.set(k, iconW);
		if (hs?.wr && Math.abs((before ?? sh.HEAD_ICON_W) - iconW) > 0.5) {
			sh.autoRows.forget(e.t.row, 1);
			sh.schedule();
		}
	}

	/** Coin rouge d'une cellule qui a une note (en haut à droite, comme Excel). */
	function noteCorner(xr: number, y: number, st?: { nt?: string; np?: boolean }) {
		const { ctx, C } = sh;
		if (st?.np && st.nt) sh.pinnedFrame.push({ x: xr + 6, y, text: st.nt });
		ctx.fillStyle = C.note;
		ctx.beginPath();
		ctx.moveTo(xr - 7, y + 1);
		ctx.lineTo(xr - 1, y + 1);
		ctx.lineTo(xr - 1, y + 7);
		ctx.closePath();
		ctx.fill();
	}

	/** Bouton ▾ au bord droit d'une cellule (xr : son bord droit) ; filtré : entonnoir, comme Excel. */
	function pivotDrop(xr: number, y: number, h: number, filtered: boolean) {
		const { ctx, C } = sh;
		const size = Math.min(15, h - 4);
		const x = xr - size - 2;
		const top = y + (h - size) / 2;
		ctx.save();
		ctx.fillStyle = C.strip;
		ctx.fillRect(x, top, size, size);
		ctx.strokeStyle = C.stripLine;
		ctx.lineWidth = 1;
		ctx.strokeRect(Math.round(x) + 0.5, Math.round(top) + 0.5, size - 1, size - 1);
		ctx.fillStyle = filtered ? C.active : C.icon;
		ctx.font = FONT_SMALL;
		ctx.textAlign = 'center';
		ctx.textBaseline = 'middle';
		ctx.fillText(filtered ? '⧩' : '▾', x + size / 2, top + size / 2 + 0.5);
		ctx.restore();
	}

	/** Case −/+ d'un groupe de TCD en arborescence (x : son bord gauche), comme Excel. */
	function pivotFoldBox(x: number, y: number, h: number, folded: boolean) {
		const { ctx, C } = sh;
		const s = PIVOT_BOX;
		const left = Math.round(x) + 0.5;
		const top = Math.round(y + (h - s) / 2) + 0.5;
		ctx.save();
		ctx.fillStyle = C.strip;
		ctx.fillRect(left, top, s - 1, s - 1);
		ctx.strokeStyle = C.icon;
		ctx.lineWidth = 1;
		ctx.strokeRect(left, top, s - 1, s - 1);
		ctx.beginPath();
		const mid = top + (s - 1) / 2;
		ctx.moveTo(left + 2, mid);
		ctx.lineTo(left + s - 3, mid);
		if (folded) {
			const cxm = left + (s - 1) / 2;
			ctx.moveTo(cxm, top + 2);
			ctx.lineTo(cxm, top + s - 3);
		}
		ctx.stroke();
		ctx.restore();
	}

	/** Première ligne des en-têtes d'un TCD (au-dessus de p.row avec plusieurs champs en colonnes). */
	function pivotHead(p: SheetPivot): number {
		return p.row - (p.error ? 0 : (p.colHead?.length ?? 0));
	}

	/** En-têtes imbriqués d'un TCD : étiquette d'un groupe de colonnes au-dessus de sa première colonne, avec sa case −. */
	function drawPivotColHead(p: SheetPivot, pc1: number, r0: number, r1: number, g: { cx: (c: number) => number; ry: (r: number) => number; spanW: (c0: number, c1: number) => number }) {
		const { ctx, C, rh, cax, styleAt } = sh;
		const { cx, ry, spanW } = g;
		const head = p.colHead ?? [];
		head.forEach((row, d) => {
			const hr = row ?? [];
			const r = pivotHead(p) + d;
			if (r < r0 || r > r1 || !rh(r)) return;
			for (let c = p.col; c <= pc1; c++)
				if (!styleAt(r, c, undefined)?.bg) {
					ctx.fillStyle = C.linkHead;
					ctx.fillRect(cx(c), ry(r), cax().size(c), rh(r));
				}
			hr.forEach((v, j) => {
				if (!v) return;
				const c = p.col + j;
				let end = j + 1;
				while (end < hr.length && !hr[end] && (p.cols?.[end]?.path?.length ?? 0) > d + 1 && p.cols?.[end]?.path?.[d] === v) end++;
				pivotFoldBox(cx(c) + pivotBoxLeft(0), ry(r), rh(r), false);
				const ind = pivotBoxLeft(0) + PIVOT_BOX + 1;
				const st = resolveStyle(styleAt(r, c, undefined), undefined);
				drawText(v, cx(c) + ind, ry(r), spanW(c, Math.min(pc1, p.col + end - 1)) - ind, 'left', { ...st, b: st?.b ?? true }, C.linkHeadText);
			});
		});
	}

	/** Pastilles posées pendant le dessin des cadres, par-dessus le bandeau du haut. */
	function drawFollowBadges() {
		const { ctx, C, MARGIN } = sh;
		for (const b of [...sh.followBadges, ...sh.pivotBadges.map((x) => ({ ...x, kind: 'pivot' as const }))]) {
			ctx.save();
			ctx.beginPath();
			ctx.rect(Math.max(b.clip.x0, MARGIN), b.clip.y0, b.clip.x1 - Math.max(b.clip.x0, MARGIN), b.clip.y1 - b.clip.y0);
			ctx.clip();
			ctx.fillStyle = b.kind === 'pivot' ? C.sourceBadge : b.kind === 'follow' ? C.followBadge : b.kind === 'name' ? C.nameBadge : b.kind === 'local' ? C.localBadge : C.unnamedBadge;
			ctx.beginPath();
			ctx.roundRect(b.x0, b.y0, b.x1 - b.x0, b.y1 - b.y0, b.radii);
			ctx.fill();
			ctx.font = FONT_SMALL;
			ctx.fillStyle = '#fff';
			ctx.textAlign = 'center';
			ctx.textBaseline = 'middle';
			ctx.fillText(b.label, (b.x0 + b.x1) / 2, (b.y0 + b.y1) / 2 + 0.5);
			ctx.restore();
		}
	}

	/**
	 * Boutons d'un en-tête de colonne, alignés à droite : ▾ (filtre), type,
	 * entonnoir d'un filtre actif, clé, erreurs. Dans la ligne d'en-tête du
	 * tableau et dans le bandeau quand l'en-tête est figé. Renvoie leur largeur.
	 */
	function headIcons(e: Entry, i: number, x: number, y: number, cw: number, h: number): number {
		const { ctx, C } = sh;
		const col = e.t.columns[i];
		// Icônes dans la couleur du tableau : celle des sources externes pour une requête.
		const ic = e.t.query && !col.hand ? C.linkIcon : C.icon;
		ctx.font = FONT_SMALL;
		ctx.fillStyle = ic;
		ctx.textAlign = 'right';
		ctx.textBaseline = 'middle';
		// Icône du type : SVG pour date et booléen, texte pour les autres.
		const svg = typeIconName(col.type);
		const ICON = 13;
		const icon = svg ? ' ▾' : (col.filter ? '⧩ ' : '') + typeIcon(col.type) + ' ▾';
		let iconW = ctx.measureText(icon).width;
		// Formule de la colonne en cours d'édition : son icône fx en vert.
		const calc = sh.calcEdit?.e === e && sh.calcEdit.i === i;
		const fx = !svg && typeIcon(col.type) === 'fx';
		if (calc && fx) ctx.fillStyle = C.active;
		ctx.fillText(icon, x + cw - 6, y + h / 2);
		ctx.fillStyle = ic;
		if (svg) {
			drawIcon(ctx, svg, x + cw - 6 - iconW - ICON, y + (h - ICON) / 2, ICON, ic, C.active, 1.3);
			iconW += ICON;
			if (col.filter) {
				ctx.fillText('⧩ ', x + cw - 6 - iconW, y + h / 2);
				iconW += ctx.measureText('⧩ ').width;
			}
		}
		if (e.t.key === i) {
			ctx.fillText('🔑 ', x + cw - 6 - iconW, y + h / 2);
			iconW += ctx.measureText('🔑 ').width;
		}
		// Classeur partagé, première colonne : tableau privé (resté sur ce
		// poste) ou publié en lecture seule.
		const share = i === 0 ? (e.t.shared === 'private' ? 'local' : e.t.readOnly ? 'readonly' : null) : null;
		if (share) {
			drawIcon(ctx, share, x + cw - 6 - iconW - ICON - 2, y + (h - ICON) / 2, ICON, ic, C.active, 1.3);
			iconW += ICON + 4;
		}
		if (sh.hasKeyHint(e, i)) {
			// Clé proposée : la même clé, grisée et pâle ; un clic la pose.
			const kx = x + cw - 6 - iconW;
			const kw = ctx.measureText('🔑 ').width;
			ctx.save();
			ctx.globalAlpha = 0.4;
			ctx.filter = 'grayscale(1)';
			ctx.fillText('🔑 ', kx, y + h / 2);
			ctx.restore();
			sh.keyHintBoxes.push({ e, x0: kx - kw, x1: kx, y0: y, y1: y + h });
			iconW += kw;
		}
		if (calc && !fx) {
			ctx.save();
			ctx.font = 'italic bold ' + FONT_SMALL;
			ctx.fillStyle = C.active;
			ctx.fillText('ƒx ', x + cw - 6 - iconW, y + h / 2);
			iconW += ctx.measureText('ƒx ').width;
			ctx.restore();
		}
		if (col.errors) {
			ctx.fillStyle = C.error;
			ctx.fillText('⚠ ' + col.errors, x + cw - 6 - iconW - ctx.measureText(' ').width, y + h / 2);
			ctx.fillStyle = ic;
		}
		return iconW;
	}

	/** Numéro d'une ligne de données du tableau (1 = première), ou sa clé : ses valeurs identifient les lignes (choix de Louis). */
	function rowLabel(e: Entry, r: number): string {
		if (e.t.key !== undefined && r <= e.last) {
			const k = e.cache.row(r - e.t.row - 1)?.text[e.t.key];
			if (k !== undefined) return k;
		}
		return String(r - e.t.row);
	}

	/** Texte raccourci avec « … » pour tenir dans width (police courante). */
	function ellipsize(text: string, width: number): string {
		const { ctx } = sh;
		if (ctx.measureText(text).width <= width) return text;
		let lo = 0;
		let hi = text.length;
		while (lo < hi) {
			const mid = (lo + hi + 1) >> 1;
			if (ctx.measureText(text.slice(0, mid) + '…').width <= width) lo = mid;
			else hi = mid - 1;
		}
		return lo > 0 ? text.slice(0, lo) + '…' : '…';
	}

	function strokeBorders() {
		const { ctx, C, MARGIN } = sh;
		if (sh.borderSet.empty) return;
		ctx.save();
		ctx.beginPath();
		ctx.rect(MARGIN, TOP_H, sh.viewW - MARGIN, sh.viewH - TOP_H);
		ctx.clip();
		sh.borderSet.stroke(ctx, (c) => (c ? themed(c) : C.text), C.bg);
		ctx.restore();
	}

	/** Texte affiché et style d'une cellule : la couleur d'un code de format ([Rouge]) remplace celle du texte. */
	function codeShown(text: string, value: unknown, st: CellStyle | undefined): [string, CellStyle | undefined] {
		if (!st?.nc) return [formatCell(text, value, st), st];
		const r = shownCell(text, value, st);
		return [r.text, r.color ? { ...st, fg: r.color } : st];
	}

	/**
	 * Classeur collaboratif : petit triangle dans le coin bas droit d'une
	 * cellule dont la modification attend le serveur (gris bleu) ou en
	 * conflit (orange).
	 */
	function syncCorner(xr: number, yb: number, kind: SyncKind) {
		const { ctx, C } = sh;
		ctx.fillStyle = kind === 'conflict' ? C.conflict : C.pending;
		ctx.beginPath();
		ctx.moveTo(xr - 1, yb - 1);
		ctx.lineTo(xr - 7, yb - 1);
		ctx.lineTo(xr - 1, yb - 7);
		ctx.closePath();
		ctx.fill();
	}

	/** Petit triangle d'angle d'une cellule en erreur (le détail est dans la barre d'état). */
	function errorCorner(x: number, y: number) {
		const { ctx, C } = sh;
		ctx.fillStyle = C.error;
		ctx.beginPath();
		ctx.moveTo(x + 1, y + 1);
		ctx.lineTo(x + 7, y + 1);
		ctx.lineTo(x + 1, y + 7);
		ctx.closePath();
		ctx.fill();
	}

	/**
	 * Texte d'une cellule avec sa mise en forme : police, taille, gras,
	 * italique, souligné, barré, couleur, alignements horizontal et vertical,
	 * renvoi à la ligne. hh : hauteur de la cellule (par défaut celle de la
	 * ligne en y ; une fusion couvre plusieurs lignes).
	 */
	function drawText(s: string, x: number, y: number, cw: number, align: 'left' | 'center' | 'right', st: CellStyle | undefined, color: string, clipW = cw, hh?: number) {
		const { ctx, C, rh, rax, MARGIN } = sh;
		const h = hh ?? rh(rax().indexAt(y - TOP_H + sh.paneTop + 1));
		if (h <= 0 || cw <= 0) return;
		ctx.save();
		ctx.beginPath();
		// Renvoyé à la ligne, ou fusion (hh) : le texte reste dans sa cellule.
		// La découpe s'arrête au bord droit de la cellule même quand son début
		// est caché sous la marge.
		const left = Math.max(x, MARGIN);
		if (st?.wr || hh !== undefined) {
			const top = Math.max(y, TOP_H);
			ctx.rect(left, top, x + (st?.wr ? cw : clipW) - left, y + h - top);
		} else ctx.rect(left, TOP_H, x + clipW - left, sh.viewH - TOP_H);
		ctx.clip();
		const fancy = st && (st.ff || st.fs);
		ctx.font = fancy ? cellFont(st) : (st?.i ? 'italic ' : '') + (st?.b ? FONT_BOLD : FONT);
		ctx.textAlign = align;
		ctx.textBaseline = 'middle';
		// Lien hypertexte : bleu (sauf couleur choisie) et souligné.
		const link = !!st?.ln && !!linkKind(st.ln);
		ctx.fillStyle = themed(st?.fg) ?? (link ? C.link : color);
		const tx = align === 'left' ? x + 6 : align === 'right' ? x + cw - 6 : x + cw / 2;
		const px = fontPx(st?.fs);
		const lh = lineHeight(px);
		// Renvoi à la ligne (largeur de la cellule), ou sauts de ligne (Alt+Entrée) :
		// les lignes qui tiennent dans la hauteur.
		// Remplissage (*x d'un code, comptabilité) : la suite du texte va à droite.
		const fillAt = st?.wr ? -1 : s.indexOf(FILL);
		if (st?.wr) s = plain(s);
		const lines = st?.wr
			? wrapLines(s, Math.max(8, cw - 12), (t) => ctx.measureText(t).width)
			: s.includes('\n')
				? cellLines(s, Math.max(1, Math.floor((h - 4) / lh)))
				: [s];
		const total = lines.length * lh;
		// Alignement vertical : centré par défaut.
		const top = st?.va === 'top' ? y + 2 : st?.va === 'bottom' ? y + h - 2 - total : y + (h - total) / 2;
		lines.forEach((line, k) => {
			const cy = top + lh * k + lh / 2;
			if (k === 0 && fillAt >= 0) {
				const after = Array.from(line.slice(fillAt + 1)).slice(1).join('');
				ctx.textAlign = 'left';
				ctx.fillText(line.slice(0, fillAt), x + 6, cy);
				ctx.textAlign = 'right';
				ctx.fillText(after, x + cw - 6, cy);
				return;
			}
			ctx.fillText(line, tx, cy);
			if (st?.u || st?.x || link) {
				const w = ctx.measureText(line).width;
				const lx = align === 'left' ? tx : align === 'right' ? tx - w : tx - w / 2;
				if (st?.u || link) ctx.fillRect(lx, cy + Math.round(px * 0.55), w, 1);
				if (st?.x) ctx.fillRect(lx, cy, w, 1);
			}
		});
		ctx.restore();
	}

	function drawWorking(now: number) {
		const { ctx, cax, rowTop, MARGIN } = sh;
		if (!sh.working.length) return;
		const anim = sh.motion();
		const rFirst = sh.rowAtY(TOP_H + rowTop(sh.fRows()));
		const cFirst = sh.colAtX(MARGIN + cax().offset(sh.fCols()));
		const rLast = sh.rowAtY(sh.viewH - 1);
		const cLast = sh.colAtX(sh.viewW - 1);
		ctx.save();
		ctx.beginPath();
		ctx.rect(MARGIN, TOP_H, sh.viewW - MARGIN, sh.viewH - TOP_H);
		ctx.clip();
		for (const w of sh.working) {
			const a = anim ? workAlpha(now, w.t0) : workAlpha(now, w.t0) > 0 ? 1 : 0;
			if (anim && a < 1) sh.animBusy = true;
			if (a <= 0) continue;
			for (const z of w.zones) {
				// Bornée à la vue : une colonne d'un million de lignes, des lignes entières.
				const r0 = z.r0 >= sh.fRows() ? Math.max(z.r0, rFirst - 1) : z.r0;
				const c0 = z.c0 >= sh.fCols() ? Math.max(z.c0, cFirst - 1) : z.c0;
				const r1 = Math.min(z.r1, rLast + 1);
				const c1 = Math.min(z.c1, cLast + 1);
				if (r0 > r1 || c0 > c1) continue;
				const x = sh.screenX(c0);
				const y = sh.screenY(r0);
				const zw = cax().offset(c1 + 1) - cax().offset(c0);
				const zh = rowTop(r1 + 1) - rowTop(r0);
				if (zw <= 0 || zh <= 0) continue;
				ctx.fillStyle = `rgba(15, 108, 189, ${(0.08 * a).toFixed(3)})`;
				ctx.fillRect(x, y, zw, zh);
				if (anim) {
					// Bande lumineuse qui parcourt la zone dans sa longueur.
					const tall = zh > zw * 1.5;
					const len = tall ? zh : zw;
					const band = Math.max(24, Math.min(180, len * 0.45));
					const at = (tall ? y : x) - band + sweepAt(now, w.t0) * (len + 2 * band);
					const g = tall ? ctx.createLinearGradient(0, at - band, 0, at + band) : ctx.createLinearGradient(at - band, 0, at + band, 0);
					g.addColorStop(0, 'rgba(15, 108, 189, 0)');
					g.addColorStop(0.5, `rgba(15, 108, 189, ${(0.2 * a).toFixed(3)})`);
					g.addColorStop(1, 'rgba(15, 108, 189, 0)');
					ctx.fillStyle = g;
					ctx.fillRect(x, y, zw, zh);
					ctx.setLineDash([6, 4]);
					ctx.lineDashOffset = -((now - w.t0) / 40) % 10;
					sh.animBusy = true;
				}
				ctx.strokeStyle = `rgba(15, 108, 189, ${(0.7 * a).toFixed(3)})`;
				ctx.lineWidth = 1.5;
				ctx.strokeRect(x + 0.75, y + 0.75, zw - 1.5, zh - 1.5);
				ctx.setLineDash([]);
			}
		}
		ctx.restore();
	}

	function drawJumpFlash(now: number) {
		const { ctx, rh, cax, MARGIN } = sh;
		const f = sh.jumpFlash;
		if (!f) return;
		const a = sh.motion() ? 1 - Math.min(1, (now - f.t) / sh.JUMP_FLASH_MS) : 1;
		if (a <= 0) return;
		const x = sh.screenX(f.c0);
		const w = cax().offset(f.c1 + 1) - cax().offset(f.c0);
		ctx.save();
		ctx.beginPath();
		ctx.rect(MARGIN, TOP_H, sh.viewW - MARGIN, sh.viewH - TOP_H);
		ctx.clip();
		ctx.fillStyle = `rgba(255, 196, 0, ${(0.35 * a).toFixed(3)})`;
		ctx.fillRect(x, sh.screenY(f.r), w, rh(f.r));
		ctx.restore();
		if (sh.motion()) sh.animBusy = true;
	}

	return {
		draw,
		sparkAt,
		drawCells,
		singleCell,
		pivotHead,
		rowLabel,
		codeShown,
		drawText,
		drawWorking
	};
}
