// Menu ▾ d'une colonne (type de données, clé), info-bulle de l'en-tête, et
// menu contextuel de la grille. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { untrack } from 'svelte';
import { isDefaultTableName } from '../defaultname';
import { type AxisLike } from '../anim';
import { type CellRange } from '../mergesel';
import { type Zone } from '../findzone';
import { spillAt, spillTip } from '../spillmenu';
import { type ChoiceCandidate } from '../tabchoice';
import { followLabel } from '#lib/shell/mergesheets.js';
import { type Citation } from '../reflink';
import { type CondOp, type PanelValue } from '../filtercond';
import { clip } from '#lib/ui/menu.js';
import { colLetters } from '../cfmodel';
import { type FormulaHold } from '../crossref';
import { typeLabel, type ColumnType } from '../format';
import { errorText, t as tr, tAny } from '#lib/i18n/index.js';
import { type TypeMix, type SheetPivot, type SheetChart, type SheetTable, type SheetData, type SheetSource, type Entry } from './model';
import { TOP_H } from './consts';

/** Ce que createMenus lit et modifie dans Sheet.svelte. */
export interface MenusHost {
	readonly source: SheetSource;
	readonly mixes: Map<string, Promise<TypeMix | null>>;
	readonly spills: { row: number; col: number; rows: string[][] }[];
	readonly nameBadgeAt: (ev: MouseEvent) => Entry | null;
	tipCell: string;
	readonly scroller: HTMLDivElement;
	readonly pivotBadgeAt: (ev: MouseEvent) => SheetPivot | null;
	readonly localBadgeAt: (ev: MouseEvent) => Entry | null;
	readonly followBadgeAt: (ev: MouseEvent) => Entry | null;
	readonly data: SheetData;
	readonly keyHintAt: (ev: MouseEvent) => Entry | null;
	readonly KEY_HINT_TIP: string;
	readonly linkUnder: (e: MouseEvent) => { row: number; col: number; url: string; } | null;
	readonly cellAt: (e: MouseEvent) => { row: number; col: number; } | null;
	readonly tableAt: (r: number, c: number) => Entry | undefined;
	readonly AUTO_NUMBER_TIP: string;
	readonly screenX: (c: number) => number;
	readonly cax: () => AxisLike;
	readonly toLocal: (e: MouseEvent) => { x: number; y: number; };
	readonly moveTo: (row: number, col: number, extend?: boolean) => void;
	filterClosed: { key: string; at: number; };
	readonly openFilter: (e: Entry, i: number) => Promise<void>;
	readonly a1: boolean;
	readonly MARGIN: number;
	readonly colAtX: (x: number) => number;
	readonly rowAtY: (y: number) => number;
	readonly rowTop: (r: number) => number;
	readonly fRows: () => number;
	readonly entries: Entry[];
	readonly filterDlg: {
		e: Entry;
		i: number;
		x: number;
		y: number;
		values: PanelValue[];
		/** Liste des valeurs en cours de chargement (grand tableau). */
		loading?: boolean;
		search: string;
		errors: boolean;
		error: string;
		touched: boolean; // liste ou recherche modifiée
		sub: boolean; // sous-menu « Filtres textuels / numériques » ouvert
		// Filtre personnalisé en cours (deux conditions au plus, comme Excel).
		custom: { conds: { op: CondOp | ''; value: string; to: string }[]; or: boolean } | null;
		// Section « Type de données » : conversion en attente de confirmation,
		// raison pour laquelle la colonne ne peut pas être clé, valeurs hors type.
		pending?: { to: ColumnType; n: number };
		keyWhy?: string | null;
		mix?: TypeMix | null;
	} | null;
	readonly closeFilter: () => void;
	menu: { x: number; y: number; e: Entry; n: number; i: number; header: boolean; rowOnly?: boolean; confirmDelete?: boolean } | null;
	readonly pivotDlg: {
		p: SheetPivot;
		field: string;
		/** filter : filtre de rapport ; rows : étiquette d'un champ des lignes (tri en plus). */
		kind: 'filter' | 'rows';
		/** Arborescence : champs des lignes au choix (un seul ▾ sur « Étiquettes de lignes »). */
		fields?: string[];
		x: number;
		y: number;
		values: PanelValue[];
		loading: boolean;
		search: string;
	} | null;
	readonly panel: { kind: 'pivot' | 'chart'; id: number } | null;
	choiceCands: ChoiceCandidate[] | null;
	readonly wrap: HTMLDivElement;
	writing: Promise<void>;
	readonly say: (m: string) => void;
	readonly selChart: number | null;
	readonly charts: SheetChart[];
	readonly onChartMenu: (ev: MouseEvent, ch: SheetChart) => void;
	readonly ensureVisible: (row: number, col: number) => void;
	readonly active: { row: number; col: number; };
	readonly zoomK: () => number;
	readonly screenY: (r: number) => number;
	readonly rh: (r: number) => number;
	readonly locked: string;
	readonly hold: FormulaHold | null;
	readonly formulaMode: boolean;
	readonly openHintMenu: (ev: MouseEvent) => boolean;
	readonly closeMenus: () => void;
	readonly headerAt: (e: MouseEvent) => { kind: "rows"; row: number; } | { kind: "cols"; col: number; } | null;
	freeMenu: { x: number; y: number; kind?: 'rows' | 'cols' } | null;
	refItems: {
		row: number;
		col: number;
		go?: { table: string; value: string; found?: boolean };
		cite?: { table: string; value: string; list: Citation[] | null };
	} | null;
	readonly groupBandAt: (r: number, c: number) => Entry | undefined;
	readonly inSelection: (row: number, col: number) => boolean;
	readonly loadRefItems: (r: number, c: number) => void;
	readonly hiddenIn: (rows: boolean) => boolean;
	readonly selection: () => CellRange;
	readonly rowLabel: (e: Entry, r: number) => string;
	message: string;
	linesHint: { t: number; rows: boolean; at: number; n: number; px: number; table?: number } | null;
	readonly refreshStatus: () => void;
}

export function createMenus(h: MenusHost) {

	function mixOf(e: Entry, i: number): Promise<TypeMix | null> {
		const k = e.t.columns[i];
		if (!h.source.typeMix || k.type !== 'text' || k.fixed || k.formula) return Promise.resolve(null);
		const id = `${e.t.id}:${i}`;
		let p = h.mixes.get(id);
		if (!p) {
			p = h.source.typeMix(e.t, i).then(
				(m) => (m.type ? m : null),
				() => null
			);
			h.mixes.set(id, p);
		}
		return p;
	}

	function mixText(m: TypeMix): string {
		// Saisies hors type d'une colonne : « 3 valeurs non numériques sur 120 ».
		const kind = m.type === 'date' ? 'date' : m.type === 'boolean' ? 'boolean' : 'number';
		return tAny(`grid.typeMix.${kind}`, { n: m.off, total: m.total });
	}

	/** Résultat déversé qui couvre la cellule (menu « Convertir en tableau »). */
	function spillUnder(row: number, col: number) {
		return spillAt(h.spills, row, col);
	}

	function updateHeaderTip(ev: MouseEvent) {
		const nb = h.nameBadgeAt(ev);
		if (nb) {
			if (h.tipCell !== 'name:' + nb.t.name)
				h.scroller.title = isDefaultTableName(nb.t.name)
					? tr('grid.nameBadge.default.tooltip', { name: nb.t.name })
					: tr('grid.nameBadge.tooltip', { name: nb.t.name });
			h.tipCell = 'name:' + nb.t.name;
			return;
		}
		const pb = h.pivotBadgeAt(ev);
		if (pb) {
			if (h.tipCell !== 'pivot:' + pb.id) h.scroller.title = tr('grid.pivot.nameBadge.tooltip', { name: pb.config.name ?? '' });
			h.tipCell = 'pivot:' + pb.id;
			return;
		}
		if (h.localBadgeAt(ev)) {
			if (h.tipCell !== 'local') h.scroller.title = tr('grid.local.tooltip');
			h.tipCell = 'local';
			return;
		}
		if (h.followBadgeAt(ev)) {
			const name = h.data?.choice?.name ?? '';
			if (h.tipCell !== 'follow')
				h.scroller.title = tr('grid.follow.tooltip', { name, value: h.data?.choice?.value || tr('grid.follow.empty'), follow: followLabel(name) });
			h.tipCell = 'follow';
			return;
		}
		if (h.keyHintAt(ev)) {
			if (h.tipCell !== 'keyhint') h.scroller.title = h.KEY_HINT_TIP;
			h.tipCell = 'keyhint';
			return;
		}
		// Lien : son adresse, et le clic.
		const lk = h.linkUnder(ev);
		if (lk) {
			const id = `link:${lk.row},${lk.col}`;
			if (h.tipCell !== id) h.scroller.title = tr('grid.link.tooltip', { address: lk.url.replace(/^mailto:/i, '') });
			h.tipCell = id;
			return;
		}
		const cell = h.cellAt(ev);
		const e = cell && h.tableAt(cell.row, cell.col);
		// Zone déversée d'une formule : d'où elle vient, et le clic droit.
		const sp = cell && !e && h.source.convertSpill ? spillUnder(cell.row, cell.col) : undefined;
		if (sp) {
			const id = `spill:${sp.row},${sp.col}`;
			if (h.tipCell !== id) h.scroller.title = spillTip(sp);
			h.tipCell = id;
			return;
		}
		if (!cell || !e || cell.row !== e.t.row) {
			if (h.tipCell) h.scroller.title = h.tipCell = '';
			return;
		}
		const i = cell.col - e.t.col;
		const id = `${e.t.id}:${i}`;
		if (id === h.tipCell) return;
		h.tipCell = id;
		const k = e.t.columns[i];
		const auto = e.t.autoNumber === i ? ' · ' + (e.t.key === i ? tr('grid.key.autoNumberKey', { tip: h.AUTO_NUMBER_TIP }) : h.AUTO_NUMBER_TIP) : e.t.key === i ? ' · ' + tr('grid.key.isKey') : '';
		h.scroller.title = `${k.name} — ${typeLabel(k.type)}${auto}`;
		mixOf(e, i).then((m) => {
			if (m && h.tipCell === id) h.scroller.title = `${k.name} — ${typeLabel(k.type)}${auto} · ${mixText(m)}`;
		});
	}

	/** Un clic sur les boutons d'un en-tête (▾ ou icône de type) : le menu de la colonne. */
	function openTypeMenu(ev: MouseEvent, cell: { row: number; col: number }): boolean {
		const e = h.tableAt(cell.row, cell.col);
		if (!e || cell.row !== e.t.row) return false;
		return headerButton(ev, e, cell.col, false);
	}

	/**
	 * Boutons d'en-tête de la colonne c du tableau e (frozen : en-tête figé
	 * dans le bandeau du haut) : le ▾ et l'icône de type (indicative) ouvrent
	 * le même menu de la colonne, comme le ▾ d'Excel ; un second clic le ferme.
	 */
	function headerButton(ev: MouseEvent, e: Entry, c: number, frozen: boolean): boolean {
		const i = c - e.t.col;
		const right = h.screenX(c) + h.cax().size(c);
		if (right - h.toLocal(ev).x > 34) return false;
		if (!h.source.sortTable && !(h.source.filterValues && h.source.setFilter) && !h.source.setType) return false;
		if (!frozen) h.moveTo(e.t.row, c);
		if (h.filterClosed.key === `${e.t.id}:${i}` && performance.now() - h.filterClosed.at < 400) {
			h.filterClosed = { key: '', at: 0 };
			return true;
		}
		h.openFilter(e, i);
		return true;
	}

	/** Colonne d'un en-tête figé (bandeau du haut) sous la souris. */
	function frozenHeaderAt(ev: MouseEvent): { e: Entry; col: number } | null {
		if (h.a1) return null;
		const { x, y } = h.toLocal(ev);
		if (y >= TOP_H || x < h.MARGIN) return null;
		const col = h.colAtX(x);
		// Première ligne de la partie qui défile (sous les volets figés).
		const r0 = h.rowAtY(TOP_H + h.rowTop(h.fRows()));
		const e = h.entries.find((e) => e.t.row < r0 && e.t.row >= h.fRows() && e.last >= r0 && col >= e.t.col && col <= e.lastCol);
		return e ? { e, col } : null;
	}

	/** Type de données choisi dans le menu de la colonne (confirmé : malgré les valeurs qui ne s'y convertissent pas). */
	async function chooseType(to: ColumnType, confirmed = false) {
		const m = h.filterDlg;
		if (!m || !h.source.setType) return;
		if (!confirmed && h.source.convertPreview && to !== m.e.t.columns[m.i].type) {
			const n = await h.source.convertPreview(m.e.t, m.i, to);
			if (n > 0) {
				m.pending = { to, n };
				return;
			}
		}
		h.closeFilter();
		const setType = h.source.setType;
		await menuAction(() => setType(m.e.t, m.i, to));
	}

	/** Clé du tableau (pied du menu de la colonne) : la poser sur la colonne ou la retirer. */
	function chooseKey(set: boolean) {
		const m = h.filterDlg;
		if (!m) return;
		h.closeFilter();
		menuAction(() => (set ? h.source.setKey!(m.e.t, m.i) : h.source.clearKey!(m.e.t)));
	}

	const wantsCands = $derived(!!h.source.setTabChoice && (!!h.menu || !!h.filterDlg || !!h.pivotDlg || h.panel?.kind === 'pivot'));
	$effect(() => {
		if (!wantsCands) return;
		untrack(() => {
			h.choiceCands = null;
			h.source.choiceCandidates?.().then(
				(c) => (h.choiceCands = c ?? []),
				() => undefined
			);
		});
	});

	function deleteTable(t: SheetTable) {
		// Pas menuAction : il effacerait le message une fois l'action finie.
		const del = h.source.deleteTables!;
		h.menu = null;
		h.wrap.focus();
		h.writing = h.writing.catch(() => undefined).then(() =>
			del([t]).then(
				() => h.say(tr('grid.table.deleted', { name: t.name })),
				(err) => h.say(errorText(err))
			)
		);
	}

	const pointAt = (clientX: number, clientY: number) => ({ clientX, clientY, preventDefault() {}, stopPropagation() {} }) as unknown as MouseEvent;

	/** Menu contextuel ouvert au clavier : posé sur la cellule active, ou sur le graphique sélectionné. */
	function keyContextMenu() {
		const el = h.selChart !== null ? h.wrap.querySelector<HTMLElement>('.chart.selected') : null;
		const ch = el ? h.charts.find((c) => c.id === h.selChart) : undefined;
		if (el && ch) {
			const r = el.getBoundingClientRect();
			h.onChartMenu(pointAt(r.left + Math.min(r.width / 2, 40), r.top + Math.min(r.height / 2, 20)), ch);
			return;
		}
		h.ensureVisible(h.active.row, h.active.col);
		const k = h.zoomK();
		const r = h.scroller.getBoundingClientRect();
		const x = h.screenX(h.active.col) + Math.min(h.cax().size(h.active.col), 60) / 2;
		const y = h.screenY(h.active.row) + h.rh(h.active.row) / 2;
		onContextMenu(pointAt(r.left + x * k, r.top + y * k));
	}

	function onContextMenu(ev: MouseEvent) {
		if (h.locked && !h.hold) {
			ev.preventDefault(); // feuille protégée : aucune cellule à cibler
			return;
		}
		if (!h.formulaMode && h.openHintMenu(ev)) return;
		const cell = h.cellAt(ev);
		ev.preventDefault();
		h.closeMenus();
		if (!cell) {
			// Clic droit sur un numéro de ligne : menu de la ligne du tableau ;
			// hors tableau (ou sur une lettre de colonne) : presse-papiers.
			const hd = h.headerAt(ev);
			const r = h.wrap.getBoundingClientRect();
			const te = hd?.kind === 'rows' ? h.entries.find((x) => hd.row > x.t.row && hd.row <= x.last) : undefined;
			h.freeMenu = null;
			h.refItems = null;
			if (te && hd?.kind === 'rows') h.menu = { x: ev.clientX - r.left, y: ev.clientY - r.top, e: te, n: hd.row - te.t.row - 1, i: 0, header: false, rowOnly: true };
			else if (hd) h.freeMenu = { x: ev.clientX - r.left, y: ev.clientY - r.top, kind: hd.kind };
			return;
		}
		// La bande des groupes donne le menu de l'en-tête.
		const e = h.tableAt(cell.row, cell.col) ?? h.groupBandAt(cell.row, cell.col);
		if (!h.inSelection(cell.row, cell.col)) h.moveTo(cell.row, cell.col);
		const r = h.wrap.getBoundingClientRect();
		h.loadRefItems(cell.row, cell.col);
		if (!e) {
			h.freeMenu = { x: ev.clientX - r.left, y: ev.clientY - r.top };
			return;
		}
		h.menu = { x: ev.clientX - r.left, y: ev.clientY - r.top, e, n: cell.row - e.t.row - 1, i: cell.col - e.t.col, header: cell.row <= e.t.row };
	}

	/** Sous-menu Ligne « … » ou Colonne « … » d'un numéro ou d'une lettre : ce que la feuille permet (ctxmenu.ts). */
	function linesOpts(kind: 'rows' | 'cols') {
		const rows = kind === 'rows';
		const on = !!h.source.sheetLines;
		const hide = on && !!h.source.setLinesHidden;
		return { insert: on, size: on && !!(rows ? h.source.setRowHeight : h.source.setColumnWidth), hide, unhide: hide && h.hiddenIn(rows) };
	}

	/**
	 * Titre du sous-menu de la ligne ou de la colonne (Ligne « 3 », Colonnes
	 * « B » à « D ») : dans un tableau, l'identifiant que montre la gouttière
	 * (sa clé, sinon son numéro, rowLabel) ; sur la feuille, le numéro ou les
	 * lettres de l'en-tête.
	 */
	function linesTitle(kind: 'rows' | 'cols', e?: Entry): string {
		const { r0, r1, c0, c1 } = h.selection();
		const name = (i: number) => clip(kind === 'cols' ? colLetters(i) : e ? h.rowLabel(e, i) : String(i + 1));
		const [a, b] = kind === 'rows' ? [r0, r1] : [c0, c1];
		if (b > a) return tr(kind === 'rows' ? 'grid.menu.rows' : 'grid.menu.columns', { first: name(a), last: name(b) });
		return tr(kind === 'rows' ? 'grid.menu.row' : 'grid.menu.column', { name: name(a) });
	}

	/**
	 * Écriture d'un menu ou d'un volet, à la suite des précédentes ; rend
	 * faux si elle est refusée. keepFocus : réglage d'un volet (graphique,
	 * TCD), le focus y reste au lieu de revenir à la grille.
	 */
	async function menuAction(fn: (() => Promise<unknown>) | undefined, keepFocus = false): Promise<boolean> {
		h.menu = null;
		h.freeMenu = null;
		if (!keepFocus) h.wrap.focus();
		if (!fn) return false;
		let ok = true;
		h.writing = h.writing.catch(() => undefined).then(async () => {
			try {
				await fn();
				h.message = '';
			} catch (err) {
				ok = false;
				h.linesHint = null; // refusée : rien ne glisse
				h.message = errorText(err);
			}
			h.refreshStatus();
		});
		await h.writing;
		return ok;
	}

	return {
		mixOf,
		mixText,
		spillUnder,
		updateHeaderTip,
		openTypeMenu,
		headerButton,
		frozenHeaderAt,
		chooseType,
		chooseKey,
		deleteTable,
		keyContextMenu,
		onContextMenu,
		linesOpts,
		menuAction
	};
}
