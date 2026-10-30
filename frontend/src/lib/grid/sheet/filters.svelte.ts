// Filtres : panneau ▾ d'une colonne de tableau (valeurs, conditions), filtres
// d'un TCD dans la grille, tri personnalisé. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { clamp } from '../scroll';
import { type AxisLike } from '../anim';
import { withoutField, type CustomOrder } from '../customsort';
import { condArgs, searchValues, valuesFilter, type CondOp, type PanelValue } from '../filtercond';
import { type ColumnType } from '../format';
import { errorText, t as tr } from '#lib/i18n/index.js';
import { type TypeMix, type PivotConfig, type SheetPivot, type ColumnFilter, type SheetSource, type Entry } from './model';
import { TOP_H } from './consts';

/** Ce que createFilters lit et modifie dans Sheet.svelte. */
export interface FiltersHost {
	readonly pivotAt: (r: number, c: number) => SheetPivot | undefined;
	readonly source: SheetSource;
	readonly screenX: (c: number) => number;
	readonly cax: () => AxisLike;
	readonly toLocal: (e: MouseEvent) => { x: number; y: number; };
	readonly PIVOT_DROP: number;
	readonly moveTo: (row: number, col: number, extend?: boolean) => void;
	readonly closeMenus: () => void;
	pivotDlg: { p: SheetPivot; field: string; kind: "filter" | "rows"; fields?: string[]; x: number; y: number; values: PanelValue[]; loading: boolean; search: string; } | null;
	readonly toView: (v: number) => number;
	readonly scroller: HTMLDivElement;
	readonly screenY: (r: number) => number;
	readonly rh: (r: number) => number;
	readonly pivotValuesOf: (cfg: PivotConfig, field: string) => Promise<{ text: string; count: number; }[]>;
	readonly say: (m: string) => void;
	readonly menuAction: (fn: (() => Promise<unknown>) | undefined, keepFocus?: boolean) => Promise<boolean>;
	menu: { x: number; y: number; e: Entry; n: number; i: number; header: boolean; rowOnly?: boolean; confirmDelete?: boolean; } | null;
	filterDlg: { e: Entry; i: number; x: number; y: number; values: PanelValue[]; loading?: boolean; search: string; errors: boolean; error: string; touched: boolean; sub: boolean; custom: { conds: { op: CondOp | ""; value: string; to: string; }[]; or: boolean; } | null; pending?: { to: ColumnType; n: number; }; keyWhy?: string | null; mix?: TypeMix | null; } | null;
	customDlg: { field: string; loadValues: () => Promise<string[]>; loadOrder?: () => Promise<CustomOrder | null>; apply: (o: CustomOrder) => void; } | null;
	readonly targetTable: () => Entry | undefined;
	readonly targetCol: (e: Entry) => number;
	readonly zoom: number;
	readonly mixOf: (e: Entry, i: number) => Promise<TypeMix | null>;
	readonly ownerAt: (r: number, c: number) => Entry | undefined;
	readonly active: { row: number; col: number; };
	readonly wrap: HTMLDivElement;
	filterClosed: { key: string; at: number; };
}

export function createFilters(h: FiltersHost) {

	/** Le clic tombe-t-il sur un ▾ de TCD ? Ouvre alors son menu. */
	function openPivotMenu(ev: MouseEvent, cell: { row: number; col: number }): boolean {
		const p = h.pivotAt(cell.row, cell.col);
		if (!p || !h.source.setPivot) return false;
		const near = h.screenX(cell.col) + h.cax().size(cell.col) - h.toLocal(ev).x <= h.PIVOT_DROP + 2;
		if (!near) return false;
		const a = p.anchor ?? p.row;
		const filters = p.config.filters ?? [];
		let field: string | undefined;
		let kind: 'filter' | 'rows' = 'filter';
		if (cell.row - a < filters.length && cell.col === p.col + 1) field = filters[cell.row - a].field;
		else if (cell.row === p.row && cell.col - p.col < (p.rows ? Math.min(1, p.config.rowFields?.length ?? 0) : (p.config.rowFields?.length ?? 0))) {
			field = p.config.rowFields![cell.col - p.col];
			kind = 'rows';
		}
		if (field === undefined) return false;
		h.moveTo(cell.row, cell.col);
		h.closeMenus();
		h.pivotDlg = {
			p,
			field,
			kind,
			// Arborescence : un seul ▾ pour tous les champs des lignes (« Sélectionner un champ », Excel).
			fields: kind === 'rows' && p.rows ? [...(p.config.rowFields ?? [])] : undefined,
			x: Math.max(4, Math.min(h.toView(h.screenX(cell.col)), h.toView(h.scroller.clientWidth) - 280)),
			y: h.toView(h.screenY(cell.row) + h.rh(cell.row)),
			values: [],
			loading: !!h.source.pivotFieldValues,
			search: ''
		};
		loadPivotValues();
		return true;
	}

	/** Charge les valeurs du champ du menu ▾ ouvert (cochées : celles gardées). */
	function loadPivotValues() {
		const d = h.pivotDlg;
		if (!d) return;
		const { p, field, kind } = d;
		const kept = (kind === 'filter' ? (p.config.filters ?? []) : (p.config.items ?? [])).find((x) => x.field === field)?.values ?? [];
		const keep = kept.length ? new Set(kept) : null;
		d.values = [];
		d.loading = !!h.source.pivotFieldValues;
		const mine = () => h.pivotDlg?.p.id === p.id && h.pivotDlg.field === field;
		h.pivotValuesOf(p.config, field).then(
			(vals) => {
				if (!mine()) return;
				h.pivotDlg!.values = vals.map((v) => ({ ...v, on: !keep || keep.has(v.text) }));
				h.pivotDlg!.loading = false;
			},
			(err) => {
				if (mine()) h.pivotDlg!.loading = false;
				h.say(errorText(err));
			}
		);
	}

	/** Enregistre la configuration du TCD du menu ▾ (annulable comme un changement du volet). */
	function savePivot(cfg: PivotConfig) {
		const d = h.pivotDlg;
		h.pivotDlg = null;
		if (!d || !h.source.setPivot) return;
		const set = h.source.setPivot;
		h.menuAction(() => set(d.p.id, cfg));
	}

	function sortPivot(dir: 'asc' | 'desc') {
		const d = h.pivotDlg;
		if (!d) return;
		const cfg: PivotConfig = { ...d.p.config, sort: { ...d.p.config.sort, [d.field]: dir }, orders: withoutField(d.p.config.orders, d.field) };
		if (!cfg.orders) delete cfg.orders;
		savePivot(cfg);
	}

	/** Tri personnalisé… d'une colonne de tableau (menus ▾ et clic droit, ruban Données). */
	function openCustomSort(e: Entry, i: number) {
		const run = h.source.sortTableCustom;
		if (!run) return;
		const t = e.t;
		h.menu = null;
		h.filterDlg = null;
		h.customDlg = {
			field: t.columns[i]?.name ?? '',
			loadValues: async () => ((await h.source.filterValues?.(t, i)) ?? []).map((v) => v.text),
			loadOrder: h.source.columnOrder ? () => h.source.columnOrder!(t, i) : undefined,
			apply: (o) => {
				h.menuAction(() => run(t, i, o)).then((ok) => ok && h.say(tr('grid.customSort.applied')));
			}
		};
	}

	/** Tri personnalisé… du champ du menu ▾ d'un TCD : enregistré dans sa configuration. */
	function openPivotCustomSort() {
		const d = h.pivotDlg;
		if (!d || !h.source.setPivot) return;
		const { p, field } = d;
		const set = h.source.setPivot;
		h.pivotDlg = null;
		h.customDlg = {
			field,
			loadValues: async () => (await h.pivotValuesOf(p.config, field)).map((v) => v.text),
			loadOrder: async () => p.config.orders?.[field] ?? null,
			apply: (o) => {
				const cfg: PivotConfig = { ...p.config, orders: { ...p.config.orders, [field]: o }, sort: withoutField(p.config.sort, field) };
				if (!cfg.sort) delete cfg.sort;
				h.menuAction(() => set(p.id, cfg));
			}
		};
	}

	/** Tri personnalisé de la colonne active (ruban : Trier et filtrer). */
	function sortCustomActive() {
		const e = h.targetTable();
		if (!e || !h.source.sortTableCustom) {
			h.say(tr('grid.sort.placeActive'));
			return;
		}
		openCustomSort(e, h.targetCol(e));
	}

	/** OK : valeurs cochées gardées (toutes : plus de filtre) ; clear : efface le filtre et le tri du champ. */
	function applyPivotFilter(clear = false) {
		const d = h.pivotDlg;
		if (!d) return;
		const on = d.values.filter((v) => v.on).map((v) => v.text);
		if (!clear && !on.length) {
			h.say(tr('grid.filter.chooseOne'));
			return;
		}
		const values = clear || on.length === d.values.length ? [] : on;
		const cfg = { ...d.p.config };
		if (d.kind === 'filter') {
			cfg.filters = (cfg.filters ?? []).map((f) => (f.field === d.field ? { field: f.field, values } : f));
		} else {
			const rest = (cfg.items ?? []).filter((x) => x.field !== d.field);
			cfg.items = values.length ? [...rest, { field: d.field, values }] : rest;
			if (!cfg.items.length) delete cfg.items;
			if (clear && cfg.sort) {
				const { [d.field]: _, ...sort } = cfg.sort;
				cfg.sort = Object.keys(sort).length ? sort : undefined;
			}
			if (clear && cfg.orders) cfg.orders = withoutField(cfg.orders, d.field);
		}
		savePivot(cfg);
	}

	/** Position d'un panneau ancré sous l'en-tête de la colonne i du tableau e
	 * (sous le bandeau quand l'en-tête est figé), gardé dans la grille (width :
	 * largeur du panneau). */
	function anchorUnder(e: Entry, i: number, width: number) {
		// Hors du calque zoomé : en pixels de l'écran (toView).
		const x = h.toView(h.screenX(e.t.col + i));
		return {
			x: Math.max(4, Math.min(x, h.toView(h.scroller.clientWidth) - width - 4)),
			y: h.toView(clamp(h.screenY(e.t.row) + h.rh(e.t.row), TOP_H, Math.max(TOP_H, h.scroller.clientHeight - 200 / h.zoom)))
		};
	}

	/** Met le focus sur le premier champ d'un panneau à son ouverture. */
	function focusFirst(node: HTMLElement) {
		requestAnimationFrame(() => (node.querySelector<HTMLElement>('.add input, .add textarea') ?? node.querySelector<HTMLElement>('.add button'))?.focus());
	}

	/**
	 * Menu ▾ d'une colonne de tableau, comme celui d'Excel (le ▾ et l'icône de
	 * type de l'en-tête, figé ou non, et le clic droit l'ouvrent) : tri,
	 * filtres (conditions, recherche, liste à cocher, OK / Annuler), puis le
	 * type de données et la clé du tableau.
	 */
	async function openFilter(e: Entry, i: number) {
		h.closeMenus();
		const f = e.t.columns[i]?.filter;
		const keep = f?.values ? new Set(f.values) : null;
		const lists = !!h.source.filterValues;
		const checksKey = !!h.source.keyCheck && e.t.key !== i;
		// Le menu s'ouvre tout de suite (tri, filtres, type) ; la liste des
		// valeurs et le contrôle de la clé, longs sur un grand tableau,
		// arrivent ensuite à la place de leur indicateur de chargement.
		h.filterDlg = {
			e,
			i,
			...anchorUnder(e, i, 310),
			values: [],
			loading: lists,
			search: '',
			errors: !!f?.errors,
			error: '',
			touched: false,
			sub: false,
			custom: null,
			keyWhy: checksKey ? undefined : null
		};
		const mine = () => h.filterDlg !== null && h.filterDlg.e.t.id === e.t.id && h.filterDlg.i === i;
		h.mixOf(e, i).then((mix) => mine() && (h.filterDlg!.mix = mix));
		if (checksKey)
			h.source.keyCheck!(e.t, i).then(
				(why) => mine() && (h.filterDlg!.keyWhy = why),
				() => mine() && (h.filterDlg!.keyWhy = null)
			);
		if (lists)
			h.source.filterValues!(e.t, i).then(
				(vals) => {
					if (!mine()) return;
					h.filterDlg!.values = vals.map((v) => ({ ...v, on: !keep || keep.has(v.text) }));
					h.filterDlg!.loading = false;
				},
				(err) => {
					if (mine()) h.filterDlg!.loading = false;
					h.say(errorText(err));
				}
			);
	}

	/** Filtre de la colonne de la cellule active (ruban Données). */
	function filterActive(errorsOnly = false) {
		const e = h.targetTable();
		if (!e || !h.source.setFilter) {
			h.say(tr('grid.filter.placeActive'));
			return;
		}
		const i = h.targetCol(e);
		if (errorsOnly) {
			const f = e.t.columns[i]?.filter;
			const set = h.source.setFilter;
			const rest: ColumnFilter = { ...f };
			delete rest.errors;
			h.menuAction(() => set(e.t, i, f?.errors ? (rest.values || rest.conds?.length ? rest : null) : { ...f, errors: true }));
			return;
		}
		openFilter(e, i);
	}

	/** Ctrl+Maj+L : efface les filtres du tableau actif, sinon ouvre celui de la colonne. */
	function toggleFilters() {
		const e = h.ownerAt(h.active.row, h.active.col);
		if (e && h.source.clearFilters && e.t.columns.some((c) => c.filter)) {
			const clear = h.source.clearFilters;
			h.menuAction(() => clear(e.t));
			return;
		}
		filterActive();
	}

	function filterShown() {
		const d = h.filterDlg;
		return d ? searchValues(d.values, d.search) : [];
	}

	/** Filtre personnalisé : première condition op (sous-menu), ou les conditions du filtre actif. */
	function startCustom(op?: CondOp) {
		const d = h.filterDlg;
		if (!d) return;
		d.sub = false;
		d.error = '';
		const cur = d.e.t.columns[d.i]?.filter;
		if (op && condArgs(op) === 0) {
			d.custom = { conds: [{ op, value: '', to: '' }], or: false };
			applyFilter();
			return;
		}
		const conds = op
			? [{ op, value: op === 'top' || op === 'bottom' ? '10' : '', to: '' }]
			: (cur?.conds ?? []).map((c) => ({ op: c.op as CondOp | '', value: c.value ?? '', to: c.to ?? '' }));
		while (conds.length < 2) conds.push({ op: conds.length ? '' : 'eq', value: '', to: '' });
		d.custom = { conds, or: !op && !!cur?.or };
	}

	async function applyFilter(clear = false) {
		const d = h.filterDlg;
		if (!d || !h.source.setFilter) return;
		let f: ColumnFilter | null = null;
		const cur = d.e.t.columns[d.i]?.filter;
		if (clear) f = null;
		else if (d.custom) {
			const conds = d.custom.conds
				.filter((c) => c.op && (condArgs(c.op) === 0 || c.value.trim() !== ''))
				.map((c) => ({ op: c.op as CondOp, ...(condArgs(c.op as CondOp) ? { value: c.value } : {}), ...(condArgs(c.op as CondOp) === 2 ? { to: c.to } : {}) }));
			if (!conds.length) {
				d.error = tr('grid.filter.cond.missingValue');
				return;
			}
			f = { conds };
			if (d.custom.or && conds.length > 1) f.or = true;
			if (d.errors) f.errors = true;
		} else if (!d.touched && cur?.conds?.length) {
			// Liste pas touchée : le filtre par condition reste.
			f = { ...cur };
			if (d.errors) f.errors = true;
			else delete f.errors;
		} else {
			const r = valuesFilter(d.values, d.search, d.errors);
			if (r.error) {
				d.error = r.error;
				return;
			}
			f = r.filter;
		}
		try {
			await h.source.setFilter(d.e.t, d.i, f);
			closeFilter();
		} catch (err) {
			d.error = errorText(err);
		}
	}

	function closeFilter() {
		h.filterDlg = null;
		h.wrap.focus();
	}

	function dismissFilter() {
		const d = h.filterDlg;
		if (d) h.filterClosed = { key: `${d.e.t.id}:${d.i}`, at: performance.now() };
		h.filterDlg = null;
	}

	return {
		sortCustomActive,
		filterActive,
		openPivotMenu,
		loadPivotValues,
		sortPivot,
		openCustomSort,
		openPivotCustomSort,
		applyPivotFilter,
		focusFirst,
		openFilter,
		toggleFilters,
		filterShown,
		startCustom,
		applyFilter,
		closeFilter,
		dismissFilter
	};
}
