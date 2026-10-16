<script lang="ts">
	// Graphique en SVG, sans bibliothèque : barres (verticales ou horizontales,
	// groupées ou empilées), courbes, aires, secteurs, anneau, nuage de points.
	// Les données arrivent déjà agrégées par le classeur.
	import { PALETTE, parseNumber, tickLabel, ticks } from './scale';

	export interface Series {
		name: string;
		values: (number | null)[];
	}

	export type ChartKind = 'bar' | 'column' | 'line' | 'area' | 'pie' | 'doughnut' | 'scatter';

	export interface ChartStyle {
		title?: string;
		legend?: 'bottom' | 'top' | 'right' | 'none' | string;
		xTitle?: string;
		yTitle?: string;
		labels?: boolean;
		noGrid?: boolean;
		stacked?: boolean;
		yMin?: number;
		yMax?: number;
		colors?: string[];
	}

	interface Props {
		kind: ChartKind | string;
		labels: string[];
		series: Series[];
		width: number;
		height: number;
		style?: ChartStyle;
	}

	let { kind, labels, series, width, height, style = {} }: Props = $props();

	const color = (k: number) => style.colors?.[k] || PALETTE[k % PALETTE.length];
	const radial = $derived(kind === 'pie' || kind === 'doughnut');
	const horizontal = $derived(kind === 'column');
	const stacked = $derived(!!style.stacked && (kind === 'bar' || kind === 'column' || kind === 'area'));
	const legend = $derived(style.legend || 'bottom');
	const legendItems = $derived(radial ? labels.map((l, i) => ({ name: l, color: color(i) })) : series.map((s, k) => ({ name: s.name, color: color(k) })));

	// Marges : titre, légende, titres d'axes.
	const pad = $derived({
		top: (style.title ? 30 : 12) + (legend === 'top' ? 22 : 0),
		right: 16 + (legend === 'right' ? 130 : 0),
		bottom: (radial ? 12 : horizontal ? 26 : 30) + (style.xTitle ? 18 : 0) + (legend === 'bottom' ? 22 : 0),
		left: (radial ? 12 : horizontal ? 80 : 48) + (style.yTitle ? 18 : 0)
	});
	const plotW = $derived(Math.max(20, width - pad.left - pad.right));
	const plotH = $derived(Math.max(20, height - pad.top - pad.bottom));

	// Échelle des valeurs (empilées : sommes positives et négatives).
	const extent = $derived.by(() => {
		let lo = 0;
		let hi = 0;
		if (stacked) {
			for (let i = 0; i < labels.length; i++) {
				let p = 0;
				let n = 0;
				for (const s of series) {
					const v = s.values[i] ?? 0;
					if (v >= 0) p += v;
					else n += v;
				}
				hi = Math.max(hi, p);
				lo = Math.min(lo, n);
			}
		} else {
			for (const s of series) for (const v of s.values) if (v !== null) (lo = Math.min(lo, v)), (hi = Math.max(hi, v));
		}
		return [style.yMin ?? lo, style.yMax ?? hi];
	});
	const vTicks = $derived(ticks(extent[0], extent[1]));
	const v0 = $derived(style.yMin ?? vTicks[0]);
	const v1 = $derived(style.yMax ?? vTicks[vTicks.length - 1]);
	const shownTicks = $derived(vTicks.filter((t) => t >= v0 - 1e-9 && t <= v1 + 1e-9));
	const span = $derived(v1 - v0 || 1);
	// Position d'une valeur : verticale (y) ou horizontale (x pour « column »).
	const vy = (v: number) => pad.top + plotH - ((Math.min(Math.max(v, v0), v1) - v0) / span) * plotH;
	const vx = (v: number) => pad.left + ((Math.min(Math.max(v, v0), v1) - v0) / span) * plotW;
	const n = $derived(labels.length);
	const band = $derived((horizontal ? plotH : plotW) / Math.max(1, n));
	const cat = (i: number) => (horizontal ? pad.top + band * (i + 0.5) : pad.left + band * (i + 0.5));
	const every = $derived(Math.max(1, Math.ceil(n / Math.max(1, Math.floor((horizontal ? plotH / 16 : plotW / 60))))));
	const short = (s: string, k = 12) => (s.length > k ? s.slice(0, k - 1) + '…' : s);
	const fmt = (v: number) => tickLabel(v);

	// Barres : position de chaque rectangle (groupé ou empilé).
	const bars = $derived.by(() => {
		const out: { x: number; y: number; w: number; h: number; c: string; v: number }[] = [];
		const base = Math.max(v0, Math.min(0, v1));
		for (let i = 0; i < n; i++) {
			let pos = 0;
			let neg = 0;
			series.forEach((s, k) => {
				const v = s.values[i];
				if (v === null || v === undefined) return;
				let a: number;
				let b: number;
				let slot = 0;
				let size = band * 0.7;
				if (stacked) {
					if (v >= 0) (a = pos), (b = pos += v);
					else (a = neg), (b = neg += v);
				} else {
					a = base;
					b = v;
					size = (band * 0.7) / series.length;
					slot = k;
				}
				const start = band * i + band * 0.15 + size * slot;
				if (horizontal) {
					const x0 = vx(Math.min(a, b));
					const x1 = vx(Math.max(a, b));
					out.push({ x: x0, y: pad.top + start, w: Math.max(1, x1 - x0), h: Math.max(1, size - 1), c: color(k), v });
				} else {
					const y0 = vy(Math.max(a, b));
					const y1 = vy(Math.min(a, b));
					out.push({ x: pad.left + start, y: y0, w: Math.max(1, size - 1), h: Math.max(1, y1 - y0), c: color(k), v });
				}
			});
		}
		return out;
	});

	// Courbes et aires (empilées : cumul).
	const lines = $derived.by(() => {
		const acc = labels.map(() => 0);
		return series.map((s, k) => {
			const pts = s.values.map((v, i) => {
				if (v === null) return null;
				const lo = stacked ? acc[i] : Math.max(v0, Math.min(0, v1));
				const hi = stacked ? (acc[i] += v) : v;
				return { x: cat(i), y: vy(hi), y0: vy(lo), v };
			});
			return { pts, c: color(k) };
		});
	});
	const path = (pts: ({ x: number; y: number } | null)[]) =>
		pts
			.filter((p): p is { x: number; y: number } => !!p)
			.map((p, i) => `${i ? 'L' : 'M'} ${p.x} ${p.y}`)
			.join(' ');
	const areaPath = (pts: ({ x: number; y: number; y0: number } | null)[]) => {
		const p = pts.filter((q): q is { x: number; y: number; y0: number } => !!q);
		if (!p.length) return '';
		return `M ${p[0].x} ${p[0].y0} ` + p.map((q) => `L ${q.x} ${q.y}`).join(' ') + ` L ${p[p.length - 1].x} ${p[p.length - 1].y0} ` + [...p].reverse().map((q) => `L ${q.x} ${q.y0}`).join(' ') + ' Z';
	};

	// Nuage : X numérique tiré des libellés.
	const xs = $derived(labels.map((l, i) => parseNumber(l) ?? i));
	const xTicks = $derived(ticks(Math.min(...xs, 0), Math.max(...xs, 1)));
	const sx = (v: number) => pad.left + ((v - xTicks[0]) / (xTicks[xTicks.length - 1] - xTicks[0] || 1)) * plotW;

	// Secteurs et anneau (première série).
	const pieTotal = $derived((series[0]?.values ?? []).reduce<number>((a, v) => a + Math.max(0, v ?? 0), 0));
	const slices = $derived.by(() => {
		let a = -Math.PI / 2;
		return (series[0]?.values ?? []).map((v, i) => {
			const part = pieTotal ? Math.max(0, v ?? 0) / pieTotal : 0;
			const s = { from: a, to: a + part * 2 * Math.PI, part, color: color(i) };
			a = s.to;
			return s;
		});
	});
	const pieR = $derived(Math.max(10, Math.min(plotH, plotW) / 2));
	const pieC = $derived({ x: pad.left + plotW / 2, y: pad.top + plotH / 2 });
	const hole = $derived(kind === 'doughnut' ? pieR * 0.55 : 0);
	function arc(from: number, to: number) {
		const { x, y } = pieC;
		const R = pieR;
		const r = hole;
		const full = to - from >= 2 * Math.PI - 1e-9;
		if (full) to = from + 2 * Math.PI - 1e-4;
		const large = to - from > Math.PI ? 1 : 0;
		const p = (rad: number, a: number) => `${x + rad * Math.cos(a)} ${y + rad * Math.sin(a)}`;
		if (!r) return `M ${x} ${y} L ${p(R, from)} A ${R} ${R} 0 ${large} 1 ${p(R, to)} Z`;
		return `M ${p(R, from)} A ${R} ${R} 0 ${large} 1 ${p(R, to)} L ${p(r, to)} A ${r} ${r} 0 ${large} 0 ${p(r, from)} Z`;
	}
	const pct = new Intl.NumberFormat('fr-FR', { style: 'percent', maximumFractionDigits: 1 });
</script>

<svg {width} {height} viewBox="0 0 {width} {height}" role="img" aria-label={style.title || 'Graphique'}>
	<rect {width} {height} fill="#fff" />
	{#if style.title}<text x={width / 2} y="21" text-anchor="middle" class="title">{style.title}</text>{/if}

	{#if radial}
		{#each slices as s, i (i)}
			{#if s.part > 0}
				<path d={arc(s.from, s.to)} fill={s.color} stroke="#fff" stroke-width="1" />
				{#if style.labels && s.part > 0.04}
					{@const mid = (s.from + s.to) / 2}
					{@const rr = hole ? (pieR + hole) / 2 : pieR * 0.65}
					<text x={pieC.x + rr * Math.cos(mid)} y={pieC.y + rr * Math.sin(mid) + 4} text-anchor="middle" class="on-color">{pct.format(s.part)}</text>
				{/if}
			{/if}
		{/each}
	{:else}
		<!-- Axe des valeurs et quadrillage. -->
		{#each shownTicks as t (t)}
			{#if horizontal}
				{#if !style.noGrid || t === 0}<line x1={vx(t)} x2={vx(t)} y1={pad.top} y2={pad.top + plotH} class={t === 0 ? 'axis' : 'grid'} />{/if}
				<text x={vx(t)} y={pad.top + plotH + 14} text-anchor="middle" class="small">{fmt(t)}</text>
			{:else}
				{#if !style.noGrid || t === 0}<line x1={pad.left} x2={pad.left + plotW} y1={vy(t)} y2={vy(t)} class={t === 0 ? 'axis' : 'grid'} />{/if}
				<text x={pad.left - 6} y={vy(t) + 4} text-anchor="end" class="small">{fmt(t)}</text>
			{/if}
		{/each}
		<!-- Axe des catégories. -->
		{#if kind === 'scatter'}
			{#each xTicks as t (t)}
				<text x={sx(t)} y={pad.top + plotH + 14} text-anchor="middle" class="small">{fmt(t)}</text>
			{/each}
		{:else}
			{#each labels as l, i (i)}
				{#if i % every === 0}
					{#if horizontal}
						<text x={pad.left - 6} y={cat(i) + 4} text-anchor="end" class="small">{short(l)}</text>
					{:else}
						<text x={cat(i)} y={pad.top + plotH + 14} text-anchor="middle" class="small">{short(l)}</text>
					{/if}
				{/if}
			{/each}
		{/if}

		{#if kind === 'bar' || kind === 'column'}
			{#each bars as b, i (i)}
				<rect x={b.x} y={b.y} width={b.w} height={b.h} fill={b.c} />
				{#if style.labels}
					{#if horizontal}
						<text x={b.x + b.w + 3} y={b.y + b.h / 2 + 4} class="small">{fmt(b.v)}</text>
					{:else}
						<text x={b.x + b.w / 2} y={b.y - 3} text-anchor="middle" class="small">{fmt(b.v)}</text>
					{/if}
				{/if}
			{/each}
		{:else if kind === 'scatter'}
			{#each series as s, k (k)}
				{#each s.values as v, i (i)}
					{#if v !== null}<circle cx={sx(xs[i])} cy={vy(v)} r="3.5" fill={color(k)} />{/if}
				{/each}
			{/each}
		{:else}
			{#each lines as l, k (k)}
				{#if kind === 'area'}<path d={areaPath(l.pts)} fill={l.c} fill-opacity="0.35" stroke="none" />{/if}
				<path d={path(l.pts)} fill="none" stroke={l.c} stroke-width="2" />
				{#each l.pts as p, i (i)}
					{#if p}
						{#if kind === 'line'}<circle cx={p.x} cy={p.y} r="2.5" fill={l.c} />{/if}
						{#if style.labels}<text x={p.x} y={p.y - 6} text-anchor="middle" class="small">{fmt(p.v)}</text>{/if}
					{/if}
				{/each}
			{/each}
		{/if}

		{#if style.xTitle}<text x={pad.left + plotW / 2} y={pad.top + plotH + 32} text-anchor="middle" class="axis-title">{style.xTitle}</text>{/if}
		{#if style.yTitle}
			<text x="14" y={pad.top + plotH / 2} text-anchor="middle" class="axis-title" transform="rotate(-90 14 {pad.top + plotH / 2})">{style.yTitle}</text>
		{/if}
	{/if}

	<!-- Légende. -->
	{#if legend !== 'none'}
		{#if legend === 'right'}
			{#each legendItems.slice(0, 14) as it, i (i)}
				<rect x={width - 136} y={pad.top + i * 18} width="10" height="10" fill={it.color} />
				<text x={width - 122} y={pad.top + i * 18 + 9} class="small">{short(it.name, 18)}{radial && slices[i] ? ` (${pct.format(slices[i].part)})` : ''}</text>
			{/each}
		{:else}
			{@const y = legend === 'top' ? (style.title ? 34 : 12) : height - 14}
			{@const w = Math.min(130, (width - 20) / Math.max(1, legendItems.length))}
			{#each legendItems.slice(0, Math.floor((width - 20) / 60)) as it, i (i)}
				<rect x={10 + i * w} y={y - 8} width="10" height="10" fill={it.color} />
				<text x={10 + i * w + 14} y={y + 1} class="small">{short(it.name, Math.max(4, Math.floor((w - 18) / 6.5)))}</text>
			{/each}
		{/if}
	{/if}
</svg>

<style>
	svg {
		display: block;
		font-family: 'Segoe UI', system-ui, sans-serif;
	}
	.title {
		font-size: 14px;
		fill: #1f2328;
	}
	.axis-title {
		font-size: 12px;
		fill: #374151;
	}
	.small {
		font-size: 11px;
		fill: #4b5563;
	}
	.on-color {
		font-size: 11px;
		fill: #fff;
		font-weight: 600;
	}
	.grid {
		stroke: #e5e7eb;
	}
	.axis {
		stroke: #9aa0a6;
	}
</style>
