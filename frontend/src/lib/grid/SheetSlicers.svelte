<script lang="ts">
	// Segments et chronologies des tableaux croisés de la feuille (pivotslicer.ts),
	// posés comme les graphiques : un clic sur un élément ne garde que lui, Ctrl+clic
	// en ajoute ou en retire (Maj+clic : une plage de périodes), l'entonnoir barré
	// efface le filtre. Glisser l'en-tête déplace, le coin redimensionne ; clic
	// droit : connexions à d'autres TCD de même source, supprimer. Chaque geste
	// est une modification du TCD (une étape d'annulation).
	import { errorText, locale, t } from '#lib/i18n/index.js';
	import { dismiss } from './dismiss.js';
	import type { PivotConfig, SheetPivot } from './sheet/model.js';
	import { periodLabel, slicerOn, slicerPick, slicerSize, timelinePick, withSlicer, type PivotSlicer } from './pivotslicer';
	import { DATE_LEVELS, levelName } from './pivotgroup';

	interface Geometry {
		colOffset(col: number): number;
		rowTop(row: number): number;
		colAt(x: number): number;
		rowAt(y: number): number;
	}

	interface Props {
		pivots: SheetPivot[];
		geo: Geometry;
		view: { x: number; y: number };
		zoom?: number;
		locked?: boolean;
		onset?: (id: number, cfg: PivotConfig) => Promise<unknown>;
		onwheel?: (ev: WheelEvent) => void;
		say: (msg: string) => void;
	}

	let { pivots, geo, view, zoom = 1, locked = false, onset, onwheel, say }: Props = $props();

	interface Placed {
		p: SheetPivot;
		k: number;
		s: PivotSlicer;
		items: { text: string; count: number }[];
		error: string;
	}

	const placed = $derived(
		pivots.flatMap((p) =>
			(p.config.slicers ?? []).map((s, k) => ({ p, k, s, items: p.slicers?.[k]?.items ?? [], error: p.slicers?.[k]?.error ?? '' }) as Placed)
		)
	);

	let gesture = $state<{ id: string; x: number; y: number; w: number; h: number } | null>(null);
	let menu = $state<{ it: Placed; x: number; y: number } | null>(null);
	const keyOf = (it: Placed) => `${it.p.id}:${it.k}`;

	function boxOf(it: Placed) {
		const { w, h } = slicerSize(it.s);
		const b = { x: geo.colOffset(it.s.col) + (it.s.offX ?? 0), y: geo.rowTop(it.s.row) + (it.s.offY ?? 0), w, h };
		return gesture?.id === keyOf(it) ? { ...b, ...gesture } : b;
	}

	async function save(it: Placed, s: PivotSlicer | null) {
		if (!onset || locked) return;
		try {
			await onset(it.p.id, withSlicer(it.p.config, it.k, s));
		} catch (err) {
			say(errorText(err));
		}
	}

	function pick(ev: MouseEvent, it: Placed, item: string) {
		if (it.s.kind === 'timeline') {
			save(it, { ...it.s, values: timelinePick(it.s.values, it.items.map((x) => x.text), item, ev.shiftKey) });
			return;
		}
		save(it, { ...it.s, values: slicerPick(it.s.values, it.items.map((x) => x.text), item, ev.ctrlKey || ev.metaKey) });
	}

	/** Glisser l'en-tête (dir vide) ou le coin (dir 'se'). */
	function start(ev: MouseEvent, it: Placed, dir = '') {
		if (ev.button !== 0 || locked || !onset) return;
		if (!dir && (ev.target as HTMLElement).closest('button, select')) return;
		ev.preventDefault();
		ev.stopPropagation();
		const b0 = boxOf(it);
		const x0 = ev.clientX;
		const y0 = ev.clientY;
		let moved = false;
		const move = (e: MouseEvent) => {
			const dx = (e.clientX - x0) / zoom;
			const dy = (e.clientY - y0) / zoom;
			if (!moved && Math.abs(dx) + Math.abs(dy) < 4) return;
			moved = true;
			gesture = dir ? { id: keyOf(it), x: b0.x, y: b0.y, w: Math.max(100, b0.w + dx), h: Math.max(60, b0.h + dy) } : { id: keyOf(it), x: Math.max(0, b0.x + dx), y: Math.max(0, b0.y + dy), w: b0.w, h: b0.h };
		};
		const up = () => {
			window.removeEventListener('mousemove', move);
			window.removeEventListener('mouseup', up);
			if (!moved || !gesture) return;
			const r = gesture;
			const col = geo.colAt(r.x);
			const row = geo.rowAt(r.y);
			save(it, { ...it.s, row, col, offX: Math.round(r.x - geo.colOffset(col)), offY: Math.round(r.y - geo.rowTop(row)), width: Math.round(r.w), height: Math.round(r.h) }).finally(() => (gesture = null));
		};
		window.addEventListener('mousemove', move);
		window.addEventListener('mouseup', up);
	}

	function openMenu(ev: MouseEvent, it: Placed) {
		ev.preventDefault();
		ev.stopPropagation();
		if (locked) return;
		menu = { it, x: ev.clientX, y: ev.clientY };
	}

	/** Autres TCD de même source, que le segment peut filtrer aussi. */
	const others = (it: Placed) => pivots.filter((q) => q.id !== it.p.id && !q.error && q.config.source.toLowerCase() === it.p.config.source.toLowerCase() && q.config.name).map((q) => q.config.name!);

	function toggleLink(it: Placed, name: string) {
		const on = (it.s.pivots ?? []).includes(name);
		const list = on ? (it.s.pivots ?? []).filter((n) => n !== name) : [...(it.s.pivots ?? []), name];
		const s: PivotSlicer = { ...it.s, pivots: list };
		if (!list.length) delete s.pivots;
		save(it, s);
	}

	/** Chronologie : la période p ouvre-t-elle une nouvelle année ? */
	const yearStart = (items: { text: string }[], i: number) => i === 0 || items[i].text.slice(0, 4) !== items[i - 1].text.slice(0, 4);
	const quarter = $derived(t('grid.pivot.group.quarter', { n: '{n}' }));
</script>

{#each placed as it (keyOf(it))}
	{@const box = boxOf(it)}
	{@const tl = it.s.kind === 'timeline'}
	{@const level = it.s.level ?? 'months'}
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions (segment : ses valeurs sont des boutons ; le cadre ne sert qu'à la souris) -->
	<div
		class="slicer"
		class:timeline={tl}
		role="group"
		aria-label={it.s.field}
		style:left="{box.x - view.x}px"
		style:top="{box.y - view.y}px"
		style:width="{box.w}px"
		style:height="{box.h}px"
		oncontextmenu={(ev) => openMenu(ev, it)}
		onwheel={(ev) => {
			if (!(ev.target as HTMLElement).closest('.items')) onwheel?.(ev);
		}}
		onmousedown={(ev) => ev.stopPropagation()}
	>
		<!-- svelte-ignore a11y_no_static_element_interactions (déplacement à la souris ; équivalent au clavier à faire, voir l'audit d'ergonomie) -->
		<header onmousedown={(ev) => start(ev, it)} title={tl ? t('grid.pivot.slicer.hintTimeline') : t('grid.pivot.slicer.hint')}>
			<span class="name">{it.s.field}</span>
			{#if tl}
				<select
					value={level}
					disabled={locked}
					aria-label={it.s.field}
					onchange={(ev) => save(it, { ...it.s, level: (ev.currentTarget as HTMLSelectElement).value as PivotSlicer['level'], values: undefined })}
				>
					{#each DATE_LEVELS as l (l)}<option value={l}>{levelName(l)}</option>{/each}
				</select>
			{/if}
			<button class="clear" disabled={!it.s.values?.length || locked} title={t('grid.pivot.slicer.clear')} aria-label={t('grid.pivot.slicer.clear')} onclick={() => save(it, { ...it.s, values: undefined })}>
				<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true"><path d="M2 3h10l-4 5v4l-2 1V8z" fill="none" stroke="currentColor" stroke-width="1.2" /><path d="M10 10l4 4M14 10l-4 4" stroke="#c0392b" stroke-width="1.4" /></svg>
			</button>
		</header>
		{#if it.error}
			<p class="note err">{it.error}</p>
		{:else if !it.items.length}
			<p class="note">{t('grid.pivot.slicer.empty')}</p>
		{:else if tl}
			<div class="items strip">
				{#each it.items as x, i (x.text)}
					<button class="period" class:on={slicerOn(it.s, x.text)} class:sel={!!it.s.values?.length && slicerOn(it.s, x.text)} title="{x.text} · {x.count}" onclick={(ev) => pick(ev, it, x.text)}>
						<span class="year">{level !== 'years' && yearStart(it.items, i) ? x.text.slice(0, 4) : ''}</span>
						<span>{periodLabel(x.text, level, locale(), quarter)}</span>
					</button>
				{/each}
			</div>
		{:else}
			<div class="items" style:grid-template-columns="repeat({it.s.columns || 1}, 1fr)">
				{#each it.items as x (x.text)}
					<button class="item" class:on={slicerOn(it.s, x.text)} title="{x.text || t('grid.filter.blanks')} · {x.count}" onclick={(ev) => pick(ev, it, x.text)}>{x.text === '' ? t('grid.filter.blanks') : x.text}</button>
				{/each}
			</div>
		{/if}
		{#if !locked && onset}
			<!-- svelte-ignore a11y_no_static_element_interactions (taille à la souris ; équivalent au clavier à faire, voir l'audit d'ergonomie) -->
			<div class="grip" onmousedown={(ev) => start(ev, it, 'se')}></div>
		{/if}
	</div>
{/each}

{#if menu}
	{@const m = menu}
	<div class="menu" role="menu" tabindex="-1" style:left="{m.x}px" style:top="{m.y}px" style:zoom={zoom === 1 ? undefined : 1 / zoom} use:dismiss={() => (menu = null)}>
		{#if others(m.it).length}
			<div class="sub">{t('grid.pivot.slicer.connect')}</div>
			{#each others(m.it) as n (n)}
				<button
					role="menuitemcheckbox"
					aria-checked={(m.it.s.pivots ?? []).includes(n)}
					onclick={() => {
						menu = null;
						toggleLink(m.it, n);
					}}><span class="check">{(m.it.s.pivots ?? []).includes(n) ? '✓' : ''}</span>{n}</button
				>
			{/each}
			<hr />
		{/if}
		{#if m.it.s.kind !== 'timeline'}
			<div class="sub">{t('grid.pivot.slicer.columns')}</div>
			{#each [1, 2, 3] as n (n)}
				<button
					role="menuitemradio"
					aria-checked={(m.it.s.columns || 1) === n}
					onclick={() => {
						menu = null;
						save(m.it, { ...m.it.s, columns: n === 1 ? undefined : n });
					}}><span class="check">{(m.it.s.columns || 1) === n ? '✓' : ''}</span>{n}</button
				>
			{/each}
			<hr />
		{/if}
		<button
			role="menuitem"
			class="danger"
			onclick={() => {
				menu = null;
				save(m.it, null);
			}}><span class="check"></span>{t('grid.pivot.slicer.remove')}</button
		>
	</div>
{/if}

<style>
	.slicer {
		position: absolute;
		pointer-events: auto;
		box-sizing: border-box;
		display: flex;
		flex-direction: column;
		background: #fff;
		border: 1px solid #b8bdc3;
		border-radius: 4px;
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.15);
		font-size: 12px;
		color: #1f2328;
		overflow: hidden;
	}
	header {
		display: flex;
		align-items: center;
		gap: 4px;
		padding: 4px 4px 4px 8px;
		border-bottom: 1px solid #e3e5e8;
		cursor: move;
		user-select: none;
	}
	.name {
		flex: 1;
		min-width: 0;
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	header select {
		font: inherit;
		font-size: 11px;
		max-width: 90px;
	}
	.clear {
		display: inline-flex;
		padding: 2px;
		border: 1px solid transparent;
		border-radius: 3px;
		background: none;
		color: #4b5563;
		cursor: pointer;
	}
	.clear:hover:not(:disabled) {
		background: #e3efe7;
	}
	.clear:disabled {
		opacity: 0.35;
		cursor: default;
	}
	.items {
		flex: 1;
		display: grid;
		align-content: start;
		gap: 4px;
		padding: 6px;
		overflow: auto;
	}
	.item {
		font: inherit;
		min-height: 24px;
		padding: 2px 6px;
		text-align: left;
		background: #f3f4f6;
		color: #9aa0a6;
		border: 1px solid #e3e5e8;
		border-radius: 3px;
		cursor: pointer;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.item.on {
		background: #d5e8db;
		border-color: #8fb79d;
		color: #14532d;
	}
	.item:hover {
		border-color: #217346;
	}
	.strip {
		display: flex;
		align-items: stretch;
		gap: 2px;
		overflow-x: auto;
		overflow-y: hidden;
	}
	.period {
		flex: none;
		display: flex;
		flex-direction: column;
		justify-content: flex-end;
		min-width: 40px;
		padding: 2px 4px;
		font: inherit;
		background: #f3f4f6;
		border: 1px solid #e3e5e8;
		border-radius: 3px;
		cursor: pointer;
		color: #4b5563;
	}
	.period .year {
		font-size: 10px;
		color: #6b7280;
		min-height: 12px;
	}
	.period.sel {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
	.period.sel .year {
		color: #e3efe7;
	}
	.period:hover {
		border-color: #217346;
	}
	.note {
		margin: 8px;
		color: #6b7280;
	}
	.note.err {
		color: #c0392b;
	}
	.grip {
		position: absolute;
		right: 0;
		bottom: 0;
		width: 10px;
		height: 10px;
		cursor: nwse-resize;
		background: linear-gradient(135deg, transparent 50%, #b8bdc3 50%);
	}
	.menu {
		position: fixed;
		z-index: 40;
		min-width: 180px;
		padding: 4px 0;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		font-size: 12px;
	}
	.menu button {
		display: flex;
		align-items: center;
		width: 100%;
		padding: 5px 14px 5px 8px;
		font: inherit;
		text-align: left;
		background: none;
		border: none;
		cursor: pointer;
		color: #1f2328;
	}
	.menu button:hover {
		background: #e3efe7;
	}
	.menu button.danger:hover {
		background: #fdecea;
		color: #a52a1e;
	}
	.menu .sub {
		padding: 2px 14px 2px 26px;
		font-size: 11px;
		font-weight: 600;
		color: #4b5563;
	}
	.menu hr {
		border: none;
		border-top: 1px solid #e3e5e8;
		margin: 4px 0;
	}
	.check {
		display: inline-block;
		width: 18px;
		color: #217346;
	}
</style>
