// Cache de lignes par blocs, côté front.
//
// La grille ne demande jamais une ligne isolée : elle déclare la plage visible
// (want) et le cache charge les blocs manquants, les plus proches d'abord, avec
// un nombre borné de requêtes en vol. Lors d'un défilement rapide à la barre,
// les blocs qui ne sont plus visibles sortent de la file avant d'être demandés.

export const BLOCK = 200;

export interface Fetched {
	first: number;
	ids: number[] | null;
	cells: (unknown[] | null)[] | null;
	micros: number;
}

export type Fetcher = (first: number, count: number) => Promise<Fetched>;

export interface Block {
	ids: number[];
	cells: unknown[][];
	text: string[][];
}

export interface Stats {
	/** Aller-retour complet d'un bloc (IPC + SQLite + JSON), en ms. */
	roundTrip: number[];
	/** Temps passé côté Go, en ms. */
	server: number[];
}

export class RowCache {
	private blocks = new Map<number, Block>();
	private inflight = new Set<number>();
	private queue: number[] = [];
	readonly stats: Stats = { roundTrip: [], server: [] };
	error: string | null = null;

	constructor(
		private fetcher: Fetcher,
		private rows: number,
		private format: (col: number, v: unknown) => string,
		private onLoad: () => void,
		private maxBlocks = 300,
		private maxInflight = 4
	) {}

	/** Ligne `row` si son bloc est chargé. */
	row(row: number): { id: number; cells: unknown[]; text: string[] } | undefined {
		const b = this.blocks.get(Math.floor(row / BLOCK));
		if (!b) return undefined;
		const i = row % BLOCK;
		return { id: b.ids[i], cells: b.cells[i], text: b.text[i] };
	}

	has(first: number, last: number): boolean {
		for (let b = Math.floor(first / BLOCK); b <= Math.floor(last / BLOCK); b++) {
			if (!this.blocks.has(b)) return false;
		}
		return true;
	}

	/** Déclare la plage visible [first, last] ; précharge un bloc de part et d'autre. */
	want(first: number, last: number) {
		const lo = Math.max(0, Math.floor(first / BLOCK) - 1);
		const hi = Math.min(Math.floor((this.rows - 1) / BLOCK), Math.floor(last / BLOCK) + 1);
		const mid = (first + last) / 2 / BLOCK;
		const q: number[] = [];
		for (let b = lo; b <= hi; b++) {
			if (!this.blocks.has(b) && !this.inflight.has(b)) q.push(b);
		}
		q.sort((a, b) => Math.abs(a - mid) - Math.abs(b - mid));
		this.queue = q;
		this.pump();
	}

	private pump() {
		while (this.inflight.size < this.maxInflight && this.queue.length > 0) {
			const b = this.queue.shift()!;
			if (this.blocks.has(b) || this.inflight.has(b)) continue;
			this.load(b);
		}
	}

	private async load(b: number) {
		this.inflight.add(b);
		const t0 = performance.now();
		try {
			const w = await this.fetcher(b * BLOCK, BLOCK);
			push(this.stats.roundTrip, performance.now() - t0);
			push(this.stats.server, w.micros / 1000);
			const cells = (w.cells ?? []).map((r) => r ?? []);
			this.blocks.set(b, {
				ids: w.ids ?? [],
				cells,
				text: cells.map((r) => r.map((v, c) => this.format(c, v)))
			});
			this.evict(b);
			this.error = null;
		} catch (e) {
			this.error = String(e);
		} finally {
			this.inflight.delete(b);
		}
		this.onLoad();
		this.pump();
	}

	/** Retire les blocs les plus éloignés du dernier chargé au-delà de maxBlocks. */
	private evict(near: number) {
		if (this.blocks.size <= this.maxBlocks) return;
		const far = [...this.blocks.keys()].sort((a, b) => Math.abs(b - near) - Math.abs(a - near));
		for (const b of far.slice(0, this.blocks.size - this.maxBlocks)) this.blocks.delete(b);
	}

	/** Met à jour une cellule déjà chargée (saisie ou retour du backend). */
	set(row: number, col: number, v: unknown) {
		const b = this.blocks.get(Math.floor(row / BLOCK));
		if (!b) return;
		const i = row % BLOCK;
		b.cells[i][col] = v;
		b.text[i][col] = this.format(col, v);
	}

	get size() {
		return this.blocks.size;
	}
}

function push(a: number[], v: number) {
	a.push(v);
	if (a.length > 500) a.shift();
}

/** Percentile p (0..1) d'une série, NaN si vide. */
export function percentile(a: number[], p: number): number {
	if (a.length === 0) return NaN;
	const s = [...a].sort((x, y) => x - y);
	return s[Math.min(s.length - 1, Math.floor(p * s.length))];
}
