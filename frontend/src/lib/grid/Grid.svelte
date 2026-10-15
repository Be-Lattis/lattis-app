<script lang="ts">
	// Grille du prototype (spike 1.1) : rendu <canvas> de la zone visible,
	// défilement virtuel, cache de blocs de lignes, éditeur DOM superposé à la
	// cellule active et clavier façon Excel.
	import { onMount, untrack } from 'svelte';
	import { RowCache, percentile, type Fetcher } from './cache';
	import { VScroll, clamp } from './scroll';
	import {
		alignment,
		defaultWidth,
		editValue,
		formatValue,
		typeIcon,
		typeLabel,
		type ColumnType
	} from './format';

	interface Column {
		name: string;
		type: ColumnType;
	}

	interface Props {
		name: string;
		rows: number;
		columns: Column[];
		fetcher: Fetcher;
		save: (id: number, col: number, raw: string) => Promise<unknown>;
		/** Lance la mesure dès l'ouverture et transmet le résultat (mode SLITABLE_BENCH). */
		autoBench?: (result: string) => void;
	}

	// Le tableau est fixé pour la durée de vie du composant (la page le recrée
	// s'il change) : on lit les props une fois.
	const props: Props = $props();
	const { name, rows, columns, fetcher, save, autoBench } = untrack(() => ({ ...props }));

	const ROW_H = 24;
	const HEAD_H = 30;
	const FONT = '13px "Segoe UI", system-ui, -apple-system, sans-serif';
	const FONT_BOLD = '600 13px "Segoe UI", system-ui, -apple-system, sans-serif';
	const FONT_SMALL = '11px "Segoe UI", system-ui, -apple-system, sans-serif';

	const C = {
		bg: '#ffffff',
		line: '#e3e5e8',
		head: '#f3f4f6',
		headText: '#1f2328',
		icon: '#8a9099',
		gutterText: '#6b7280',
		text: '#1f2328',
		placeholder: '#eef0f3',
		select: 'rgba(33, 115, 70, 0.10)',
		active: '#217346',
		activeHead: '#d3e7dc'
	};

	let wrap: HTMLDivElement;
	let scroller: HTMLDivElement;
	let canvas: HTMLCanvasElement;
	let input = $state<HTMLInputElement>();
	let ctx: CanvasRenderingContext2D;

	// Géométrie des colonnes.
	const widths = columns.map((c) => defaultWidth(c.type));
	const colX: number[] = [0];
	for (const w of widths) colX.push(colX[colX.length - 1] + w);
	const totalW = colX[colX.length - 1];
	const gutter = Math.max(48, String(rows).length * 8 + 20);

	const vs = new VScroll(rows * ROW_H, 0);
	let viewW = 0;
	let viewH = 0;

	const cache = new RowCache(
		fetcher,
		rows,
		(c, v) => formatValue(columns[c].type, v),
		() => schedule()
	);

	// Sélection : cellule active + ancre de la plage (Maj + flèches / glisser).
	let active = $state.raw({ row: 0, col: 0 });
	let anchor = { row: 0, col: 0 };
	let dragging = false;

	// Édition.
	let editing = $state(false);
	let editByTyping = false;
	let editValueText = $state('');

	// Barre d'état (mise à jour au plus 4 fois par seconde).
	let status = $state({
		cell: '',
		fetch: '',
		draw: '',
		fps: '',
		edit: '',
		error: '',
		blocks: 0
	});
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

	function visibleRange() {
		const first = Math.floor(vs.top / ROW_H);
		const last = Math.min(rows - 1, Math.floor((vs.top + vs.view - 1) / ROW_H));
		return { first, last };
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
		const { first, last } = visibleRange();
		cache.want(first, last);
		if (!cache.has(first, last)) missingFrames++;
		const y0 = HEAD_H + first * ROW_H - vs.top;

		let c0 = 0;
		while (c0 < columns.length - 1 && colX[c0 + 1] <= left) c0++;
		let c1 = c0;
		while (c1 < columns.length - 1 && colX[c1 + 1] < left + w - gutter) c1++;
		const cx = (c: number) => gutter + colX[c] - left;

		// Plage sélectionnée.
		const r0 = Math.min(active.row, anchor.row);
		const r1 = Math.max(active.row, anchor.row);
		const s0 = Math.min(active.col, anchor.col);
		const s1 = Math.max(active.col, anchor.col);
		if (r0 !== r1 || s0 !== s1) {
			ctx.fillStyle = C.select;
			const top = HEAD_H + r0 * ROW_H - vs.top;
			ctx.fillRect(cx(s0), top, colX[s1 + 1] - colX[s0], (r1 - r0 + 1) * ROW_H);
		}

		// Cellules, colonne par colonne (une seule zone de découpe par colonne).
		ctx.font = FONT;
		ctx.textBaseline = 'middle';
		for (let c = c0; c <= c1; c++) {
			const x = cx(c);
			const cw = widths[c];
			const align = alignment(columns[c].type);
			ctx.save();
			ctx.beginPath();
			ctx.rect(x, HEAD_H, cw, h - HEAD_H);
			ctx.clip();
			ctx.textAlign = align;
			const tx = align === 'left' ? x + 6 : align === 'right' ? x + cw - 6 : x + cw / 2;
			for (let r = first; r <= last; r++) {
				const y = y0 + (r - first) * ROW_H;
				const row = cache.row(r);
				if (!row) {
					ctx.fillStyle = C.placeholder;
					ctx.fillRect(x + 6, y + 8, cw * 0.6, ROW_H - 16);
					continue;
				}
				const s = row.text[c];
				if (s) {
					ctx.fillStyle = C.text;
					ctx.fillText(s, tx, y + ROW_H / 2);
				}
			}
			ctx.restore();
		}

		// Quadrillage.
		ctx.beginPath();
		ctx.strokeStyle = C.line;
		ctx.lineWidth = 1;
		for (let r = first; r <= last + 1; r++) {
			const y = Math.round(y0 + (r - first) * ROW_H) - 0.5;
			ctx.moveTo(gutter, y);
			ctx.lineTo(Math.min(w, cx(c1 + 1)), y);
		}
		for (let c = c0; c <= c1 + 1; c++) {
			const x = Math.round(cx(c)) - 0.5;
			ctx.moveTo(x, HEAD_H);
			ctx.lineTo(x, Math.min(h, y0 + (last - first + 1) * ROW_H));
		}
		ctx.stroke();

		// Cellule active.
		if (active.row >= first - 1 && active.row <= last + 1) {
			const y = HEAD_H + active.row * ROW_H - vs.top;
			ctx.save();
			ctx.beginPath();
			ctx.rect(gutter, HEAD_H, w - gutter, h - HEAD_H);
			ctx.clip();
			ctx.strokeStyle = C.active;
			ctx.lineWidth = 2;
			ctx.strokeRect(cx(active.col) + 0.5, y + 0.5, widths[active.col] - 1, ROW_H - 1);
			ctx.restore();
		}

		// En-têtes de colonnes (figés).
		ctx.fillStyle = C.head;
		ctx.fillRect(0, 0, w, HEAD_H);
		for (let c = c0; c <= c1; c++) {
			const x = cx(c);
			ctx.save();
			ctx.beginPath();
			ctx.rect(Math.max(x, gutter), 0, widths[c], HEAD_H);
			ctx.clip();
			if (c >= s0 && c <= s1) {
				ctx.fillStyle = C.activeHead;
				ctx.fillRect(x, 0, widths[c], HEAD_H);
			}
			ctx.font = FONT_SMALL;
			ctx.fillStyle = C.icon;
			ctx.textAlign = 'right';
			const icon = typeIcon(columns[c].type);
			ctx.fillText(icon, x + widths[c] - 6, HEAD_H / 2);
			const iconW = ctx.measureText(icon).width + 10;
			ctx.beginPath();
			ctx.rect(Math.max(x, gutter), 0, widths[c] - iconW, HEAD_H);
			ctx.clip();
			ctx.font = FONT_BOLD;
			ctx.fillStyle = C.headText;
			ctx.textAlign = 'left';
			ctx.fillText(columns[c].name, x + 6, HEAD_H / 2);
			ctx.restore();
		}

		// Numéros de ligne (figés) : position 1..n dans le tableau.
		ctx.fillStyle = C.head;
		ctx.fillRect(0, HEAD_H, gutter, h - HEAD_H);
		ctx.font = FONT_SMALL;
		ctx.textAlign = 'right';
		for (let r = first; r <= last; r++) {
			const y = y0 + (r - first) * ROW_H;
			if (r >= r0 && r <= r1) {
				ctx.fillStyle = C.activeHead;
				ctx.fillRect(0, y, gutter, ROW_H);
			}
			ctx.fillStyle = C.gutterText;
			ctx.fillText(String(r + 1), gutter - 8, y + ROW_H / 2);
		}

		ctx.beginPath();
		ctx.strokeStyle = C.line;
		ctx.moveTo(0, HEAD_H - 0.5);
		ctx.lineTo(w, HEAD_H - 0.5);
		ctx.moveTo(gutter - 0.5, 0);
		ctx.lineTo(gutter - 0.5, h);
		for (let c = c0; c <= c1 + 1; c++) {
			const x = Math.round(cx(c)) - 0.5;
			if (x < gutter) continue;
			ctx.moveTo(x, 0);
			ctx.lineTo(x, HEAD_H);
		}
		ctx.stroke();
		ctx.fillStyle = C.head;
		ctx.fillRect(0, 0, gutter - 1, HEAD_H - 1);

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

	function refreshStatus() {
		const s = cache.stats;
		const row = cache.row(active.row);
		const raw = row ? row.text[active.col] : '';
		status = {
			cell: `${columns[active.col].name} · ligne ${active.row + 1}${raw ? ' : ' + raw : ''}`,
			fetch: `bloc p50 ${ms(percentile(s.roundTrip, 0.5))} / p95 ${ms(percentile(s.roundTrip, 0.95))} (Go p50 ${ms(percentile(s.server, 0.5))})`,
			draw: `rendu p50 ${ms(percentile(drawTimes, 0.5))} / p95 ${ms(percentile(drawTimes, 0.95))}`,
			fps: `${frameStamps.length} img/s`,
			edit: `saisie ${ms(lastEditMs)}`,
			error: cache.error ?? status.error,
			blocks: cache.size
		};
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
		const cw = viewW - gutter;
		if (colX[col] < left) scroller.scrollLeft = colX[col];
		else if (colX[col + 1] > left + cw) scroller.scrollLeft = colX[col + 1] - cw;
		syncNative();
	}

	// --- Sélection et clavier ---------------------------------------------

	function moveTo(row: number, col: number, extend = false) {
		active = { row: clamp(row, 0, rows - 1), col: clamp(col, 0, columns.length - 1) };
		if (!extend) anchor = { ...active };
		ensureVisible(active.row, active.col);
		refreshStatus();
	}

	function cellAt(e: MouseEvent): { row: number; col: number } | null {
		const r = scroller.getBoundingClientRect();
		const x = e.clientX - r.left;
		const y = e.clientY - r.top;
		if (x < gutter || y < HEAD_H || x >= scroller.clientWidth || y >= scroller.clientHeight) return null;
		const vx = x - gutter + scroller.scrollLeft;
		let col = 0;
		while (col < columns.length - 1 && colX[col + 1] <= vx) col++;
		if (vx >= totalW) return null;
		const row = Math.floor((y - HEAD_H + vs.top) / ROW_H);
		if (row >= rows) return null;
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
		const cell = cellAt(e);
		if (!cell) return;
		startEdit(false);
	}

	function onKeyDown(e: KeyboardEvent) {
		if (editing) return;
		const page = Math.max(1, Math.floor(vs.view / ROW_H) - 1);
		const { row, col } = active;
		const ext = e.shiftKey;
		const ctrl = e.ctrlKey || e.metaKey;
		switch (e.key) {
			case 'ArrowDown':
				moveTo(ctrl ? rows - 1 : row + 1, col, ext);
				break;
			case 'ArrowUp':
				moveTo(ctrl ? 0 : row - 1, col, ext);
				break;
			case 'ArrowRight':
				moveTo(row, ctrl ? columns.length - 1 : col + 1, ext);
				break;
			case 'ArrowLeft':
				moveTo(row, ctrl ? 0 : col - 1, ext);
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
				moveTo(ctrl ? rows - 1 : row, columns.length - 1, ext);
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
			case 'Backspace':
				if (e.key === 'Backspace') startEdit(true, '');
				else clearSelection();
				break;
			default:
				if (e.key.length === 1 && !ctrl && !e.altKey) {
					startEdit(true, e.key);
				} else return;
		}
		e.preventDefault();
	}

	// --- Édition ------------------------------------------------------------

	function startEdit(byTyping: boolean, initial?: string) {
		const row = cache.row(active.row);
		if (!row) return;
		anchor = { ...active };
		editByTyping = byTyping;
		editValueText = initial ?? editValue(columns[active.col].type, row.cells[active.col]);
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
		const x = gutter + colX[active.col] - scroller.scrollLeft;
		const y = HEAD_H + active.row * ROW_H - vs.top;
		input.style.transform = `translate(${x}px, ${y}px)`;
		input.style.width = `${Math.max(widths[active.col], 60)}px`;
		input.style.textAlign = alignment(columns[active.col].type);
	}

	async function write(row: number, col: number, raw: string) {
		const r = cache.row(row);
		if (!r) return;
		const t0 = performance.now();
		try {
			const v = await save(r.id, col, raw);
			lastEditMs = performance.now() - t0;
			cache.set(row, col, v);
			status.error = '';
		} catch (e) {
			status.error = String(e instanceof Error ? e.message : e);
		}
		refreshStatus();
		schedule();
	}

	function commit() {
		if (!editing) return;
		editing = false;
		const { row, col } = active;
		const row_ = cache.row(row);
		const before = row_ ? editValue(columns[col].type, row_.cells[col]) : '';
		if (editValueText !== before) write(row, col, editValueText);
		wrap.focus();
	}

	function cancel() {
		editing = false;
		wrap.focus();
		schedule();
	}

	function clearSelection() {
		const r0 = Math.min(active.row, anchor.row);
		const r1 = Math.max(active.row, anchor.row);
		const c0 = Math.min(active.col, anchor.col);
		const c1 = Math.max(active.col, anchor.col);
		if ((r1 - r0 + 1) * (c1 - c0 + 1) > 1000) {
			status.error = 'Prototype : effacement limité à 1 000 cellules';
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

	// --- Mesures ------------------------------------------------------------

	/** Défilement continu puis sauts aléatoires, pour le critère de sortie du spike. */
	async function runBench() {
		bench = 'Mesure en cours…';
		const frames = (n: number) => new Promise<void>((res) => {
			let k = 0;
			const step = () => (++k >= n ? res() : requestAnimationFrame(step));
			requestAnimationFrame(step);
		});

		// 1. Défilement continu : 10 lignes par image pendant 5 s.
		vs.to(Math.random() * vs.maxTop * 0.5);
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

		// 2. Sauts aléatoires : temps jusqu'à ce que la zone visible soit chargée.
		const jumps: number[] = [];
		for (let i = 0; i < 20; i++) {
			vs.to(Math.random() * vs.maxTop);
			syncNative();
			const t0 = performance.now();
			const { first, last } = visibleRange();
			await new Promise<void>((res) => {
				const check = () => (cache.has(first, last) ? res() : requestAnimationFrame(check));
				check();
			});
			jumps.push(performance.now() - t0);
			await frames(2);
		}

		// 3. Saisie : aller-retour d'une écriture de cellule (valeur réécrite à l'identique).
		const edits: number[] = [];
		const { first: e0 } = visibleRange();
		for (let i = 0; i < 20; i++) {
			const row = cache.row(e0 + i);
			if (!row) continue;
			const t0 = performance.now();
			await save(row.id, 1, editValue(columns[1].type, row.cells[1]));
			edits.push(performance.now() - t0);
		}

		const result = {
			date: new Date().toISOString(),
			userAgent: navigator.userAgent,
			rows,
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
			`Saut : données en p50 ${result.jumpToDataP50} ms / p95 ${result.jumpToDataP95} ms · ` +
			`Saisie : p50 ${result.editP50} ms / p95 ${result.editP95} ms`;
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
			vs.view = viewH - HEAD_H;
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

<div class="grid-root">
	<div class="toolbar">
		<span class="title">{name}</span>
		<span class="muted">{rows.toLocaleString('fr-FR')} lignes × {columns.length} colonnes</span>
		<span class="spacer"></span>
		<button onclick={runBench}>Mesurer</button>
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
			<div style:width="{gutter + totalW}px" style:height="{HEAD_H + vs.spacer}px"></div>
		</div>
		{#if editing}
			<input
				class="editor"
				bind:this={input}
				bind:value={editValueText}
				onkeydown={onEditorKey}
				onblur={commit}
				title={typeLabel(columns[active.col].type)}
			/>
		{/if}
	</div>

	<div class="statusbar">
		<span class="cell">{status.cell}</span>
		{#if status.error}<span class="error">{status.error}</span>{/if}
		<span class="spacer"></span>
		<span>{status.fetch}</span>
		<span>{status.draw}</span>
		<span>{status.fps}</span>
		<span>{status.edit}</span>
		<span>{status.blocks} blocs</span>
	</div>
</div>

<style>
	.grid-root {
		display: flex;
		flex-direction: column;
		height: 100%;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.toolbar,
	.statusbar,
	.bench {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 4px 10px;
		white-space: nowrap;
		overflow: hidden;
	}
	.toolbar {
		border-bottom: 1px solid #e3e5e8;
		height: 34px;
	}
	.title {
		font-weight: 600;
	}
	.muted {
		color: #6b7280;
	}
	.spacer {
		flex: 1;
	}
	.bench {
		background: #eef6f1;
		color: #14532d;
		border-bottom: 1px solid #d3e7dc;
	}
	button {
		font: inherit;
		padding: 3px 12px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 4px;
		cursor: pointer;
	}
	button:hover {
		background: #f3f4f6;
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
		height: 24px;
		box-sizing: border-box;
		border: 2px solid #217346;
		padding: 0 4px;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		outline: none;
		background: #fff;
		box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
		z-index: 2;
	}
	.statusbar {
		border-top: 1px solid #e3e5e8;
		background: #f7f8f9;
		color: #4b5563;
		font-size: 12px;
		height: 26px;
	}
	.statusbar .cell {
		color: #1f2328;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.error {
		color: #b42318;
	}
</style>
