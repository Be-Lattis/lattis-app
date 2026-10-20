<script lang="ts">
	// Graphiques sans axes cartésiens, dessinés dans la zone (x, y, w, h) de
	// Chart.svelte : secteurs 3D, compartimentage, radar, entonnoir, jauge.
	import { axisFormat, shortLabel, textOn, tickLabel, ticks } from './scale';
	import { gaugeValue, pie3d, radarPoint, ringPath, shade, squarify } from './shapes';
	import { numberFormat, t } from '#lib/i18n/index.js';

	interface Props {
		kind: string;
		labels: string[];
		series: { name: string; values: (number | null)[] }[];
		x: number;
		y: number;
		w: number;
		h: number;
		color: (k: number) => string;
		style: { labels?: boolean; noGrid?: boolean; yMin?: number; yMax?: number };
		/** Format des valeurs de la première série (celui de sa colonne). */
		fmt?: (v: number) => string;
	}

	let { kind, labels, series, x, y, w, h, color, style, fmt = tickLabel }: Props = $props();

	const first = $derived(series[0]?.values ?? []);
	const pct = $derived(numberFormat({ style: 'percent', maximumFractionDigits: 1 }));

	// Secteurs 3D : ellipse inclinée, épaisseur d'un dixième de la largeur.
	const pie = $derived.by(() => {
		const rx = Math.max(10, Math.min(w / 2, (h / 0.72) / 2) - 4);
		const ry = rx * 0.55;
		const depth = Math.max(6, rx * 0.16);
		return { rx, ry, depth, slices: pie3d(first, x + w / 2, y + (h - depth) / 2, rx, ry, depth) };
	});

	const tilesT = $derived(kind === 'treemap' ? squarify(first, x, y, w, h) : []);

	// Radar : une branche par catégorie, graduations communes.
	const rTicks = $derived.by(() => {
		let lo = 0;
		let hi = 0;
		for (const s of series) for (const v of s.values) if (v !== null) (lo = Math.min(lo, v)), (hi = Math.max(hi, v));
		return ticks(style.yMin ?? lo, style.yMax ?? hi);
	});
	const r0 = $derived(rTicks[0]);
	const r1 = $derived(rTicks[rTicks.length - 1]);
	const rFmt = $derived(axisFormat(rTicks));
	const R = $derived(Math.max(10, Math.min(w / 2 - 40, h / 2 - 14)));
	const C = $derived({ x: x + w / 2, y: y + h / 2 });
	const frac = (v: number) => (Math.min(Math.max(v, r0), r1) - r0) / (r1 - r0 || 1);
	const ring = (f: number) =>
		labels
			.map((_, i) => radarPoint(C.x, C.y, R, i, labels.length, f))
			.map((p, i) => `${i ? 'L' : 'M'} ${p.x} ${p.y}`)
			.join(' ') + ' Z';

	// Entonnoir : barres centrées, largeur proportionnelle à la valeur.
	const fMax = $derived(Math.max(0, ...first.map((v) => Math.abs(v ?? 0))));
	const fBand = $derived(h / Math.max(1, labels.length));

	// Jauge : demi-cercle.
	const g = $derived(gaugeValue(first, style.yMin, style.yMax));
	const gR = $derived(Math.max(10, Math.min(w / 2 - 10, h - 54)));
	const gC = $derived({ x: x + w / 2, y: y + (h + gR - 48) / 2 });
</script>

{#if kind === 'pie3d'}
	{#each pie.slices as s (s.i)}
		{#each s.walls as d, j (j)}<path {d} fill={shade(color(s.i), -0.3)} stroke={shade(color(s.i), -0.4)} stroke-width="0.5" />{/each}
	{/each}
	{#each pie.slices as s (s.i)}
		<path d={s.top} fill={color(s.i)} stroke="#fff" stroke-width="1"><title>{labels[s.i]} : {pct.format(s.part)}</title></path>
		{#if style.labels && s.part > 0.04}<text x={s.mid.x} y={s.mid.y + 4} text-anchor="middle" class="on-color" style:fill={textOn(color(s.i))}>{pct.format(s.part)}</text>{/if}
	{/each}
{:else if kind === 'treemap'}
	{#each tilesT as t (t.i)}
		<rect x={t.x} y={t.y} width={Math.max(0, t.w - 1)} height={Math.max(0, t.h - 1)} fill={color(t.i)}><title>{labels[t.i]} : {fmt(first[t.i] ?? 0)}</title></rect>
		{#if t.w > 40 && t.h > 18}
			<text x={t.x + 5} y={t.y + 15} class="on-color" style:fill={textOn(color(t.i))}>{shortLabel(labels[t.i], Math.max(3, Math.floor((t.w - 8) / 6.5)))}</text>
			{#if style.labels && t.h > 32}<text x={t.x + 5} y={t.y + 29} class="on-color light" style:fill={textOn(color(t.i))}>{fmt(first[t.i] ?? 0)}</text>{/if}
		{/if}
	{/each}
{:else if kind === 'radar'}
	{#if labels.length >= 3}
		{#each rTicks as t (t)}
			{#if !style.noGrid || t === r1}<path d={ring(frac(t))} fill="none" class="grid" />{/if}
			{#if t !== r0}<text x={C.x + 3} y={C.y - R * frac(t) + 11} class="small">{rFmt(t)}</text>{/if}
		{/each}
		{#each labels as l, i (i)}
			{@const p = radarPoint(C.x, C.y, R, i, labels.length, 1)}
			{@const q = radarPoint(C.x, C.y, R + 8, i, labels.length, 1)}
			<line x1={C.x} y1={C.y} x2={p.x} y2={p.y} class="grid" />
			<text x={q.x} y={q.y + 4} text-anchor={Math.abs(q.x - C.x) < 4 ? 'middle' : q.x > C.x ? 'start' : 'end'} class="small"><title>{l}</title>{shortLabel(l, 14)}</text>
		{/each}
		{#each series as s, k (k)}
			{@const ps = s.values.map((v, i) => radarPoint(C.x, C.y, R, i, labels.length, frac(v ?? r0)))}
			<path d={ps.map((p, i) => `${i ? 'L' : 'M'} ${p.x} ${p.y}`).join(' ') + ' Z'} fill={color(k)} fill-opacity="0.15" stroke={color(k)} stroke-width="2" />
			{#each ps as p, i (i)}
				{#if s.values[i] !== null}
					<circle cx={p.x} cy={p.y} r="2.5" fill={color(k)} />
					{#if style.labels}<text x={p.x} y={p.y - 6} text-anchor="middle" class="small">{fmt(s.values[i]!)}</text>{/if}
				{/if}
			{/each}
		{/each}
	{:else}
		<text x={C.x} y={C.y} text-anchor="middle" class="small">{t('chart.radarMin')}</text>
	{/if}
{:else if kind === 'funnel'}
	{#each labels as l, i (i)}
		{@const v = first[i] ?? 0}
		{@const bw = fMax ? Math.max(2, (Math.abs(v) / fMax) * w) : 0}
		<rect x={x + (w - bw) / 2} y={y + fBand * i + 1} width={bw} height={Math.max(1, fBand - 2)} fill={color(i)}><title>{l} : {fmt(v)}</title></rect>
		{#if fBand >= 12}
			{@const inside = bw > 90}
			<text x={inside ? x + w / 2 : x + (w + bw) / 2 + 4} y={y + fBand * (i + 0.5) + 4} text-anchor={inside ? 'middle' : 'start'} class={inside ? 'on-color' : 'small'} style:fill={inside ? textOn(color(i)) : undefined}>{shortLabel(l, 18)}{style.labels || inside ? ' · ' + fmt(v) : ''}</text>
		{/if}
	{/each}
{:else if kind === 'gauge'}
	<path d={ringPath(gC.x, gC.y, gR * 0.68, gR, Math.PI, 2 * Math.PI)} fill="#e5e7eb" />
	{#if g.frac > 0}<path d={ringPath(gC.x, gC.y, gR * 0.68, gR, Math.PI, Math.PI * (1 + g.frac))} fill={color(0)} />{/if}
	{@const a = Math.PI * (1 + g.frac)}
	<line x1={gC.x} y1={gC.y} x2={gC.x + gR * 0.9 * Math.cos(a)} y2={gC.y + gR * 0.9 * Math.sin(a)} stroke="#374151" stroke-width="2.5" stroke-linecap="round" />
	<circle cx={gC.x} cy={gC.y} r="4" fill="#374151" />
	<text x={gC.x - gR * 0.84} y={gC.y + 16} text-anchor="middle" class="small">{fmt(g.lo)}</text>
	<text x={gC.x + gR * 0.84} y={gC.y + 16} text-anchor="middle" class="small">{fmt(g.hi)}</text>
	<text x={gC.x} y={gC.y + 28} text-anchor="middle" class="big">{g.value === null ? '—' : fmt(g.value)}</text>
	{#if series[0]}<text x={gC.x} y={gC.y + 46} text-anchor="middle" class="small">{shortLabel(series[0].name, 24)}</text>{/if}
{/if}

<style>
	.small {
		font-size: 11px;
		fill: #4b5563;
	}
	.on-color {
		font-size: 11px;
		fill: #fff;
		font-weight: 600;
	}
	.light {
		font-weight: 400;
	}
	.big {
		font-size: 20px;
		font-weight: 600;
		fill: #1f2328;
	}
	.grid {
		stroke: #d1d5db;
	}
</style>
