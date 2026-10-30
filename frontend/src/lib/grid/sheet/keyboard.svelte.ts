// Clavier de la grille : déplacements (Tab, Entrée, flèches, Ctrl+flèche),
// raccourcis d'un tableur, recopie (Ctrl+D, Ctrl+R). Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { type SyncKind } from '../cache';
import { VScroll, clamp } from '../scroll';
import { type Clip } from '../clipboard';
import { type PasteKind } from '../pastespecial';
import { skipHidden, type MergeRange } from '../sheetfmt';
import { extendCorner, type Cell, type CellRange } from '../mergesel';
import { jumpLine, lineSpans, type Area, type Span } from '../jump';
import { type FormatTab } from '../FormatDialog.svelte';
import { copyFillBlock, copyFillPlan, readKey, shortcutOf, timeText, todayText } from '../shortcuts';
import { type FormulaHold } from '../crossref';
import { type OutlineAction } from '../outline';
import { errorText, t as tr } from '#lib/i18n/index.js';
import { type SheetPivot, type ChartConfig, type SheetChart, type SheetSource, type Link, type Entry, type DvPick } from './model';
import { SHEET_ROWS, SHEET_COLS, ROW_H, TOP_H } from './consts';

/** Ce que createKeyboard lit et modifie dans Sheet.svelte. */
export interface KeyboardHost {
	readonly boxOf: (r: number, c: number) => MergeRange | undefined;
	active: { row: number; col: number; };
	readonly step: (dr: number, dc: number) => Cell;
	anchor: { row: number; col: number; };
	readonly hiddenRows: Set<number>;
	readonly hiddenCols: Set<number>;
	message: string;
	readonly ensureVisible: (row: number, col: number) => void;
	readonly refreshFx: () => void;
	readonly refreshStatus: () => void;
	readonly entries: Entry[];
	readonly topRow: (t: { row: number; twoLevel?: boolean; }) => number;
	readonly cells: Map<string, string>;
	readonly spills: { row: number; col: number; rows: string[][] }[];
	readonly pivots: SheetPivot[];
	readonly toLocal: (e: MouseEvent) => { x: number; y: number; };
	readonly MARGIN: number;
	readonly scroller: HTMLDivElement;
	readonly rowAtY: (y: number) => number;
	readonly colAtX: (x: number) => number;
	readonly startEdit: (byTyping: boolean, initial?: string) => void;
	readonly autoSum: () => void;
	readonly openFormat: (tab?: FormatTab) => void;
	readonly tableAt: (r: number, c: number) => Entry | undefined;
	readonly schedule: () => void;
	readonly activeRect: () => CellRange;
	readonly selectRows: (from: number, to: number) => void;
	readonly toggleFilters: () => void;
	readonly editActive: (what: "fillDown" | "fillRight" | "clearContents" | "clearFilters" | "formatAsTable") => void;
	readonly source: SheetSource;
	readonly applyStyle: (patch: Record<string, unknown>) => void;
	readonly hideLines: (rows: boolean, hide: boolean) => void;
	readonly editNote: () => void;
	readonly newComment: () => void;
	readonly openFunctions: () => void;
	readonly chartActive: (kind: ChartConfig["kind"]) => void;
	readonly pasteSpecial: (kind?: PasteKind) => Promise<void>;
	readonly outlineAction: (what: OutlineAction) => void;
	lastRepeat: (() => void) | null;
	readonly say: (m: string) => void;
	readonly selection: () => CellRange;
	readonly cellInfo: (r: number, c: number) => { e: Entry; i: number; kind: "header"; value: string; n?: undefined; row?: undefined; } | { e: undefined; kind: "free"; value: string; i?: undefined; n?: undefined; row?: undefined; } | { e: Entry; i: number; n: number; row: { id: number; cells: unknown[]; text: string[]; bad?: Map<number, string>; sty?: Map<number, string>; marks?: Set<number>; links?: Map<number, string>; sync?: Map<number, SyncKind>; locked?: boolean; } | undefined; kind: "data"; value: string; };
	writing: Promise<void>;
	noAnimUntil: number;
	readonly editing: boolean;
	readonly spaceKey: (e: KeyboardEvent) => boolean;
	readonly dvPick: DvPick | null;
	readonly endPick: (keep: boolean) => void;
	readonly locked: string;
	readonly hold: FormulaHold | null;
	readonly keyContextMenu: () => void;
	painter: Record<string, unknown> | null;
	links: Link[];
	clipRange: Clip | null;
	selChart: number | null;
	readonly charts: SheetChart[];
	readonly removeChart: (id: number) => Promise<void>;
	panel: { kind: 'pivot' | 'chart'; id: number } | null;
	readonly nudgeChart: (ch: SheetChart, dx: number, dy: number, resize: boolean) => void;
	readonly selectChart: (ch: SheetChart) => void;
	readonly vs: VScroll;
	readonly boxes: MergeRange[];
	readonly listOf: (r: number, c: number) => string[] | undefined;
	readonly openList: () => void;
	readonly onrefback: (() => void) | undefined;
	readonly openFinder: (replace: boolean) => void;
	readonly editLink: () => void;
	readonly openLinesDialog: (insert: boolean, x?: number, y?: number) => void;
	readonly deleteWholeColumns: () => boolean;
	readonly clearSelection: (range?: CellRange, all?: boolean) => Promise<void>;
}

export function createKeyboard(sh: KeyboardHost) {
	// --- Sélection et clavier ---------------------------------------------

	/** Bord de la cellule active dans le sens du déplacement (au-delà de sa fusion). */
	function mergeEdge(dr: number, dc: number) {
		const g = sh.boxOf(sh.active.row, sh.active.col);
		if (!g) return { row: sh.active.row, col: sh.active.col };
		return { row: dr > 0 ? g.row + g.rows - 1 : sh.active.row, col: dc > 0 ? g.col + g.cols - 1 : sh.active.col };
	}

	// Saisie d'une ligne avec Tab : Entrée revient sous la cellule de départ, comme Excel.
	let tabStart: number | null = null;
	function tabMove(back: boolean) {
		const start = tabStart ?? sh.active.col;
		const n = sh.step(0, back ? -1 : 1);
		moveTo(n.row, n.col);
		tabStart = start;
	}
	function enterMove(back: boolean) {
		const n = sh.step(back ? -1 : 1, 0);
		moveTo(n.row, tabStart ?? n.col);
	}

	/**
	 * Va à la cellule (row, col) ; extend (Maj) : y porte le coin mobile de la
	 * sélection, la cellule active ne bouge pas (comme Excel).
	 */
	function moveTo(row: number, col: number, extend = false) {
		tabStart = null;
		row = clamp(row, 0, SHEET_ROWS - 1);
		col = clamp(col, 0, SHEET_COLS - 1);
		// Lignes et colonnes masquées : on passe au-delà, dans le sens du déplacement.
		const from = extend ? sh.anchor : sh.active;
		row = skipHidden(row, Math.sign(row - from.row), sh.hiddenRows, SHEET_ROWS);
		col = skipHidden(col, Math.sign(col - from.col), sh.hiddenCols, SHEET_COLS);
		if (extend) sh.anchor = { row, col };
		else {
			// Cellules fusionnées : la cellule en haut à gauche, comme Excel.
			const g = sh.boxOf(row, col);
			if (g) {
				row = g.row;
				col = g.col;
			}
			sh.active = { row, col };
			sh.anchor = { ...sh.active };
		}
		sh.message = '';
		sh.ensureVisible(row, col);
		sh.refreshFx();
		sh.refreshStatus();
	}

	// Ctrl + flèche, comme Excel : au bord du bloc de données, ou au prochain
	// bloc (tableaux, cellules libres, déversements, tableaux croisés).
	function jump(row: number, col: number, dr: number, dc: number): [number, number] {
		const areas: Area[] = [];
		const add = (r0: number, c0: number, r1: number, c1: number) => areas.push({ r0, c0, r1, c1 });
		// Un tableau : de sa bande des groupes à sa dernière ligne (voir jump.ts).
		for (const e of sh.entries) add(sh.topRow(e.t), e.t.col, e.last, e.lastCol);
		for (const k of sh.cells.keys()) {
			const [r, c] = k.split(',').map(Number);
			add(r, c, r, c);
		}
		for (const sp of sh.spills) add(sp.row, sp.col, sp.row + sp.rows.length - 1, sp.col + (sp.rows[0]?.length ?? 1) - 1);
		for (const p of sh.pivots) if (!p.error) add(p.anchor ?? p.row, p.col, p.row + p.cells.length - 1, p.col + (p.cells[0]?.length ?? 1) - 1);
		const spans: Span[] = lineSpans(areas, row, col, dr !== 0);
		return dr !== 0 ? [jumpLine(spans, row, dr), col] : [row, jumpLine(spans, col, dc)];
	}

	function cellAt(e: MouseEvent): { row: number; col: number } | null {
		const { x, y } = sh.toLocal(e);
		if (x < sh.MARGIN || y < TOP_H || x >= sh.scroller.clientWidth || y >= sh.scroller.clientHeight) return null;
		return { row: sh.rowAtY(y), col: sh.colAtX(x) };
	}

	// --- Raccourcis d'Excel -------------------------------------------------------

	// Fin pressée : la flèche suivante va au bout de la zone.
	let endArmed = false;

	/** Exécute un raccourci de la grille ; faux s'il ne s'applique pas. */
	function runShortcut(sc: string): boolean {
		switch (sc) {
			case 'fillDown':
			case 'fillRight':
				copyFill(sc === 'fillDown');
				return true;
			case 'today':
			case 'now':
				sh.startEdit(true, sc === 'today' ? todayText() : timeText());
				return true;
			case 'autoSum':
				sh.autoSum();
				return true;
			case 'formatCells':
				// Ctrl+1 (shortcuts.ts, claviers AZERTY compris).
				sh.openFormat();
				return true;
			case 'selectRow': {
				const t = sh.tableAt(sh.active.row, sh.active.col);
				if (t && sh.active.row > t.t.row) {
					// Dans un tableau : sa ligne, comme Excel.
					sh.active = { row: sh.active.row, col: t.t.col };
					sh.anchor = { row: sh.active.row, col: t.lastCol };
					sh.refreshFx();
					sh.refreshStatus();
					sh.schedule();
				} else {
					// Cellule fusionnée : toutes ses lignes (Excel).
					const ar = sh.activeRect();
					sh.selectRows(ar.r0, ar.r1);
				}
				return true;
			}
			case 'toggleFilter':
				sh.toggleFilters();
				return true;
			case 'formatTable':
				sh.editActive('formatAsTable');
				return true;
			case 'strike':
				if (sh.source.applyStyle) sh.applyStyle({ x: 'toggle' });
				return true;
			case 'hideRows':
			case 'hideCols':
				sh.hideLines(sc === 'hideRows', true);
				return true;
			case 'note':
				sh.editNote();
				return true;
			case 'comment':
				sh.newComment();
				return true;
			case 'insertFunction':
				sh.openFunctions();
				return true;
			case 'chart':
				sh.chartActive('bar');
				return true;
			case 'pasteSpecial':
				void sh.pasteSpecial();
				return true;
			case 'group':
			case 'ungroup':
				sh.outlineAction(sc);
				return true;
			case 'repeat':
				if (sh.lastRepeat) sh.lastRepeat();
				else sh.say(tr('grid.repeat.nothing'));
				return true;
		}
		return false;
	}

	/** Ctrl+D / Ctrl+R : recopie la première ligne (colonne) de la sélection, ou celle du dessus (de gauche). */
	function copyFill(down: boolean) {
		if (!sh.source.paste) return;
		const plan = copyFillPlan(sh.selection(), down);
		if (!plan) return;
		const { dst } = plan;
		if ((dst.r1 - dst.r0 + 1) * (dst.c1 - dst.c0 + 1) > 10000) {
			sh.say(tr('grid.fill.limit'));
			return;
		}
		const block = copyFillBlock(plan, down, (r, c) => sh.cellInfo(r, c).value);
		const paste = sh.source.paste;
		sh.lastRepeat = () => copyFill(down);
		sh.writing = sh.writing.catch(() => undefined).then(async () => {
			try {
				await paste(dst.r0, dst.c0, block);
				sh.message = down ? tr('grid.fill.down') : tr('grid.fill.right');
			} catch (err) {
				sh.message = errorText(err);
			}
			sh.refreshFx();
			sh.refreshStatus();
			sh.schedule();
		});
	}

	function onKeyDown(e: KeyboardEvent) {
		// Touche maintenue : le cadre suit sans animation.
		if (e.repeat) sh.noAnimUntil = performance.now() + 150;
		if (sh.editing) return;
		if (sh.spaceKey(e)) return;
		// Plage source d'une validation pointée : Entrée la reprend, Échap
		// annule ; on ne saisit rien dans la grille, on s'y déplace seulement.
		if (sh.dvPick) {
			if (e.key === 'Enter' || e.key === 'Escape') {
				e.preventDefault();
				sh.endPick(e.key === 'Enter');
				return;
			}
			const mod = e.ctrlKey || e.metaKey;
			if ((e.key.length === 1 && (!mod || /^[vxzyd]$/i.test(e.key))) || e.key === 'F2' || e.key === 'Delete' || e.key === 'Backspace') {
				e.preventDefault();
				return;
			}
		}
		// Feuille protégée : déplacement et lecture seulement (Rechercher,
		// copier, annuler, recalculer, comme le ruban), pas de saisie, sans
		// message ; les raccourcis de la page (Ctrl+S, onglets…) passent.
		if (sh.locked && !sh.hold && !readKey(e)) return;
		// Maj+F10 ou touche Menu : menu contextuel de la cellule active (ou du graphique sélectionné).
		if ((e.shiftKey && e.key === 'F10' && !e.ctrlKey && !e.altKey) || e.key === 'ContextMenu') {
			e.preventDefault();
			sh.keyContextMenu();
			return;
		}
		if (e.key === 'Escape' && sh.painter) sh.painter = null;
		if (e.key === 'Escape' && sh.links.length) {
			sh.links = [];
			sh.schedule();
		}
		if (e.key === 'Escape' && sh.clipRange) {
			sh.clipRange = null;
			sh.schedule();
		}
		if (sh.selChart !== null) {
			const ch = sh.charts.find((c) => c.id === sh.selChart);
			if (ch && (e.key === 'Delete' || e.key === 'Backspace')) {
				e.preventDefault();
				sh.removeChart(ch.id);
				return;
			}
			if (e.key === 'Escape') {
				sh.selChart = null;
				if (sh.panel?.kind === 'chart') sh.panel = null;
				return;
			}
			// Clavier : flèches pour déplacer, Maj+flèches pour redimensionner
			// (10 pixels), Tab et Maj+Tab pour passer au graphique suivant.
			const step = ({ ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1] } as Record<string, number[]>)[e.key];
			if (ch && step && !e.ctrlKey && !e.altKey && !e.metaKey && !sh.locked) {
				e.preventDefault();
				sh.nudgeChart(ch, step[0] * 10, step[1] * 10, e.shiftKey);
				return;
			}
			if (ch && e.key === 'Tab' && sh.charts.length > 1) {
				e.preventDefault();
				const i = sh.charts.indexOf(ch);
				sh.selectChart(sh.charts[(i + (e.shiftKey ? sh.charts.length - 1 : 1)) % sh.charts.length]);
				sh.say(tr('grid.chart.nth', { n: sh.charts.indexOf(sh.charts.find((c) => c.id === sh.selChart)!) + 1, total: sh.charts.length }));
				return;
			}
		}
		// Raccourcis d'Excel (shortcuts.ts) ; Fin puis flèche saute comme Ctrl+flèche.
		const ended = endArmed;
		if (!['Shift', 'Control', 'Alt', 'Meta'].includes(e.key)) endArmed = false;
		const sc = shortcutOf(e);
		if (sc === 'nextSheet' || sc === 'prevSheet' || sc === 'insertSheet' || sc === 'nextChoice' || sc === 'prevChoice') return; // onglets : la page s'en charge
		if (sc === 'refresh' || sc === 'refreshAll') return; // F5 : la page s'en charge (tableHere)
		if (sc === 'endMode') {
			endArmed = !ended;
			sh.say(endArmed ? tr('grid.endMode') : '');
			e.preventDefault();
			return;
		}
		if (sc && runShortcut(sc)) {
			e.preventDefault();
			return;
		}
		const page = Math.max(1, Math.floor(sh.vs.view / ROW_H) - 1);
		const { row, col } = sh.active;
		const ext = e.shiftKey;
		const ctrl = e.ctrlKey || e.metaKey;
		// Maj : le coin mobile de la sélection bouge, la cellule active reste.
		const cur = ext ? sh.anchor : sh.active;
		const arrow = (dr: number, dc: number) => {
			if (ctrl || ended) {
				const from = ext ? sh.anchor : mergeEdge(dr, dc);
				const [r, c] = jump(from.row, from.col, dr, dc);
				moveTo(r, c, ext);
			} else if (ext) {
				// Le bord de la plage (agrandie aux fusions) avance d'une cellule.
				const k = extendCorner(sh.active, sh.anchor, dr, dc, sh.boxes, SHEET_ROWS - 1, SHEET_COLS - 1);
				moveTo(k.row, k.col, true);
			} else {
				const n = sh.step(dr, dc);
				moveTo(n.row, n.col);
			}
		};
		// Alt+↓ : liste déroulante de la cellule (validation).
		if (e.altKey && e.key === 'ArrowDown' && sh.listOf(row, col)) {
			e.preventDefault();
			sh.openList();
			return;
		}
		// Alt+← : revenir d'où l'on a basculé (colonne de référence), comme un navigateur.
		if (e.altKey && !ctrl && !e.shiftKey && e.key === 'ArrowLeft' && sh.onrefback) {
			e.preventDefault();
			sh.onrefback();
			return;
		}
		if (ctrl && (e.key === 'z' || e.key === 'Z') && sh.source.undo) {
			e.preventDefault();
			sh.source.undo().catch((err) => sh.say(errorText(err)));
			return;
		}
		if (ctrl && (e.key === 'a' || e.key === 'A' || e.key === ' ')) {
			const t = sh.tableAt(row, col);
			if (t) {
				e.preventDefault();
				sh.active = { row: t.t.row, col: e.key === ' ' ? col : t.t.col };
				sh.anchor = { row: t.last, col: e.key === ' ' ? col : t.lastCol };
				sh.refreshFx();
				sh.refreshStatus();
				sh.schedule();
				return;
			}
		}
		if (ctrl && (e.key.toLowerCase() === 'f' || e.key.toLowerCase() === 'h') && (sh.source.find || sh.source.search)) {
			e.preventDefault();
			sh.openFinder(e.key.toLowerCase() === 'h');
			return;
		}
		if (ctrl && ['b', 'i', 'u'].includes(e.key.toLowerCase()) && sh.source.applyStyle) {
			e.preventDefault();
			sh.applyStyle({ [e.key.toLowerCase()]: 'toggle' });
			return;
		}
		// Ctrl+K : lien hypertexte de la cellule active.
		if (ctrl && !e.shiftKey && !e.altKey && e.key.toLowerCase() === 'k' && sh.source.setLink) {
			e.preventDefault();
			sh.editLink();
			return;
		}
		// Ctrl + et Ctrl - : insérer ou supprimer des lignes ou colonnes (comme Excel).
		if (ctrl && (e.key === '+' || (e.key === '=' && e.shiftKey) || e.key === '-') && sh.source.sheetLines) {
			e.preventDefault();
			sh.openLinesDialog(e.key !== '-');
			return;
		}
		if (ctrl && (e.key === 'y' || e.key === 'Y') && sh.source.redo) {
			e.preventDefault();
			sh.source.redo().catch((err) => sh.say(errorText(err)));
			return;
		}
		switch (e.key) {
			case 'ArrowDown':
				arrow(1, 0);
				break;
			case 'ArrowUp':
				arrow(-1, 0);
				break;
			case 'ArrowRight':
				arrow(0, 1);
				break;
			case 'ArrowLeft':
				arrow(0, -1);
				break;
			case 'PageDown':
				moveTo(cur.row + page, cur.col, ext);
				break;
			case 'PageUp':
				moveTo(cur.row - page, cur.col, ext);
				break;
			case 'Home':
				moveTo(ctrl ? 0 : cur.row, 0, ext);
				break;
			case 'End':
				if (ctrl) {
					// Dernière ligne et dernière colonne utilisées (tableaux et cellules libres), comme Excel.
					let last = sh.entries.reduce((m, e) => Math.max(m, e.last), 0);
					let lastCol = sh.entries.reduce((m, e) => Math.max(m, e.lastCol), 0);
					for (const k of sh.cells.keys()) {
						const [r, c] = k.split(',').map(Number);
						last = Math.max(last, r);
						lastCol = Math.max(lastCol, c);
					}
					moveTo(last, lastCol, ext);
				}
				break;
			case 'Tab':
				tabMove(e.shiftKey);
				break;
			case 'Enter':
				enterMove(e.shiftKey);
				break;
			case 'F2':
				sh.startEdit(false);
				break;
			case 'F9':
				if (sh.source.recalc) sh.source.recalc().then(() => sh.say(tr('grid.recalculated')), (err) => sh.say(errorText(err)));
				break;
			case 'Delete':
				// Colonnes entières d'un tableau (en-tête compris), pas tout le tableau : supprimées.
				if (sh.deleteWholeColumns()) break;
				sh.clearSelection();
				sh.lastRepeat = () => sh.clearSelection();
				break;
			case 'Backspace':
				sh.startEdit(true, '');
				break;
			default:
				if (e.key.length === 1 && !ctrl && !e.altKey) sh.startEdit(true, e.key);
				else return;
		}
		e.preventDefault();
	}

	return {
		tabMove,
		enterMove,
		moveTo,
		jump,
		cellAt,
		copyFill,
		onKeyDown
	};
}
