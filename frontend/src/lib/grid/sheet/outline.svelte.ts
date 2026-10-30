// Plan (Données › Grouper), lignes et colonnes masquées, hauteur et largeur,
// volets figés. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { untrack } from 'svelte';
import { VScroll } from '../scroll';
import { type AxisLike } from '../anim';
import { type CellRange } from '../mergesel';
import { detailTarget, type OutlineAction, type OutlineGeom, type SheetOutline } from '../outline';
import { t as tr } from '#lib/i18n/index.js';
import { type SheetSource } from './model';
import { ROW_H, DEF_W, TOP_H } from './consts';

/** Ce que createOutline lit et modifie dans Sheet.svelte. */
export interface OutlineHost {
	readonly olW: number;
	readonly olH: number;
	readonly resizeCanvas: () => void;
	readonly mounted: boolean;
	readonly zoom: number;
	readonly MARGIN: number;
	readonly rh: (r: number) => number;
	readonly screenY: (r: number) => number;
	readonly cax: () => AxisLike;
	readonly screenX: (c: number) => number;
	readonly source: SheetSource;
	readonly menuAction: (fn: (() => Promise<unknown>) | undefined, keepFocus?: boolean) => Promise<boolean>;
	readonly selection: () => CellRange;
	readonly wholeKind: () => "rows" | "cols" | null;
	readonly outline: SheetOutline | null;
	readonly active: { row: number; col: number; };
	readonly say: (m: string) => void;
	readonly styleHiddenRows: Set<number>;
	readonly styleHiddenCols: Set<number>;
	readonly setHidden: (rows: boolean, hide: boolean) => void;
	readonly closeMenus: () => void;
	sizeDlg: { rows: boolean; value: number; x: number; y: number } | null;
	readonly wrap: HTMLDivElement;
	readonly userHeights: Map<number, number>;
	readonly rebuildRows: (override?: { row: number; height: number; }) => void;
	readonly updateExtent: () => void;
	readonly schedule: () => void;
	readonly hiddenCols: Set<number>;
	readonly autoFitColumns: (cols: number[]) => void;
	readonly rowTop: (r: number) => number;
	readonly vs: VScroll;
	readonly viewW: number;
	readonly scroller: HTMLDivElement;
	readonly syncNative: () => void;
}

export function createOutline(h: OutlineHost) {
	$effect(() => {
		void h.olW;
		void h.olH;
		// La grille rétrécit de la barre : ses dimensions sont à reprendre.
		untrack(() => h.resizeCanvas());
	});
	/** Place des lignes et colonnes à l'écran, pour la barre du plan. */
	function outlineGeom(): OutlineGeom | null {
		if (!h.mounted) return null;
		return {
			zoom: h.zoom,
			top: TOP_H,
			margin: h.MARGIN,
			row: (r) => (h.rh(r) ? [h.screenY(r), h.rh(r)] : null),
			col: (c) => (h.cax().size(c) ? [h.screenX(c), h.cax().size(c)] : null)
		};
	}
	function foldOutline(rows: boolean, level: number, line: number, fold: boolean) {
		const f = h.source.foldGroup;
		if (f) void h.menuAction(() => f(rows, level, line, fold));
	}
	function outlineLevel(rows: boolean, level: number) {
		const f = h.source.showOutlineLevel;
		if (f) void h.menuAction(() => f(rows, level));
	}
	/**
	 * Données › Plan (et Alt+Maj+→ / ←) : Grouper ou Dissocier les lignes de
	 * la sélection (ses colonnes si des colonnes entières sont choisies),
	 * Effacer le plan, Afficher ou Masquer le détail du groupe de la cellule
	 * active.
	 */
	function outlineAction(what: OutlineAction) {
		const { r0, r1, c0, c1 } = h.selection();
		const cols = what === 'groupCols' || what === 'ungroupCols' || ((what === 'group' || what === 'ungroup') && h.wholeKind() === 'cols');
		if (what === 'clear') {
			const f = h.source.clearOutline;
			if (f) void h.menuAction(() => f());
			return;
		}
		if (what === 'show' || what === 'hide') {
			// Colonnes : quand la sélection n'est que des colonnes, ou que les lignes n'ont pas de groupe.
			const onCols = h.wholeKind() === 'cols' || (!h.outline?.rows?.length && !!h.outline?.cols?.length);
			const lines = onCols ? h.outline?.cols : h.outline?.rows;
			const before = onCols ? !!h.outline?.colsLeft : !!h.outline?.rowsAbove;
			const at = detailTarget(lines, before, onCols ? h.active.col : h.active.row, what === 'hide');
			if (!at) {
				h.say(tr('grid.outline.nothing'));
				return;
			}
			foldOutline(!onCols, at.level, at.line, what === 'hide');
			return;
		}
		const f = h.source.groupLines;
		if (!f) return;
		const group = what === 'group' || what === 'groupRows' || what === 'groupCols';
		void h.menuAction(() => (cols ? f(false, c0, c1, group) : f(true, r0, r1, group)));
	}

	/** Lignes ou colonnes masquées dans la sélection (pour proposer Afficher). */
	function hiddenIn(rows: boolean): boolean {
		const { r0, r1, c0, c1 } = h.selection();
		const [i0, i1] = rows ? [r0, r1] : [c0, c1];
		for (const i of rows ? h.styleHiddenRows : h.styleHiddenCols) if (i >= i0 - 1 && i <= i1 + 1) return true;
		return false;
	}

	/** Ruban (Accueil › Cellules › Format) : masquer ou afficher les lignes ou les colonnes de la sélection. */
	function hideLines(rows: boolean, hide: boolean) {
		if (hide) h.setHidden(rows, true);
		else unhide(rows);
	}

	/** Afficher : les lignes (ou colonnes) masquées de la sélection et celles qui la bordent. */
	function unhide(rows: boolean) {
		const set = h.source.setLinesHidden;
		if (!set) return;
		const { r0, r1, c0, c1 } = h.selection();
		let [i0, i1] = rows ? [r0, r1] : [c0, c1];
		const hidden = rows ? h.styleHiddenRows : h.styleHiddenCols;
		while (i0 > 0 && hidden.has(i0 - 1)) i0--;
		while (hidden.has(i1 + 1)) i1++;
		h.menuAction(() => set(rows, i0, i1, false));
	}

	function openSize(rows: boolean, x: number, y: number) {
		h.closeMenus();
		h.sizeDlg = { rows, value: rows ? h.rh(h.active.row) || ROW_H : h.cax().size(h.active.col) || DEF_W, x, y };
	}

	function applySize() {
		const d = h.sizeDlg;
		h.sizeDlg = null;
		h.wrap.focus();
		if (!d) return;
		const v = Math.round(Number(d.value));
		if (!Number.isFinite(v) || v < 4 || v > 1000) {
			h.say(tr('grid.size.range'));
			return;
		}
		const { r0, r1, c0, c1 } = h.selection();
		const set = d.rows ? h.source.setRowHeight : h.source.setColumnWidth;
		if (!set) return;
		h.menuAction(async () => {
			for (let i = d.rows ? r0 : c0; i <= (d.rows ? r1 : c1) && i - (d.rows ? r0 : c0) < 500; i++) await set(i, v);
		});
	}

	/**
	 * Ajuster la hauteur (lignes sélectionnées) : retour à la hauteur
	 * automatique, d'après le contenu (comme le double-clic sur le bord
	 * d'un numéro de ligne).
	 */
	function autoFitRows() {
		h.closeMenus();
		const { r0, r1 } = h.selection();
		// Seules les lignes à hauteur choisie changent (la feuille entière peut
		// compter des millions de lignes).
		const write = resetRowHeights([...h.userHeights.keys()].filter((r) => r >= r0 && r <= r1));
		if (write) h.menuAction(write);
	}

	/**
	 * Remet les lignes rows en hauteur automatique, à l'écran tout de suite ;
	 * rend l'écriture (une seule étape d'annulation), ou undefined s'il n'y a
	 * rien à faire.
	 */
	function resetRowHeights(rows: number[]): (() => Promise<unknown>) | undefined {
		const one = h.source.setRowHeight;
		const many = h.source.setRowHeights;
		if (!rows.length || (!one && !many)) return undefined;
		for (const r of rows) h.userHeights.delete(r);
		h.rebuildRows();
		h.updateExtent();
		h.schedule();
		if (many && (rows.length > 1 || !one)) return () => many(rows.map((row) => ({ row, height: 0 })));
		return async () => {
			for (const r of rows) await one!(r, 0);
		};
	}

	/** Ajuster la largeur de colonne (colonnes sélectionnées), d'après le contenu affiché. */
	function autoFitCols() {
		h.closeMenus();
		const { c0, c1 } = h.selection();
		const cols: number[] = [];
		for (let c = c0; c <= c1; c++) if (!h.hiddenCols.has(c)) cols.push(c);
		h.autoFitColumns(cols);
	}

	// Volets figés.
	/** Affichage › Figer les volets : à la cellule active, la ligne du haut, la première colonne, ou les libérer. */
	function freezeActive(what: 'cell' | 'row' | 'col' | 'none') {
		const set = h.source.setFreeze;
		if (!set) return;
		let rows = 0;
		let cols = 0;
		if (what === 'row') rows = 1;
		else if (what === 'col') cols = 1;
		else if (what === 'cell') {
			rows = h.active.row;
			cols = h.active.col;
			if (!rows && !cols) {
				h.say(tr('grid.freeze.place'));
				return;
			}
		}
		// La partie figée part du haut de la feuille : elle doit tenir à l'écran.
		if (h.rowTop(rows) > h.vs.view * 0.75 || h.cax().offset(cols) > (h.viewW - h.MARGIN) * 0.75) {
			h.say(tr('grid.freeze.tooBig'));
			return;
		}
		if (rows || cols) {
			h.vs.to(0);
			h.scroller.scrollLeft = 0;
			h.syncNative();
		}
		h.menuAction(() => set(rows, cols));
	}

	return {
		outlineAction,
		hideLines,
		freezeActive,
		outlineGeom,
		foldOutline,
		outlineLevel,
		hiddenIn,
		unhide,
		openSize,
		applySize,
		autoFitRows,
		resetRowHeights,
		autoFitCols
	};
}
