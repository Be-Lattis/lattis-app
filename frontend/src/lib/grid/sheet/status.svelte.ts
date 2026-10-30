// Barre d'état : cellule active, statistiques de la sélection (locales ou
// calculées par le classeur), temps de dessin. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { noticeTtl } from '#lib/shell/notice.js';
import { percentile } from '../cache';
import { formatStat, type CellStyle } from '../style';
import { type Cell, type CellRange } from '../mergesel';
import { headerLabel, spanAt } from '../twolevel';
import { editValue, typeLabel } from '../format';
import { t as tr } from '#lib/i18n/index.js';
import { type SheetPivot, type SheetSource, type SheetStatus, type Entry, type DvPick } from './model';

/** Ce que createStatus lit et modifie dans Sheet.svelte. */
export interface StatusHost {
	readonly tableAt: (r: number, c: number) => Entry | undefined;
	readonly groupBandAt: (r: number, c: number) => Entry | undefined;
	readonly singleCol: (e: Entry, i: number) => boolean;
	readonly cells: Map<string, string>;
	readonly key: (r: number, c: number) => string;
	readonly names: Map<string, string>;
	readonly ownerAt: (r: number, c: number) => Entry | undefined;
	readonly syncFinderZone: () => void;
	readonly dvPick: DvPick | null;
	dvPickRef: string;
	readonly pickedRef: () => string | null;
	readonly prefetchDv: () => void;
	readonly message: string;
	shownMessage: string;
	messageTimer: ReturnType<typeof setTimeout> | undefined;
	readonly clearMessage: () => void;
	readonly entries: Entry[];
	readonly active: { row: number; col: number; };
	readonly onstatus: ((s: SheetStatus) => void) | undefined;
	readonly noteOf: (r: number, c: number) => string | undefined;
	readonly activeThread: () => boolean;
	readonly ms: (v: number) => string;
	readonly drawTimes: number[];
	readonly frameStamps: number[];
	readonly lastEditMs: number;
	readonly selection: () => CellRange;
	readonly activeStyle: () => CellStyle;
	readonly mergedAt: (r: number, c: number) => boolean;
	readonly styleAt: (r: number, c: number, e?: Entry | undefined) => CellStyle | undefined;
	readonly source: SheetSource;
	readonly statsEpoch: number;
	readonly shown: Map<string, { text: string; error: boolean; }>;
	readonly isNumeric: (s: string) => boolean;
	readonly singleSelection: () => boolean;
	readonly pivotAt: (r: number, c: number) => SheetPivot | undefined;
}

export function createStatus(h: StatusHost) {
	function pushStat(a: number[], v: number) {
		a.push(v);
		if (a.length > 300) a.shift();
	}

	/** Ce que contient une cellule, pour l'édition et la barre de formule. */
	function cellInfo(r: number, c: number) {
		const e = h.tableAt(r, c);
		if (!e) {
			// Bande des groupes : la saisie y renomme le groupe (le moteur s'en charge).
			const g = h.groupBandAt(r, c);
			// Colonne sur un niveau : la bande et l'en-tête sont une cellule, son nom.
			if (g && h.singleCol(g, c - g.t.col)) return { e: g, i: c - g.t.col, kind: 'header' as const, value: g.t.columns[c - g.t.col].name };
			if (g) return { e, kind: 'free' as const, value: spanAt(g.t.columns.map((k) => k.name), c - g.t.col)?.group ?? '' };
			return { e, kind: 'free' as const, value: h.cells.get(h.key(r, c)) ?? '' };
		}
		const i = c - e.t.col;
		if (r === e.t.row) return { e, i, kind: 'header' as const, value: headerLabel(e.t.columns[i].name, e.t.twoLevel) };
		const n = r - e.t.row - 1;
		const row = e.cache.row(n);
		return {
			e,
			i,
			n,
			row,
			kind: 'data' as const,
			// Une saisie hors type se corrige à partir de sa valeur brute.
			// Colonne calculée : la barre de formule montre la formule, comme Excel.
			value: e.t.columns[i].formula ?? (row ? (row.bad?.get(i) ?? editValue(e.t.columns[i].type, row.cells[i])) : '')
		};
	}

	/** Désignation de la cellule active (pas d'adresse A1). */
	/** Adresse d'une cellule libre, telle qu'elle s'écrit dans une formule ou en SQL : Cell[A1]. */
	function cellAddr(r: number, c: number): string {
		return `Cell[${addrA1(r, c)}]`;
	}

	/** Lettre(s) de colonne et numéro de ligne : A1. */
	function addrA1(r: number, c: number): string {
		let s = '';
		for (let n = c + 1; n > 0; n = Math.floor((n - 1) / 26)) s = String.fromCharCode(65 + ((n - 1) % 26)) + s;
		return s + (r + 1);
	}

	function cellName(r: number, c: number): string {
		const named = h.names.get(h.key(r, c));
		if (named) return named;
		const e = h.ownerAt(r, c);
		// Cellule libre : lettre et numéro seulement (A1) ; Cell_A1 dans une formule.
		if (!e) return addrA1(r, c);
		const k = e.t.columns[c - e.t.col];
		const one = r < e.t.row && h.singleCol(e, c - e.t.col);
		if (r < e.t.row && !one) {
			const s = spanAt(e.t.columns.map((x) => x.name), c - e.t.col);
			const n = s ? s.i1 - s.i0 + 1 : 1;
			return s?.group ? tr('grid.status.group', { table: e.t.name, group: s.group, n }) : tr('grid.status.groupEmpty', { table: e.t.name });
		}
		const col = `${e.t.name}[${k.name}]`;
		if (r === e.t.row || one) {
			const errs = k.errors ? ' · ' + tr('grid.status.errors', { n: k.errors }) : '';
			return `${col} · ${tr(k.fixed ? 'grid.status.type.fixed' : 'grid.status.type.inferred', { type: typeLabel(k.type) })}${errs}` + (e.t.twoLevel && h.singleCol(e, c - e.t.col) ? ' · ' + tr('grid.status.oneLevel') : '');
		}
		const bad = e.cache.row(r - e.t.row - 1)?.bad?.get(c - e.t.col);
		return `${col} · ${r - e.t.row}` + (bad !== undefined ? ' · ' + tr('grid.status.badType', { value: String(bad), type: typeLabel(k.type) }) : '');
	}

	function refreshStatus() {
		h.syncFinderZone();
		if (h.dvPick) h.dvPickRef = h.pickedRef() ?? '';
		else h.prefetchDv();
		if (h.message !== h.shownMessage) {
			h.shownMessage = h.message;
			clearTimeout(h.messageTimer);
			if (h.message) {
				const m = h.message;
				h.messageTimer = setTimeout(() => {
					if (h.message === m) h.clearMessage();
				}, noticeTtl(m));
			}
		}
		const stats = h.entries[0]?.cache.stats ?? { roundTrip: [], server: [] };
		const here = h.ownerAt(h.active.row, h.active.col);
		h.onstatus?.({
			note: !!h.noteOf(h.active.row, h.active.col),
			comment: h.activeThread(),
			filters: !!here?.t.columns.some((c) => c.filter),
			errorsOnly: !!here?.t.columns[h.active.col - here.t.col]?.filter?.errors,
			readOnly: here?.t.readOnly,
			// Une cellule libre : son adresse est déjà dans la zone de nom.
			cell: here || h.names.has(h.key(h.active.row, h.active.col)) ? cellName(h.active.row, h.active.col) : '',
			message: h.message,
			fetch: tr('grid.status.perf.fetch', { ms: h.ms(percentile(stats.roundTrip, 0.5)) }),
			draw: tr('grid.status.perf.draw', { ms: h.ms(percentile(h.drawTimes, 0.5)) }),
			fps: tr('grid.status.perf.fps', { n: String(h.frameStamps.length) }),
			edit: tr('grid.status.perf.edit', { ms: h.ms(h.lastEditMs) }),
			blocks: h.entries.reduce((n, e) => n + e.cache.size, 0),
			stats: selectionStats(),
			sel: h.selection(),
			style: h.activeStyle(),
			merged: h.mergedAt(h.active.row, h.active.col)
		});
	}

	let statsCache: { key: string; text: string } | null = null;
	let statsPending = '';
	let statsTimer = 0;

	function statsText(count: number, nums: number, sum: number): string {
		if (count === 0) return '';
		const parts = [];
		// Comme Excel : au format de la cellule active, sinon arrondies.
		let st = h.styleAt(h.active.row, h.active.col);
		// Moyenne et somme à deux décimales (demande de Louis, 21/10), sauf
		// nombre de décimales fixé par le format de la cellule.
		if (st?.dp === undefined) st = { ...st, dp: 2 };
		if (nums) parts.push(tr('grid.status.average', { value: formatStat(sum / nums, st) }));
		parts.push(tr('grid.status.count', { n: count }));
		if (nums) parts.push(tr('grid.status.sum', { value: formatStat(sum, st) }));
		return parts.join('  •  ');
	}

	/** Totaux de la sélection par le moteur (SQL) : cellules libres et en-têtes ici, données là-bas. */
	function remoteStats(r0: number, r1: number, c0: number, c1: number): string {
		const rs = h.source.rangeStats;
		if (!rs) return '';
		const key = `${h.statsEpoch}|${r0},${r1},${c0},${c1}`;
		if (statsCache?.key === key) return statsCache.text;
		if (statsPending !== key) {
			statsPending = key;
			window.clearTimeout(statsTimer);
			statsTimer = window.setTimeout(async () => {
				let count = 0;
				let nums = 0;
				let sum = 0;
				try {
					for (const e of h.entries) {
						if (e.t.row > r1 || e.last < r0 || e.t.col > c1 || e.lastCol < c0) continue;
						const ka = Math.max(c0, e.t.col) - e.t.col;
						const kb = Math.min(c1, e.lastCol) - e.t.col;
						if (r0 <= e.t.row && e.t.row <= r1) count += kb - ka + 1; // en-têtes
						const first = Math.max(r0, e.t.row + 1) - e.t.row - 1;
						const last = Math.min(r1, e.last) - e.t.row - 1;
						if (last < first) continue;
						const cols = Array.from({ length: kb - ka + 1 }, (_, i) => ka + i);
						const st = await rs(e.t, first, last - first + 1, cols);
						count += st.count;
						nums += st.nums;
						sum += st.sum;
					}
				} catch {
					// Échec : pas de nouvel essai pour cette sélection (pas de boucle).
					if (statsPending === key) {
						statsPending = '';
						statsCache = { key, text: '' };
						refreshStatus();
					}
					return;
				}
				// Cellules libres : toutes en mémoire.
				for (const [k, raw] of h.cells) {
					const [r, c] = k.split(',').map(Number);
					if (r < r0 || r > r1 || c < c0 || c > c1 || h.tableAt(r, c)) continue;
					const f = h.shown.get(k);
					let v: unknown = f ? f.text : raw;
					if (v === null || v === undefined || v === '') continue;
					count++;
					if (typeof v === 'string' && h.isNumeric(v)) v = Number(v.replace(/[\s\u00a0\u202f]/g, '').replace(',', '.'));
					if (typeof v === 'number' && Number.isFinite(v)) {
						nums++;
						sum += v;
					}
				}
				if (statsPending !== key) return; // sélection changée entre-temps
				statsPending = '';
				statsCache = { key, text: statsText(count, nums, sum) };
				refreshStatus();
			}, 150);
		}
		return statsCache && statsCache.key.endsWith(`|${r0},${r1},${c0},${c1}`) ? statsCache.text : tr('grid.status.computing');
	}

	/** « Moyenne · Nb · Somme » de la sélection, si plusieurs cellules. */
	function selectionStats(): string {
		const { r0, r1, c0, c1 } = h.selection();
		const n = (r1 - r0 + 1) * (c1 - c0 + 1);
		if (n < 2 || h.singleSelection()) return '';
		// Grande sélection (colonne entière d'un million de lignes…) : SQL.
		if (n > 50_000) return h.source.rangeStats ? remoteStats(r0, r1, c0, c1) : tr('grid.status.tooBig');
		let count = 0;
		let nums = 0;
		let sum = 0;
		let missing = false;
		for (let r = r0; r <= r1; r++) {
			for (let c = c0; c <= c1; c++) {
				const e = h.tableAt(r, c);
				let v: unknown;
				if (e) {
					if (r === e.t.row) {
						count++;
						continue;
					}
					const row = e.cache.row(r - e.t.row - 1);
					if (!row) {
						missing = true;
						continue;
					}
					v = row.bad?.has(c - e.t.col) ? row.bad.get(c - e.t.col) : row.cells[c - e.t.col];
				} else {
					// Tableau crois\u00e9 : ses cellules calcul\u00e9es comptent aussi (nombre, somme, moyenne).
					const p = h.pivotAt(r, c);
					const pv = p && r >= p.row ? p.cells[r - p.row]?.[c - p.col] : undefined;
					const f = h.shown.get(h.key(r, c));
					v = pv !== undefined ? pv : f ? f.text : h.cells.get(h.key(r, c));
					if (typeof v === 'string' && h.isNumeric(v)) v = Number(v.replace(/[\s\u00a0\u202f]/g, '').replace(',', '.'));
				}
				if (v === null || v === undefined || v === '') continue;
				count++;
				if (typeof v === 'number' && Number.isFinite(v)) {
					nums++;
					sum += v;
				}
			}
		}
		// Lignes pas encore chargées : le moteur fait le compte exact.
		if (missing && h.source.rangeStats) return remoteStats(r0, r1, c0, c1);
		return statsText(count, nums, sum) + (missing ? ' ' + tr('grid.status.loading') : '');
	}

	return {
		pushStat,
		cellInfo,
		cellAddr,
		addrA1,
		cellName,
		refreshStatus
	};
}
