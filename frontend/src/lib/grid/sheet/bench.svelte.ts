// Mesure de la grille (mode LATTIS_BENCH, Démo › Mesurer) : défilement
// simulé, temps de dessin, images par seconde. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { percentile } from '../cache';
import { VScroll } from '../scroll';
import { tableTop } from '../twolevel';
import { editValue } from '../format';
import { t as tr } from '#lib/i18n/index.js';
import { type SheetSource, type Entry } from './model';
import { ROW_H } from './consts';

/** Ce que createBench lit et modifie dans Sheet.svelte. */
export interface BenchHost {
	readonly entries: Entry[];
	bench: string;
	readonly rowTop: (r: number) => number;
	readonly rspan: (r0: number, r1: number) => number;
	readonly vs: VScroll;
	readonly syncNative: () => void;
	missingFrames: number;
	readonly scroller: HTMLDivElement;
	readonly draw: () => void;
	readonly visibleRows: () => { first: number; last: number; };
	readonly visibleData: (e: Entry, first: number, last: number) => { first: number; last: number; } | null;
	readonly source: SheetSource;
	readonly drawTimes: number[];
	readonly refreshStatus: () => void;
}

export function createBench(h: BenchHost) {
	/** Défilement continu, sauts aléatoires et saisies, sur le premier tableau. */
	async function runBench() {
		const e = h.entries[0];
		if (!e) return;
		h.bench = tr('grid.bench.running');
		const frames = (n: number) =>
			new Promise<void>((res) => {
				let k = 0;
				const step = () => (++k >= n ? res() : requestAnimationFrame(step));
				requestAnimationFrame(step);
			});
		const tableTop = h.rowTop(e.t.row + 1);
		const tableSpan = Math.max(0, h.rspan(e.t.row + 1, e.t.row + e.t.rows) - h.vs.view);

		h.vs.to(tableTop + Math.random() * tableSpan * 0.5);
		h.syncNative();
		await frames(30);
		const intervals: number[] = [];
		h.missingFrames = 0;
		let drawn = 0;
		let prev = performance.now();
		const end = prev + 5000;
		await new Promise<void>((res) => {
			const step = () => {
				const now = performance.now();
				intervals.push(now - prev);
				prev = now;
				drawn++;
				h.vs.by(10 * ROW_H);
				h.scroller.scrollTop = h.vs.toNative();
				h.draw();
				if (now < end) requestAnimationFrame(step);
				else res();
			};
			requestAnimationFrame(step);
		});
		const scrollMissing = h.missingFrames;
		const janky = intervals.filter((d) => d > 1000 / 30).length;

		const jumps: number[] = [];
		for (let i = 0; i < 20; i++) {
			h.vs.to(tableTop + Math.random() * tableSpan);
			h.syncNative();
			const t0 = performance.now();
			const { first, last } = h.visibleRows();
			const d = h.visibleData(e, first, last)!;
			await new Promise<void>((res) => {
				const check = () => (e.cache.has(d.first, d.last) ? res() : requestAnimationFrame(check));
				check();
			});
			jumps.push(performance.now() - t0);
			await frames(2);
		}

		const edits: number[] = [];
		const { first: f0, last: f1 } = h.visibleRows();
		const d0 = h.visibleData(e, f0, f1)!;
		for (let i = 0; i < 20; i++) {
			const n = d0.first + i;
			const row = e.cache.row(n);
			if (!row) continue;
			const t0 = performance.now();
			await h.source.setCell(e.t.row + 1 + n, e.t.col + 1, editValue(e.t.columns[1].type, row.cells[1]), {
				table: e.t,
				column: 1,
				index: n,
				rowId: row.id
			});
			edits.push(performance.now() - t0);
		}

		const result = {
			date: new Date().toISOString(),
			userAgent: navigator.userAgent,
			rows: e.t.rows,
			scroll: {
				frames: drawn,
				fps: Math.round(drawn / 5),
				frameP50: +percentile(intervals, 0.5).toFixed(1),
				frameP95: +percentile(intervals, 0.95).toFixed(1),
				framesOver33ms: janky,
				framesWithPlaceholders: scrollMissing
			},
			jumpToDataP50: +percentile(jumps, 0.5).toFixed(1),
			jumpToDataP95: +percentile(jumps, 0.95).toFixed(1),
			editP50: +percentile(edits, 0.5).toFixed(2),
			editP95: +percentile(edits, 0.95).toFixed(2),
			drawP50: +percentile(h.drawTimes, 0.5).toFixed(2),
			drawP95: +percentile(h.drawTimes, 0.95).toFixed(2),
			blockRoundTripP50: +percentile(e.cache.stats.roundTrip, 0.5).toFixed(2),
			blockRoundTripP95: +percentile(e.cache.stats.roundTrip, 0.95).toFixed(2),
			blockGoP50: +percentile(e.cache.stats.server, 0.5).toFixed(2)
		};
		console.log('lattis-bench', JSON.stringify(result));
		(window as unknown as { lattisBench: unknown }).lattisBench = result;
		h.bench = tr('grid.bench.result', { fps: String(result.scroll.fps), frame: String(result.scroll.frameP95), janky: String(janky), missing: String(scrollMissing), drawn: String(drawn), jump: String(result.jumpToDataP50), edit: String(result.editP50) });
		h.refreshStatus();
		return result;
	}

	return {
		runBench
	};
}
