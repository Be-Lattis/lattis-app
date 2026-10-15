<script lang="ts">
	// Feuille façon Excel : une grille infinie (1 048 576 lignes × 16 384
	// colonnes, comme Excel) sur laquelle un tableau est posé à son ancre.
	// Pas d'adressage A1 : les numéros de ligne et les noms de colonnes ne
	// s'affichent que pour le tableau, et le bandeau du haut reprend les noms de
	// colonnes du tableau quand son en-tête sort de l'écran (comme Excel).
	// Rendu <canvas> de la zone visible, cache de blocs de lignes, éditeur DOM
	// superposé à la cellule active, barre de formule.
	import { onMount, untrack } from 'svelte';
	import { RowCache, percentile, type Fetcher } from './cache';
	import { VScroll, clamp } from './scroll';
	import { Axis } from './axis';
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
		/** Tableau posé sur la feuille : nom, ancre (cellule d'en-tête), lignes de données. */
		table: { name: string; row: number; col: number; rows: number; columns: Column[] };
		fetcher: Fetcher;
		save: (id: number, col: number, raw: string) => Promise<unknown>;
		onstatus?: (s: SheetStatus) => void;
		/** Lance la mesure dès l'ouverture et transmet le résultat (mode SLITABLE_BENCH). */
		autoBench?: (result: string) => void;
	}

	const props: Props = $props();
	// Le tableau est fixé pour la durée de vie du composant (la page le recrée
	// s'il change) : on lit les props une fois.
	const { table: T, fetcher, save, onstatus, autoBench } = untrack(() => ({ ...props }));
	const columns = T.columns;

	export const SHEET_ROWS = 1_048_576;
	export const SHEET_COLS = 16_384;
	const ROW_H = 22;
	const DEF_W = 88;
	const TOP_H = 22;
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
		activeStrip: '#d3e7dc'
	};

	// Géométrie du tableau sur la feuille.
	const headRow = T.row;
	const lastRow = T.row + T.rows;
	const firstCol = T.col;
	const lastCol = T.col + columns.length - 1;
	const inCols = (c: number) => c >= firstCol && c <= lastCol;
	const inRows = (r: number) => r >= headRow && r <= lastRow;
	const isData = (r: number, c: number) => inCols(c) && r > headRow && r <= lastRow;

	const colAxis = new Axis(
		SHEET_COLS,
		DEF_W,
		new Map(columns.map((c, i) => [firstCol + i, defaultWidth(c.type)]))
	);
	const margin = Math.max(44, String(T.rows).length * 8 + 18);

	const vs = new VScroll(SHEET_ROWS * ROW_H, 0);
	let viewW = 0;
	let viewH = 0;

	let wrap: HTMLDivElement;
	let scroller: HTMLDivElement;
	let canvas: HTMLCanvasElement;
	let input = $state<HTMLInputElement>();
	let fx: HTMLInputElement;
	let ctx: CanvasRenderingContext2D;

	const cache = new RowCache(
		fetcher,
		T.rows,
		(c, v) => formatValue(columns[c].type, v),
		() => {
			schedule();
			refreshFx();
		}
	);

	// Sélection : cellule active + ancre de la plage (Maj + flèches / glisser).
	let active = $state.raw({ row: T.row + 1, col: T.col });
	let anchor = { row: T.row + 1, col: T.col };
	let dragging = false;

	// Édition (dans la cellule ou dans la barre de formule).
	let editing = $state(false);
	let editByTyping = false;
	let editValueText = $state('');
	let fxText = $state('');
	let fxEditing = false;

	let message = '';
	let bench = $state('');

	// Mesures.
	const drawTimes: number[] = [];
	const frameStamps: number[] = [];
	let lastEditMs = NaN;
	let missingFrames = 0;

	let frame = 0;
	function schedule() {
		if (!frame) frame = requestAnimationFrame(draw);
	}

	/** Lignes de la feuille visibles. */
	function visibleRows() {
		const first = Math.floor(vs.top / ROW_H);
		const last = Math.min(SHEET_ROWS - 1, Math.floor((vs.top + vs.view - 1) / ROW_H));
		return { first, last };
	}

	/** Lignes de données du tableau visibles (indices de données), ou null. */
	function visibleData(first: number, last: number) {
		const a = Math.max(first, headRow + 1);
		const b = Math.min(last, lastRow);
		return a <= b ? { first: a - headRow - 1, last: b - headRow - 1 } : null;
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
		const data = visibleData(r0, r1);
		if (data) {
			cache.want(data.first, data.last);
			if (!cache.has(data.first, data.last)) missingFrames++;
		}
		const c0 = colAxis.indexAt(left);
		const c1 = colAxis.indexAt(left + w - margin);
		const cx = (c: number) => margin + colAxis.offset(c) - left;
		const ry = (r: number) => TOP_H + r * ROW_H - vs.top;
		const xEnd = Math.min(w, cx(c1 + 1));

		// Zone grisée qui sépare le tableau du reste de la feuille.
		ctx.fillStyle = C.gray;
		if (r0 <= lastRow + 1 && r1 >= lastRow + 1) {
			ctx.fillRect(cx(firstCol), ry(lastRow + 1), colAxis.offset(lastCol + 2) - colAxis.offset(firstCol), ROW_H);
		}
		if (c0 <= lastCol + 1 && c1 >= lastCol + 1) {
			ctx.fillRect(cx(lastCol + 1), ry(headRow), colAxis.size(lastCol + 1), (lastRow - headRow + 1) * ROW_H);
		}
		// En-tête du tableau.
		if (headRow >= r0 && headRow <= r1) {
			ctx.fillStyle = C.head;
			ctx.fillRect(cx(firstCol), ry(headRow), colAxis.offset(lastCol + 1) - colAxis.offset(firstCol), ROW_H);
		}

		// Plage sélectionnée.
		const s0 = Math.min(active.row, anchor.row);
		const s1 = Math.max(active.row, anchor.row);
		const k0 = Math.min(active.col, anchor.col);
		const k1 = Math.max(active.col, anchor.col);
		if (s0 !== s1 || k0 !== k1) {
			ctx.fillStyle = C.select;
			ctx.fillRect(cx(k0), ry(s0), colAxis.offset(k1 + 1) - colAxis.offset(k0), (s1 - s0 + 1) * ROW_H);
		}

		// Contenu du tableau, colonne par colonne (une découpe par colonne).
		ctx.textBaseline = 'middle';
		const tr0 = Math.max(r0, headRow);
		const tr1 = Math.min(r1, lastRow);
		for (let c = Math.max(c0, firstCol); c <= Math.min(c1, lastCol) && tr0 <= tr1; c++) {
			const i = c - firstCol;
			const x = cx(c);
			const cw = colAxis.size(c);
			ctx.save();
			ctx.beginPath();
			ctx.rect(x, TOP_H, cw, h - TOP_H);
			ctx.clip();
			for (let r = tr0; r <= tr1; r++) {
				const y = ry(r);
				if (r === headRow) {
					ctx.font = FONT_SMALL;
					ctx.fillStyle = C.icon;
					ctx.textAlign = 'right';
					const icon = typeIcon(columns[i].type);
					ctx.fillText(icon, x + cw - 6, y + ROW_H / 2);
					ctx.save();
					ctx.beginPath();
					ctx.rect(x, y, cw - ctx.measureText(icon).width - 10, ROW_H);
					ctx.clip();
					ctx.font = FONT_BOLD;
					ctx.fillStyle = C.headText;
					ctx.textAlign = 'left';
					ctx.fillText(columns[i].name, x + 6, y + ROW_H / 2);
					ctx.restore();
					continue;
				}
				const row = cache.row(r - headRow - 1);
				if (!row) {
					ctx.fillStyle = C.placeholder;
					ctx.fillRect(x + 6, y + 7, cw * 0.6, ROW_H - 14);
					continue;
				}
				const s = row.text[i];
				if (!s) continue;
				const align = alignment(columns[i].type);
				ctx.font = FONT;
				ctx.textAlign = align;
				ctx.fillStyle = C.text;
				ctx.fillText(s, align === 'left' ? x + 6 : align === 'right' ? x + cw - 6 : x + cw / 2, y + ROW_H / 2);
			}
			ctx.restore();
		}

		// Quadrillage de toute la feuille.
		ctx.beginPath();
		ctx.strokeStyle = C.grid;
		ctx.lineWidth = 1;
		for (let r = r0; r <= r1 + 1; r++) {
			const y = Math.round(ry(r)) - 0.5;
			ctx.moveTo(margin, y);
			ctx.lineTo(xEnd, y);
		}
		for (let c = c0; c <= c1 + 1; c++) {
			const x = Math.round(cx(c)) - 0.5;
			ctx.moveTo(x, TOP_H);
			ctx.lineTo(x, h);
		}
		ctx.stroke();

		// Bordure discrète du tableau.
		ctx.save();
		ctx.beginPath();
		ctx.rect(margin, TOP_H, w - margin, h - TOP_H);
		ctx.clip();
		ctx.strokeStyle = C.border;
		ctx.strokeRect(
			Math.round(cx(firstCol)) - 0.5,
			Math.round(ry(headRow)) - 0.5,
			colAxis.offset(lastCol + 1) - colAxis.offset(firstCol),
			(lastRow - headRow + 1) * ROW_H
		);
		ctx.beginPath();
		ctx.moveTo(cx(firstCol), Math.round(ry(headRow + 1)) - 0.5);
		ctx.lineTo(cx(lastCol + 1), Math.round(ry(headRow + 1)) - 0.5);
		ctx.stroke();

		// Cellule active.
		if (active.row >= r0 - 1 && active.row <= r1 + 1) {
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 2;
			ctx.strokeRect(cx(active.col) + 0.5, ry(active.row) + 0.5, colAxis.size(active.col) - 1, ROW_H - 1);
		}
		ctx.restore();

		// Bandeau du haut : vide (pas de lettres de colonnes), sauf quand
		// l'en-tête du tableau est sorti de l'écran : il reprend alors ses noms.
		ctx.fillStyle = C.strip;
		ctx.fillRect(0, 0, w, TOP_H);
		if (headRow < r0 && lastRow >= r0) {
			for (let c = Math.max(c0, firstCol); c <= Math.min(c1, lastCol); c++) {
				const x = cx(c);
				ctx.save();
				ctx.beginPath();
				ctx.rect(Math.max(x, margin), 0, colAxis.size(c), TOP_H);
				ctx.clip();
				ctx.fillStyle = C.head;
				ctx.fillRect(x, 0, colAxis.size(c), TOP_H);
				ctx.font = FONT_BOLD;
				ctx.fillStyle = C.headText;
				ctx.textAlign = 'left';
				ctx.fillText(columns[c - firstCol].name, x + 6, TOP_H / 2);
				ctx.restore();
			}
		}
		for (let c = Math.max(c0, k0); c <= Math.min(c1, k1); c++) {
			ctx.fillStyle = C.active;
			ctx.fillRect(cx(c), TOP_H - 2, colAxis.size(c), 2);
		}

		// Marge de gauche : numéros 1..n des lignes de données du tableau.
		ctx.fillStyle = C.strip;
		ctx.fillRect(0, TOP_H, margin, h - TOP_H);
		ctx.font = FONT_SMALL;
		ctx.textAlign = 'right';
		for (let r = r0; r <= r1; r++) {
			const y = ry(r);
			if (r >= s0 && r <= s1) {
				ctx.fillStyle = C.activeStrip;
				ctx.fillRect(0, y, margin, ROW_H);
			}
			if (r > headRow && r <= lastRow) {
				ctx.fillStyle = C.stripText;
				ctx.fillText(String(r - headRow), margin - 8, y + ROW_H / 2);
			}
		}
		ctx.beginPath();
		ctx.strokeStyle = C.stripLine;
		ctx.lineWidth = 1;
		ctx.moveTo(0, TOP_H - 0.5);
		ctx.lineTo(w, TOP_H - 0.5);
		ctx.moveTo(margin - 0.5, 0);
		ctx.lineTo(margin - 0.5, h);
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

	/** Désignation de la cellule active (pas d'adresse A1). */
	function cellName(r: number, c: number): string {
		if (!inCols(c) || !inRows(r)) return 'Hors tableau';
		const col = `${T.name}[${columns[c - firstCol].name}]`;
		return r === headRow ? `${col} · en-tête` : `${col} · ${r - headRow}`;
	}

	function refreshStatus() {
		const s = cache.stats;
		onstatus?.({
			cell: cellName(active.row, active.col),
			message,
			fetch: `bloc p50 ${ms(percentile(s.roundTrip, 0.5))} / p95 ${ms(percentile(s.roundTrip, 0.95))}`,
			draw: `rendu ${ms(percentile(drawTimes, 0.5))}`,
			fps: `${frameStamps.length} img/s`,
			edit: `saisie ${ms(lastEditMs)}`,
			blocks: cache.size
		});
	}

	function refreshFx() {
		if (fxEditing) return;
		const { row, col } = active;
		if (isData(row, col)) {
			const r = cache.row(row - headRow - 1);
			fxText = r ? editValue(columns[col - firstCol].type, r.cells[col - firstCol]) : '';
		} else if (row === headRow && inCols(col)) {
			fxText = columns[col - firstCol].name;
		} else {
			fxText = '';
		}
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
		const cw = viewW - margin;
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
		if (dr > 0) return [inCols(col) && row < headRow ? headRow : inCols(col) && row < lastRow ? lastRow : SHEET_ROWS - 1, col];
		if (dr < 0) return [inCols(col) && row > lastRow ? lastRow : inCols(col) && row > headRow ? headRow : 0, col];
		if (dc > 0) return [row, inRows(row) && col < firstCol ? firstCol : inRows(row) && col < lastCol ? lastCol : SHEET_COLS - 1];
		return [row, inRows(row) && col > lastCol ? lastCol : inRows(row) && col > firstCol ? firstCol : 0];
	}

	function cellAt(e: MouseEvent): { row: number; col: number } | null {
		const r = scroller.getBoundingClientRect();
		const x = e.clientX - r.left;
		const y = e.clientY - r.top;
		if (x < margin || y < TOP_H || x >= scroller.clientWidth || y >= scroller.clientHeight) return null;
		const col = colAxis.indexAt(x - margin + scroller.scrollLeft);
		const row = Math.min(SHEET_ROWS - 1, Math.floor((y - TOP_H + vs.top) / ROW_H));
		return { row, col };
	}

	function onMouseDown(e: MouseEvent) {
		const cell = cellAt(e);
		if (!cell) return;
		if (editing) commit();
		e.preventDefault();
		wrap.focus();
		moveTo(cell.row, cell.col, e.shiftKey);
		dragging = true;
	}

	function onMouseMove(e: MouseEvent) {
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
				if (ctrl) moveTo(lastRow, lastCol, ext);
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

	/** Vrai si la cellule active est modifiable dans ce prototype. */
	function editable(): boolean {
		const { row, col } = active;
		if (isData(row, col)) return !!cache.row(row - headRow - 1);
		say(
			row === headRow && inCols(col)
				? 'Renommer une colonne : à venir (phase 3)'
				: 'Saisie hors tableau (nouvel îlot, cellule libre) : à venir (phase 3)'
		);
		return false;
	}

	function startEdit(byTyping: boolean, initial?: string) {
		if (!editable()) return;
		const row = cache.row(active.row - headRow - 1)!;
		const i = active.col - firstCol;
		anchor = { ...active };
		editByTyping = byTyping;
		editValueText = initial ?? editValue(columns[i].type, row.cells[i]);
		editing = true;
		ensureVisible(active.row, active.col);
		requestAnimationFrame(() => {
			if (!input) return;
			placeEditor();
			input.focus();
			const n = input.value.length;
			input.setSelectionRange(n, n);
		});
	}

	function placeEditor() {
		if (!input) return;
		const x = margin + colAxis.offset(active.col) - scroller.scrollLeft;
		const y = TOP_H + active.row * ROW_H - vs.top;
		input.style.transform = `translate(${x}px, ${y}px)`;
		input.style.width = `${Math.max(colAxis.size(active.col), 60)}px`;
		input.style.textAlign = alignment(columns[active.col - firstCol].type);
	}

	async function write(row: number, col: number, raw: string) {
		const n = row - headRow - 1;
		const i = col - firstCol;
		const r = cache.row(n);
		if (!r) return;
		const t0 = performance.now();
		try {
			const v = await save(r.id, i, raw);
			lastEditMs = performance.now() - t0;
			cache.set(n, i, v);
			message = '';
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
		const r = cache.row(row - headRow - 1);
		const before = r ? editValue(columns[col - firstCol].type, r.cells[col - firstCol]) : '';
		if (editValueText !== before) write(row, col, editValueText);
		wrap.focus();
	}

	function cancel() {
		editing = false;
		wrap.focus();
		schedule();
	}

	function clearSelection() {
		const r0 = Math.max(Math.min(active.row, anchor.row), headRow + 1);
		const r1 = Math.min(Math.max(active.row, anchor.row), lastRow);
		const c0 = Math.max(Math.min(active.col, anchor.col), firstCol);
		const c1 = Math.min(Math.max(active.col, anchor.col), lastCol);
		if (r0 > r1 || c0 > c1) return;
		if ((r1 - r0 + 1) * (c1 - c0 + 1) > 1000) {
			say('Prototype : effacement limité à 1 000 cellules');
			return;
		}
		for (let r = r0; r <= r1; r++) for (let c = c0; c <= c1; c++) write(r, c, '');
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
	function onFxFocus() {
		fxEditing = true;
	}

	function onFxKey(e: KeyboardEvent) {
		e.stopPropagation();
		if (e.key === 'Escape') {
			fxEditing = false;
			refreshFx();
			wrap.focus();
		} else if (e.key === 'Enter') {
			fxEditing = false;
			const { row, col } = active;
			if (editable() && fxText !== '' + (cache.row(row - headRow - 1)?.cells[col - firstCol] ?? '')) {
				write(row, col, fxText);
			}
			wrap.focus();
			moveTo(row + 1, col);
		} else return;
		e.preventDefault();
	}

	function onFxBlur() {
		fxEditing = false;
		refreshFx();
	}

	// --- Mesures ------------------------------------------------------------

	/** Défilement continu, sauts aléatoires et saisies, pour le critère du spike 1.1. */
	export async function runBench() {
		bench = 'Mesure en cours…';
		const frames = (n: number) =>
			new Promise<void>((res) => {
				let k = 0;
				const step = () => (++k >= n ? res() : requestAnimationFrame(step));
				requestAnimationFrame(step);
			});
		const tableTop = (headRow + 1) * ROW_H;
		const tableSpan = T.rows * ROW_H - vs.view;

		// 1. Défilement continu dans le tableau : 10 lignes par image pendant 5 s.
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

		// 2. Sauts aléatoires dans le tableau : délai jusqu'aux données affichées.
		const jumps: number[] = [];
		for (let i = 0; i < 20; i++) {
			vs.to(tableTop + Math.random() * tableSpan);
			syncNative();
			const t0 = performance.now();
			const { first, last } = visibleRows();
			const d = visibleData(first, last)!;
			await new Promise<void>((res) => {
				const check = () => (cache.has(d.first, d.last) ? res() : requestAnimationFrame(check));
				check();
			});
			jumps.push(performance.now() - t0);
			await frames(2);
		}

		// 3. Saisie : aller-retour d'une écriture (valeur réécrite à l'identique).
		const edits: number[] = [];
		const { first: f0, last: f1 } = visibleRows();
		const d0 = visibleData(f0, f1)!;
		for (let i = 0; i < 20; i++) {
			const row = cache.row(d0.first + i);
			if (!row) continue;
			const t0 = performance.now();
			await save(row.id, 1, editValue(columns[1].type, row.cells[1]));
			edits.push(performance.now() - t0);
		}

		const result = {
			date: new Date().toISOString(),
			userAgent: navigator.userAgent,
			rows: T.rows,
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
			blockRoundTripP50: +percentile(cache.stats.roundTrip, 0.5).toFixed(2),
			blockRoundTripP95: +percentile(cache.stats.roundTrip, 0.95).toFixed(2),
			blockGoP50: +percentile(cache.stats.server, 0.5).toFixed(2)
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
		const up = () => (dragging = false);
		window.addEventListener('mouseup', up);
		const timer = setInterval(refreshStatus, 250);
		wrap.focus();
		refreshStatus();
		if (autoBench) setTimeout(async () => autoBench(JSON.stringify(await runBench())), 1500);
		return () => {
			ro.disconnect();
			scroller.removeEventListener('wheel', onWheel);
			window.removeEventListener('mouseup', up);
			clearInterval(timer);
			cancelAnimationFrame(frame);
		};
	});
</script>

<div class="sheet-root">
	<div class="formula-bar">
		<div class="name-box" title="Cellule active">{cellName(active.row, active.col)}</div>
		<span class="fx">fx</span>
		<input
			class="fx-input"
			bind:this={fx}
			bind:value={fxText}
			onfocus={onFxFocus}
			onkeydown={onFxKey}
			onblur={onFxBlur}
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
		>
			<div style:width="{margin + colAxis.total}px" style:height="{TOP_H + vs.spacer}px"></div>
		</div>
		{#if editing}
			<input
				class="editor"
				bind:this={input}
				bind:value={editValueText}
				onkeydown={onEditorKey}
				onblur={commit}
				title={typeLabel(columns[active.col - firstCol].type)}
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
