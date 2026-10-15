<script lang="ts">
	// Feuille façon Excel : une grille infinie (1 048 576 lignes × 16 384
	// colonnes, comme Excel) sur laquelle sont posés des tableaux et des
	// cellules libres. Pas d'adressage A1 : les numéros de ligne et les noms
	// de colonnes ne s'affichent que pour les tableaux, et le bandeau du haut
	// reprend les noms de colonnes d'un tableau quand son en-tête sort de
	// l'écran (comme Excel). Saisir n'importe où passe par la source, qui
	// décide (cellule libre, nouveau tableau, extension, élargissement).
	// Rendu <canvas> de la zone visible, cache de blocs de lignes par tableau,
	// éditeur DOM superposé à la cellule active, barre de formule.
	import { flushSync, onMount, untrack } from 'svelte';
	import { RowCache, percentile, type Fetched } from './cache';
	import { VScroll, clamp } from './scroll';
	import { Axis } from './axis';
	import { parseTSV, toTSV } from './clipboard';
	import {
		alignment,
		defaultWidth,
		editValue,
		formatValue,
		typeIcon,
		typeLabel,
		type ColumnType
	} from './format';

	export interface Column {
		name: string;
		type: ColumnType;
		fixed?: boolean; // type choisi par l'utilisateur (sinon inféré)
		errors?: number; // saisies hors type
	}

	export interface SheetTable {
		id: number;
		name: string;
		row: number; // ancre : ligne d'en-tête
		col: number;
		rows: number; // lignes de données
		columns: Column[];
	}

	export interface SheetData {
		tables: SheetTable[];
		cells: { row: number; col: number; raw: string }[];
		/** Largeurs choisies par l'utilisateur (sinon : selon le type). */
		widths?: { col: number; width: number }[];
	}

	/** Cible d'une saisie, pour la source. */
	export interface Target {
		table?: SheetTable;
		index?: number; // ligne de données
		column?: number; // colonne du tableau
		rowId?: number; // clé technique de la ligne
	}

	export interface SheetSource {
		window(table: SheetTable, first: number, count: number): Promise<Fetched>;
		/**
		 * Enregistre une saisie. Renvoie un message pour la barre d'état, ou la
		 * valeur stockée quand la source ne relit pas sa mise en page.
		 */
		setCell(row: number, col: number, raw: string, target: Target): Promise<string | void | { value: unknown }>;
		undo?(): Promise<unknown>;
		redo?(): Promise<unknown>;
		/** Colle un bloc (une seule étape d'annulation). */
		paste?(row: number, col: number, block: string[][]): Promise<string | void>;
		insertRow?(table: SheetTable, n: number): Promise<unknown>;
		deleteRow?(table: SheetTable, n: number): Promise<unknown>;
		insertColumn?(table: SheetTable, i: number): Promise<unknown>;
		deleteColumn?(table: SheetTable, i: number): Promise<unknown>;
		/** Nombre de valeurs qui ne se convertiraient pas au type to. */
		convertPreview?(table: SheetTable, i: number, to: ColumnType): Promise<number>;
		setType?(table: SheetTable, i: number, to: ColumnType): Promise<unknown>;
		setColumnWidth?(col: number, width: number): Promise<unknown>;
	}

	export interface SheetStatus {
		cell: string;
		message: string;
		fetch: string;
		draw: string;
		fps: string;
		edit: string;
		blocks: number;
	}

	interface Props {
		data: SheetData;
		source: SheetSource;
		onstatus?: (s: SheetStatus) => void;
		/** Lance la mesure dès l'ouverture et transmet le résultat (mode SLITABLE_BENCH). */
		autoBench?: (result: string) => void;
	}

	let { data, source, onstatus, autoBench }: Props = $props();

	export const SHEET_ROWS = 1_048_576;
	export const SHEET_COLS = 16_384;
	const ROW_H = 22;
	const DEF_W = 88;
	const TOP_H = 22;
	const MARGIN = 52;
	const FONT = '13px "Segoe UI", system-ui, -apple-system, sans-serif';
	const FONT_BOLD = '600 13px "Segoe UI", system-ui, -apple-system, sans-serif';
	const FONT_SMALL = '11px "Segoe UI", system-ui, -apple-system, sans-serif';

	const C = {
		bg: '#ffffff',
		grid: '#e2e4e7',
		strip: '#f5f6f7',
		stripLine: '#d4d7db',
		stripText: '#5f6368',
		head: '#e3efe7',
		headText: '#14532d',
		icon: '#6b8f78',
		border: '#8fbc9f',
		gray: '#f1f2f4',
		text: '#1f2328',
		placeholder: '#eef0f3',
		select: 'rgba(33, 115, 70, 0.10)',
		active: '#217346',
		activeStrip: '#d3e7dc',
		error: '#c0392b'
	};

	// --- Tableaux posés sur la feuille ------------------------------------

	interface Entry {
		t: SheetTable;
		cache: RowCache;
		last: number; // dernière ligne de la feuille occupée
		lastCol: number;
	}

	let entries: Entry[] = [];
	let cells = new Map<string, string>();
	let colAxis = $state.raw(new Axis(SHEET_COLS, DEF_W));
	const key = (r: number, c: number) => `${r},${c}`;

	function makeEntry(t: SheetTable): Entry {
		const e: Entry = {
			t,
			last: t.row + t.rows,
			lastCol: t.col + t.columns.length - 1,
			cache: new RowCache(
				(first, count) => source.window(e.t, first, count),
				t.rows,
				(c, v) => formatValue(e.t.columns[c]?.type ?? 'text', v),
				() => {
					schedule();
					refreshFx();
				}
			)
		};
		return e;
	}

	/** Recalcule la géométrie quand la mise en page change (après une saisie). */
	function applyData(d: SheetData) {
		const old = new Map(entries.map((e) => [e.t.id, e]));
		entries = d.tables.map((t) => {
			const e = old.get(t.id);
			if (!e) return makeEntry(t);
			e.t = t;
			e.last = t.row + t.rows;
			e.lastCol = t.col + t.columns.length - 1;
			e.cache.invalidate(t.rows);
			return e;
		});
		cells = new Map(d.cells.map((c) => [key(c.row, c.col), c.raw]));
		userWidths = new Map((d.widths ?? []).map((w) => [w.col, w.width]));
		rebuildAxis();
		refreshFx();
		schedule();
	}

	// Largeurs : choisies par l'utilisateur, sinon selon le type de la colonne
	// du premier tableau qui l'occupe, sinon la largeur par défaut.
	let userWidths = new Map<number, number>();
	function rebuildAxis(override?: { col: number; width: number }) {
		const widths = new Map<number, number>();
		for (const e of entries) {
			e.t.columns.forEach((c, i) => {
				if (!widths.has(e.t.col + i)) widths.set(e.t.col + i, defaultWidth(c.type));
			});
		}
		for (const [c, w] of userWidths) widths.set(c, w);
		if (override) widths.set(override.col, override.width);
		colAxis = new Axis(SHEET_COLS, DEF_W, widths);
	}

	const inTable = (e: Entry, r: number, c: number) => r >= e.t.row && r <= e.last && c >= e.t.col && c <= e.lastCol;
	const tableAt = (r: number, c: number) => entries.find((e) => inTable(e, r, c));

	const vs = new VScroll(SHEET_ROWS * ROW_H, 0);
	let viewW = 0;
	let viewH = 0;

	let wrap: HTMLDivElement;
	let scroller: HTMLDivElement;
	let canvas: HTMLCanvasElement;
	let input = $state<HTMLInputElement>();
	let ctx: CanvasRenderingContext2D;
	let mounted = false;

	// Sélection : cellule active + ancre de la plage (Maj + flèches / glisser).
	let active = $state.raw({ row: 0, col: 0 });
	let anchor = { row: 0, col: 0 };
	let dragging = false;

	// Édition (dans la cellule ou dans la barre de formule).
	let editing = $state(false);
	let editByTyping = false;
	let editValueText = $state('');
	let fxText = $state('');
	let nameBox = $state('');
	let fxEditing = false;

	let message = '';
	let bench = $state('');

	// Mesures.
	const drawTimes: number[] = [];
	const frameStamps: number[] = [];
	let lastEditMs = NaN;
	let missingFrames = 0;

	$effect(() => {
		const d = data;
		untrack(() => applyData(d));
	});

	let frame = 0;
	function schedule() {
		if (mounted && !frame) frame = requestAnimationFrame(draw);
	}

	function visibleRows() {
		const first = Math.floor(vs.top / ROW_H);
		const last = Math.min(SHEET_ROWS - 1, Math.floor((vs.top + vs.view - 1) / ROW_H));
		return { first, last };
	}

	/** Lignes de données d'un tableau visibles (indices de données), ou null. */
	function visibleData(e: Entry, first: number, last: number) {
		const a = Math.max(first, e.t.row + 1);
		const b = Math.min(last, e.last);
		return a <= b ? { first: a - e.t.row - 1, last: b - e.t.row - 1 } : null;
	}

	function isNumeric(s: string) {
		return /^[-+]?[\d\s  ]+([,.]\d+)?%?$/.test(s.trim());
	}

	function draw() {
		frame = 0;
		const t0 = performance.now();
		const w = viewW;
		const h = viewH;
		const dpr = window.devicePixelRatio || 1;
		ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
		ctx.fillStyle = C.bg;
		ctx.fillRect(0, 0, w, h);

		const left = scroller.scrollLeft;
		const { first: r0, last: r1 } = visibleRows();
		const c0 = colAxis.indexAt(left);
		const c1 = colAxis.indexAt(left + w - MARGIN);
		const cx = (c: number) => MARGIN + colAxis.offset(c) - left;
		const ry = (r: number) => TOP_H + r * ROW_H - vs.top;
		const xEnd = Math.min(w, cx(c1 + 1));
		const spanW = (a: number, b: number) => colAxis.offset(b + 1) - colAxis.offset(a);

		const visible = entries.filter((e) => e.last + 1 >= r0 && e.t.row <= r1 && e.lastCol + 1 >= c0 && e.t.col <= c1);
		let incomplete = false;
		for (const e of visible) {
			const d = visibleData(e, r0, r1);
			if (d) {
				e.cache.want(d.first, d.last);
				if (!e.cache.has(d.first, d.last)) incomplete = true;
			}
		}
		if (incomplete) missingFrames++;

		// Zones grisées qui séparent chaque tableau du reste, puis en-têtes.
		for (const e of visible) {
			ctx.fillStyle = C.gray;
			ctx.fillRect(cx(e.t.col), ry(e.last + 1), spanW(e.t.col, e.lastCol + 1), ROW_H);
			ctx.fillRect(cx(e.lastCol + 1), ry(e.t.row), colAxis.size(e.lastCol + 1), (e.last - e.t.row + 1) * ROW_H);
			ctx.fillStyle = C.head;
			ctx.fillRect(cx(e.t.col), ry(e.t.row), spanW(e.t.col, e.lastCol), ROW_H);
		}

		// Plage sélectionnée.
		const s0 = Math.min(active.row, anchor.row);
		const s1 = Math.max(active.row, anchor.row);
		const k0 = Math.min(active.col, anchor.col);
		const k1 = Math.max(active.col, anchor.col);
		if (s0 !== s1 || k0 !== k1) {
			ctx.fillStyle = C.select;
			ctx.fillRect(cx(k0), ry(s0), spanW(k0, k1), (s1 - s0 + 1) * ROW_H);
		}

		// Contenu des tableaux, colonne par colonne (une découpe par colonne).
		ctx.textBaseline = 'middle';
		for (const e of visible) {
			const tr0 = Math.max(r0, e.t.row);
			const tr1 = Math.min(r1, e.last);
			for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol) && tr0 <= tr1; c++) {
				const i = c - e.t.col;
				const col = e.t.columns[i];
				const x = cx(c);
				const cw = colAxis.size(c);
				ctx.save();
				ctx.beginPath();
				ctx.rect(x, TOP_H, cw, h - TOP_H);
				ctx.clip();
				for (let r = tr0; r <= tr1; r++) {
					const y = ry(r);
					if (r === e.t.row) {
						ctx.font = FONT_SMALL;
						ctx.fillStyle = C.icon;
						ctx.textAlign = 'right';
						const icon = typeIcon(col.type) + ' ▾';
						if (col.errors) {
							ctx.fillStyle = C.error;
							ctx.fillText('⚠ ' + col.errors, x + cw - 6 - ctx.measureText(icon + ' ').width, y + ROW_H / 2);
							ctx.fillStyle = C.icon;
						}
						ctx.fillText(icon, x + cw - 6, y + ROW_H / 2);
						ctx.save();
						ctx.beginPath();
						ctx.rect(x, y, cw - ctx.measureText(icon).width - 10, ROW_H);
						ctx.clip();
						ctx.font = FONT_BOLD;
						ctx.fillStyle = C.headText;
						ctx.textAlign = 'left';
						ctx.fillText(col.name, x + 6, y + ROW_H / 2);
						ctx.restore();
						continue;
					}
					const row = e.cache.row(r - e.t.row - 1);
					if (!row) {
						ctx.fillStyle = C.placeholder;
						ctx.fillRect(x + 6, y + 7, cw * 0.6, ROW_H - 14);
						continue;
					}
					const s = row.text[i];
					if (!s) continue;
					if (row.bad?.has(i)) {
						// Saisie hors type : valeur brute en rouge, coin rouge comme le
						// triangle d'Excel.
						ctx.font = FONT;
						ctx.textAlign = 'left';
						ctx.fillStyle = C.error;
						ctx.fillText(s, x + 6, y + ROW_H / 2);
						ctx.beginPath();
						ctx.moveTo(x + cw - 7, y + 1);
						ctx.lineTo(x + cw - 1, y + 1);
						ctx.lineTo(x + cw - 1, y + 7);
						ctx.closePath();
						ctx.fill();
						continue;
					}
					const align = alignment(col.type);
					ctx.font = FONT;
					ctx.textAlign = align;
					ctx.fillStyle = C.text;
					ctx.fillText(s, align === 'left' ? x + 6 : align === 'right' ? x + cw - 6 : x + cw / 2, y + ROW_H / 2);
				}
				ctx.restore();
			}
		}

		// Cellules libres : texte à gauche, nombres à droite, comme Excel.
		ctx.font = FONT;
		ctx.fillStyle = C.text;
		for (const [k, raw] of cells) {
			const [r, c] = k.split(',').map(Number);
			if (r < r0 || r > r1 || c < c0 || c > c1) continue;
			const x = cx(c);
			const cw = colAxis.size(c);
			const num = isNumeric(raw);
			ctx.save();
			ctx.beginPath();
			ctx.rect(Math.max(x, MARGIN), TOP_H, num ? cw : w, h - TOP_H);
			ctx.clip();
			ctx.textAlign = num ? 'right' : 'left';
			ctx.fillText(raw, num ? x + cw - 6 : x + 6, ry(r) + ROW_H / 2);
			ctx.restore();
		}

		// Quadrillage de toute la feuille.
		ctx.beginPath();
		ctx.strokeStyle = C.grid;
		ctx.lineWidth = 1;
		for (let r = r0; r <= r1 + 1; r++) {
			const y = Math.round(ry(r)) - 0.5;
			ctx.moveTo(MARGIN, y);
			ctx.lineTo(xEnd, y);
		}
		for (let c = c0; c <= c1 + 1; c++) {
			const x = Math.round(cx(c)) - 0.5;
			ctx.moveTo(x, TOP_H);
			ctx.lineTo(x, h);
		}
		ctx.stroke();

		ctx.save();
		ctx.beginPath();
		ctx.rect(MARGIN, TOP_H, w - MARGIN, h - TOP_H);
		ctx.clip();
		// Bordure discrète de chaque tableau, trait sous l'en-tête.
		ctx.strokeStyle = C.border;
		ctx.lineWidth = 1;
		for (const e of visible) {
			const x = Math.round(cx(e.t.col)) - 0.5;
			const y = Math.round(ry(e.t.row)) - 0.5;
			ctx.strokeRect(x, y, spanW(e.t.col, e.lastCol), (e.last - e.t.row + 1) * ROW_H);
			ctx.beginPath();
			ctx.moveTo(x, y + ROW_H);
			ctx.lineTo(x + spanW(e.t.col, e.lastCol), y + ROW_H);
			ctx.stroke();
		}
		// Cellule active.
		if (active.row >= r0 - 1 && active.row <= r1 + 1) {
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 2;
			ctx.strokeRect(cx(active.col) + 0.5, ry(active.row) + 0.5, colAxis.size(active.col) - 1, ROW_H - 1);
		}
		ctx.restore();

		// Bandeau du haut : vide (pas de lettres de colonnes), sauf quand
		// l'en-tête d'un tableau visible est sorti de l'écran.
		ctx.fillStyle = C.strip;
		ctx.fillRect(0, 0, w, TOP_H);
		for (const e of visible) {
			if (e.t.row >= r0 || e.last < r0) continue;
			for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol); c++) {
				const x = cx(c);
				ctx.save();
				ctx.beginPath();
				ctx.rect(Math.max(x, MARGIN), 0, colAxis.size(c), TOP_H);
				ctx.clip();
				ctx.fillStyle = C.head;
				ctx.fillRect(x, 0, colAxis.size(c), TOP_H);
				ctx.font = FONT_BOLD;
				ctx.fillStyle = C.headText;
				ctx.textAlign = 'left';
				ctx.fillText(e.t.columns[c - e.t.col].name, x + 6, TOP_H / 2);
				ctx.restore();
			}
		}
		for (let c = Math.max(c0, k0); c <= Math.min(c1, k1); c++) {
			ctx.fillStyle = C.active;
			ctx.fillRect(cx(c), TOP_H - 2, colAxis.size(c), 2);
		}

		// Marge de gauche : numéros 1..n des lignes de données des tableaux.
		ctx.fillStyle = C.strip;
		ctx.fillRect(0, TOP_H, MARGIN, h - TOP_H);
		ctx.font = FONT_SMALL;
		ctx.textAlign = 'right';
		for (let r = r0; r <= r1; r++) {
			const y = ry(r);
			if (r >= s0 && r <= s1) {
				ctx.fillStyle = C.activeStrip;
				ctx.fillRect(0, y, MARGIN, ROW_H);
			}
			const e = visible.find((e) => r > e.t.row && r <= e.last);
			if (e) {
				ctx.fillStyle = C.stripText;
				ctx.fillText(String(r - e.t.row), MARGIN - 8, y + ROW_H / 2);
			}
		}
		ctx.beginPath();
		ctx.strokeStyle = C.stripLine;
		ctx.lineWidth = 1;
		ctx.moveTo(0, TOP_H - 0.5);
		ctx.lineTo(w, TOP_H - 0.5);
		ctx.moveTo(MARGIN - 0.5, 0);
		ctx.lineTo(MARGIN - 0.5, h);
		ctx.stroke();

		if (editing) placeEditor();

		const t1 = performance.now();
		pushStat(drawTimes, t1 - t0);
		frameStamps.push(t1);
		while (frameStamps.length && frameStamps[0] < t1 - 1000) frameStamps.shift();
	}

	function pushStat(a: number[], v: number) {
		a.push(v);
		if (a.length > 300) a.shift();
	}

	const ms = (v: number) => (Number.isFinite(v) ? v.toFixed(v < 10 ? 1 : 0) + ' ms' : '–');

	/** Ce que contient une cellule, pour l'édition et la barre de formule. */
	function cellInfo(r: number, c: number) {
		const e = tableAt(r, c);
		if (!e) return { e, kind: 'free' as const, value: cells.get(key(r, c)) ?? '' };
		const i = c - e.t.col;
		if (r === e.t.row) return { e, i, kind: 'header' as const, value: e.t.columns[i].name };
		const n = r - e.t.row - 1;
		const row = e.cache.row(n);
		return {
			e,
			i,
			n,
			row,
			kind: 'data' as const,
			// Une saisie hors type se corrige à partir de sa valeur brute.
			value: row ? (row.bad?.get(i) ?? editValue(e.t.columns[i].type, row.cells[i])) : ''
		};
	}

	/** Désignation de la cellule active (pas d'adresse A1). */
	function cellName(r: number, c: number): string {
		const e = tableAt(r, c);
		if (!e) return cells.has(key(r, c)) ? 'Cellule libre' : '';
		const k = e.t.columns[c - e.t.col];
		const col = `${e.t.name}[${k.name}]`;
		if (r === e.t.row) {
			const errs = k.errors ? ` · ${k.errors} erreur${k.errors > 1 ? 's' : ''}` : '';
			return `${col} · ${typeLabel(k.type)} (${k.fixed ? 'fixé' : 'inféré'})${errs}`;
		}
		const bad = e.cache.row(r - e.t.row - 1)?.bad?.get(c - e.t.col);
		return `${col} · ${r - e.t.row}` + (bad !== undefined ? ` · « ${bad} » n'est pas du type ${typeLabel(k.type)}` : '');
	}

	function refreshStatus() {
		const stats = entries[0]?.cache.stats ?? { roundTrip: [], server: [] };
		onstatus?.({
			cell: cellName(active.row, active.col),
			message,
			fetch: `bloc p50 ${ms(percentile(stats.roundTrip, 0.5))}`,
			draw: `rendu ${ms(percentile(drawTimes, 0.5))}`,
			fps: `${frameStamps.length} img/s`,
			edit: `saisie ${ms(lastEditMs)}`,
			blocks: entries.reduce((n, e) => n + e.cache.size, 0)
		});
	}

	function refreshFx() {
		nameBox = cellName(active.row, active.col);
		if (fxEditing) return;
		fxText = cellInfo(active.row, active.col).value;
	}

	function say(m: string) {
		message = m;
		refreshStatus();
	}

	// --- Défilement -------------------------------------------------------

	function onScroll() {
		if (!vs.inSync(scroller.scrollTop)) vs.fromNative(scroller.scrollTop);
		schedule();
	}

	function onWheel(e: WheelEvent) {
		if (e.ctrlKey) return;
		const unit = e.deltaMode === 1 ? ROW_H : e.deltaMode === 2 ? vs.view : 1;
		let dy = e.deltaY * unit;
		let dx = e.deltaX * unit;
		if (e.shiftKey && dx === 0) [dx, dy] = [dy, 0];
		e.preventDefault();
		if (dy) {
			vs.by(dy);
			scroller.scrollTop = vs.toNative();
		}
		if (dx) scroller.scrollLeft += dx;
		schedule();
	}

	function syncNative() {
		scroller.scrollTop = vs.toNative();
		schedule();
	}

	function ensureVisible(row: number, col: number) {
		const top = row * ROW_H;
		if (top < vs.top) vs.to(top);
		else if (top + ROW_H > vs.top + vs.view) vs.to(top + ROW_H - vs.view);
		const left = scroller.scrollLeft;
		const cw = viewW - MARGIN;
		const x0 = colAxis.offset(col);
		const x1 = x0 + colAxis.size(col);
		if (x0 < left) scroller.scrollLeft = x0;
		else if (x1 > left + cw) scroller.scrollLeft = x1 - cw;
		syncNative();
	}

	// --- Sélection et clavier ---------------------------------------------

	function moveTo(row: number, col: number, extend = false) {
		active = { row: clamp(row, 0, SHEET_ROWS - 1), col: clamp(col, 0, SHEET_COLS - 1) };
		if (!extend) anchor = { ...active };
		message = '';
		ensureVisible(active.row, active.col);
		refreshFx();
		refreshStatus();
	}

	// Ctrl + flèche, comme Excel : saute au bord du bloc de données.
	function jump(row: number, col: number, dr: number, dc: number): [number, number] {
		const filled = (r: number, c: number) => !!tableAt(r, c) || cells.has(key(r, c));
		if (dr !== 0) {
			const e = entries.find((e) => col >= e.t.col && col <= e.lastCol && (dr > 0 ? e.last > row : e.t.row < row));
			const edges = entries
				.filter((e) => col >= e.t.col && col <= e.lastCol)
				.flatMap((e) => [e.t.row, e.last])
				.filter((r) => (dr > 0 ? r > row : r < row))
				.sort((a, b) => (dr > 0 ? a - b : b - a));
			if (e || edges.length) return [edges[0] ?? row, col];
			for (let r = row + dr; r >= 0 && r < Math.min(SHEET_ROWS, row + 10000) && r > row - 10000; r += dr) if (filled(r, col)) return [r, col];
			return [dr > 0 ? SHEET_ROWS - 1 : 0, col];
		}
		const edges = entries
			.filter((e) => row >= e.t.row && row <= e.last)
			.flatMap((e) => [e.t.col, e.lastCol])
			.filter((c) => (dc > 0 ? c > col : c < col))
			.sort((a, b) => (dc > 0 ? a - b : b - a));
		if (edges.length) return [row, edges[0]];
		return [row, dc > 0 ? SHEET_COLS - 1 : 0];
	}

	function cellAt(e: MouseEvent): { row: number; col: number } | null {
		const r = scroller.getBoundingClientRect();
		const x = e.clientX - r.left;
		const y = e.clientY - r.top;
		if (x < MARGIN || y < TOP_H || x >= scroller.clientWidth || y >= scroller.clientHeight) return null;
		const col = colAxis.indexAt(x - MARGIN + scroller.scrollLeft);
		const row = Math.min(SHEET_ROWS - 1, Math.floor((y - TOP_H + vs.top) / ROW_H));
		return { row, col };
	}

	// --- Largeur des colonnes : glisser une séparation du bandeau du haut ----

	let resizing: { col: number; startX: number; startW: number; width: number } | null = null;

	/** Colonne dont le bord droit est sous la souris (bandeau du haut), ou null. */
	function boundaryAt(e: MouseEvent): number | null {
		const r = scroller.getBoundingClientRect();
		const x = e.clientX - r.left;
		const y = e.clientY - r.top;
		if (y >= TOP_H || x <= MARGIN) return null;
		const px = x - MARGIN + scroller.scrollLeft;
		const c = colAxis.indexAt(px);
		const right = colAxis.offset(c + 1);
		const left = colAxis.offset(c);
		if (right - px <= 4) return c;
		if (px - left <= 4 && c > 0) return c - 1;
		return null;
	}

	function onMouseDown(e: MouseEvent) {
		const b = boundaryAt(e);
		if (b !== null && source.setColumnWidth) {
			e.preventDefault();
			resizing = { col: b, startX: e.clientX, startW: colAxis.size(b), width: colAxis.size(b) };
			return;
		}
		const cell = cellAt(e);
		if (!cell) return;
		if (editing) commit();
		e.preventDefault();
		if (e.button === 0 && openTypeMenu(e, cell)) return;
		wrap.focus();
		moveTo(cell.row, cell.col, e.shiftKey);
		dragging = true;
	}

	function onMouseMove(e: MouseEvent) {
		if (resizing) {
			resizing.width = Math.max(24, Math.round(resizing.startW + e.clientX - resizing.startX));
			rebuildAxis({ col: resizing.col, width: resizing.width });
			schedule();
			return;
		}
		scroller.style.cursor = source.setColumnWidth && boundaryAt(e) !== null ? 'col-resize' : '';
		if (!dragging) return;
		const cell = cellAt(e);
		if (!cell) return;
		active = cell;
		schedule();
	}

	function onDblClick(e: MouseEvent) {
		if (cellAt(e)) startEdit(false);
	}

	function onKeyDown(e: KeyboardEvent) {
		if (editing) return;
		const page = Math.max(1, Math.floor(vs.view / ROW_H) - 1);
		const { row, col } = active;
		const ext = e.shiftKey;
		const ctrl = e.ctrlKey || e.metaKey;
		const arrow = (dr: number, dc: number) => {
			const [r, c] = ctrl ? jump(row, col, dr, dc) : [row + dr, col + dc];
			moveTo(r, c, ext);
		};
		if (ctrl && (e.key === 'z' || e.key === 'Z') && source.undo) {
			e.preventDefault();
			source.undo().catch((err) => say(String(err?.message ?? err)));
			return;
		}
		if (ctrl && (e.key === 'y' || e.key === 'Y') && source.redo) {
			e.preventDefault();
			source.redo().catch((err) => say(String(err?.message ?? err)));
			return;
		}
		switch (e.key) {
			case 'ArrowDown':
				arrow(1, 0);
				break;
			case 'ArrowUp':
				arrow(-1, 0);
				break;
			case 'ArrowRight':
				arrow(0, 1);
				break;
			case 'ArrowLeft':
				arrow(0, -1);
				break;
			case 'PageDown':
				moveTo(row + page, col, ext);
				break;
			case 'PageUp':
				moveTo(row - page, col, ext);
				break;
			case 'Home':
				moveTo(ctrl ? 0 : row, 0, ext);
				break;
			case 'End':
				if (ctrl) {
					const last = entries.reduce((m, e) => Math.max(m, e.last), 0);
					const lastCol = entries.reduce((m, e) => Math.max(m, e.lastCol), 0);
					moveTo(last, lastCol, ext);
				}
				break;
			case 'Tab':
				moveTo(row, col + (e.shiftKey ? -1 : 1));
				break;
			case 'Enter':
				moveTo(row + (e.shiftKey ? -1 : 1), col);
				break;
			case 'F2':
				startEdit(false);
				break;
			case 'Delete':
				clearSelection();
				break;
			case 'Backspace':
				startEdit(true, '');
				break;
			default:
				if (e.key.length === 1 && !ctrl && !e.altKey) startEdit(true, e.key);
				else return;
		}
		e.preventDefault();
	}

	// --- Édition ------------------------------------------------------------

	function startEdit(byTyping: boolean, initial?: string) {
		const info = cellInfo(active.row, active.col);
		// Ligne pas encore chargée (juste après une saisie qui l'a créée) : la
		// frappe ne doit pas être perdue ; seule l'édition de la valeur (F2)
		// attend les données.
		if (info.kind === 'data' && !info.row && initial === undefined) return;
		anchor = { ...active };
		editByTyping = byTyping;
		editValueText = initial ?? info.value;
		// L'éditeur doit avoir le focus avant la frappe suivante : sinon une
		// saisie rapide perdrait des caractères.
		flushSync(() => (editing = true));
		ensureVisible(active.row, active.col);
		if (!input) return;
		placeEditor();
		input.focus();
		const n = input.value.length;
		input.setSelectionRange(n, n);
	}

	function placeEditor() {
		if (!input) return;
		const x = MARGIN + colAxis.offset(active.col) - scroller.scrollLeft;
		const y = TOP_H + active.row * ROW_H - vs.top;
		input.style.transform = `translate(${x}px, ${y}px)`;
		input.style.width = `${Math.max(colAxis.size(active.col), 80)}px`;
		const e = tableAt(active.row, active.col);
		input.style.textAlign = e && active.row > e.t.row ? alignment(e.t.columns[active.col - e.t.col].type) : 'left';
	}

	// Les saisies partent l'une après l'autre : la suivante peut dépendre de
	// la précédente (une saisie fait naître un tableau, la suivante le remplit).
	let writing: Promise<void> = Promise.resolve();
	function write(row: number, col: number, raw: string): Promise<void> {
		writing = writing.then(() => writeNow(row, col, raw));
		return writing;
	}

	async function writeNow(row: number, col: number, raw: string) {
		const info = cellInfo(row, col);
		const target =
			info.kind === 'free'
				? {}
				: { table: info.e!.t, column: info.i, index: info.kind === 'data' ? info.n : undefined, rowId: info.kind === 'data' ? info.row?.id : undefined };
		const t0 = performance.now();
		try {
			const m = await source.setCell(row, col, raw, target);
			lastEditMs = performance.now() - t0;
			if (m && typeof m === 'object') {
				if (info.kind === 'data' && info.n !== undefined) info.e!.cache.set(info.n, info.i!, m.value);
				message = '';
			} else message = m ?? '';
		} catch (e) {
			message = String(e instanceof Error ? e.message : e);
		}
		refreshFx();
		refreshStatus();
		schedule();
	}

	function commit() {
		if (!editing) return;
		editing = false;
		const { row, col } = active;
		if (editValueText !== cellInfo(row, col).value) write(row, col, editValueText);
		wrap.focus();
	}

	function cancel() {
		editing = false;
		wrap.focus();
		schedule();
	}

	async function clearSelection() {
		const r0 = Math.min(active.row, anchor.row);
		const r1 = Math.max(active.row, anchor.row);
		const c0 = Math.min(active.col, anchor.col);
		const c1 = Math.max(active.col, anchor.col);
		if ((r1 - r0 + 1) * (c1 - c0 + 1) > 1000) {
			say('Effacement limité à 1 000 cellules pour l’instant');
			return;
		}
		for (let r = r0; r <= r1; r++)
			for (let c = c0; c <= c1; c++) {
				const info = cellInfo(r, c);
				if (info.kind !== 'header' && info.value !== '') await write(r, c, '');
			}
	}

	function onEditorKey(e: KeyboardEvent) {
		e.stopPropagation();
		const { row, col } = active;
		switch (e.key) {
			case 'Enter':
				commit();
				moveTo(row + (e.shiftKey ? -1 : 1), col);
				break;
			case 'Tab':
				commit();
				moveTo(row, col + (e.shiftKey ? -1 : 1));
				break;
			case 'Escape':
				cancel();
				break;
			case 'ArrowDown':
			case 'ArrowUp':
			case 'ArrowLeft':
			case 'ArrowRight':
				if (!editByTyping) return;
				commit();
				moveTo(
					row + (e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0),
					col + (e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0)
				);
				break;
			default:
				return;
		}
		e.preventDefault();
	}

	// Barre de formule : même chemin d'écriture que l'éditeur de cellule.
	function onFxKey(e: KeyboardEvent) {
		e.stopPropagation();
		if (e.key === 'Escape') {
			fxEditing = false;
			refreshFx();
			wrap.focus();
		} else if (e.key === 'Enter') {
			fxEditing = false;
			const { row, col } = active;
			if (fxText !== cellInfo(row, col).value) write(row, col, fxText);
			wrap.focus();
			moveTo(row + 1, col);
		} else return;
		e.preventDefault();
	}

	// --- Presse-papiers -----------------------------------------------------

	function selection() {
		return {
			r0: Math.min(active.row, anchor.row),
			r1: Math.max(active.row, anchor.row),
			c0: Math.min(active.col, anchor.col),
			c1: Math.max(active.col, anchor.col)
		};
	}

	function onCopy(e: ClipboardEvent) {
		if (document.activeElement !== wrap || editing) return;
		const { r0, r1, c0, c1 } = selection();
		if ((r1 - r0 + 1) * (c1 - c0 + 1) > 100_000) {
			say('Copie limitée à 100 000 cellules pour l’instant');
			return;
		}
		const rows: string[][] = [];
		for (let r = r0; r <= r1; r++) {
			const line: string[] = [];
			for (let c = c0; c <= c1; c++) line.push(cellInfo(r, c).value);
			rows.push(line);
		}
		e.clipboardData?.setData('text/plain', toTSV(rows));
		e.preventDefault();
		say(`${rows.length * (c1 - c0 + 1)} cellule(s) copiée(s)`);
	}

	function onPaste(e: ClipboardEvent) {
		if (document.activeElement !== wrap || editing || !source.paste) return;
		const text = e.clipboardData?.getData('text/plain') ?? '';
		const block = parseTSV(text);
		if (block.length === 0) return;
		e.preventDefault();
		const { row, col } = active;
		const paste = source.paste;
		writing = writing.then(async () => {
			try {
				const m = await paste(row, col, block);
				message = m ?? `${block.length} ligne(s) collée(s)`;
			} catch (err) {
				message = String(err instanceof Error ? err.message : err);
			}
			anchor = { row: row + block.length - 1, col: col + Math.max(...block.map((r) => r.length)) - 1 };
			refreshFx();
			refreshStatus();
			schedule();
		});
	}

	// --- Menu de type (icône de l'en-tête) ------------------------------------

	const TYPES: ColumnType[] = ['text', 'integer', 'decimal', 'date', 'boolean'];
	let typeMenu = $state<{ x: number; y: number; e: Entry; i: number; pending?: { to: ColumnType; n: number } } | null>(null);

	/** Un clic sur l'icône de type d'un en-tête ouvre le menu de type. */
	function openTypeMenu(ev: MouseEvent, cell: { row: number; col: number }): boolean {
		const e = tableAt(cell.row, cell.col);
		if (!e || cell.row !== e.t.row || !source.setType) return false;
		const r = scroller.getBoundingClientRect();
		const right = MARGIN + colAxis.offset(cell.col + 1) - scroller.scrollLeft;
		const mx = ev.clientX - r.left;
		if (right - mx > 34) return false;
		moveTo(cell.row, cell.col);
		typeMenu = { x: mx, y: ev.clientY - r.top + 8, e, i: cell.col - e.t.col };
		return true;
	}

	async function chooseType(to: ColumnType, confirmed = false) {
		const m = typeMenu;
		if (!m) return;
		if (!confirmed && source.convertPreview && to !== m.e.t.columns[m.i].type) {
			const n = await source.convertPreview(m.e.t, m.i, to);
			if (n > 0) {
				typeMenu = { ...m, pending: { to, n } };
				return;
			}
		}
		typeMenu = null;
		const setType = source.setType!;
		await menuAction(() => setType(m.e.t, m.i, to));
	}

	// --- Menu contextuel ----------------------------------------------------

	let menu = $state<{ x: number; y: number; e: Entry; n: number; i: number; header: boolean } | null>(null);

	function onContextMenu(ev: MouseEvent) {
		const cell = cellAt(ev);
		ev.preventDefault();
		menu = null;
		if (!cell) return;
		const e = tableAt(cell.row, cell.col);
		if (!e) return;
		if (cell.row !== active.row || cell.col !== active.col) moveTo(cell.row, cell.col);
		const r = wrap.getBoundingClientRect();
		menu = { x: ev.clientX - r.left, y: ev.clientY - r.top, e, n: cell.row - e.t.row - 1, i: cell.col - e.t.col, header: cell.row === e.t.row };
	}

	async function menuAction(fn: (() => Promise<unknown>) | undefined) {
		menu = null;
		wrap.focus();
		if (!fn) return;
		writing = writing.then(async () => {
			try {
				await fn();
				message = '';
			} catch (err) {
				message = String(err instanceof Error ? err.message : err);
			}
			refreshStatus();
		});
	}

	// --- Mesures ------------------------------------------------------------

	/** Défilement continu, sauts aléatoires et saisies, sur le premier tableau. */
	export async function runBench() {
		const e = entries[0];
		if (!e) return;
		bench = 'Mesure en cours…';
		const frames = (n: number) =>
			new Promise<void>((res) => {
				let k = 0;
				const step = () => (++k >= n ? res() : requestAnimationFrame(step));
				requestAnimationFrame(step);
			});
		const tableTop = (e.t.row + 1) * ROW_H;
		const tableSpan = Math.max(0, e.t.rows * ROW_H - vs.view);

		vs.to(tableTop + Math.random() * tableSpan * 0.5);
		syncNative();
		await frames(30);
		const intervals: number[] = [];
		missingFrames = 0;
		let drawn = 0;
		let prev = performance.now();
		const end = prev + 5000;
		await new Promise<void>((res) => {
			const step = () => {
				const now = performance.now();
				intervals.push(now - prev);
				prev = now;
				drawn++;
				vs.by(10 * ROW_H);
				scroller.scrollTop = vs.toNative();
				draw();
				if (now < end) requestAnimationFrame(step);
				else res();
			};
			requestAnimationFrame(step);
		});
		const scrollMissing = missingFrames;
		const janky = intervals.filter((d) => d > 1000 / 30).length;

		const jumps: number[] = [];
		for (let i = 0; i < 20; i++) {
			vs.to(tableTop + Math.random() * tableSpan);
			syncNative();
			const t0 = performance.now();
			const { first, last } = visibleRows();
			const d = visibleData(e, first, last)!;
			await new Promise<void>((res) => {
				const check = () => (e.cache.has(d.first, d.last) ? res() : requestAnimationFrame(check));
				check();
			});
			jumps.push(performance.now() - t0);
			await frames(2);
		}

		const edits: number[] = [];
		const { first: f0, last: f1 } = visibleRows();
		const d0 = visibleData(e, f0, f1)!;
		for (let i = 0; i < 20; i++) {
			const n = d0.first + i;
			const row = e.cache.row(n);
			if (!row) continue;
			const t0 = performance.now();
			await source.setCell(e.t.row + 1 + n, e.t.col + 1, editValue(e.t.columns[1].type, row.cells[1]), {
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
			drawP50: +percentile(drawTimes, 0.5).toFixed(2),
			drawP95: +percentile(drawTimes, 0.95).toFixed(2),
			blockRoundTripP50: +percentile(e.cache.stats.roundTrip, 0.5).toFixed(2),
			blockRoundTripP95: +percentile(e.cache.stats.roundTrip, 0.95).toFixed(2),
			blockGoP50: +percentile(e.cache.stats.server, 0.5).toFixed(2)
		};
		console.log('slitable-bench', JSON.stringify(result));
		(window as unknown as { slitableBench: unknown }).slitableBench = result;
		bench =
			`Défilement : ${result.scroll.fps} img/s, image p95 ${result.scroll.frameP95} ms, ` +
			`${janky} images > 33 ms, ${scrollMissing}/${drawn} images incomplètes · ` +
			`Saut : p50 ${result.jumpToDataP50} ms · Saisie : p50 ${result.editP50} ms`;
		refreshStatus();
		return result;
	}

	onMount(() => {
		ctx = canvas.getContext('2d', { alpha: false })!;
		mounted = true;
		const resize = () => {
			const dpr = window.devicePixelRatio || 1;
			viewW = scroller.clientWidth;
			viewH = scroller.clientHeight;
			canvas.width = Math.round(viewW * dpr);
			canvas.height = Math.round(viewH * dpr);
			canvas.style.width = `${viewW}px`;
			canvas.style.height = `${viewH}px`;
			vs.view = viewH - TOP_H;
			vs.to(vs.top);
			syncNative();
		};
		const ro = new ResizeObserver(resize);
		ro.observe(wrap);
		resize();
		scroller.addEventListener('wheel', onWheel, { passive: false });
		const up = () => {
			dragging = false;
			if (!resizing) return;
			const { col, width, startW } = resizing;
			resizing = null;
			if (width === startW || !source.setColumnWidth) return;
			userWidths.set(col, width);
			const setWidth = source.setColumnWidth;
			writing = writing.then(() => setWidth(col, width).then(() => undefined, (err) => say(String(err?.message ?? err))));
		};
		window.addEventListener('mouseup', up);
		const timer = setInterval(refreshStatus, 250);
		window.addEventListener('copy', onCopy);
		window.addEventListener('paste', onPaste);
		wrap.focus();
		refreshStatus();
		if (autoBench) setTimeout(async () => autoBench(JSON.stringify(await runBench())), 1500);
		return () => {
			ro.disconnect();
			scroller.removeEventListener('wheel', onWheel);
			window.removeEventListener('mouseup', up);
			clearInterval(timer);
			window.removeEventListener('copy', onCopy);
			window.removeEventListener('paste', onPaste);
			cancelAnimationFrame(frame);
		};
	});

	/** Redonne le focus clavier à la feuille (après un dialogue, un menu…). */
	export function focus() {
		wrap?.focus();
	}
</script>

<div class="sheet-root">
	<div class="formula-bar">
		<div class="name-box" title="Cellule active">{nameBox || '—'}</div>
		<span class="fx">fx</span>
		<input
			class="fx-input"
			bind:value={fxText}
			onfocus={() => (fxEditing = true)}
			onkeydown={onFxKey}
			onblur={() => {
				fxEditing = false;
				refreshFx();
			}}
			spellcheck="false"
		/>
	</div>
	{#if bench}<div class="bench">{bench}</div>{/if}

	<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
	<div class="wrap" bind:this={wrap} tabindex="0" role="grid" onkeydown={onKeyDown}>
		<canvas bind:this={canvas}></canvas>
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="scroller"
			bind:this={scroller}
			onscroll={onScroll}
			onmousedown={onMouseDown}
			onmousemove={onMouseMove}
			ondblclick={onDblClick}
			oncontextmenu={onContextMenu}
		>
			<div style:width="{MARGIN + colAxis.total}px" style:height="{TOP_H + vs.spacer}px"></div>
		</div>
		{#if typeMenu}
			{@const m = typeMenu}
			{@const col = m.e.t.columns[m.i]}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" onmouseleave={() => (typeMenu = null)}>
				<div class="menu-title">Type de « {col.name} » : {col.fixed ? 'fixé' : 'inféré automatiquement'}</div>
				{#if m.pending}
					<p class="menu-note">
						{m.pending.n > 1
							? `${m.pending.n} valeurs ne peuvent pas devenir « ${typeLabel(m.pending.to)} » : elles seront gardées et marquées en erreur.`
							: `1 valeur ne peut pas devenir « ${typeLabel(m.pending.to)} » : elle sera gardée et marquée en erreur.`}
					</p>
					<button onclick={() => chooseType(m.pending!.to, true)}>Convertir quand même</button>
					<button onclick={() => (typeMenu = { ...m, pending: undefined })}>Annuler</button>
				{:else}
					{#each TYPES as t (t)}
						<button onclick={() => chooseType(t)}>
							<span class="check">{t === col.type ? '✓' : ''}</span>{typeIcon(t)}&nbsp;&nbsp;{typeLabel(t)}
						</button>
					{/each}
					{#if col.errors}<p class="menu-note error">{col.errors} saisie{col.errors > 1 ? 's' : ''} hors type dans la colonne</p>{/if}
				{/if}
			</div>
		{/if}
		{#if menu}
			{@const m = menu}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="menu" style:left="{m.x}px" style:top="{m.y}px" onmouseleave={() => (menu = null)}>
				<div class="menu-title">{m.e.t.name}</div>
				{#if !m.header}
					<button onclick={() => menuAction(source.insertRow && (() => source.insertRow!(m.e.t, m.n)))}>Insérer une ligne au-dessus</button>
					<button onclick={() => menuAction(source.insertRow && (() => source.insertRow!(m.e.t, m.n + 1)))}>Insérer une ligne en dessous</button>
					<button onclick={() => menuAction(source.deleteRow && (() => source.deleteRow!(m.e.t, m.n)))}>Supprimer la ligne</button>
					<hr />
				{/if}
				<button onclick={() => menuAction(source.insertColumn && (() => source.insertColumn!(m.e.t, m.i)))}>Insérer une colonne à gauche</button>
				<button onclick={() => menuAction(source.insertColumn && (() => source.insertColumn!(m.e.t, m.i + 1)))}>Insérer une colonne à droite</button>
				<button onclick={() => menuAction(source.deleteColumn && (() => source.deleteColumn!(m.e.t, m.i)))}>Supprimer la colonne « {m.e.t.columns[m.i]?.name} »</button>
			</div>
		{/if}
		{#if editing}
			<input
				class="editor"
				bind:this={input}
				bind:value={editValueText}
				onkeydown={onEditorKey}
				onblur={commit}
				title={(() => {
					const e = tableAt(active.row, active.col);
					return e ? typeLabel(e.t.columns[active.col - e.t.col].type) : 'Cellule libre';
				})()}
			/>
		{/if}
	</div>
</div>

<style>
	.sheet-root {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.formula-bar {
		display: flex;
		align-items: center;
		gap: 6px;
		height: 30px;
		padding: 0 8px;
		border-bottom: 1px solid #d4d7db;
		background: #fff;
	}
	.name-box {
		width: 260px;
		padding: 3px 8px;
		border: 1px solid #d4d7db;
		border-radius: 3px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		color: #374151;
	}
	.fx {
		color: #6b7280;
		font-style: italic;
		padding: 0 6px;
		border-left: 1px solid #d4d7db;
	}
	.fx-input {
		flex: 1;
		font: inherit;
		border: none;
		outline: none;
		padding: 4px 2px;
	}
	.bench {
		padding: 4px 10px;
		background: #eef6f1;
		color: #14532d;
		border-bottom: 1px solid #d3e7dc;
		white-space: nowrap;
		overflow: hidden;
	}
	.wrap {
		position: relative;
		flex: 1;
		min-height: 0;
		outline: none;
		overflow: hidden;
	}
	canvas {
		position: absolute;
		left: 0;
		top: 0;
	}
	.scroller {
		position: absolute;
		inset: 0;
		overflow: auto;
	}
	.menu {
		position: absolute;
		z-index: 5;
		background: #fff;
		border: 1px solid #d4d7db;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 4px 0;
		min-width: 240px;
	}
	.menu-title {
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
	}
	.menu button {
		display: block;
		width: 100%;
		text-align: left;
		padding: 6px 14px;
		background: none;
		border: none;
		font: inherit;
		cursor: pointer;
	}
	.menu button:hover {
		background: #e3efe7;
	}
	.menu-note {
		margin: 4px 14px;
		max-width: 260px;
		font-size: 12px;
		color: #4b5563;
	}
	.menu-note.error {
		color: #c0392b;
	}
	.check {
		display: inline-block;
		width: 16px;
		color: #217346;
	}
	.menu hr {
		border: none;
		border-top: 1px solid #e3e5e8;
		margin: 4px 0;
	}
	.editor {
		position: absolute;
		left: 0;
		top: 0;
		height: 22px;
		box-sizing: border-box;
		border: 2px solid #217346;
		padding: 0 4px;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		outline: none;
		background: #fff;
		box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
		z-index: 2;
	}
</style>
