// Commandes du ruban sur la sélection : Formules (somme automatique, insérer
// une fonction), tri, insérer et supprimer des lignes ou colonnes, clé,
// onglet à choix, objets et boutons. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { clamp } from '../scroll';
import { type AxisLike } from '../anim';
import { type CellStyle } from '../style';
import { type DvList } from '../sheetfmt';
import { type DvAlert, type DvForm } from '../dvrule';
import { type CellRange } from '../mergesel';
import { type Zone } from '../findzone';
import { type FormatTab } from '../FormatDialog.svelte';
import { type KeyChoice, type KeyQuestion } from '../keychoice';
import { dismiss } from '../dismiss';
import { tabChoiceItem, type ChoiceCandidate, type TabChoiceItem } from '../tabchoice';
import { type CondOp, type PanelValue } from '../filtercond';
import { type ObjectKind } from '../objects';
import { type ControlKind, type SheetButton } from '../buttons';
import { type Catalog, type FunctionDoc } from '../autocomplete';
import { type ColumnType } from '../format';
import { errorText, fnName, lower, t as tr } from '#lib/i18n/index.js';
import { type TypeMix, type SheetPivot, type SheetChart, type SheetTable, type SheetData, type LinesOp, type SheetSource, type TextField, type Entry } from './model';
import { SHEET_ROWS, SHEET_COLS } from './consts';

/** Ce que createCommands lit et modifie dans Sheet.svelte. */
export interface CommandsHost {
	fnDlg: { search: string; sel: string | null; } | null;
	readonly wrap: HTMLDivElement;
	readonly startEdit: (byTyping: boolean, initial?: string) => void;
	readonly input: HTMLTextAreaElement | undefined;
	readonly onFormulaInput: (el: TextField, where: "cell" | "fx") => void;
	readonly active: { row: number; col: number; };
	readonly tableAt: (r: number, c: number) => Entry | undefined;
	readonly entries: Entry[];
	readonly catalog: Catalog | undefined;
	readonly targetTable: () => Entry | undefined;
	readonly source: SheetSource;
	readonly say: (m: string) => void;
	readonly menuAction: (fn: (() => Promise<unknown>) | undefined, keepFocus?: boolean) => Promise<boolean>;
	readonly targetCol: (e: Entry) => number;
	readonly selection: () => CellRange;
	readonly lastCol: () => number;
	readonly lastRow: () => number;
	lastRepeat: (() => void) | null;
	readonly hintLines: (rows: boolean, at: number, n: number, table?: number) => void;
	keyDlg: (KeyQuestion & { row: number; col: number; raw: string; keep: boolean; choice: KeyChoice; }) | null;
	readonly write: (row: number, col: number, raw: string, keep?: boolean, onrefused?: (e: unknown) => void, key?: KeyChoice) => Promise<void>;
	readonly reopenFormula: (row: number, col: number, raw: string, msg: string) => void;
	handDlg: { table: string; columns: string[]; choice: string; write?: { row: number; col: number; raw: string; keep: boolean; }; t?: SheetTable; } | null;
	menu: { x: number; y: number; e: Entry; n: number; i: number; header: boolean; rowOnly?: boolean; confirmDelete?: boolean; } | null;
	readonly toView: (v: number) => number;
	readonly screenX: (c: number) => number;
	readonly screenY: (r: number) => number;
	readonly rh: (r: number) => number;
	linesDlg: { x: number; y: number; insert: boolean; choice: "rows" | "cols"; } | null;
	readonly wholeKind: () => "rows" | "cols" | null;
	readonly queryIn: (r0: number, c0: number, r1: number, c1: number, whole?: boolean) => Entry | undefined;
	readonly QUERY_READONLY: string;
	writing: Promise<void>;
	readonly schedule: () => void;
	filterDlg: { e: Entry; i: number; x: number; y: number; values: PanelValue[]; loading?: boolean; search: string; errors: boolean; error: string; touched: boolean; sub: boolean; custom: { conds: { op: CondOp | ""; value: string; to: string; }[]; or: boolean; } | null; pending?: { to: ColumnType; n: number; }; keyWhy?: string | null; mix?: TypeMix | null; } | null;
	cfView: any;
	dedupDlg: { e: Entry; on: boolean[]; count: number | null; error: string; } | null;
	freeMenu: { x: number; y: number; kind?: "rows" | "cols"; } | null;
	chartMenu: { ch: SheetChart; x: number; y: number; } | null;
	formatDlg: { tab: FormatTab; style: CellStyle; sample: { text: string; value: unknown; }; canMerge: boolean; merged: boolean; } | null;
	validDlg: (DvForm & { target: string; error: string; preview: DvList | null; loading: boolean; }) | null;
	readonly dvAlert: (DvAlert & { row: number; col: number; raw: string; keep: boolean; }) | null;
	noteDlg: { row: number; col: number; text: string; had: boolean; pinned: boolean; x: number; y: number; } | null;
	listMenu: { x: number; y: number; values: string[]; row: number; col: number; error?: string; loading?: boolean; } | null;
	sizeDlg: { rows: boolean; value: number; x: number; y: number; } | null;
	commentDlg: { row: number; col: number; x: number; y: number; preview: boolean; } | null;
	readonly data: SheetData;
	readonly choiceCands: ChoiceCandidate[] | null;
	followMenu: { x: number; y: number; e: Entry; } | null;
	pivotDlg: { p: SheetPivot; field: string; kind: "filter" | "rows"; fields?: string[]; x: number; y: number; values: PanelValue[]; loading: boolean; search: string; } | null;
	readonly dvPick: { active: { row: number; col: number; }; anchor: { row: number; col: number; }; whole: { kind: "rows" | "cols"; from: number; range: CellRange; anchor: { row: number; col: number; }; active: { row: number; col: number; }; } | null; sheet: number; back?: boolean; ref?: string | null; } | null;
	noteTip: { x: number; y: number; text: string; key: string; } | null;
	readonly moveTo: (row: number, col: number, extend?: boolean) => void;
	anchor: { row: number; col: number; };
	readonly ensureVisible: (row: number, col: number) => void;
	readonly refreshFx: () => void;
	readonly refreshStatus: () => void;
	readonly scroller: HTMLDivElement;
	readonly zoomK: () => number;
	readonly cax: () => AxisLike;
	readonly objectsLayer: ({ $on?(type: string, callback: (e: any) => void): () => void; $set?(props: Partial<Record<string, any>>): void; } & Record<string, any>) | undefined;
	readonly buttonsLayer: ({ $on?(type: string, callback: (e: any) => void): () => void; $set?(props: Partial<Record<string, any>>): void; } & Record<string, any>) | undefined;
	readonly pivots: SheetPivot[];
}

export function createCommands(h: CommandsHost) {

	/** Commence une formule dans la cellule active (Insérer une fonction, REFERENCE). */
	function startFormula(text: string) {
		h.fnDlg = null;
		h.wrap.focus();
		h.startEdit(true, text);
		if (h.input) h.onFormulaInput(h.input, 'cell');
	}

	/** Somme automatique : SOMME de la colonne du tableau situé juste au-dessus. */
	function autoSum() {
		const { row, col } = h.active;
		const inTbl = h.tableAt(row, col);
		const above = h.entries.filter((e) => col >= e.t.col && col <= e.lastCol && e.last < row).sort((a, b) => b.last - a.last)[0];
		if (!inTbl && above) {
			startFormula(`=${fnName({ en: 'SUM', fr: 'SOMME', de: 'SUMME', es: 'SUMA' })}(${above.t.name}[${above.t.columns[col - above.t.col].name}])`);
			return;
		}
		startFormula(`=${fnName({ en: 'SUM', fr: 'SOMME', de: 'SUMME', es: 'SUMA' })}(`);
	}

	function openFunctions() {
		closeMenus();
		h.fnDlg = { search: '', sel: null };
	}

	/** Fonctions du dialogue : nom (français ou anglais), catégorie ou description. */
	function shownFunctions() {
		const q = h.fnDlg?.search.trim().toLowerCase() ?? '';
		const all = h.catalog?.functions ?? [];
		const hit = (f: FunctionDoc) => [f.name, f.other, f.category ?? '', f.description ?? ''].some((s) => s.toLowerCase().includes(q));
		return (q ? all.filter(hit) : all).slice(0, 200);
	}

	/** Tri du tableau de la cellule active (ruban Données). */
	function sortActive(desc: boolean) {
		const e = h.targetTable();
		if (!e || !h.source.sortTable) {
			h.say(tr('grid.sort.placeActive'));
			return;
		}
		const sort = h.source.sortTable;
		h.menuAction(() => sort(e.t, h.targetCol(e), desc));
	}

	/** Lignes (ou colonnes) entières couvertes par la sélection, comme Excel les insère ou les supprime. */
	function selectionLines(): { r0: number; r1: number; c0: number; c1: number; rows: boolean; cols: boolean } {
		const { r0, r1, c0, c1 } = h.selection();
		return { r0, r1, c0, c1, rows: c0 === 0 && c1 >= h.lastCol(), cols: r0 === 0 && r1 >= h.lastRow() };
	}

	/** Insère ou supprime des lignes ou colonnes entières pour la sélection. */
	function sheetLines(op: LinesOp) {
		if (!h.source.sheetLines) return;
		h.lastRepeat = () => sheetLines(op); // F4
		const { r0, r1, c0, c1 } = h.selection();
		const lines = h.source.sheetLines;
		const args: Record<LinesOp, [number, number]> = {
			insertRows: [r0, r1 - r0 + 1],
			deleteRows: [r0, r1],
			insertCols: [c0, c1 - c0 + 1],
			deleteCols: [c0, c1]
		};
		// Animation : ce qui suit glisse (posée quand l'opération part, dans la file des écritures).
		const hint: Record<LinesOp, () => void> = {
			insertRows: () => h.hintLines(true, r0, r1 - r0 + 1),
			deleteRows: () => h.hintLines(true, r0, -(r1 - r0 + 1)),
			insertCols: () => h.hintLines(false, c0, c1 - c0 + 1),
			deleteCols: () => h.hintLines(false, c0, -(c1 - c0 + 1))
		};
		h.menuAction(() => (hint[op](), lines(op, ...args[op])));
	}

	function applyKeyDialog() {
		const d = h.keyDlg;
		h.keyDlg = null;
		if (!d) return;
		h.wrap.focus();
		void h.write(d.row, d.col, d.raw, d.keep, undefined, d.choice);
	}

	function cancelKeyDialog() {
		const d = h.keyDlg;
		h.keyDlg = null;
		if (!d) return;
		if (d.keep) h.reopenFormula(d.row, d.col, d.raw, tr('grid.key.lookupCancelled'));
		else h.wrap.focus();
	}

	function applyHandDialog() {
		const d = h.handDlg;
		h.handDlg = null;
		if (!d) return;
		h.wrap.focus();
		if (d.write) void h.write(d.write.row, d.write.col, d.write.raw, d.write.keep, undefined, `hand:${d.choice}`);
		else if (d.t && h.source.setHandKey) void h.menuAction(() => h.source.setHandKey!(d.t!, d.choice));
	}

	function cancelHandDialog() {
		h.handDlg = null;
		h.wrap.focus();
	}

	function openHandKey(t: SheetTable) {
		h.menu = null;
		const columns = t.columns.filter((k) => !k.hand && !k.formula).map((k) => k.name);
		// Clé de plusieurs colonnes (« id + Année ») : proposée telle quelle.
		if (t.handKey && t.handKey !== '#' && !columns.includes(t.handKey)) columns.unshift(t.handKey);
		h.handDlg = { table: t.name, columns, choice: t.handKey ?? columns[0] ?? '#', t };
	}

	function openLinesDialog(insert: boolean, x?: number, y?: number) {
		const sel = selectionLines();
		if (sel.rows || sel.cols) {
			sheetLines(insert ? (sel.rows ? 'insertRows' : 'insertCols') : sel.rows ? 'deleteRows' : 'deleteCols');
			return;
		}
		closeMenus();
		const px = x ?? h.toView(h.screenX(h.active.col) + 20);
		const py = y ?? h.toView(h.screenY(h.active.row) + h.rh(h.active.row) + 4);
		h.linesDlg = { x: px, y: py, insert, choice: 'rows' };
	}

	function applyLinesDialog() {
		const d = h.linesDlg;
		h.linesDlg = null;
		if (!d) return;
		sheetLines(d.insert ? (d.choice === 'rows' ? 'insertRows' : 'insertCols') : d.choice === 'rows' ? 'deleteRows' : 'deleteCols');
	}

	/** Insérer ou supprimer la ligne ou la colonne de la cellule active (ruban Accueil › Cellules). */
	function cellsActive(op: 'insertRow' | 'deleteRow' | 'insertColumn' | 'deleteColumn') {
		const e = h.tableAt(h.active.row, h.active.col);
		const sel = selectionLines();
		// Hors tableau, ou lignes / colonnes entières sélectionnées : celles de la feuille.
		if (!e || ((op === 'insertRow' || op === 'deleteRow') && sel.rows) || ((op === 'insertColumn' || op === 'deleteColumn') && sel.cols)) {
			sheetLines(({ insertRow: 'insertRows', deleteRow: 'deleteRows', insertColumn: 'insertCols', deleteColumn: 'deleteCols' } as const)[op]);
			return;
		}
		h.lastRepeat = () => cellsActive(op); // F4
		const n = h.active.row - e.t.row - 1; // -1 : ligne d'en-tête
		const i = h.active.col - e.t.col;
		if (op === 'deleteRow' && n < 0) {
			h.say(tr('grid.rows.headerNoDelete'));
			return;
		}
		const fn = {
			insertRow: h.source.insertRow && (() => tableLines(e, 'insertRow', Math.max(0, n))),
			deleteRow: h.source.deleteRow && (() => tableLines(e, 'deleteRow', n)),
			insertColumn: h.source.insertColumn && (() => tableLines(e, 'insertColumn', i)),
			deleteColumn: h.source.deleteColumn && (() => tableLines(e, 'deleteColumn', i))
		}[op];
		h.menuAction(fn);
	}

	/** Insère ou supprime la ligne n (ou la colonne n) d'un tableau ; ce qui suit glisse à sa place. */
	function tableLines(e: Entry, op: 'insertRow' | 'deleteRow' | 'insertColumn' | 'deleteColumn', n: number): Promise<unknown> {
		const rows = op === 'insertRow' || op === 'deleteRow';
		h.hintLines(rows, rows ? e.t.row + 1 + n : e.t.col + n, op.startsWith('insert') ? 1 : -1, e.t.id);
		return { insertRow: h.source.insertRow, deleteRow: h.source.deleteRow, insertColumn: h.source.insertColumn, deleteColumn: h.source.deleteColumn }[op]!(e.t, n);
	}

	/** Suppr sur des colonnes entières d'un tableau : « Supprimer la colonne » ; faux sinon. */
	function deleteWholeColumns(): boolean {
		const { r0, r1, c0, c1 } = h.selection();
		// Colonnes entières de la feuille (lettres) : les colonnes des tableaux
		// qu'elles traversent sont retirées de ces tableaux (rien d'autre ne
		// bouge) ; un tableau entièrement dedans est laissé à « Effacer ».
		if (h.wholeKind() === 'cols' && h.source.deleteColumn) {
			const cut = h.entries.filter((e) => e.t.col <= c1 && e.lastCol >= c0 && !(e.t.col >= c0 && e.lastCol <= c1));
			if (!cut.length) return false;
			if (cut.some((e) => h.queryIn(e.t.row, Math.max(c0, e.t.col), e.last, Math.min(c1, e.lastCol), true))) {
				h.say(h.QUERY_READONLY);
				return true;
			}
			h.writing = h.writing.catch(() => undefined).then(async () => {
				try {
					// Une seule étape d'annulation pour toutes les colonnes.
					if (h.source.deleteColumnSpans) {
						for (const e of cut) h.hintLines(false, Math.max(c0, e.t.col), -(Math.min(c1, e.lastCol) - Math.max(c0, e.t.col) + 1), e.t.id);
						await h.source.deleteColumnSpans(cut.map((e) => ({ table: e.t.name, from: Math.max(c0, e.t.col) - e.t.col, to: Math.min(c1, e.lastCol) - e.t.col })));
					} else for (const e of cut) for (let c = Math.min(c1, e.lastCol); c >= Math.max(c0, e.t.col); c--) await tableLines(e, 'deleteColumn', c - e.t.col);
					h.schedule();
				} catch (err) {
					h.say(errorText(err));
				}
			});
			return true;
		}
		const e = h.tableAt(r0, c0);
		// En-tête compris ou données seules (Ctrl+Espace), toutes les lignes, au moins une.
		if (!e || !h.source.deleteColumn || e.last <= e.t.row || (r0 !== e.t.row && (r0 !== e.t.row + 1 || r1 === r0)) || r1 < e.last || c1 > e.lastCol || (c0 === e.t.col && c1 === e.lastCol)) return false;
		if (h.queryIn(r0, c0, r1, c1, true)) {
			h.say(h.QUERY_READONLY);
			return true;
		}
		// De droite à gauche : les numéros des colonnes restantes ne bougent pas.
		h.writing = h.writing.catch(() => undefined).then(async () => {
			try {
				// Une seule étape d'annulation ; des colonnes vides au milieu coupent le tableau.
				if (h.source.suppressColumns) {
					h.hintLines(false, c0, -(c1 - c0 + 1), e.t.id);
					await h.source.suppressColumns(e.t, c0 - e.t.col, c1 - e.t.col);
				} else for (let c = c1; c >= c0; c--) await tableLines(e, 'deleteColumn', c - e.t.col);
				h.schedule();
			} catch (err) {
				h.say(errorText(err));
			}
		});
		return true;
	}

	/** Échap ferme le dialogue ouvert, où que soit le focus. */
	function onGlobalKey(e: KeyboardEvent) {
		if (e.key !== 'Escape') return;
		if (!menusOpen()) return;
		closeMenus();
		h.wrap.focus();
	}

	/** Un menu, panneau ou boîte de la grille est-il ouvert ? */
	function menusOpen(): boolean {
		return !!(h.filterDlg || h.cfView || h.dedupDlg || h.fnDlg || h.menu || h.freeMenu || h.chartMenu || h.linesDlg || h.formatDlg || h.validDlg || h.dvAlert || h.noteDlg || h.listMenu || h.sizeDlg || (h.commentDlg && !h.commentDlg.preview));
	}

	/**
	 * Ferme les menus, panneaux et boîtes ouverts sur la grille : un seul à la
	 * fois (ouvrir le ▾ d'une colonne ferme celui d'une autre). Chaque ouverture
	 * passe par ici ; un clic en dehors les ferme aussi (dismiss.ts).
	 */

	// --- « Filtrer l'onglet par cette colonne / ce champ » (tabchoice.ts) ---

	/** L'entrée de menu pour la colonne column du tableau table (field : champ d'un TCD dont table est la source). */
	function tabChoiceFor(table: string, column: string, o: { formula?: boolean; field?: boolean } = {}): TabChoiceItem {
		return tabChoiceItem({ table, column, ...o, choice: h.data?.choice, candidates: h.choiceCands });
	}

	/** Pose ou retire l'onglet à choix (une étape d'annulation). */
	function runTabChoice(table: string, column: string, it: TabChoiceItem) {
		if (it.disabled) return;
		closeMenus();
		const set = h.source.setTabChoice;
		const clear = h.source.clearTabChoice;
		h.menuAction(it.clear ? clear && (() => clear(column)) : set && (() => set(table, column)));
	}

	function closeMenus() {
		h.menu = h.freeMenu = h.chartMenu = h.listMenu = h.followMenu = null;
		h.filterDlg = h.cfView = h.dedupDlg = h.fnDlg = h.linesDlg = h.pivotDlg = null;
		h.formatDlg = h.noteDlg = h.sizeDlg = h.commentDlg = null;
		// Plage source d'une validation en cours de pointage (peut-être sur
		// une autre feuille) : le dialogue reste, retiré, jusqu'à Entrée ou Échap.
		if (!h.dvPick) h.validDlg = null;
		h.noteTip = null;
	}

	/** Plage sélectionnée (lignes et colonnes de la feuille, à partir de 0). */
	function selectedRange(): { r0: number; c0: number; r1: number; c1: number } {
		const s = h.selection();
		return { r0: s.r0, c0: s.c0, r1: s.r1, c1: s.c1 };
	}

	/** Sélectionne une plage (scripts : ui.select ; à partir de 0). */
	function selectRange(r0: number, c0: number, r1: number, c1: number) {
		h.moveTo(r0, c0);
		h.anchor = { row: clamp(r1, 0, SHEET_ROWS - 1), col: clamp(c1, 0, SHEET_COLS - 1) };
		h.ensureVisible(r0, c0);
		h.refreshFx();
		h.refreshStatus();
		h.schedule();
	}

	/** Rectangle à l'écran (pixels de la fenêtre) d'une plage : bulles de la visite guidée (lib/onboarding). */
	function cellBox(r0: number, c0: number, r1: number, c1: number): DOMRect | null {
		if (!h.scroller) return null;
		const k = h.zoomK();
		const r = h.scroller.getBoundingClientRect();
		const x0 = h.screenX(c0);
		const y0 = h.screenY(r0);
		return new DOMRect(r.left + x0 * k, r.top + y0 * k, (h.screenX(c1) + h.cax().size(c1) - x0) * k, (h.screenY(r1) + h.rh(r1) - y0) * k);
	}

	/** Insertion › Image, Formes, Zone de texte : objet posé à la cellule active. */
	function insertObject(kind: ObjectKind) {
		if (kind === 'image') h.objectsLayer?.pickImage();
		else h.objectsLayer?.insert(kind);
	}

	/** Insertion › Contrôles (bouton, case, liste, zone de saisie, sélecteur de fichier) : contrôle à la cellule active. */
	function insertButton(kind: ControlKind = '') {
		if (!h.source.createButton) return;
		h.buttonsLayer?.openEditor(undefined, kind);
	}

	function runButton(b: SheetButton) {
		if (!h.source.runButton) return;
		h.say(tr('grid.controls.running', { name: b.config.name || b.config.label }));
		h.source.runButton(b.id).then(
			(msg) => h.say(typeof msg === 'string' ? msg : ''),
			(err) => h.say(errorText(err))
		);
	}

	/** Évènement d'un contrôle de la feuille : son action et les écouteurs des scripts. */
	function fireControl(b: SheetButton, event: 'click' | 'change', value: string, old: string) {
		if (!h.source.fireControl) return runButton(b);
		h.say(tr('grid.controls.running', { name: b.config.name || b.config.label }));
		h.source.fireControl(b.id, event, value, old).then(
			(msg) => h.say(typeof msg === 'string' ? msg : ''),
			(err) => h.say(errorText(err))
		);
	}

	/** Sélectionne un tableau ou un tableau croisé de la feuille par son nom (onglet SQL). */
	function revealObject(name: string): boolean {
		const lower = name.toLowerCase();
		const e = h.entries.find((x) => x.t.name.toLowerCase() === lower);
		if (e) {
			h.moveTo(e.t.row, e.t.col);
			h.anchor = { row: e.last, col: e.lastCol };
			h.ensureVisible(e.t.row, e.t.col);
			h.refreshFx();
			h.schedule();
			return true;
		}
		const p = h.pivots.find((x) => (x.config.name ?? '').toLowerCase() === lower);
		if (p) {
			h.moveTo(p.row, p.col);
			return true;
		}
		return false;
	}

	return {
		startFormula,
		autoSum,
		openFunctions,
		sortActive,
		sheetLines,
		cellsActive,
		selectedRange,
		selectRange,
		cellBox,
		insertObject,
		insertButton,
		revealObject,
		shownFunctions,
		selectionLines,
		applyKeyDialog,
		cancelKeyDialog,
		applyHandDialog,
		cancelHandDialog,
		openHandKey,
		openLinesDialog,
		applyLinesDialog,
		tableLines,
		deleteWholeColumns,
		onGlobalKey,
		tabChoiceFor,
		runTabChoice,
		closeMenus,
		runButton,
		fireControl
	};
}
