// Souris sur la grille : sélection, largeur des colonnes et hauteur des
// lignes, déplacement d'une plage, poignée de recopie, références pointées
// d'une formule, défilement automatique pendant un glisser. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { type SyncKind } from '../cache';
import { VScroll, clamp } from '../scroll';
import { type AxisLike } from '../anim';
import { type Clip, type Rect as ClipRect } from '../clipboard';
import { type CellStyle } from '../style';
import { PanGesture } from '../pan';
import { toGrid } from '../panes';
import { type MergeRange } from '../sheetfmt';
import { wholeLines, type Cell, type CellRange } from '../mergesel';
import { type Zone } from '../findzone';
import { edgeScroll, frameStep } from '../autoscroll';
import { pivotBoxHit, pivotLabelOffset } from '../pivottree';
import { fillSeries, shiftTableColumns } from '../fill';
import { fitColumns, fitRows } from '../autofit';
import { pivotHandAt } from '../pivothand';
import { pivotRangeRef, pivotRefText } from '../pivotref';
import { placesOnClick, type Placement } from '../pivotplace';
import SheetButtons from '../SheetButtons.svelte';
import { argKindAt, needsNextArg, toNextArg, type Catalog, type Completion, type FunctionDoc } from '../autocomplete';
import { formulaRefs, movedRef, pointedRef, rangeRef, refRect, type Rect, type RefTable } from '../formulaEdit';
import { insertRefAt, qualifyRef, updateHold, type FormulaHold } from '../crossref';
import { errorText, t as tr } from '#lib/i18n/index.js';
import { type PivotConfig, type SheetPivot, type SheetSource, type TextField, type HiddenMark, type Link, type Range, type Entry, pivotRowOf, pivotBold, type WholeSel, type DragPtr } from './model';
import { SHEET_ROWS, SHEET_COLS, DEF_W, TOP_H } from './consts';

/** Ce que createMouse lit et modifie dans Sheet.svelte. */
export interface MouseHost {
	readonly structure: boolean;
	readonly toLocal: (e: MouseEvent) => { x: number; y: number; };
	readonly MARGIN: number;
	readonly rowTop: (r: number) => number;
	readonly fRows: () => number;
	readonly vs: VScroll;
	readonly rax: () => AxisLike;
	readonly cax: () => AxisLike;
	readonly fCols: () => number;
	readonly scroller: HTMLDivElement;
	readonly visibleRows: () => { first: number; last: number; };
	readonly entries: Entry[];
	readonly visibleData: (e: Entry, first: number, last: number) => { first: number; last: number; } | null;
	readonly headerWidth: (e: Entry, i: number) => number;
	readonly cellWidth: (text: string, value: unknown, st: CellStyle | undefined) => number;
	readonly styleAt: (r: number, c: number, e?: Entry | undefined) => CellStyle | undefined;
	readonly cells: Map<string, string>;
	readonly shown: Map<string, { text: string; error: boolean; }>;
	readonly pivots: SheetPivot[];
	readonly textWidth: (s: string, bold?: boolean) => number;
	readonly MAX_FIT: number;
	readonly source: SheetSource;
	readonly userWidths: Map<number, number>;
	readonly rebuildAxis: (override?: { col: number; width: number; }) => void;
	readonly updateExtent: () => void;
	readonly schedule: () => void;
	writing: Promise<void>;
	readonly say: (m: string) => void;
	readonly selection: () => CellRange;
	readonly screenX: (c: number) => number;
	readonly screenY: (r: number) => number;
	readonly rh: (r: number) => number;
	moving: {
		sel: { r0: number; r1: number; c0: number; c1: number };
		from: { row: number; col: number };
		to?: { row: number; col: number };
		/** Ctrl enfoncé : copie au lieu de déplacer (Excel). */
		copy?: boolean;
		/** Tableau croisé emporté en entier (sélection prise dans un seul TCD) : sel est sa zone. */
		pivot?: number;
	} | null;
	clipRange: Clip | null;
	active: { row: number; col: number; };
	anchor: { row: number; col: number; };
	message: string;
	readonly refreshFx: () => void;
	readonly refreshStatus: () => void;
	readonly pivotIn: (r0: number, c0: number, r1: number, c1: number) => SheetPivot | undefined;
	readonly queryIn: (r0: number, c0: number, r1: number, c1: number, whole?: boolean) => Entry | undefined;
	readonly QUERY_READONLY: string;
	readonly rangeValues: (sel: ClipRect) => string[][] | null;
	readonly sheetId: () => number;
	readonly snapMerge: (r: number, c: number) => { row: number; col: number; };
	readonly boxOf: (r: number, c: number) => MergeRange | undefined;
	readonly editing: boolean;
	readonly cellInfo: (r: number, c: number) => { e: Entry; i: number; kind: "header"; value: string; n?: undefined; row?: undefined; } | { e: undefined; kind: "free"; value: string; i?: undefined; n?: undefined; row?: undefined; } | { e: Entry; i: number; n: number; row: { id: number; cells: unknown[]; text: string[]; bad?: Map<number, string>; sty?: Map<number, string>; marks?: Set<number>; links?: Map<number, string>; sync?: Map<number, SyncKind>; locked?: boolean; } | undefined; kind: "data"; value: string; };
	readonly rowAtY: (y: number) => number;
	readonly colAtX: (x: number) => number;
	readonly extentCols: number;
	wholeSel: WholeSel | null;
	readonly boxes: MergeRange[];
	readonly panDown: (e: MouseEvent) => boolean;
	hold: FormulaHold | null;
	readonly placing: Placement<PivotConfig> | null;
	readonly cellAt: (e: MouseEvent) => { row: number; col: number; } | null;
	readonly commitEditor: () => boolean;
	readonly wrap: HTMLDivElement;
	readonly placePivotAt: (cell: { row: number; col: number; }) => void;
	links: Link[];
	readonly locked: string;
	selChart: number | null;
	readonly moveTo: (row: number, col: number, extend?: boolean) => void;
	readonly hiddenMarkAt: (e: MouseEvent) => HiddenMark | undefined;
	readonly menuAction: (fn: (() => Promise<unknown>) | undefined, keepFocus?: boolean) => Promise<boolean>;
	filling: { sel: Range; to?: Range } | null;
	readonly listOf: (r: number, c: number) => string[] | undefined;
	readonly dropButton: () => { x: number; y: number; w: number; h: number; };
	listMenu: { x: number; y: number; values: string[]; row: number; col: number; error?: string; loading?: boolean } | null;
	readonly openList: () => void;
	readonly linkButton: () => { x: number; y: number; w: number; h: number; };
	readonly refAt: (r: number, c: number) => { src: string; table: string; key: string; value: string; } | null;
	readonly followRef: (r: number, c: number) => Promise<void>;
	readonly formulaMode: boolean;
	panel: { kind: 'pivot' | 'chart'; id: number } | null;
	readonly tableHintAt: (ev: MouseEvent) => boolean;
	readonly nameBadgeAt: (ev: MouseEvent) => Entry | null;
	readonly closeMenus: () => void;
	readonly renameFromBadge: (en: Entry) => Promise<void>;
	readonly pivotBadgeAt: (ev: MouseEvent) => SheetPivot | null;
	readonly ensureVisible: (row: number, col: number) => void;
	readonly localBadgeAt: (ev: MouseEvent) => Entry | null;
	readonly onaddshared: ((table: string) => void) | undefined;
	readonly followBadgeAt: (ev: MouseEvent) => Entry | null;
	followMenu: { x: number; y: number; e: Entry } | null;
	readonly keyHintAt: (ev: MouseEvent) => Entry | null;
	readonly acceptKeyHint: (e: Entry) => void;
	readonly frozenHeaderAt: (ev: MouseEvent) => { e: Entry; col: number; } | null;
	readonly headerButton: (ev: MouseEvent, e: Entry, c: number, frozen: boolean) => boolean;
	headerDrag: 'rows' | 'cols' | null;
	readonly pivotHeld: (sel: { r0: number; c0: number; r1: number; c1: number; }) => SheetPivot | undefined;
	readonly pivotZone: (p: SheetPivot) => { r0: number; c0: number; r1: number; c1: number; };
	readonly resetRowHeights: (rows: number[]) => (() => Promise<unknown>) | undefined;
	readonly userHeights: Map<number, number>;
	rowResizing: { row: number; startY: number; startH: number; height: number } | null;
	resizing: { col: number; startX: number; startW: number; width: number } | null;
	refDrag: { from: { row: number; col: number }; to: { row: number; col: number } } | null;
	readonly buttonsLayer: ReturnType<typeof SheetButtons> | undefined;
	readonly shownValue: (r: number, c: number) => string;
	readonly openTypeMenu: (ev: MouseEvent, cell: { row: number; col: number; }) => boolean;
	readonly openPivotMenu: (ev: MouseEvent, cell: { row: number; col: number; }) => boolean;
	readonly togglePivotFold: (ev: MouseEvent, cell: { row: number; col: number; }) => boolean;
	readonly pressLink: (e: MouseEvent) => boolean;
	dragging: boolean;
	readonly tableAt: (r: number, c: number) => Entry | undefined;
	readonly pivotAt: (r: number, c: number) => SheetPivot | undefined;
	readonly cellAddr: (r: number, c: number) => string;
	readonly input: HTMLTextAreaElement | undefined;
	readonly fxEditing: boolean;
	readonly fxInput: HTMLInputElement | undefined;
	lastRef: { el: TextField; start: number; end: number; value: string } | null;
	readonly catalog: Catalog | undefined;
	readonly sheetName: string;
	editValueText: string;
	fxText: string;
	readonly onFormulaInput: (el: TextField, where: "cell" | "fx") => void;
	completion: (Completion & { sel: number; where: 'cell' | 'fx' }) | null;
	sig: { fn: FunctionDoc; index: number; where: 'cell' | 'fx' } | null;
	readonly onsheet: ((sheet: number) => void) | undefined;
	refMove: { start: number; end: number; rect: Rect; from: { row: number; col: number }; to: { row: number; col: number } } | null;
	readonly pan: PanGesture<MouseEvent>;
	readonly lockedRowAt: (r: number, c: number) => boolean;
	hoverLocked: number;
	readonly zoomK: () => number;
	readonly rebuildRows: (override?: { row: number; height: number; }) => void;
	placeHover: { row: number; col: number } | null;
	readonly linkUnder: (e: MouseEvent) => { row: number; col: number; url: string; } | null;
	readonly updateHeaderTip: (ev: MouseEvent) => void;
	readonly updateNoteTip: (cell: { row: number; col: number; } | null) => void;
	readonly updateCommentTip: (cell: { row: number; col: number; } | null) => void;
	nameBox: string;
	readonly singleSelection: () => boolean;
	readonly cellName: (r: number, c: number) => string;
	readonly holdWhileScrolling: () => void;
	readonly spacer: HTMLDivElement;
	readonly viewW: number;
	readonly hiddenCols: Set<number>;
	readonly startEdit: (byTyping: boolean, initial?: string) => void;
	readonly pivotActive: () => void;
}

export function createMouse(sh: MouseHost) {

	/** Ligne dont le bord bas est sous la souris (marge de gauche), ou null. */
	function rowBoundaryAt(e: MouseEvent): number | null {
		if (sh.structure) return null; // structure protégée : pas de hauteur à tirer
		const { x, y } = sh.toLocal(e);
		if (x >= sh.MARGIN || y <= TOP_H) return null;
		const py = toGrid(y, TOP_H, sh.rowTop(sh.fRows()), sh.vs.top);
		const row = sh.rax().indexAt(py);
		if (sh.rowTop(row + 1) - py <= 3) return row;
		if (py - sh.rowTop(row) <= 3 && row > 0) return row - 1;
		return null;
	}

	/** Colonne dont le bord droit est sous la souris (bandeau du haut), ou null. */
	function boundaryAt(e: MouseEvent): number | null {
		if (sh.structure) return null; // structure protégée : pas de largeur à tirer
		const { x, y } = sh.toLocal(e);
		if (y >= TOP_H || x <= sh.MARGIN) return null;
		const px = toGrid(x, sh.MARGIN, sh.cax().offset(sh.fCols()), sh.scroller.scrollLeft);
		const c = sh.cax().indexAt(px);
		const right = sh.cax().offset(c + 1);
		const left = sh.cax().offset(c);
		if (right - px <= 4) return c;
		if (px - left <= 4 && c > 0) return c - 1;
		return null;
	}

	/** Largeurs ajustées au contenu affiché des colonnes cols (double-clic sur
	 * le trait de droite d'un en-tête, comme Excel) : en-têtes des tableaux,
	 * lignes visibles seulement (rien de plus n'est lu, même sur un tableau
	 * d'un million de lignes), cellules libres et tableaux croisés. Un seul
	 * passage pour toutes les colonnes (toute la feuille sélectionnée). */
	function fitWidths(cols: number[]): Map<number, number> {
		const want = new Set(cols);
		const w = new Map<number, number>();
		const up = (c: number, v: number) => v > (w.get(c) ?? 0) && w.set(c, v);
		if (!cols.length) return w;
		let lo = cols[0];
		let hi = cols[0];
		for (const c of cols) (lo = Math.min(lo, c)), (hi = Math.max(hi, c));
		const { first, last } = sh.visibleRows();
		for (const e of sh.entries) {
			if (e.lastCol < lo || e.t.col > hi) continue;
			const d = sh.visibleData(e, first, last);
			for (let c = Math.max(lo, e.t.col); c <= Math.min(hi, e.lastCol); c++) {
				if (!want.has(c)) continue;
				const i = c - e.t.col;
				let m = sh.headerWidth(e, i);
				if (d)
					for (let k = d.first; k <= d.last; k++) {
						const row = e.cache.row(k);
						m = Math.max(m, sh.cellWidth(row?.text[i] ?? '', row?.cells[i], sh.styleAt(e.t.row + 1 + k, c, e)));
					}
				up(c, m);
			}
		}
		for (const [k, raw] of sh.cells) {
			const [r, cc] = k.split(',').map(Number);
			if (want.has(cc)) up(cc, sh.cellWidth(sh.shown.get(k)?.text ?? raw, undefined, sh.styleAt(r, cc, undefined)));
		}
		for (const p of sh.pivots) {
			const n = p.cells[0]?.length ?? 0;
			if (p.error || p.col > hi || p.col + n - 1 < lo) continue;
			for (let j = 0; j < n; j++) {
				if (!want.has(p.col + j)) continue;
				p.cells.forEach((r, i) => up(p.col + j, sh.textWidth(r[j] ?? '', pivotBold(p, p.row + i)) + (j === 0 ? pivotLabelOffset(pivotRowOf(p, p.row + i)) : 0)));
			}
		}
		const out = new Map<number, number>();
		for (const c of cols) {
			const v = w.get(c) ?? 0;
			out.set(c, v ? clamp(Math.ceil(v) + 14, 24, sh.MAX_FIT) : DEF_W);
		}
		return out;
	}

	/** Ajuste les colonnes cols à leur contenu, en une seule étape d'annulation. */
	function autoFitColumns(cols: number[]) {
		const one = sh.source.setColumnWidth;
		const many = sh.source.setColumnWidths;
		if (!one && !many) return;
		const widths: { col: number; width: number }[] = [];
		for (const [col, width] of fitWidths(cols)) if (width !== sh.cax().size(col)) widths.push({ col, width });
		if (!widths.length) return;
		for (const { col, width } of widths) sh.userWidths.set(col, width);
		sh.rebuildAxis();
		sh.updateExtent();
		sh.schedule();
		const write = many && (widths.length > 1 || !one) ? () => many(widths) : () => one!(widths[0].col, widths[0].width);
		sh.writing = sh.writing.catch(() => undefined).then(() => write().then(() => undefined, (err) => sh.say(errorText(err))));
	}

	/** La souris est-elle sur la bordure de la sélection (poignée de déplacement) ? */
	function onSelectionEdge(e: MouseEvent): boolean {
		if (!sh.source.transfer) return false;
		const { x, y } = sh.toLocal(e);
		if (x < sh.MARGIN || y < TOP_H) return false;
		const { r0, r1, c0, c1 } = sh.selection();
		const left = sh.screenX(c0);
		const right = sh.screenX(c1) + sh.cax().size(c1);
		const top = sh.screenY(r0);
		const bottom = sh.screenY(r1) + sh.rh(r1);
		const near = (v: number, edge: number) => Math.abs(v - edge) <= 3;
		const inX = x >= left - 3 && x <= right + 3;
		const inY = y >= top - 3 && y <= bottom + 3;
		return (inY && (near(x, left) || near(x, right))) || (inX && (near(y, top) || near(y, bottom)));
	}

	/**
	 * Fin du glisser de la bordure de la sélection, comme Excel : la zone se
	 * déplace (Ctrl : se copie) d'un seul bloc, en une étape d'annulation —
	 * tableaux entiers, colonnes entières, cellules libres et cellules de
	 * tableau mêlés (voir transfer). Posée sur un tableau, elle remplace ses
	 * valeurs ; juste à côté, elle l'étend ou le fusionne ; un tableau
	 * chevauché s'écarte vers la droite.
	 */
	async function dropMove() {
		const m = sh.moving;
		sh.moving = null;
		sh.scroller.style.cursor = '';
		sh.schedule();
		if (!m?.to || !sh.source.transfer) return;
		const dr = m.to.row - m.from.row;
		const dc = m.to.col - m.from.col;
		if (!dr && !dc) return;
		const { r0, r1, c0, c1 } = m.sel;
		const dst = { r0: r0 + dr, c0: c0 + dc, r1: r1 + dr, c1: c1 + dc };
		if (dst.r0 < 0 || dst.c0 < 0) return;
		// Tableau croisé emporté : il se déplace (Ctrl : se copie) en entier ;
		// ce qu'il recouvre s'écarte (le moteur).
		if (m.pivot !== undefined) {
			const op = m.copy ? sh.source.copyPivot : sh.source.movePivot;
			if (!op) return;
			const id = m.pivot;
			sh.clipRange = null;
			sh.writing = sh.writing.catch(() => undefined).then(() =>
				op(id, dst.r0, dst.c0).then(
					() => {
						sh.active = { row: dst.r0, col: dst.c0 };
						sh.anchor = { ...sh.active };
						sh.message = m.copy ? tr('grid.pivot.pasted') : tr('grid.pivot.place.moved');
						sh.refreshFx();
						sh.refreshStatus();
						sh.schedule();
					},
					(err) => sh.say(errorText(err))
				)
			);
			return;
		}
		// Des cellules déposées sur un tableau croisé (ou une partie de lui
		// emportée avec d'autres cellules) : ce n'est pas un déplacement de
		// tableau croisé.
		const hitPivot = sh.pivotIn(dst.r0, dst.c0, dst.r1, dst.c1) ?? (!m.copy ? sh.pivotIn(r0, c0, r1, c1) : undefined);
		if (hitPivot) {
			sh.say(tr('grid.pivot.overlap', { name: hitPivot.config.name ?? '' }));
			return;
		}
		// Un tableau de requête déplacé en entier reste permis ; ses cellules, non.
		const carried = (e: Entry) => !m.copy && e.t.row >= r0 && e.last <= r1 && e.t.col >= c0 && e.lastCol <= c1;
		const hit = sh.entries.some((e) => e.t.query && !carried(e) && e.t.row <= dst.r1 && e.last >= dst.r0 && e.t.col <= dst.c1 && e.lastCol >= dst.c0);
		if (hit || (!m.copy && sh.queryIn(r0, c0, r1, c1, true))) {
			sh.say(sh.QUERY_READONLY);
			return;
		}
		const values = sh.rangeValues(m.sel);
		if (!values) return;
		const copy = !!m.copy;
		const transfer = sh.source.transfer;
		sh.clipRange = null;
		sh.writing = sh.writing.catch(() => undefined).then(async () => {
			try {
				const msg = await transfer(sh.sheetId(), m.sel, dst, !copy, values);
				sh.active = sh.snapMerge(dst.r0, dst.c0);
				sh.anchor = { row: dst.r1, col: dst.c1 };
				sh.message = msg || (copy ? tr('grid.cells.copied') : tr('grid.cells.moved'));
			} catch (err) {
				sh.message = errorText(err);
			}
			sh.refreshFx();
			sh.refreshStatus();
			sh.schedule();
		});
	}

	/** Position (écran) du coin bas droit de la sélection. */
	function fillHandle() {
		let { r1, c1 } = sh.selection();
		const g = sh.boxOf(r1, c1);
		if (g) {
			r1 = g.row + g.rows - 1;
			c1 = g.col + g.cols - 1;
		}
		return { x: sh.screenX(c1) + sh.cax().size(c1), y: sh.screenY(r1) + sh.rh(r1) };
	}

	function onFillHandle(e: MouseEvent): boolean {
		if (sh.editing || !sh.source.paste) return false;
		const p = sh.toLocal(e);
		const h = fillHandle();
		return Math.abs(p.x - h.x) <= 5 && Math.abs(p.y - h.y) <= 5;
	}

	/** Zone étendue par la poignée jusqu'à la cellule visée (une seule direction). */
	function fillTarget(sel: Range, cell: { row: number; col: number }): Range | undefined {
		const down = cell.row - sel.r1;
		const up = sel.r0 - cell.row;
		const right = cell.col - sel.c1;
		const left = sel.c0 - cell.col;
		const best = Math.max(down, up, right, left);
		if (best <= 0) return undefined;
		if (best === down) return { ...sel, r1: cell.row };
		if (best === up) return { ...sel, r0: cell.row };
		if (best === right) return { ...sel, c1: cell.col };
		return { ...sel, c0: cell.col };
	}

	/** Remplit to à partir de sel (séries, dates, mois… voir fill.ts). */
	function doFill(sel: Range, to: Range) {
		if (!sh.source.paste) return;
		const val = (r: number, c: number) => sh.cellInfo(r, c).value;
		let row = 0;
		let col = 0;
		let block: string[][] = [];
		if (to.r1 > sel.r1 || to.r0 < sel.r0) {
			const down = to.r1 > sel.r1;
			const n = down ? to.r1 - sel.r1 : sel.r0 - to.r0;
			const cols: string[][] = [];
			for (let c = sel.c0; c <= sel.c1; c++) {
				const src: string[] = [];
				for (let r = sel.r0; r <= sel.r1; r++) src.push(val(r, c));
				cols.push(down ? fillSeries(src, n) : fillSeries([...src].reverse(), n).reverse());
			}
			block = Array.from({ length: n }, (_, k) => cols.map((x) => x[k]));
			row = down ? sel.r1 + 1 : to.r0;
			col = sel.c0;
		} else {
			const right = to.c1 > sel.c1;
			const n = right ? to.c1 - sel.c1 : sel.c0 - to.c0;
			const w = sel.c1 - sel.c0 + 1;
			const tables = refTables();
			for (let r = sel.r0; r <= sel.r1; r++) {
				const src: string[] = [];
				for (let c = sel.c0; c <= sel.c1; c++) src.push(val(r, c));
				const line = right ? fillSeries(src, n) : fillSeries([...src].reverse(), n).reverse();
				// Les colonnes de tableau citées suivent le décalage (Ventes[Prix] → Ventes[Qté]).
				block.push(
					line.map((v, j) => {
						const dest = right ? sel.c1 + 1 + j : to.c0 + j;
						const from = right ? sel.c0 + (j % w) : sel.c1 - ((sel.c0 - 1 - dest) % w);
						return shiftTableColumns(v, dest - from, tables);
					})
				);
			}
			row = sel.r0;
			col = right ? sel.c1 + 1 : to.c0;
		}
		const paste = sh.source.paste;
		sh.writing = sh.writing.catch(() => undefined).then(async () => {
			try {
				await paste(row, col, block);
				sh.message = tr('grid.fill.series');
			} catch (err) {
				sh.message = errorText(err);
			}
			sh.active = { row: to.r0, col: to.c0 };
			sh.anchor = { row: to.r1, col: to.c1 };
			sh.refreshFx();
			sh.refreshStatus();
			sh.schedule();
		});
	}

	/** Double-clic sur la poignée : recopie jusqu'en bas du tableau voisin. */
	function fillDown() {
		const sel = sh.selection();
		const near = sh.entries.find((e) => e.t.col <= sel.c1 + 1 && e.lastCol >= sel.c0 - 1 && e.t.row <= sel.r1 && e.last > sel.r1);
		if (!near) return;
		doFill(sel, { ...sel, r1: near.last });
	}

	/** Ligne (marge de gauche) ou colonne (bandeau du haut) sous la souris. */
	function headerAt(e: MouseEvent): { kind: 'rows'; row: number } | { kind: 'cols'; col: number } | null {
		const { x, y } = sh.toLocal(e);
		if (x < sh.MARGIN && y >= TOP_H) return { kind: 'rows', row: clamp(sh.rowAtY(y), 0, SHEET_ROWS - 1) };
		if (y < TOP_H && x >= sh.MARGIN) return { kind: 'cols', col: sh.colAtX(x) };
		return null;
	}

	/** Dernière ligne et colonne de la zone affichée (la sélection entière s'y arrête). */
	function lastRow() {
		return Math.max(0, sh.rax().indexAt(sh.vs.content) - 1);
	}
	function lastCol() {
		return Math.max(0, sh.extentCols - 1);
	}

	function wholeKind(): 'rows' | 'cols' | null {
		const w = sh.wholeSel;
		if (!w) return null;
		const same = (a: { row: number; col: number }, b: { row: number; col: number }) => a.row === b.row && a.col === b.col;
		return same(w.anchor, sh.anchor) && same(w.active, sh.active) ? w.kind : null;
	}
	/** Début d'un Maj+clic ou d'un glisser dans la gouttière (rows) ou le bandeau. */
	function wholeFrom(kind: 'rows' | 'cols'): number {
		return wholeKind() === kind ? sh.wholeSel!.from : kind === 'rows' ? sh.active.row : sh.active.col;
	}

	/** Lignes from à to entières ; la cellule active est au début de from (comme Excel). */
	function selectRows(from: number, to: number) {
		const w = wholeLines('rows', from, to, lastCol(), sh.boxes);
		sh.active = w.active;
		sh.anchor = { row: to, col: lastCol() };
		sh.wholeSel = { kind: 'rows', from, range: w.range, anchor: { ...sh.anchor }, active: { ...sh.active } };
		sh.refreshFx();
		sh.refreshStatus();
		sh.schedule();
	}
	function selectCols(from: number, to: number) {
		const w = wholeLines('cols', from, to, lastRow(), sh.boxes);
		sh.active = w.active;
		sh.anchor = { row: lastRow(), col: to };
		sh.wholeSel = { kind: 'cols', from, range: w.range, anchor: { ...sh.anchor }, active: { ...sh.active } };
		sh.refreshFx();
		sh.refreshStatus();
		sh.schedule();
	}

	/** La cellule est-elle dans la sélection en cours ? */
	function inSelection(row: number, col: number) {
		const { r0, r1, c0, c1 } = sh.selection();
		return row >= r0 && row <= r1 && col >= c0 && col <= c1;
	}

	function onMouseDown(e: MouseEvent) {
		if (!sh.scroller) return;
		if (sh.panDown(e)) return;
		if (sh.hold) {
			pointAway(e);
			return;
		}
		// Pose d'un tableau croisé : le clic gauche sur une cellule le pose ;
		// en-têtes, marge et clic droit gardent leur rôle.
		if (sh.placing) {
			const cell = sh.cellAt(e);
			if (placesOnClick(e.button, !!cell && headerAt(e) === null)) {
				e.preventDefault();
				if (!sh.commitEditor()) return;
				sh.wrap.focus();
				sh.placePivotAt(cell!);
				return;
			}
		}
		// Flèches d'audit : un clic dans la grille les efface, même sur la cellule
		// active (le ruban n'a plus de « Supprimer les flèches »).
		if (sh.links.length) {
			sh.links = [];
			sh.schedule();
		}
		// Feuille protégée : comme Excel, un clic place la cellule active (pour
		// lire, copier, figer les volets), sans glisser ni menus de tableau ni
		// redimensionnement.
		if (sh.locked) {
			e.preventDefault();
			sh.wrap.focus();
			if (sh.selChart !== null) sh.selChart = null;
			const cell = e.button === 0 ? sh.cellAt(e) : null;
			if (cell) sh.moveTo(cell.row, cell.col, e.shiftKey);
			return;
		}
		{
			const hm = e.button === 0 ? sh.hiddenMarkAt(e) : undefined;
			if (hm && sh.source.setLinesHidden) {
				e.preventDefault();
				const set = sh.source.setLinesHidden;
				sh.menuAction(() => set(hm.rows, hm.i0, hm.i1, false));
				return;
			}
		}
		if (e.button === 0 && onFillHandle(e)) {
			e.preventDefault();
			if (e.detail === 2) {
				fillDown();
				return;
			}
			sh.filling = { sel: sh.selection() };
			return;
		}
		// Flèche de la liste déroulante (validation) de la cellule active.
		if (e.button === 0 && !sh.editing && sh.listOf(sh.active.row, sh.active.col)) {
			const b = sh.dropButton();
			const { x, y } = sh.toLocal(e);
			if (x >= b.x && x <= b.x + b.w && y >= b.y && y <= b.y + b.h) {
				e.preventDefault();
				if (sh.listMenu) sh.listMenu = null;
				else sh.openList();
				return;
			}
			const l = sh.linkButton();
			if (x >= l.x && x <= l.x + l.w && y >= l.y && y <= l.y + l.h && sh.refAt(sh.active.row, sh.active.col)) {
				e.preventDefault();
				void sh.followRef(sh.active.row, sh.active.col);
				return;
			}
		}
		// Ctrl+clic sur une cellule d'une colonne de référence : basculer sur la ligne citée.
		if (e.button === 0 && (e.ctrlKey || e.metaKey) && !e.shiftKey && !sh.editing && !sh.formulaMode) {
			const cell = sh.cellAt(e);
			if (cell && sh.refAt(cell.row, cell.col)) {
				e.preventDefault();
				sh.wrap.focus();
				sh.moveTo(cell.row, cell.col);
				void sh.followRef(cell.row, cell.col);
				return;
			}
		}
		if (sh.selChart !== null) {
			sh.selChart = null;
			if (sh.panel?.kind === 'chart') sh.panel = null;
		}
		if (e.button === 0 && !sh.formulaMode && sh.tableHintAt(e) && sh.source.formatAsTable) {
			e.preventDefault();
			if (!sh.commitEditor()) return;
			const { row, col } = sh.active;
			const fmt = sh.source.formatAsTable;
			sh.writing = sh.writing.catch(() => undefined).then(() => fmt(row, col).then(
					() => undefined,
					(err) => sh.say(errorText(err))
				)
			);
			return;
		}
		if (e.button === 0 && !sh.formulaMode && sh.nameBadgeAt(e)) {
			e.preventDefault();
			if (!sh.commitEditor()) return;
			const ne = sh.nameBadgeAt(e)!;
			sh.closeMenus();
			void sh.renameFromBadge(ne);
			return;
		}
		if (e.button === 0 && !sh.formulaMode && sh.pivotBadgeAt(e)) {
			// Nom d'un TCD : sa cellule d'en-tête et son volet (où se lit son nom).
			e.preventDefault();
			if (!sh.commitEditor()) return;
			const pb = sh.pivotBadgeAt(e)!;
			sh.closeMenus();
			sh.moveTo(pb.row, pb.col);
			sh.ensureVisible(pb.row, pb.col);
			sh.panel = { kind: 'pivot', id: pb.id };
			return;
		}
		if (e.button === 0 && !sh.formulaMode && sh.localBadgeAt(e)) {
			// « Sur ce poste » : propose de l'ajouter au classeur partagé
			// (confirmation, ou pourquoi pas maintenant).
			e.preventDefault();
			if (!sh.commitEditor()) return;
			const le = sh.localBadgeAt(e)!;
			sh.closeMenus();
			if (!le.t.query) sh.onaddshared?.(le.t.name);
			return;
		}
		if (e.button === 0 && !sh.formulaMode && sh.followBadgeAt(e)) {
			e.preventDefault();
			if (!sh.commitEditor()) return;
			const fe = sh.followBadgeAt(e)!;
			sh.closeMenus();
			const r = sh.wrap.getBoundingClientRect();
			sh.followMenu = { x: e.clientX - r.left, y: e.clientY - r.top, e: fe };
			return;
		}
		if (e.button !== 1 && !sh.formulaMode && sh.keyHintAt(e)) {
			e.preventDefault();
			if (!sh.commitEditor()) return;
			if (e.button === 0) sh.acceptKeyHint(sh.keyHintAt(e)!);
			return;
		}
		const frozen = e.button === 0 && boundaryAt(e) === null ? sh.frozenHeaderAt(e) : null;
		if (frozen) {
			sh.commitEditor();
			if (sh.headerButton(e, frozen.e, frozen.col, true)) {
				e.preventDefault();
				return;
			}
		}
		// Coin en haut à gauche (entre la marge et le bandeau) : toute la
		// feuille, toutes les lignes et toutes les colonnes, comme dans Excel.
		{
			const { x, y } = sh.toLocal(e);
			if (e.button === 0 && x < sh.MARGIN && y < TOP_H) {
				e.preventDefault();
				if (!sh.commitEditor()) return;
				sh.wrap.focus();
				selectRows(0, lastRow());
				return;
			}
		}
		const hd = boundaryAt(e) === null && rowBoundaryAt(e) === null ? headerAt(e) : null;
		if (hd && e.button !== 1) {
			e.preventDefault();
			if (!sh.commitEditor()) return; // saisie refusée : l'éditeur reste ouvert
			sh.wrap.focus();
			// Clic droit dans la sélection : on la garde.
			if (e.button === 2 && (hd.kind === 'rows' ? inSelection(hd.row, sh.active.col) : inSelection(sh.active.row, hd.col))) return;
			if (hd.kind === 'rows') selectRows(e.shiftKey ? wholeFrom('rows') : hd.row, hd.row);
			else selectCols(e.shiftKey ? wholeFrom('cols') : hd.col, hd.col);
			sh.headerDrag = e.button === 0 ? hd.kind : null;
			return;
		}
		if (e.button === 0 && !sh.editing && onSelectionEdge(e)) {
			const cell = sh.cellAt(e);
			if (cell) {
				e.preventDefault();
				// Point saisi ramené dans la sélection : la bordure se prend à
				// quelques pixels de part et d'autre, et une prise juste à
				// l'extérieur décalait le dépôt d'une ligne ou d'une colonne.
				const sel = sh.selection();
				const from = { row: clamp(cell.row, sel.r0, sel.r1), col: clamp(cell.col, sel.c0, sel.c1) };
				// Sélection dans un seul tableau croisé (une cellule, une partie,
				// ou lui en entier) : c'est le tableau croisé entier qui se
				// déplace, comme un objet, son contour suit la souris.
				const pv = sh.pivotHeld(sel);
				if (pv && sh.source.movePivot) {
					sh.moving = { sel: sh.pivotZone(pv), from, pivot: pv.id };
					return;
				}
				sh.moving = { sel, from };
				return;
			}
		}
		const rb = rowBoundaryAt(e);
		if (rb !== null && sh.source.setRowHeight) {
			e.preventDefault();
			if (e.detail === 2) {
				// Double-clic : hauteur automatique ; sur une ligne de la
				// sélection, toutes les lignes sélectionnées (comme Excel).
				const { r0, r1 } = sh.selection();
				const write = sh.resetRowHeights(fitRows(rb, r0, r1, sh.userHeights.keys()));
				if (write) sh.writing = sh.writing.catch(() => undefined).then(() => write().then(() => undefined, (err) => sh.say(errorText(err))));
				return;
			}
			sh.rowResizing = { row: rb, startY: e.clientY, startH: sh.rh(rb), height: sh.rh(rb) };
			return;
		}
		const b = boundaryAt(e);
		if (b !== null && sh.source.setColumnWidth) {
			e.preventDefault();
			sh.resizing = { col: b, startX: e.clientX, startW: sh.cax().size(b), width: sh.cax().size(b) };
			return;
		}
		const cell = sh.cellAt(e);
		if (!cell) return;
		if (e.button === 0 && sh.formulaMode && grabRefMark(e, cell)) {
			e.preventDefault();
			return;
		}
		if (e.button === 0 && sh.formulaMode && insertRef(cell)) {
			e.preventDefault();
			// Glisser ensuite insère une plage (Cell[A1:B3], Tableau[Col][1:3]…).
			sh.refDrag = { from: cell, to: cell };
			return;
		}
		e.preventDefault();
		if (!sh.commitEditor()) return; // saisie refusée : l'éditeur reste ouvert
		// Clic droit dans la sélection : elle reste (le menu porte sur elle).
		if (e.button === 2 && inSelection(cell.row, cell.col)) return;
		// Volet d'un contrôle ouvert, champ actif : le clic y met l'adresse ou la valeur de la cellule.
		if (e.button === 0 && sh.buttonsLayer?.pickCell(cell.row, cell.col, sh.shownValue(cell.row, cell.col))) return;
		if (e.button === 0 && sh.openTypeMenu(e, cell)) return;
		if (e.button === 0 && sh.openPivotMenu(e, cell)) return;
		if (e.button === 0 && sh.togglePivotFold(e, cell)) {
			sh.wrap.focus();
			return;
		}
		// Texte d'un lien : un clic court le suit au relâcher, Ctrl+clic tout de suite.
		if (sh.pressLink(e)) return;
		sh.wrap.focus();
		sh.moveTo(cell.row, cell.col, e.shiftKey);
		sh.dragging = e.button === 0; // clic droit glissé : la feuille glisse (pan.ts)
	}

	/**
	 * Clic sur la grille pendant le pointage d'une formule d'une autre feuille :
	 * la référence au curseur (ou une plage au glissé), rien d'autre ; la saisie
	 * reste ouverte dans la barre de formule.
	 */
	function pointAway(e: MouseEvent) {
		e.preventDefault();
		const cell = boundaryAt(e) === null && rowBoundaryAt(e) === null ? sh.cellAt(e) : null;
		if (!cell || e.button !== 0) return;
		if (grabRefMark(e, cell)) return;
		if (insertRef(cell)) {
			sh.refDrag = { from: cell, to: cell };
			return;
		}
		sh.say(tr('grid.formula.refPlace'));
	}

	/**
	 * Référence d'une cellule dans une formule (nomenclature de Lattis) :
	 * Cell[A1] pour une cellule libre ; dans le tableau de la formule, [Col]
	 * pour sa ligne et [Col][k] pour une autre ; ailleurs Tableau[Col][k]
	 * (k : le rang de la ligne, même si le tableau a une clé) ; un en-tête donne
	 * Tableau[Col], toute la colonne.
	 */
	function refFor(r: number, c: number, kind: '' | 'table' | 'column' = ''): string {
		const e = sh.tableAt(r, c);
		// Tableau croisé : lu comme un tableau nommé (TCD1[Col], [Col] dans
		// ses colonnes saisies), jamais par l'adresse de la cellule (pivotref.ts).
		const pv = e ? undefined : sh.pivotAt(r, c);
		const pref = pv ? pivotRefText(pv, r, c, sh.hold ? null : sh.active, kind) : null;
		if (pref) return pref;
		if (!e) return sh.cellAddr(r, c);
		const col = e.t.columns[c - e.t.col].name;
		// L'argument en cours attend un tableau ou une colonne entière
		// (REFERENCE(Tableau[Colonne]; …), SOMME(T[Col]), EXISTE(Tableau; …)) :
		// le clic dans le tableau donne cette forme, quelle que soit la ligne.
		if (kind === 'table') return e.t.name;
		if (kind === 'column') return `${e.t.name}[${col}]`;
		// Formule d'une autre feuille (pointage) : elle n'est dans aucun tableau
		// d'ici. Sinon [Col] / [Col][-1] dans le tableau de la formule (ou à côté),
		// Tableau[Col][n] depuis une cellule libre : une valeur, pas la colonne
		// (demande de Louis) ; l'en-tête seul donne Tableau[Col] (voir pointedRef).
		return pointedRef(r, c, refTables(), sh.hold ? null : sh.active);
	}

	/** Pendant la saisie d'une formule, insère la référence de la cellule cliquée au curseur. */
	function insertRef(cell: { row: number; col: number }, text?: string): boolean {
		const el = sh.editing ? sh.input : sh.fxEditing ? sh.fxInput : null;
		if (!el) return false;
		if (text === undefined && sh.editing && cell.row === sh.active.row && cell.col === sh.active.col) return false;
		const v = el.value;
		const s0 = el.selectionStart ?? v.length;
		const s1 = el.selectionEnd ?? s0;
		const last = sh.lastRef?.el === el ? sh.lastRef : null;
		const at = insertRefAt(v, s0, s1, '', last);
		if (!at) return false;
		let ref = text ?? refFor(cell.row, cell.col, sh.catalog ? argKindAt(v, at.start, sh.catalog) : '');
		// Formule d'une autre feuille : une cellule libre d'ici prend le nom de la feuille.
		if (sh.hold) ref = qualifyRef(ref, sh.sheetName);
		const r = insertRefAt(v, s0, s1, ref, last)!;
		if (sh.editing) sh.editValueText = r.value;
		else sh.fxText = r.value;
		el.value = r.value;
		el.setSelectionRange(r.end, r.end);
		el.focus();
		sh.lastRef = { el, start: r.start, end: r.end, value: r.value };
		return true;
	}

	/**
	 * Référence pointée (clic ou glissé terminé) : si la fonction attend encore
	 * un argument obligatoire, la saisie passe au suivant (« ; »), comme le
	 * ferait l'utilisateur (choix de Louis) ; pas après le dernier argument.
	 */
	function nextArg() {
		const el = sh.editing ? sh.input : sh.fxEditing ? sh.fxInput : null;
		if (!el || !sh.catalog) return;
		const caret = el.selectionEnd ?? el.value.length;
		if (!needsNextArg(el.value, caret, sh.catalog)) return;
		const r = toNextArg(el.value, caret);
		if (sh.editing) sh.editValueText = r.text;
		else sh.fxText = r.text;
		el.value = r.text;
		el.setSelectionRange(r.caret, r.caret);
		sh.lastRef = null; // le clic suivant ajoute une référence, il ne remplace plus celle-ci
		sh.onFormulaInput(el, sh.editing ? 'cell' : 'fx');
		// Pointage sur une autre feuille : l'argument suivant se saisit dans la
		// feuille de la formule, où l'on revient (demande de Louis).
		if (sh.hold) {
			sh.hold = updateHold(sh.hold, r.text, r.caret, r.caret);
			sh.completion = sh.sig = null;
			// Après la fin du clic (mouseup) : la saisie reprend dans sa cellule sans être validée.
			const back = sh.hold.sheet;
			setTimeout(() => sh.onsheet?.(back));
		}
	}

	/** Contexte des références : tableaux et ligne de la formule. */
	function refContext() {
		const tables = refTables();
		// Formule d'une autre feuille (pointage) : ni tableau ni ligne d'ici.
		if (sh.hold) return { tables, at: undefined };
		const here = sh.tableAt(sh.active.row, sh.active.col);
		const at = here && sh.active.row > here.t.row ? { table: tables[sh.entries.indexOf(here)], row: sh.active.row } : undefined;
		return { tables, at };
	}

	/** Le bord d'un marqueur de référence est-il sous la souris ? */
	function hitRefMark(e: MouseEvent): boolean {
		return markAt(e) !== null;
	}

	/** Attrape le marqueur dont le bord est sous la souris ; faux s'il n'y en a pas. */
	function grabRefMark(e: MouseEvent, cell: { row: number; col: number }): boolean {
		const m = markAt(e);
		if (!m) return false;
		sh.refMove = { start: m.f.start, end: m.f.end, rect: m.k, from: cell, to: cell };
		return true;
	}

	/** Marqueur (référence et plage) dont le bord est à moins de 4 px de la souris. */
	function markAt(e: MouseEvent) {
		const { tables, at } = refContext();
		const { x, y } = sh.toLocal(e);
		for (const f of formulaRefs(sh.editing ? sh.editValueText : sh.fxText)) {
			const k = refRect(f.text, tables, sh.sheetName, at, !!sh.hold);
			if (!k) continue;
			const left = sh.screenX(k.c0);
			const right = sh.screenX(k.c1) + sh.cax().size(k.c1);
			const top = sh.screenY(k.r0);
			const bottom = sh.screenY(k.r1) + sh.rh(k.r1);
			const near = (v: number, edge: number) => Math.abs(v - edge) <= 4;
			const inX = x >= left - 4 && x <= right + 4;
			const inY = y >= top - 4 && y <= bottom + 4;
			if ((inY && (near(x, left) || near(x, right))) || (inX && (near(y, top) || near(y, bottom)))) return { f, k };
		}
		return null;
	}

	/** Dépose le marqueur glissé : la référence est réécrite dans la formule. */
	function dropRefMark() {
		const m = sh.refMove;
		sh.refMove = null;
		sh.schedule();
		if (!m) return;
		const dr = m.to.row - m.from.row;
		const dc = m.to.col - m.from.col;
		if (!dr && !dc) return;
		const { tables, at } = refContext();
		const moved = movedRef(m.rect, dr, dc, tables, at, m.to);
		const text = moved !== null && sh.hold ? qualifyRef(moved, sh.sheetName) : moved;
		if (text === null) {
			sh.say(tr('grid.formula.refRange'));
			return;
		}
		const el = sh.editing ? sh.input : sh.fxEditing ? sh.fxInput : null;
		if (!el) return;
		const next = el.value.slice(0, m.start) + text + el.value.slice(m.end);
		if (sh.editing) sh.editValueText = next;
		else sh.fxText = next;
		el.value = next;
		const caret = m.start + text.length;
		el.setSelectionRange(caret, caret);
		el.focus();
		sh.lastRef = null;
	}

	/** Tableaux de la feuille, pour situer et écrire les références. */
	function refTables(): RefTable[] {
		return sh.entries.map((e) => ({
			name: e.t.name,
			row: e.t.row,
			col: e.t.col,
			columns: e.t.columns.map((c) => c.name),
			rows: e.t.rows,
			keyed: false, // [k] est toujours un rang
			keyRow: (k: string) => {
				if (e.t.key === undefined) return undefined;
				for (let n = 0; n < e.t.rows; n++) {
					const v = e.cache.row(n)?.text[e.t.key];
					if (v !== undefined && (v === k || v.replace(/\s/g, '') === k)) return n;
				}
				return undefined;
			}
		}));
	}

	/** Plage glissée en mode formule : sa référence remplace celle insérée au clic. */
	function dragRef(cell: { row: number; col: number }) {
		if (!sh.refDrag || (cell.row === sh.refDrag.to.row && cell.col === sh.refDrag.to.col)) return;
		sh.refDrag.to = cell;
		const { from } = sh.refDrag;
		const { tables, at } = refContext();
		const hereT = at?.table;
		const pv = sh.pivotAt(from.row, from.col);
		const text =
			from.row === cell.row && from.col === cell.col
				? refFor(cell.row, cell.col)
				: pv && sh.pivotAt(cell.row, cell.col) === pv
					? pivotRangeRef(pv, from.row, from.col, cell.row, cell.col)
					: rangeRef({ r0: from.row, c0: from.col, r1: cell.row, c1: cell.col }, tables, hereT);
		if (text === null) {
			sh.say(tr('grid.formula.refOneColumn'));
			return;
		}
		insertRef(cell, text);
		sh.schedule();
	}

	function onMouseMove(e: MouseEvent) {
		if (sh.pan.panning) return; // la feuille glisse (pan.ts) : ni survol ni sélection
		{
			const hc = sh.entries.some((x) => x.t.access === 'participant') ? sh.cellAt(e) : null;
			const hl = hc && sh.lockedRowAt(hc.row, hc.col) ? hc.row : -1;
			if (hl !== sh.hoverLocked) {
				sh.hoverLocked = hl;
				sh.schedule();
			}
		}
		if (sh.refMove) {
			const cell = sh.cellAt(e);
			if (cell && (cell.row !== sh.refMove.to.row || cell.col !== sh.refMove.to.col)) {
				sh.refMove.to = cell;
				sh.schedule();
			}
			return;
		}
		if (sh.refDrag) {
			const cell = sh.cellAt(e);
			if (cell) dragRef(cell);
			return;
		}
		if (sh.rowResizing) {
			sh.rowResizing.height = Math.max(12, Math.round(sh.rowResizing.startH + (e.clientY - sh.rowResizing.startY) / sh.zoomK()));
			sh.rebuildRows({ row: sh.rowResizing.row, height: sh.rowResizing.height });
			sh.updateExtent();
			sh.schedule();
			return;
		}
		if (sh.resizing) {
			sh.resizing.width = Math.max(24, Math.round(sh.resizing.startW + (e.clientX - sh.resizing.startX) / sh.zoomK()));
			sh.rebuildAxis({ col: sh.resizing.col, width: sh.resizing.width });
			sh.schedule();
			return;
		}
		if (sh.filling) {
			const cell = sh.cellAt(e);
			if (cell) {
				sh.filling.to = fillTarget(sh.filling.sel, cell);
				sh.schedule();
			}
			return;
		}
		if (sh.moving) {
			const cell = sh.cellAt(e);
			sh.moving.copy = e.ctrlKey || e.metaKey;
			// Ctrl : petit + sur le curseur, comme Excel.
			sh.scroller.style.cursor = sh.moving.copy ? 'copy' : 'move';
			if (cell) {
				sh.moving.to = cell;
				sh.schedule();
			}
			return;
		}
		if (sh.headerDrag) {
			const p = sh.toLocal(e);
			if (sh.headerDrag === 'rows') {
				const row = clamp(sh.rowAtY(p.y), 0, SHEET_ROWS - 1);
				selectRows(wholeFrom('rows'), row);
			} else {
				const col = sh.colAtX(Math.max(sh.MARGIN, p.x));
				selectCols(wholeFrom('cols'), col);
			}
			return;
		}
		if (sh.placing) {
			const cell = sh.cellAt(e);
			if (cell?.row !== sh.placeHover?.row || cell?.col !== sh.placeHover?.col) {
				sh.placeHover = cell;
				sh.schedule();
			}
			if (cell && !sh.dragging) {
				sh.scroller.style.cursor = 'crosshair';
				return;
			}
		}
		sh.scroller.style.cursor =
			sh.formulaMode && !sh.refDrag && hitRefMark(e)
				? 'move' // bord d'un marqueur de référence : il se glisse
				: sh.source.setRowHeight && rowBoundaryAt(e) !== null
				? 'row-resize'
				: sh.source.setColumnWidth && boundaryAt(e) !== null
				? 'col-resize'
				: !sh.dragging && onFillHandle(e)
					? 'crosshair'
					: !sh.dragging && onSelectionEdge(e)
						? 'move'
						: headerAt(e)
							? 'pointer'
							: '';
		// Texte d'un lien : la main, comme sur une page web.
		if (!sh.dragging && !sh.scroller.style.cursor && sh.linkUnder(e)) sh.scroller.style.cursor = 'pointer';
		if (!sh.dragging) sh.updateHeaderTip(e);
		sh.updateNoteTip(sh.dragging ? null : sh.cellAt(e));
		sh.updateCommentTip(sh.dragging ? null : sh.cellAt(e));
		if (!sh.dragging) return;
		const cell = sh.cellAt(e);
		if (!cell) return;
		// Glisser : le coin mobile suit la souris, la cellule active reste.
		sh.anchor = cell;
		const sel = sh.selection();
		sh.nameBox = !sh.singleSelection() ? tr('grid.nameBox.size', { rows: String(sel.r1 - sel.r0 + 1), cols: String(sel.c1 - sel.c0 + 1) }) : sh.cellName(sh.active.row, sh.active.col);
		sh.schedule();
	}

	// --- Défilement automatique pendant un glisser (autoscroll.ts) ----------

	// Dernière position de la souris pendant un glisser (coordonnées de l'écran).
	let dragPtr: DragPtr | null = null;
	let autoFrame = 0;
	let autoLast = 0;

	/** Un glisser qui suit la souris sur les cellules est-il en cours ? */
	const cellDrag = () => sh.dragging || !!sh.filling || !!sh.moving || !!sh.refDrag || !!sh.refMove || !!sh.headerDrag;

	/** Zone qui défile, en pixels locaux (hors marge, bandeau et volets figés). */
	function scrollArea() {
		return {
			left: sh.MARGIN + sh.cax().offset(sh.fCols()),
			top: TOP_H + sh.rowTop(sh.fRows()),
			right: sh.scroller.clientWidth,
			bottom: sh.scroller.clientHeight
		};
	}

	/** Défilement voulu (pixels par image, signés) pour la souris p (coordonnées de l'écran). */
	function dragScroll(p: { x: number; y: number }) {
		const r = sh.scroller.getBoundingClientRect();
		const k = sh.zoomK();
		const a = scrollArea();
		// Rien à défiler en largeur si les colonnes figées prennent tout.
		const dx = sh.headerDrag === 'rows' ? 0 : edgeScroll((p.x - r.left) / k, a.left, a.right);
		const dy = sh.headerDrag === 'cols' ? 0 : edgeScroll((p.y - r.top) / k, a.top, a.bottom);
		return { dx: sh.scroller.scrollLeft <= 0 && dx < 0 ? 0 : dx, dy: sh.vs.top <= 0 && dy < 0 ? 0 : dy };
	}

	/**
	 * Rejoue le glisser à la souris ramenée sur la zone des cellules (au bord
	 * qui défile) : hors de la grille, la sélection suit la ligne ou la
	 * colonne du bord, comme Excel.
	 */
	function dragAt(p: DragPtr, dir: { dx: number; dy: number }) {
		const r = sh.scroller.getBoundingClientRect();
		const k = sh.zoomK();
		const a = scrollArea();
		let x = (p.x - r.left) / k;
		let y = (p.y - r.top) / k;
		// Dans le sens du défilement, la souris est ramenée sur la zone qui
		// défile (pas sur les volets figés) ; sinon sur la grille.
		x = clamp(x, dir.dx < 0 ? a.left : sh.MARGIN, sh.scroller.clientWidth - 1);
		y = clamp(y, dir.dy < 0 ? a.top : TOP_H, sh.scroller.clientHeight - 1);
		if (sh.headerDrag === 'rows') x = Math.min(x, sh.MARGIN - 1);
		if (sh.headerDrag === 'cols') y = Math.min(y, TOP_H - 1);
		onMouseMove(new MouseEvent('mousemove', { clientX: r.left + x * k, clientY: r.top + y * k, ctrlKey: p.ctrl, metaKey: p.meta, shiftKey: p.shift, buttons: 1 }));
	}

	/** Souris pendant un glisser, sur toute la fenêtre : hors de la grille, la sélection continue et la feuille défile. */
	function onDragMove(e: MouseEvent) {
		if (!cellDrag() || !sh.scroller) return;
		dragPtr = { x: e.clientX, y: e.clientY, ctrl: e.ctrlKey, meta: e.metaKey, shift: e.shiftKey };
		const dir = dragScroll(dragPtr);
		// Hors de la grille (ou sur un graphique) : la grille ne reçoit pas l'évènement.
		if (!(e.target instanceof Node && sh.scroller.contains(e.target))) dragAt(dragPtr, dir);
		if ((dir.dx || dir.dy) && !autoFrame) {
			autoLast = performance.now();
			autoFrame = requestAnimationFrame(autoTick);
		}
	}

	function stopAutoScroll() {
		cancelAnimationFrame(autoFrame);
		autoFrame = 0;
		dragPtr = null;
	}

	function autoTick(now: number) {
		autoFrame = 0;
		if (!cellDrag() || !dragPtr || !sh.scroller) return stopAutoScroll();
		const dir = dragScroll(dragPtr);
		if (!dir.dx && !dir.dy) return;
		const dt = now - autoLast;
		autoLast = now;
		sh.holdWhileScrolling();
		const dy = frameStep(dir.dy, dt);
		const dx = frameStep(dir.dx, dt);
		if (dy) {
			// Vers le bas, la feuille s'agrandit d'autant (comme à la molette).
			if (dy > 0) sh.vs.content = Math.min(sh.rax().total, Math.max(sh.vs.content, sh.vs.top + dy + 2 * sh.vs.view));
			sh.vs.by(dy);
			sh.updateExtent();
			sh.scroller.scrollTop = sh.vs.toNative();
		}
		if (dx) {
			if (dx > 0 && sh.spacer) {
				const want = sh.cax().indexAt(sh.scroller.scrollLeft + dx + sh.viewW) + 2;
				sh.spacer.style.width = `${sh.MARGIN + sh.cax().offset(Math.min(SHEET_COLS, Math.max(sh.extentCols, want)))}px`;
			}
			sh.scroller.scrollLeft += dx;
			sh.updateExtent();
		}
		dragAt(dragPtr, dir);
		sh.schedule();
		autoFrame = requestAnimationFrame(autoTick);
	}

	function onDblClick(e: MouseEvent) {
		if (sh.hold) return; // pointage d'une formule d'une autre feuille
		if (sh.locked) return; // feuille protégée : pas de saisie, sans message
		const b = boundaryAt(e);
		if (b !== null && (sh.source.setColumnWidth || sh.source.setColumnWidths)) {
			// Poignée d'une colonne de la sélection : toutes les colonnes
			// sélectionnées (toute la feuille comprise), comme Excel.
			const { c0, c1 } = sh.selection();
			autoFitColumns(fitColumns(b, c0, c1, sh.hiddenCols));
			return;
		}
		const cell = sh.cellAt(e);
		const pv = cell && sh.pivotAt(cell.row, cell.col);
		if (cell && pv && pivotHandAt(pv, cell.row, cell.col) >= 0) {
			// Colonne saisie à droite du TCD (en-tête vert compris) : elle se
			// modifie comme une cellule, au lieu d'ouvrir le volet des champs.
			sh.startEdit(false);
			return;
		}
		if (cell && pv) {
			// Double clic sur −/+ : deux bascules, pas le volet.
			if (pv.rows && cell.col === pv.col && pivotBoxHit(pivotRowOf(pv, cell.row), sh.toLocal(e).x - sh.screenX(cell.col))) return;
			sh.pivotActive();
			return;
		}
		if (cell) sh.startEdit(false);
	}

	return {
		autoFitColumns,
		dropMove,
		fillHandle,
		doFill,
		fillDown,
		headerAt,
		lastRow,
		lastCol,
		wholeKind,
		selectRows,
		inSelection,
		onMouseDown,
		nextArg,
		refContext,
		dropRefMark,
		refTables,
		onMouseMove,
		onDragMove,
		stopAutoScroll,
		onDblClick
	};
}
