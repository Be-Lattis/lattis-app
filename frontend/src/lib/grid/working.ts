// Retour immédiat sur ce qui est traité : dès qu'une action vise des
// cellules, des lignes ou des colonnes (suppression d'une colonne, tri,
// collage, effacement, mise en forme…), la zone visée reçoit un reflet
// animé (dessiné par la grille, Sheet.svelte : drawWorking) jusqu'à la fin
// de l'action, réussie ou non. Sur un grand tableau, on voit tout de suite
// que l'action est prise en compte, et sur quoi.
//
// Ce module traduit un appel de la source de la feuille (SheetSource) en
// zones de la feuille (fonctions pures, testées) et enveloppe la source.

/** Zone de la feuille (rangs à partir de 0, bornes comprises). */
export interface WorkZone {
	r0: number;
	c0: number;
	r1: number;
	c1: number;
}

/** Place d'un tableau sur la feuille : en-tête en (row, col), dernière ligne et colonne occupées. */
export interface TablePlace {
	row: number;
	col: number;
	last: number;
	lastCol: number;
}

/** Ce que l'enveloppe sait de la feuille au moment de l'appel. */
export interface WorkLookup {
	/** Place du tableau désigné (objet SheetTable de la mise en page, ou nom). */
	table(t: unknown): TablePlace | undefined;
}

/** Lignes ou colonnes entières : bornées à la fin pratique de la feuille. */
const ALL = 1_000_000_000;

const tableArg = (t: unknown, look: WorkLookup) => look.table(t);
const num = (v: unknown) => (typeof v === 'number' && Number.isFinite(v) ? v : NaN);

function rect(r0: unknown, c0: unknown, r1: unknown, c1: unknown): WorkZone[] | null {
	const z = { r0: num(r0), c0: num(c0), r1: num(r1), c1: num(c1) };
	if ([z.r0, z.c0, z.r1, z.c1].some(Number.isNaN)) return null;
	return [{ r0: Math.min(z.r0, z.r1), c0: Math.min(z.c0, z.c1), r1: Math.max(z.r0, z.r1), c1: Math.max(z.c0, z.c1) }];
}

/** Colonnes i à j d'un tableau (rangs dans le tableau), en-tête compris. */
function tableCols(p: TablePlace | undefined, i: unknown, j: unknown = i): WorkZone[] | null {
	const a = num(i);
	const b = num(j);
	if (!p || Number.isNaN(a) || Number.isNaN(b)) return null;
	return [{ r0: p.row, c0: p.col + Math.min(a, b), r1: p.last, c1: Math.min(p.lastCol, p.col + Math.max(a, b)) }];
}

/** Ligne de données n d'un tableau. */
function tableRow(p: TablePlace | undefined, n: unknown): WorkZone[] | null {
	const k = num(n);
	if (!p || Number.isNaN(k)) return null;
	const r = Math.min(p.last, p.row + 1 + k);
	return [{ r0: r, c0: p.col, r1: r, c1: p.lastCol }];
}

const wholeTable = (p: TablePlace | undefined): WorkZone[] | null => (p ? [{ r0: p.row, c0: p.col, r1: p.last, c1: p.lastCol }] : null);

/** Bloc collé : hauteur et largeur du bloc. */
function blockZone(row: unknown, col: unknown, block: unknown): WorkZone[] | null {
	if (!Array.isArray(block) || !block.length) return null;
	const w = Math.max(1, ...block.map((r) => (Array.isArray(r) ? r.length : 1)));
	return rect(row, col, num(row) + block.length - 1, num(col) + w - 1);
}

/**
 * Zones visées par un appel de la source (méthode, arguments), ou null
 * (méthode sans zone : lecture, annulation, réglage…).
 */
export function workZones(method: string, args: unknown[], look: WorkLookup): WorkZone[] | null {
	const a = args;
	switch (method) {
		case 'deleteColumn':
		case 'insertColumn':
		case 'setType':
		case 'setRules':
			return tableCols(tableArg(a[0], look), a[1]);
		case 'suppressColumns':
			return tableCols(tableArg(a[0], look), a[1], a[2]);
		case 'deleteColumnSpans': {
			if (!Array.isArray(a[0])) return null;
			const out: WorkZone[] = [];
			for (const s of a[0] as { table: string; from: number; to: number }[]) out.push(...(tableCols(look.table(s?.table), s?.from, s?.to) ?? []));
			return out.length ? out : null;
		}
		case 'columnsToValues': {
			const p = tableArg(a[0], look);
			if (!Array.isArray(a[1])) return null;
			const out = (a[1] as number[]).flatMap((i) => tableCols(p, i) ?? []);
			return out.length ? out : null;
		}
		case 'deleteRow':
		case 'insertRow':
			return tableRow(tableArg(a[0], look), a[1]);
		case 'sortTable':
		case 'sortTableCustom':
		case 'removeDuplicates':
		case 'refreshTable':
		case 'clearFilters':
		case 'setFilter':
			return wholeTable(tableArg(a[0], look));
		case 'paste':
			return blockZone(a[0], a[1], a[2]);
		case 'clearRange':
		case 'clearCells':
		case 'applyStyle':
		case 'rangeToValues':
		case 'mergeCells':
		case 'unmergeCells':
			return rect(a[0], a[1], a[2], a[3]);
		case 'pasteBlock':
		case 'transfer':
		case 'pasteSpecial': {
			const d = a[method === 'pasteBlock' ? 0 : 2] as { r0?: number; c0?: number; r1?: number; c1?: number } | undefined;
			return d ? rect(d.r0, d.c0, d.r1, d.c1) : null;
		}
		case 'textToColumns':
			return rect(a[0], a[2], a[1], a[2]);
		case 'sheetLines': {
			const op = a[0];
			const x = num(a[1]);
			const y = num(a[2]);
			if (Number.isNaN(x) || Number.isNaN(y)) return null;
			// Insertion : avant x, y lignes ; suppression : x à y.
			const [i0, i1] = op === 'insertRows' || op === 'insertCols' ? [x, x + Math.max(1, y) - 1] : [Math.min(x, y), Math.max(x, y)];
			return op === 'insertRows' || op === 'deleteRows' ? [{ r0: i0, c0: 0, r1: i1, c1: ALL }] : [{ r0: 0, c0: i0, r1: ALL, c1: i1 }];
		}
		case 'applyLineStyle':
		case 'setLinesHidden': {
			const i0 = num(a[1]);
			const i1 = num(a[2]);
			if (Number.isNaN(i0) || Number.isNaN(i1)) return null;
			return a[0] ? [{ r0: Math.min(i0, i1), c0: 0, r1: Math.max(i0, i1), c1: ALL }] : [{ r0: 0, c0: Math.min(i0, i1), r1: ALL, c1: Math.max(i0, i1) }];
		}
	}
	return null;
}

/** Actions qui ont une zone (les autres méthodes passent telles quelles). */
const ZONED = new Set([
	'deleteColumn',
	'insertColumn',
	'setType',
	'setRules',
	'suppressColumns',
	'deleteColumnSpans',
	'columnsToValues',
	'deleteRow',
	'insertRow',
	'sortTable',
	'sortTableCustom',
	'removeDuplicates',
	'refreshTable',
	'clearFilters',
	'setFilter',
	'paste',
	'clearRange',
	'clearCells',
	'applyStyle',
	'rangeToValues',
	'mergeCells',
	'unmergeCells',
	'transfer',
	'pasteSpecial',
	'pasteBlock',
	'textToColumns',
	'sheetLines',
	'applyLineStyle',
	'setLinesHidden'
]);

/**
 * Enveloppe la source de la feuille : chaque action qui vise une zone
 * appelle start(zones) avant de partir et end(jeton) quand sa promesse est
 * tenue ou rompue. Les autres membres sont rendus tels quels.
 */
export function watchSource<S extends object>(source: S, look: WorkLookup, start: (zones: WorkZone[]) => number, end: (token: number) => void): S {
	const wrapped = new Map<string, unknown>();
	return new Proxy(source, {
		get(target, prop, receiver) {
			const v = Reflect.get(target, prop, receiver);
			if (typeof prop !== 'string' || typeof v !== 'function' || !ZONED.has(prop)) return v;
			let w = wrapped.get(prop) as { fn: unknown; call: unknown } | undefined;
			if (!w || w.fn !== v) {
				const fn = v as (...args: unknown[]) => unknown;
				w = {
					fn,
					call: (...args: unknown[]) => {
						let token = -1;
						try {
							const zones = workZones(prop, args, look);
							if (zones?.length) token = start(zones);
						} catch {
							token = -1;
						}
						let out: unknown;
						try {
							out = Reflect.apply(fn, target, args);
						} catch (err) {
							if (token >= 0) end(token);
							throw err;
						}
						if (token >= 0) {
							if (out && typeof (out as Promise<unknown>).then === 'function') {
								(out as Promise<unknown>).then(
									() => end(token),
									() => end(token)
								);
							} else end(token);
						}
						return out;
					}
				};
				wrapped.set(prop, w);
			}
			return w.call;
		}
	});
}

/** Délai avant le reflet : une action rapide ne fait rien clignoter. */
export const WORK_DELAY = 150;
/** Durée de l'apparition du reflet, après le délai. */
export const WORK_FADE = 250;
/** Durée d'un passage du reflet sur la zone. */
export const WORK_SWEEP = 1400;

/** Opacité du reflet (0..1) à l'instant now, pour une action commencée à t0. */
export function workAlpha(now: number, t0: number): number {
	return Math.min(1, Math.max(0, (now - t0 - WORK_DELAY) / WORK_FADE));
}

/** Position (0..1) de la bande lumineuse dans son passage. */
export function sweepAt(now: number, t0: number): number {
	const k = (now - t0) / WORK_SWEEP;
	return k - Math.floor(k);
}
