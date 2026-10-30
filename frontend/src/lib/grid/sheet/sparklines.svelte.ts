// Graphiques sparkline : insertion (Insertion › Sparklines), modification,
// suppression, rendu pour l'impression. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { type AxisLike } from '../anim';
import { type CellRange } from '../mergesel';
import { a1 as sparkA1, defaultLocation, sparkGeometry, sparkSvg, type SparkKind, type SparklineConfig, type SparklineGroup } from '../sparkline';
import type { PrintCell } from '#lib/print/sheetprint.js';
import { type SheetSource, type Entry } from './model';

/** Ce que createSparklines lit et modifie dans Sheet.svelte. */
export interface SparklinesHost {
	readonly sparkGroups: SparklineGroup[];
	readonly sparkAt: (r: number, c: number) => { g: SparklineGroup; values: (number | null)[]; } | null;
	readonly cax: () => AxisLike;
	readonly rh: (r: number) => number;
	readonly source: SheetSource;
	readonly active: { row: number; col: number; };
	readonly entries: Entry[];
	sparkDlg: { config: SparklineConfig; columns: string[]; id?: number } | null;
	readonly selection: () => CellRange;
	readonly tableAt: (r: number, c: number) => Entry | undefined;
	readonly wrap: HTMLDivElement;
}

export function createSparklines(sh: SparklinesHost) {
	/** Cellule imprimée avec son graphique sparkline (SVG). */
	function printSpark(cell: PrintCell | undefined, r: number, c: number): PrintCell | undefined {
		if (!sh.sparkGroups.length) return cell;
		const s = sh.sparkAt(r, c);
		if (!s || s.g.error) return cell;
		const w = sh.cax().size(c);
		const h = sh.rh(r);
		const svg = sparkSvg(sparkGeometry(s.values, s.g.config, w, h, s.g.config.same ? { lo: s.g.lo, hi: s.g.hi } : undefined), w, h);
		return { ...(cell ?? { text: '' }), spark: svg };
	}

	function insertSparkline(kind: SparkKind) {
		if (!sh.source.createSparkline) return;
		const here = sh.sparkAt(sh.active.row, sh.active.col);
		if (here) {
			const e = here.g.config.table ? sh.entries.find((x) => x.t.name === here.g.config.table) : undefined;
			sh.sparkDlg = { config: { ...here.g.config }, columns: e?.t.columns.map((k) => k.name) ?? [], id: here.g.id };
			return;
		}
		const { r0, r1, c0, c1 } = sh.selection();
		const e = sh.tableAt(r0, c0);
		if (e && r0 > e.t.row && c1 <= e.lastCol) {
			// Dans un tableau : une colonne de graphiques, les colonnes choisies de chaque ligne.
			const cols = e.t.columns.slice(c0 - e.t.col, c1 - e.t.col + 1).map((k) => k.name);
			const all = e.t.columns.map((k) => k.name);
			const numeric = e.t.columns.filter((k) => k.type === 'integer' || k.type === 'decimal').map((k) => k.name);
			const nums = cols.length > 1 ? cols : numeric.length ? numeric : all.slice(1);
			sh.sparkDlg = { config: { kind, table: e.t.name, cols: nums }, columns: all };
			return;
		}
		sh.sparkDlg = { config: { kind, source: sparkA1(r0, c0, r1, c1), location: defaultLocation(r0, c0, r1, c1) }, columns: [] };
	}
	async function saveSparkline(cfg: SparklineConfig) {
		const d = sh.sparkDlg;
		if (!d) return;
		if (d.id !== undefined) await sh.source.setSparkline?.(d.id, cfg);
		else await sh.source.createSparkline?.(cfg);
		sh.sparkDlg = null;
		sh.wrap.focus();
	}
	async function removeSparkline() {
		const d = sh.sparkDlg;
		if (d?.id !== undefined) await sh.source.deleteSparkline?.(d.id);
		sh.sparkDlg = null;
		sh.wrap.focus();
	}

	return {
		insertSparkline,
		printSpark,
		saveSparkline,
		removeSparkline
	};
}
