// Arborescence d'un tableau croisé dans la grille : plier et déplier un
// groupe (+/−), une colonne, ajouter un segment, filtres du rapport. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { foldAll, foldTarget, pivotBoxHit, pivotBoxLeft, PIVOT_BOX, setColFolded, setFolded } from '../pivottree';
import { newSlicer, withSlicer } from '../pivotslicer';
import { t as tr } from '#lib/i18n/index.js';
import { type PivotConfig, type SheetPivot, type SheetSource, pivotRowOf } from './model';

/** Ce que createPivotFold lit et modifie dans Sheet.svelte. */
export interface PivotFoldHost {
	readonly source: SheetSource;
	readonly pivotHead: (p: SheetPivot) => number;
	readonly toLocal: (e: MouseEvent) => { x: number; y: number; };
	readonly screenX: (c: number) => number;
	readonly moveTo: (row: number, col: number, extend?: boolean) => void;
	readonly menuAction: (fn: (() => Promise<unknown>) | undefined, keepFocus?: boolean) => Promise<boolean>;
	readonly pivotAt: (r: number, c: number) => SheetPivot | undefined;
	readonly active: { row: number; col: number; };
}

export function createPivotFold(h: PivotFoldHost) {
	/** Clic sur la case −/+ d'un groupe de colonnes : le plie ou le déplie. */
	function toggleColFold(ev: MouseEvent, p: SheetPivot, cell: { row: number; col: number }): boolean {
		if (!p.cols || !h.source.setPivot || cell.row < h.pivotHead(p) || cell.row > p.row) return false;
		const j = cell.col - p.col;
		const d = cell.row - h.pivotHead(p);
		const info = p.cols[j];
		const x = h.toLocal(ev).x - h.screenX(cell.col);
		if (!info?.path || x < pivotBoxLeft(0) - 3 || x > pivotBoxLeft(0) + PIVOT_BOX + 3) return false;
		let path: string[] | null = null;
		let fold = false;
		if (cell.row === p.row && info.collapsed) path = info.path;
		else if (cell.row < p.row && p.colHead?.[d]?.[j] && info.path.length > d + 1) {
			path = info.path.slice(0, d + 1);
			fold = true;
		}
		if (!path) return false;
		h.moveTo(cell.row, cell.col);
		const set = h.source.setPivot;
		const cfg = setColFolded(p.config, path, fold);
		h.menuAction(() => set(p.id, cfg));
		return true;
	}

	/** Nouveau segment (ou chronologie) du champ field, posé à droite du TCD. */
	function addSlicer(p: SheetPivot, field: string, timeline: boolean) {
		const set = h.source.setPivot;
		if (!set) return;
		const s = newSlicer(p.config, field, p.anchor ?? p.row, p.col + (p.cells[0]?.length ?? 1) - 1, timeline);
		h.menuAction(() => set(p.id, withSlicer(p.config, p.config.slicers?.length ?? 0, s)), true);
	}

	/** Valeurs d'un champ d'un TCD (groupes et champs calculés compris). */
	function pivotValuesOf(cfg: PivotConfig, field: string): Promise<{ text: string; count: number }[]> {
		if (h.source.pivotValues) return h.source.pivotValues(cfg, field);
		return h.source.pivotFieldValues?.(cfg.source, field) ?? Promise.resolve([]);
	}

	/** Le clic tombe-t-il sur une case −/+ de TCD ? Plie ou déplie alors le groupe (une étape d'annulation). */
	function togglePivotFold(ev: MouseEvent, cell: { row: number; col: number }): boolean {
		const pc = h.pivotAt(cell.row, cell.col);
		if (pc && toggleColFold(ev, pc, cell)) return true;
		const p = h.pivotAt(cell.row, cell.col);
		if (!p?.rows || cell.col !== p.col || !h.source.setPivot) return false;
		const info = pivotRowOf(p, cell.row);
		if (!pivotBoxHit(info, h.toLocal(ev).x - h.screenX(cell.col)) || !info?.path) return false;
		h.moveTo(cell.row, cell.col);
		const set = h.source.setPivot;
		const cfg = setFolded(p.config, info.path, !info.collapsed);
		h.menuAction(() => set(p.id, cfg));
		return true;
	}

	/** Développer, Réduire (le groupe de la cellule active), Développer tout, Réduire tout. */
	function foldPivot(p: SheetPivot, what: 'expand' | 'collapse' | 'expandAll' | 'collapseAll') {
		const set = h.source.setPivot;
		if (!set) return;
		let cfg: PivotConfig;
		if (what === 'expandAll' || what === 'collapseAll') cfg = foldAll(p.config, what === 'collapseAll');
		else {
			const path = foldTarget(pivotRowOf(p, h.active.row), what === 'expand');
			if (!path) return;
			cfg = setFolded(p.config, path, what === 'collapse');
		}
		h.menuAction(() => set(p.id, cfg));
	}

	/** Valeur affichée d'un filtre de rapport : « (Tous) », la valeur, ou « (Plusieurs éléments) ». */
	function pivotFilterText(values: string[]): string {
		if (!values.length) return tr('grid.pivot.filter.all');
		if (values.length === 1) return values[0] === '' ? tr('grid.filter.blanks') : values[0];
		return tr('grid.pivot.filter.several');
	}

	/** Le champ f du TCD est-il filtré ou trié par le ▾ de son étiquette ? */
	function pivotFiltered(p: SheetPivot, f: string): boolean {
		return !!p.config.items?.some((x) => x.field === f && x.values.length) || !!p.config.sort?.[f];
	}

	return {
		addSlicer,
		pivotValuesOf,
		togglePivotFold,
		foldPivot,
		pivotFilterText,
		pivotFiltered
	};
}
