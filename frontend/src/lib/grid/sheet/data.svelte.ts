// Données reçues du classeur : tableaux posés (cache de lignes), cellules
// libres, styles, règles, fusions, TCD et graphiques ; relecture après une
// lecture refusée. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { RowCache } from '../cache';
import { VScroll } from '../scroll';
import { followClip, type Clip } from '../clipboard';
import { parseStyle, type StyleLayer } from '../style';
import { AutoRows } from '../rowfit';
import { hiddenLines, mergeList, type DvList, type MergeRange } from '../sheetfmt';
import { type Zone } from '../findzone';
import { type FindOpts, type FoundCell } from '../findopts';
import { type SparklineGroup } from '../sparkline';
import { choiceHiddenCols } from '#lib/shell/choice.js';
import { ReadRecovery, readFailNotice } from '../readfail';
import { bandMerges } from '../twolevel';
import { SheetViews } from '../sheetview';
import { type SheetObject } from '../objects';
import { type SheetButton } from '../buttons';
import { parseRules, todaySerial, type Rule, type Stats } from '../rules';
import { type FormulaHold } from '../crossref';
import { type SheetOutline } from '../outline';
import { formatColumnValue } from '../format';
import { type TypeMix, type SheetPivot, type SheetChart, type SheetTable, type SheetData, type SheetSource, type Entry, type WholeSel, type DvPick } from './model';

/** Ce que createData lit et modifie dans Sheet.svelte. */
export interface DataHost {
	readonly recovery: ReadRecovery;
	fetchError: string;
	readonly say: (m: string) => void;
	readonly source: SheetSource;
	entries: Entry[];
	readonly autoRows: AutoRows;
	readonly schedule: () => void;
	readonly refreshFx: () => void;
	placedSheet: number | undefined;
	readonly keepFormula: (from: number) => void;
	dvGen: number;
	readonly dvLists: Map<string, DvList>;
	readonly dvAfterData: (switching: boolean) => void;
	readonly sheetViews: SheetViews;
	active: { row: number; col: number; };
	anchor: { row: number; col: number; };
	readonly vs: VScroll;
	readonly scroller: HTMLDivElement;
	readonly placed: () => { id: number; r0: number; c0: number; r1: number; c1: number; }[];
	readonly finder: {
		replace: boolean;
		text: string;
		by: string;
		count: string;
		info: string;
		zone: Zone | null;
		at: { row: number; col: number } | null;
		opts: FindOpts;
		more: boolean;
		list: FoundCell[] | null;
		total: number;
		busy: boolean;
		current: number;
	} | null;
	matchKey: string;
	charts: SheetChart[];
	cells: Map<string, string>;
	readonly key: (r: number, c: number) => string;
	readonly mixes: Map<string, Promise<TypeMix | null>>;
	tipCell: string;
	clipRange: Clip | null;
	pivots: SheetPivot[];
	openPivotOnLoad: boolean;
	panel: { kind: 'pivot' | 'chart'; id: number } | null;
	buttons: SheetButton[];
	sheetObjects: SheetObject[];
	sparkGroups: SparklineGroup[];
	controlsTick: number;
	spills: { row: number; col: number; rows: string[][] }[];
	shown: Map<string, { text: string; error: boolean; }>;
	freeLinks: Map<string, string>;
	userWidths: Map<number, number>;
	userHeights: Map<number, number>;
	names: Map<string, string>;
	cellRules: Map<string, Rule[]>;
	freeStyles: Map<string, StyleLayer>;
	freeMarks: Set<string>;
	cfStats: Map<string, Stats | null>;
	readonly cfStale: Map<string, Stats | null>;
	cfGen: number;
	cfToday: number;
	sheetRowStyles: Map<number, StyleLayer>;
	sheetColStyles: Map<number, StyleLayer>;
	merges: MergeRange[];
	boxes: MergeRange[];
	readonly isSplit: (t: { name?: string; }, name: string) => boolean;
	outline: SheetOutline | null;
	styleHiddenRows: Set<number>;
	hiddenRows: Set<number>;
	styleHiddenCols: Set<number>;
	hiddenCols: Set<number>;
	sheetRowRules: Map<number, Rule[]>;
	sheetColRules: Map<number, Rule[]>;
	colStyles: Map<number, (StyleLayer | undefined)[]>;
	headStyles: Map<number, (StyleLayer | undefined)[]>;
	colRules: Map<number, (Rule[] | undefined)[]>;
	readonly rebuildAxis: (override?: { col: number; width: number; }) => void;
	readonly topRow: (t: { row: number; twoLevel?: boolean; }) => number;
	readonly measureHeights: () => Map<number, number>;
	readonly rebuildRows: (override?: { row: number; height: number; }) => void;
	wholeSel: WholeSel | null;
	readonly dvPick: DvPick | null;
	readonly finishPick: (p: DvPick, ref: string | null | undefined) => void;
	readonly updateExtent: () => void;
	readonly syncNative: () => void;
	readonly findArrived: (sheet: number) => void;
	linkGo: { sheet: number; r0: number; c0: number; r1: number; c1: number } | null;
	readonly selectPlace: (p: { r0: number; c0: number; r1: number; c1: number; }) => void;
	readonly startAnims: (nav: boolean, known: Set<number>, oldCharts: Set<number>, now: number) => void;
	readonly refreshStatus: () => void;
	readonly hold: FormulaHold | null;
	readonly placeHold: (sheet: number) => void;
}

export function createData(sh: DataHost) {
	/**
	 * Lecture d'un tableau refusée (le bloc n'est pas redemandé, cache.ts) :
	 * la mise en page est relue et le tableau rechargé, une fois, avec un
	 * message en français ; le détail technique va dans la console (le moteur
	 * l'écrit aussi dans son journal).
	 */
	function readFailed(e: Entry, err: string) {
		console.warn(`[grille] lecture du tableau « ${e.t.name} » (${e.t.rows} lignes affichées) refusée : ${err}`);
		if (sh.recovery.take(e.t.id, Date.now())) {
			sh.fetchError = err;
			sh.say(readFailNotice(e.t.name));
			if (sh.source.reload) void sh.source.reload().catch(() => {});
			else e.cache.invalidate(e.t.rows);
			return;
		}
		if (err !== sh.fetchError) {
			sh.fetchError = err;
			sh.say(readFailNotice(e.t.name));
		}
	}

	function makeEntry(t: SheetTable): Entry {
		const e: Entry = {
			t,
			last: t.row + t.rows,
			lastCol: t.col + t.columns.length - 1,
			cache: new RowCache(
				(first, count) => sh.source.window(e.t, first, count),
				t.rows,
				(c, v) => formatColumnValue(e.t.columns[c], v),
				(first, count) => {
					// Lecture refusée : un tableau quitté (bloc en route au
					// changement d'onglet) ne dit rien ; sinon rechargé.
					const err = e.cache.error;
					if (err && sh.entries.includes(e)) readFailed(e, err);
					// Bloc arrivé : ses lignes se mesurent (hauteur automatique) au prochain dessin.
					if (first !== undefined && count) sh.autoRows.forget(e.t.row + 1 + first, count);
					sh.schedule();
					sh.refreshFx();
				}
			)
		};
		return e;
	}

	/** Recalcule la géométrie quand la mise en page change (après une saisie). */
	function applyData(d: SheetData) {
		// Changement de feuille pendant la saisie d'une formule : la saisie est
		// gardée (texte, curseur, cellule et feuille d'origine), pour pointer
		// des références sur l'autre feuille (crossref.ts).
		const switching = sh.placedSheet !== undefined && sh.placedSheet !== (d.sheet ?? 0);
		if (switching) sh.keepFormula(sh.placedSheet!);
		// Listes tirées d'une plage : relues (la source a pu changer).
		sh.dvGen++;
		sh.dvLists.clear();
		sh.dvAfterData(switching);
		// Changement de feuille : garder la position de la feuille quittée,
		// retrouver celle de la nouvelle (A1 si elle n'a pas été visitée).
		const back = sh.sheetViews.switchTo(d.sheet ?? 1, { active: { ...sh.active }, anchor: { ...sh.anchor }, top: sh.vs.top, left: sh.scroller?.scrollLeft ?? 0 });
		const old = new Map(sh.entries.map((e) => [e.t.id, e]));
		sh.fetchError = '';
		const placedBefore = sh.placedSheet === (d.sheet ?? 0) ? sh.placed() : null;
		// Navigation (ouverture, autre feuille) : rien ne s'anime.
		const nav = sh.placedSheet !== (d.sheet ?? 0);
		if (nav) sh.recovery.reset();
		// Recherche : occurrences relues après une saisie, un Ctrl+Z, une actualisation ou sur une autre feuille
		// (sauf dans tout le classeur : passer d'une feuille à l'autre ne les change pas).
		if (!nav || !sh.finder?.opts.book) sh.matchKey = '';
		if (nav && sh.finder) {
			sh.finder.at = null;
			sh.finder.count = '';
			sh.finder.info = '';
		}
		const oldCharts = new Set(sh.charts.map((c) => c.id));
		sh.entries =d.tables.map((t) => {
			const e = old.get(t.id);
			if (!e) return makeEntry(t);
			// Colonnes existantes déplacées (une colonne ajoutée devant, par
			// exemple) : l'ancien contenu ne s'affiche pas en attendant.
			const kept = e.t.columns.every((c, i) => t.columns[i]?.name === c.name);
			e.t = t;
			e.last = t.row + t.rows;
			e.lastCol = t.col + t.columns.length - 1;
			e.cache.invalidate(t.rows, kept);
			return e;
		});
		sh.cells = new Map(d.cells.map((c) => [sh.key(c.row, c.col), c.raw]));
		sh.mixes.clear();
		sh.tipCell = '';
		// Cadre de copie : il suit le tableau copié poussé par un collage, ou
		// disparaît s'il ne désigne plus ce qui a été copié.
		if (sh.clipRange && sh.clipRange.sheet === (d.sheet ?? 0) && placedBefore) sh.clipRange = followClip(sh.clipRange, placedBefore, sh.placed());
		sh.placedSheet = d.sheet ?? 0;
		sh.pivots = (d.pivots ?? []).map((p) => {
			// Pas encore calculé (LayoutFirst) : traité comme un TCD en erreur
			// (une seule cellule, sans contenu), avec son propre message.
			if (p.pending) p = { ...p, error: p.error || '…', cells: [] };
			const f = p.error ? 0 : (p.config.filters?.length ?? 0);
			return { ...p, anchor: p.row, row: p.row + (f ? f + 1 : 0) + (p.error ? 0 : (p.colHead?.length ?? 0)) };
		});
		if (sh.openPivotOnLoad && sh.pivots.length) {
			sh.openPivotOnLoad = false;
			const p = sh.pivots[sh.pivots.length - 1];
			sh.panel = { kind: 'pivot', id: p.id };
			sh.active = { row: p.row, col: p.col };
			sh.anchor = { ...sh.active };
		}
		sh.charts = d.charts ?? [];
		sh.buttons = d.buttons ?? [];
		sh.sheetObjects = d.objects ?? [];
		sh.sparkGroups = d.sparklines ?? [];
		sh.controlsTick++;
		sh.spills = d.cells.filter((c) => c.spill?.length).map((c) => ({ row: c.row, col: c.col, rows: c.spill! }));
		sh.shown = new Map(d.cells.filter((c) => c.raw.startsWith('=')).map((c) => [sh.key(c.row, c.col), { text: c.value ?? '', error: !!c.error }]));
		sh.freeLinks = new Map(d.cells.filter((c) => c.link).map((c) => [sh.key(c.row, c.col), c.link!]));
		sh.userWidths = new Map((d.widths ?? []).map((w) => [w.col, w.width]));
		sh.userHeights = new Map((d.heights ?? []).map((h) => [h.row, h.height]));
		sh.names = new Map((d.names ?? []).map((n) => [sh.key(n.row, n.col), n.name]));
		// Règles des cellules libres : dans leur style (propriété cf), mises à part.
		sh.cellRules = new Map();
		sh.freeStyles = new Map(
			(d.styles ?? []).map((s) => {
				const l = (parseStyle(s.style) ?? {}) as StyleLayer & { cf?: Rule[] };
				if (Array.isArray(l.cf) && l.cf.length) sh.cellRules.set(sh.key(s.row, s.col), l.cf.map((r) => ({ ...r, style: r.style ?? {} })));
				delete l.cf;
				return [sh.key(s.row, s.col), l];
			})
		);
		sh.freeMarks = new Set((d.ruleMarks ?? []).map((m) => `${m.row},${m.col},${m.rule}`));
		for (const [k, v] of sh.cfStats) if (v) sh.cfStale.set(k, v);
		sh.cfStats = new Map();
		sh.cfGen++;
		sh.cfToday = todaySerial();
		sh.sheetRowStyles = new Map((d.rowStyles ?? []).map((s) => [s.index, parseStyle(s.style) ?? {}]));
		sh.sheetColStyles = new Map((d.colStyles ?? []).map((s) => [s.index, parseStyle(s.style) ?? {}]));
		sh.merges = mergeList(sh.freeStyles);
		sh.boxes = [...sh.merges, ...bandMerges(d.tables, sh.isSplit)];
		sh.outline = d.outline ?? null;
		sh.styleHiddenRows = hiddenLines(sh.sheetRowStyles);
		sh.hiddenRows = new Set([...sh.styleHiddenRows, ...(sh.outline?.hiddenRows ?? [])]);
		sh.styleHiddenCols = hiddenLines(sh.sheetColStyles);
		// Onglet à choix sur une valeur : la colonne de catégorie (la même
		// partout) n'est pas affichée ; elle revient avec « (Toutes) ».
		sh.hiddenCols = new Set([...sh.styleHiddenCols, ...(sh.outline?.hiddenCols ?? []), ...choiceHiddenCols(d.choice, d.tables, d.cells)]);
		sh.sheetRowRules = new Map((d.rowRules ?? []).map((s) => [s.index, parseRules(s.rules) ?? []]));
		sh.sheetColRules = new Map((d.colRules ?? []).map((s) => [s.index, parseRules(s.rules) ?? []]));
		sh.colStyles = new Map(d.tables.map((t) => [t.id, t.columns.map((c) => parseStyle(c.style))]));
		sh.headStyles = new Map(d.tables.map((t) => [t.id, t.columns.map((c) => parseStyle(c.headerStyle))]));
		sh.colRules = new Map(d.tables.map((t) => [t.id, t.columns.map((c) => parseRules(c.rules))]));
		sh.rebuildAxis();
		// Données ou mise en forme changées : les lignes de tableau affichées se
		// remesurent (elles gardent leur hauteur d'ici là, sans clignoter).
		if (back) sh.autoRows.clear();
		else sh.autoRows.forgetAll((r) => sh.entries.some((e) => r >= sh.topRow(e.t) && r <= e.t.row + e.t.rows));
		sh.autoRows.free = sh.measureHeights();
		sh.rebuildRows();
		if (back) {
			sh.active = back.active;
			sh.anchor = back.anchor;
			sh.wholeSel = null;
		}
		// Plage source d'une validation pointée sur une autre feuille : de
		// retour sur la feuille de la cellule, la sélection d'avant revient.
		const pick = sh.dvPick;
		if (pick?.back && (d.sheet ?? 0) === pick.sheet) sh.finishPick(pick, pick.ref);
		sh.updateExtent();
		if (back && sh.scroller) {
			sh.vs.to(back.top);
			sh.scroller.scrollLeft = back.left;
			sh.updateExtent();
			sh.syncNative();
		}
		// Occurrence trouvée sur une autre feuille : arrivé, elle se sélectionne.
		sh.findArrived(d.sheet ?? 0);
		// Lien suivi vers une autre feuille : arrivé, sa destination se sélectionne.
		const go = sh.linkGo;
		if (go && (d.sheet ?? 0) === go.sheet) {
			sh.linkGo = null;
			sh.selectPlace(go);
		}
		sh.startAnims(nav, new Set(old.keys()), oldCharts, performance.now());
		sh.refreshFx();
		if (back) sh.refreshStatus();
		sh.schedule();
		if (switching && sh.hold) {
			const sheet = d.sheet ?? 0;
			queueMicrotask(() => sh.placeHold(sheet));
		}
	}

	return {
		applyData
	};
}
