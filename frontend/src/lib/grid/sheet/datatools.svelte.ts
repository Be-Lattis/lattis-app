// Outils de données de la sélection : convertir (texte en colonnes),
// formules en valeurs, supprimer les doublons. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { type CellRange } from '../mergesel';
import { errorText, t as tr } from '#lib/i18n/index.js';
import { type SheetSource, type Entry } from './model';

/** Ce que createDataTools lit et modifie dans Sheet.svelte. */
export interface DataToolsHost {
	readonly ownerAt: (r: number, c: number) => Entry | undefined;
	readonly active: { row: number; col: number; };
	readonly selectionLines: () => { r0: number; r1: number; c0: number; c1: number; rows: boolean; cols: boolean; };
	readonly entries: Entry[];
	readonly selection: () => CellRange;
	readonly source: SheetSource;
	readonly locked: string;
	readonly say: (m: string) => void;
	readonly closeMenus: () => void;
	ttcDlg: { r0: number; r1: number; col: number; } | null;
	readonly menuAction: (fn: (() => Promise<unknown>) | undefined, keepFocus?: boolean) => Promise<boolean>;
	readonly cells: Map<string, string>;
	dedupDlg: { e: Entry; on: boolean[]; count: number | null; error: string; } | null;
	readonly wrap: HTMLDivElement;
}

export function createDataTools(h: DataToolsHost) {
	/**
	 * Tableau visé par une commande de tableau (TCD, graphique, tri, filtre,
	 * doublons, SQL) : celui de la cellule active ; ou, quand la sélection est
	 * faite de lignes ou de colonnes entières qui ne contiennent qu'un tableau,
	 * ce tableau-là, comme s'il était sélectionné.
	 */
	function targetTable(): Entry | undefined {
		const here = h.ownerAt(h.active.row, h.active.col);
		if (here) return here;
		const sel = h.selectionLines();
		if (!sel.rows && !sel.cols) return undefined;
		const hit = h.entries.filter((e) => (sel.cols ? e.t.col <= sel.c1 && e.lastCol >= sel.c0 : e.t.row <= sel.r1 && e.last >= sel.r0));
		return hit.length === 1 ? hit[0] : undefined;
	}

	/** Colonne visée dans ce tableau : l'active si elle en fait partie, sinon la première colonne sélectionnée du tableau. */
	function targetCol(e: Entry): number {
		if (h.active.col >= e.t.col && h.active.col <= e.lastCol) return h.active.col - e.t.col;
		const { c0 } = h.selection();
		return Math.min(e.lastCol, Math.max(c0, e.t.col)) - e.t.col;
	}

	function textToColumnsActive() {
		if (!h.source.textToColumns || !h.source.textColumnsPreview) return;
		if (h.locked) {
			h.say(h.locked);
			return;
		}
		const { r0, r1, c0, c1 } = h.selection();
		if (c0 !== c1) {
			h.say(tr('grid.textCols.oneColumn'));
			return;
		}
		h.closeMenus();
		h.ttcDlg = { r0, r1, col: c0 };
	}

	/** Formules › Convertir en valeurs : les formules de la sélection (colonnes calculées touchées, cellules libres). */
	function toValuesActive() {
		if (!h.source.rangeToValues) return;
		if (h.locked) {
			h.say(h.locked);
			return;
		}
		const { r0, r1, c0, c1 } = h.selection();
		const run = h.source.rangeToValues;
		h.closeMenus();
		void h.menuAction(() => run(r0, c0, r1, c1));
	}

	/**
	 * Colonnes calculées du tableau que « Convertir en valeurs » prend au clic
	 * droit sur la colonne i : celles de la sélection quand elle la contient,
	 * sinon la seule colonne i.
	 */
	function valueColumns(e: Entry, i: number): number[] {
		const s = h.selection();
		const c = e.t.col + i;
		const inSel = c >= s.c0 && c <= s.c1 && s.r0 <= e.last && s.r1 >= e.t.row;
		const lo = inSel ? Math.max(s.c0, e.t.col) - e.t.col : i;
		const hi = inSel ? Math.min(s.c1, e.lastCol) - e.t.col : i;
		const out: number[] = [];
		for (let k = lo; k <= hi; k++) if (e.t.columns[k]?.formula) out.push(k);
		return out;
	}

	/** La sélection contient-elle une formule à convertir en valeurs (cellule libre, colonne calculée) ? */
	function formulasInSelection(): boolean {
		const s = h.selection();
		for (const [k, raw] of h.cells) {
			if (!raw.startsWith('=')) continue;
			const [r, c] = k.split(',').map(Number);
			if (r >= s.r0 && r <= s.r1 && c >= s.c0 && c <= s.c1) return true;
		}
		return h.entries.some((e) => s.r0 <= e.last && s.r1 >= e.t.row && e.t.columns.some((col, i) => !!col.formula && e.t.col + i >= s.c0 && e.t.col + i <= s.c1));
	}

	function dedupActive() {
		const e = targetTable();
		if (!e || !h.source.removeDuplicates) {
			h.say(tr('grid.table.placeActive'));
			return;
		}
		// Comme Excel : toutes les colonnes cochées au départ.
		h.closeMenus();
		h.dedupDlg = { e, on: e.t.columns.map((c) => !c.formula), count: null, error: '' };
		previewDedup();
	}

	let dedupSeq = 0;
	async function previewDedup() {
		const d = h.dedupDlg;
		if (!d || !h.source.duplicatesPreview) return;
		const cols = d.on.flatMap((on, i) => (on ? [i] : []));
		const seq = ++dedupSeq;
		d.count = null;
		if (!cols.length) {
			d.count = 0;
			return;
		}
		try {
			const n = await h.source.duplicatesPreview(d.e.t, cols);
			if (seq === dedupSeq && h.dedupDlg === d) d.count = n;
		} catch (err) {
			d.error = errorText(err);
		}
	}

	async function applyDedup() {
		const d = h.dedupDlg;
		if (!d || !h.source.removeDuplicates) return;
		const cols = d.on.flatMap((on, i) => (on ? [i] : []));
		try {
			const n = await h.source.removeDuplicates(d.e.t, cols);
			h.dedupDlg = null;
			h.wrap.focus();
			h.say(n ? tr('grid.dedup.removed', { n }) : tr('grid.dedup.none'));
		} catch (err) {
			d.error = errorText(err);
		}
	}

	return {
		textToColumnsActive,
		toValuesActive,
		dedupActive,
		targetTable,
		targetCol,
		valueColumns,
		formulasInSelection,
		previewDedup,
		applyDedup
	};
}
