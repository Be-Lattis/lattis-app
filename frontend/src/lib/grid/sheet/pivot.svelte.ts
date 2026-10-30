// Tableaux croisés dynamiques de la feuille : volet des champs, source,
// pose à la cellule cliquée (pivotplace.ts), déplacement. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { changeSource, startValues } from '../pivotfields';
import { placeAt, placedMessage, placeHint, placeKey, type Placement } from '../pivotplace';
import { errorText, t as tr } from '#lib/i18n/index.js';
import { type PivotConfig, type SheetPivot, type SheetSource, type Entry } from './model';

/** Ce que createPivot lit et modifie dans Sheet.svelte. */
export interface PivotHost {
	readonly pivotIn: (r0: number, c0: number, r1: number, c1: number) => SheetPivot | undefined;
	readonly pivotZone: (p: SheetPivot) => { r0: number; c0: number; r1: number; c1: number; };
	readonly entries: Entry[];
	readonly topRow: (t: { row: number; twoLevel?: boolean; }) => number;
	readonly pivots: SheetPivot[];
	readonly cells: Map<string, string>;
	panel: { kind: "pivot" | "chart"; id: number; } | null;
	readonly source: SheetSource;
	readonly menuAction: (fn: (() => Promise<unknown>) | undefined, keepFocus?: boolean) => Promise<boolean>;
	readonly say: (m: string) => void;
	pivotSourcePick: number | null;
	readonly sourceColumns: (name: string) => { name: string; numeric: boolean; type: string; }[];
	freeMenu: { x: number; y: number; kind?: "rows" | "cols"; } | null;
	active: { row: number; col: number; };
	anchor: { row: number; col: number; };
	readonly pivotAt: (r: number, c: number) => SheetPivot | undefined;
	readonly targetTable: () => Entry | undefined;
	readonly targetCol: (e: Entry) => number;
	openPivotOnLoad: boolean;
	writing: Promise<void>;
	placing: Placement<PivotConfig> | null;
	readonly closeMenus: () => void;
	menu: { x: number; y: number; e: Entry; n: number; i: number; header: boolean; rowOnly?: boolean; confirmDelete?: boolean; } | null;
	placeHover: { row: number; col: number; } | null;
	placeSize: { h: number; w: number; };
	readonly onplacing: ((p: { hint: string; create: boolean; } | null) => void) | undefined;
	readonly wrap: HTMLDivElement;
	readonly schedule: () => void;
	readonly scroller: HTMLDivElement;
	readonly sheetId: () => number;
	message: string;
	readonly refreshFx: () => void;
	readonly refreshStatus: () => void;
}

export function createPivot(sh: PivotHost) {
	/**
	 * Tableau croisé que la sélection désigne pour un déplacement : elle est
	 * dans un seul TCD, ou le couvre en entier sans rien prendre d'autre
	 * (ni tableau, ni cellule remplie, ni autre TCD).
	 */
	function pivotHeld(sel: { r0: number; c0: number; r1: number; c1: number }): SheetPivot | undefined {
		const pv = sh.pivotIn(sel.r0, sel.c0, sel.r1, sel.c1);
		if (!pv) return undefined;
		const z = sh.pivotZone(pv);
		if (sel.r0 >= z.r0 && sel.r1 <= z.r1 && sel.c0 >= z.c0 && sel.c1 <= z.c1) return pv;
		if (sel.r0 > z.r0 || sel.c0 > z.c0 || sel.r1 < z.r1 || sel.c1 < z.c1) return undefined;
		const outside = (r: number, c: number) => r < z.r0 || r > z.r1 || c < z.c0 || c > z.c1;
		if (sh.entries.some((e) => sh.topRow(e.t) <= sel.r1 && e.last >= sel.r0 && e.t.col <= sel.c1 && e.lastCol >= sel.c0)) return undefined;
		const meets = (q: { r0: number; c0: number; r1: number; c1: number }) => q.r0 <= sel.r1 && q.r1 >= sel.r0 && q.c0 <= sel.c1 && q.c1 >= sel.c0;
		if (sh.pivots.some((o) => o.id !== pv.id && !o.error && meets(sh.pivotZone(o)))) return undefined;
		for (const k of sh.cells.keys()) {
			const [r, c] = k.split(',').map(Number);
			if (r >= sel.r0 && r <= sel.r1 && c >= sel.c0 && c <= sel.c1 && outside(r, c)) return undefined;
		}
		return pv;
	}

	/** Supprime un tableau croisé, en erreur ou non (volet, clic droit). */
	function removePivot(p: SheetPivot) {
		if (sh.panel?.kind === 'pivot' && sh.panel.id === p.id) sh.panel = null;
		const del = sh.source.deletePivot;
		void sh.menuAction(del && (() => del(p.id))).then((ok) => ok && sh.say(tr('grid.pivot.deleted', { name: p.config.name ?? '' })));
	}

	/** Rattache un TCD à un autre tableau source : les champs présents y sont gardés. */
	function repointPivot(p: SheetPivot, name: string) {
		const set = sh.source.setPivot;
		sh.pivotSourcePick = null;
		const cfg = changeSource($state.snapshot(p.config) as PivotConfig, name, sh.sourceColumns(name).map((c) => ({ name: c.name, type: c.type })));
		void sh.menuAction(set && (() => set(p.id, cfg)), true);
	}

	/** « Changer la source… » : ouvre le volet du TCD sur la liste des sources. */
	function pickPivotSource(p: SheetPivot) {
		sh.freeMenu = null;
		sh.active = { row: p.anchor ?? p.row, col: p.col };
		sh.anchor = { ...sh.active };
		sh.pivotSourcePick = p.id;
		sh.panel = { kind: 'pivot', id: p.id };
	}

	/** Crée un tableau croisé à partir du tableau actif, ou ouvre le volet de celui sous la cellule active. */
	function pivotActive() {
		const p = sh.pivotAt(sh.active.row, sh.active.col) ?? sh.pivots.find((p) => p.row === sh.active.row && p.col === sh.active.col);
		if (p) {
			sh.panel = { kind: 'pivot', id: p.id };
			return;
		}
		const e = sh.targetTable();
		if (!e || !sh.source.createPivot) {
			sh.say(tr('grid.pivot.placeActive'));
			return;
		}
		const k = e.t.columns[sh.targetCol(e)];
		// La colonne active en lignes, le nombre de lignes en valeur
		// (startValues) ; le volet s'ouvre pour la suite.
		const first = k && !k.formula ? k.name : (e.t.columns.find((c) => !c.formula)?.name ?? '');
		const cfg: PivotConfig = { source: e.t.name, rowFields: [first], values: startValues() };
		// Le premier clic sur une cellule (de n'importe quelle feuille) le
		// pose ; « Nouvelle feuille » dans la barre d'état garde l'ancienne
		// façon (pivotplace.ts).
		if (sh.source.createPivotAt) startPlacement({ kind: 'create', config: cfg, source: e.t.name });
		else pivotOnNewSheet(cfg);
	}

	/** Crée le tableau croisé sur une nouvelle feuille (comme Excel par défaut). */
	function pivotOnNewSheet(cfg: PivotConfig) {
		const create = sh.source.createPivot;
		if (!create) return;
		sh.openPivotOnLoad = true;
		sh.writing = sh.writing.catch(() => undefined).then(() => create(cfg).then(() => undefined, (err) => sh.say(errorText(err))));
	}

	function onPlaceKey(ev: KeyboardEvent) {
		if (!sh.placing || placeKey(ev.key) !== 'cancel') return;
		ev.preventDefault();
		ev.stopPropagation();
		cancelPlacement();
		sh.say(tr('grid.pivot.place.cancelled'));
	}

	function startPlacement(p: Placement<PivotConfig>) {
		sh.closeMenus();
		sh.menu = null;
		if (!sh.placing) window.addEventListener('keydown', onPlaceKey, true);
		sh.placing = p;
		sh.placeHover = null;
		sh.placeSize = { h: 1, w: 1 };
		sh.onplacing?.({ hint: placeHint(p), create: p.kind === 'create' });
		sh.wrap?.focus();
		sh.schedule();
	}

	/** Quitte la pose sans rien faire (Échap, bouton Annuler de la barre d'état). */
	function cancelPlacement() {
		if (!sh.placing) return;
		sh.placing = null;
		sh.placeHover = null;
		window.removeEventListener('keydown', onPlaceKey, true);
		sh.onplacing?.(null);
		if (sh.scroller) sh.scroller.style.cursor = '';
		sh.schedule();
	}

	/** « Nouvelle feuille » pendant la pose d'un nouveau tableau croisé : l'ancienne façon. */
	function placeOnNewSheet() {
		const p = sh.placing;
		if (p?.kind !== 'create') return;
		cancelPlacement();
		pivotOnNewSheet(p.config);
	}

	/** Clic droit › Déplacer : la pose d'un tableau croisé existant. */
	function startPivotMove(p: SheetPivot) {
		const top = p.anchor ?? p.row;
		startPlacement({ kind: 'move', id: p.id, name: p.config.name ?? '', sheet: sh.sheetId(), row: top, col: p.col });
		if (!p.error) sh.placeSize = { h: p.row + p.cells.length - top, w: p.cells[0]?.length ?? 1 };
	}

	/** Le clic sur la cellule : pose (création ou déplacement), en une étape d'annulation. */
	function placePivotAt(cell: { row: number; col: number }) {
		const p = sh.placing;
		if (!p) return;
		const op = placeAt(p, sh.sheetId(), cell.row, cell.col);
		cancelPlacement();
		if (op.op === 'none') {
			sh.say(placedMessage(op));
			return;
		}
		const run =
			op.op === 'create'
				? sh.source.createPivotAt && ((create) => () => create(op.row, op.col, op.config))(sh.source.createPivotAt)
				: sh.source.movePivot && ((move) => () => move(op.id, op.row, op.col))(sh.source.movePivot);
		if (!run) return;
		if (op.op === 'create') sh.openPivotOnLoad = true;
		sh.writing = sh.writing.catch(() => undefined).then(() =>
			run().then(
				() => {
					if (op.op === 'move') {
						sh.active = { row: op.row, col: op.col };
						sh.anchor = { ...sh.active };
					}
					sh.message = placedMessage(op);
					sh.refreshFx();
					sh.refreshStatus();
					sh.schedule();
				},
				(err) => {
					sh.openPivotOnLoad = false;
					sh.say(errorText(err));
				}
			)
		);
	}

	return {
		pivotActive,
		cancelPlacement,
		placeOnNewSheet,
		pivotHeld,
		removePivot,
		repointPivot,
		pickPivotSource,
		onPlaceKey,
		startPivotMove,
		placePivotAt
	};
}
