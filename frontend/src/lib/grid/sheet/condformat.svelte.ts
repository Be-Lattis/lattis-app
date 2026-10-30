// Mise en forme conditionnelle : statistiques des règles (en cache ou
// demandées au classeur), règles d'une cellule, volet des règles. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { type CellRange } from '../mergesel';
import { freeDisplay } from '../freecell';
import { applyRules, computeStats, mergeStats, prepareStats, reqKey, statsReq, type CfOut, type Rule, type RuleEnv, type Stats, type StatsReq } from '../rules';
import { formatLocs, groupKey, rectLocs, type Loc, type SheetRules } from '../cfmodel';
import { type CondView } from '../CondFormat.svelte';
import { t as tr } from '#lib/i18n/index.js';
import { type SheetPivot, type SheetSource, type Entry } from './model';

/** Ce que createCondFormat lit et modifie dans Sheet.svelte. */
export interface CondFormatHost {
	readonly cfStats: Map<string, Stats | null>;
	readonly cfStale: Map<string, Stats | null>;
	readonly cfGen: number;
	readonly cfWaits: Promise<unknown>[] | null;
	readonly schedule: () => void;
	readonly key: (r: number, c: number) => string;
	readonly shown: Map<string, { text: string; error: boolean; }>;
	readonly cells: Map<string, string>;
	readonly pivots: SheetPivot[];
	readonly pivotAt: (r: number, c: number) => SheetPivot | undefined;
	readonly sheetRowRules: Map<number, Rule[]>;
	readonly sheetColRules: Map<number, Rule[]>;
	readonly tableAt: (r: number, c: number) => Entry | undefined;
	readonly cellRules: Map<string, Rule[]>;
	readonly entries: Entry[];
	readonly colRules: Map<number, (Rule[] | undefined)[]>;
	readonly source: SheetSource;
	readonly cfToday: number;
	readonly freeMark: (row: number, col: number) => (r: Rule) => boolean;
	readonly noMark: () => boolean;
	readonly isNumeric: (s: string) => boolean;
	readonly selection: () => CellRange;
	readonly selectionLines: () => { r0: number; r1: number; c0: number; c1: number; rows: boolean; cols: boolean; };
	readonly locked: string;
	readonly say: (m: string) => void;
	readonly closeMenus: () => void;
	cfView: CondView | null;
	readonly selectRange: (r0: number, c0: number, r1: number, c1: number) => void;
	readonly wrap: HTMLDivElement;
}

export function createCondFormat(h: CondFormatHost) {
	function cfStat(k: string, compute: () => Stats | undefined | Promise<Stats | undefined>): Stats | undefined {
		if (h.cfStats.has(k)) return h.cfStats.get(k) ?? h.cfStale.get(k) ?? undefined;
		const got = compute();
		if (!(got instanceof Promise)) {
			h.cfStats.set(k, got ?? null);
			return got;
		}
		const gen = h.cfGen;
		h.cfStats.set(k, null);
		h.cfWaits?.push(got);
		got.then((s) => {
			if (gen !== h.cfGen) return;
			h.cfStats.set(k, s ?? null);
			h.schedule();
		}).catch(() => {});
		return h.cfStale.get(k) ?? undefined;
	}

	/** Valeur d'une cellule libre pour les règles (undefined : vide). */
	function freeValue(r: number, c: number): { v: unknown; text: string; error: boolean } | undefined {
		const k = h.key(r, c);
		const f = h.shown.get(k);
		const raw = h.cells.get(k);
		if (f === undefined && raw === undefined) {
			// Cellule d'un tableau croisé : sa valeur calculée (mise en forme
			// conditionnelle posée sur le TCD).
			const p = h.pivots.length ? h.pivotAt(r, c) : undefined;
			const text = p && r >= p.row ? (p.cells[r - p.row]?.[c - p.col] ?? '') : '';
			return text ? { v: ruleValue(text), text, error: false } : undefined;
		}
		const text = f ? f.text : freeDisplay(raw!, undefined).text;
		return { v: ruleValue(text), text, error: !!f?.error };
	}

	/**
	 * Statistiques de la plage d'une règle (clé gk, voir groupKey) : ses
	 * cellules libres, celles de ses lignes et colonnes entières, et ses
	 * colonnes de tableau (ou celles sous ses colonnes entières), en SQL.
	 */
	function rangeStats(gk: string, q: StatsReq): Stats | undefined {
		return cfStat(gk + '|' + reqKey(q), () => {
			const rows = new Set<number>();
			const cols = new Set<number>();
			for (const [i, rs] of h.sheetRowRules) if (rs.some((x) => groupKey(x, { k: 'row', i }) === gk)) rows.add(i);
			for (const [i, rs] of h.sheetColRules) if (rs.some((x) => groupKey(x, { k: 'col', i }) === gk)) cols.add(i);
			const values: unknown[] = [];
			const seen = new Set<string>();
			const add = (r: number, c: number) => {
				const k = h.key(r, c);
				if (seen.has(k) || h.tableAt(r, c)) return;
				seen.add(k);
				const fv = freeValue(r, c);
				if (fv && !fv.error) values.push(fv.v);
			};
			for (const [k, rs] of h.cellRules)
				if (rs.some((x) => groupKey(x, { k: 'cell', r: 0, c: 0 }) === gk)) {
					const [r, c] = k.split(',').map(Number);
					add(r, c);
				}
			if (rows.size || cols.size)
				for (const k of h.cells.keys()) {
					const [r, c] = k.split(',').map(Number);
					if (rows.has(r) || cols.has(c)) add(r, c);
				}
			const free = values.length ? computeStats(values, q) : undefined;
			const parts: Promise<Stats>[] = [];
			for (const e of h.entries) {
				const tr = h.colRules.get(e.t.id);
				const ci = e.t.columns.flatMap((_, i) => (tr?.[i]?.some((x) => groupKey(x, { k: 'tbl', t: e.t.id, c: i }) === gk) || cols.has(e.t.col + i) ? [i] : []));
				if (ci.length && h.source.ruleStats && e.t.rows > 0) parts.push(h.source.ruleStats(e.t, ci, q).then((s) => prepareStats(s, q)));
			}
			if (!parts.length) return free;
			return Promise.all(parts).then((list) => list.reduce<Stats | undefined>((a, b) => mergeStats(a, b), free));
		});
	}

	/** Contexte d'évaluation des règles d'un emplacement. */
	function cfEnv(loc: Loc, mark: (r: Rule, k: number) => boolean): RuleEnv {
		return {
			stats: (r) => {
				const q = statsReq(r);
				return q ? rangeStats(groupKey(r, loc), q) : undefined;
			},
			mark,
			today: h.cfToday
		};
	}

	/**
	 * Mise en forme conditionnelle d'une cellule : ses propres règles (colonne
	 * de tableau ou cellule libre), puis celles de sa ligne et de sa colonne
	 * entières (la ligne l'emporte, comme pour la mise en forme).
	 */
	function cfAt(
		r: number,
		c: number,
		val: { v: unknown; text: string; error: boolean },
		own?: { rules: Rule[] | undefined; loc: Loc; mark: (r: Rule, k: number) => boolean }
	): CfOut | undefined {
		const rr = h.sheetRowRules.get(r);
		const cr = h.sheetColRules.get(c);
		if (!own?.rules && !rr && !cr) return undefined;
		const out: CfOut = {};
		if (own?.rules) applyRules(own.rules, val, cfEnv(own.loc, own.mark), out);
		const fm = rr || cr ? h.freeMark(r, c) : h.noMark;
		if (rr) applyRules(rr, val, cfEnv({ k: 'row', i: r }, fm), out);
		if (cr) applyRules(cr, val, cfEnv({ k: 'col', i: c }, fm), out);
		return out;
	}

	/** Mise en forme conditionnelle d'une cellule libre (vide comprise). */
	function freeCf(r: number, c: number): CfOut | undefined {
		const k = h.key(r, c);
		const own = h.cellRules.get(k);
		if (!own && !h.sheetRowRules.size && !h.sheetColRules.size) return undefined;
		return cfAt(r, c, freeValue(r, c) ?? { v: null, text: '', error: false }, own ? { rules: own, loc: { k: 'cell', r, c }, mark: h.freeMark(r, c) } : undefined);
	}

	/** Valeur d'un texte affiché pour les règles : nombre si c'en est un. */
	function ruleValue(text: string): unknown {
		if (!h.isNumeric(text)) return text;
		const n = Number(text.replace(/[\s\u00a0\u202f%]/g, '').replace(',', '.'));
		return Number.isFinite(n) ? n : text;
	}

	/** Règles de la feuille, par emplacement (gestionnaire). */
	function sheetRules(): SheetRules {
		return {
			tables: h.entries.map((e) => ({ id: e.t.id, name: e.t.name, row: e.t.row, col: e.t.col, rows: e.t.rows, columns: e.t.columns.map((c) => c.name), rules: h.colRules.get(e.t.id) ?? [] })),
			rows: h.sheetRowRules,
			cols: h.sheetColRules,
			cells: h.cellRules
		};
	}

	/** Emplacements des règles de la sélection : lignes ou colonnes entières, colonnes de tableau, cellules libres. */
	function selectionLocs(): Loc[] | string {
		const { r0, r1, c0, c1 } = h.selection();
		const lines = h.selectionLines();
		if (lines.rows) return Array.from({ length: r1 - r0 + 1 }, (_, k) => ({ k: 'row' as const, i: r0 + k }));
		if (lines.cols) return Array.from({ length: c1 - c0 + 1 }, (_, k) => ({ k: 'col' as const, i: c0 + k }));
		return rectLocs(r0, c0, r1, c1, sheetRules().tables);
	}

	/** Menu « Mise en forme conditionnelle » du ruban, sous son bouton (coordonnées de la fenêtre). */
	function rulesMenu(x: number, y: number) {
		if (!h.source.setSheetRules) return;
		if (h.locked) return h.say(h.locked);
		h.closeMenus();
		h.cfView = { kind: 'menu', x, y };
	}

	/** Gestionnaire des règles de la sélection. */
	function rulesActive() {
		if (!h.source.setSheetRules) return;
		if (h.locked) return h.say(h.locked);
		h.closeMenus();
		h.cfView = { kind: 'manager' };
	}

	/** Gestionnaire des règles d'une colonne de tableau (menu ▾ de la colonne). */
	function openRules(e: Entry, i: number) {
		h.selectRange(e.t.row + 1, e.t.col + i, Math.max(e.t.row + 1, e.last), e.t.col + i);
		rulesActive();
	}

	/** « S'applique à » de la sélection (titre des boîtes). */
	function rulesTarget(): string {
		const l = selectionLocs();
		return typeof l === 'string' ? '' : formatLocs(l, sheetRules().tables);
	}

	function closeRules() {
		h.cfView = null;
		h.wrap.focus();
	}

	return {
		rulesMenu,
		rulesActive,
		rulesTarget,
		cfAt,
		freeCf,
		sheetRules,
		selectionLocs,
		openRules,
		closeRules
	};
}
